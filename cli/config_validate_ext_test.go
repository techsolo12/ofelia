// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDurationGTE_AboveMin(t *testing.T) {
	t.Parallel()

	type Config struct {
		Timeout time.Duration `validate:"duration_gte=1m"`
	}

	err := ValidateConfig(&Config{Timeout: 5 * time.Minute})
	assert.NoError(t, err, "5m >= 1m should be valid")
}

func TestValidateDurationGTE_EqualMin(t *testing.T) {
	t.Parallel()

	type Config struct {
		Timeout time.Duration `validate:"duration_gte=1m"`
	}

	err := ValidateConfig(&Config{Timeout: time.Minute})
	assert.NoError(t, err, "1m >= 1m should be valid")
}

func TestValidateDurationGTE_BelowMin(t *testing.T) {
	t.Parallel()

	type Config struct {
		Timeout time.Duration `validate:"duration_gte=1m"`
	}

	err := ValidateConfig(&Config{Timeout: 30 * time.Second})
	assert.Error(t, err, "30s < 1m should be invalid")
}

func TestValidateDurationGTE_ZeroWithZeroMin(t *testing.T) {
	t.Parallel()

	type Config struct {
		Timeout time.Duration `validate:"duration_gte=0s"`
	}

	err := ValidateConfig(&Config{Timeout: 0})
	assert.NoError(t, err, "0 >= 0s should be valid")
}

func TestValidateDurationGTE_ZeroBelowMin(t *testing.T) {
	t.Parallel()

	type Config struct {
		Timeout time.Duration `validate:"duration_gte=1s"`
	}

	err := ValidateConfig(&Config{Timeout: 0})
	assert.Error(t, err, "0 < 1s should be invalid")
}

func TestValidateDurationGTE_LargeDuration(t *testing.T) {
	t.Parallel()

	type Config struct {
		Timeout time.Duration `validate:"duration_gte=1h"`
	}

	err := ValidateConfig(&Config{Timeout: 24 * time.Hour})
	assert.NoError(t, err, "24h >= 1h should be valid")
}

func TestValidateDurationGTE_InvalidParam(t *testing.T) {
	t.Parallel()

	// A bad param (not a valid Go duration) should cause the validator to return false
	type Config struct {
		Timeout time.Duration `validate:"duration_gte=notaduration"`
	}

	cfg := Config{Timeout: 5 * time.Minute}
	err := ValidateConfig(&cfg)
	assert.Error(t, err, "invalid duration param should fail validation")
}

func TestValidateConfig_NonValidationError(t *testing.T) {
	t.Parallel()

	// ValidateConfig with a non-struct should produce a non-ValidationErrors error,
	// testing the !ok branch of errors.AsType
	err := ValidateConfig("not a struct")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidationFailed)
}

func TestFormatValidationError_AllTags(t *testing.T) {
	t.Parallel()

	// Exercise multiple formatValidationError branches by triggering different tag failures
	tests := []struct {
		name      string
		cfg       any
		wantSubst string
	}{
		{
			"required",
			&struct {
				Name string `validate:"required"`
			}{},
			"required field is empty",
		},
		{
			"min length",
			&struct {
				Name string `validate:"min=5"`
			}{Name: "ab"},
			"must be at least",
		},
		{
			"max length",
			&struct {
				Name string `validate:"max=2"`
			}{Name: "toolong"},
			"must be at most",
		},
		{
			"url",
			&struct {
				URL string `validate:"url"`
			}{URL: "not a url"},
			"must be a valid URL",
		},
		{
			"email",
			&struct {
				Email string `validate:"email"`
			}{Email: "bademail"},
			"must be a valid email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantSubst)
		})
	}
}
