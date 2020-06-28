// Copyright (c) 2020, Adam Vakil-Kirchberger
// Licensed under the MIT license

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/adamkirchberger/pingsheet"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	sheet := flag.String("sheet", "", "google sheet ID")
	credentials := flag.String("credentials", "", "path to key file")
	hostname := flag.String("hostname", "", "hostname")
	secret := flag.String("secret", "", "secret")
	showVersion := flag.Bool("version", false, "show version")
	debug := flag.Bool("debug", false, "enable debug")
	flag.Parse()

	if *showVersion {
		fmt.Println("Pingsheet")
		fmt.Println("Author: Adam Vakil-Kirchberger")
		fmt.Printf("Version: %s\nCommit: %s\nDate: %s\n", version, commit, date)
		return
	}

	// Logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// Handle required args
	if *sheet == "" {
		fmt.Println("sheet ID must be supplied!")
		os.Exit(1)
	}
	if *credentials == "" {
		fmt.Println("credentials must be supplied!")
		os.Exit(1)
	}
	if *hostname == "" {
		fmt.Println("hostname must be supplied!")
		os.Exit(1)
	}
	if *secret == "" {
		fmt.Println("secret must be supplied!")
		os.Exit(1)
	}

	p, err := pingsheet.NewPingsheet(
		*sheet,       // SheetID
		*credentials, // Path to GSheets key
		*hostname,    // Hostname
		*secret,      // Secret
	)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}
	p.Run()
}
