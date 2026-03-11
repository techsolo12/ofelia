// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"sync"
	"testing"
)

func TestDaemonCommand_CloseDoneMultipleTimes(t *testing.T) {
	t.Parallel()

	c := &DaemonCommand{
		done: make(chan struct{}),
	}

	// Call closeDone concurrently from multiple goroutines.
	// Without sync.Once protection, this panics with "close of closed channel".
	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			c.closeDone()
		}()
	}
	wg.Wait()

	// Verify the channel is actually closed (receive should return immediately)
	select {
	case <-c.done:
		// expected: channel is closed
	default:
		t.Error("done channel is not closed after closeDone() calls")
	}
}

func TestDaemonCommand_CloseDoneSequential(t *testing.T) {
	t.Parallel()

	c := &DaemonCommand{
		done: make(chan struct{}),
	}

	// Calling closeDone multiple times sequentially must not panic
	c.closeDone()
	c.closeDone()
	c.closeDone()

	select {
	case <-c.done:
		// expected
	default:
		t.Error("done channel is not closed after sequential closeDone() calls")
	}
}
