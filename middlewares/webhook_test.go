// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core"
)

func TestNewWebhook_WithConfig(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{
		Name:   "test",
		Preset: "slack",
		ID:     "T12345/B67890",
		Secret: "xoxb-test-secret",
	}
	loader := NewPresetLoader(nil)

	middleware, err := NewWebhook(config, loader)
	require.NoError(t, err)
	assert.NotNil(t, middleware)
}

func TestNewWebhook_NilConfig(t *testing.T) {
	t.Parallel()

	loader := NewPresetLoader(nil)
	middleware, err := NewWebhook(nil, loader)

	require.NoError(t, err)
	assert.Nil(t, middleware)
}

func TestNewWebhook_InvalidPreset(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{
		Name:   "test",
		Preset: "nonexistent-preset",
	}
	loader := NewPresetLoader(nil)

	middleware, err := NewWebhook(config, loader)
	require.Error(t, err)
	assert.Nil(t, middleware)
}

func TestWebhook_ContinueOnStop(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{
		Name:   "test",
		Preset: "slack",
		ID:     "T12345/B67890",
		Secret: "xoxb-test-secret",
	}
	loader := NewPresetLoader(nil)

	middleware, err := NewWebhook(config, loader)
	require.NoError(t, err)

	webhook, ok := middleware.(*Webhook)
	assert.True(t, ok)
	assert.True(t, webhook.ContinueOnStop())
}

func TestWebhookManager_Creation(t *testing.T) {
	t.Parallel()

	globalConfig := DefaultWebhookGlobalConfig()
	manager := NewWebhookManager(globalConfig)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.webhooks)
}

func TestWebhookManager_Register(t *testing.T) {
	t.Parallel()

	manager := NewWebhookManager(DefaultWebhookGlobalConfig())

	config := &WebhookConfig{
		Name:   "test-webhook",
		Preset: "slack",
	}

	err := manager.Register(config)
	require.NoError(t, err)
}

func TestWebhookManager_RegisterEmptyName(t *testing.T) {
	t.Parallel()

	manager := NewWebhookManager(DefaultWebhookGlobalConfig())

	config := &WebhookConfig{
		Name:   "",
		Preset: "slack",
	}

	err := manager.Register(config)
	assert.Error(t, err)
}

func TestWebhookManager_Get(t *testing.T) {
	t.Parallel()

	manager := NewWebhookManager(DefaultWebhookGlobalConfig())

	config := &WebhookConfig{
		Name:   "slack-alerts",
		Preset: "slack",
	}

	err := manager.Register(config)
	require.NoError(t, err)

	webhook, ok := manager.Get("slack-alerts")
	assert.True(t, ok)
	assert.NotNil(t, webhook)
}

func TestWebhookManager_GetNonExistent(t *testing.T) {
	t.Parallel()

	manager := NewWebhookManager(DefaultWebhookGlobalConfig())

	webhook, ok := manager.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, webhook)
}

func TestWebhookConfig_ShouldNotify(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{Trigger: TriggerError}
	assert.True(t, config.ShouldNotify(true, false))
	assert.False(t, config.ShouldNotify(false, false))

	config = &WebhookConfig{Trigger: TriggerSuccess}
	assert.True(t, config.ShouldNotify(false, false))
	assert.False(t, config.ShouldNotify(true, false))

	config = &WebhookConfig{Trigger: TriggerAlways}
	assert.True(t, config.ShouldNotify(true, false))
	assert.True(t, config.ShouldNotify(false, false))
}

func TestWebhook_SendsRequest(t *testing.T) {
	// Note: Not parallel - modifies global security config
	SetValidateWebhookURLForTest(func(rawURL string) error { return nil })
	defer SetValidateWebhookURLForTest(ValidateWebhookURLImpl)

	SetTransportFactoryForTest(func() *http.Transport { return http.DefaultTransport.(*http.Transport).Clone() })
	defer SetTransportFactoryForTest(NewSafeTransport)

	var receivedBody string
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &WebhookConfig{
		Name:       "test",
		Preset:     "slack",
		ID:         "T12345/B67890",
		Secret:     "xoxb-test-secret",
		URL:        server.URL,
		Trigger:    TriggerAlways,
		Timeout:    5 * time.Second,
		RetryCount: 0,
	}

	loader := NewPresetLoader(nil)
	middleware, err := NewWebhook(config, loader)
	require.NoError(t, err)

	webhook := middleware.(*Webhook)

	job := &TestJob{}
	job.Name = "test-job"
	job.Command = "echo hello"

	sh := core.NewScheduler(newDiscardLogger())
	e, err := core.NewExecution()
	require.NoError(t, err)

	ctx := core.NewContext(sh, job, e)
	ctx.Start()
	ctx.Stop(nil)

	err = webhook.send(ctx)
	require.NoError(t, err)

	assert.NotEmpty(t, receivedBody)
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
}

