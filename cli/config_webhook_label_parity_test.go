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
// webhook-trusted-preset-sources, webhook-preset-cache-dir) from the label
// allow-list so a malicious container cannot widen the network egress
// surface — see the comment in docker-labels.go above globalLabelAllowList
// and TestGlobalLabelAllowList_OmitsSSRFSensitiveWebhookKeys below.
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

// TestGlobalLabelAllowList_OmitsSSRFSensitiveWebhookKeys asserts that the
// SSRF-sensitive webhook globals are NOT in the Docker label allow-list. The
// security policy in #486 keeps these keys INI-only so a malicious container
// cannot widen the egress surface or redirect preset loading. The previous
// parity test only catches drift toward the allow-list; this test catches
// drift the other way (a future change accidentally enabling a sensitive key
// for labels).
func TestGlobalLabelAllowList_OmitsSSRFSensitiveWebhookKeys(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"webhook-allowed-hosts",
		"webhook-allow-remote-presets",
		"webhook-trusted-preset-sources",
		"webhook-preset-cache-dir",
		// preset-cache-ttl is excluded because mergeWebhookConfigs
		// does not currently propagate it from labels to the live
		// config; allow-listing it would silently document a label
		// that gets dropped on every reconcile.
		"webhook-preset-cache-ttl",
	}
	for _, key := range forbidden {
		assert.Falsef(t, globalLabelAllowList[key],
			"globalLabelAllowList must NOT contain %q — see #486 (SSRF) / #620 and the docker-labels.go comment",
			key)
	}
}

// TestApplyGlobalWebhookLabels_PrefixedKey_Applied asserts that the canonical
// webhook-webhooks key from a Docker label reaches WebhookConfigs.Global. A
// user copying their INI [global] webhook-webhooks key into a service-container
// label must get equivalent behavior.
func TestApplyGlobalWebhookLabels_PrefixedKey_Applied(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(test.NewTestLogger())
	globals := map[string]any{
		webhookGlobalKeyWebhooks: "wh-from-label",
	}

	applyGlobalWebhookLabels(cfg, globals)

	assert.Equal(t, "wh-from-label", cfg.WebhookConfigs.Global.Webhooks)
}

// TestApplyGlobalWebhookLabels_LegacyKey_BackwardCompat asserts that the OLD
// unprefixed webhooks label key (the one #618 left behind when it renamed the
// INI side) continues to work. Renaming-without-shim would silently drop user
// values for one release; the deprecation warning is the migration signal,
// the value is still applied.
func TestApplyGlobalWebhookLabels_LegacyKey_BackwardCompat(t *testing.T) {
	t.Parallel()

	t.Cleanup(resetDeprecatedLabelLogForTest)

	cfg := NewConfig(test.NewTestLogger())
	globals := map[string]any{
		legacyLabelKeyWebhooks: "legacy-wh",
	}

	applyGlobalWebhookLabels(cfg, globals)

	assert.Equal(t, "legacy-wh", cfg.WebhookConfigs.Global.Webhooks)
}

// TestApplyGlobalWebhookLabels_SSRFKeysIgnored is the security regression
// asserted by the #486 + #620 review: even when the SSRF-sensitive globals
// reach this helper directly (bypassing the production allow-list filter),
// they MUST NOT be applied to WebhookConfigs.Global. The defaults from
// NewWebhookConfigs must survive untouched.
func TestApplyGlobalWebhookLabels_SSRFKeysIgnored(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(test.NewTestLogger())
	defaults := *cfg.WebhookConfigs.Global

	globals := map[string]any{
		"webhook-allowed-hosts":          "evil.example.com",
		"webhook-allow-remote-presets":   "true",
		"webhook-trusted-preset-sources": "https://attacker.example/",
		"webhook-preset-cache-dir":       "/tmp/attacker",
		"webhook-preset-cache-ttl":       "1ns",
		// Legacy unprefixed forms must also be ignored.
		"allow-remote-presets":   "true",
		"trusted-preset-sources": "https://attacker.example/",
		"preset-cache-dir":       "/tmp/attacker",
		"preset-cache-ttl":       "1ns",
	}

	applyGlobalWebhookLabels(cfg, globals)

	assert.Equal(t, defaults.AllowedHosts, cfg.WebhookConfigs.Global.AllowedHosts,
		"webhook-allowed-hosts is INI-only; labels must not change it (#486)")
	assert.Equal(t, defaults.AllowRemotePresets, cfg.WebhookConfigs.Global.AllowRemotePresets,
		"webhook-allow-remote-presets is INI-only; labels must not change it (#486)")
	assert.Equal(t, defaults.TrustedPresetSources, cfg.WebhookConfigs.Global.TrustedPresetSources,
		"webhook-trusted-preset-sources is INI-only; labels must not change it (#486)")
	assert.Equal(t, defaults.PresetCacheDir, cfg.WebhookConfigs.Global.PresetCacheDir,
		"webhook-preset-cache-dir is INI-only; labels must not change it (#486)")
	assert.Equal(t, defaults.PresetCacheTTL, cfg.WebhookConfigs.Global.PresetCacheTTL,
		"webhook-preset-cache-ttl is not yet propagated; labels must not change it")
}

