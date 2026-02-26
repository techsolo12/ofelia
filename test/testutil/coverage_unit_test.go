// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package testutil_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/netresearch/ofelia/test/testutil"
)

// ---------------------------------------------------------------------------
// WaitForClose - closed channel path (60% → 100%)
// ---------------------------------------------------------------------------

func TestWaitForClose_ClosedChannel(t *testing.T) {
	t.Parallel()
	ch := make(chan int)
	close(ch)

	result := testutil.WaitForClose(t, ch, 1*time.Second)
	assert.True(t, result)
}

func TestWaitForClose_ChannelWithValue(t *testing.T) {
	t.Parallel()
	ch := make(chan int, 1)
	ch <- 42

	// WaitForClose receives a value, so ok is true → returns false (not closed)
	result := testutil.WaitForClose(t, ch, 1*time.Second)
	assert.False(t, result)
}

// ---------------------------------------------------------------------------
// EventuallyWithT - success/failure paths (95.8% → 100%)
// ---------------------------------------------------------------------------

func TestEventuallyWithT_SuccessOnSecondTry(t *testing.T) {
	t.Parallel()
	attempts := 0

	result := testutil.EventuallyWithT(t, func(collect *testutil.T) bool {
		attempts++
		if attempts < 2 {
			collect.Errorf("not ready yet, attempt %d", attempts)
			return false
		}
		return true
	},
		testutil.WithTimeout(2*time.Second),
		testutil.WithInterval(50*time.Millisecond),
	)

	assert.True(t, result)
	assert.GreaterOrEqual(t, attempts, 2)
}

func TestEventuallyWithT_ImmediateSuccess(t *testing.T) {
	t.Parallel()

	result := testutil.EventuallyWithT(t, func(collect *testutil.T) bool {
		return true
	}, testutil.WithTimeout(1*time.Second))

	assert.True(t, result)
}
