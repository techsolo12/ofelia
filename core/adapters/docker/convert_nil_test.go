// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/swarm"

	"github.com/netresearch/ofelia/core/domain"
)

// Regression tests for #632. PR #626 nil-guarded the *To*Swarm/*Mount
// converters; #632 covers the symmetric *From* helpers and the
// ContainerServiceAdapter.Create entry path that still dereferenced their
// pointer arguments unconditionally.
//
// The helpers under test are package-private. Production callers always
// pass non-nil pointers today, but the helper signatures invite unsafe
// direct calls from tests/refactors and the public Create() path passes
// the config straight through. Same bug class as #619 (panics in Exec) and
// #626 (panics in *To*Swarm).

// TestConvertFromSwarmService_NilInput pins the contract that
// convertFromSwarmService returns nil (no panic) when called with a nil
// *swarm.Service. Latent today (only test-callable), guarded for symmetry
// with convertToSwarmSpec.
func TestConvertFromSwarmService_NilInput(t *testing.T) {
	t.Parallel()

	defer failOnPanic(t, "convertFromSwarmService(nil)")()

	if got := convertFromSwarmService(nil); got != nil {
		t.Errorf("convertFromSwarmService(nil) = %+v, want nil", got)
	}
}

// TestConvertTaskTemplateFromSwarm_NilInputs pins the contract that
// convertTaskTemplateFromSwarm is a no-op (no panic) when either src or
// dst is nil. Mirrors the convertTaskTemplateToSwarm guard from #626.
func TestConvertTaskTemplateFromSwarm_NilInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  *swarm.TaskSpec
		dst  *domain.TaskSpec
	}{
		{name: "nil src", src: nil, dst: &domain.TaskSpec{}},
		{name: "nil dst", src: &swarm.TaskSpec{}, dst: nil},
		{name: "both nil", src: nil, dst: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer failOnPanic(t, "convertTaskTemplateFromSwarm "+tt.name)()
			convertTaskTemplateFromSwarm(tt.src, tt.dst)
		})
	}
}

// TestConvertFromSwarmTask_NilInput pins the contract that
// convertFromSwarmTask returns the zero domain.Task (no panic) when
// called with nil. Reachable through SwarmServiceAdapter.WaitForServiceTasks
// if the SDK ever yields a nil entry — defense-in-depth.
func TestConvertFromSwarmTask_NilInput(t *testing.T) {
	t.Parallel()

	defer failOnPanic(t, "convertFromSwarmTask(nil)")()

	got := convertFromSwarmTask(nil)
	if got.ID != "" || got.ServiceID != "" || got.NodeID != "" {
		t.Errorf("convertFromSwarmTask(nil) = %+v, want zero domain.Task", got)
	}
	if got.Status.ContainerStatus != nil {
		t.Errorf("convertFromSwarmTask(nil).Status.ContainerStatus = %+v, want nil",
			got.Status.ContainerStatus)
	}
	if !got.CreatedAt.IsZero() || !got.UpdatedAt.IsZero() {
		t.Errorf("convertFromSwarmTask(nil) timestamps = %v / %v, want zero",
			got.CreatedAt, got.UpdatedAt)
	}
}

// TestContainerServiceAdapter_Create_NilConfig pins the contract that
// Create returns ErrNilContainerConfig (and does NOT panic) when called
// with a nil *domain.ContainerConfig.
//
// Before the fix this panicked on `config.HostConfig` / `config.NetworkConfig`
// at container.go:47-48 because both sub-config converters were called
// unconditionally on the embedded pointer fields.
//
// Uses a loopback SDK client so the new input-validation guard fires
// before the daemon dial would fail (and before the existing
// ErrNilDockerClient guard).
func TestContainerServiceAdapter_Create_NilConfig(t *testing.T) {
	t.Parallel()

	defer failOnPanic(t, "ContainerServiceAdapter.Create with nil config")()

	adapter := &ContainerServiceAdapter{client: newLoopbackSDKClient(t)}

	id, err := adapter.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
	if id != "" {
		t.Errorf("expected empty container ID on error, got %q", id)
	}
	if !errors.Is(err, ErrNilContainerConfig) {
		t.Errorf("expected errors.Is(err, ErrNilContainerConfig), got: %v", err)
	}
}
