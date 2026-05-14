// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"errors"
	"strings"
	"testing"
)

// ErrHTTPSRequiresUsableCertMaterial is the typed sentinel returned by
// NewClientWithConfig when DOCKER_HOST uses the explicit-TLS https:// scheme
// AND TLS material is configured (DOCKER_CERT_PATH or
// ClientConfig.TLSCertPath) but resolveTLSConfig fails to load it.
//
// Asymmetry vs tcp+tls://: tcp+tls:// REQUIRES TLS material (fail-closed even
// when nothing is set, because the scheme is an explicit mTLS opt-in).
// https:// is a TLS scheme but not necessarily an mTLS scheme — operators may
// legitimately rely on the system CA bundle without configuring a client
// cert. Only when material IS configured but unloadable do we fail closed,
// because that is the operator-believes-mTLS-is-on-but-it-isn't case (the
// silent-downgrade pattern from issue #653).

// TestNewClientWithConfig_HTTPSRejectsUnreadableCertPath pins the fail-closed
// behavior for https:// when the operator HAS configured TLS material but it
// is unreadable. Mirrors the tcp+tls:// counterpart in
// TestNewClientWithConfig_TCPPlusTLSRejectsUnreadableCertPath.
//
// Without this gate, applyDockerTLS would emit slog.Warn and leave
// TLSClientConfig nil; the SDK would then dial with Go's default TLS — the
// system CA pool, NO client certificate — silently downgrading what the
// operator declared as mTLS into an unauthenticated TLS handshake.
//
// Not parallel: mutates DOCKER_HOST / DOCKER_TLS_VERIFY / DOCKER_CERT_PATH
// via t.Setenv to prevent ambient env from leaking in.
func TestNewClientWithConfig_HTTPSRejectsUnreadableCertPath(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	// Empty directory — resolveTLSConfig will fail on the missing
	// cert.pem / key.pem, while hasTLSMaterial still reports "configured".
	emptyDir := t.TempDir()
	t.Setenv("DOCKER_CERT_PATH", emptyDir)

	_, err := NewClientWithConfig(&ClientConfig{Host: "https://docker.example:2376"})
	if err == nil {
		t.Fatal("expected error for https:// with unreadable cert path, got nil")
	}
	if !errors.Is(err, ErrHTTPSRequiresUsableCertMaterial) {
		t.Fatalf("expected error to wrap ErrHTTPSRequiresUsableCertMaterial, got %v", err)
	}
	if !strings.Contains(err.Error(), "TROUBLESHOOTING") {
		t.Errorf("expected error to reference docs/TROUBLESHOOTING.md, got %q", err.Error())
	}
}

// TestNewClientWithConfig_HTTPSAllowsNoCertMaterial confirms the asymmetry
// vs tcp+tls://: https:// without ANY TLS material configured is FINE
// (operator relies on system CA). Construction must NOT return an
// ErrHTTPSRequiresUsableCertMaterial when no material is set; this is the
// well-defined "use system trust store" path of the upstream SDK.
func TestNewClientWithConfig_HTTPSAllowsNoCertMaterial(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	_, err := NewClientWithConfig(&ClientConfig{Host: "https://docker.example:2376"})
	if errors.Is(err, ErrHTTPSRequiresUsableCertMaterial) {
		t.Fatalf("https:// without TLS material must not return ErrHTTPSRequiresUsableCertMaterial: %v", err)
	}
	// Other errors (dial / negotiate failure) are acceptable; this test
	// pins only that the gate does not over-fire.
}

// TestNewClientWithConfig_HTTPSAcceptsValidCertMaterial is the positive
// counterpart: when usable TLS material IS present via ClientConfig, the
// gate must not fire.
func TestNewClientWithConfig_HTTPSAcceptsValidCertMaterial(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	certPath := writeFakeTLSMaterial(t)

	_, err := NewClientWithConfig(&ClientConfig{
		Host:        "https://docker.example:2376",
		TLSCertPath: certPath,
	})
	if errors.Is(err, ErrHTTPSRequiresUsableCertMaterial) {
		t.Fatalf("usable cert material was provided, must not return ErrHTTPSRequiresUsableCertMaterial: %v", err)
	}
}
