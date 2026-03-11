// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

const (
	// BearerPrefix is the prefix for Bearer tokens in Authorization header
	BearerPrefix = "Bearer"
	// httpsProto is used to check X-Forwarded-Proto header for HTTPS
	httpsProto = "https"
)

type TokenData struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// SecureAuthConfig holds secure authentication configuration
type SecureAuthConfig struct {
	Enabled        bool     `json:"enabled"`
	Username       string   `json:"username"`
	PasswordHash   string   `json:"passwordHash"` // bcrypt hash of password
	SecretKey      string   `json:"secretKey"`
	TokenExpiry    int      `json:"tokenExpiry"`    // in hours
	MaxAttempts    int      `json:"maxAttempts"`    // max login attempts per minute
	TrustedProxies []string `json:"trustedProxies"` // CIDRs or IPs whose X-Forwarded-For is trusted
}

// RateLimiter manages login attempt rate limiting
type RateLimiter struct {
	limiters   map[string]*rate.Limiter
	lastAccess map[string]time.Time
	mu         sync.RWMutex
	rate       int // attempts per minute
	burst      int // burst size
}

func NewRateLimiter(ratePerMinute, burst int) *RateLimiter {
	return &RateLimiter{
		limiters:   make(map[string]*rate.Limiter),
		lastAccess: make(map[string]time.Time),
		rate:       ratePerMinute,
		burst:      burst,
	}
}

func (rl *RateLimiter) GetLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.lastAccess[key] = time.Now()
	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(rl.rate)), rl.burst)
		rl.limiters[key] = limiter
	}

	return limiter
}

func (rl *RateLimiter) Allow(key string) bool {
	limiter := rl.GetLimiter(key)
	return limiter.Allow()
}

// CleanupOldLimiters removes limiters that haven't been accessed for the given maxAge.
func (rl *RateLimiter) CleanupOldLimiters(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for key, lastAccess := range rl.lastAccess {
		if lastAccess.Before(cutoff) {
			delete(rl.limiters, key)
			delete(rl.lastAccess, key)
		}
	}
}

// SecureTokenManager handles token management with enhanced security
type SecureTokenManager struct {
	secretKey   []byte
	tokens      map[string]*TokenData
	mu          sync.RWMutex
	tokenExpiry time.Duration
	csrfTokens  map[string]time.Time // CSRF token storage
	csrfMu      sync.RWMutex
	done        chan struct{}
	closeOnce   sync.Once
}

func NewSecureTokenManager(secretKey string, expiryHours int) (*SecureTokenManager, error) {
	if secretKey == "" {
		// Generate a cryptographically secure random key
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("failed to generate secret key: %w", err)
		}
		secretKey = base64.StdEncoding.EncodeToString(key)
	}

	tm := &SecureTokenManager{
		secretKey:   []byte(secretKey),
		tokens:      make(map[string]*TokenData),
		tokenExpiry: time.Duration(expiryHours) * time.Hour,
		csrfTokens:  make(map[string]time.Time),
		done:        make(chan struct{}),
	}

	// Start cleanup routine
	go tm.cleanupRoutine()

	return tm, nil
}

// GenerateCSRFToken creates a new CSRF token
func (tm *SecureTokenManager) GenerateCSRFToken() (string, error) {
	tm.csrfMu.Lock()
	defer tm.csrfMu.Unlock()

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(b)
	tm.csrfTokens[token] = time.Now().Add(1 * time.Hour)

	return token, nil
}

// ValidateCSRFToken checks if a CSRF token is valid
func (tm *SecureTokenManager) ValidateCSRFToken(token string) bool {
	tm.csrfMu.Lock()
	defer tm.csrfMu.Unlock()

	expiry, exists := tm.csrfTokens[token]
	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		delete(tm.csrfTokens, token)
		return false
	}

	// Remove token after use (one-time use)
	delete(tm.csrfTokens, token)
	return true
}

// GenerateToken creates a new authentication token
func (tm *SecureTokenManager) GenerateToken(username string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Generate random token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate authentication token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(b)

	// Store token data
	tm.tokens[token] = &TokenData{
		Username:  username,
		ExpiresAt: time.Now().Add(tm.tokenExpiry),
	}

	return token, nil
}

