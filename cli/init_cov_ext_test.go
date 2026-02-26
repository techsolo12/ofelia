// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ini "gopkg.in/ini.v1"

	"github.com/netresearch/ofelia/test"
)

// --- saveConfig edge cases ---

func TestSaveConfig_WithWebAuth(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	output := filepath.Join(tmpDir, "auth.ini")

	cmd := &InitCommand{
		Output: output,
		Logger: test.NewTestLogger(),
	}

	config := &initConfig{
		Global: &globalConfig{
			EnableWeb:       true,
			WebAddr:         "0.0.0.0:8081",
			LogLevel:        "debug",
			WebAuthEnabled:  true,
			WebUsername:     "admin",
			WebPasswordHash: "$2a$12$abcdef",
		},
		Jobs: []initJobConfig{},
	}

	err := cmd.saveConfig(config)
	require.NoError(t, err)

	cfg, err := ini.Load(output)
	require.NoError(t, err)

	global := cfg.Section("global")
	assert.Equal(t, "true", global.Key("enable-web").String())
	assert.Equal(t, "0.0.0.0:8081", global.Key("web-address").String())
	assert.Equal(t, "true", global.Key("web-auth-enabled").String())
	assert.Equal(t, "admin", global.Key("web-username").String())
	assert.Equal(t, "$2a$12$abcdef", global.Key("web-password-hash").String())
	assert.Equal(t, "debug", global.Key("log-level").String())
}

func TestSaveConfig_NoWebNoLogLevel(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	output := filepath.Join(tmpDir, "minimal.ini")

	cmd := &InitCommand{
		Output: output,
		Logger: test.NewTestLogger(),
	}

	config := &initConfig{
		Global: &globalConfig{
			EnableWeb: false,
			LogLevel:  "",
		},
		Jobs: []initJobConfig{},
	}

	err := cmd.saveConfig(config)
	require.NoError(t, err)

	cfg, err := ini.Load(output)
	require.NoError(t, err)

	global := cfg.Section("global")
	assert.False(t, global.HasKey("enable-web"))
	assert.False(t, global.HasKey("log-level"))
}

func TestSaveConfig_WithMultipleJobTypes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	output := filepath.Join(tmpDir, "multi.ini")

	cmd := &InitCommand{
		Output: output,
		Logger: test.NewTestLogger(),
	}

	config := &initConfig{
		Global: &globalConfig{LogLevel: "info"},
		Jobs: []initJobConfig{
			&runJobConfig{
				JobName:  "run-job",
				Schedule: "@daily",
				Image:    "alpine:latest",
				Command:  "echo run",
				Delete:   true,
			},
			&localJobConfig{
				JobName:  "local-job",
				Schedule: "@hourly",
				Command:  "echo local",
				Dir:      "/tmp",
			},
		},
	}

	err := cmd.saveConfig(config)
	require.NoError(t, err)

	cfg, err := ini.Load(output)
	require.NoError(t, err)

	runSec := cfg.Section(`job-run "run-job"`)
	assert.Equal(t, "@daily", runSec.Key("schedule").String())
	assert.Equal(t, "alpine:latest", runSec.Key("image").String())
	assert.Equal(t, "true", runSec.Key("delete").String())

	localSec := cfg.Section(`job-local "local-job"`)
	assert.Equal(t, "@hourly", localSec.Key("schedule").String())
	assert.Equal(t, "/tmp", localSec.Key("dir").String())
}

func TestSaveConfig_InvalidPath(t *testing.T) {
	t.Parallel()

	cmd := &InitCommand{
		Output: "/proc/nonexistent/path/config.ini", // Invalid path
		Logger: test.NewTestLogger(),
	}

	config := &initConfig{
		Global: &globalConfig{},
		Jobs:   []initJobConfig{},
	}

	err := cmd.saveConfig(config)
	assert.Error(t, err)
}

// --- runJobConfig / localJobConfig edge cases ---

func TestRunJobConfig_WithNetwork(t *testing.T) {
	t.Parallel()

	job := &runJobConfig{
		JobName:  "net-job",
		Schedule: "@daily",
		Image:    "alpine",
		Command:  "echo test",
		Network:  "my-network",
	}

	cfgFile := ini.Empty()
	sec := cfgFile.Section("test")
	err := job.ToINI(sec)
	require.NoError(t, err)

	assert.Equal(t, "my-network", sec.Key("network").String())
}

func TestLocalJobConfig_Type(t *testing.T) {
	t.Parallel()

	job := &localJobConfig{JobName: "test"}
	assert.Equal(t, "job-local", job.Type())
	assert.Equal(t, "test", job.Name())
}

func TestRunJobConfig_Type(t *testing.T) {
	t.Parallel()

	job := &runJobConfig{JobName: "test"}
	assert.Equal(t, "job-run", job.Type())
	assert.Equal(t, "test", job.Name())
}

// --- validateSchedule edge cases ---

func TestValidateSchedule_EveryFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"every 5s", "@every 5s", false},
		{"every 1m30s", "@every 1m30s", false},
		{"every empty", "@every ", false},
		{"every with space", "@every 10m", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateSchedule(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- printNextSteps ---

func TestPrintNextSteps(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	cmd := &InitCommand{
		Output: "/etc/ofelia/config.ini",
		Logger: logger,
	}

	cmd.printNextSteps()

	assert.True(t, handler.HasMessage("Setup complete"))
	assert.True(t, handler.HasMessage("cat /etc/ofelia/config.ini"))
	assert.True(t, handler.HasMessage("ofelia validate"))
	assert.True(t, handler.HasMessage("ofelia daemon"))
}

// --- saveConfig creates directories ---

func TestSaveConfig_CreatesDeepNestedDirs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "d", "config.ini")

	cmd := &InitCommand{
		Output: deepPath,
		Logger: test.NewTestLogger(),
	}

	config := &initConfig{
		Global: &globalConfig{},
		Jobs:   []initJobConfig{},
	}

	err := cmd.saveConfig(config)
	require.NoError(t, err)

	_, err = os.Stat(deepPath)
	assert.NoError(t, err, "config file should exist")
}
