// Copyright (c) 2020, Adam Vakil-Kirchberger
// Licensed under the MIT license

package ping

import (
	"math"
	"sync"
	"time"

	"github.com/adamkirchberger/pingsheet/pkg/config"

	"github.com/rs/zerolog/log"
	"github.com/sparrc/go-ping"
)

// Result holds a single target ping result
type Result struct {
	Target config.Target
	RTT    float64
	JTT    float64
	Sent   int
	Drops  int
}

// Results holds multiple result objects
//type Results map[string]Result

var wg sync.WaitGroup

// Run will perform pings and return the results
func Run(count int, targets []config.Target) []Result {
	results := make([]Result, 0)
	resultsChan := make(chan Result, 1)

	// Run each test to targets
	for idx, target := range targets {
		wg.Add(1)
		log.Debug().Msgf("Run ping %d: %s", idx+1, target.Name)
		go pingTarget(target, count, 1, 3, resultsChan)
	}

	// Watch channel and append
	go func() {
		for r := range resultsChan {
			results = append(results, r)
			wg.Done()
		}
	}()

	wg.Wait()
	return results
}

// pingTarget will run a single test to a supplied target
func pingTarget(t config.Target, count, interval, timeout int, results chan<- Result) {
	pinger, err := ping.NewPinger(t.Name)
	if err != nil {
		log.Warn().Msgf("Ping had an issue with target `%s`: %s", t.Name, err)
		wg.Done()
		return
	}
	pinger.Count = count
	pinger.Interval = time.Duration(interval) * time.Second
	pinger.Timeout = time.Duration(timeout) * time.Second

	pinger.Run()
	stats := pinger.Statistics()

	// Handle bug with sent packets and drop being negative
	drops := stats.PacketsSent - stats.PacketsRecv
	if drops < 0 {
		drops = 0
	}

	result := Result{
		Target: t,
		RTT:    float64(stats.AvgRtt) / float64(time.Millisecond),
		JTT:    calculateJitter(stats.Rtts),
		Sent:   stats.PacketsSent,
		Drops:  drops,
	}

	// Save results
	results <- result
}

// calculateJitter in milliseconds from supplied slice of RTT's
func calculateJitter(rtts []time.Duration) float64 {
	var diff float64 = 0
	for i := 1; i < len(rtts); i++ {
		diff += math.Abs(float64(rtts[i-1]) - float64(rtts[i]))
	}
	return diff / float64(len(rtts)-1) / float64(time.Millisecond)
}
