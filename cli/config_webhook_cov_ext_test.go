// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ini "gopkg.in/ini.v1"

	"github.com/netresearch/ofelia/core"
	"github.com/netresearch/ofelia/middlewares"
	"github.com/netresearch/ofelia/test"
)

// --- WebhookGlobalConfig aliasing (single source of truth, #620) ---

// TestWebhookGlobalConfig_AllFields verifies the full INI pipeline populates
// c.WebhookConfigs.Global via the embedded struct alias.
func TestWebhookGlobalConfig_AllFields(t *testing.T) {
	t.Parallel()

	c, err := BuildFromString(`
[global]
webhook-webhooks               = wh1,wh2
webhook-allow-remote-presets   = true
webhook-trusted-preset-sources = https://example.com
webhook-preset-cache-ttl       = 1h
webhook-preset-cache-dir       = /tmp/presets
webhook-allowed-hosts          = example.com,test.com
`, test.NewTestLogger())
	require.NoError(t, err)

	assert.Equal(t, "wh1,wh2", c.WebhookConfigs.Global.Webhooks)
	assert.True(t, c.WebhookConfigs.Global.AllowRemotePresets)
	assert.Equal(t, "https://example.com", c.WebhookConfigs.Global.TrustedPresetSources)
	assert.Equal(t, time.Hour, c.WebhookConfigs.Global.PresetCacheTTL)
	assert.Equal(t, "/tmp/presets", c.WebhookConfigs.Global.PresetCacheDir)
	assert.Equal(t, "example.com,test.com", c.WebhookConfigs.Global.AllowedHosts)
}

// TestWebhookGlobalConfig_AliasesEmbeddedStruct asserts that c.WebhookConfigs.Global
// is the same address as &c.Global.WebhookGlobalConfig — the dual-store
// antipattern collapse from #620. A mutation via either side must be visible
// from the other without an explicit sync call.
func TestWebhookGlobalConfig_AliasesEmbeddedStruct(t *testing.T) {
	t.Parallel()

	c := NewConfig(test.NewTestLogger())
	require.Same(t, &c.Global.WebhookGlobalConfig, c.WebhookConfigs.Global,
		"c.WebhookConfigs.Global must alias the embedded WebhookGlobalConfig (single source of truth)")

	// Mutate via the embedded struct → visible via the WebhookConfigs.Global pointer.
	c.Global.WebhookGlobalConfig.Webhooks = "from-embedded"
	assert.Equal(t, "from-embedded", c.WebhookConfigs.Global.Webhooks)

	// Mutate via the WebhookConfigs.Global pointer → visible via the embedded struct.
	c.WebhookConfigs.Global.AllowedHosts = "from-pointer.example.com"
	assert.Equal(t, "from-pointer.example.com", c.Global.WebhookGlobalConfig.AllowedHosts)
}

func TestWebhookGlobalConfig_NoKeys_DefaultsPreserved(t *testing.T) {
	t.Parallel()

	// Empty [global] section: defaults must be preserved.
	c, err := BuildFromString(`
[global]
`, test.NewTestLogger())
	require.NoError(t, err)

	// Defaults from DefaultWebhookGlobalConfig() preserved.
	assert.Empty(t, c.WebhookConfigs.Global.Webhooks)
	assert.Equal(t, "*", c.WebhookConfigs.Global.AllowedHosts)
	assert.Equal(t, 24*time.Hour, c.WebhookConfigs.Global.PresetCacheTTL)
}

func TestWebhookGlobalConfig_InvalidDuration(t *testing.T) {
	t.Parallel()

	// Invalid duration: mapstructure returns an error from BuildFromString,
	// which is the expected behavior (surface bad config to the user).
	_, err := BuildFromString(`
[global]
webhook-preset-cache-ttl = not-a-duration
`, test.NewTestLogger())
	require.Error(t, err)
}

// --- parseWebhookConfig ---

