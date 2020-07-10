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

// CheckPingPermissions tests if root is required and present
//
// Returns true if ping needs to be privileged
func CheckPingPermissions() (bool, error) {
	// Try unprivileged
	if ok := tryPing("127.0.0.1", false); ok {
		log.Debug().Msgf("Ping unprivileged test passed")
		return false, nil
	}
	log.Info().Msgf("Elevate privileges for raw socket access")

	// Try privileged
	if ok := tryPing("127.0.0.1", true); ok {
		log.Info().Msgf("Privileged access successful")
		return true, nil
	}
	log.Warn().Msgf("Privileged access unsuccessful")

	return true, errors.New("permission denied, need root")
}

// tryPing will send a single ping and return true if successful
func tryPing(target string, privileged bool) bool {
	pinger, err := ping.NewPinger(target)
	pinger.Count = 1
	pinger.Timeout = time.Duration(1) * time.Second
	pinger.SetPrivileged(privileged)
	if err != nil {
		log.Warn().Msgf("Ping error: %s", err)
		return false
	}

	pinger.Run()
	if pinger.Statistics().PacketsRecv == 1 {
		return true
	}
	return false
}

// calculateJitter in milliseconds from supplied slice of RTT's
func calculateJitter(rtts []time.Duration) float64 {
	var diff float64 = 0
	for i := 1; i < len(rtts); i++ {
		diff += math.Abs(float64(rtts[i-1]) - float64(rtts[i]))
	}
	return diff / float64(len(rtts)-1) / float64(time.Millisecond)
}
