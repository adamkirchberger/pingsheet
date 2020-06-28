// Copyright (c) 2020, Adam Vakil-Kirchberger
// Licensed under the MIT license

package config

import (
	"errors"
	"strconv"
	"time"

	"github.com/adamkirchberger/pingsheet/pkg/gsheets"
)

// Host model
type Host struct {
	ID       int64
	Hostname string
	Secret   string
	Interval time.Duration
	Count    int
	MaxRows  int
	Targets  []Target
}

// NewHost will create a new Host type from a Gsheet row
func NewHost(row gsheets.SheetRow) (*Host, error) {
	newH := Host{}

	if val, ok := row["hostname"]; ok {
		newH.Hostname = val.(string)
	} else {
		return nil, errors.New("Host is missing `HOSTNAME`")
	}

	if val, ok := row["secret"]; ok {
		newH.Secret = val.(string)
	} else {
		return nil, errors.New("Host is missing `SECRET`")
	}

	if val, ok := row["interval"]; ok {
		valInt, err := strconv.Atoi(val.(string))
		if err != nil {
			return nil, errors.New("`INTERVAL` must be number")
		}
		newH.Interval = time.Duration(valInt) * time.Second
	} else {
		return nil, errors.New("Host is missing `INTERVAL`")
	}

	if val, ok := row["count"]; ok {
		valInt, err := strconv.Atoi(val.(string))
		if err != nil {
			return nil, errors.New("`COUNT` must be number")
		}
		newH.Count = valInt
	} else {
		return nil, errors.New("Host is missing `COUNT`")
	}

	if val, ok := row["maxrows"]; ok {
		valInt, err := strconv.Atoi(val.(string))
		if err != nil {
			return nil, errors.New("`MAXROWS` must be number")
		}
		newH.MaxRows = valInt
	} else {
		return nil, errors.New("Host is missing `MAXROWS`")
	}

	newH.Targets = BuildTargets(row)

	return &newH, nil
}