// ValidateToken checks if a token is valid
func (tm *SecureTokenManager) ValidateToken(token string) (*TokenData, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	data, exists := tm.tokens[token]
	if !exists {
		return nil, false
	}

	if time.Now().After(data.ExpiresAt) {
		return nil, false
	}

	return data, true
}

// RevokeToken invalidates a token
func (tm *SecureTokenManager) RevokeToken(token string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tokens, token)
}

// cleanupExpiredTokens removes expired tokens from memory
func (tm *SecureTokenManager) cleanupExpiredTokens() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	for token, data := range tm.tokens {
		if now.After(data.ExpiresAt) {
			delete(tm.tokens, token)
		}
	}
}

// Close stops the cleanup goroutine. It is safe to call multiple times.
func (tm *SecureTokenManager) Close() {
	tm.closeOnce.Do(func() { close(tm.done) })
}

// cleanupRoutine periodically cleans up expired tokens and CSRF tokens
func (tm *SecureTokenManager) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			tm.cleanupExpiredTokens()
			tm.cleanupExpiredCSRFTokens()
		case <-tm.done:
			return
		}
	}
}

func (tm *SecureTokenManager) cleanupExpiredCSRFTokens() {
	tm.csrfMu.Lock()
	defer tm.csrfMu.Unlock()

	now := time.Now()
	for token, expiry := range tm.csrfTokens {
		if now.After(expiry) {
			delete(tm.csrfTokens, token)
		}
	}
}

// SecureLoginHandler handles authentication with security best practices
type SecureLoginHandler struct {
	config         *SecureAuthConfig
	tokenManager   *SecureTokenManager
	rateLimiter    *RateLimiter
	trustedProxies []*net.IPNet
}

func NewSecureLoginHandler(
	config *SecureAuthConfig, tm *SecureTokenManager, rl *RateLimiter, trustedProxies ...*net.IPNet,
) *SecureLoginHandler {
	return &SecureLoginHandler{
		config:         config,
		tokenManager:   tm,
		rateLimiter:    rl,
		trustedProxies: trustedProxies,
	}
}

func (h *SecureLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check rate limiting by IP
	clientIP := getClientIP(r, h.trustedProxies...)
	if !h.rateLimiter.Allow(clientIP) {
		http.Error(w, "Too many login attempts", http.StatusTooManyRequests)
		return
	}

	// Validate CSRF token for all requests
	csrfToken := r.Header.Get("X-CSRF-Token")
	if csrfToken == "" {
		http.Error(w, "CSRF token required", http.StatusForbidden)
		return
	}
	if !h.tokenManager.ValidateCSRFToken(csrfToken) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate username with constant-time comparison
	usernameMatch := subtle.ConstantTimeCompare([]byte(credentials.Username), []byte(h.config.Username)) == 1

	// Validate password with bcrypt (already constant-time)
	passwordErr := bcrypt.CompareHashAndPassword([]byte(h.config.PasswordHash), []byte(credentials.Password))
	passwordMatch := passwordErr == nil

	// Combine results to prevent timing attacks
	if !usernameMatch || !passwordMatch {
		// Add slight delay to prevent brute force
		time.Sleep(100 * time.Millisecond)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate auth token
	token, err := h.tokenManager.GenerateToken(credentials.Username)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Generate new CSRF token for the session
	csrfToken, err = h.tokenManager.GenerateCSRFToken()
	if err != nil {
		http.Error(w, "Failed to generate CSRF token", http.StatusInternalServerError)
		return
	}

	// Set secure cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == httpsProto,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.tokenManager.tokenExpiry.Seconds()),
	})

	// Return tokens in response
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"csrf_token": csrfToken,
		"expires_in": h.tokenManager.tokenExpiry.Seconds(),
	})
}

// getClientIP extracts the client IP address from the request.
// Forwarded headers (X-Forwarded-For, X-Real-IP) are only trusted when the
// direct connection comes from a trusted proxy (loopback by default, or any
// CIDR in trustedProxies). For other connections, the headers are ignored to
// prevent IP spoofing.
func getClientIP(r *http.Request, trustedProxies ...*net.IPNet) string {
	remoteIP := extractRemoteIP(r.RemoteAddr)

	// Only trust forwarded headers from trusted proxies
	if isTrustedProxy(remoteIP, trustedProxies) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
	}

	return remoteIP
}

// HashPassword generates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	// Use cost 12 for good security/performance balance
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("failed to generate password hash: %w", err)
	}
	return string(hash), nil
}
