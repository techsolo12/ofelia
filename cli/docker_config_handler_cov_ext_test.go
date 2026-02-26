// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/test"
)

// --- watchConfig ---

func TestWatchConfig_ImmediateCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	logger := test.NewTestLogger()
	cfg := NewConfig(logger)
	cfg.configPath = "/nonexistent/config.ini"

	handler := &DockerHandler{
		ctx:                ctx,
		cancel:             cancel,
		logger:             logger,
		notifier:           cfg,
		configPollInterval: 100 * time.Millisecond,
	}

	// watchConfig should return immediately on canceled context
	done := make(chan struct{})
	go func() {
		handler.watchConfig()
		close(done)
	}()

	select {
	case <-done:
		// Returned as expected
	case <-time.After(2 * time.Second):
		t.Fatal("watchConfig did not return on canceled context")
	}
}

func TestWatchConfig_ZeroInterval(t *testing.T) {
	t.Parallel()

	handler := &DockerHandler{
		configPollInterval: 0,
		logger:             test.NewTestLogger(),
	}

	// watchConfig should return immediately with zero interval
	done := make(chan struct{})
	go func() {
		handler.watchConfig()
		close(done)
	}()

	select {
	case <-done:
		// Returned as expected
	case <-time.After(2 * time.Second):
		t.Fatal("watchConfig did not return with zero interval")
	}
}

func TestWatchConfig_NegativeInterval(t *testing.T) {
	t.Parallel()

	handler := &DockerHandler{
		configPollInterval: -1,
		logger:             test.NewTestLogger(),
	}

	// watchConfig should return immediately with negative interval
	done := make(chan struct{})
	go func() {
		handler.watchConfig()
		close(done)
	}()

	select {
	case <-done:
		// Returned as expected
	case <-time.After(2 * time.Second):
		t.Fatal("watchConfig did not return with negative interval")
	}
}

// --- watchContainerPolling ---

func TestWatchContainerPolling_ImmediateCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := &DockerHandler{
		ctx:                ctx,
		cancel:             cancel,
		logger:             test.NewTestLogger(),
		dockerPollInterval: 100 * time.Millisecond,
	}

	done := make(chan struct{})
	go func() {
		handler.watchContainerPolling()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchContainerPolling did not return on canceled context")
	}
}

func TestWatchContainerPolling_ZeroInterval(t *testing.T) {
	t.Parallel()

	handler := &DockerHandler{
		dockerPollInterval: 0,
		logger:             test.NewTestLogger(),
	}

	done := make(chan struct{})
	go func() {
		handler.watchContainerPolling()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchContainerPolling did not return with zero interval")
	}
}

// --- startFallbackPolling ---

func TestStartFallbackPolling_AlreadyActive(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &DockerHandler{
		ctx:                   ctx,
		cancel:                cancel,
		logger:                test.NewTestLogger(),
		pollingFallback:       100 * time.Millisecond,
		fallbackPollingActive: true,
	}

	done := make(chan struct{})
	go func() {
		handler.startFallbackPolling()
		close(done)
	}()

	select {
	case <-done:
		// Should return immediately since already active
	case <-time.After(2 * time.Second):
		t.Fatal("startFallbackPolling did not return when already active")
	}
}

func TestStartFallbackPolling_CanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	logger := test.NewTestLogger()
	notifier := &dummyNotifier{}
	provider := &mockDockerProviderForHandler{}

	handler := &DockerHandler{
		ctx:             ctx,
		cancel:          cancel,
		logger:          logger,
		notifier:        notifier,
		dockerProvider:  provider,
		pollingFallback: 50 * time.Millisecond,
	}

	// Cancel after a short delay to let startFallbackPolling start
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		handler.startFallbackPolling()
		close(done)
	}()

	select {
	case <-done:
		assert.False(t, handler.fallbackPollingActive)
	case <-time.After(5 * time.Second):
		t.Fatal("startFallbackPolling did not stop on context cancellation")
	}
}

// --- handleEventStreamError ---

func TestHandleEventStreamError_WithFallback(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := test.NewTestLogger()
	notifier := &dummyNotifier{}
	provider := &mockDockerProviderForHandler{}

	handler := &DockerHandler{
		ctx:             ctx,
		cancel:          cancel,
		logger:          logger,
		notifier:        notifier,
		dockerProvider:  provider,
		pollingFallback: 100 * time.Millisecond,
	}

	handler.handleEventStreamError()

	assert.True(t, handler.eventsFailed)

	// Give fallback goroutine time to start
	time.Sleep(50 * time.Millisecond)

	handler.mu.Lock()
	active := handler.fallbackPollingActive
	handler.mu.Unlock()
	assert.True(t, active, "fallback polling should be active")
}

func TestHandleEventStreamError_AlreadyFailed_Coverage(t *testing.T) {
	t.Parallel()

	handler := &DockerHandler{
		logger:       test.NewTestLogger(),
		eventsFailed: true,
	}

	// Should return early without starting fallback
	handler.handleEventStreamError()
	assert.True(t, handler.eventsFailed)
}

