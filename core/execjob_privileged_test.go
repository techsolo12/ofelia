// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"io"
	"testing"

	"github.com/netresearch/ofelia/core/domain"
)

// Bug #3: ExecJob does not expose the Privileged field,
// even though domain.ExecConfig.Privileged exists and the Docker API supports it.

func TestExecJob_Run_Privileged(t *testing.T) {
	t.Parallel()
	k := newTestExecJobKit(t)
	k.job.Privileged = true

	k.exec.OnRun = func(_ context.Context, _ string, config *domain.ExecConfig, _, _ io.Writer) (int, error) {
		if !config.Privileged {
			t.Error("expected Privileged=true in ExecConfig")
		}
		return 0, nil
	}

	ctx := newExecJobContext(t, k.job)
	if err := k.job.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(k.exec.RunCalls) == 0 {
		t.Fatal("expected RunExec to be called")
	}
	if !k.exec.RunCalls[0].Config.Privileged {
		t.Error("expected Privileged=true in captured RunCall config")
	}
}

func TestExecJob_Run_PrivilegedDefault(t *testing.T) {
	t.Parallel()
	k := newTestExecJobKit(t)
	// Privileged not set — should default to false

	ctx := newExecJobContext(t, k.job)
	if err := k.job.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(k.exec.RunCalls) == 0 {
		t.Fatal("expected RunExec to be called")
	}
	if k.exec.RunCalls[0].Config.Privileged {
		t.Error("expected Privileged=false by default")
	}
}
