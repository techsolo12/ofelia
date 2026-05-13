// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core"
	"github.com/netresearch/ofelia/middlewares"
	"github.com/netresearch/ofelia/test"
)

// TestIniConfigUpdate_WebhookAllowedHosts_LiveReload_EnforcementRefreshes is the
// regression test required by issue #620: when an operator tightens
// webhook-allowed-hosts at runtime via INI live-reload, the URL validator must
// refuse hosts that are no longer in the whitelist — without any explicit sync
// call between the embedded WebhookGlobalConfig and the WebhookConfigs.Global
// store, and without the operator restarting the daemon.
//
// This test deliberately exercises BOTH halves of the dual-store collapse:
//  1. The data store: c.WebhookConfigs.Global must reflect the new value
//     (verified via direct read).
//  2. The enforcement: middlewares.ValidateWebhookURL must return an error for
//     a host outside the new whitelist (verified via the package-level validator
//     that NewWebhookManager wires up via SetGlobalSecurityConfig).
//
// Marked non-parallel: the URL validator lives in a package-global, so this
// must serialize against any other test that mutates SetGlobalSecurityConfig
// or SetValidateWebhookURLForTest.
func TestIniConfigUpdate_WebhookAllowedHosts_LiveReload_EnforcementRefreshes(t *testing.T) {
	// Snapshot and restore the entire package-global webhook security state.
	// NewWebhookManager calls SetGlobalSecurityConfig, which mutates BOTH the
	// URL validator AND the transport factory. Restoring only the validator
	// would leak the configured transport factory into other tests; passing
	// nil to SetGlobalSecurityConfig restores both to their package defaults.
	t.Cleanup(func() {
		middlewares.SetGlobalSecurityConfig(nil)
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")

	// Initial config: AllowedHosts="*" (allow all) plus a webhook whose name is
	// referenced from a job, so InitManager() will be called from BuildFromFile
	// → InitializeApp would normally do that, but the test cuts that path. We
	// drive InitManager() directly to mirror what the production startup path
	// would have set up.
	initial := `[global]
webhook-allowed-hosts = *
[webhook "alert"]
url = https://hooks.slack.com/services/ABC
trigger = on-error
`
	require.NoError(t, os.WriteFile(configPath, []byte(initial), 0o644))

	logger := test.NewTestLogger()
	config, err := BuildFromFile(configPath, logger)
	require.NoError(t, err)

	// Sanity check the dual-store-collapse invariant: the manager's globalConfig
	// pointer, the embedded struct, and c.WebhookConfigs.Global all alias.
	require.Same(t, &config.Global.WebhookGlobalConfig, config.WebhookConfigs.Global,
		"WebhookConfigs.Global must alias the embedded WebhookGlobalConfig")
	require.Equal(t, "*", config.WebhookConfigs.Global.AllowedHosts)

	// Mirror the InitializeApp wiring the test bypassed.
	config.sh = core.NewScheduler(logger)
	config.dockerHandler = &DockerHandler{ctx: context.Background(), logger: logger}
	config.configPath = configPath
	config.levelVar = &slog.LevelVar{}
	require.NoError(t, config.WebhookConfigs.InitManager(),
		"webhook manager must initialize so SetGlobalSecurityConfig wires up the URL validator")
	config.buildSchedulerMiddlewares(config.sh)

	// Baseline: with AllowedHosts="*", any host is permitted.
	require.NoError(t, middlewares.ValidateWebhookURL("https://forbidden.example.com/path"),
		"baseline: with AllowedHosts=*, all hosts must be allowed")

	// Operator tightens the whitelist via INI: only hooks.slack.com is permitted.
	tightened := `[global]
webhook-allowed-hosts = hooks.slack.com
[webhook "alert"]
url = https://hooks.slack.com/services/ABC
trigger = on-error
`
	// Make sure the file appears changed.
	config.configModTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.WriteFile(configPath, []byte(tightened), 0o644))

	require.NoError(t, config.iniConfigUpdate())

	// (1) The data store must reflect the new value — via the alias, no sync call.
	assert.Equal(t, "hooks.slack.com", config.WebhookConfigs.Global.AllowedHosts,
		"after live-reload, c.WebhookConfigs.Global.AllowedHosts must reflect the new INI value via the embedded-struct alias")

	// (2) Enforcement must be refreshed — the validator must now reject hosts
	// outside the new whitelist. This is what fails when iniConfigUpdate forgets
	// to re-call InitManager().
	require.NoError(t, middlewares.ValidateWebhookURL("https://hooks.slack.com/services/X"),
		"whitelisted host must still be allowed after live-reload")
	require.Error(t, middlewares.ValidateWebhookURL("https://forbidden.example.com/path"),
		"after live-reload tightens webhook-allowed-hosts, the URL validator must reject hosts outside the new whitelist (#620)")
}
