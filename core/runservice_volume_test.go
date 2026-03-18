// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"strings"
	"testing"

	"github.com/netresearch/ofelia/core/domain"
)

// TestRunServiceJob_BuildService_Volume verifies that Volume strings on
// RunServiceJob are parsed into ContainerSpec.Mounts in the service spec.
func TestRunServiceJob_BuildService_Volume(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.Volume = []string{
		"/host/data:/container/data",
		"/host/config:/container/config:ro",
	}

	svcID, err := k.job.buildService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svcID == "" {
		t.Fatal("expected non-empty service ID")
	}

	if len(k.services.CreateCalls) == 0 {
		t.Fatal("expected Create to be called")
	}
	mounts := k.services.CreateCalls[0].Spec.TaskTemplate.ContainerSpec.Mounts

	if len(mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d: %v", len(mounts), mounts)
	}

	// First mount: /host/data:/container/data (read-write)
	if mounts[0].Type != domain.MountTypeBind {
		t.Errorf("expected mount[0] type %q, got %q", domain.MountTypeBind, mounts[0].Type)
	}
	if mounts[0].Source != "/host/data" {
		t.Errorf("expected mount[0] source %q, got %q", "/host/data", mounts[0].Source)
	}
	if mounts[0].Target != "/container/data" {
		t.Errorf("expected mount[0] target %q, got %q", "/container/data", mounts[0].Target)
	}
	if mounts[0].ReadOnly {
		t.Error("expected mount[0] to be read-write")
	}

	// Second mount: /host/config:/container/config:ro (read-only)
	if mounts[1].Source != "/host/config" {
		t.Errorf("expected mount[1] source %q, got %q", "/host/config", mounts[1].Source)
	}
	if mounts[1].Target != "/container/config" {
		t.Errorf("expected mount[1] target %q, got %q", "/container/config", mounts[1].Target)
	}
	if !mounts[1].ReadOnly {
		t.Error("expected mount[1] to be read-only")
	}
}

// TestRunServiceJob_BuildService_VolumeNamedVolume verifies that named volumes
// (without leading /) are parsed as volume type mounts.
func TestRunServiceJob_BuildService_VolumeNamedVolume(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.Volume = []string{"mydata:/app/data"}

	svcID, err := k.job.buildService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svcID == "" {
		t.Fatal("expected non-empty service ID")
	}

	mounts := k.services.CreateCalls[0].Spec.TaskTemplate.ContainerSpec.Mounts
	if len(mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(mounts))
	}
	if mounts[0].Type != domain.MountTypeVolume {
		t.Errorf("expected mount type %q for named volume, got %q", domain.MountTypeVolume, mounts[0].Type)
	}
	if mounts[0].Source != "mydata" {
		t.Errorf("expected source %q, got %q", "mydata", mounts[0].Source)
	}
	if mounts[0].Target != "/app/data" {
		t.Errorf("expected target %q, got %q", "/app/data", mounts[0].Target)
	}
}

// TestRunServiceJob_BuildService_VolumeInvalid verifies that an invalid volume
// string causes buildService to return an error.
func TestRunServiceJob_BuildService_VolumeInvalid(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.Volume = []string{"invalid-no-target"}

	_, err := k.job.buildService(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid volume string")
	}
	if !strings.Contains(err.Error(), "volume config") {
		t.Errorf("expected error to contain %q, got %q", "volume config", err.Error())
	}
}

// TestRunServiceJob_BuildService_VolumeEmpty verifies that no volumes
// results in no mounts in the service spec.
func TestRunServiceJob_BuildService_VolumeEmpty(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)

	svcID, err := k.job.buildService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svcID == "" {
		t.Fatal("expected non-empty service ID")
	}

	mounts := k.services.CreateCalls[0].Spec.TaskTemplate.ContainerSpec.Mounts
	if len(mounts) != 0 {
		t.Errorf("expected no mounts when Volume is empty, got %d", len(mounts))
	}
}
