// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package middlewares

// Execution status labels emitted in user-facing notification payloads
// (mail body, webhook body, save artifacts). These are stable strings that
// downstream consumers parse, so they live in one place.
const (
	statusSuccessful = "successful"
	statusFailed     = "failed"
	statusSkipped    = "skipped"
)

// JSON/template variable names for the per-notification context payload
// shared by mail, save, webhook, and restore. The struct tags on
// restore.go's payload type pin the wire format to these names.
const (
	notificationVarJob       = "Job"
	notificationVarExecution = "Execution"
)

// contentTypeJSON is the default Content-Type for webhook POSTs that
// don't override it via preset headers.
const contentTypeJSON = "application/json"
