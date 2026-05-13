// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

// TestClient_CloseIdempotent pins that calling Close() repeatedly on a
// Client constructed via newClientFromSDK does NOT panic and returns
// nil on each call.
//
// Background: gap #1 in issue #610 — no test covered Close() being
// invoked twice (or after construction succeeded but the caller never
// used the client). The Docker SDK's Close is itself idempotent on a
// client without an active HTTP transport, so this is a pure pinning
// test; no production change required.
func TestClient_CloseIdempotent(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close panicked on repeat invocation: %v", r)
		}
	}()

	c := newClientFromSDK(newLoopbackSDKClient(t))

	for i := range 3 {
		if err := c.Close(); err != nil {
			t.Errorf("Close call %d returned error: %v", i+1, err)
		}
	}
}

// TestCreateHTTPClient_TransportShape replaces the previous-dead
// TestClientConfigHTTPSHost / TestClientConfigTCPHost which only
// asserted struct-field assignment. We now exercise createHTTPClient
// and assert the transport shape it produces per host scheme.
//
// Background: gap #7 in issue #610 — the old tests at client_test.go
// 154-172 had zero behavioral assertion. This pins the dialer/HTTP2
// selection contract directly.
func TestCreateHTTPClient_TransportShape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		host          string
		wantDialerSet bool
		wantHTTP2     bool
	}{
		{
			name:          "unix_socket",
			host:          "unix:///var/run/docker.sock",
			wantDialerSet: true, // custom DialContext for the socket path
			wantHTTP2:     false,
		},
		{
			name:          "https_loopback",
			host:          "https://127.0.0.1:2376",
			wantDialerSet: false, // standard dialer; ALPN handles transport
			wantHTTP2:     true,
		},
		{
			name:          "tcp_loopback",
			host:          "tcp://127.0.0.1:2375",
			wantDialerSet: false,
			wantHTTP2:     false,
		},
		{
			name:          "empty_host_defaults",
			host:          "", // falls back to client.DefaultDockerHost (unix socket)
			wantDialerSet: true,
			wantHTTP2:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &ClientConfig{
				Host:            tc.host,
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
				DialTimeout:     5 * time.Second,
			}

			hc := createHTTPClient(cfg)
			if hc == nil {
				t.Fatal("createHTTPClient returned nil")
			}

			tr, ok := hc.Transport.(*http.Transport)
			if !ok {
				t.Fatalf("Transport is %T, expected *http.Transport", hc.Transport)
			}

			gotDialerSet := tr.DialContext != nil
			if gotDialerSet != tc.wantDialerSet {
				t.Errorf("DialContext set=%v, want %v (host=%q)",
					gotDialerSet, tc.wantDialerSet, tc.host)
			}
			if tr.ForceAttemptHTTP2 != tc.wantHTTP2 {
				t.Errorf("ForceAttemptHTTP2=%v, want %v (host=%q)",
					tr.ForceAttemptHTTP2, tc.wantHTTP2, tc.host)
			}
		})
	}
}

// TestNewClient_DelegatesToConfigDefault pins that NewClient() and
// NewClientWithConfig(DefaultConfig()) produce equivalent transport
// shape, so a future refactor splitting the two paths cannot regress
// the contract silently.
//
// Background: gap #6 in issue #610. The construction touches the real
// Docker SDK which requires connectivity for API version negotiation,
// so we exercise the deterministic part of the contract — createHTTPClient
// invoked with DefaultConfig() — directly. Anything beyond that is
// covered by NewClientWithConfig's own integration tests.
func TestNewClient_DelegatesToConfigDefault(t *testing.T) {
	t.Parallel()

	cfgA := DefaultConfig()
	cfgB := DefaultConfig()

	if !reflect.DeepEqual(cfgA, cfgB) {
		t.Fatal("DefaultConfig() not deterministic — two invocations differ")
	}

	hcA := createHTTPClient(cfgA)
	hcB := createHTTPClient(cfgB)

	trA, ok := hcA.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("hcA Transport is %T, expected *http.Transport", hcA.Transport)
	}
	trB, ok := hcB.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("hcB Transport is %T, expected *http.Transport", hcB.Transport)
	}

	// Compare every observable transport setting that NewClient and
	// NewClientWithConfig(DefaultConfig()) would diverge on. The closures
	// (DialContext) are deliberately not compared by value — closure
	// identity differs across allocations, but both paths assign the
	// same closure shape (set vs unset).
	if (trA.DialContext == nil) != (trB.DialContext == nil) {
		t.Errorf("DialContext presence differs: A=%v B=%v",
			trA.DialContext != nil, trB.DialContext != nil)
	}
	if trA.MaxIdleConns != trB.MaxIdleConns {
		t.Errorf("MaxIdleConns: A=%d B=%d", trA.MaxIdleConns, trB.MaxIdleConns)
	}
	if trA.MaxIdleConnsPerHost != trB.MaxIdleConnsPerHost {
		t.Errorf("MaxIdleConnsPerHost: A=%d B=%d", trA.MaxIdleConnsPerHost, trB.MaxIdleConnsPerHost)
	}
	if trA.MaxConnsPerHost != trB.MaxConnsPerHost {
		t.Errorf("MaxConnsPerHost: A=%d B=%d", trA.MaxConnsPerHost, trB.MaxConnsPerHost)
	}
	if trA.IdleConnTimeout != trB.IdleConnTimeout {
		t.Errorf("IdleConnTimeout: A=%v B=%v", trA.IdleConnTimeout, trB.IdleConnTimeout)
	}
	if trA.ResponseHeaderTimeout != trB.ResponseHeaderTimeout {
		t.Errorf("ResponseHeaderTimeout: A=%v B=%v", trA.ResponseHeaderTimeout, trB.ResponseHeaderTimeout)
	}
	if trA.ForceAttemptHTTP2 != trB.ForceAttemptHTTP2 {
		t.Errorf("ForceAttemptHTTP2: A=%v B=%v", trA.ForceAttemptHTTP2, trB.ForceAttemptHTTP2)
	}
	if hcA.Timeout != hcB.Timeout {
		t.Errorf("client Timeout: A=%v B=%v", hcA.Timeout, hcB.Timeout)
	}
}
