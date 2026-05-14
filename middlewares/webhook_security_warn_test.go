// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package middlewares

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureSlog swaps slog.Default with a JSON handler writing to a buffer for
// the duration of the test. Returns the buffer; callers should run
// .String() AFTER the code under test has emitted.
//
// Not parallel-safe across tests sharing slog.Default — this helper expects
// the caller to NOT call t.Parallel().
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var (
		buf bytes.Buffer
		mu  sync.Mutex
	)
	handler := slog.NewJSONHandler(&lockedWriter{w: &buf, mu: &mu}, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

type lockedWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// TestSetGlobalSecurityConfig_WarnsOnAllowAll pins the startup-time warning
// emitted when the resolved AllowedHosts collapses to ["*"]. This catches
// the silent-egress vector tracked in https://github.com/netresearch/ofelia/issues/653:
// a typo in `webhook-allowed-hosts` (or simply leaving it unset) yields
// wide-open egress with no operator-visible signal that the allowlist they
// thought they had configured is actually empty.
//
// Single emission: the warning lives in SetGlobalSecurityConfig (the
// startup seam, called once from NewWebhookManager), NOT in
// NewWebhookSecurityValidator (called per-request) or
// SecurityConfigFromGlobal (called per-reload). One log line per startup
// is the contract.
//
// Not parallel: mutates slog.Default and the global webhook security
// config; serialized via existing test conventions.
func TestSetGlobalSecurityConfig_WarnsOnAllowAll(t *testing.T) {
	buf := captureSlog(t)
	t.Cleanup(func() { SetGlobalSecurityConfig(nil) })

	SetGlobalSecurityConfig(&WebhookSecurityConfig{
		AllowedHosts: []string{"*"},
	})

	got := buf.String()
	require.NotEmpty(t, got, "expected slog output for wide-open allow-list")
	assert.Contains(t, got, `"level":"WARN"`, "wide-open allow-list must warn at WARN level")
	assert.Contains(t, got, "egress",
		"warning must mention egress so operators understand the implication")
}

// TestSetGlobalSecurityConfig_WarnsOnEmpty covers the typo case:
// SecurityConfigFromGlobal collapses an empty (or absent / typo'd) INI key
// into ["*"]. The warning must still fire because the resolved state is
// wide-open egress.
func TestSetGlobalSecurityConfig_WarnsOnEmpty(t *testing.T) {
	buf := captureSlog(t)
	t.Cleanup(func() { SetGlobalSecurityConfig(nil) })

	// Simulate the SecurityConfigFromGlobal path: empty string → "*".
	cfg := SecurityConfigFromGlobal(&WebhookGlobalConfig{AllowedHosts: ""})
	SetGlobalSecurityConfig(cfg)

	got := buf.String()
	assert.Contains(t, got, `"level":"WARN"`)
	assert.Contains(t, got, "egress")
}

// TestSetGlobalSecurityConfig_NoWarnOnExplicitAllowList confirms the warn
// does NOT fire when operators have an actual allow-list configured. Only
// the wide-open case must trigger the warning, otherwise routine startup
// becomes noisy.
func TestSetGlobalSecurityConfig_NoWarnOnExplicitAllowList(t *testing.T) {
	buf := captureSlog(t)
	t.Cleanup(func() { SetGlobalSecurityConfig(nil) })

	SetGlobalSecurityConfig(&WebhookSecurityConfig{
		AllowedHosts: []string{"hooks.slack.com", "ntfy.internal"},
	})

	got := buf.String()
	if strings.Contains(got, "egress") {
		t.Errorf("did not expect wide-open egress warning when an explicit allow-list is configured; got: %s", got)
	}
}

// TestSetGlobalSecurityConfig_NoWarnOnNil ensures the nil-restore path used
// by tests (and by reloads that revert to defaults) does not emit the
// startup warning. This keeps test cleanup quiet and avoids double-warnings
// on reload paths.
//
// The contract: the warning is for OPERATOR-visible startup state. Nil
// resets the in-process default validator; that is not a user-meaningful
// startup event.
func TestSetGlobalSecurityConfig_NoWarnOnNil(t *testing.T) {
	buf := captureSlog(t)

	SetGlobalSecurityConfig(nil)

	got := buf.String()
	if strings.Contains(got, "egress") {
		t.Errorf("nil reset must not emit egress warning; got: %s", got)
	}
}
