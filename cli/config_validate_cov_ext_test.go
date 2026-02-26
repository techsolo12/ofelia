// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- validateDurationGTE ---

func TestValidateDurationGTE_TimeDuration(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	err := validate.RegisterValidation("duration_gte", validateDurationGTE)
	require.NoError(t, err)

	type testStruct struct {
		Duration time.Duration `validate:"duration_gte=1s"`
	}

	tests := []struct {
		name    string
		dur     time.Duration
		wantErr bool
	}{
		{"above threshold", 5 * time.Second, false},
		{"equal threshold", 1 * time.Second, false},
		{"below threshold", 500 * time.Millisecond, true},
		{"zero", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := testStruct{Duration: tt.dur}
			err := validate.Struct(s)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDurationGTE_InvalidParam_Coverage(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	err := validate.RegisterValidation("duration_gte_cov", validateDurationGTE)
	require.NoError(t, err)

	type testStruct struct {
		Duration time.Duration `validate:"duration_gte_cov=not-a-duration"`
	}

	s := testStruct{Duration: 5 * time.Second}
	err = validate.Struct(s)
	// Should fail because param can't be parsed
	assert.Error(t, err)
}

func TestValidateDurationGTE_OtherType(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	err := validate.RegisterValidation("duration_gte", validateDurationGTE)
	require.NoError(t, err)

	// String fields should always pass (not a duration type)
	type testStruct struct {
		Name string `validate:"duration_gte=1s"`
	}

	s := testStruct{Name: "test"}
	err = validate.Struct(s)
	assert.NoError(t, err)
}

// --- formatValidationError ---

func TestFormatValidationError_CronAndDockerImage(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	_ = validate.RegisterValidation("cron_cov", validateCron)
	_ = validate.RegisterValidation("dockerimage_cov", validateDockerImage)
	_ = validate.RegisterValidation("duration_gte_fmt", validateDurationGTE)

	type testStruct struct {
		CronField  string        `validate:"cron_cov"`
		ImageField string        `validate:"dockerimage_cov"`
		DurField   time.Duration `validate:"duration_gte_fmt=1h"`
	}

	s := testStruct{
		CronField:  "invalid cron",
		ImageField: "!!!invalid!!!",
		DurField:   1 * time.Second,
	}

	err := validate.Struct(s)
	require.Error(t, err)

	validationErrors, ok := err.(validator.ValidationErrors)
	require.True(t, ok)

	for _, ve := range validationErrors {
		msg := formatValidationError(ve)
		assert.NotEmpty(t, msg)
		assert.Contains(t, msg, ve.Field())
	}
}

func TestFormatValidationError_DefaultCase(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	_ = validate.RegisterValidation("custom_check", func(fl validator.FieldLevel) bool {
		return false
	})

	type testStruct struct {
		Value string `validate:"custom_check"`
	}

	s := testStruct{Value: "test"}
	err := validate.Struct(s)
	require.Error(t, err)

	validationErrors, ok := err.(validator.ValidationErrors)
	require.True(t, ok)
	for _, ve := range validationErrors {
		msg := formatValidationError(ve)
		assert.Contains(t, msg, "custom_check")
		assert.Contains(t, msg, "validation")
	}
}

// --- findClosestMatch ---

func TestFindClosestMatch_EmptyCandidates(t *testing.T) {
	t.Parallel()

	result := findClosestMatch("test", []string{})
	assert.Empty(t, result)
}

func TestFindClosestMatch_ExactMatch(t *testing.T) {
	t.Parallel()

	result := findClosestMatch("schedule", []string{"schedule", "command", "image"})
	assert.Equal(t, "schedule", result)
}

func TestFindClosestMatch_CloseMatch(t *testing.T) {
	t.Parallel()

	result := findClosestMatch("scheduel", []string{"schedule", "command", "image"})
	assert.Equal(t, "schedule", result)
}

func TestFindClosestMatch_NoCloseMatch(t *testing.T) {
	t.Parallel()

	result := findClosestMatch("zzzzzzzzzzz", []string{"schedule", "command", "image"})
	assert.Empty(t, result)
}

func TestFindClosestMatch_LongKey(t *testing.T) {
	t.Parallel()

	// For keys > 5 chars, threshold = len*2/5
	result := findClosestMatch("web-addresss", []string{"web-address", "web-auth", "web-port"})
	assert.Equal(t, "web-address", result)
}

// --- validateDockerImage ---

func TestValidateDockerImage_RegistryWithPort(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	_ = validate.RegisterValidation("dockerimage", validateDockerImage)

	type testStruct struct {
		Image string `validate:"dockerimage"`
	}

	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"registry with port", "registry.example.com:5000/image:tag", false},
		{"invalid port chars", "registry.example.com:abc/image:tag", true},
		{"standard image", "alpine:latest", false},
		{"image with digest", "alpine@sha256:" + strings.Repeat("a", 64), false},
		{"empty", "", false}, // Empty is valid (required check handles this)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := testStruct{Image: tt.image}
			err := validate.Struct(s)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- validateCron edge cases ---

func TestValidateCron_SpecialExpressions(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	_ = validate.RegisterValidation("cron", validateCron)

	type testStruct struct {
		Cron string `validate:"cron"`
	}

	tests := []struct {
		name    string
		cron    string
		wantErr bool
	}{
		{"@triggered", "@triggered", false},
		{"@manual", "@manual", false},
		{"@none", "@none", false},
		{"@every 1h30m", "@every 1h30m", false},
		{"@every invalid", "@every notaduration", true},
		{"invalid @prefix", "@invalid", true},
		{"6 field cron", "0 30 14 * * 1-5", false},
		{"too many fields", "0 0 0 0 0 0 0", true},
		{"too few fields", "0 0 0 0", true},
		{"invalid chars", "a b c d e", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := testStruct{Cron: tt.cron}
			err := validate.Struct(s)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
