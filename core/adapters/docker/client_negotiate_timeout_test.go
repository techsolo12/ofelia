// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// TestNewClientWithConfig_NegotiateAPIVersionTimeout verifies that NewClientWithConfig
// returns within a bounded time when the Docker daemon is reachable but does not
// respond to API version negotiation (e.g. a wedged socket proxy whose upstream is hung).
//
// Regression for https://github.com/netresearch/ofelia/issues/608. Without
// NegotiateTimeout, sdk.NegotiateAPIVersion is invoked with context.Background() and
// can block forever, hanging Ofelia at startup with no diagnostic output.
func TestNewClientWithConfig_NegotiateAPIVersionTimeout(t *testing.T) {
	// Loopback listener that accepts connections but never reads/writes -
	// requests to /_ping will hang on the response. This mirrors a wedged
	// socket-proxy whose upstream is unresponsive.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}

	// Drain accepted connections in a goroutine but never serve them.
	// Hold accepted conns under a mutex so we can close them all on cleanup
	// without racing with the Accept goroutine. Closing ln makes Accept
	// return an error and the goroutine exits.
	var (
		connsMu sync.Mutex
		conns   []net.Conn
	)
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			c, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			connsMu.Lock()
			conns = append(conns, c)
			connsMu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-acceptDone
		connsMu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		connsMu.Unlock()
	})

	host := "tcp://" + ln.Addr().String()

	// Point DOCKER_HOST at our stalling listener. t.Setenv restores after the test.
	t.Setenv("DOCKER_HOST", host)
	// Make sure no TLS env vars from the developer environment leak in.
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	const negotiateTimeout = 200 * time.Millisecond
	// Generous tolerance to avoid CI flakes: expect return within 5x the configured timeout.
	const returnWithin = 5 * negotiateTimeout
	// Hard safety net: if even the timeout path hangs, fail the test rather than the suite.
	const hardDeadline = 5 * time.Second

	cfg := DefaultConfig()
	cfg.Host = host
	cfg.NegotiateTimeout = negotiateTimeout

	type result struct {
		client *Client
		err    error
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		c, e := NewClientWithConfig(cfg)
		done <- result{client: c, err: e}
	}()

	select {
	case r := <-done:
		elapsed := time.Since(start)
		if r.client != nil {
			_ = r.client.Close()
		}
		// We do not require an error - NegotiateAPIVersion swallows ping
		// failures silently. The point is that the call must return.
		if elapsed > returnWithin {
			t.Fatalf("NewClientWithConfig returned after %v, expected <= %v (configured timeout %v)",
				elapsed, returnWithin, negotiateTimeout)
		}
		// Sanity check: errors from client.NewClientWithOpts (e.g. bad host) are fine to surface,
		// but a network ping error should not be propagated by NegotiateAPIVersion.
		if r.err != nil && !errors.Is(r.err, context.DeadlineExceeded) {
			t.Logf("NewClientWithConfig returned err=%v (ignored - timeout path is what matters)", r.err)
		}
	case <-time.After(hardDeadline):
		t.Fatalf("NewClientWithConfig did not return within %v (configured NegotiateTimeout=%v); likely hanging on NegotiateAPIVersion",
			hardDeadline, negotiateTimeout)
	}
}

// TestDefaultConfig_NegotiateTimeout asserts the default configuration ships with a
// sane, non-zero NegotiateTimeout so that operators who use DefaultConfig() inherit
// the hang-prevention without explicit opt-in.
func TestDefaultConfig_NegotiateTimeout(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.NegotiateTimeout <= 0 {
		t.Fatalf("DefaultConfig().NegotiateTimeout = %v, want > 0", cfg.NegotiateTimeout)
	}
	if cfg.NegotiateTimeout > 2*time.Minute {
		t.Fatalf("DefaultConfig().NegotiateTimeout = %v, want a sane upper bound (<=2m)", cfg.NegotiateTimeout)
	}
}
