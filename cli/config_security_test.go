// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/netresearch/ofelia/test"
)

func TestConfigAPI_DoesNotLeakSecrets(t *testing.T) {
	t.Parallel()

	c := NewConfig(test.NewTestLogger())
	c.Global.WebPasswordHash = "$2a$10$supersecretbcrypthashvalue"
	c.Global.WebSecretKey = "my-super-secret-jwt-signing-key-12345"

	data, err := json.Marshal(c.Global)
	if err != nil {
		t.Fatalf("Failed to marshal config Global: %v", err)
	}
	jsonStr := string(data)

	// The JSON output must not contain the actual secret values
	if strings.Contains(jsonStr, "$2a$10$supersecretbcrypthashvalue") {
		t.Error("JSON output contains WebPasswordHash value - secrets are leaking")
	}
	if strings.Contains(jsonStr, "my-super-secret-jwt-signing-key-12345") {
		t.Error("JSON output contains WebSecretKey value - secrets are leaking")
	}

	// The JSON output must not contain the field keys at all
	if strings.Contains(jsonStr, "WebPasswordHash") {
		t.Error("JSON output contains WebPasswordHash key - field should be excluded with json:\"-\"")
	}
	if strings.Contains(jsonStr, "WebSecretKey") {
		t.Error("JSON output contains WebSecretKey key - field should be excluded with json:\"-\"")
	}

	// Sanity check: other non-secret fields should still be present
	if !strings.Contains(jsonStr, "WebAddr") && !strings.Contains(jsonStr, "EnableWeb") {
		t.Error("JSON output is missing expected non-secret fields - sanity check failed")
	}
}
