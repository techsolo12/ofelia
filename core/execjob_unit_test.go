// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/netresearch/ofelia/core/adapters/mock"
	"github.com/netresearch/ofelia/core/domain"
	"github.com/netresearch/ofelia/test"
)

// testExecJobKit holds mock objects for ExecJob unit tests.
type testExecJobKit struct {
	job     *ExecJob
	client  *mock.DockerClient
	exec    *mock.ExecService
	handler *test.Handler
}

// newTestExecJobKit creates an ExecJob wired to a mock DockerClient.
func newTestExecJobKit(t *testing.T) *testExecJobKit {
	t.Helper()
	mc := mock.NewDockerClient()
	logger, handler := test.NewTestLoggerWithHandler()
	provider := NewSDKDockerProviderFromClient(mc, logger, nil)
	job := NewExecJob(provider)
	job.BareJob = BareJob{Name: "unit-exec", Command: "echo hello"}
	job.Container = "test-container"
	return &testExecJobKit{
		job:     job,
		client:  mc,
		exec:    mc.Exec().(*mock.ExecService),
		handler: handler,
	}
}

// newExecJobContext creates a Context suitable for ExecJob unit tests.
func newExecJobContext(t *testing.T, job *ExecJob) *Context {
	t.Helper()
	logger := test.NewTestLogger()
	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	if err != nil {
		t.Fatalf("NewExecution: %v", err)
	}
	exec.Start()
	return NewContext(scheduler, job, exec)
}

// ---------------------------------------------------------------------------
// Run() exit code handling
// ---------------------------------------------------------------------------

func TestExecJobUnit_Run_ExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exitCode int
		runErr   error
		wantErr  error
	}{
		{"exit_0_success", 0, nil, nil},
		{"exit_negative1_unexpected", -1, nil, ErrUnexpected},
		{"exit_1_nonzero", 1, nil, NonZeroExitError{ExitCode: 1}},
		{"exit_137_killed", 137, nil, NonZeroExitError{ExitCode: 137}},
		{"exec_error", 0, fmt.Errorf("connection refused"), fmt.Errorf("exec run")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newTestExecJobKit(t)
			ctx := newExecJobContext(t, k.job)

			k.exec.OnRun = func(_ context.Context, _ string, _ *domain.ExecConfig, _, _ io.Writer) (int, error) {
				if tc.runErr != nil {
					return -1, tc.runErr
				}
				return tc.exitCode, nil
			}

			err := k.job.Run(ctx)

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if errors.Is(tc.wantErr, ErrUnexpected) {
				if !errors.Is(err, ErrUnexpected) {
					t.Errorf("expected ErrUnexpected, got %v", err)
				}
			} else if nze, ok := tc.wantErr.(NonZeroExitError); ok {
				var got NonZeroExitError
				if !errors.As(err, &got) {
					t.Fatalf("expected NonZeroExitError, got %T: %v", err, err)
				}
				if got.ExitCode != nze.ExitCode {
					t.Errorf("expected exit code %d, got %d", nze.ExitCode, got.ExitCode)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InitializeRuntimeFields()
// ---------------------------------------------------------------------------

func TestExecJobUnit_InitializeRuntimeFields(t *testing.T) {
	t.Parallel()
	k := newTestExecJobKit(t)
	k.job.InitializeRuntimeFields()
	if k.job.Provider == nil {
		t.Error("provider should remain set after InitializeRuntimeFields")
	}
}

// ---------------------------------------------------------------------------
// RunWithStreams()
// ---------------------------------------------------------------------------

func TestExecJobUnit_RunWithStreams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exitCode int
		runErr   error
		wantErr  bool
	}{
		{"success", 0, nil, false},
		{"nonzero_exit", 2, nil, false},
		{"exec_error", 0, errors.New("connection lost"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newTestExecJobKit(t)

			k.exec.OnRun = func(_ context.Context, _ string, _ *domain.ExecConfig, _, _ io.Writer) (int, error) {
				if tc.runErr != nil {
					return -1, tc.runErr
				}
				return tc.exitCode, nil
			}

			exitCode, err := k.job.RunWithStreams(context.Background(), io.Discard, io.Discard)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if exitCode != tc.exitCode {
				t.Errorf("expected exit code %d, got %d", tc.exitCode, exitCode)
			}
		})
	}
}
