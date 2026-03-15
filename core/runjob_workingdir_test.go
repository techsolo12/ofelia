// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"testing"
)

// Bug #1: RunJob.buildContainer() does not pass WorkingDir to ContainerConfig,
// even though domain.ContainerConfig.WorkingDir exists and the Docker API supports it.

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
