// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package mock_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core/adapters/mock"
	"github.com/netresearch/ofelia/core/domain"
)

// ---------------------------------------------------------------------------
// CopyLogs error path (87.5% → 100%)
// ---------------------------------------------------------------------------

func TestContainerService_CopyLogs_LogsError(t *testing.T) {
	t.Parallel()
	cs := mock.NewContainerService()
	cs.OnLogs = func(_ context.Context, _ string, _ domain.LogOptions) (io.ReadCloser, error) {
		return nil, errors.New("logs error")
	}

	var stdout, stderr mockWriter
	err := cs.CopyLogs(context.Background(), "c1", &stdout, &stderr, domain.LogOptions{})
	require.Error(t, err)
}

func TestContainerService_CopyLogs_NilStdout(t *testing.T) {
	t.Parallel()
	cs := mock.NewContainerService()

	err := cs.CopyLogs(context.Background(), "c1", nil, nil, domain.LogOptions{})
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// EventService Subscribe with error (95% → 100%)
// ---------------------------------------------------------------------------

func TestEventService_Subscribe_WithError(t *testing.T) {
	t.Parallel()
	es := mock.NewEventService()
	es.SetSubscribeError(errors.New("subscribe error"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, errCh := es.Subscribe(ctx, domain.EventFilter{})

	select {
	case err := <-errCh:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "subscribe error")
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for error")
	}
}

func TestEventService_Subscribe_ContextCanceled(t *testing.T) {
	t.Parallel()
	es := mock.NewEventService()
	es.SetEvents([]domain.Event{
		{Type: "container", Action: "start"},
	})

	ctx, cancel := context.WithCancel(context.Background())

	eventCh, _ := es.Subscribe(ctx, domain.EventFilter{})

	// Read first event
	select {
	case ev := <-eventCh:
		assert.Equal(t, "container", ev.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Cancel context
	cancel()

	// eventCh should be closed eventually
	select {
	case _, ok := <-eventCh:
		assert.False(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}

// ---------------------------------------------------------------------------
// SubscribeWithCallback error path (63.6% → higher)
// ---------------------------------------------------------------------------

func TestEventService_SubscribeWithCallback_CallbackError(t *testing.T) {
	t.Parallel()
	es := mock.NewEventService()
	es.SetEvents([]domain.Event{
		{Type: "container", Action: "start"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := es.SubscribeWithCallback(ctx, domain.EventFilter{}, func(event domain.Event) error {
		return errors.New("callback error")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "callback error")
}

func TestEventService_SubscribeWithCallback_SubscribeError(t *testing.T) {
	t.Parallel()
	es := mock.NewEventService()
	es.SetSubscribeError(errors.New("subscribe error"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := es.SubscribeWithCallback(ctx, domain.EventFilter{}, func(event domain.Event) error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscribe error")
}

func TestEventService_SubscribeWithCallback_ContextCancel(t *testing.T) {
	t.Parallel()
	es := mock.NewEventService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	err := es.SubscribeWithCallback(ctx, domain.EventFilter{}, func(event domain.Event) error {
		return nil
	})
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// PullAndWait with OnPull error (90.9% → 100%)
// ---------------------------------------------------------------------------

func TestImageService_PullAndWait_PullError(t *testing.T) {
	t.Parallel()
	is := mock.NewImageService()
	is.OnPull = func(_ context.Context, _ domain.PullOptions) (io.ReadCloser, error) {
		return nil, errors.New("pull error")
	}

	err := is.PullAndWait(context.Background(), domain.PullOptions{Repository: "alpine"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pull error")
}

// ---------------------------------------------------------------------------
// WaitForServiceTasks with error (83.3% → 100%)
// ---------------------------------------------------------------------------

func TestSwarmService_WaitForServiceTasks_ListError(t *testing.T) {
	t.Parallel()
	ss := mock.NewSwarmService()
	ss.OnListTasks = func(_ context.Context, _ domain.TaskListOptions) ([]domain.Task, error) {
		return nil, errors.New("list tasks error")
	}

	_, err := ss.WaitForServiceTasks(context.Background(), "svc-1", 5*time.Second)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Ping/Version with errors (90% → 100%)
// ---------------------------------------------------------------------------

func TestSystemService_Ping_Error(t *testing.T) {
	t.Parallel()
	ss := mock.NewSystemService()
	ss.SetPingError(errors.New("ping failed"))

	result, err := ss.Ping(context.Background())
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestSystemService_Version_Error(t *testing.T) {
	t.Parallel()
	ss := mock.NewSystemService()
	ss.SetVersionError(errors.New("version failed"))

	result, err := ss.Version(context.Background())
	require.Error(t, err)
	assert.Nil(t, result)
}

// mockWriter is a simple io.Writer for tests
type mockWriter struct {
	data []byte
}

func (w *mockWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}
