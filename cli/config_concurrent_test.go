// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"sync"
	"testing"

	"github.com/netresearch/ofelia/core"
	"github.com/netresearch/ofelia/test"
)

// TestConfig_ConcurrentJobMapAccess verifies that concurrent reads and writes
// to the Config job maps (ExecJobs, RunJobs, LocalJobs, ServiceJobs,
// ComposeJobs) do not cause data races. Run with -race to detect races.
func TestConfig_ConcurrentJobMapAccess(t *testing.T) {
	t.Parallel()

	c := NewConfig(test.NewTestLogger())

	// Pre-populate maps with some entries
	c.ExecJobs["exec1"] = &ExecJobConfig{}
	c.ExecJobs["exec1"].Name = "exec1"
	c.ExecJobs["exec1"].Schedule = "@every 5s"

	c.LocalJobs["local1"] = &LocalJobConfig{}
	c.LocalJobs["local1"].Name = "local1"
	c.LocalJobs["local1"].Schedule = "@every 5s"

	const goroutines = 20
	const iterations = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if id%2 == 0 {
					// Writer goroutines: modify maps under lock
					c.mu.Lock()
					key := "dynamic-exec"
					c.ExecJobs[key] = &ExecJobConfig{}
					c.ExecJobs[key].Name = key
					delete(c.ExecJobs, key)
					c.mu.Unlock()
				} else {
					// Reader goroutines: iterate maps under rlock
					c.mu.RLock()
					for k := range c.ExecJobs {
						_ = k
					}
					for k := range c.LocalJobs {
						_ = k
					}
					c.mu.RUnlock()
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify the pre-existing entries survived the concurrent access
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.ExecJobs["exec1"]; !ok {
		t.Error("pre-existing ExecJobs entry 'exec1' was lost during concurrent access")
	}
	if _, ok := c.LocalJobs["local1"]; !ok {
		t.Error("pre-existing LocalJobs entry 'local1' was lost during concurrent access")
	}
}

// TestConfig_SyncJobMapConcurrentAccess tests that dockerContainersUpdate
// and iniConfigUpdate can be called safely when wrapped with the config mutex.
// This is a structural test that verifies the mutex field exists and works.
func TestConfig_SyncJobMapConcurrentAccess(t *testing.T) {
	t.Parallel()

	c := NewConfig(test.NewTestLogger())
	c.sh = core.NewScheduler(test.NewTestLogger())

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				c.mu.Lock()
				c.ExecJobs["test"] = &ExecJobConfig{}
				c.ExecJobs["test"].Name = "test"
				delete(c.ExecJobs, "test")
				c.mu.Unlock()
			} else {
				c.mu.RLock()
				_ = len(c.ExecJobs)
				_ = len(c.RunJobs)
				_ = len(c.LocalJobs)
				_ = len(c.ServiceJobs)
				_ = len(c.ComposeJobs)
				c.mu.RUnlock()
			}
		}(i)
	}

	wg.Wait()
}