func TestWebhook_Retry(t *testing.T) {
	// Note: Not parallel - modifies global security config
	SetValidateWebhookURLForTest(func(rawURL string) error { return nil })
	defer SetValidateWebhookURLForTest(ValidateWebhookURLImpl)

	SetTransportFactoryForTest(func() *http.Transport { return http.DefaultTransport.(*http.Transport).Clone() })
	defer SetTransportFactoryForTest(NewSafeTransport)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &WebhookConfig{
		Name:       "test",
		Preset:     "slack",
		ID:         "T12345/B67890",
		Secret:     "xoxb-test-secret",
		URL:        server.URL,
		Trigger:    TriggerAlways,
		Timeout:    5 * time.Second,
		RetryCount: 3,
		RetryDelay: 10 * time.Millisecond,
	}

	loader := NewPresetLoader(nil)
	middleware, err := NewWebhook(config, loader)
	require.NoError(t, err)

	webhook := middleware.(*Webhook)

	job := &TestJob{}
	job.Name = "test-job"
	job.Command = "echo hello"

	sh := core.NewScheduler(newDiscardLogger())
	e, err := core.NewExecution()
	require.NoError(t, err)

	ctx := core.NewContext(sh, job, e)
	ctx.Start()
	ctx.Stop(nil)

	err = webhook.sendWithRetry(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestWebhook_TriggerError_OnlyOnFailure(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{
		Name:       "test",
		Preset:     "slack",
		URL:        "https://example.com",
		Trigger:    TriggerError,
		Timeout:    5 * time.Second,
		RetryCount: 0,
	}

	assert.False(t, config.ShouldNotify(false, false))
	assert.True(t, config.ShouldNotify(true, false))
}

func TestWebhook_TriggerSuccess_OnlyOnSuccess(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{
		Name:       "test",
		Preset:     "slack",
		URL:        "https://example.com/webhook",
		Trigger:    TriggerSuccess,
		Timeout:    5 * time.Second,
		RetryCount: 0,
	}

	assert.True(t, config.ShouldNotify(false, false))
	assert.False(t, config.ShouldNotify(true, false))
}

func TestWebhook_TriggerAlways(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{
		Name:       "test",
		Preset:     "slack",
		URL:        "https://example.com/webhook",
		Trigger:    TriggerAlways,
		Timeout:    5 * time.Second,
		RetryCount: 0,
	}

	assert.True(t, config.ShouldNotify(false, false))
	assert.True(t, config.ShouldNotify(true, false))
}

func TestWebhook_BuildWebhookData_Success(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{
		Name:   "test",
		Preset: "slack",
		ID:     "T12345/B67890",
		Secret: "xoxb-test-secret",
	}
	loader := NewPresetLoader(nil)

	middleware, err := NewWebhook(config, loader)
	require.NoError(t, err)

	webhook := middleware.(*Webhook)

	job := &TestJob{}
	job.Name = "test-job"
	job.Command = "echo hello"

	sh := core.NewScheduler(newDiscardLogger())
	e, err := core.NewExecution()
	require.NoError(t, err)

	ctx := core.NewContext(sh, job, e)
	ctx.Start()
	ctx.Stop(nil)

	data := webhook.buildWebhookData(ctx)

	assert.Equal(t, "test-job", data.Job.Name)
	assert.False(t, data.Execution.Failed)
	assert.Equal(t, "successful", data.Execution.Status)
}

func TestWebhook_BuildWebhookData_Error(t *testing.T) {
	t.Parallel()

	config := &WebhookConfig{
		Name:   "test",
		Preset: "slack",
		ID:     "T12345/B67890",
		Secret: "xoxb-test-secret",
	}
	loader := NewPresetLoader(nil)

	middleware, err := NewWebhook(config, loader)
	require.NoError(t, err)

	webhook := middleware.(*Webhook)

	job := &TestJob{}
	job.Name = "failing-job"
	job.Command = "exit 1"

	sh := core.NewScheduler(newDiscardLogger())
	e, err := core.NewExecution()
	require.NoError(t, err)

	ctx := core.NewContext(sh, job, e)
	ctx.Start()
	ctx.Stop(errors.New("command failed"))

	data := webhook.buildWebhookData(ctx)

	assert.Equal(t, "failing-job", data.Job.Name)
	assert.True(t, data.Execution.Failed)
	assert.Equal(t, "failed", data.Execution.Status)
}

func TestWebhookManager_GetMiddlewares_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		register    []string
		lookup      []string
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name:      "known webhook returns middleware",
			register:  []string{"alert"},
			lookup:    []string{"alert"},
			wantCount: 1,
		},
		{
			name:        "unknown webhook returns error",
			register:    []string{},
			lookup:      []string{"missing"},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:      "empty name in lookup is skipped",
			register:  []string{"alert"},
			lookup:    []string{"", "alert", "  "},
			wantCount: 1,
		},
		{
			name:      "multiple known webhooks",
			register:  []string{"slack-alert", "discord-alert"},
			lookup:    []string{"slack-alert", "discord-alert"},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manager := NewWebhookManager(DefaultWebhookGlobalConfig())

			for _, name := range tt.register {
				err := manager.Register(&WebhookConfig{
					Name:   name,
					Preset: "slack",
					ID:     "T00000/B00000",
					Secret: "xoxb-secret",
				})
				require.NoError(t, err)
			}

			mws, err := manager.GetMiddlewares(tt.lookup)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Len(t, mws, tt.wantCount)
		})
	}
}

