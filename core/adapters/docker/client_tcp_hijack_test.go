// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

// TestPlainTCPHijack_ContainerExecAttachWorks pins the fix for
// https://github.com/netresearch/ofelia/issues/668.
//
// Repro: with DOCKER_HOST=tcp://socket-proxy:2375 (plain HTTP, no TLS) the
// regular HTTP path works but ContainerExecAttach (the hijack path used by
// ofelia's run_exec) failed with
//
//	tls: first record does not look like a TLS handshake
//
// Root cause: Go's net/http lazily auto-configures HTTP/2 on the first
// request and ALLOCATES Transport.TLSClientConfig in place (to seed
// NextProtos=[h2 http/1.1] for ALPN). The Docker SDK's hijack dialer reads
// baseTransport.TLSClientConfig as its "TLS is required" signal, so after
// the first plain-HTTP request lands the hijack path calls tls.Dial against
// a plain Docker daemon and the handshake fails.
//
// The fix sets transport.TLSNextProto to a non-nil empty map on non-TLS
// apply paths (disableHTTP2AutoConfig), which is the documented stdlib
// contract for "don't auto-enable HTTP/2". This test drives the full hijack
// path against a plain HTTP TCP listener that issues 101 Switching
// Protocols, asserting ContainerExecAttach returns cleanly.
//
// This is the exact-shape regression test the existing client_tcp_upgrade
// suite lacked — DaemonHost() assertions never exercise the hijack dialer.
func TestPlainTCPHijack_ContainerExecAttachWorks(t *testing.T) {
	srv, addr := newFakePlainDockerProxy(t)
	t.Cleanup(func() { _ = srv.Close() })

	cfg := DefaultConfig()
	cfg.Host = "tcp://" + addr
	cfg.NegotiateTimeout = 500 * time.Millisecond

	c, err := NewClientWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientWithConfig: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	// Force the regular HTTP path to run first so we trigger Go's lazy
	// http2 setup that previously mutated TLSClientConfig in place. Without
	// this priming step a naive impl could still pass the hijack call by
	// chance.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.SDK().Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// Now exercise the hijack path. With the fix this returns cleanly;
	// without it this fails with "tls: first record does not look like a TLS
	// handshake".
	resp, err := c.SDK().ContainerExecAttach(ctx, "deadbeef", containertypes.ExecAttachOptions{})
	if err != nil {
		t.Fatalf("ContainerExecAttach over plain tcp://: %v", err)
	}
	defer resp.Close()
	_, _ = io.Copy(io.Discard, resp.Reader)
}

// Note on http:// scope: the http:// hijack path is broken in a separate,
// pre-existing way that this fix does NOT address. The SDK's default-case
// dialer falls to net.Dial(cli.proto, cli.addr); for proto == "http" that
// is net.Dial("http", addr), which Go's net package rejects with "unknown
// network http". Fixing that requires installing a TCP DialContext for the
// http:// transport and is filed as a separate enhancement.

// newFakePlainDockerProxy stands up a minimal plain-HTTP TCP listener that
// mimics tecnativa/docker-socket-proxy enough to drive ContainerExecAttach
// down the SDK's hijack path. Returns the server and its address.
func newFakePlainDockerProxy(t *testing.T) (*http.Server, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{
		Handler:           fakePlainDockerHandler(),
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			t.Logf("fake proxy serve: %v", serveErr)
		}
	}()
	return srv, ln.Addr().String()
}

// fakePlainDockerHandler answers the minimum routes the Docker SDK touches
// during client setup + ContainerExecAttach: _ping, exec create, exec start
// (hijack).
func fakePlainDockerHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/_ping"):
			w.Header().Set("API-Version", "1.43")
			w.Header().Set("OSType", "linux")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		case strings.HasSuffix(path, "/exec") && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"deadbeef"}`))
		case strings.Contains(path, "/exec/") && strings.HasSuffix(path, "/start") && r.Method == http.MethodPost:
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "no hijacker", http.StatusInternalServerError)
				return
			}
			conn, _, hjErr := hj.Hijack()
			if hjErr != nil {
				return
			}
			defer conn.Close()
			_, _ = conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}
	})
}
