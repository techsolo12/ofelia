// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/test"
)

// TestNewScratchConfig_AliasesWebhookGlobal pins the invariant that scratch
// Config instances created by newScratchConfig mirror NewConfig's pointer
// alias between Global.WebhookGlobalConfig (the embedded struct that
// mapstructure decodes into) and WebhookConfigs.Global (the live store the
// webhook subsystem reads from).
//
// Without this alias, the Docker label sync path silently drops webhook
// global values: decodeWithMetadata writes into Global.WebhookGlobalConfig
// while every consumer reads from WebhookConfigs.Global, leaving consumers
// pinned to the defaults seeded by NewWebhookConfigs(). This is the
// underlying defect tracked as #641 and the prerequisite for #640's
// PresetCacheTTL forwarding.
func TestNewScratchConfig_AliasesWebhookGlobal(t *testing.T) {
	t.Parallel()
	c := NewConfig(test.NewTestLogger())
	scratch := newScratchConfig(c)

	require.NotNil(t, scratch)
	require.NotNil(t, scratch.WebhookConfigs)
	require.Same(t, &scratch.Global.WebhookGlobalConfig, scratch.WebhookConfigs.Global,
		"scratch.WebhookConfigs.Global must alias the embedded WebhookGlobalConfig — same invariant as NewConfig")

	scratch.Global.WebhookGlobalConfig.PresetCacheTTL = 99 * time.Minute
	assert.Equal(t, 99*time.Minute, scratch.WebhookConfigs.Global.PresetCacheTTL,
		"alias must propagate writes from Global.WebhookGlobalConfig to WebhookConfigs.Global")

	scratch.WebhookConfigs.Global.AllowedHosts = "example.com"
	assert.Equal(t, "example.com", scratch.Global.WebhookGlobalConfig.AllowedHosts,
		"alias must propagate writes in the reverse direction too")
}

// TestNewScratchConfig_CarriesGlobalAndLogger verifies that the scratch
// helper inherits the live Config's Global settings (so AllowHostJobsFromLabels
// and friends still gate label decoding) and shares the same logger so warnings
// surface on the same writer as the rest of the daemon.
func TestNewScratchConfig_CarriesGlobalAndLogger(t *testing.T) {
	t.Parallel()
	c := NewConfig(test.NewTestLogger())
	c.Global.AllowHostJobsFromLabels = true
	c.Global.WebhookGlobalConfig.AllowedHosts = "configured.example"

	scratch := newScratchConfig(c)

	assert.True(t, scratch.Global.AllowHostJobsFromLabels,
		"scratch must inherit AllowHostJobsFromLabels from the live config")
	assert.Equal(t, "configured.example", scratch.Global.WebhookGlobalConfig.AllowedHosts,
		"scratch must inherit the embedded WebhookGlobalConfig values")
	assert.Equal(t, "configured.example", scratch.WebhookConfigs.Global.AllowedHosts,
		"alias must surface inherited values via WebhookConfigs.Global as well")
	assert.Same(t, c.logger, scratch.logger,
		"scratch must share the live logger so warnings surface in the same destination")
}
