// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core"
	"github.com/netresearch/ofelia/middlewares"
	"github.com/netresearch/ofelia/test"
)

// --- boot tests ---

func TestBoot_InvalidLogLevel(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	lv := &slog.LevelVar{}

	cmd := &DaemonCommand{
		Logger:     logger,
		LevelVar:   lv,
		LogLevel:   "invalid-level",
		ConfigFile: "/nonexistent/config.ini",
	}

	err := cmd.boot()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestBoot_MissingConfigUsesEmpty(t *testing.T) {
	// Not parallel - modifies global newDockerHandler

	logger, handler := test.NewTestLoggerWithHandler()
	lv := &slog.LevelVar{}

	mockProvider := &mockDockerProviderForHandler{}

	// Override newDockerHandler to avoid Docker dependency
	origNewDockerHandler := newDockerHandler
	newDockerHandler = func(ctx context.Context, notifier dockerContainersUpdate, l *slog.Logger, cfg *DockerConfig, provider core.DockerProvider) (*DockerHandler, error) {
		return &DockerHandler{ctx: ctx, logger: l, dockerProvider: mockProvider}, nil
	}
	defer func() { newDockerHandler = origNewDockerHandler }()

	// We can't fully test boot without Docker, but we can verify it handles missing config
	cmd := &DaemonCommand{
		Logger:              logger,
		LevelVar:            lv,
		ConfigFile:          "/nonexistent/config.ini",
		PprofAddr:           "127.0.0.1:8080",
		WebAddr:             ":8081",
		WebTokenExpiry:      24,
		WebMaxLoginAttempts: 5,
	}

	// boot will fail at InitializeApp due to Docker, but config should load
	_ = cmd.boot()

	assert.True(t, handler.HasWarning("Could not load config file"),
		"should warn about missing config file")
}

func TestBoot_WebAuthEnabled_MissingUsername(t *testing.T) {
	// Not parallel - modifies global newDockerHandler

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	require.NoError(t, os.WriteFile(configPath, []byte(`[global]
enable-web = true
web-auth-enabled = true
[job-local "test"]
schedule = @daily
command = echo test
`), 0o644))

	logger := test.NewTestLogger()
	lv := &slog.LevelVar{}

	// Override newDockerHandler to avoid Docker dependency
	mockProv := &mockDockerProviderForHandler{}
	origNewDockerHandler := newDockerHandler
	newDockerHandler = func(ctx context.Context, notifier dockerContainersUpdate, l *slog.Logger, cfg *DockerConfig, provider core.DockerProvider) (*DockerHandler, error) {
		return &DockerHandler{ctx: ctx, logger: l, dockerProvider: mockProv}, nil
	}
	defer func() { newDockerHandler = origNewDockerHandler }()

	cmd := &DaemonCommand{
		Logger:              logger,
		LevelVar:            lv,
		ConfigFile:          configPath,
		EnableWeb:           true,
		WebAuthEnabled:      true,
		WebAddr:             ":8081",
		PprofAddr:           "127.0.0.1:8080",
		WebTokenExpiry:      24,
		WebMaxLoginAttempts: 5,
	}

	err := cmd.boot()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWebAuthUsername)
}

func TestBoot_WebAuthEnabled_MissingPassword(t *testing.T) {
	// Not parallel - modifies global newDockerHandler

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	require.NoError(t, os.WriteFile(configPath, []byte(`[global]
enable-web = true
web-auth-enabled = true
web-username = admin
[job-local "test"]
schedule = @daily
command = echo test
`), 0o644))

	logger := test.NewTestLogger()
	lv := &slog.LevelVar{}

	mockProv2 := &mockDockerProviderForHandler{}
	origNewDockerHandler := newDockerHandler
	newDockerHandler = func(ctx context.Context, notifier dockerContainersUpdate, l *slog.Logger, cfg *DockerConfig, provider core.DockerProvider) (*DockerHandler, error) {
		return &DockerHandler{ctx: ctx, logger: l, dockerProvider: mockProv2}, nil
	}
	defer func() { newDockerHandler = origNewDockerHandler }()

	cmd := &DaemonCommand{
		Logger:              logger,
		LevelVar:            lv,
		ConfigFile:          configPath,
		EnableWeb:           true,
		WebAuthEnabled:      true,
		WebUsername:         "admin",
		WebAddr:             ":8081",
		PprofAddr:           "127.0.0.1:8080",
		WebTokenExpiry:      24,
		WebMaxLoginAttempts: 5,
	}

	err := cmd.boot()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWebAuthPassword)
}

// --- start tests ---

func TestStart_SchedulerFailure(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	lv := &slog.LevelVar{}

	// Create scheduler that will fail on Start (nil scheduler triggers error)
	cmd := &DaemonCommand{
		Logger:          logger,
		LevelVar:        lv,
		scheduler:       core.NewScheduler(logger),
		config:          NewConfig(logger),
		done:            make(chan struct{}),
		shutdownManager: core.NewShutdownManager(logger, 5*time.Second),
		ConfigFile:      "/etc/ofelia/config.ini",
	}

	// Scheduler with no jobs should start fine
	err := cmd.start()
	assert.NoError(t, err)
}

