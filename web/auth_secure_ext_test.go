// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupOldLimiters_Empty(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(10, 5)

	// Cleanup on empty map should not panic
	rl.CleanupOldLimiters()
	assert.NotNil(t, rl.limiters)
	assert.Empty(t, rl.limiters)
}

func TestCleanupOldLimiters_WithEntries(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(10, 5)
	rl.GetLimiter("ip-1")
	rl.GetLimiter("ip-2")
	rl.GetLimiter("ip-3")

	assert.Len(t, rl.limiters, 3)

	// Cleanup should not panic and map should remain valid
	rl.CleanupOldLimiters()
	assert.NotNil(t, rl.limiters)
}

func TestValidateCSRFToken_Valid(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret-key", 24)
	require.NoError(t, err)
	defer tm.Close()

	token, err := tm.GenerateCSRFToken()
	require.NoError(t, err)

	assert.True(t, tm.ValidateCSRFToken(token), "valid CSRF token should pass validation")
}

func TestValidateCSRFToken_OneTimeUse(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret-key", 24)
	require.NoError(t, err)
	defer tm.Close()

	token, err := tm.GenerateCSRFToken()
	require.NoError(t, err)

	assert.True(t, tm.ValidateCSRFToken(token))
	assert.False(t, tm.ValidateCSRFToken(token),
		"CSRF token should be invalidated after single use")
}

func TestValidateCSRFToken_Missing(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret-key", 24)
	require.NoError(t, err)
	defer tm.Close()

	assert.False(t, tm.ValidateCSRFToken("nonexistent-token"),
		"non-existent CSRF token should fail validation")
}

func TestValidateCSRFToken_Expired(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret-key", 24)
	require.NoError(t, err)
	defer tm.Close()

	token, err := tm.GenerateCSRFToken()
	require.NoError(t, err)

	tm.csrfMu.Lock()
	tm.csrfTokens[token] = time.Now().Add(-1 * time.Hour)
	tm.csrfMu.Unlock()

	assert.False(t, tm.ValidateCSRFToken(token),
		"expired CSRF token should fail validation")
}

func TestCleanupExpiredTokens(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret-key", 1)
	require.NoError(t, err)
	defer tm.Close()

	token, err := tm.GenerateToken("testuser")
	require.NoError(t, err)

	tm.mu.Lock()
	tm.tokens[token].ExpiresAt = time.Now().Add(-1 * time.Hour)
	tm.mu.Unlock()

	validToken, err := tm.GenerateToken("testuser2")
	require.NoError(t, err)

	tm.cleanupExpiredTokens()

	tm.mu.RLock()
	_, expiredExists := tm.tokens[token]
	_, validExists := tm.tokens[validToken]
	tm.mu.RUnlock()

	assert.False(t, expiredExists, "expired token should be cleaned up")
	assert.True(t, validExists, "valid token should survive cleanup")
}

func TestCleanupExpiredCSRFTokens(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret-key", 24)
	require.NoError(t, err)
	defer tm.Close()

	expiredToken, err := tm.GenerateCSRFToken()
	require.NoError(t, err)

	tm.csrfMu.Lock()
	tm.csrfTokens[expiredToken] = time.Now().Add(-1 * time.Hour)
	tm.csrfMu.Unlock()

	validToken, err := tm.GenerateCSRFToken()
	require.NoError(t, err)

	tm.cleanupExpiredCSRFTokens()

	tm.csrfMu.RLock()
	_, expiredExists := tm.csrfTokens[expiredToken]
	_, validExists := tm.csrfTokens[validToken]
	tm.csrfMu.RUnlock()

	assert.False(t, expiredExists, "expired CSRF token should be cleaned up")
	assert.True(t, validExists, "valid CSRF token should survive cleanup")
}

func TestHashPassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
	}{
		{"simple password", "password123"},
		{"empty password", ""},
		{"long password", "this-is-a-very-long-password-that-should-still-work"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			require.NoError(t, err)
			assert.NotEmpty(t, hash)
			assert.Contains(t, hash, "$2a$12$", "hash should use cost 12")

			hash2, err := HashPassword(tt.password)
			require.NoError(t, err)
			assert.NotEqual(t, hash, hash2,
				"bcrypt should produce different hashes for same password")
		})
	}
}

func TestNewSecureTokenManager_EmptySecretKey(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("", 24)
	require.NoError(t, err)
	defer tm.Close()

	assert.NotEmpty(t, tm.secretKey, "empty secret key should result in generated key")

	token, err := tm.GenerateToken("admin")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	data, valid := tm.ValidateToken(token)
	assert.True(t, valid)
	assert.Equal(t, "admin", data.Username)
}

func TestSecureTokenManager_Close_Multiple(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret", 24)
	require.NoError(t, err)

	tm.Close()
	tm.Close()
	tm.Close()
}

func TestSecureTokenManager_RevokeToken(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret", 24)
	require.NoError(t, err)
	defer tm.Close()

	token, err := tm.GenerateToken("user")
	require.NoError(t, err)

	_, valid := tm.ValidateToken(token)
	assert.True(t, valid)

	tm.RevokeToken(token)

	_, valid = tm.ValidateToken(token)
	assert.False(t, valid, "revoked token should not be valid")
}

func TestSecureTokenManager_ValidateToken_Expired(t *testing.T) {
	t.Parallel()

	tm, err := NewSecureTokenManager("test-secret", 1)
	require.NoError(t, err)
	defer tm.Close()

	token, err := tm.GenerateToken("user")
	require.NoError(t, err)

	tm.mu.Lock()
	tm.tokens[token].ExpiresAt = time.Now().Add(-1 * time.Hour)
	tm.mu.Unlock()

	_, valid := tm.ValidateToken(token)
	assert.False(t, valid, "expired token should not be valid")
}

func TestGetClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		xff        string
		xri        string
		remoteAddr string
		expected   string
	}{
		{
			name:       "X-Forwarded-For single IP",
			xff:        "203.0.113.1",
			remoteAddr: "127.0.0.1:1234",
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			xff:        "203.0.113.1, 70.41.3.18",
			remoteAddr: "127.0.0.1:1234",
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Real-IP header",
			xri:        "198.51.100.1",
			remoteAddr: "127.0.0.1:1234",
			expected:   "198.51.100.1",
		},
		{
			name:       "RemoteAddr with port",
			remoteAddr: "192.168.1.1:5678",
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			got := getClientIP(req)
			assert.Equal(t, tt.expected, got)
		})
	}
}
