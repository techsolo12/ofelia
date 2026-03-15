// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"testing"
)

// TestRunJob_BuildContainer_WorkingDir verifies that RunJob.WorkingDir is passed
// through to domain.ContainerConfig.WorkingDir when building a container.

func TestRunJob_BuildContainer_WorkingDir(t *testing.T) {
	t.Parallel()
	k := newTestRunJobKit(t)
	k.job.WorkingDir = "/app/data"

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

	if cfg.WorkingDir != "/app/data" {
		t.Errorf("expected WorkingDir=%q, got %q", "/app/data", cfg.WorkingDir)
	}
}

func TestRunJob_BuildContainer_WorkingDirEmpty(t *testing.T) {
	t.Parallel()
	k := newTestRunJobKit(t)
	// WorkingDir not set — should default to empty (Docker uses image default)

	id, err := k.job.buildContainer(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty container ID")
	}

	cfg := k.containers.CreateCalls[0].Config
	if cfg.WorkingDir != "" {
		t.Errorf("expected empty WorkingDir when not set, got %q", cfg.WorkingDir)
	}
}
