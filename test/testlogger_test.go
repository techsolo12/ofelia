// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package test

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewHandler(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if len(h.records) != 0 {
		t.Errorf("new handler should have 0 records, got %d", len(h.records))
	}
}

func TestHandler_Enabled(t *testing.T) {
	t.Parallel()

	h := NewHandler()

	tests := []struct {
		name  string
		level slog.Level
		want  bool
	}{
		{"debug", slog.LevelDebug, true},
		{"info", slog.LevelInfo, true},
		{"warn", slog.LevelWarn, true},
		{"error", slog.LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := h.Enabled(context.Background(), tt.level); got != tt.want {
				t.Errorf("Enabled(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestHandler_Handle(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	logger := slog.New(h)

	logger.Info("hello world")
	logger.Warn("something warning")
	logger.Error("something broke")

	messages := h.GetMessages()
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	if messages[0].Message != "hello world" {
		t.Errorf("messages[0].Message = %q, want %q", messages[0].Message, "hello world")
	}
	if messages[0].Level != "INFO" {
		t.Errorf("messages[0].Level = %q, want %q", messages[0].Level, "INFO")
	}
	if messages[1].Level != "WARN" {
		t.Errorf("messages[1].Level = %q, want %q", messages[1].Level, "WARN")
	}
	if messages[2].Level != "ERROR" {
		t.Errorf("messages[2].Level = %q, want %q", messages[2].Level, "ERROR")
	}
}

func TestHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	result := h.WithAttrs([]slog.Attr{slog.String("key", "value")})

	// WithAttrs returns the same handler (no-op implementation)
	if result != h {
		t.Error("WithAttrs should return the same handler")
	}
}

func TestHandler_WithGroup(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	result := h.WithGroup("mygroup")

	// WithGroup returns the same handler (no-op implementation)
	if result != h {
		t.Error("WithGroup should return the same handler")
	}
}

func TestNewTestLogger(t *testing.T) {
	t.Parallel()

	logger := NewTestLogger()
	if logger == nil {
		t.Fatal("NewTestLogger() returned nil")
	}

	// Verify it accepts log calls without panicking
	logger.Info("test info")
	logger.Warn("test warn")
	logger.Error("test error")
}

func TestNewTestLoggerWithHandler(t *testing.T) {
	t.Parallel()

	logger, h := NewTestLoggerWithHandler()
	if logger == nil {
		t.Fatal("logger is nil")
	}
	if h == nil {
		t.Fatal("handler is nil")
	}

	logger.Info("captured message")
	if !h.HasMessage("captured message") {
		t.Error("handler should have captured the message")
	}
}

func TestHandler_HasMessage(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	logger := slog.New(h)

	logger.Info("hello world")
	logger.Warn("a warning here")

	if !h.HasMessage("hello") {
		t.Error("HasMessage should match substring 'hello'")
	}
	if !h.HasMessage("warning") {
		t.Error("HasMessage should match substring 'warning'")
	}
	if h.HasMessage("nonexistent") {
		t.Error("HasMessage should not match 'nonexistent'")
	}
}

func TestHandler_HasError(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	logger := slog.New(h)

	logger.Info("info message")
	logger.Error("critical failure")

	if !h.HasError("critical") {
		t.Error("HasError should match error containing 'critical'")
	}
	if h.HasError("info") {
		t.Error("HasError should not match non-error messages")
	}
	if h.HasError("nonexistent") {
		t.Error("HasError should not match nonexistent messages")
	}
}

func TestHandler_HasWarning(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	logger := slog.New(h)

	logger.Info("info message")
	logger.Warn("disk space low")
	logger.Error("disk full")

	if !h.HasWarning("space low") {
		t.Error("HasWarning should match warning containing 'space low'")
	}
	if h.HasWarning("info message") {
		t.Error("HasWarning should not match non-warning messages")
	}
	if h.HasWarning("nonexistent") {
		t.Error("HasWarning should not match nonexistent messages")
	}
}

func TestHandler_Clear(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	logger := slog.New(h)

	logger.Info("msg1")
	logger.Info("msg2")

	if h.MessageCount() != 2 {
		t.Fatalf("expected 2 messages before clear, got %d", h.MessageCount())
	}

	h.Clear()

	if h.MessageCount() != 0 {
		t.Errorf("expected 0 messages after clear, got %d", h.MessageCount())
	}
	if h.HasMessage("msg1") {
		t.Error("HasMessage should return false after Clear")
	}
}

func TestHandler_MessageCount(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	logger := slog.New(h)

	if h.MessageCount() != 0 {
		t.Errorf("empty handler MessageCount = %d, want 0", h.MessageCount())
	}

	logger.Info("one")
	logger.Warn("two")
	logger.Error("three")

	if h.MessageCount() != 3 {
		t.Errorf("MessageCount = %d, want 3", h.MessageCount())
	}
}

func TestHandler_ErrorCount(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	logger := slog.New(h)

	if h.ErrorCount() != 0 {
		t.Errorf("empty handler ErrorCount = %d, want 0", h.ErrorCount())
	}

	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error1")
	logger.Error("error2")

	if h.ErrorCount() != 2 {
		t.Errorf("ErrorCount = %d, want 2", h.ErrorCount())
	}
}

func TestHandler_WarningCount(t *testing.T) {
	t.Parallel()

	h := NewHandler()
	logger := slog.New(h)

	if h.WarningCount() != 0 {
		t.Errorf("empty handler WarningCount = %d, want 0", h.WarningCount())
	}

	logger.Info("info")
	logger.Warn("warn1")
	logger.Warn("warn2")
	logger.Warn("warn3")
	logger.Error("error")

	if h.WarningCount() != 3 {
		t.Errorf("WarningCount = %d, want 3", h.WarningCount())
	}
}
