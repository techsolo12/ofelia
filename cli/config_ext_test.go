// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core"
	"github.com/netresearch/ofelia/test"
)

// testLevelVar is a package-level slog.LevelVar for tests
var testLevelVar slog.LevelVar

func TestBuildFromFile_InvalidGlob(t *testing.T) {
	t.Parallel()

	// A bracket-unmatched pattern causes filepath.Glob to fail
	_, err := BuildFromFile("[invalid", test.NewTestLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid glob pattern")
}

func TestBuildFromFile_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := BuildFromFile("/nonexistent/path/config.ini", test.NewTestLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config file")
}

func TestBuildFromFile_InvalidIniSyntax(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.ini")
	// Write something that can be loaded by ini but has a bad section
	require.NoError(t, os.WriteFile(configPath, []byte("[global\n"), 0o644))

	_, err := BuildFromFile(configPath, test.NewTestLogger())
	require.Error(t, err)
}

func TestBuildFromString_InvalidSyntax(t *testing.T) {
	t.Parallel()

	// Malformed INI that ini.LoadSources can't parse
	_, err := BuildFromString("[global\n", test.NewTestLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load ini from string")
}

func TestResolveConfigFiles_NoMatch(t *testing.T) {
	t.Parallel()

	// Pattern that matches nothing -> returns pattern as literal path
	files, err := resolveConfigFiles("/nonexistent/path/config.ini")
	require.NoError(t, err)
	assert.Equal(t, []string{"/nonexistent/path/config.ini"}, files)
}

func TestResolveConfigFiles_GlobMatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.ini"), []byte("[global]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "b.ini"), []byte("[global]\n"), 0o644))

	pattern := filepath.Join(tmpDir, "*.ini")
	files, err := resolveConfigFiles(pattern)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	// Files should be sorted
	assert.True(t, files[0] < files[1], "files should be sorted: %v", files)
}

func TestLatestChanged_NoChange(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	require.NoError(t, os.WriteFile(configPath, []byte("[global]\n"), 0o644))

	info, err := os.Stat(configPath)
	require.NoError(t, err)

	latest, changed, err := latestChanged([]string{configPath}, info.ModTime())
	require.NoError(t, err)
	assert.False(t, changed, "should not have changed when modtime equals previous")
	assert.Equal(t, info.ModTime(), latest)
}

func TestLatestChanged_Changed(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	require.NoError(t, os.WriteFile(configPath, []byte("[global]\n"), 0o644))

	// Use a time well in the past as previous
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	latest, changed, err := latestChanged([]string{configPath}, past)
	require.NoError(t, err)
	assert.True(t, changed, "should detect change when previous time is in the past")
	assert.True(t, latest.After(past))
}

