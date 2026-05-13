// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/netresearch/ofelia/core/domain"
)

// TestExecServiceAdapter_Create_NilConfig pins the contract that Create
// returns an error (and does NOT panic) when called with a nil ExecConfig.
//
// Before the fix this panicked on `config.User` at exec.go:27 because
// ExecOptions construction dereferences every config field unconditionally.
//
// Uses a loopback SDK client so the input-validation guard fires before
// the new ErrNilDockerClient guard added in #623 short-circuits the call.
func TestExecServiceAdapter_Create_NilConfig(t *testing.T) {
	t.Parallel()

	defer failOnPanic(t, "Create with nil config")()

	adapter := &ExecServiceAdapter{client: newLoopbackSDKClient(t)}

	id, err := adapter.Create(context.Background(), "some-container", nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
	if id != "" {
		t.Errorf("expected empty exec ID on error, got %q", id)
	}
	if !errors.Is(err, ErrNilExecConfig) {
		t.Errorf("expected errors.Is(err, ErrNilExecConfig), got: %v", err)
	}
}

// TestExecServiceAdapter_Run_NilWritersNonTTY pins the contract that Run
// returns an error (and does NOT panic) when stdout AND stderr are nil
// in non-TTY mode. stdcopy.StdCopy panics on nil writers when there is
// real output to demultiplex, so the adapter must guard the input.
//
// Uses a loopback SDK client so the writer-validation guard fires before
// the new ErrNilDockerClient guard added in #623 short-circuits the call.
func TestExecServiceAdapter_Run_NilWritersNonTTY(t *testing.T) {
	t.Parallel()

	defer failOnPanic(t, "Run with nil stdout+stderr in non-TTY")()

	adapter := &ExecServiceAdapter{client: newLoopbackSDKClient(t)}

	cfg := &domain.ExecConfig{Cmd: []string{"true"}, Tty: false}
	// Non-TTY mode: stdcopy demuxing path is exercised.
	code, err := adapter.Run(context.Background(), "some-container", cfg, nil, nil)
	if !errors.Is(err, ErrNoExecOutputWriter) {
		t.Errorf("expected errors.Is(err, ErrNoExecOutputWriter), got: %v", err)
	}
	if code != -1 {
		t.Errorf("expected exit code -1 on error, got %d", code)
	}
}
