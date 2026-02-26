// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/netresearch/ofelia/test"
)

// --- HashPasswordCommand ---

func TestHashPasswordCommand_InvalidCost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cost int
	}{
		{"cost too low", bcrypt.MinCost - 1},
		{"cost too high", bcrypt.MaxCost + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := test.NewTestLogger()
			cmd := &HashPasswordCommand{
				Cost:     tt.cost,
				Logger:   logger,
				LevelVar: &slog.LevelVar{},
			}

			err := cmd.Execute(nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "bcrypt cost must be between")
		})
	}
}

func TestHashPasswordCommand_InvalidLogLevel(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	cmd := &HashPasswordCommand{
		Cost:     12,
		LogLevel: "invalid-level",
		Logger:   logger,
		LevelVar: &slog.LevelVar{},
	}

	// Execute will fail at the prompt, but the log level warning should be logged
	_ = cmd.Execute(nil)

	assert.True(t, handler.HasWarning("Failed to apply log level"))
}
