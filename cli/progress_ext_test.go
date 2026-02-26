// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/netresearch/ofelia/test"
)

func TestProgressIndicator_Update_Terminal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &buf,
		message:    "Initial message",
		done:       make(chan struct{}),
		isTerminal: true,
		started:    true,
	}

	progress.Update("Updated terminal message")

	assert.Equal(t, "Updated terminal message", progress.message)
	// Terminal mode should write clear sequence to buffer
	assert.Positive(t, buf.Len(), "terminal update should write to buffer")
}

func TestProgressIndicator_Update_NonTerminal(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	progress := &ProgressIndicator{
		logger:     logger,
		writer:     &bytes.Buffer{},
		message:    "Initial message",
		done:       make(chan struct{}),
		isTerminal: false,
		started:    true,
	}

	progress.Update("Non-terminal update")

	assert.Equal(t, "Non-terminal update", progress.message)
	assert.True(t, handler.HasMessage("Non-terminal update"),
		"non-terminal update should log the message")
}

func TestProgressIndicator_Stop_Success_NonTerminal(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	progress := &ProgressIndicator{
		logger:     logger,
		writer:     &bytes.Buffer{},
		message:    "Test operation",
		done:       make(chan struct{}),
		isTerminal: false,
		started:    true,
	}

	progress.Stop(true, "Success message")

	assert.True(t, handler.HasMessage("Success message"))
	assert.False(t, progress.started)
}

func TestProgressIndicator_Stop_Failure_NonTerminal(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	progress := &ProgressIndicator{
		logger:     logger,
		writer:     &bytes.Buffer{},
		message:    "Test operation",
		done:       make(chan struct{}),
		isTerminal: false,
		started:    true,
	}

	progress.Stop(false, "Failure message")

	assert.True(t, handler.HasError("Failure message"))
	assert.False(t, progress.started)
}

func TestProgressIndicator_Stop_Success_Terminal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &buf,
		message:    "Terminal op",
		done:       make(chan struct{}),
		isTerminal: true,
		started:    true,
	}

	progress.Stop(true, "Terminal success")

	output := buf.String()
	assert.Contains(t, output, "Terminal success")
}

func TestProgressIndicator_Stop_Failure_Terminal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &buf,
		message:    "Terminal op",
		done:       make(chan struct{}),
		isTerminal: true,
		started:    true,
	}

	progress.Stop(false, "Terminal failure")

	output := buf.String()
	assert.Contains(t, output, "Terminal failure")
}

func TestProgressIndicator_Stop_NotStarted(t *testing.T) {
	t.Parallel()

	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &bytes.Buffer{},
		message:    "Not started",
		done:       make(chan struct{}),
		isTerminal: false,
		started:    false,
	}

	// Should be a no-op and not panic
	progress.Stop(true, "Should not appear")
	assert.False(t, progress.started)
}

func TestProgressIndicator_DoubleStop(t *testing.T) {
	t.Parallel()

	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &bytes.Buffer{},
		message:    "Double stop",
		done:       make(chan struct{}),
		isTerminal: false,
		started:    true,
	}

	// First stop
	progress.Stop(true, "First")
	// Second stop should be idempotent
	progress.Stop(true, "Second")

	assert.False(t, progress.started)
}

func TestProgressIndicator_Start_AlreadyStarted(t *testing.T) {
	t.Parallel()

	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &bytes.Buffer{},
		message:    "Already started",
		done:       make(chan struct{}),
		isTerminal: false,
		started:    true,
	}

	// Start should be idempotent
	progress.Start()
	assert.True(t, progress.started)
}

func TestProgressReporter_Step_ZeroSteps(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	reporter := &ProgressReporter{
		logger:     logger,
		totalSteps: 0,
		isTerminal: false,
	}

	// Should not panic with zero total steps
	reporter.Step(1, "Step on zero total")

	// currentStep should not be updated when totalSteps is 0
	// (the function returns early after the guard)
}

func TestProgressReporter_Step_NonTerminal(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	reporter := &ProgressReporter{
		logger:     logger,
		totalSteps: 3,
		isTerminal: false,
	}

	reporter.Step(1, "Checking config")
	reporter.Step(2, "Checking Docker")
	reporter.Step(3, "Checking schedules")

	assert.Equal(t, 3, reporter.currentStep)
	assert.True(t, handler.HasMessage("[1/3]"))
	assert.True(t, handler.HasMessage("[2/3]"))
	assert.True(t, handler.HasMessage("[3/3]"))
}

func TestProgressReporter_Complete_NonTerminal(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	reporter := &ProgressReporter{
		logger:     logger,
		totalSteps: 2,
		isTerminal: false,
	}

	reporter.Complete("All done")
	assert.True(t, handler.HasMessage("All done"))
}

func TestProgressReporter_RenderProgressBar_BoundaryValues(t *testing.T) {
	t.Parallel()

	reporter := &ProgressReporter{
		logger:     test.NewTestLogger(),
		totalSteps: 10,
	}

	tests := []struct {
		name    string
		percent float64
	}{
		{"0 percent", 0},
		{"10 percent", 10},
		{"25 percent", 25},
		{"50 percent", 50},
		{"75 percent", 75},
		{"100 percent", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reporter.renderProgressBar(tt.percent)
			assert.True(t, strings.Contains(got, "█") || strings.Contains(got, "░"),
				"progress bar should contain bar characters")
			assert.Contains(t, got, "%", "progress bar should show percentage")
		})
	}
}

func TestProgressIndicator_MessagePreservation_AfterUpdate(t *testing.T) {
	t.Parallel()

	progress := &ProgressIndicator{
		logger:     test.NewTestLogger(),
		writer:     &bytes.Buffer{},
		message:    "original",
		done:       make(chan struct{}),
		isTerminal: false,
		started:    true,
	}

	progress.Update("message-1")
	assert.Equal(t, "message-1", progress.message)

	progress.Update("message-2")
	assert.Equal(t, "message-2", progress.message)

	progress.Update("")
	assert.Empty(t, progress.message)
}
