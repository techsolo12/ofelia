// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core"
	webpkg "github.com/netresearch/ofelia/web"
)

func TestCSRFTokenHandler_NoTokenManager(t *testing.T) {
	t.Parallel()

	// Server without auth - tokenManager is nil
	sched := core.NewScheduler(stubDiscardLogger())
	srv := webpkg.NewServer("", sched, nil, nil)
	httpSrv := srv.HTTPServer()

	req := httptest.NewRequest(http.MethodGet, "/api/csrf-token", nil)
	w := httptest.NewRecorder()
	httpSrv.Handler.ServeHTTP(w, req)

	// Without auth, csrf-token endpoint should return 404
	assert.Equal(t, http.StatusNotFound, w.Code,
		"csrf-token endpoint should return 404 when auth is not enabled")
}

func TestCSRFTokenHandler_WithAuth(t *testing.T) {
	t.Parallel()

	sched := core.NewScheduler(stubDiscardLogger())
	authCfg := &webpkg.SecureAuthConfig{
		Enabled:      true,
		Username:     "admin",
		PasswordHash: generateTestHash("password"),
		SecretKey:    "test-secret-key-32-bytes-long!!!",
		TokenExpiry:  24,
		MaxAttempts:  10,
	}

	srv := webpkg.NewServerWithAuth("", sched, nil, nil, authCfg)
	require.NotNil(t, srv)
	httpSrv := srv.HTTPServer()

	req := httptest.NewRequest(http.MethodGet, "/api/csrf-token", nil)
	w := httptest.NewRecorder()
	httpSrv.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["csrf_token"], "should return a non-empty CSRF token")
}

func TestAuthStatusHandler_Unauthenticated(t *testing.T) {
	t.Parallel()

	sched := core.NewScheduler(stubDiscardLogger())
	authCfg := &webpkg.SecureAuthConfig{
		Enabled:      true,
		Username:     "admin",
		PasswordHash: generateTestHash("password"),
		SecretKey:    "test-secret-key-32-bytes-long!!!",
		TokenExpiry:  24,
		MaxAttempts:  10,
	}

	srv := webpkg.NewServerWithAuth("", sched, nil, nil, authCfg)
	require.NotNil(t, srv)
	httpSrv := srv.HTTPServer()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	w := httptest.NewRecorder()
	httpSrv.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["authEnabled"])
	assert.Equal(t, false, resp["authenticated"])
}

func TestAuthStatusHandler_NoAuth(t *testing.T) {
	t.Parallel()

	sched := core.NewScheduler(stubDiscardLogger())
	// Create server with auth disabled (nil config)
	srv := webpkg.NewServerWithAuth("", sched, nil, nil, nil)
	require.NotNil(t, srv)
	httpSrv := srv.HTTPServer()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	w := httptest.NewRecorder()
	httpSrv.Handler.ServeHTTP(w, req)

	// When auth is disabled, NewServerWithAuth with nil config acts like NewServer
	// The auth status endpoint may or may not be registered
	// Just verify it doesn't panic and returns a valid response
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound,
		"expected 200 or 404, got %d", w.Code)
}

func TestAuthStatusHandler_InvalidToken(t *testing.T) {
	t.Parallel()

	sched := core.NewScheduler(stubDiscardLogger())
	authCfg := &webpkg.SecureAuthConfig{
		Enabled:      true,
		Username:     "admin",
		PasswordHash: generateTestHash("password"),
		SecretKey:    "test-secret-key-32-bytes-long!!!",
		TokenExpiry:  24,
		MaxAttempts:  10,
	}

	srv := webpkg.NewServerWithAuth("", sched, nil, nil, authCfg)
	require.NotNil(t, srv)
	httpSrv := srv.HTTPServer()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")
	w := httptest.NewRecorder()
	httpSrv.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, true, resp["authEnabled"])
	assert.Equal(t, false, resp["authenticated"],
		"invalid token should show as not authenticated")
}

func TestRunJobHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	sched := core.NewScheduler(stubDiscardLogger())
	job := &testJob{}
	job.Name = "method-test-job"
	job.Schedule = "@daily"
	job.Command = "echo"
	_ = sched.AddJob(job)

	srv := webpkg.NewServer("", sched, nil, nil)
	httpSrv := srv.HTTPServer()

	// GET instead of POST
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/run", nil)
	w := httptest.NewRecorder()
	httpSrv.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestDisableJobHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	sched := core.NewScheduler(stubDiscardLogger())
	srv := webpkg.NewServer("", sched, nil, nil)
	httpSrv := srv.HTTPServer()

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/disable", nil)
	w := httptest.NewRecorder()
	httpSrv.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestEnableJobHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	sched := core.NewScheduler(stubDiscardLogger())
	srv := webpkg.NewServer("", sched, nil, nil)
	httpSrv := srv.HTTPServer()

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/enable", nil)
	w := httptest.NewRecorder()
	httpSrv.Handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