func TestParseWebhookConfig_AllFields(t *testing.T) {
	t.Parallel()

	cfgFile := ini.Empty()
	sec, _ := cfgFile.NewSection("webhook")
	sec.Key("preset").SetValue("slack")
	sec.Key("id").SetValue("my-id")
	sec.Key("secret").SetValue("my-secret")
	sec.Key("url").SetValue("https://hooks.example.com")
	sec.Key("trigger").SetValue("on-error")
	sec.Key("timeout").SetValue("30s")
	sec.Key("retry-count").SetValue("3")
	sec.Key("retry-delay").SetValue("5s")
	sec.Key("link").SetValue("https://example.com/dashboard")
	sec.Key("link-text").SetValue("View Dashboard")

	config := middlewares.DefaultWebhookConfig()
	err := parseWebhookConfig(sec, config)
	require.NoError(t, err)

	assert.Equal(t, "slack", config.Preset)
	assert.Equal(t, "my-id", config.ID)
	assert.Equal(t, "my-secret", config.Secret)
	assert.Equal(t, "https://hooks.example.com", config.URL)
	assert.Equal(t, middlewares.TriggerType("on-error"), config.Trigger)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.RetryCount)
	assert.Equal(t, 5*time.Second, config.RetryDelay)
	assert.Equal(t, "https://example.com/dashboard", config.Link)
	assert.Equal(t, "View Dashboard", config.LinkText)
}

func TestParseWebhookConfig_InvalidDurations(t *testing.T) {
	t.Parallel()

	cfgFile := ini.Empty()
	sec, _ := cfgFile.NewSection("webhook")
	sec.Key("timeout").SetValue("invalid")
	sec.Key("retry-count").SetValue("not-int")
	sec.Key("retry-delay").SetValue("also-invalid")

	config := middlewares.DefaultWebhookConfig()
	origTimeout := config.Timeout
	origRetryCount := config.RetryCount
	origRetryDelay := config.RetryDelay

	err := parseWebhookConfig(sec, config)
	require.NoError(t, err)

	// Invalid values should leave defaults
	assert.Equal(t, origTimeout, config.Timeout)
	assert.Equal(t, origRetryCount, config.RetryCount)
	assert.Equal(t, origRetryDelay, config.RetryDelay)
}

// --- parseWebhookSections ---

