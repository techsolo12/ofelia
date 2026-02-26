// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/test"
)

// --- ValidateCommand.Execute ---

func TestValidateExecute_InvalidLogLevel(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cmd := &ValidateCommand{
		ConfigFile: "/nonexistent/config.ini",
		LogLevel:   "invalid-level",
		Logger:     logger,
		LevelVar:   &slog.LevelVar{},
	}

	err := cmd.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestValidateExecute_ConfigNotFound(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cmd := &ValidateCommand{
		ConfigFile: "/nonexistent/config.ini",
		Logger:     logger,
		LevelVar:   &slog.LevelVar{},
	}

	err := cmd.Execute(nil)
	require.Error(t, err)
}

func TestValidateExecute_ValidConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	require.NoError(t, os.WriteFile(configPath, []byte(`[global]
log-level = debug
[job-local "test"]
schedule = @daily
command = echo test
`), 0o644))

	logger := test.NewTestLogger()
	cmd := &ValidateCommand{
		ConfigFile: configPath,
		Logger:     logger,
		LevelVar:   &slog.LevelVar{},
	}

	err := cmd.Execute(nil)
	assert.NoError(t, err)
}

func TestValidateExecute_AppliesConfigLogLevel(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	require.NoError(t, os.WriteFile(configPath, []byte(`[global]
log-level = debug
[job-local "test"]
schedule = @daily
command = echo test
`), 0o644))

	logger := test.NewTestLogger()
	lv := &slog.LevelVar{}
	cmd := &ValidateCommand{
		ConfigFile: configPath,
		Logger:     logger,
		LevelVar:   lv,
		// LogLevel is empty, so config log level should be applied
	}

	err := cmd.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, slog.LevelDebug, lv.Level())
}

func TestValidateExecute_CLILogLevelOverridesConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	require.NoError(t, os.WriteFile(configPath, []byte(`[global]
log-level = debug
[job-local "test"]
schedule = @daily
command = echo test
`), 0o644))

	logger := test.NewTestLogger()
	lv := &slog.LevelVar{}
	cmd := &ValidateCommand{
		ConfigFile: configPath,
		LogLevel:   "error",
		Logger:     logger,
		LevelVar:   lv,
	}

	err := cmd.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, slog.LevelError, lv.Level())
}

// --- applyConfigDefaults ---

func TestApplyConfigDefaults_AllJobTypes(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	conf := NewConfig(logger)

	conf.ExecJobs["exec1"] = &ExecJobConfig{}
	conf.RunJobs["run1"] = &RunJobConfig{}
	conf.LocalJobs["local1"] = &LocalJobConfig{}
	conf.ServiceJobs["svc1"] = &RunServiceConfig{}
	conf.ComposeJobs["compose1"] = &ComposeJobConfig{}

	// Should not panic
	applyConfigDefaults(conf)
}
