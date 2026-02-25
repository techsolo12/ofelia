// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
	"testing"
)

type stubCloser struct {
	err    error
	called bool
}

func (s *stubCloser) Close() error {
	s.called = true
	return s.err
}

func TestHijackedResponse_Close(t *testing.T) {
	t.Parallel()

	t.Run("nil_conn", func(t *testing.T) {
		t.Parallel()
		h := &HijackedResponse{Conn: nil}
		if err := h.Close(); err != nil {
			t.Errorf("Close() with nil Conn returned error: %v", err)
		}
	})

	t.Run("successful_close", func(t *testing.T) {
		t.Parallel()
		conn := &stubCloser{}
		h := &HijackedResponse{Conn: conn}

		if err := h.Close(); err != nil {
			t.Errorf("Close() returned unexpected error: %v", err)
		}
		if !conn.called {
			t.Error("expected Conn.Close() to be called")
		}
	})

	t.Run("close_error", func(t *testing.T) {
		t.Parallel()
		connErr := errors.New("broken pipe")
		conn := &stubCloser{err: connErr}
		h := &HijackedResponse{Conn: conn}

		err := h.Close()
		if err == nil {
			t.Fatal("Close() should return an error")
		}
		if !errors.Is(err, connErr) {
			t.Errorf("Close() error should wrap %v, got %v", connErr, err)
		}
		if !conn.called {
			t.Error("expected Conn.Close() to be called")
		}
	})
}
