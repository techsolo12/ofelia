// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core/domain"
)

// TestConvertToHostConfig_VolumesFrom verifies that domain.HostConfig.VolumesFrom
// is correctly mapped to the Docker SDK's container.HostConfig.VolumesFrom field.

func TestConvertToHostConfig_VolumesFrom(t *testing.T) {
	t.Parallel()

	input := &domain.HostConfig{
		VolumesFrom: []string{"data-container", "config-container:ro"},
	}

	result := convertToHostConfig(input)
	require.NotNil(t, result)
	assert.Equal(t, []string{"data-container", "config-container:ro"}, result.VolumesFrom)
}

func TestConvertToHostConfig_VolumesFromEmpty(t *testing.T) {
	t.Parallel()

	input := &domain.HostConfig{
		Binds: []string{"/host:/container"},
	}

	result := convertToHostConfig(input)
	require.NotNil(t, result)
	assert.Empty(t, result.VolumesFrom)
}
