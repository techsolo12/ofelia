// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package middlewares

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core"
)

const testNameFoo = "foo"

func setupSaveTestContext(t *testing.T) (*core.Context, *TestJob) {
	t.Helper()
	job := &TestJobConfig{
		TestJob: TestJob{
			BareJob: core.BareJob{
				Name: "test-job-save",
			},
		},
		MailConfig: MailConfig{
			SMTPHost:     "test-host",
			SMTPPassword: "secret-password",
			SMTPUser:     "secret-user",
		},
		SlackConfig: SlackConfig{
			SlackWebhook: "secret-url",
		},
	}

	sh := core.NewScheduler(newDiscardLogger())
	e, err := core.NewExecution()
	require.NoError(t, err)

	ctx := core.NewContext(sh, job, e)
	return ctx, &job.TestJob
}

func TestNewSaveEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, NewSave(&SaveConfig{}))
}

func TestSaveRunSuccess(t *testing.T) {
	t.Parallel()
	ctx, job := setupSaveTestContext(t)

	dir := t.TempDir()

	ctx.Start()
	ctx.Stop(nil)

	job.Name = testNameFoo
	ctx.Execution.Date = time.Time{}

	m := NewSave(&SaveConfig{SaveFolder: dir})
	require.NoError(t, m.Run(ctx))

	_, err := os.Stat(filepath.Join(dir, "00010101_000000_"+testNameFoo+".json"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "00010101_000000_"+testNameFoo+".stdout.log"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "00010101_000000_"+testNameFoo+".stderr.log"))
	require.NoError(t, err)
}

func TestSaveRunSuccessOnError(t *testing.T) {
	t.Parallel()
	ctx, job := setupSaveTestContext(t)

	dir := t.TempDir()

	ctx.Start()
	ctx.Stop(nil)

	job.Name = testNameFoo
	ctx.Execution.Date = time.Time{}

	m := NewSave(&SaveConfig{SaveFolder: dir, SaveOnlyOnError: new(true)})
	require.NoError(t, m.Run(ctx))

	_, err := os.Stat(filepath.Join(dir, "00010101_000000_"+testNameFoo+".json"))
	assert.Error(t, err)
}

func TestSaveSensitiveData(t *testing.T) {
	t.Parallel()
	ctx, job := setupSaveTestContext(t)

	dir := t.TempDir()

	ctx.Start()
	ctx.Stop(nil)

	job.Name = "job-with-sensitive-data"
	ctx.Execution.Date = time.Time{}

	m := NewSave(&SaveConfig{SaveFolder: dir})
	require.NoError(t, m.Run(ctx))

	expectedFileName := "00010101_000000_job-with-sensitive-data"
	_, err := os.Stat(filepath.Join(dir, expectedFileName+".json"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, expectedFileName+".stdout.log"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, expectedFileName+".stderr.log"))
	require.NoError(t, err)

	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, files, 3)

	for _, file := range files {
		b, err := os.ReadFile(filepath.Join(dir, file.Name()))
		require.NoError(t, err)

		if strings.Contains(string(b), "secret") {
			t.Logf("Content: %s", string(b))
			t.Errorf("found secret string in %q", file.Name())
		}
	}
}

func TestSaveCreatesSaveFolder(t *testing.T) {
	t.Parallel()
	ctx, job := setupSaveTestContext(t)

	dir := filepath.Join(t.TempDir(), "save-subdir")

	ctx.Start()
	ctx.Stop(nil)

	job.Name = testNameFoo
	ctx.Execution.Date = time.Time{}

	m := NewSave(&SaveConfig{SaveFolder: dir})
	require.NoError(t, m.Run(ctx))

	fi, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
}

func TestSave_ContinueOnStop(t *testing.T) {
	t.Parallel()
	m := &Save{}
	assert.True(t, m.ContinueOnStop(), "Save.ContinueOnStop() should return true")
}

func TestSaveSafeFilename(t *testing.T) {
	t.Parallel()
	ctx, job := setupSaveTestContext(t)

	dir := t.TempDir()

	ctx.Start()
	ctx.Stop(nil)

	job.Name = "foo/bar\\baz"
	ctx.Execution.Date = time.Time{}

	m := NewSave(&SaveConfig{SaveFolder: dir})
	require.NoError(t, m.Run(ctx))

	safe := strings.NewReplacer("/", "_", "\\", "_").Replace(job.Name)
	_, err := os.Stat(filepath.Join(dir, "00010101_000000_"+safe+".stdout.log"))
	require.NoError(t, err)
}

// Phase 8: Additional coverage tests for save.go

func TestSaveConfig_RestoreHistoryEnabled_NilPointer(t *testing.T) {
	t.Parallel()

	cfg := SaveConfig{RestoreHistory: nil, SaveFolder: ""}
	assert.False(t, cfg.RestoreHistoryEnabled(), "nil RestoreHistory with empty SaveFolder should be false")
}

func TestSaveConfig_GetRestoreHistoryMaxAge_ZeroReturnsDefault(t *testing.T) {
	t.Parallel()

	cfg := SaveConfig{RestoreHistoryMaxAge: 0}
	assert.Equal(t, 24*time.Hour, cfg.GetRestoreHistoryMaxAge())
}

func TestSaveConfig_GetRestoreHistoryMaxAge_NegativeReturnsDefault(t *testing.T) {
	t.Parallel()

	cfg := SaveConfig{RestoreHistoryMaxAge: -1 * time.Hour}
	assert.Equal(t, 24*time.Hour, cfg.GetRestoreHistoryMaxAge())
}

func TestSaveRunOnlyOnError_Saves(t *testing.T) {
	t.Parallel()
	ctx, job := setupSaveTestContext(t)

	dir := t.TempDir()

	ctx.Start()
	ctx.Stop(fmt.Errorf("simulated failure"))
	ctx.Execution.Failed = true

	job.Name = "error-job"
	ctx.Execution.Date = time.Time{}

	trueVal := true
	m := NewSave(&SaveConfig{SaveFolder: dir, SaveOnlyOnError: &trueVal})
	require.NoError(t, m.Run(ctx))

	_, err := os.Stat(filepath.Join(dir, "00010101_000000_error-job.json"))
	require.NoError(t, err, "JSON file should exist when save-only-on-error is set and job failed")
}
