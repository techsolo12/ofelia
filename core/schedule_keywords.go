// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

// Special schedule keywords recognized by go-cron and Ofelia.
//
// The time-based keywords (Yearly, Monthly, Weekly, Daily, Hourly and their
// aliases) map to fixed cron expressions inside go-cron's parser.
//
// The run-on-trigger-only keywords (Triggered, Manual, None) map to a
// TriggeredSchedule whose Next() returns the zero time, so jobs registered
// with these schedules never fire automatically — they only run when invoked
// via TriggerEntryByName() or by another job's on-success/on-failure chain.
const (
	TriggeredSchedule = "@triggered"
	ManualSchedule    = "@manual"
	NoneSchedule      = "@none"

	YearlySchedule   = "@yearly"
	AnnuallySchedule = "@annually" // alias of @yearly
	MonthlySchedule  = "@monthly"
	WeeklySchedule   = "@weekly"
	DailySchedule    = "@daily"
	MidnightSchedule = "@midnight" // alias of @daily
	HourlySchedule   = "@hourly"
)