func TestParseWebhookSections_EmptyName(t *testing.T) {
	t.Parallel()

	configStr := `[webhook]
url = https://example.com
`
	_, err := BuildFromString(configStr, test.NewTestLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook section must have a name")
}

func TestParseWebhookSections_NilWebhookConfigs(t *testing.T) {
	t.Parallel()

	cfgFile := ini.Empty()
	_, _ = cfgFile.NewSection("global")

	c := NewConfig(test.NewTestLogger())
	c.WebhookConfigs = nil

	err := parseWebhookSections(cfgFile, c)
	require.NoError(t, err)
	assert.NotNil(t, c.WebhookConfigs)
}

// --- syncWebhookConfigs ---

func TestSyncWebhookConfigs_NoChanges_Coverage(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cfg := NewConfig(logger)
	cfg.sh = core.NewScheduler(logger)
	cfg.buildSchedulerMiddlewares(cfg.sh)
	cfg.WebhookConfigs = NewWebhookConfigs()

	parsed := NewWebhookConfigs()
	// No webhooks in parsed = no changes
	cfg.syncWebhookConfigs(parsed)
}

func TestSyncWebhookConfigs_NewWebhookAdded(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cfg := NewConfig(logger)
	cfg.sh = core.NewScheduler(logger)
	cfg.buildSchedulerMiddlewares(cfg.sh)
	cfg.WebhookConfigs = NewWebhookConfigs()

	parsed := NewWebhookConfigs()
	parsed.Webhooks["new-hook"] = &middlewares.WebhookConfig{
		Name: "new-hook",
		URL:  "https://example.com/hook",
	}

	cfg.syncWebhookConfigs(parsed)

	assert.Contains(t, cfg.WebhookConfigs.Webhooks, "new-hook")
}

func TestSyncWebhookConfigs_INIProtected(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cfg := NewConfig(logger)
	cfg.sh = core.NewScheduler(logger)
	cfg.buildSchedulerMiddlewares(cfg.sh)
	cfg.WebhookConfigs = NewWebhookConfigs()

	// Add an INI-defined webhook
	cfg.WebhookConfigs.Webhooks["protected"] = &middlewares.WebhookConfig{
		Name: "protected",
		URL:  "https://ini.example.com",
	}
	cfg.WebhookConfigs.iniWebhookNames = map[string]struct{}{
		"protected": {},
	}

	parsed := NewWebhookConfigs()
	parsed.Webhooks["protected"] = &middlewares.WebhookConfig{
		Name: "protected",
		URL:  "https://label.example.com", // Different URL from labels
	}

	cfg.syncWebhookConfigs(parsed)

	// INI webhook should be preserved
	assert.Equal(t, "https://ini.example.com", cfg.WebhookConfigs.Webhooks["protected"].URL)
}

func TestSyncWebhookConfigs_WebhookRemoved(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cfg := NewConfig(logger)
	cfg.sh = core.NewScheduler(logger)
	cfg.buildSchedulerMiddlewares(cfg.sh)
	cfg.WebhookConfigs = NewWebhookConfigs()

	// Add a label-defined webhook
	cfg.WebhookConfigs.Webhooks["label-hook"] = &middlewares.WebhookConfig{
		Name: "label-hook",
		URL:  "https://example.com/old",
	}

	parsed := NewWebhookConfigs()
	// label-hook is NOT in parsed -> should be removed

	cfg.syncWebhookConfigs(parsed)

	assert.NotContains(t, cfg.WebhookConfigs.Webhooks, "label-hook")
}

func TestSyncWebhookConfigs_WebhookChanged(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cfg := NewConfig(logger)
	cfg.sh = core.NewScheduler(logger)
	cfg.buildSchedulerMiddlewares(cfg.sh)
	cfg.WebhookConfigs = NewWebhookConfigs()

	cfg.WebhookConfigs.Webhooks["my-hook"] = &middlewares.WebhookConfig{
		Name: "my-hook",
		URL:  "https://example.com/old",
	}

	parsed := NewWebhookConfigs()
	parsed.Webhooks["my-hook"] = &middlewares.WebhookConfig{
		Name: "my-hook",
		URL:  "https://example.com/new", // Changed URL
	}

	cfg.syncWebhookConfigs(parsed)

	assert.Equal(t, "https://example.com/new", cfg.WebhookConfigs.Webhooks["my-hook"].URL)
}

// --- rebuildAllMiddlewares ---

func TestRebuildAllMiddlewares(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cfg := NewConfig(logger)
	cfg.sh = core.NewScheduler(logger)
	cfg.buildSchedulerMiddlewares(cfg.sh)
	cfg.WebhookConfigs = NewWebhookConfigs()

	// Add a job to the scheduler
	j := &LocalJobConfig{}
	j.Schedule = "@daily"
	j.Command = "echo test"
	j.Name = "testjob"
	_ = cfg.sh.AddJob(j)

	// Should not panic
	cfg.rebuildAllMiddlewares()
}

// --- webhookConfigChanged ---

func TestWebhookConfigChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		a, b    *middlewares.WebhookConfig
		changed bool
	}{
		{
			name:    "identical configs",
			a:       &middlewares.WebhookConfig{URL: "http://a.com", Preset: "slack"},
			b:       &middlewares.WebhookConfig{URL: "http://a.com", Preset: "slack"},
			changed: false,
		},
		{
			name:    "different URL",
			a:       &middlewares.WebhookConfig{URL: "http://a.com"},
			b:       &middlewares.WebhookConfig{URL: "http://b.com"},
			changed: true,
		},
		{
			name:    "different preset",
			a:       &middlewares.WebhookConfig{Preset: "slack"},
			b:       &middlewares.WebhookConfig{Preset: "discord"},
			changed: true,
		},
		{
			name:    "different trigger",
			a:       &middlewares.WebhookConfig{Trigger: "on-error"},
			b:       &middlewares.WebhookConfig{Trigger: "always"},
			changed: true,
		},
		{
			name:    "different timeout",
			a:       &middlewares.WebhookConfig{Timeout: 5 * time.Second},
			b:       &middlewares.WebhookConfig{Timeout: 10 * time.Second},
			changed: true,
		},
		{
			name:    "different retry count",
			a:       &middlewares.WebhookConfig{RetryCount: 1},
			b:       &middlewares.WebhookConfig{RetryCount: 3},
			changed: true,
		},
		{
			name:    "different retry delay",
			a:       &middlewares.WebhookConfig{RetryDelay: 1 * time.Second},
			b:       &middlewares.WebhookConfig{RetryDelay: 5 * time.Second},
			changed: true,
		},
		{
			name:    "different link",
			a:       &middlewares.WebhookConfig{Link: "http://a.com"},
			b:       &middlewares.WebhookConfig{Link: "http://b.com"},
			changed: true,
		},
		{
			name:    "different link text",
			a:       &middlewares.WebhookConfig{LinkText: "A"},
			b:       &middlewares.WebhookConfig{LinkText: "B"},
			changed: true,
		},
		{
			name:    "different ID",
			a:       &middlewares.WebhookConfig{ID: "id1"},
			b:       &middlewares.WebhookConfig{ID: "id2"},
			changed: true,
		},
		{
			name:    "different secret",
			a:       &middlewares.WebhookConfig{Secret: "s1"},
			b:       &middlewares.WebhookConfig{Secret: "s2"},
			changed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.changed, webhookConfigChanged(tt.a, tt.b))
		})
	}
}