func TestWebhookManager_GetGlobalMiddlewares_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		webhooks  string
		register  []string
		wantCount int
		wantNil   bool
		wantErr   bool
	}{
		{name: "empty webhooks string returns nil", wantNil: true},
		{name: "with names resolves", webhooks: "a1,a2", register: []string{"a1", "a2"}, wantCount: 2},
		{name: "unknown name errors", webhooks: "nonexistent", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gc := DefaultWebhookGlobalConfig()
			gc.Webhooks = tt.webhooks
			manager := NewWebhookManager(gc)

			for _, name := range tt.register {
				_ = manager.Register(&WebhookConfig{Name: name, Preset: "slack", ID: "T0/B0", Secret: "s"})
			}

			mws, err := manager.GetGlobalMiddlewares()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, mws)
				return
			}
			assert.Len(t, mws, tt.wantCount)
		})
	}
}

func TestNewWebhookMiddleware_Variants(t *testing.T) {
	t.Parallel()

	assert.Nil(t, NewWebhookMiddleware(nil), "nil input should return nil")
	assert.Nil(t, NewWebhookMiddleware([]core.Middleware{}), "empty input should return nil")
	assert.NotNil(t, NewWebhookMiddleware([]core.Middleware{&stubMW{}}), "non-empty should return middleware")
}

func TestWebhookMiddleware_ContinueOnStop(t *testing.T) {
	t.Parallel()
	wm := &WebhookMiddleware{webhooks: []core.Middleware{&stubMW{}}}
	assert.True(t, wm.ContinueOnStop())
}

func TestWebhookMiddleware_Run_DispatchesToAll(t *testing.T) {
	t.Parallel()

	var called []string
	s1 := &stubMW{onRun: func() { called = append(called, "s1") }}
	s2 := &stubMW{onRun: func() { called = append(called, "s2") }}
	wm := &WebhookMiddleware{webhooks: []core.Middleware{s1, s2}}

	ctx, _ := setupTestContext(t)
	ctx.Start()
	err := wm.Run(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"s1", "s2"}, called)
}

func TestGetJobType_AllVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		job      core.Job
		expected string
	}{
		{"exec job", &core.ExecJob{}, "exec"},
		{"run job", &core.RunJob{}, "run"},
		{"local job", &core.LocalJob{}, "local"},
		{"run-service job", &core.RunServiceJob{}, "run-service"},
		{"unknown job", &TestJob{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, getJobType(tt.job))
		})
	}
}

func TestGetExecutionStatus_AllVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exec     *core.Execution
		expected string
	}{
		{"failed", &core.Execution{Failed: true}, "failed"},
		{"skipped", &core.Execution{Skipped: true}, "skipped"},
		{"successful", &core.Execution{}, "successful"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, getExecutionStatus(tt.exec))
		})
	}
}

func TestParseWebhookNames_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", nil},
		{"single name", "slack", []string{"slack"}},
		{"multiple names", "slack,discord", []string{"slack", "discord"}},
		{"with spaces", " slack , discord , teams ", []string{"slack", "discord", "teams"}},
		{"empty segments", "slack,,discord,", []string{"slack", "discord"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseWebhookNames(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type stubMW struct {
	onRun func()
}

func (s *stubMW) ContinueOnStop() bool { return true }
func (s *stubMW) Run(_ *core.Context) error {
	if s.onRun != nil {
		s.onRun()
	}
	return nil
}
