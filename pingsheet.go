// Copyright (c) 2020, Adam Vakil-Kirchberger
// Licensed under the MIT license

package pingsheet

import (
	"errors"
	"os"
	"time"

	"github.com/adamkirchberger/pingsheet/pkg/config"
	"github.com/adamkirchberger/pingsheet/pkg/gsheets"

	ping "github.com/adamkirchberger/pingsheet/pkg"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/sheets/v4"
)

// Pingsheet is the main structure for daemon data
type Pingsheet struct {
	SheetID  string
	hostname string
	secret   string
	svc      *sheets.Service
	host     *config.Host
}

const (
	configPullInterval int = 300 // Interval in secs between pulling new config
)

// NewPingsheet is used to create a new Pingsheet instance
func NewPingsheet(sheetID, keyPath, hostname, secret string) (*Pingsheet, error) {
	svc, err := gsheets.NewService(keyPath)
	if err != nil {
		log.Error().Msgf("Error creating GSheets service: %s", err)
		os.Exit(1)
	}

	p := &Pingsheet{
		SheetID:  sheetID,
		hostname: hostname,
		secret:   secret,
		svc:      svc,
		host:     nil,
	}

	err = p.pullLatestConfig()
	if err != nil {
		log.Error().Msgf("Error registering host: %s", err)
		os.Exit(1)
	}
	return p, nil
}

// Run is used to start the daemon
func (p *Pingsheet) Run() {
	log.Info().Msg("Start host daemon")

	// Make a sheet for host if one isn't present
	gsheets.MakeWorksheet(p.svc, p.SheetID, p.host.Hostname)

	configTime := time.Now()
	for {
		// Pull new config
		if time.Since(configTime) >= time.Duration(configPullInterval)*time.Second {
			updateTime := time.Now()
			log.Info().Msgf("Config update start")
			err := p.pullLatestConfig()
			if err != nil {
				log.Error().Msgf("An error has been encountered: %s", err)
				log.Info().Msgf("We will try again in 60 seconds")
				time.Sleep(p.host.Interval)
				continue
			}
			configTime = time.Now() // Reset last config pull time
			log.Info().Msgf("Config update finish: duration %s", time.Since(updateTime))

			// Clear old rows
			err = p.clearOldRows()
			if err != nil {
				log.Error().Msgf("Error when deleting rows: %s", err)
			}
		}

		// Run tests
		startTime := time.Now()
		log.Debug().Msgf("Ping targets start")
		p.prepHeaders()
		p.pingTargets()

		// Tests complete
		dur := time.Since(startTime)
		log.Debug().Msgf("Ping targets finish: duration %s", dur.String())
		log.Debug().Msgf("Sleep for %s", (p.host.Interval - dur).String())

		// Wait for interval
		time.Sleep(p.host.Interval - dur)
	}
}

// pullLatestConfig will get the latest host config and configure targets
func (p *Pingsheet) pullLatestConfig() error {
	cfgMap, err := gsheets.MapFromSheet(p.svc, p.SheetID, "CONFIG")
	if err != nil {
		return err
	}

	var hosts config.Hosts
	err = hosts.BuildHosts(cfgMap)
	if err != nil {
		log.Error().Msgf("Error in config: %s", err)
	}

	host := hosts.Authenticate(p.hostname, p.secret)
	if host == nil {
		log.Debug().Msgf("Host authentication failed")
		return errors.New("hostname and matching secret not found")
	} else {
		log.Debug().Msgf("Host authentication successful")
	}

	// Update with new host
	p.host = host

	// Update our host ID with the worksheet ID
	worksheetID, err := gsheets.GetWorksheetID(p.svc, p.SheetID, p.host.Hostname)
	if err != nil {
		log.Warn().Msgf("Host worksheet was not found, one will be created")
		p.host.ID = 0
	} else {
		log.Debug().Msgf("Host worksheet found with ID %d", worksheetID)
		p.host.ID = worksheetID
	}

	return nil
}