func TestHandleEventStreamError_NoFallback_Coverage(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	dh := &DockerHandler{
		logger:          logger,
		pollingFallback: 0,
	}

	dh.handleEventStreamError()
	assert.True(t, dh.eventsFailed)
	assert.True(t, handler.HasError("Docker event stream failed"))
}

// --- clearEventStreamError ---

func TestClearEventStreamError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &DockerHandler{
		ctx:          ctx,
		cancel:       cancel,
		logger:       test.NewTestLogger(),
		eventsFailed: true,
	}

	handler.clearEventStreamError()
	assert.False(t, handler.eventsFailed)
}

func TestClearEventStreamError_StopsFallback(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fallbackCtx, fallbackCancel := context.WithCancel(ctx)

	handler := &DockerHandler{
		ctx:            ctx,
		cancel:         cancel,
		logger:         test.NewTestLogger(),
		eventsFailed:   true,
		fallbackCancel: fallbackCancel,
	}

	handler.clearEventStreamError()
	assert.False(t, handler.eventsFailed)

	// Verify fallback context was canceled
	select {
	case <-fallbackCtx.Done():
		// Expected
	default:
		t.Fatal("fallback context should have been canceled")
	}
}

// --- buildSDKProvider ---

func TestBuildSDKProvider_InvalidDockerHost_Coverage(t *testing.T) {
	// Not parallel - modifies env
	t.Setenv("DOCKER_HOST", "=")

	handler := &DockerHandler{
		ctx:    context.Background(),
		logger: test.NewTestLogger(),
	}

	_, err := handler.buildSDKProvider()
	require.Error(t, err)
}

// --- Shutdown ---

func TestShutdown_WithProvider(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	provider := &mockDockerProviderForHandler{}

	handler := &DockerHandler{
		ctx:            ctx,
		cancel:         cancel,
		logger:         test.NewTestLogger(),
		dockerProvider: provider,
	}

	err := handler.Shutdown(context.Background())
	require.NoError(t, err)
	assert.Nil(t, handler.dockerProvider)
}

func TestShutdown_NilCancel(t *testing.T) {
	t.Parallel()

	handler := &DockerHandler{
		logger: test.NewTestLogger(),
	}

	// Should not panic
	err := handler.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestShutdown_NilProvider(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	handler := &DockerHandler{
		ctx:    ctx,
		cancel: cancel,
		logger: test.NewTestLogger(),
	}

	err := handler.Shutdown(context.Background())
	assert.NoError(t, err)
}

// --- resolveConfig ---

func TestResolveConfig_WarnsWhenBothEventsAndPolling(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	cfg := &DockerConfig{
		UseEvents:          true,
		DockerPollInterval: 10 * time.Second,
		ConfigPollInterval: 10 * time.Second,
		PollingFallback:    10 * time.Second,
	}

	configPoll, dockerPoll, fallback, useEvents := resolveConfig(cfg, logger)

	assert.Equal(t, 10*time.Second, configPoll)
	assert.Equal(t, 10*time.Second, dockerPoll)
	assert.Equal(t, 10*time.Second, fallback)
	assert.True(t, useEvents)
	assert.True(t, handler.HasWarning("Both Docker events and container polling"))
}

func TestResolveConfig_EventsOnly(t *testing.T) {
	t.Parallel()

	logger, handler := test.NewTestLoggerWithHandler()
	cfg := &DockerConfig{
		UseEvents:          true,
		DockerPollInterval: 0,
		ConfigPollInterval: 10 * time.Second,
		PollingFallback:    10 * time.Second,
	}

	_, _, _, useEvents := resolveConfig(cfg, logger)

	assert.True(t, useEvents)
	assert.False(t, handler.HasWarning("Both Docker events and container polling"))
}

// --- NewDockerHandler edge cases ---

func TestNewDockerHandler_NilContext_Coverage(t *testing.T) {
	t.Parallel()

	provider := &mockDockerProviderForHandler{}
	notifier := &dummyNotifier{}

	handler, err := NewDockerHandler(nil, notifier, test.NewTestLogger(), &DockerConfig{}, provider)
	require.NoError(t, err)
	require.NotNil(t, handler)
	defer func() { _ = handler.Shutdown(context.Background()) }()

	// Context should have been created internally
	assert.NotNil(t, handler.ctx)
}

func TestNewDockerHandler_WithConfigAndEvents(t *testing.T) {
	t.Parallel()

	provider := &mockDockerProviderForHandler{}
	notifier := &dummyNotifier{}

	handler, err := NewDockerHandler(
		context.Background(),
		notifier,
		test.NewTestLogger(),
		&DockerConfig{
			ConfigPollInterval: 100 * time.Millisecond,
			UseEvents:          true,
			DockerPollInterval: 100 * time.Millisecond,
		},
		provider,
	)
	require.NoError(t, err)
	require.NotNil(t, handler)
	defer func() { _ = handler.Shutdown(context.Background()) }()

	assert.True(t, handler.useEvents)
	assert.Equal(t, 100*time.Millisecond, handler.configPollInterval)
	assert.Equal(t, 100*time.Millisecond, handler.dockerPollInterval)
}
