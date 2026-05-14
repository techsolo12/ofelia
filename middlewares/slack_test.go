// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package middlewares

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core"
)

func TestNewSlackEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, NewSlack(&SlackConfig{}))
}

func TestSlackRunSuccess(t *testing.T) {
	t.Parallel()
	ctx, _ := setupTestContext(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m slackMessage
		_ = json.Unmarshal([]byte(r.FormValue(slackPayloadVar)), &m)
		assert.Equal(t, "Execution successful", m.Attachments[0].Title)
	}))
	defer ts.Close()

	ctx.Start()
	ctx.Stop(nil)

	m := NewSlack(&SlackConfig{SlackWebhook: ts.URL})
	require.NoError(t, m.Run(ctx))
}

func TestSlackRunSuccessFailed(t *testing.T) {
	t.Parallel()
	ctx, _ := setupTestContext(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m slackMessage
		_ = json.Unmarshal([]byte(r.FormValue(slackPayloadVar)), &m)
		assert.Equal(t, "Execution failed", m.Attachments[0].Title)
	}))
	defer ts.Close()

	ctx.Start()
	ctx.Stop(errors.New("foo"))

	m := NewSlack(&SlackConfig{SlackWebhook: ts.URL})
	require.NoError(t, m.Run(ctx))
}

func TestSlackRunSuccessOnError(t *testing.T) {
	t.Parallel()
	ctx, _ := setupTestContext(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	}))
	defer ts.Close()

	ctx.Start()
	ctx.Stop(nil)

	m := NewSlack(&SlackConfig{SlackWebhook: ts.URL, SlackOnlyOnError: new(true)})
	require.NoError(t, m.Run(ctx))
}

func TestSlackCustomHTTPClient(t *testing.T) {
	t.Parallel()
	ctx, _ := setupTestContext(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m slackMessage
		_ = json.Unmarshal([]byte(r.FormValue(slackPayloadVar)), &m)
		assert.Equal(t, "Execution successful", m.Attachments[0].Title)
	}))
	defer ts.Close()

	ctx.Start()
	ctx.Stop(nil)

	m := NewSlack(&SlackConfig{SlackWebhook: ts.URL}).(*Slack)
	custom := ts.Client()
	custom.Timeout = 2 * time.Second
	m.Client = custom

	require.NoError(t, m.Run(ctx))
}

func TestSlack_ContinueOnStop(t *testing.T) {
	t.Parallel()
	m := &Slack{}
	assert.True(t, m.ContinueOnStop(), "Slack.ContinueOnStop() should return true")
}

// TestSlack_FallbackClient_UsesTransportFactory is a regression guard for #630.
// The (deprecated) Slack middleware constructs its default *http.Client with no
// custom Transport, so notifications fall back to http.DefaultTransport instead
// of the webhook stack's TLS-aware transport. NewSlack must wire the fallback
// Client.Transport through TransportFactory() for defense-in-depth during the
// deprecation window.
//
// Not parallel: installs a sentinel TransportFactory via the global hook.
func TestSlack_FallbackClient_UsesTransportFactory(t *testing.T) {
	sentinel := &http.Transport{}
	SetTransportFactoryForTest(func() *http.Transport { return sentinel })
	t.Cleanup(func() { SetTransportFactoryForTest(NewSafeTransport) })

	m := NewSlack(&SlackConfig{SlackWebhook: "https://example.com/hook"}).(*Slack)
	require.NotNil(t, m.Client, "NewSlack must construct a default *http.Client")
	assert.Same(t, sentinel, m.Client.Transport,
		"NewSlack fallback client must obtain its Transport from TransportFactory()")
}

func TestSlackDedupSuppressesDuplicateErrors(t *testing.T) {
	t.Parallel()

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	defer ts.Close()

	dedup := NewNotificationDedup(time.Hour)

	job := &TestJob{}
	sh := core.NewScheduler(newDiscardLogger())
	e, err := core.NewExecution()
	require.NoError(t, err)
	ctx := core.NewContext(sh, job, e)

	ctx.Start()
	ctx.Stop(errors.New("test error"))

	m := NewSlack(&SlackConfig{SlackWebhook: ts.URL, Dedup: dedup}).(*Slack)
	require.NoError(t, m.Run(ctx))
	assert.Equal(t, 1, callCount)

	e2, _ := core.NewExecution()
	ctx2 := core.NewContext(sh, job, e2)
	ctx2.Start()
	ctx2.Stop(errors.New("test error"))
	require.NoError(t, m.Run(ctx2))
	assert.Equal(t, 1, callCount) // Still 1, not 2 (suppressed)

	e3, _ := core.NewExecution()
	ctx3 := core.NewContext(sh, job, e3)
	ctx3.Start()
	ctx3.Stop(errors.New("different error"))
	require.NoError(t, m.Run(ctx3))
	assert.Equal(t, 2, callCount) // Now 2
}

func TestSlackDedupAllowsSuccessNotifications(t *testing.T) {
	t.Parallel()

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	defer ts.Close()

	dedup := NewNotificationDedup(time.Hour)

	job := &TestJob{}
	sh := core.NewScheduler(newDiscardLogger())
	e, _ := core.NewExecution()
	ctx := core.NewContext(sh, job, e)

	ctx.Start()
	ctx.Stop(nil)

	m := NewSlack(&SlackConfig{SlackWebhook: ts.URL, Dedup: dedup}).(*Slack)
	require.NoError(t, m.Run(ctx))
	assert.Equal(t, 1, callCount)

	e2, _ := core.NewExecution()
	ctx2 := core.NewContext(sh, job, e2)
	ctx2.Start()
	ctx2.Stop(nil)
	require.NoError(t, m.Run(ctx2))
	assert.Equal(t, 2, callCount)
}

func TestSlackNoDedupWhenNotConfigured(t *testing.T) {
	t.Parallel()

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	}))
	defer ts.Close()

	job := &TestJob{}
	sh := core.NewScheduler(newDiscardLogger())
	e, _ := core.NewExecution()
	ctx := core.NewContext(sh, job, e)

	m := NewSlack(&SlackConfig{SlackWebhook: ts.URL}).(*Slack)

	ctx.Start()
	ctx.Stop(errors.New("test error"))
	require.NoError(t, m.Run(ctx))
	assert.Equal(t, 1, callCount)

	e2, _ := core.NewExecution()
	ctx2 := core.NewContext(sh, job, e2)
	ctx2.Start()
	ctx2.Stop(errors.New("test error"))
	require.NoError(t, m.Run(ctx2))
	assert.Equal(t, 2, callCount)
}
