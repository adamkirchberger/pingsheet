// Copyright (c) 2020, Adam Vakil-Kirchberger
// Licensed under the MIT license

package config

// Result holds test results to a single target
type Result struct {
	Host     Host
	Target   Target
	Sent     int
	Received int
	RTT      float64
	JTT      float64
	Hops     int
}
