// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/test"
)

// ---------------------------------------------------------------------------
// ComposeJob.Run (0% → covers the Run function)
// ---------------------------------------------------------------------------

func TestComposeJob_Run_NoDockerCompose(t *testing.T) {
	t.Parallel()
	job := NewComposeJob()
	job.BareJob = BareJob{Name: "compose-test", Command: "echo hello"}
	job.File = "compose.yml"
	job.Service = "test-service"

	logger := test.NewTestLogger()
	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	require.NoError(t, err)
	exec.Start()
	ctx := NewContext(scheduler, job, exec)

	// Run will likely fail because docker compose binary may not exist
	// or the compose file doesn't exist - but it exercises the Run code path
	err = job.Run(ctx)
	// We expect either a path-not-found error (docker not in PATH)
	// or the command to fail
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// ComposeJob.buildCommand edge cases
// ---------------------------------------------------------------------------

func TestComposeJob_buildCommand_InvalidFilePath(t *testing.T) {
	t.Parallel()
	job := NewComposeJob()
	job.BareJob = BareJob{Name: "compose-invalid", Command: "echo hello"}
	job.File = "../../etc/passwd" // will fail path validation
	job.Service = "test-service"

	logger := test.NewTestLogger()
	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	require.NoError(t, err)
	exec.Start()
	ctx := NewContext(scheduler, job, exec)

	err = job.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compose")
}

func TestComposeJob_buildCommand_InvalidServiceName(t *testing.T) {
	t.Parallel()
	job := NewComposeJob()
	job.BareJob = BareJob{Name: "compose-bad-svc", Command: "echo hello"}
	job.File = "compose.yml"
	job.Service = "test;service" // semicolons are invalid in service name

	logger := test.NewTestLogger()
	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	require.NoError(t, err)
	exec.Start()
	ctx := NewContext(scheduler, job, exec)

	err = job.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compose")
}

func TestComposeJob_buildCommand_ExecMode(t *testing.T) {
	t.Parallel()
	job := NewComposeJob()
	job.BareJob = BareJob{Name: "compose-exec", Command: "ls -la"}
	job.File = "compose.yml"
	job.Service = "test-service"
	job.Exec = true

	logger := test.NewTestLogger()
	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	require.NoError(t, err)
	exec.Start()
	ctx := NewContext(scheduler, job, exec)

	// Will fail but exercises the exec branch
	err = job.Run(ctx)
	assert.Error(t, err)
}