// TestApplyGlobalWebhookLabels_PrefixedWinsOverLegacy asserts that when both
// the canonical and the legacy form are set on the same container, the new
// form wins. Operators mid-migration must not see the legacy key clobber an
// explicit new-style value.
func TestApplyGlobalWebhookLabels_PrefixedWinsOverLegacy(t *testing.T) {
	t.Parallel()

	t.Cleanup(resetDeprecatedLabelLogForTest)

	cfg := NewConfig(test.NewTestLogger())
	globals := map[string]any{
		legacyLabelKeyWebhooks:   "legacy-wins-if-no-new",
		webhookGlobalKeyWebhooks: "new-prefixed-wins",
	}

	applyGlobalWebhookLabels(cfg, globals)

	assert.Equal(t, "new-prefixed-wins", cfg.WebhookConfigs.Global.Webhooks,
		"the new prefixed key must take precedence over the legacy unprefixed form")
}

// TestApplyGlobalWebhookLabels_DeprecationWarning_OneShot is the explicit
// gate test for the per-key one-shot semantics: two passes against the same
// legacy key must emit the deprecation warning exactly once. Without the
// resetDeprecatedLabelLogForTest helper this would be order-dependent on
// other tests in the package.
func TestApplyGlobalWebhookLabels_DeprecationWarning_OneShot(t *testing.T) {
	// Not parallel: the gate is a process-global sync.Map.
	t.Cleanup(resetDeprecatedLabelLogForTest)
	resetDeprecatedLabelLogForTest()

	logger, handler := test.NewTestLoggerWithHandler()
	cfg := NewConfig(logger)
	globals := map[string]any{legacyLabelKeyWebhooks: "wh"}

	applyGlobalWebhookLabels(cfg, globals)
	applyGlobalWebhookLabels(cfg, globals)

	deprecationWarnings := 0
	for _, r := range handler.GetMessages() {
		if strings.Contains(r.Message, "DEPRECATED Docker label key") {
			deprecationWarnings++
		}
	}
	assert.Equal(t, 1, deprecationWarnings,
		"the per-key one-shot gate must emit exactly one deprecation warning regardless of how many times applyGlobalWebhookLabels runs")
}

// TestApplyGlobalWebhookLabels_NilLogger_DoesNotConsumeOneShot asserts that
// an early invocation with a nil logger does NOT consume the one-shot gate.
// Otherwise a nil-logger pass during config bootstrap would permanently
// suppress the deprecation warning for the rest of the process, defeating
// the migration signal.
func TestApplyGlobalWebhookLabels_NilLogger_DoesNotConsumeOneShot(t *testing.T) {
	t.Cleanup(resetDeprecatedLabelLogForTest)
	resetDeprecatedLabelLogForTest()

	nilLoggerCfg := NewConfig(nil)
	applyGlobalWebhookLabels(nilLoggerCfg, map[string]any{legacyLabelKeyWebhooks: "wh"})

	logger, handler := test.NewTestLoggerWithHandler()
	loggerCfg := NewConfig(logger)
	applyGlobalWebhookLabels(loggerCfg, map[string]any{legacyLabelKeyWebhooks: "wh"})

	deprecationWarnings := 0
	for _, r := range handler.GetMessages() {
		if strings.Contains(r.Message, "DEPRECATED Docker label key") {
			deprecationWarnings++
		}
	}
	assert.Equal(t, 1, deprecationWarnings,
		"a prior nil-logger invocation must not consume the one-shot gate; the first real-logger invocation must still emit the warning")
}
