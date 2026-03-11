// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/netresearch/ofelia/core/adapters/mock"
	"github.com/netresearch/ofelia/core/domain"
	"github.com/netresearch/ofelia/test"
)

// TestContextPropagation_ExecJob verifies that ExecJob.Run propagates
// the middleware chain context (ctx.Ctx) to the Docker provider, not
// context.Background(). This ensures scheduler shutdown, job removal,
// and max-runtime cancellation reach the Docker API calls.
func TestContextPropagation_ExecJob(t *testing.T) {
	t.Parallel()

	mc := mock.NewDockerClient()
	logger := test.NewTestLogger()
	provider := NewSDKDockerProviderFromClient(mc, logger, nil)

	job := NewExecJob(provider)
	job.BareJob = BareJob{Name: "ctx-exec", Command: "echo hello"}
	job.Container = "test-container"

	// Create a context with a cancel func and a recognizable value
	type ctxKey struct{}
	cancelCtx, cancel := context.WithCancel(context.Background())
	parentCtx := context.WithValue(cancelCtx, ctxKey{}, "propagated")
	defer cancel()

	// Track what context the Docker exec call receives
	var receivedCtx context.Context
	execSvc := mc.Exec().(*mock.ExecService)
	execSvc.OnRun = func(ctx context.Context, _ string, _ *domain.ExecConfig, _, _ io.Writer) (int, error) {
		receivedCtx = ctx //nolint:fatcontext // capturing ctx for test assertion, not nesting
		return 0, nil
	}

	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	if err != nil {
		t.Fatalf("NewExecution: %v", err)
	}
	exec.Start()

	jobCtx := NewContextWithContext(parentCtx, scheduler, job, exec)

	err = job.Run(jobCtx)
	if err != nil {
		t.Fatalf("ExecJob.Run: %v", err)
	}

	if receivedCtx == nil {
		t.Fatal("Docker provider RunExec was never called")
	}

	// The received context should carry the value from parentCtx
	val, ok := receivedCtx.Value(ctxKey{}).(string)
	if !ok || val != "propagated" {
		t.Errorf("ExecJob.Run did not propagate ctx.Ctx to Docker provider; got context value %q, want %q", val, "propagated")
	}
}

// TestContextPropagation_ExecJob_Canceled verifies that canceling the
// middleware context causes the ExecJob Docker call to see a canceled context.
func TestContextPropagation_ExecJob_Canceled(t *testing.T) {
	t.Parallel()

	mc := mock.NewDockerClient()
	logger := test.NewTestLogger()
	provider := NewSDKDockerProviderFromClient(mc, logger, nil)

	job := NewExecJob(provider)
	job.BareJob = BareJob{Name: "ctx-exec-cancel", Command: "echo hello"}
	job.Container = "test-container"

	parentCtx, cancel := context.WithCancel(context.Background())
	// Cancel BEFORE running so the context is already done
	cancel()

	var receivedCtx context.Context
	execSvc := mc.Exec().(*mock.ExecService)
	execSvc.OnRun = func(ctx context.Context, _ string, _ *domain.ExecConfig, _, _ io.Writer) (int, error) {
		receivedCtx = ctx //nolint:fatcontext // capturing ctx for test assertion, not nesting
		return 0, ctx.Err()
	}

	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	if err != nil {
		t.Fatalf("NewExecution: %v", err)
	}
	exec.Start()

	jobCtx := NewContextWithContext(parentCtx, scheduler, job, exec)

	err = job.Run(jobCtx)
	// We expect an error because the context was canceled
	if err == nil {
		t.Fatal("Expected error from canceled context, got nil")
	}

	if receivedCtx == nil {
		t.Fatal("Docker provider RunExec was never called")
	}

	// The received context should be canceled
	if receivedCtx.Err() == nil {
		t.Error("ExecJob.Run did not propagate canceled ctx.Ctx; context.Err() is nil")
	}
}

