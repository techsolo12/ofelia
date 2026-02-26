// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// RealClock method coverage
// ---------------------------------------------------------------------------

func TestRealClockUnit_Now(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	before := time.Now()
	now := clock.Now()
	after := time.Now()

	if now.Before(before) || now.After(after) {
		t.Errorf("Now() = %v, want between %v and %v", now, before, after)
	}
}

func TestRealClockUnit_NewTicker(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	ticker := clock.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	select {
	case ts := <-ticker.C():
		if ts.IsZero() {
			t.Error("ticker sent zero time")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ticker did not fire within 100ms")
	}
}

func TestRealClockUnit_After(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	ch := clock.After(10 * time.Millisecond)

	select {
	case ts := <-ch:
		if ts.IsZero() {
			t.Error("After sent zero time")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("After did not fire within 100ms")
	}
}

func TestRealClockUnit_Sleep(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	start := time.Now()
	clock.Sleep(10 * time.Millisecond)
	elapsed := time.Since(start)

	if elapsed < 10*time.Millisecond {
		t.Errorf("Sleep returned too early: %v", elapsed)
	}
}

func TestRealClockUnit_NewTimer(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	timer := clock.NewTimer(10 * time.Millisecond)

	select {
	case ts := <-timer.C():
		if ts.IsZero() {
			t.Error("timer sent zero time")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timer did not fire within 100ms")
	}

	// Stop after firing should return false (already expired)
	if timer.Stop() {
		t.Error("Stop() should return false for already-fired timer")
	}
}

func TestRealClockUnit_TimerReset(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	timer := clock.NewTimer(100 * time.Millisecond)

	// Reset to shorter duration
	wasActive := timer.Reset(10 * time.Millisecond)
	if !wasActive {
		t.Error("Reset() should return true for active timer")
	}

	select {
	case <-timer.C():
		// good - fired after reset
	case <-time.After(50 * time.Millisecond):
		t.Fatal("timer did not fire after reset")
	}
}

func TestRealClockUnit_TimerStop(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	timer := clock.NewTimer(100 * time.Millisecond)

	wasActive := timer.Stop()
	if !wasActive {
		t.Error("Stop() should return true for active timer")
	}

	// Ensure it doesn't fire
	select {
	case <-timer.C():
		t.Fatal("stopped timer should not fire")
	case <-time.After(150 * time.Millisecond):
		// good - did not fire
	}
}

// ---------------------------------------------------------------------------
// RealTicker Stop coverage
// ---------------------------------------------------------------------------

func TestRealClockUnit_TickerStop(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	ticker := clock.NewTicker(5 * time.Millisecond)

	// Consume one tick
	select {
	case <-ticker.C():
	case <-time.After(50 * time.Millisecond):
		t.Fatal("ticker did not fire")
	}

	ticker.Stop()

	// After stop, channel should not deliver more ticks
	select {
	case <-ticker.C():
		// Might get one buffered tick, that's OK
	case <-time.After(50 * time.Millisecond):
		// good - stopped
	}
}

// ---------------------------------------------------------------------------
// FakeClock additional edge cases
// ---------------------------------------------------------------------------

func TestFakeClockUnit_Set(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewFakeClock(start)

	target := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	clock.Set(target)

	if !clock.Now().Equal(target) {
		t.Errorf("expected %v, got %v", target, clock.Now())
	}
}

func TestFakeClockUnit_MultipleTimers(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewFakeClock(start)

	t1 := clock.NewTimer(50 * time.Millisecond)
	t2 := clock.NewTimer(100 * time.Millisecond)

	// Advance past first timer only
	clock.Advance(75 * time.Millisecond)

	select {
	case <-t1.C():
		// expected
	default:
		t.Error("timer1 should have fired at 50ms")
	}

	select {
	case <-t2.C():
		t.Error("timer2 should not have fired yet")
	default:
		// expected
	}

	// Advance past second timer
	clock.Advance(50 * time.Millisecond)

	select {
	case <-t2.C():
		// expected
	default:
		t.Error("timer2 should have fired at 100ms")
	}
}

func TestFakeClockUnit_SleepZero(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Now())

	// Sleep(0) should return immediately without blocking
	done := make(chan struct{})
	go func() {
		clock.Sleep(0)
		close(done)
	}()

	select {
	case <-done:
		// good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Sleep(0) should return immediately")
	}
}

func TestFakeClockUnit_NegativeSleep(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Now())

	done := make(chan struct{})
	go func() {
		clock.Sleep(-1 * time.Second)
		close(done)
	}()

	select {
	case <-done:
		// good - negative sleep returns immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Sleep(-1s) should return immediately")
	}
}

func TestFakeClockUnit_AfterNegative(t *testing.T) {
	t.Parallel()

	clock := NewFakeClock(time.Now())

	ch := clock.After(-1 * time.Second)
	select {
	case <-ch:
		// good - fires immediately for non-positive duration
	case <-time.After(100 * time.Millisecond):
		t.Fatal("After(-1s) should fire immediately")
	}
}

// ---------------------------------------------------------------------------
// CronClock
// ---------------------------------------------------------------------------

func TestCronClockUnit_NewTimerReturnsCronTimer(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cc := NewCronClock(start)

	timer := cc.NewTimer(50 * time.Millisecond)
	if timer == nil {
		t.Fatal("NewTimer returned nil")
	}

	cc.Advance(50 * time.Millisecond)
	select {
	case <-timer.C():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("CronClock timer did not fire")
	}
}
