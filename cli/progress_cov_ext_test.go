// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/netresearch/ofelia/test"
)

// --- animate ---

func TestProgressIndicator_Animate_TerminalMode(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	done := make(chan struct{})
	ticker := time.NewTicker(10 * time.Millisecond) // fast ticker for test

	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &buf,
		message:    "Animating...",
		done:       done,
		isTerminal: true,
		started:    true,
		ticker:     ticker,
	}

	animateDone := make(chan struct{})
	go func() {
		progress.animate()
		close(animateDone)
	}()

	// Let it animate a few frames
	time.Sleep(50 * time.Millisecond)
	close(done)

	// Wait for animate goroutine to finish before reading buffer
	select {
	case <-animateDone:
	case <-time.After(2 * time.Second):
		t.Fatal("animate goroutine did not exit after done was closed")
	}

	output := buf.String()
	assert.NotEmpty(t, output, "animate should have written output")
	assert.Contains(t, output, "Animating...", "output should contain the message")
}

func TestProgressIndicator_Animate_NilTicker(t *testing.T) {
	t.Parallel()

	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &bytes.Buffer{},
		message:    "Test",
		done:       make(chan struct{}),
		isTerminal: true,
		started:    true,
		ticker:     nil, // nil ticker
	}

	done := make(chan struct{})
	go func() {
		progress.animate()
		close(done)
	}()

	select {
	case <-done:
		// Should return immediately with nil ticker
	case <-time.After(2 * time.Second):
		t.Fatal("animate did not return with nil ticker")
	}
}

func TestProgressIndicator_Animate_DoneBeforeTick(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	done := make(chan struct{})
	ticker := time.NewTicker(1 * time.Second) // slow ticker
	close(done)                               // Close immediately

	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &buf,
		message:    "Quick",
		done:       done,
		isTerminal: true,
		started:    true,
		ticker:     ticker,
	}

	doneGoroutine := make(chan struct{})
	go func() {
		progress.animate()
		close(doneGoroutine)
	}()

	select {
	case <-doneGoroutine:
		// Should return immediately since done is already closed
	case <-time.After(2 * time.Second):
		t.Fatal("animate did not return when done was already closed")
	}
}

// --- Start ---

func TestProgressIndicator_Start_Terminal(t *testing.T) {
	t.Parallel()

	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &bytes.Buffer{},
		message:    "Starting...",
		done:       make(chan struct{}),
		isTerminal: true,
		started:    false,
	}

	progress.Start()
	assert.True(t, progress.started)
	assert.NotNil(t, progress.ticker)

	// Clean up
	progress.Stop(true, "Done")
}

func TestProgressIndicator_Start_NonTerminal(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	progress := &ProgressIndicator{
		logger:     logger,
		writer:     &bytes.Buffer{},
		message:    "Starting non-terminal",
		done:       make(chan struct{}),
		isTerminal: false,
		started:    false,
	}

	progress.Start()
	assert.True(t, progress.started)
	assert.True(t, handler.HasMessage("Starting non-terminal"))
}

// --- Step ---

func TestProgressReporter_Step_Terminal(t *testing.T) {
	t.Parallel()

	// We can't fully test terminal output without a real terminal,
	// but we can verify the logic with isTerminal set
	reporter := &ProgressReporter{
		logger:     test.NewTestLogger(),
		totalSteps: 3,
		isTerminal: true, // Simulates terminal
	}

	// Step should not panic even in terminal mode with redirected stdout
	reporter.Step(1, "First step")
	reporter.Step(2, "Second step")
	reporter.Step(3, "Final step") // Last step

	assert.Equal(t, 3, reporter.currentStep)
}

// --- Complete ---

func TestProgressReporter_Complete_Terminal(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	reporter := &ProgressReporter{
		logger:     logger,
		totalSteps: 2,
		isTerminal: true,
	}

	reporter.Complete("All done terminal")
	assert.True(t, handler.HasMessage("All done terminal"))
}

func TestProgressReporter_Complete_NonTerminal_Coverage(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	reporter := &ProgressReporter{
		logger:     logger,
		totalSteps: 5,
		isTerminal: false,
	}

	reporter.Complete("All five done")
	assert.True(t, handler.HasMessage("All five done"))
}