// --- applyGlobalWebhookLabels ---

func TestApplyGlobalWebhookLabels_PartialFields(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(test.NewTestLogger())
	globals := map[string]any{
		"webhooks":              "wh1",
		"webhook-allowed-hosts": "myhost.com",
	}

	applyGlobalWebhookLabels(cfg, globals)

	assert.Equal(t, "wh1", cfg.WebhookConfigs.Global.Webhooks)
	assert.Equal(t, "myhost.com", cfg.WebhookConfigs.Global.AllowedHosts)
	// Other fields should remain at defaults
	assert.False(t, cfg.WebhookConfigs.Global.AllowRemotePresets)
}

func TestApplyGlobalWebhookLabels_NilWebhookConfigs_Coverage(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(test.NewTestLogger())
	cfg.WebhookConfigs = nil

	globals := map[string]any{
		"preset-cache-ttl": "30m",
		"preset-cache-dir": "/my/cache",
	}

	applyGlobalWebhookLabels(cfg, globals)

	assert.NotNil(t, cfg.WebhookConfigs)
	assert.Equal(t, 30*time.Minute, cfg.WebhookConfigs.Global.PresetCacheTTL)
	assert.Equal(t, "/my/cache", cfg.WebhookConfigs.Global.PresetCacheDir)
}

func TestApplyGlobalWebhookLabels_InvalidTypes_Coverage(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(test.NewTestLogger())
	globals := map[string]any{
		"webhooks":             123,   // int, not string - should be ignored
		"allow-remote-presets": false, // bool, not string - should be ignored
		"preset-cache-dir":     42,    // int, not string - should be ignored
	}

	applyGlobalWebhookLabels(cfg, globals)

	// Should keep defaults since types don't match
	assert.Empty(t, cfg.WebhookConfigs.Global.Webhooks)
}

// --- parseWebhookName ---

func TestParseWebhookName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"double quotes", `webhook "my-hook"`, "my-hook"},
		{"single quotes", `webhook 'my-hook'`, "my-hook"},
		{"no quotes", "webhook bare-name", "bare-name"},
		{"just prefix", "webhook", ""},
		{"with extra spaces", `webhook  "spaced"`, "spaced"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseWebhookName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- mergeWebhookConfigs ---

func TestMergeWebhookConfigs_GlobalMerge_NoOverrideExisting(t *testing.T) {
	t.Parallel()

	logger := test.NewTestLogger()
	cfg := NewConfig(logger)
	cfg.WebhookConfigs = NewWebhookConfigs()
	cfg.WebhookConfigs.Global.Webhooks = "existing-wh"
	cfg.WebhookConfigs.Global.AllowedHosts = "specific.com"

	parsed := NewWebhookConfigs()
	parsed.Global.Webhooks = "label-wh"
	parsed.Global.AllowedHosts = "label.com"
	parsed.Webhooks["new-hook"] = &middlewares.WebhookConfig{
		Name: "new-hook",
		URL:  "https://label.example.com",
	}

	mergeWebhookConfigs(cfg, parsed)

	// Existing global settings should NOT be overridden
	assert.Equal(t, "existing-wh", cfg.WebhookConfigs.Global.Webhooks)
	assert.Equal(t, "specific.com", cfg.WebhookConfigs.Global.AllowedHosts)
	// But new webhook should still be added
	assert.Contains(t, cfg.WebhookConfigs.Webhooks, "new-hook")
}

// --- applyWebhookLabelParams (case insensitivity) ---

func TestApplyWebhookLabelParams_CaseInsensitive(t *testing.T) {
	t.Parallel()

	config := middlewares.DefaultWebhookConfig()
	params := map[string]string{
		"PRESET":    "discord",
		"URL":       "https://discord.example.com",
		"Trigger":   "always",
		"LINK":      "https://example.com",
		"LINK-TEXT": "Click Here",
	}

	applyWebhookLabelParams(config, params)

	assert.Equal(t, "discord", config.Preset)
	assert.Equal(t, "https://discord.example.com", config.URL)
	assert.Equal(t, middlewares.TriggerType("always"), config.Trigger)
	assert.Equal(t, "https://example.com", config.Link)
	assert.Equal(t, "Click Here", config.LinkText)
}
