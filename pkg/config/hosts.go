// Copyright (c) 2020, Adam Vakil-Kirchberger
// Licensed under the MIT license

package config

import (
	"errors"

	"github.com/adamkirchberger/pingsheet/pkg/gsheets"

	"github.com/rs/zerolog/log"
)

// Hosts is where we hold all host configs
type Hosts []Host

// BuildHosts will take a map from Gsheets, build hosts config and return total
// built hosts. When function is re-run it will reset and rebuild hosts.
func (h *Hosts) BuildHosts(cfgMap *gsheets.SheetRows) error {
	h.resetHosts()
	for rowNum, row := range *cfgMap {
		newHost, err := NewHost(row)
		if err != nil {
			log.Error().Msgf("Error building host on row %d: %s", rowNum+2, err)
		} else {
			h.addHost(newHost)
		}
	}
	log.Debug().Msgf("Built %d hosts", h.getTotal())
	return nil
}

// resetHosts will reset hosts
func (h *Hosts) resetHosts() {
	*h = make([]Host, 0)
}

// addHost will add a single host to config
func (h *Hosts) addHost(newHost *Host) error {
	if newHost == nil {
		return errors.New("Cannot add `nil` Host to Hosts")
	}

	*h = append(*h, *newHost)
	return nil

}

// getTotal is a quick method to query total hosts
func (h *Hosts) getTotal() int {
	return len(*h)
}

// Authenticate will look at all configured hosts for a Hostname+Secret match
func (h *Hosts) Authenticate(hostname, secret string) *Host {
	for _, host := range *h {
		if host.Hostname == hostname && host.Secret == secret {
			return &host
		}
	}
	return nil
}
