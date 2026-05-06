// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package config

// Special schedule keywords used by ofelia's validators and sanitizers.
//
// These mirror core's schedule keywords (core/schedule_keywords.go) and
// cli's log level names (cli/logging.go), but are duplicated here because
// the config package sits below both in the import graph and cannot depend
// on them. Keep them in sync; consumers downstream should reference the
// exported names from core/cli where possible.
const (
	scheduleTriggered = "@triggered"
	scheduleManual    = "@manual"
	scheduleNone      = "@none"

	logLevelTrace    = "trace"
	logLevelDebug    = "debug"
	logLevelInfo     = "info"
	logLevelNotice   = "notice"
	logLevelWarn     = "warn"
	logLevelWarning  = "warning"
	logLevelError    = "error"
	logLevelFatal    = "fatal"
	logLevelPanic    = "panic"
	logLevelCritical = "critical"
)
