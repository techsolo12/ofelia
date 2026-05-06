// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"maps"
	"os"
	"strings"
	"time"
)

// parseAnnotations converts annotation strings in "key=value" format to a map.
// Invalid entries (missing '=' separator) are silently skipped.
// Leading/trailing whitespace in keys is trimmed, but whitespace in values is preserved.
func parseAnnotations(annotations []string) map[string]string {
	result := make(map[string]string)
	for _, ann := range annotations {
		parts := strings.SplitN(ann, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := parts[1] // preserve whitespace in value
			if key != "" {
				result[key] = value
			}
		}
	}
	return result
}

// Version is the Ofelia version, set via ldflags during build.
// Defaults to "dev" if not set.
var Version = "dev"

// Default annotation keys that Ofelia attaches to every job execution.
// These are stable wire-level identifiers consumed by downstream tooling
// (logging pipelines, dashboards). Don't change without coordinating with
// consumers.
const (
	AnnotationKeyJobName       = "ofelia.job.name"
	AnnotationKeyJobType       = "ofelia.job.type"
	AnnotationKeyExecutionTime = "ofelia.execution.time"
	AnnotationKeySchedulerHost = "ofelia.scheduler.host"
	AnnotationKeyVersion       = "ofelia.version"
)

// getDefaultAnnotations returns default annotations that Ofelia automatically adds.
// User-provided annotations take precedence over these defaults.
func getDefaultAnnotations(jobName, jobType string) map[string]string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	version := Version
	if version == "" {
		version = "dev"
	}

	return map[string]string{
		AnnotationKeyJobName:       jobName,
		AnnotationKeyJobType:       jobType,
		AnnotationKeyExecutionTime: time.Now().UTC().Format(time.RFC3339),
		AnnotationKeySchedulerHost: hostname,
		AnnotationKeyVersion:       version,
	}
}

// mergeAnnotations combines user annotations with default Ofelia annotations.
// User annotations take precedence over defaults (won't be overwritten).
func mergeAnnotations(userAnnotations []string, defaults map[string]string) map[string]string {
	// Start with defaults
	result := make(map[string]string)
	maps.Copy(result, defaults)

	// Override with user annotations
	parsed := parseAnnotations(userAnnotations)
	maps.Copy(result, parsed)

	return result
}
