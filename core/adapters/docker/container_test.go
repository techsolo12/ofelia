// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"strings"
	"testing"
)

// TestContainerServiceAdapter_Inspect_EmptyID pins the contract that
// Inspect("") returns a non-nil error and does NOT panic.
//
// The Docker SDK validates empty IDs locally (client.emptyIDError) before
// issuing any HTTP request, so this test does not require a running
// Docker daemon. Pure coverage — no production change required.
func TestContainerServiceAdapter_Inspect_EmptyID(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Inspect panicked on empty ID: %v", r)
		}
	}()

	// Loopback SDK client — the SDK rejects the empty ID before
	// attempting to connect, so the host is never dialed.
	adapter := &ContainerServiceAdapter{client: newLoopbackSDKClient(t)}

	got, err := adapter.Inspect(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty container ID, got nil")
	}
	if got != nil {
		t.Errorf("expected nil container on error, got %+v", got)
	}
	// The SDK error message contains "empty" — keep the assertion loose
	// to avoid coupling to upstream wording, but verify the spirit.
	if !strings.Contains(strings.ToLower(err.Error()), "empty") &&
		!strings.Contains(strings.ToLower(err.Error()), "invalid") {
		t.Errorf("expected error mentioning empty/invalid ID, got: %v", err)
	}
}
