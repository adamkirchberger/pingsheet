// Copyright (c) 2020, Adam Vakil-Kirchberger
// Licensed under the MIT license

package config

import (
	"strings"

	"github.com/adamkirchberger/pingsheet/pkg/gsheets"
)

type Target struct {
	Name string
}

// BuildTargets is used to get all targets from config sheet ready for config
func BuildTargets(row gsheets.SheetRow) []Target {
	var newTargets []Target
	for col, val := range row {
		if strings.HasPrefix(col, "target_") {
			newTarget := Target{
				val.(string),
			}
			newTargets = append(newTargets, newTarget)
		}
	}
	return newTargets
}
