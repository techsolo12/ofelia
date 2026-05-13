// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"testing"
	"time"
)

// TestUpgradeTCPToHTTPSIfTLSMaterial covers the helper that closes the
// docker-CLI parity gap from issue
// https://github.com/netresearch/ofelia/issues/634:
//
// Setting DOCKER_HOST=tcp://… alongside DOCKER_TLS_VERIFY/DOCKER_CERT_PATH
// previously left the SDK pointed at a tcp:// URL. Go's http.Transport only
// triggers TLS on https://, so the cert material wired into the custom
// transport (issue #613) was silently unused. Mirror the docker CLI: when
// TLS material is configured, rewrite tcp:// to https:// so the SDK and
// transport agree.
//
// Other schemes are passed through unchanged — explicit tcp+tls:// keeps its
// own dispatch handler, https:// is already correct, and unix:// / http://
// have no TLS implication here.
func TestUpgradeTCPToHTTPSIfTLSMaterial(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		certEnv string // value for DOCKER_CERT_PATH
		cfg     *ClientConfig
		want    string
	}{
		{
			name:    "tcp upgrades when DOCKER_CERT_PATH set",
			host:    "tcp://example:2376",
			certEnv: "/etc/docker/certs",
			cfg:     &ClientConfig{Host: "tcp://example:2376"},
			want:    "https://example:2376",
		},
		{
			name:    "tcp upgrades when explicit ClientConfig.TLSCertPath set",
			host:    "tcp://example:2376",
			certEnv: "",
			cfg:     &ClientConfig{Host: "tcp://example:2376", TLSCertPath: "/explicit/path"},
			want:    "https://example:2376",
		},
		{
			name:    "tcp stays plain without TLS material",
			host:    "tcp://example:2375",
			certEnv: "",
			cfg:     &ClientConfig{Host: "tcp://example:2375"},
			want:    "tcp://example:2375",
		},
		{
			name:    "tcp+tls is left alone (its own dispatch handler runs TLS)",
			host:    "tcp+tls://example:2376",
			certEnv: "/etc/docker/certs",
			cfg:     &ClientConfig{Host: "tcp+tls://example:2376"},
			want:    "tcp+tls://example:2376",
		},
		{
			name:    "https is left alone",
			host:    "https://example:2376",
			certEnv: "/etc/docker/certs",
			cfg:     &ClientConfig{Host: "https://example:2376"},
			want:    "https://example:2376",
		},
		{
			name:    "unix is left alone even with TLS env",
			host:    "unix:///var/run/docker.sock",
			certEnv: "/etc/docker/certs",
			cfg:     &ClientConfig{Host: "unix:///var/run/docker.sock"},
			want:    "unix:///var/run/docker.sock",
		},
		{
			name:    "http is left alone (caller opted into plaintext explicitly)",
			host:    "http://example:2375",
			certEnv: "/etc/docker/certs",
			cfg:     &ClientConfig{Host: "http://example:2375"},
			want:    "http://example:2375",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOCKER_CERT_PATH", tc.certEnv)
			got := upgradeTCPToHTTPSIfTLSMaterial(tc.host, tc.cfg)
			if got != tc.want {
				t.Errorf("upgradeTCPToHTTPSIfTLSMaterial(%q) = %q, want %q", tc.host, got, tc.want)
			}
		})
	}
}

// TestNewClientWithConfig_TCPSchemeUpgradesToHTTPSWithTLSEnv is the integration
// counterpart: drive the full constructor with tcp:// + TLS env material and
// assert the SDK itself ends up pointed at https://. Without the rewrite, the
// SDK would carry tcp://, and Go's http.Transport would never trigger TLS —
// the bug from https://github.com/netresearch/ofelia/issues/634.
//
// Uses cli.DaemonHost() to introspect what URL the SDK is actually using.
// NegotiateAPIVersion will fail (port 0, no real daemon), but the SDK swallows
// ping errors silently, so the constructor returns and we can inspect the
// host. The bounded NegotiateTimeout keeps the test fast.
func TestNewClientWithConfig_TCPSchemeUpgradesToHTTPSWithTLSEnv(t *testing.T) {
	certDir := t.TempDir()
	writeTLSFixtures(t, certDir)

	t.Setenv("DOCKER_CERT_PATH", certDir)
	t.Setenv("DOCKER_TLS_VERIFY", "1")

	cfg := DefaultConfig()
	cfg.Host = "tcp://127.0.0.1:0"
	cfg.NegotiateTimeout = 10 * time.Millisecond // fail fast — we never actually dial

	c, err := NewClientWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientWithConfig returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	got := c.SDK().DaemonHost()
	const want = "https://127.0.0.1:0"
	if got != want {
		t.Errorf("SDK DaemonHost = %q; want %q (tcp:// + DOCKER_TLS_VERIFY should auto-upgrade to https:// for SDK)", got, want)
	}
}

// TestNewClientWithConfig_TCPSchemeStaysPlainWithoutTLSEnv is the negative
// case: tcp:// without any TLS material must pass through to the SDK as-is.
// Mirrors TestCreateHTTPClient_TCPWithoutTLSEnvStaysPlaintext on the
// transport side; this asserts the SDK URL also stays plain.
func TestNewClientWithConfig_TCPSchemeStaysPlainWithoutTLSEnv(t *testing.T) {
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")

	cfg := DefaultConfig()
	cfg.Host = "tcp://127.0.0.1:0"
	cfg.NegotiateTimeout = 10 * time.Millisecond

	c, err := NewClientWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientWithConfig returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	got := c.SDK().DaemonHost()
	const want = "tcp://127.0.0.1:0"
	if got != want {
		t.Errorf("SDK DaemonHost = %q; want %q (tcp:// without TLS must NOT auto-upgrade)", got, want)
	}
}
