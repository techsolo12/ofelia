// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// validateSliceField (0% → 100%)
// ---------------------------------------------------------------------------

func TestValidator2_ValidateSliceField(t *testing.T) {
	t.Parallel()

	type configWithSlice struct {
		Tags []string `mapstructure:"tags"`
	}

	cfg := configWithSlice{
		Tags: []string{"tag1", "tag2"},
	}

	cv := NewConfigValidator(cfg)
	err := cv.Validate()

	// Should not error - slice validation is currently deferred to runtime
	assert.NoError(t, err)
}

func TestValidator2_ValidateSliceField_Empty(t *testing.T) {
	t.Parallel()

	type configWithSlice struct {
		Tags []string `mapstructure:"tags"`
	}

	cfg := configWithSlice{
		Tags: []string{},
	}

	cv := NewConfigValidator(cfg)
	err := cv.Validate()
	assert.NoError(t, err)
}
