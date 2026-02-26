// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/netresearch/ofelia/core/adapters/mock"
	"github.com/netresearch/ofelia/core/domain"
	"github.com/netresearch/ofelia/test"
)

// testRunServiceKit holds mock objects for RunServiceJob unit tests.
type testRunServiceKit struct {
	job      *RunServiceJob
	client   *mock.DockerClient
	services *mock.SwarmService
	images   *mock.ImageService
	handler  *test.Handler
}

// newTestRunServiceKit creates a RunServiceJob wired to a mock DockerClient.
func newTestRunServiceKit(t *testing.T) *testRunServiceKit {
	t.Helper()
	mc := mock.NewDockerClient()
	logger, handler := test.NewTestLoggerWithHandler()
	provider := NewSDKDockerProviderFromClient(mc, logger, nil)
	job := NewRunServiceJob(provider)
	job.BareJob = BareJob{Name: "unit-svc", Command: "echo hello"}
	job.Image = "alpine:latest"
	job.Delete = "true"
	return &testRunServiceKit{
		job:      job,
		client:   mc,
		services: mc.Services().(*mock.SwarmService),
		images:   mc.Images().(*mock.ImageService),
		handler:  handler,
	}
}

// newRunServiceJobContext creates a Context suitable for RunServiceJob unit tests.
func newRunServiceJobContext(t *testing.T, job *RunServiceJob) *Context {
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
// NewRunServiceJob / InitializeRuntimeFields / Validate
// ---------------------------------------------------------------------------

func TestRunServiceJobUnit_NewAndInit(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	if k.job.Provider == nil {
		t.Error("Provider should be set")
	}
	k.job.InitializeRuntimeFields()
}

func TestRunServiceJobUnit_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"valid_image", "nginx:latest", false},
		{"empty_image", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			job := &RunServiceJob{Image: tc.image}
			err := job.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr && !errors.Is(err, ErrImageRequired) {
				t.Errorf("expected ErrImageRequired, got %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Run(): success and error paths
// ---------------------------------------------------------------------------

func TestRunServiceJobUnit_Run_Success(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	ctx := newRunServiceJobContext(t, k.job)

	k.services.SetTasks([]domain.Task{
		{
			ID:        "task-1",
			ServiceID: "mock-service-id",
			Status: domain.TaskStatus{
				State:           domain.TaskStateComplete,
				ContainerStatus: &domain.ContainerStatus{ExitCode: 0},
			},
		},
	})

	err := k.job.Run(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(k.services.CreateCalls) == 0 {
		t.Error("expected service to be created")
	}
	if len(k.services.RemoveCalls) == 0 {
		t.Error("expected service to be removed")
	}
}

func TestRunServiceJobUnit_Run_EnsureImageError(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	ctx := newRunServiceJobContext(t, k.job)

	k.images.OnPullAndWait = func(_ context.Context, _ domain.PullOptions) error {
		return errors.New("registry down")
	}
	k.images.SetExistsResult(false)

	err := k.job.Run(ctx)
	if err == nil {
		t.Fatal("expected error from EnsureImage")
	}
}

func TestRunServiceJobUnit_Run_CreateServiceError(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	ctx := newRunServiceJobContext(t, k.job)

	k.services.OnCreate = func(_ context.Context, _ domain.ServiceSpec, _ domain.ServiceCreateOptions) (string, error) {
		return "", errors.New("swarm unavailable")
	}

	err := k.job.Run(ctx)
	if err == nil {
		t.Fatal("expected error from buildService")
	}
}

// ---------------------------------------------------------------------------
// buildService()
// ---------------------------------------------------------------------------

func TestRunServiceJobUnit_BuildService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		network     string
		command     string
		annotations []string
	}{
		{"basic_service", "", "", nil},
		{"with_network", "overlay-net", "", nil},
		{"with_command", "", "sleep 10", nil},
		{"with_annotations", "", "", []string{"team=devops", "tier=backend"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newTestRunServiceKit(t)
			k.job.Network = tc.network
			k.job.Command = tc.command
			k.job.Annotations = tc.annotations

			svcID, err := k.job.buildService(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if svcID == "" {
				t.Error("expected non-empty service ID")
			}

			if len(k.services.CreateCalls) == 0 {
				t.Fatal("expected Create to be called")
			}
			spec := k.services.CreateCalls[0].Spec

			if tc.network != "" {
				if len(spec.TaskTemplate.Networks) == 0 {
					t.Error("expected network attachment")
				} else if spec.TaskTemplate.Networks[0].Target != tc.network {
					t.Errorf("expected network %q, got %q", tc.network, spec.TaskTemplate.Networks[0].Target)
				}
			}

			if tc.command != "" && len(spec.TaskTemplate.ContainerSpec.Command) == 0 {
				t.Error("expected command to be set")
			}

			if _, ok := spec.Labels["ofelia.job.name"]; !ok {
				t.Error("expected default annotation ofelia.job.name")
			}
			if _, ok := spec.Labels["ofelia.job.type"]; !ok {
				t.Error("expected default annotation ofelia.job.type")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findTaskStatus()
// ---------------------------------------------------------------------------

func TestRunServiceJobUnit_FindTaskStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tasks        []domain.Task
		listErr      error
		wantExitCode int
		wantDone     bool
	}{
		{"no_tasks_means_done", []domain.Task{}, nil, 0, true},
		{"complete_exit_0", []domain.Task{{ID: "t1", Status: domain.TaskStatus{
			State: domain.TaskStateComplete, ContainerStatus: &domain.ContainerStatus{ExitCode: 0},
		}}}, nil, 0, true},
		{"failed_exit_1", []domain.Task{{ID: "t2", Status: domain.TaskStatus{
			State: domain.TaskStateFailed, ContainerStatus: &domain.ContainerStatus{ExitCode: 1},
		}}}, nil, 1, true},
		{"rejected_forces_255", []domain.Task{{ID: "t3", Status: domain.TaskStatus{
			State: domain.TaskStateRejected, ContainerStatus: &domain.ContainerStatus{ExitCode: 0},
		}}}, nil, 255, true},
		{"running_not_done", []domain.Task{{ID: "t4", Status: domain.TaskStatus{
			State: domain.TaskStateRunning,
		}}}, nil, 0, false},
		{"list_error_not_done", nil, errors.New("swarm error"), 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newTestRunServiceKit(t)
			ctx := newRunServiceJobContext(t, k.job)

			if tc.listErr != nil {
				k.services.OnListTasks = func(_ context.Context, _ domain.TaskListOptions) ([]domain.Task, error) {
					return nil, tc.listErr
				}
			} else {
				k.services.SetTasks(tc.tasks)
			}

			exitCode, done := k.job.findTaskStatus(context.Background(), ctx, "svc-1")
			if done != tc.wantDone {
				t.Errorf("expected done=%v, got %v", tc.wantDone, done)
			}
			if done && exitCode != tc.wantExitCode {
				t.Errorf("expected exitCode=%d, got %d", tc.wantExitCode, exitCode)
			}
		})
	}
}

func TestRunServiceJobUnit_WatchContainer_Timeout(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.MaxRuntime = 100 * time.Millisecond
	ctx := newRunServiceJobContext(t, k.job)

	k.services.SetTasks([]domain.Task{{ID: "t-running", Status: domain.TaskStatus{State: domain.TaskStateRunning}}})

	err := k.job.watchContainer(context.Background(), ctx, "svc-timeout")
	if !errors.Is(err, ErrMaxTimeRunning) {
		t.Fatalf("expected ErrMaxTimeRunning, got %v", err)
	}
}

func TestRunServiceJobUnit_WatchContainer_InspectError(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	ctx := newRunServiceJobContext(t, k.job)

	k.services.OnInspect = func(_ context.Context, _ string) (*domain.Service, error) {
		return nil, errors.New("service gone")
	}

	err := k.job.watchContainer(context.Background(), ctx, "svc-gone")
	if err == nil {
		t.Fatal("expected error from inspect")
	}
}

// ---------------------------------------------------------------------------
// deleteService()
// ---------------------------------------------------------------------------

func TestRunServiceJobUnit_DeleteService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		deleteFlag string
		removeErr  error
		wantRemove bool
		wantErr    bool
	}{
		{"delete_true", "true", nil, true, false},
		{"delete_false", "false", nil, false, false},
		{"not_found_is_not_error", "true", errors.New("service not found"), true, false},
		{"real_remove_error", "true", errors.New("permission denied"), true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := newTestRunServiceKit(t)
			k.job.Delete = tc.deleteFlag
			ctx := newRunServiceJobContext(t, k.job)

			if tc.removeErr != nil {
				k.services.OnRemove = func(_ context.Context, _ string) error {
					return tc.removeErr
				}
			}

			err := k.job.deleteService(context.Background(), ctx, "svc-del")
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			removed := len(k.services.RemoveCalls) > 0
			if removed != tc.wantRemove {
				t.Errorf("expected remove=%v, got %v", tc.wantRemove, removed)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isNotFoundError()
// ---------------------------------------------------------------------------

func TestRunServiceJobUnit_IsNotFoundError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil_error", nil, false},
		{"not_found", errors.New("service not found"), true},
		{"no_such", errors.New("no such service"), true},
		{"404_status", errors.New("HTTP 404"), true},
		{"other_error", errors.New("permission denied"), false},
		{"mixed_case", errors.New("Not Found: service xyz"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isNotFoundError(tc.err)
			if got != tc.want {
				t.Errorf("isNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
