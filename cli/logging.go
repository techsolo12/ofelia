// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// ErrInvalidLogLevel indicates an invalid log level string was provided.
var ErrInvalidLogLevel = errors.New("invalid log level")

// Recognized log level names (canonical and legacy/logrus aliases).
// Exported so other commands (init prompts, validators) can reference them
// without duplicating string literals.
const (
	LogLevelTrace    = "trace"
	LogLevelDebug    = "debug"
	LogLevelInfo     = "info"
	LogLevelNotice   = "notice"
	LogLevelWarn     = "warn"
	LogLevelWarning  = "warning"
	LogLevelError    = "error"
	LogLevelFatal    = "fatal"
	LogLevelPanic    = "panic"
	LogLevelCritical = "critical"
)

// ApplyLogLevel sets the logging level if level is valid.
// Returns an error if the level is invalid, with a list of valid options.
func ApplyLogLevel(level string, lv *slog.LevelVar) error {
	if level == "" {
		return nil
	}

	// Map legacy logrus level names to slog levels
	var l slog.Level
	switch strings.ToLower(level) {
	case LogLevelTrace, LogLevelDebug:
		l = slog.LevelDebug
	case LogLevelInfo, LogLevelNotice:
		l = slog.LevelInfo
	case LogLevelWarning, LogLevelWarn:
		l = slog.LevelWarn
	case LogLevelError, LogLevelFatal, LogLevelPanic, LogLevelCritical:
		l = slog.LevelError
	default:
		return fmt.Errorf("%w: %q (valid levels are debug, info, warn, error)", ErrInvalidLogLevel, level)
	}

	if lv != nil {
		lv.Set(l)
	}
	return nil
}
