// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/netresearch/ofelia/test"
)

// TestBuildFromString_SaveUndocumentedKeysWarn verifies that the keys which
// docs/CONFIGURATION.md previously documented for the Save middleware but
// which have no corresponding struct field in middlewares.SaveConfig produce
// "Unknown configuration key" warnings.
//
// Regression guard for issue #621: if `save-format` / `save-retention` come
// back to the docs without an actual struct field landing first, this test
// fires.
//
// See https://github.com/netresearch/ofelia/issues/621
func TestBuildFromString_SaveUndocumentedKeysWarn(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	_, err := BuildFromString(`
[global]
log-level = info
save-folder = /var/log/ofelia
save-format = json
save-retention = 30d
`, logger)
	if err != nil {
		t.Fatalf("BuildFromString failed: %v", err)
	}

	want := []string{
		"save-format",
		"save-retention",
	}
	for _, key := range want {
		if !handler.HasWarning("Unknown configuration key '" + key + "'") {
			t.Errorf("expected 'Unknown configuration key' warning for %q, did not see it", key)
		}
	}
}

// TestBuildFromString_SaveRestoreHistoryKeysAccepted verifies that the
// restore-history and restore-history-max-age keys (which exist on
// middlewares.SaveConfig but were undocumented before issue #621) are
// accepted without warnings and that their values land on the embedded
// SaveConfig in Config.Global.
//
// See https://github.com/netresearch/ofelia/issues/621
func TestBuildFromString_SaveRestoreHistoryKeysAccepted(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	c, err := BuildFromString(`
[global]
log-level = info
save-folder = /var/log/ofelia
restore-history = false
restore-history-max-age = 48h
`, logger)
	if err != nil {
		t.Fatalf("BuildFromString failed: %v", err)
	}

	for _, key := range []string{"restore-history", "restore-history-max-age"} {
		if handler.HasWarning("Unknown configuration key '" + key + "'") {
			t.Errorf("got 'Unknown configuration key' warning for %q (issue #621)", key)
		}
	}

	if c.Global.RestoreHistory == nil || *c.Global.RestoreHistory != false {
		t.Errorf("RestoreHistory = %v, want pointer to false", c.Global.RestoreHistory)
	}
	if c.Global.RestoreHistoryMaxAge.String() != "48h0m0s" {
		t.Errorf("RestoreHistoryMaxAge = %v, want 48h", c.Global.RestoreHistoryMaxAge)
	}
}
