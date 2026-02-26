// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/test"
)

// ---------------------------------------------------------------------------
// Scheduler.Run (the no-context version) - 0%
// ---------------------------------------------------------------------------

func TestScheduler_Run_Empty(t *testing.T) {
	t.Parallel()
	logger := test.NewTestLogger()
	s := NewScheduler(logger)

	// No jobs added - Start will still work but no jobs fire
	err := s.Start()
	require.NoError(t, err)
	defer s.StopAndWait()
}

// ---------------------------------------------------------------------------
// jobWrapper.Run (the no-context version) - 0%
// ---------------------------------------------------------------------------

func TestJobWrapper_Run_CallsRunWithCtx(t *testing.T) {
	t.Parallel()
	logger := test.NewTestLogger()
	s := NewScheduler(logger)

	called := false
	j := &testJob{
		name:     "wrapper-test",
		schedule: "@triggered",
		command:  "echo test",
		runFunc: func(_ *Context) error {
			called = true
			return nil
		},
	}

	err := s.AddJob(j)
	require.NoError(t, err)

	err = s.Start()
	require.NoError(t, err)
	defer s.StopAndWait()

	// Directly call Run on the wrapper
	w := &jobWrapper{s: s, j: j}
	w.Run()

	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// EntryByName with nil cron - 0% (the nil-cron branch)
// ---------------------------------------------------------------------------

func TestScheduler_EntryByName_NilCron(t *testing.T) {
	t.Parallel()
	s := &Scheduler{} // cron is nil

	entry := s.EntryByName("nonexistent")
	assert.Empty(t, entry.Name)
}

// ---------------------------------------------------------------------------
// IsRunning with nil cron - 0% (the nil-cron branch)
// ---------------------------------------------------------------------------

func TestScheduler_IsRunning_NilCron(t *testing.T) {
	t.Parallel()
	s := &Scheduler{} // cron is nil

	assert.False(t, s.IsRunning())
}

// ---------------------------------------------------------------------------
// RunJob on disabled job
// ---------------------------------------------------------------------------

func TestScheduler_RunJob_DisabledJob(t *testing.T) {
	t.Parallel()
	logger := test.NewTestLogger()
	s := NewScheduler(logger)

	j := &testJob{
		name:     "disable-me",
		schedule: "@triggered",
		command:  "echo test",
		runFunc: func(_ *Context) error {
			return nil
		},
	}

	err := s.AddJob(j)
	require.NoError(t, err)

	err = s.Start()
	require.NoError(t, err)
	defer s.StopAndWait()

	// Disable the job
	err = s.DisableJob("disable-me")
	require.NoError(t, err)

	// Attempt to run disabled job
	err = s.RunJob(context.Background(), "disable-me")
	assert.ErrorIs(t, err, ErrJobNotFound)
}

// ---------------------------------------------------------------------------
// RunJob on nonexistent job
// ---------------------------------------------------------------------------

func TestScheduler_RunJob_NonexistentJob(t *testing.T) {
	t.Parallel()
	logger := test.NewTestLogger()
	s := NewScheduler(logger)

	err := s.Start()
	require.NoError(t, err)
	defer s.StopAndWait()

	err = s.RunJob(context.Background(), "does-not-exist")
	assert.ErrorIs(t, err, ErrJobNotFound)
}

// testJob is a minimal Job implementation for unit tests.
type testJob struct {
	BareJob
	name     string
	schedule string
	command  string
	runFunc  func(*Context) error
}

func (j *testJob) GetName() string     { return j.name }
func (j *testJob) GetSchedule() string { return j.schedule }
func (j *testJob) GetCommand() string  { return j.command }
func (j *testJob) Run(ctx *Context) error {
	if j.runFunc != nil {
		return j.runFunc(ctx)
	}
	return nil
}
func (j *testJob) ShouldRunOnStartup() bool { return false }
