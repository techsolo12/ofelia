// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/netresearch/ofelia/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFromString_EnvFileShadowKeys(t *testing.T) {
	// Create two env files
	dir := t.TempDir()
	f1 := filepath.Join(dir, "one.env")
	require.NoError(t, os.WriteFile(f1, []byte("FOO=from-file-one\nBAR=from-file-one\n"), 0o644))
	f2 := filepath.Join(dir, "two.env")
	require.NoError(t, os.WriteFile(f2, []byte("BAR=from-file-two\nBAZ=from-file-two\n"), 0o644))

	configStr := `
[job-local "test"]
schedule = @daily
command = echo ok
env-file = ` + f1 + `
env-file = ` + f2 + `
environment = BAZ=explicit
`
	cfg, err := BuildFromString(configStr, test.NewTestLogger())
	require.NoError(t, err)

	job, ok := cfg.LocalJobs["test"]
	require.True(t, ok, "job 'test' should exist")
	assert.Equal(t, []string{f1, f2}, job.EnvFile)
	assert.Equal(t, []string{"BAZ=explicit"}, job.Environment)
}

func TestBuildFromString_EnvFrom(t *testing.T) {
	configStr := `
[job-run "backup"]
schedule = @daily
image = alpine
command = backup.sh
env-from = my-app-container
`
	cfg, err := BuildFromString(configStr, test.NewTestLogger())
	require.NoError(t, err)

	job, ok := cfg.RunJobs["backup"]
	require.True(t, ok, "job 'backup' should exist")
	assert.Equal(t, []string{"my-app-container"}, job.EnvFrom)
}

func TestBuildFromString_EnvFileAndEnvFrom(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.env")
	require.NoError(t, os.WriteFile(f, []byte("KEY=value\n"), 0o644))

	configStr := `
[job-exec "test"]
schedule = @daily
container = app
command = echo ok
env-file = ` + f + `
env-from = other-container
environment = EXPLICIT=yes
`
	cfg, err := BuildFromString(configStr, test.NewTestLogger())
	require.NoError(t, err)

	job, ok := cfg.ExecJobs["test"]
	require.True(t, ok, "job 'test' should exist")
	assert.Equal(t, []string{f}, job.EnvFile)
	assert.Equal(t, []string{"other-container"}, job.EnvFrom)
	assert.Equal(t, []string{"EXPLICIT=yes"}, job.Environment)
}