// clearOldRows ensures that rows in a host sheet do not exceed MAXROWS
func (p *Pingsheet) clearOldRows() error {
	currTotal, err := gsheets.GetWorksheetTotalRows(p.svc, p.SheetID, p.host.Hostname)
	if err != nil {
		log.Error().Msgf("Unable to get total rows: %s", err)
	}

	// Remove header and latest row
	currTotal -= 2

	if int(currTotal) < p.host.MaxRows {
		// Nothing to clear
		log.Debug().Msgf("No rows to delete, total rows below maxrows")
		return nil
	}

	deleteCount := currTotal - int64(p.host.MaxRows)

	// Do delete
	err = gsheets.DeleteLastRows(p.svc, p.SheetID, p.host.ID, deleteCount)
	if err != nil {
		return err
	}

	log.Debug().Msgf("Cleared %d rows", deleteCount)
	return nil
}

// makeMissingTargetHeaders will return a slice of strings with all the
// headers which are required for the configured targets.
func (p *Pingsheet) makeMissingTargetHeaders(headers []string) []string {
	metrics := []string{"RTT", "JTT", "SENT", "DROPS"}
	for _, target := range p.host.Targets {
		for _, metric := range metrics {
			if !contains(headers, target.Name+"_"+metric) {
				headers = append(headers, target.Name+"_"+metric)
			}
		}
	}
	return headers
}

// prepHeaders will ensure that all headers are in host sheet ready for results
func (p *Pingsheet) prepHeaders() {
	log.Debug().Msgf("Prepare headers")
	// Get current headers
	cols, err := gsheets.GetHeadersFromSheet(p.svc, p.SheetID, p.host.Hostname)
	if err != nil {
		log.Info().Msgf("Got an error when getting headers: %s", err)
	}

	// Create slice of all headers we need plus current ones
	newHeaders := p.makeMissingTargetHeaders(cols)

	// Update headers on sheet
	err = gsheets.SetHeaders(p.svc, p.SheetID, p.host.Hostname, newHeaders)
	if err != nil {
		log.Warn().Msgf("Got an error when setting headers: %s", err)
	}
	newHeadersCount := len(newHeaders) - len(cols)
	if newHeadersCount > 0 {
		log.Info().Msgf("Added %d new headers", newHeadersCount)
	}

	// Add latest row
	err = gsheets.AddLatestRow(p.svc, p.SheetID, p.host.Hostname)
	if err != nil {
		log.Warn().Msgf("Got an error when setting latest row: %s", err)
	}
}

// pingTargets is what runs the ping tests to each target, gathers the results
// and uploads the results to the host sheet.
func (p *Pingsheet) pingTargets() {
	log.Debug().Msg("Request ping test to all targets")
	results := ping.Run(p.host.Count, p.host.Targets)

	log.Debug().Msgf("Ping returned %d target results", len(results))

	log.Debug().Msg("Get headers for positions")
	cols, err := gsheets.GetHeadersFromSheet(p.svc, p.SheetID, p.host.Hostname)
	if err != nil {
		log.Warn().Msgf("Got an error when getting headers: %s", err)
	}

	if len(cols) == 0 {
		log.Error().Msgf("An error has occurred: worksheet may be missing")
		// Try make a worksheet in case this is the issue
		log.Info().Msgf("Attempt to re-create worksheet to solve issue")
		gsheets.MakeWorksheet(p.svc, p.SheetID, p.host.Hostname)
		return
	}

	log.Debug().Msg("Prepare results for upload")
	newUpload := make([]interface{}, len(cols))
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05")

	// Add timestamp
	newUpload[0] = timestamp

	for _, r := range results {
		// RTT
		rttIdx := colIndex(cols, r.Target.Name+"_RTT")
		newUpload[rttIdx] = r.RTT

		// JTT
		jttIdx := colIndex(cols, r.Target.Name+"_JTT")
		newUpload[jttIdx] = r.JTT

		// SENT
		sentIdx := colIndex(cols, r.Target.Name+"_SENT")
		newUpload[sentIdx] = r.Sent

		// DROPS
		dropsIdx := colIndex(cols, r.Target.Name+"_DROPS")
		newUpload[dropsIdx] = r.Drops
	}

	log.Debug().Msg("Upload results")
	err = gsheets.AddRow(p.svc, p.SheetID, p.host.Hostname, newUpload)
	if err != nil {
		log.Error().Msgf("Error uploading results: %s", err)
	} else {
		log.Debug().Msg("Upload successful")
	}
}

// contains is a handy function to check for string in a slice of strings
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// colIndex is a handy function to return the index of a string element in a
// slice of strings
func colIndex(data []string, col string) int {
	for idx, elem := range data {
		if elem == col {
			return idx
		}
	}
	return -1
}
