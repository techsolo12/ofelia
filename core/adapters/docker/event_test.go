// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"

	"github.com/netresearch/ofelia/core/domain"
)

// TestEventServiceAdapter_Subscribe_ExitsOnError pins the documented
// contract that the goroutine spawned by Subscribe exits after the
// underlying SDK reports an error.
//
// Background: gap #5 in issue #610 hypothesized that the errCh buffer of
// 1 could block the producer on "two consecutive errors". Reading
// event.go shows the producer returns after one error (and both channels
// are closed via deferred close()). This test pins that behavior so a
// future refactor that introduces a loop on sdkErrCh would surface here.
//
// The test stands up an httptest server whose /events endpoint responds
// with HTTP 500 immediately. The SDK's Events() goroutine reports that
// as an error on the SDK error channel; our Subscribe must forward
// exactly one error, close both channels, and exit.
func TestEventServiceAdapter_Subscribe_ExitsOnError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only fail the events endpoint; everything else returns a
		// trivial 200 (the SDK pings the daemon during version negotiation
		// — we sidestep that by constructing the client without it below).
		if strings.HasSuffix(r.URL.Path, "/events") {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	// Disable API version negotiation — it would issue a /_ping the
	// stub server cannot satisfy meaningfully, and it is irrelevant to
	// the contract under test.
	sdk, err := client.NewClientWithOpts(
		client.WithHost("tcp://"+u.Host),
		client.WithHTTPClient(srv.Client()),
		client.WithVersion("1.43"),
	)
	if err != nil {
		t.Fatalf("constructing SDK client: %v", err)
	}
	t.Cleanup(func() { _ = sdk.Close() })

	adapter := &EventServiceAdapter{client: sdk}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventCh, errCh := adapter.Subscribe(ctx, domain.EventFilter{})

	// We expect (in any order, but typically): one error on errCh, then
	// both channels close. The total wait must be well under our context
	// timeout — a regression that hangs would be caught by the deadline.
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()

	var (
		gotErr        bool
		eventChClosed bool
		errChClosed   bool
	)

	for !(eventChClosed && errChClosed) {
		select {
		case _, ok := <-eventCh:
			if !ok {
				eventCh = nil // disable case
				eventChClosed = true
			}
		case e, ok := <-errCh:
			if !ok {
				errCh = nil
				errChClosed = true
				continue
			}
			gotErr = gotErr || e != nil
		case <-deadline.C:
			t.Fatalf("Subscribe goroutine did not exit within deadline (gotErr=%v, eventChClosed=%v, errChClosed=%v)",
				gotErr, eventChClosed, errChClosed)
		}
	}

	if !gotErr {
		t.Error("expected at least one error from errCh before close")
	}
}

// TestEventServiceAdapter_Subscribe_ExitsOnContextCancel pins the
// complementary contract that the producer exits cleanly when the
// caller cancels the context — no goroutine leak, channels closed.
func TestEventServiceAdapter_Subscribe_ExitsOnContextCancel(t *testing.T) {
	t.Parallel()

	// Server that holds the events connection open forever (until the
	// client cancels). This isolates the ctx.Done() exit branch.
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/events") {
			// Flush a 200 with no body, then block. The SDK will keep
			// the connection open waiting for events; our Subscribe
			// will sit in the select.
			w.WriteHeader(http.StatusOK)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			select {
			case <-r.Context().Done():
			case <-hold:
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	sdk, err := client.NewClientWithOpts(
		client.WithHost("tcp://"+u.Host),
		client.WithHTTPClient(srv.Client()),
		client.WithVersion("1.43"),
	)
	if err != nil {
		t.Fatalf("constructing SDK client: %v", err)
	}
	t.Cleanup(func() { _ = sdk.Close() })

	adapter := &EventServiceAdapter{client: sdk}

	ctx, cancel := context.WithCancel(context.Background())
	eventCh, errCh := adapter.Subscribe(ctx, domain.EventFilter{})

	// Let the goroutine wire up, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range eventCh {
		}
		for range errCh {
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Subscribe goroutine did not exit within 3s of ctx cancel")
	}
}
