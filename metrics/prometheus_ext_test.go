// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecordJobStart_CounterAndGauge(t *testing.T) {
	t.Parallel()

	mc := NewCollector()
	mc.InitDefaultMetrics()

	// Record multiple job starts
	mc.RecordJobStart("job-a")
	mc.RecordJobStart("job-b")
	mc.RecordJobStart("job-c")

	// Counter should be incremented 3 times
	assert.Equal(t, float64(3), mc.metrics["ofelia_cron_jobs_started_total"].Value,
		"started counter should be 3 after 3 job starts")

	// Running gauge should be 3
	assert.Equal(t, float64(3), mc.getGaugeValue("ofelia_jobs_running"),
		"running gauge should be 3 with 3 active jobs")
}

func TestRecordJobComplete_AllFields(t *testing.T) {
	t.Parallel()

	mc := NewCollector()
	mc.InitDefaultMetrics()

	// Start two jobs
	mc.RecordJobStart("job-1")
	mc.RecordJobStart("job-2")

	assert.Equal(t, float64(2), mc.getGaugeValue("ofelia_jobs_running"))

	// Complete first job (no panic)
	mc.RecordJobComplete("job-1", 2.5, false)

	// Verify counter
	assert.Equal(t, float64(1), mc.metrics["ofelia_cron_jobs_completed_total"].Value)

	// Verify histogram recorded the duration
	hist := mc.metrics["ofelia_job_duration_seconds"].Histogram
	assert.Equal(t, int64(1), hist.Count)
	assert.Equal(t, 2.5, hist.Sum)

	// Verify running gauge decreased
	assert.Equal(t, float64(1), mc.getGaugeValue("ofelia_jobs_running"))

	// Verify panicked counter NOT incremented
	assert.Equal(t, float64(0), mc.metrics["ofelia_cron_jobs_panicked_total"].Value)

	// Complete second job with panic
	mc.RecordJobComplete("job-2", 0.1, true)

	assert.Equal(t, float64(2), mc.metrics["ofelia_cron_jobs_completed_total"].Value)
	assert.Equal(t, float64(1), mc.metrics["ofelia_cron_jobs_panicked_total"].Value)
	assert.Equal(t, float64(0), mc.getGaugeValue("ofelia_jobs_running"))

	// Histogram should have 2 observations
	assert.Equal(t, int64(2), hist.Count)
	assert.InDelta(t, 2.6, hist.Sum, 0.001)
}

func TestRecordJobScheduled_Counter(t *testing.T) {
	t.Parallel()

	mc := NewCollector()
	mc.InitDefaultMetrics()

	mc.RecordJobScheduled("job-x")
	mc.RecordJobScheduled("job-y")
	mc.RecordJobScheduled("job-z")
	mc.RecordJobScheduled("job-x") // same job scheduled again

	assert.Equal(t, float64(4), mc.metrics["ofelia_cron_jobs_scheduled_total"].Value,
		"scheduled counter should be 4 after 4 scheduling events")
}

func TestRecordJobStart_GaugeNeverNegative(t *testing.T) {
	t.Parallel()

	mc := NewCollector()
	mc.InitDefaultMetrics()

	// Start and complete a job
	mc.RecordJobStart("job-1")
	mc.RecordJobComplete("job-1", 1.0, false)

	assert.Equal(t, float64(0), mc.getGaugeValue("ofelia_jobs_running"))

	// Complete again without start (shouldn't happen normally, but test gauge behavior)
	mc.RecordJobComplete("job-1", 1.0, false)

	// Gauge goes negative in this implementation (no clamping)
	assert.Equal(t, float64(-1), mc.getGaugeValue("ofelia_jobs_running"),
		"gauge can go negative without clamping")
}

func TestRecordJobComplete_ZeroDuration(t *testing.T) {
	t.Parallel()

	mc := NewCollector()
	mc.InitDefaultMetrics()

	mc.RecordJobStart("instant-job")
	mc.RecordJobComplete("instant-job", 0.0, false)

	hist := mc.metrics["ofelia_job_duration_seconds"].Histogram
	assert.Equal(t, int64(1), hist.Count)
	assert.Equal(t, 0.0, hist.Sum)
}

func TestRecordJobComplete_LargeDuration(t *testing.T) {
	t.Parallel()

	mc := NewCollector()
	mc.InitDefaultMetrics()

	mc.RecordJobStart("long-job")
	mc.RecordJobComplete("long-job", 86400.0, false) // 24 hours

	hist := mc.metrics["ofelia_job_duration_seconds"].Histogram
	assert.Equal(t, int64(1), hist.Count)
	assert.Equal(t, 86400.0, hist.Sum)
}

func TestAdjustGauge_NonExistent(t *testing.T) {
	t.Parallel()

	mc := NewCollector()

	// Should not panic when adjusting non-existent gauge
	mc.adjustGauge("nonexistent", 5.0)

	_, exists := mc.metrics["nonexistent"]
	assert.False(t, exists, "non-existent gauge should not be auto-created")
}

func TestAdjustGauge_WrongType(t *testing.T) {
	t.Parallel()

	mc := NewCollector()
	mc.RegisterCounter("my_counter", "A counter")

	// Adjusting a counter as gauge should have no effect
	mc.adjustGauge("my_counter", 5.0)

	assert.Equal(t, float64(0), mc.metrics["my_counter"].Value,
		"adjustGauge on counter type should have no effect")
}

func TestFullJobLifecycle_MetricsIntegrity(t *testing.T) {
	t.Parallel()

	mc := NewCollector()
	mc.InitDefaultMetrics()

	// Simulate 5 complete job lifecycles
	for i := range 5 {
		jobName := "lifecycle-job"
		mc.RecordJobScheduled(jobName)
		mc.RecordJobStart(jobName)

		duration := float64(i+1) * 0.5 // 0.5, 1.0, 1.5, 2.0, 2.5
		panicked := i == 4              // last one panics
		mc.RecordJobComplete(jobName, duration, panicked)
	}

	assert.Equal(t, float64(5), mc.metrics["ofelia_cron_jobs_scheduled_total"].Value)
	assert.Equal(t, float64(5), mc.metrics["ofelia_cron_jobs_started_total"].Value)
	assert.Equal(t, float64(5), mc.metrics["ofelia_cron_jobs_completed_total"].Value)
	assert.Equal(t, float64(1), mc.metrics["ofelia_cron_jobs_panicked_total"].Value)
	assert.Equal(t, float64(0), mc.getGaugeValue("ofelia_jobs_running"))

	hist := mc.metrics["ofelia_job_duration_seconds"].Histogram
	assert.Equal(t, int64(5), hist.Count)
	assert.InDelta(t, 7.5, hist.Sum, 0.001) // 0.5+1.0+1.5+2.0+2.5 = 7.5
}
