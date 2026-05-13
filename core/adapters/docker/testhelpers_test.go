// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"testing"

	"github.com/docker/docker/client"
)

// newLoopbackSDKClient builds an SDK client pointed at a loopback address
// nothing listens on. Useful for adapter tests that exercise input
// validation (empty ID, nil config, etc.) — the SDK rejects bad inputs
// before attempting to dial, so the host is never reached. Registered with
// t.Cleanup to close the client.
func newLoopbackSDKClient(t *testing.T) *client.Client {
	t.Helper()
	sdk, err := client.NewClientWithOpts(client.WithHost("tcp://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("constructing loopback SDK client: %v", err)
	}
	t.Cleanup(func() { _ = sdk.Close() })
	return sdk
}

// failOnPanic registers a deferred recover that fails the test with a
// descriptive message if the surrounding test function panics. Used by the
// nil-input regression tests to assert that the new guards return errors
// instead of crashing the daemon. Call inline, not via defer:
//
//	defer failOnPanic(t, "Create with nil config")
func failOnPanic(t *testing.T, what string) func() {
	t.Helper()
	return func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked: %v", what, r)
		}
	}
}
