// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/test"
)

func TestCheckDockerImages_NoRunJobs_SkipMessage(t *testing.T) {
	t.Parallel()

	// Config with local-only jobs (no RunJobs needing Docker images)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	configContent := `[global]
[job-local "test"]
schedule = @daily
command = echo test`

	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	cmd := &DoctorCommand{
		ConfigFile: configPath,
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	cmd.checkDockerImages(report)

	// Should add a skip check about no job-run jobs
	found := false
	for _, check := range report.Checks {
		if check.Category == "Docker Images" && check.Status == statusSkip {
			found = true
			assert.Contains(t, check.Message, "No job-run jobs configured")
		}
	}
	assert.True(t, found, "expected skip check for Docker images when no RunJobs defined")
}

func TestCheckDockerImages_ConfigParseError(t *testing.T) {
	t.Parallel()

	cmd := &DoctorCommand{
		ConfigFile: "/nonexistent/config.ini",
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	cmd.checkDockerImages(report)

	// When config can't be parsed, function returns early with no checks added
	assert.Empty(t, report.Checks, "expected no checks when config file doesn't exist")
}

func TestCheckDocker_NoDockerJobs_SkipMessage(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	configContent := `[global]
[job-local "test"]
schedule = @daily
command = echo test`

	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	cmd := &DoctorCommand{
		ConfigFile: configPath,
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	ok := cmd.checkDocker(report)

	// No Docker jobs -> returns true (not needed, counts as OK) with skip
	assert.True(t, ok)
	found := false
	for _, check := range report.Checks {
		if check.Category == "Docker" && check.Status == statusSkip {
			found = true
			assert.Contains(t, check.Message, "No Docker-based jobs configured")
		}
	}
	assert.True(t, found, "expected skip check for Docker connectivity with no Docker jobs")
}

func TestCheckDocker_ConfigParseError(t *testing.T) {
	t.Parallel()

	cmd := &DoctorCommand{
		ConfigFile: "/nonexistent/config.ini",
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	ok := cmd.checkDocker(report)

	// Config parse error -> returns false
	assert.False(t, ok)
}

func TestCheckDockerImages_DockerInitError(t *testing.T) {
	t.Parallel()

	// Config with run jobs that would need Docker images,
	// but Docker is not available in tests
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	configContent := `[global]
[job-run "test"]
schedule = @daily
image = alpine:latest`

	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	cmd := &DoctorCommand{
		ConfigFile: configPath,
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	cmd.checkDockerImages(report)

	// Docker init will fail in test environment, function returns early
	// No Docker-image-specific checks added (Docker failure already reported elsewhere)
	// Just verify it doesn't panic
}

func TestCheckSchedules_ConfigParseError(t *testing.T) {
	t.Parallel()

	cmd := &DoctorCommand{
		ConfigFile: "/nonexistent/config.ini",
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	cmd.checkSchedules(report)

	// Should add a skip check
	found := false
	for _, check := range report.Checks {
		if check.Category == "Job Schedules" && check.Status == statusSkip {
			found = true
			assert.Contains(t, check.Message, "configuration validation failed")
		}
	}
	assert.True(t, found, "expected skip check for schedules when config fails")
}

func TestCheckWebAuth_AuthEnabled_MissingUsername(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	configContent := `[global]
web-auth-enabled = true
web-password-hash = $2a$12$hash
[job-local "test"]
schedule = @daily
command = echo test`

	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	cmd := &DoctorCommand{
		ConfigFile: configPath,
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	cmd.checkWebAuth(report)

	assert.False(t, report.Healthy, "missing username should mark report unhealthy")
	found := false
	for _, check := range report.Checks {
		if check.Name == "Web Auth Username" {
			found = true
			assert.Equal(t, statusFail, check.Status)
		}
	}
	assert.True(t, found, "expected username failure check")
}

func TestCheckWebAuth_AuthEnabled_MissingPassword(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	configContent := `[global]
web-auth-enabled = true
web-username = admin
[job-local "test"]
schedule = @daily
command = echo test`

	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	cmd := &DoctorCommand{
		ConfigFile: configPath,
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	cmd.checkWebAuth(report)

	assert.False(t, report.Healthy, "missing password hash should mark report unhealthy")
	found := false
	for _, check := range report.Checks {
		if check.Name == "Web Auth Password" {
			found = true
			assert.Equal(t, statusFail, check.Status)
		}
	}
	assert.True(t, found, "expected password failure check")
}

func TestCheckWebAuth_AuthDisabled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	configContent := `[global]
[job-local "test"]
schedule = @daily
command = echo test`

	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	cmd := &DoctorCommand{
		ConfigFile: configPath,
		Logger:     test.NewTestLogger(),
	}

	report := &DoctorReport{Healthy: true, Checks: []CheckResult{}}
	cmd.checkWebAuth(report)

	// When auth is disabled, no auth-related checks should be added
	for _, check := range report.Checks {
		assert.NotContains(t, check.Name, "Web Auth",
			"no web auth checks expected when auth is disabled")
	}
}

func TestOutputJSON_Healthy(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cmd := &DoctorCommand{Logger: logger, JSON: true}

	report := &DoctorReport{
		Healthy: true,
		Checks: []CheckResult{
			{Category: "Test", Name: "Pass Check", Status: statusPass, Message: "All good"},
		},
	}

	err := cmd.outputJSON(report)
	assert.NoError(t, err)
}

func TestOutputJSON_Unhealthy(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cmd := &DoctorCommand{Logger: logger, JSON: true}

	report := &DoctorReport{
		Healthy: false,
		Checks: []CheckResult{
			{Category: "Test", Name: "Fail Check", Status: statusFail, Message: "Problem"},
		},
	}

	err := cmd.outputJSON(report)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestOutputHuman_SkipCount(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	cmd := &DoctorCommand{Logger: logger}

	report := &DoctorReport{
		Healthy: true,
		Checks: []CheckResult{
			{Category: "Configuration", Name: "File Exists", Status: statusPass},
			{Category: "Docker", Name: "Connectivity", Status: statusSkip, Message: "Skipped"},
		},
	}

	err := cmd.outputHuman(report)
	assert.NoError(t, err)
	assert.True(t, handler.HasMessage("skipped"))
}
