// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetKnownKeysForJobType_ServiceRunContainerSpecFields verifies that
// job-service-run recognizes environment, hostname, and dir as valid keys,
// matching capabilities exposed by the Docker Swarm ContainerSpec.
func TestGetKnownKeysForJobType_ServiceRunContainerSpecFields(t *testing.T) {
	t.Parallel()

	keys := getKnownKeysForJobType("job-service-run")

	assert.Contains(t, keys, "environment", "job-service-run should accept 'environment'")
	assert.Contains(t, keys, "hostname", "job-service-run should accept 'hostname'")
	assert.Contains(t, keys, "dir", "job-service-run should accept 'dir'")
}

// TestExtractMapstructureKeys_RunServiceConfig_ContainerSpecFields verifies that
// extractMapstructureKeys finds environment, hostname, and dir in RunServiceConfig.
func TestExtractMapstructureKeys_RunServiceConfig_ContainerSpecFields(t *testing.T) {
	t.Parallel()

	keys := extractMapstructureKeys(RunServiceConfig{})

	assert.Contains(t, keys, "environment", "RunServiceConfig should have 'environment' key")
	assert.Contains(t, keys, "hostname", "RunServiceConfig should have 'hostname' key")
	assert.Contains(t, keys, "dir", "RunServiceConfig should have 'dir' key")
}

// TestGetKnownKeysForJobType_RunJobWorkingDir verifies that
// job-run recognizes working-dir as a valid config key.
func TestGetKnownKeysForJobType_RunJobWorkingDir(t *testing.T) {
	t.Parallel()

	keys := getKnownKeysForJobType("job-run")

	assert.Contains(t, keys, "working-dir", "job-run should accept 'working-dir'")
}

// TestGetKnownKeysForJobType_ExecJobPrivileged verifies that
// job-exec recognizes privileged as a valid config key.
func TestGetKnownKeysForJobType_ExecJobPrivileged(t *testing.T) {
	t.Parallel()

	keys := getKnownKeysForJobType("job-exec")

	assert.Contains(t, keys, "privileged", "job-exec should accept 'privileged'")
}
