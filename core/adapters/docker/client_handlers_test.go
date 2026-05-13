// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"net/http"
	"testing"
	"time"
)

// TestSchemeHandlers_ApplyDirect exercises every entry in schemeHandlers by
// invoking its apply func directly against a fresh *http.Transport. Without
// this layer, the per-scheme apply* functions are only covered transitively
// through createHTTPClient — a refactor that breaks one scheme without
// breaking the others would slip through the existing TestCreateHTTPClient_*
// table because the breakage is masked by the other schemes still passing.
//
// The map iteration in the runner is intentional: it ensures every key in
// schemeHandlers is asserted against (a future scheme addition without a
// corresponding case here fails with the explicit "no case for scheme"
// fallthrough). See issue #633 / PR #629 for context.
func TestSchemeHandlers_ApplyDirect(t *testing.T) {
	// No t.Parallel(): subtests use t.Setenv to scrub DOCKER_CERT_PATH /
	// DOCKER_TLS_VERIFY, which is incompatible with parallel parents.

	type assertion struct {
		// host passed into apply; must match the scheme key.
		host string
		// wantHTTP2 pins the ForceAttemptHTTP2 flag the handler should set.
		wantHTTP2 bool
		// wantDialerSet asserts whether the handler installed a custom
		// DialContext (only the unix handler does).
		wantDialerSet bool
	}

	cases := map[string]assertion{
		schemeUnix: {
			host:          "unix:///var/run/docker.sock",
			wantHTTP2:     false,
			wantDialerSet: true,
		},
		schemeTCP: {
			// No TLS material in env (cleared per-test below) so tcp:// stays
			// HTTP/1.1 — the upgrade-on-DOCKER_TLS_VERIFY path is covered by
			// TestCreateHTTPClient_TCPWithTLSEnvUpgrades.
			host:          "tcp://127.0.0.1:2375",
			wantHTTP2:     false,
			wantDialerSet: false,
		},
		schemeHTTP: {
			host:          "http://127.0.0.1:2375",
			wantHTTP2:     false,
			wantDialerSet: false,
		},
		schemeHTTPS: {
			host:          "https://127.0.0.1:2376",
			wantHTTP2:     true,
			wantDialerSet: false,
		},
		schemeNpipe: {
			// npipe handler is applyPlainTransport on non-Windows builds; the
			// test asserts the no-op plain shape, not the actual dialer
			// (which is build-tagged inside the SDK).
			host:          "npipe:////./pipe/docker_engine",
			wantHTTP2:     false,
			wantDialerSet: false,
		},
		schemeTCPTLS: {
			host:          "tcp+tls://127.0.0.1:2376",
			wantHTTP2:     true,
			wantDialerSet: false,
		},
	}

	for scheme, handler := range schemeHandlers {
		want, ok := cases[scheme]
		if !ok {
			t.Errorf("no case for scheme %q in TestSchemeHandlers_ApplyDirect; "+
				"add an assertion entry for it", scheme)
			continue
		}

		t.Run(scheme, func(t *testing.T) {
			// Cannot t.Parallel() — applyTCPTransport / applyTLSTransport
			// consult DOCKER_CERT_PATH / DOCKER_TLS_VERIFY via the package
			// getenv seam; t.Setenv requires a non-parallel test.
			t.Setenv("DOCKER_CERT_PATH", "")
			t.Setenv("DOCKER_TLS_VERIFY", "")

			cfg := &ClientConfig{
				Host:            want.host,
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
				DialTimeout:     5 * time.Second,
			}

			tr := &http.Transport{}

			// Recover so a future regression that nil-derefs surfaces as a
			// test failure rather than killing the test binary.
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("handler for scheme %q panicked: %v", scheme, r)
				}
			}()

			handler.apply(tr, cfg, want.host)

			if got := tr.ForceAttemptHTTP2; got != want.wantHTTP2 {
				t.Errorf("scheme=%q ForceAttemptHTTP2=%v, want %v",
					scheme, got, want.wantHTTP2)
			}
			if gotDialer := tr.DialContext != nil; gotDialer != want.wantDialerSet {
				t.Errorf("scheme=%q DialContext set=%v, want %v",
					scheme, gotDialer, want.wantDialerSet)
			}

			// The TLS handlers may leave TLSClientConfig nil when no cert
			// material is configured (matches resolveTLSConfig's
			// "no TLS configured" sentinel) — that is fine; the explicit
			// TLS-material path is asserted by TestCreateHTTPClient_HonorsTLSEnv.
			// The plain handler must NEVER install a TLS config.
			if scheme == schemeHTTP || scheme == schemeNpipe {
				if tr.TLSClientConfig != nil {
					t.Errorf("scheme=%q installed TLSClientConfig=%+v; "+
						"plain handler must not touch TLS",
						scheme, tr.TLSClientConfig)
				}
			}
		})
	}
}

// TestCreateHTTPClient_UnknownSchemeFallback asserts the defensive branch in
// createHTTPClient: an unrecognized scheme must NOT panic and must return a
// usable *http.Client with a plain HTTP/1.1 transport. Production rejects
// such a host upstream via NewClientWithConfig (see
// ErrUnsupportedDockerHostScheme), so this test exercises only the seam —
// not the production gate. See issue #633.
func TestCreateHTTPClient_UnknownSchemeFallback(t *testing.T) {
	// Cannot t.Parallel(): touches the getenv seam through resolveHostForDispatch.
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("createHTTPClient panicked on unknown scheme: %v", r)
		}
	}()

	hc := createHTTPClient(&ClientConfig{Host: "weird://foo"})
	if hc == nil {
		t.Fatal("createHTTPClient returned nil for unknown scheme; want non-nil fallback client")
	}

	tr, ok := hc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport is %T, want *http.Transport", hc.Transport)
	}

	if tr.ForceAttemptHTTP2 {
		t.Errorf("ForceAttemptHTTP2=true for unknown scheme; "+
			"unknown-scheme fallback must stay HTTP/1.1 (transport=%+v)", tr)
	}
	if tr.TLSClientConfig != nil {
		t.Errorf("TLSClientConfig=%+v for unknown scheme; "+
			"unknown-scheme fallback must not install TLS", tr.TLSClientConfig)
	}
}
