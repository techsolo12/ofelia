// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core/domain"
)

// ---------------------------------------------------------------------------
// WaitForContainer (0% → 100%)
// ---------------------------------------------------------------------------

func TestTestContainerMonitor_WaitForContainer_Default(t *testing.T) {
	t.Parallel()
	mon := &TestContainerMonitor{}

	state, err := mon.WaitForContainer("container-1", 30*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 0, state.ExitCode)
	assert.False(t, state.Running)
}

func TestTestContainerMonitor_WaitForContainer_CustomFunc(t *testing.T) {
	t.Parallel()
	mon := &TestContainerMonitor{
		waitForContainerFunc: func(containerID string, maxRuntime time.Duration) (*domain.ContainerState, error) {
			return &domain.ContainerState{ExitCode: 42, Running: false}, nil
		},
	}

	state, err := mon.WaitForContainer("container-1", 30*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 42, state.ExitCode)
}

// ---------------------------------------------------------------------------
// SetUseEventsAPI (0% → 100%)
// ---------------------------------------------------------------------------

func TestTestContainerMonitor_SetUseEventsAPI(t *testing.T) {
	t.Parallel()
	mon := &TestContainerMonitor{}

	// Should not panic (no-op)
	mon.SetUseEventsAPI(true)
	mon.SetUseEventsAPI(false)
}
