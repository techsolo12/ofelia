// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// CleanupOldLimiters (0% → 100%)
// ---------------------------------------------------------------------------

func TestRateLimiter_CleanupOldLimiters(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(10, 5)

	// Add some limiters
	rl.GetLimiter("192.168.1.1")
	rl.GetLimiter("192.168.1.2")

	// Should not panic; all entries are recent so none should be removed
	rl.CleanupOldLimiters(5 * time.Minute)

	// Limiters should still exist (cleanup is a no-op)
	assert.True(t, rl.Allow("192.168.1.1"))
}

// ---------------------------------------------------------------------------
// SecureTokenManager Close idempotent
// ---------------------------------------------------------------------------

func TestSecureTokenManager_Close_Idempotent(t *testing.T) {
	t.Parallel()
	tm, err := NewSecureTokenManager("test-secret", 24)
	if err != nil {
		t.Fatalf("NewSecureTokenManager: %v", err)
	}

	// Close should be safe to call multiple times
	tm.Close()
	tm.Close()
	tm.Close()
}

// ---------------------------------------------------------------------------
// cleanupExpiredCSRFTokens
// ---------------------------------------------------------------------------

func TestSecureTokenManager_CleanupExpiredCSRFTokens(t *testing.T) {
	t.Parallel()
	tm, err := NewSecureTokenManager("test-secret", 24)
	if err != nil {
		t.Fatalf("NewSecureTokenManager: %v", err)
	}
	defer tm.Close()

	// Generate a CSRF token
	token, err := tm.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken: %v", err)
	}

	// Token should be valid
	assert.True(t, tm.ValidateCSRFToken(token))

	// Cleanup should not panic
	tm.cleanupExpiredCSRFTokens()
}

// ---------------------------------------------------------------------------
// cleanupExpiredTokens
// ---------------------------------------------------------------------------

func TestSecureTokenManager_CleanupExpiredTokens(t *testing.T) {
	t.Parallel()
	tm, err := NewSecureTokenManager("test-secret", 24)
	if err != nil {
		t.Fatalf("NewSecureTokenManager: %v", err)
	}
	defer tm.Close()

	// Generate a token
	token, err := tm.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	// Token should be valid
	data, valid := tm.ValidateToken(token)
	assert.True(t, valid)
	assert.Equal(t, "admin", data.Username)

	// Cleanup should not remove unexpired tokens
	tm.cleanupExpiredTokens()

	data, valid = tm.ValidateToken(token)
	assert.True(t, valid)
	assert.Equal(t, "admin", data.Username)
}