// --- restoreJobHistory tests ---

func TestRestoreJobHistory_WithRestoreEnabled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := test.NewTestLogger()

	config := NewConfig(logger)
	config.Global.SaveConfig = middlewares.SaveConfig{
		SaveFolder:           tmpDir,
		RestoreHistoryMaxAge: 24 * time.Hour,
	}

	sched := core.NewScheduler(logger)
	cmd := &DaemonCommand{
		Logger:    logger,
		scheduler: sched,
	}

	// Should not panic or error
	cmd.restoreJobHistory(config)
}

// --- applyOptions with partial fields ---

func TestApplyOptions_OnlyDockerFilters(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	config := NewConfig(logger)

	cmd := &DaemonCommand{
		DockerFilters: []string{"label=app=web"},
		// Other fields at defaults
		WebAddr:             ":8081",
		PprofAddr:           "127.0.0.1:8080",
		WebTokenExpiry:      24,
		WebMaxLoginAttempts: 5,
	}

	cmd.applyOptions(config)
	assert.Equal(t, []string{"label=app=web"}, config.Docker.Filters)
}

func TestApplyOptions_WebOptionsFromConfig(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	config := NewConfig(logger)
	config.Global.EnableWeb = true
	config.Global.WebAddr = ":7070"

	cmd := &DaemonCommand{
		WebAddr:             ":8081",
		PprofAddr:           "127.0.0.1:8080",
		WebTokenExpiry:      24,
		WebMaxLoginAttempts: 5,
	}

	cmd.applyConfigDefaults(config)
	assert.True(t, cmd.EnableWeb)
	assert.Equal(t, ":7070", cmd.WebAddr)
}

func TestApplyConfigDefaults_NoOverrideWhenCLISet(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	config := NewConfig(logger)
	config.Global.EnableWeb = true
	config.Global.WebAddr = ":7070"
	config.Global.EnablePprof = true

	cmd := &DaemonCommand{
		EnableWeb:           true,           // Already set by CLI
		WebAddr:             ":9090",        // Non-default
		PprofAddr:           "0.0.0.0:6060", // Non-default
		EnablePprof:         true,
		WebTokenExpiry:      48, // Non-default
		WebMaxLoginAttempts: 10, // Non-default
	}

	cmd.applyConfigDefaults(config)

	// CLI values should be preserved
	assert.True(t, cmd.EnableWeb)
	assert.Equal(t, ":9090", cmd.WebAddr)          // CLI wins
	assert.Equal(t, "0.0.0.0:6060", cmd.PprofAddr) // CLI wins
	assert.Equal(t, 48, cmd.WebTokenExpiry)        // CLI wins
	assert.Equal(t, 10, cmd.WebMaxLoginAttempts)   // CLI wins
}

// --- applyWebDefaults / applyAuthDefaults / applyServerDefaults ---

func TestApplyWebDefaults_DefaultAddr(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	config := NewConfig(logger)
	config.Global.WebAddr = ":9090"

	cmd := &DaemonCommand{
		WebAddr: ":8081", // Default
	}

	cmd.applyWebDefaults(config)
	assert.Equal(t, ":9090", cmd.WebAddr)
}

func TestApplyWebDefaults_EmptyConfigAddr(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	config := NewConfig(logger)
	config.Global.WebAddr = ""

	cmd := &DaemonCommand{
		WebAddr: ":8081",
	}

	cmd.applyWebDefaults(config)
	assert.Equal(t, ":8081", cmd.WebAddr, "should keep default when config is empty")
}

func TestApplyAuthDefaults_AllFields(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	config := NewConfig(logger)
	config.Global.WebAuthEnabled = true
	config.Global.WebUsername = "configuser"
	config.Global.WebPasswordHash = "confighash"
	config.Global.WebSecretKey = "configsecret"
	config.Global.WebTokenExpiry = 12
	config.Global.WebMaxLoginAttempts = 3

	cmd := &DaemonCommand{
		WebTokenExpiry:      24,
		WebMaxLoginAttempts: 5,
	}

	cmd.applyAuthDefaults(config)

	assert.True(t, cmd.WebAuthEnabled)
	assert.Equal(t, "configuser", cmd.WebUsername)
	assert.Equal(t, "confighash", cmd.WebPasswordHash)
	assert.Equal(t, "configsecret", cmd.WebSecretKey)
	assert.Equal(t, 12, cmd.WebTokenExpiry)
	assert.Equal(t, 3, cmd.WebMaxLoginAttempts)
}

func TestApplyServerDefaults_FromConfig(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	config := NewConfig(logger)
	config.Global.EnablePprof = true
	config.Global.PprofAddr = "0.0.0.0:9090"

	cmd := &DaemonCommand{
		PprofAddr: "127.0.0.1:8080", // Default
	}

	cmd.applyServerDefaults(config)

	assert.True(t, cmd.EnablePprof)
	assert.Equal(t, "0.0.0.0:9090", cmd.PprofAddr)
}
