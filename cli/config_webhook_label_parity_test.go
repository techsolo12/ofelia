// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/middlewares"
	"github.com/netresearch/ofelia/test"
)

// webhookGlobalConfigMapstructureTags collects every mapstructure tag declared on
// middlewares.WebhookGlobalConfig. The webhook-* INI keys live here, and any
// label-side allow-list / reader must reference one of these names — otherwise
// the data drifts between the INI and label code paths (see #620 part B).
func webhookGlobalConfigMapstructureTags(t *testing.T) map[string]bool {
	t.Helper()
	tags := make(map[string]bool)
	rt := reflect.TypeOf(middlewares.WebhookGlobalConfig{})
	for i := range rt.NumField() {
		tag := rt.Field(i).Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}
		// strip ",squash" / ",omitempty" suffixes
		if comma := strings.Index(tag, ","); comma != -1 {
			tag = tag[:comma]
		}
		if tag == "" {
			continue
		}
		tags[tag] = true
	}
	require.NotEmpty(t, tags,
		"WebhookGlobalConfig must declare mapstructure tags (sanity check)")
	return tags
}

// TestGlobalLabelAllowList_WebhookKeys_MatchMapstructureTags asserts that every
// webhook-prefixed entry in globalLabelAllowList corresponds to a real
// mapstructure tag on middlewares.WebhookGlobalConfig. This catches drift in
// either direction: a typo in the allow-list, or a struct-tag rename that
// forgot to update the label path.
//
// Direction is one-way on purpose: the security policy intentionally excludes
// SSRF-risky keys (webhook-allowed-hosts, webhook-allow-remote-presets,
// webhook-trusted-preset-sources) from the label allow-list so a malicious
// container cannot widen the network egress surface — see the comment in
// docker-labels.go above globalLabelAllowList. Strict set equality would
// either violate that policy or force a flag to opt into label-driven SSRF
// surface, neither of which the issue intended (#620).
func TestGlobalLabelAllowList_WebhookKeys_MatchMapstructureTags(t *testing.T) {
	t.Parallel()

	validTags := webhookGlobalConfigMapstructureTags(t)

	for key := range globalLabelAllowList {
		if !strings.HasPrefix(key, "webhook-") {
			continue
		}
		assert.Truef(t, validTags[key],
			"globalLabelAllowList contains %q but no field on middlewares.WebhookGlobalConfig declares mapstructure:%q — likely a typo or an INI rename that forgot to update docker-labels.go",
			key, key)
	}
}

// TestApplyGlobalWebhookLabels_PrefixedKeys_AlignWithINI asserts that the
// label-path reader (applyGlobalWebhookLabels) accepts the same webhook-* key
// names the INI parser uses. A user copying their INI [global] webhook-* keys
// into Docker labels (a reasonable workflow) must get equivalent behavior.
func TestApplyGlobalWebhookLabels_PrefixedKeys_AlignWithINI(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(test.NewTestLogger())
	globals := map[string]any{
		"webhook-webhooks":               "wh-from-label",
		"webhook-allow-remote-presets":   "true",
		"webhook-trusted-preset-sources": "https://gh.example.com",
		"webhook-preset-cache-ttl":       "30m",
		"webhook-preset-cache-dir":       "/srv/cache",
		"webhook-allowed-hosts":          "labels.example.com",
	}

	applyGlobalWebhookLabels(cfg, globals)

	assert.Equal(t, "wh-from-label", cfg.WebhookConfigs.Global.Webhooks)
	assert.True(t, cfg.WebhookConfigs.Global.AllowRemotePresets)
	assert.Equal(t, "https://gh.example.com", cfg.WebhookConfigs.Global.TrustedPresetSources)
	assert.Equal(t, "labels.example.com", cfg.WebhookConfigs.Global.AllowedHosts)
}

// TestApplyGlobalWebhookLabels_LegacyUnprefixedKeys_BackwardCompat asserts
// that the OLD unprefixed label keys (the ones #618 left behind when it
// renamed the INI side) continue to work, since rename-without-shim would
// silently drop user values for one release. The deprecation warning is the
// signal — value is still applied (#620).
func TestApplyGlobalWebhookLabels_LegacyUnprefixedKeys_BackwardCompat(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(test.NewTestLogger())
	globals := map[string]any{
		"webhooks":               "legacy-wh",
		"allow-remote-presets":   "true",
		"trusted-preset-sources": "https://legacy.example.com",
		"preset-cache-ttl":       "15m",
		"preset-cache-dir":       "/var/cache/legacy",
	}

	applyGlobalWebhookLabels(cfg, globals)

	// Values must still be applied for backward compatibility.
	assert.Equal(t, "legacy-wh", cfg.WebhookConfigs.Global.Webhooks)
	assert.True(t, cfg.WebhookConfigs.Global.AllowRemotePresets)
	assert.Equal(t, "https://legacy.example.com", cfg.WebhookConfigs.Global.TrustedPresetSources)
	assert.Equal(t, "/var/cache/legacy", cfg.WebhookConfigs.Global.PresetCacheDir)
}

// TestApplyGlobalWebhookLabels_PrefixedWinsOverLegacy asserts that when both
// the new prefixed key and the legacy unprefixed key are set on the same
// container, the new form wins. Operators mid-migration should not be
// surprised by the legacy key clobbering the explicit new-style value.
func TestApplyGlobalWebhookLabels_PrefixedWinsOverLegacy(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(test.NewTestLogger())
	globals := map[string]any{
		"webhooks":         "legacy-wins-if-no-new",
		"webhook-webhooks": "new-prefixed-wins",
	}

	applyGlobalWebhookLabels(cfg, globals)

	assert.Equal(t, "new-prefixed-wins", cfg.WebhookConfigs.Global.Webhooks,
		"the new prefixed key must take precedence over the legacy unprefixed form")
}
