// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/middlewares"
	"github.com/netresearch/ofelia/test"
)

// TestSyncWebhookConfigs_PresetCacheTTLFromLabel_Forwarded covers fix (A) for
// issue #640. PR #637 originally allow-listed `webhook-preset-cache-ttl` as a
// Docker label, but the merge path dropped the value silently because
// mergeWebhookConfigs only forwarded `Webhooks` and `AllowedHosts`. PR #637's
// review removed the label from the allow-list to match reality. This test
// asserts the label value reaches `c.WebhookConfigs.Global.PresetCacheTTL` via
// the production merge path.
//
// Precedence mirrors the existing AllowedHosts policy: the label wins only
// when the INI side still holds the documented default (24h). This carries the
// same INI-set-to-default-is-indistinguishable-from-unset ambiguity as the
// AllowedHosts="*" sentinel; that's a deliberate, documented limitation
// (see mergeWebhookConfigs).
func TestSyncWebhookConfigs_PresetCacheTTLFromLabel_Forwarded(t *testing.T) {
	t.Parallel()

	c := NewConfig(test.NewTestLogger())
	// Sanity: the default seeded by NewConfig matches DefaultWebhookGlobalConfig.
	require.Equal(t, 24*time.Hour, c.WebhookConfigs.Global.PresetCacheTTL,
		"baseline: NewConfig must seed PresetCacheTTL to the documented default")

	parsed := NewWebhookConfigs()
	parsed.Global.PresetCacheTTL = 12 * time.Hour
	// At least one webhook so the merge actually iterates — the per-webhook
	// path is exercised independently in TestSyncWebhookConfigs_GlobalSelectorAlone_Applied.
	parsed.Webhooks["from-label"] = &middlewares.WebhookConfig{
		Name:   "from-label",
		Preset: "slack",
	}

	mergeWebhookConfigs(c, parsed)

	assert.Equal(t, 12*time.Hour, c.WebhookConfigs.Global.PresetCacheTTL,
		"webhook-preset-cache-ttl from a Docker label must reach c.WebhookConfigs.Global via the merge path (#640)")
}

// TestMergeWebhookConfigs_PresetCacheTTLINIWins covers the precedence half of
// fix (A): when the operator set a non-default PresetCacheTTL in the INI
// [global] section, a label on a service container must NOT clobber it. The
// "INI is at default → take label" sentinel keeps the same shape as the
// existing AllowedHosts="*" policy so reviewers see one consistent rule.
func TestMergeWebhookConfigs_PresetCacheTTLINIWins(t *testing.T) {
	t.Parallel()

	c := NewConfig(test.NewTestLogger())
	c.WebhookConfigs.Global.PresetCacheTTL = 6 * time.Hour // operator set in INI

	parsed := NewWebhookConfigs()
	parsed.Global.PresetCacheTTL = 12 * time.Hour
	parsed.Webhooks["from-label"] = &middlewares.WebhookConfig{
		Name:   "from-label",
		Preset: "slack",
	}

	mergeWebhookConfigs(c, parsed)

	assert.Equal(t, 6*time.Hour, c.WebhookConfigs.Global.PresetCacheTTL,
		"INI-set PresetCacheTTL must take precedence over a label-set value (#640)")
}

// TestSyncWebhookConfigs_GlobalSelectorAlone_Applied covers fix (B) for issue
// #640. mergeWebhookConfigs returned early when len(parsed.Webhooks) == 0, so
// an operator who set only `ofelia.webhook-webhooks=slack-alerts` on a service
// container (referencing INI-defined webhooks) saw the value silently dropped.
// The global-selector merge must happen regardless of per-webhook label count.
func TestSyncWebhookConfigs_GlobalSelectorAlone_Applied(t *testing.T) {
	t.Parallel()

	c := NewConfig(test.NewTestLogger())
	// Pre-existing INI webhook the label-only selector references.
	c.WebhookConfigs.Webhooks["slack-alerts"] = &middlewares.WebhookConfig{
		Name:   "slack-alerts",
		Preset: "slack",
	}
	if c.WebhookConfigs.iniWebhookNames == nil {
		c.WebhookConfigs.iniWebhookNames = make(map[string]struct{})
	}
	c.WebhookConfigs.iniWebhookNames["slack-alerts"] = struct{}{}

	// Operator labels on the service container: ONLY the global selector,
	// no per-webhook labels (because the webhook itself comes from INI).
	parsed := NewWebhookConfigs()
	parsed.Global.Webhooks = "slack-alerts"
	// parsed.Webhooks intentionally empty — this is the regression vector.

	mergeWebhookConfigs(c, parsed)

	assert.Equal(t, "slack-alerts", c.WebhookConfigs.Global.Webhooks,
		"webhook-webhooks selector from a Docker label must be applied even when no per-webhook labels are present (#640)")
}
