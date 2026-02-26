// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// RecordWorkflowComplete (0% → 100%)
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_RecordWorkflowComplete(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	// No-op but should not panic
	pm.RecordWorkflowComplete("root-job", "success")
	pm.RecordWorkflowComplete("root-job", "failure")
}

// ---------------------------------------------------------------------------
// RecordWorkflowJobResult (0% → 100%)
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_RecordWorkflowJobResult(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	// No-op but should not panic
	pm.RecordWorkflowJobResult("job-a", "success")
	pm.RecordWorkflowJobResult("job-b", "failure")
}

// ---------------------------------------------------------------------------
// RecordJobStart (0% → 100%)
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_RecordJobStart(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordJobStart("test-job")

	metrics := pm.GetMetrics()
	system := metrics["system"].(map[string]any)
	assert.Equal(t, int64(1), system["concurrent_jobs"])
}

// ---------------------------------------------------------------------------
// RecordJobComplete (0% → 100%)
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_RecordJobComplete(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordJobStart("complete-job")
	pm.RecordJobComplete("complete-job", 1.5, false)

	jobMetrics := pm.GetJobMetrics()
	assert.Equal(t, int64(1), jobMetrics["total_executed"])
}

func TestPerformanceMetrics_RecordJobComplete_Panicked(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordJobStart("panic-job")
	pm.RecordJobComplete("panic-job", 0.5, true)

	jobMetrics := pm.GetJobMetrics()
	assert.Equal(t, int64(1), jobMetrics["total_failed"])
}

// ---------------------------------------------------------------------------
// RecordJobSkipped
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_RecordJobSkipped_MultipleReasons(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordJobSkipped("skip-job", "concurrent_limit")
	pm.RecordJobSkipped("skip-job", "disabled")
	pm.RecordJobSkipped("skip-job", "concurrent_limit")

	custom := pm.getCustomMetrics()
	reasons := custom["job_skip_reasons"].(map[string]int64)
	assert.Equal(t, int64(2), reasons["concurrent_limit"])
	assert.Equal(t, int64(1), reasons["disabled"])
}

// ---------------------------------------------------------------------------
// RecordConcurrentJobs - peak tracking with CAS loop
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_RecordConcurrentJobs_PeakTracking(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordConcurrentJobs(5)
	pm.RecordConcurrentJobs(10)
	pm.RecordConcurrentJobs(3) // lower, should not update peak

	system := pm.getSystemMetrics()
	assert.Equal(t, int64(10), system["max_concurrent_jobs"])
}

// ---------------------------------------------------------------------------
// RecordMemoryUsage - peak tracking
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_RecordMemoryUsage_PeakTracking(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordMemoryUsage(1024)
	pm.RecordMemoryUsage(4096)
	pm.RecordMemoryUsage(2048) // lower, peak stays at 4096

	system := pm.getSystemMetrics()
	assert.Equal(t, int64(4096), system["peak_memory_usage"])
	assert.Equal(t, int64(2048), system["current_memory_usage"])
}

// ---------------------------------------------------------------------------
// RecordDockerLatency - min/max tracking
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_RecordDockerLatency_MinMax(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordDockerLatency("pull", 100*time.Millisecond)
	pm.RecordDockerLatency("pull", 50*time.Millisecond)
	pm.RecordDockerLatency("pull", 200*time.Millisecond)

	docker := pm.GetDockerMetrics()
	latencies := docker["latencies"].(map[string]map[string]any)
	pull := latencies["pull"]
	assert.Equal(t, int64(3), pull["count"])
	assert.Equal(t, 50*time.Millisecond, pull["min"])
	assert.Equal(t, 200*time.Millisecond, pull["max"])
}

// ---------------------------------------------------------------------------
// GetSummaryReport
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_GetSummaryReport(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	// Add some data
	pm.RecordDockerOperation("pull")
	pm.RecordDockerError("pull")
	pm.RecordJobExecution("test-job", 1*time.Second, true)
	pm.RecordConcurrentJobs(3)

	report := pm.GetSummaryReport()
	assert.Contains(t, report, "Performance Summary")
	assert.Contains(t, report, "Docker Operations")
	assert.Contains(t, report, "Job Execution")
	assert.Contains(t, report, "System Performance")
}

// ---------------------------------------------------------------------------
// GetMetrics - full output
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_GetMetrics_AllSections(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordDockerOperation("pull")
	pm.RecordJobRetry("job-a", 1, true)
	pm.RecordContainerEvent()
	pm.RecordContainerMonitorFallback()
	pm.RecordContainerWaitDuration(1.5)
	pm.RecordBufferPoolStats(map[string]any{"hits": 10})
	pm.RecordCustomMetric("custom_1", 42)

	m := pm.GetMetrics()
	assert.NotNil(t, m["docker"])
	assert.NotNil(t, m["jobs"])
	assert.NotNil(t, m["system"])
	assert.NotNil(t, m["buffer_pool"])
	assert.NotNil(t, m["retries"])
	assert.NotNil(t, m["container"])
	assert.NotNil(t, m["custom"])
	assert.NotNil(t, m["uptime"])
}

// ---------------------------------------------------------------------------
// Reset
// ---------------------------------------------------------------------------

func TestPerformanceMetrics_Reset(t *testing.T) {
	t.Parallel()
	pm := NewPerformanceMetrics()

	pm.RecordDockerOperation("pull")
	pm.RecordJobExecution("job-x", 1*time.Second, true)
	pm.RecordJobRetry("job-x", 1, true)
	pm.RecordContainerWaitDuration(2.0)
	pm.RecordBufferPoolStats(map[string]any{"hits": 5})
	pm.RecordCustomMetric("key", "value")

	pm.Reset()

	docker := pm.GetDockerMetrics()
	assert.Equal(t, int64(0), docker["total_operations"])

	jobs := pm.GetJobMetrics()
	assert.Equal(t, int64(0), jobs["total_executed"])
}
