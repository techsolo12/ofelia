// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/netresearch/ofelia/core/adapters/mock"
	"github.com/netresearch/ofelia/test"
)

// ---------------------------------------------------------------------------
// InitializeRuntimeFields (0% → 100%)
// ---------------------------------------------------------------------------

func TestRunServiceJob_InitializeRuntimeFields(t *testing.T) {
	t.Parallel()
	mc := mock.NewDockerClient()
	provider := NewSDKDockerProviderFromClient(mc, test.NewTestLogger(), nil)
	job := NewRunServiceJob(provider)

	// Should not panic and is a no-op
	job.InitializeRuntimeFields()
}