func TestLatestChanged_MissingFile(t *testing.T) {
	t.Parallel()

	_, _, err := latestChanged([]string{"/nonexistent/file.ini"}, time.Time{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat")
}

func TestIniConfigUpdate_EmptyConfigPath(t *testing.T) {
	t.Parallel()

	config := NewConfig(test.NewTestLogger())
	config.configPath = ""

	err := config.iniConfigUpdate()
	assert.NoError(t, err, "empty configPath should return nil")
}

func TestIniConfigUpdate_FileNotChanged(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")
	configContent := `[global]
[job-local "test"]
schedule = @daily
command = echo test`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	logger := test.NewTestLogger()
	config, err := BuildFromFile(configPath, logger)
	require.NoError(t, err)

	// Set up scheduler so iniConfigUpdate has something to work with
	config.sh = core.NewScheduler(logger)
	config.configPath = configPath
	config.levelVar = &testLevelVar

	// Call iniConfigUpdate - file hasn't changed since build
	err = config.iniConfigUpdate()
	assert.NoError(t, err)
}

func TestIniConfigUpdate_GlobError(t *testing.T) {
	t.Parallel()

	config := NewConfig(test.NewTestLogger())
	config.configPath = "[invalid"

	err := config.iniConfigUpdate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid glob pattern")
}

func TestParseGlobalAndDocker_GlobalDecodeError(t *testing.T) {
	t.Parallel()

	// Test with invalid global section values that cause decode error
	configStr := `[global]
web-token-expiry = not-a-number
[job-local "test"]
schedule = @daily
command = echo test`

	_, err := BuildFromString(configStr, test.NewTestLogger())
	require.Error(t, err)
}

func TestBuildFromFile_MultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create two config files that will be merged
	config1 := filepath.Join(tmpDir, "01-base.ini")
	require.NoError(t, os.WriteFile(config1, []byte(`[global]
[job-local "job1"]
schedule = @daily
command = echo job1
`), 0o644))

	config2 := filepath.Join(tmpDir, "02-extra.ini")
	require.NoError(t, os.WriteFile(config2, []byte(`[job-local "job2"]
schedule = @hourly
command = echo job2
`), 0o644))

	pattern := filepath.Join(tmpDir, "*.ini")
	config, err := BuildFromFile(pattern, test.NewTestLogger())
	require.NoError(t, err)

	// Both jobs should be present
	assert.Len(t, config.LocalJobs, 2, "expected 2 local jobs from merged files")
	assert.Contains(t, config.LocalJobs, "job1")
	assert.Contains(t, config.LocalJobs, "job2")
}

func TestGetKnownKeysForJobType_UnknownType(t *testing.T) {
	t.Parallel()

	keys := getKnownKeysForJobType("unknown-type")
	assert.Nil(t, keys, "unknown job type should return nil keys")
}

func TestGetKnownKeysForJobType_AllTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		jobType   string
		wantEmpty bool
	}{
		{jobExec, false},
		{jobRun, false},
		{jobServiceRun, false},
		{jobLocal, false},
		{jobCompose, false},
	}

	for _, tt := range tests {
		t.Run(tt.jobType, func(t *testing.T) {
			keys := getKnownKeysForJobType(tt.jobType)
			if tt.wantEmpty {
				assert.Empty(t, keys)
			} else {
				assert.NotEmpty(t, keys, "expected known keys for %s", tt.jobType)
			}
		})
	}
}

func TestLogUnknownKeyWarnings_NilResult(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	// Should not panic with nil result
	logUnknownKeyWarnings(logger, "", nil)
}

func TestLogUnknownKeyWarnings_WithWarnings(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()

	res := &parseResult{
		unknownGlobal: []string{"unknownkey"},
		unknownDocker: []string{"badkey"},
		unknownJobs: []jobUnknownKeys{
			{
				JobType:     jobLocal,
				JobName:     "testjob",
				UnknownKeys: []string{"scheduel"},
			},
		},
	}

	logUnknownKeyWarnings(logger, "test.ini", res)

	assert.True(t, handler.HasWarning("unknownkey"), "expected warning for unknown global key")
	assert.True(t, handler.HasWarning("badkey"), "expected warning for unknown docker key")
	assert.True(t, handler.HasWarning("scheduel"), "expected warning for unknown job key")
}

func TestLogJobUnknownKeyWarnings_NoFilename(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()

	unknownJobs := []jobUnknownKeys{
		{
			JobType:     jobLocal,
			JobName:     "myjob",
			UnknownKeys: []string{"comand"},
		},
	}

	logJobUnknownKeyWarnings(logger, unknownJobs, "")

	assert.True(t, handler.HasWarning("comand"), "expected warning for unknown key without filename")
	assert.True(t, handler.HasWarning("did you mean"), "expected suggestion for close match")
}

func TestLogJobUnknownKeyWarnings_NoSuggestion(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()

	unknownJobs := []jobUnknownKeys{
		{
			JobType:     jobLocal,
			JobName:     "myjob",
			UnknownKeys: []string{"zzzzzzz"},
		},
	}

	logJobUnknownKeyWarnings(logger, unknownJobs, "test.ini")

	assert.True(t, handler.HasWarning("zzzzzzz"), "expected warning for totally unknown key")
	assert.True(t, handler.HasWarning("typo"), "expected typo hint for no-suggestion case")
}

func TestBuildFromString_UnknownKeys(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()

	_, err := BuildFromString(`[global]
unknownstuff = value
[job-local "test"]
schedule = @daily
command = echo test
unknownjobkey = value
`, logger)

	// Should not error (unknown keys are warnings, not errors)
	require.NoError(t, err)
	assert.True(t, handler.HasWarning("unknownstuff"), "expected warning for unknown global key")
	assert.True(t, handler.HasWarning("unknownjobkey"), "expected warning for unknown job key")
}
