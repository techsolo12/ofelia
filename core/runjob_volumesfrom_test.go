// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"testing"
)

// Bug #2: RunJob.VolumesFrom exists in the struct but is never passed to
// buildContainer(). The domain.HostConfig also lacks a VolumesFrom field,
// so the full chain (job → domain → adapter) needs wiring.

func TestRunJob_BuildContainer_VolumesFrom(t *testing.T) {
	t.Parallel()
	k := newTestRunJobKit(t)
	k.job.VolumesFrom = []string{"data-container", "config-container:ro"}

	id, err := k.job.buildContainer(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty container ID")
	}

	if len(k.containers.CreateCalls) == 0 {
		t.Fatal("expected Create to be called")
	}
	cfg := k.containers.CreateCalls[0].Config

	if cfg.HostConfig == nil {
		t.Fatal("expected HostConfig to be set")
	}
	vf := cfg.HostConfig.VolumesFrom
	if len(vf) != 2 {
		t.Fatalf("expected 2 VolumesFrom entries, got %d: %v", len(vf), vf)
	}
	if vf[0] != "data-container" {
		t.Errorf("expected VolumesFrom[0]=%q, got %q", "data-container", vf[0])
	}
	if vf[1] != "config-container:ro" {
		t.Errorf("expected VolumesFrom[1]=%q, got %q", "config-container:ro", vf[1])
	}
}

func TestRunJob_BuildContainer_VolumesFromEmpty(t *testing.T) {
	t.Parallel()
	k := newTestRunJobKit(t)
	// VolumesFrom not set — HostConfig.VolumesFrom should be nil/empty

	id, err := k.job.buildContainer(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty container ID")
	}

	cfg := k.containers.CreateCalls[0].Config
	if cfg.HostConfig != nil && len(cfg.HostConfig.VolumesFrom) > 0 {
		t.Errorf("expected empty VolumesFrom when not set, got %v", cfg.HostConfig.VolumesFrom)
	}
}
