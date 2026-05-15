// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
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
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	cfg := DefaultConfig()
	cfg.Host = "tcp://" + addr
	cfg.NegotiateTimeout = 500 * time.Millisecond

	c, err := NewClientWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientWithConfig: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	// Prime: force the regular HTTP path to run first so we trigger Go's
	// lazy http2 setup that previously mutated TLSClientConfig in place.
	// Without this priming step a naive impl could still pass the hijack
	// call by chance. The bug requires *first* ordinary request -> hijack.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, pingErr := c.SDK().Ping(ctx); pingErr != nil {
		t.Fatalf("Ping: %v", pingErr)
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

// TestPlainHTTPHijack_ContainerExecAttachWorks pins the fix for
// https://github.com/netresearch/ofelia/issues/682. Sibling bug to #668.
//
// Pre-fix, http:// hijack failed with "dial http: unknown network http"
// because the SDK's default-case dialer fell to net.Dial("http", addr),
// which Go's net package rejects ("http" is not a valid network name).
// The fix installs an explicit TCP DialContext on the http:// transport
// (applyHTTPTransport) so the SDK picks our dialer via dialerFromTransport
// and never reaches the broken fallback.
//
// Same plain-HTTP TCP listener as the tcp:// test — only the host scheme
// changes — so we exercise exactly the path #682 reported.
func TestPlainHTTPHijack_ContainerExecAttachWorks(t *testing.T) {
	srv, addr := newFakePlainDockerProxy(t)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	cfg := DefaultConfig()
	cfg.Host = "http://" + addr
	cfg.NegotiateTimeout = 500 * time.Millisecond

	c, err := NewClientWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientWithConfig: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, pingErr := c.SDK().Ping(ctx); pingErr != nil {
		t.Fatalf("Ping: %v", pingErr)
	}

	resp, err := c.SDK().ContainerExecAttach(ctx, "deadbeef", containertypes.ExecAttachOptions{})
	if err != nil {
		t.Fatalf("ContainerExecAttach over plain http://: %v", err)
	}
	defer resp.Close()
	_, _ = io.Copy(io.Discard, resp.Reader)
}

// TestDisableHTTP2AutoConfig_KeepsTLSClientConfigNil is a focused unit test
// on the helper. It documents the underlying stdlib contract directly: a
// transport with TLSNextProto set to a non-nil (empty) map must NOT have
// TLSClientConfig auto-mutated on the first request.
//
// Without this, a future stdlib refactor that changes the auto-config
// trigger could silently re-enable the #668 bug while the end-to-end
// regression test still passes (because the SDK might pick a different
// dial path). This test fails fast and on the right layer.
func TestDisableHTTP2AutoConfig_KeepsTLSClientConfigNil(t *testing.T) {
	ts := httpTestServer(t)
	t.Cleanup(func() { _ = ts.Shutdown(context.Background()) })

	transport := &http.Transport{}
	disableHTTP2AutoConfig(transport)
	httpClient := &http.Client{Transport: transport, Timeout: 2 * time.Second}

	// Trigger Go's lazy onceSetNextProtoDefaults via a real request.
	resp, err := httpClient.Get("http://" + ts.Addr)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = resp.Body.Close()

	if transport.TLSClientConfig != nil {
		t.Fatalf("disableHTTP2AutoConfig did not suppress lazy http2 setup: TLSClientConfig = %+v (expected nil)", transport.TLSClientConfig)
	}
}

// TestApplyTLSTransport_DoesNotDisableHTTP2AutoConfig is the negative
// control for the #668 fix: TLS apply paths (https://, tcp+tls://) MUST
// leave TLSNextProto nil so ALPN h2 negotiation still works. If a refactor
// accidentally adds disableHTTP2AutoConfig to the TLS path, HTTPS Docker
// hosts silently lose HTTP/2.
func TestApplyTLSTransport_DoesNotDisableHTTP2AutoConfig(t *testing.T) {
	transport := &http.Transport{}
	applyTLSTransport(transport, DefaultConfig(), "https://example.invalid:2376")

	if transport.TLSNextProto != nil {
		t.Fatalf("applyTLSTransport set TLSNextProto (got %v); TLS paths must keep it nil so ALPN h2 negotiation works", transport.TLSNextProto)
	}
	if !transport.ForceAttemptHTTP2 {
		t.Fatalf("applyTLSTransport must set ForceAttemptHTTP2=true (got false)")
	}
}

// TestApplyTransport_NonTLSDisableHTTP2AutoConfig asserts the inverse:
// every non-TLS apply path MUST suppress HTTP/2 auto-config. Adding a new
// non-TLS scheme handler without the call would silently re-introduce
// #668; this table pins the invariant.
func TestApplyTransport_NonTLSDisableHTTP2AutoConfig(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*http.Transport, *ClientConfig, string)
		host  string
	}{
		{"unix", applyUnixTransport, "unix:///var/run/docker.sock"},
		{"tcp", applyTCPTransport, "tcp://127.0.0.1:2375"},
		{"http", applyHTTPTransport, "http://127.0.0.1:2375"},
		{"plain", applyPlainTransport, "npipe://./pipe/docker_engine"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &http.Transport{}
			tc.apply(transport, DefaultConfig(), tc.host)
			if transport.TLSNextProto == nil {
				t.Fatalf("apply%s did not call disableHTTP2AutoConfig — silent regression risk for #668", tc.name)
			}
		})
	}
}

// addrServer wraps an *http.Server with its listen address. Lets cleanup
// route through Shutdown so the serve goroutine exits before t.Logf can
// race with test completion.
type addrServer struct {
	*http.Server
	Addr string
	done <-chan struct{}
}

func (a *addrServer) Shutdown(ctx context.Context) error {
	err := a.Server.Shutdown(ctx)
	// Wait for the serve goroutine to exit so no late t.Log races test end.
	if a.done != nil {
		select {
		case <-a.done:
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
	return err
}

// newFakePlainDockerProxy stands up a minimal plain-HTTP TCP listener that
// mimics tecnativa/docker-socket-proxy enough to drive ContainerExecAttach
// down the SDK's hijack path.
func newFakePlainDockerProxy(t *testing.T) (*addrServer, string) {
	t.Helper()
	return startTestServer(t, fakePlainDockerHandler())
}

// httpTestServer is a generic plain-HTTP listener used by the helper-only
// unit test that just needs to fire one round-trip.
func httpTestServer(t *testing.T) *addrServer {
	t.Helper()
	srv, _ := startTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	return srv
}

// startTestServer binds 127.0.0.1:0, serves handler in a goroutine, and
// returns an addrServer whose Shutdown synchronizes with the serve loop
// to avoid t.Logf races at test exit.
func startTestServer(t *testing.T, handler http.Handler) (*addrServer, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	var once sync.Once
	srv := &addrServer{
		Server: &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 2 * time.Second,
		},
		Addr: ln.Addr().String(),
		done: done,
	}
	go func() {
		defer once.Do(func() { close(done) })
		if serveErr := srv.Server.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			// Best-effort log; the t.Helper + done sync above gates this
			// before test cleanup completes.
			t.Logf("fake proxy serve: %v", serveErr)
		}
	}()
	return srv, srv.Addr
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

// Keep tls package imported (used by helper signature). Empty struct used
// rather than _ = tls.Config{} so the import stays without a dead value.
var _ tls.Conn
