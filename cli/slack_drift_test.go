// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/netresearch/ofelia/test"
)

// TestBuildFromString_SlackUndocumentedKeysWarn verifies that the keys which
// docs/CONFIGURATION.md previously documented for the (deprecated) Slack
// middleware but which have no corresponding struct field in
// middlewares.SlackConfig produce "Unknown configuration key" warnings.
//
// Regression guard for issue #621: if someone re-adds the unsupported keys
// to SlackConfig (or adds them to the docs again) without thinking through
// the deprecation strategy, this test still locks in the rejection contract.
// Operators following the outdated docs hit the warning - the documented
// surface is no longer silently accepted-then-ignored. The supported path
// for channel routing, mentions, custom username/avatar, etc. is the
// generic webhook system with `preset = slack`.
//
// See https://github.com/netresearch/ofelia/issues/621
func TestBuildFromString_SlackUndocumentedKeysWarn(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	_, err := BuildFromString(`
[global]
log-level = info
slack-url = https://hooks.slack.com/services/XXX/YYY/ZZZ
slack-channel = #alerts
slack-mentions = @channel
slack-icon-emoji = :robot:
slack-username = Ofelia Bot
`, logger)
	if err != nil {
		t.Fatalf("BuildFromString failed: %v", err)
	}

	want := []string{
		"slack-url",
		"slack-channel",
		"slack-mentions",
		"slack-icon-emoji",
		"slack-username",
	}
	for _, key := range want {
		if !handler.HasWarning("Unknown configuration key '" + key + "'") {
			t.Errorf("expected 'Unknown configuration key' warning for %q, did not see it", key)
		}
	}
}
