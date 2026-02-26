// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// WaitForAdvance (0% → 100%)
// ---------------------------------------------------------------------------

func TestFakeClock_WaitForAdvance(t *testing.T) {
	t.Parallel()
	fc := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	done := make(chan struct{})
	go func() {
		fc.WaitForAdvance()
		close(done)
	}()

	// Advance the clock so WaitForAdvance unblocks
	fc.Advance(1 * time.Second)

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForAdvance did not return in time")
	}
}

// ---------------------------------------------------------------------------
// FakeClock fire paths with multiple tickers, timers, and waiters
// ---------------------------------------------------------------------------

func TestFakeClock_FireTickers_ChannelFull(t *testing.T) {
	t.Parallel()
	fc := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ticker := fc.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Don't consume the ticker channel - it should be full after advance
	fc.Advance(100 * time.Millisecond)

	// Advance again - should hit the default branch (channel already full)
	fc.Advance(100 * time.Millisecond)

	// Now drain
	select {
	case <-ticker.C():
		// got one tick
	default:
		t.Error("expected at least one tick")
	}
}

func TestFakeClock_FireWaiters_ChannelFull(t *testing.T) {
	t.Parallel()
	fc := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ch := fc.After(100 * time.Millisecond)

	fc.Advance(200 * time.Millisecond)

	select {
	case tm := <-ch:
		assert.False(t, tm.IsZero())
	default:
		t.Error("expected waiter to fire")
	}
}

func TestFakeClock_FindEarliestEvent_NoEvents(t *testing.T) {
	t.Parallel()
	fc := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	// No tickers, timers, or waiters - advance should just update now
	fc.Advance(1 * time.Hour)
	assert.Equal(t, time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), fc.Now())
}

func TestFakeClock_StoppedTicker_Skipped(t *testing.T) {
	t.Parallel()
	fc := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ticker := fc.NewTicker(50 * time.Millisecond)
	ticker.Stop()

	// Advancing should not panic even with stopped ticker
	fc.Advance(100 * time.Millisecond)
}

func TestFakeClock_StoppedTimer_Skipped(t *testing.T) {
	t.Parallel()
	fc := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	timer := fc.NewTimer(50 * time.Millisecond)
	timer.Stop()

	// Advancing should not fire stopped timer
	fc.Advance(100 * time.Millisecond)

	select {
	case <-timer.C():
		t.Error("stopped timer should not fire")
	default:
		// expected
	}
}

func TestFakeClock_FiredTimer_Skipped(t *testing.T) {
	t.Parallel()
	fc := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	timer := fc.NewTimer(50 * time.Millisecond)

	// Fire the timer
	fc.Advance(60 * time.Millisecond)
	<-timer.C()

	// Advancing again should not re-fire
	fc.Advance(100 * time.Millisecond)
}

func TestFakeClock_AdvancedChannel_Full(t *testing.T) {
	t.Parallel()
	fc := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	// Fill the advanced channel (capacity 100)
	for range 101 {
		fc.Advance(1 * time.Millisecond)
	}

	// Should not block even though channel is full
	fc.Advance(1 * time.Millisecond)
}