// TestContextPropagation_RunJob verifies that RunJob.Run propagates
// ctx.Ctx to the Docker provider calls instead of context.Background().
func TestContextPropagation_RunJob(t *testing.T) {
	t.Parallel()

	mc := mock.NewDockerClient()
	logger := test.NewTestLogger()
	provider := NewSDKDockerProviderFromClient(mc, logger, nil)

	job := NewRunJob(provider)
	job.BareJob = BareJob{Name: "ctx-run", Command: "echo hello"}
	job.Image = "alpine:latest"
	job.Delete = "true"
	job.Pull = "false"

	type ctxKey struct{}
	cancelCtx, cancel := context.WithCancel(context.Background())
	parentCtx := context.WithValue(cancelCtx, ctxKey{}, "run-propagated")
	defer cancel()

	// Track what context the Docker calls receive
	var receivedCreateCtx context.Context
	containerSvc := mc.Containers().(*mock.ContainerService)
	containerSvc.OnCreate = func(ctx context.Context, _ *domain.ContainerConfig) (string, error) {
		receivedCreateCtx = ctx //nolint:fatcontext // capturing ctx for test assertion, not nesting
		return "test-container-id", nil
	}

	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	if err != nil {
		t.Fatalf("NewExecution: %v", err)
	}
	exec.Start()

	jobCtx := NewContextWithContext(parentCtx, scheduler, job, exec)

	// The Run will fail at some point because we don't fully mock everything,
	// but CreateContainer should still be called with the right context.
	_ = job.Run(jobCtx)

	if receivedCreateCtx == nil {
		t.Fatal("Docker provider CreateContainer was never called")
	}

	val, ok := receivedCreateCtx.Value(ctxKey{}).(string)
	if !ok || val != "run-propagated" {
		t.Errorf("RunJob.Run did not propagate ctx.Ctx to Docker provider; got context value %q, want %q", val, "run-propagated")
	}
}

// TestContextPropagation_RunServiceJob verifies that RunServiceJob.Run
// propagates ctx.Ctx to the Docker provider calls.
func TestContextPropagation_RunServiceJob(t *testing.T) {
	t.Parallel()

	mc := mock.NewDockerClient()
	logger := test.NewTestLogger()
	provider := NewSDKDockerProviderFromClient(mc, logger, nil)

	job := NewRunServiceJob(provider)
	job.BareJob = BareJob{Name: "ctx-svc", Command: "echo hello"}
	job.Image = "alpine:latest"
	job.Delete = "true"

	type ctxKey struct{}
	cancelCtx, cancel := context.WithCancel(context.Background())
	parentCtx := context.WithValue(cancelCtx, ctxKey{}, "svc-propagated")
	defer cancel()

	// Track what context the Docker image ensure call receives
	var receivedEnsureCtx context.Context
	imageSvc := mc.Images().(*mock.ImageService)
	imageSvc.OnPullAndWait = func(ctx context.Context, _ domain.PullOptions) error {
		receivedEnsureCtx = ctx        //nolint:fatcontext // capturing ctx for test assertion, not nesting
		return errors.New("stop-here") // Stop early so we can check context
	}

	scheduler := NewScheduler(logger)
	exec, err := NewExecution()
	if err != nil {
		t.Fatalf("NewExecution: %v", err)
	}
	exec.Start()

	jobCtx := NewContextWithContext(parentCtx, scheduler, job, exec)

	// Run will fail at EnsureImage, but we can check the context it received
	_ = job.Run(jobCtx)

	if receivedEnsureCtx == nil {
		t.Fatal("Docker provider EnsureImage was never called")
	}

	val, ok := receivedEnsureCtx.Value(ctxKey{}).(string)
	if !ok || val != "svc-propagated" {
		t.Errorf("RunServiceJob.Run did not propagate ctx.Ctx; got context value %q, want %q", val, "svc-propagated")
	}
}
