// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package domain

import "testing"

func TestEvent_IsContainerStopEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event Event
		want  bool
	}{
		{
			name:  "die_action",
			event: Event{Type: EventTypeContainer, Action: EventActionDie},
			want:  true,
		},
		{
			name:  "kill_action",
			event: Event{Type: EventTypeContainer, Action: EventActionKill},
			want:  true,
		},
		{
			name:  "stop_action",
			event: Event{Type: EventTypeContainer, Action: EventActionStop},
			want:  true,
		},
		{
			name:  "oom_action",
			event: Event{Type: EventTypeContainer, Action: EventActionOOM},
			want:  true,
		},
		{
			name:  "start_action_not_stop",
			event: Event{Type: EventTypeContainer, Action: EventActionStart},
			want:  false,
		},
		{
			name:  "create_action_not_stop",
			event: Event{Type: EventTypeContainer, Action: EventActionCreate},
			want:  false,
		},
		{
			name:  "restart_action_not_stop",
			event: Event{Type: EventTypeContainer, Action: EventActionRestart},
			want:  false,
		},
		{
			name:  "pause_action_not_stop",
			event: Event{Type: EventTypeContainer, Action: EventActionPause},
			want:  false,
		},
		{
			name:  "image_event_with_die_action",
			event: Event{Type: EventTypeImage, Action: EventActionDie},
			want:  false,
		},
		{
			name:  "network_event",
			event: Event{Type: EventTypeNetwork, Action: EventActionStop},
			want:  false,
		},
		{
			name:  "empty_type",
			event: Event{Type: "", Action: EventActionDie},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.event.IsContainerStopEvent(); got != tt.want {
				t.Errorf("IsContainerStopEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvent_GetContainerID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event Event
		want  string
	}{
		{
			name: "container_event",
			event: Event{
				Type:  EventTypeContainer,
				Actor: EventActor{ID: "abc123def456"},
			},
			want: "abc123def456",
		},
		{
			name: "container_event_empty_id",
			event: Event{
				Type:  EventTypeContainer,
				Actor: EventActor{ID: ""},
			},
			want: "",
		},
		{
			name: "image_event",
			event: Event{
				Type:  EventTypeImage,
				Actor: EventActor{ID: "sha256:deadbeef"},
			},
			want: "",
		},
		{
			name: "network_event",
			event: Event{
				Type:  EventTypeNetwork,
				Actor: EventActor{ID: "net-id"},
			},
			want: "",
		},
		{
			name: "volume_event",
			event: Event{
				Type:  EventTypeVolume,
				Actor: EventActor{ID: "vol-id"},
			},
			want: "",
		},
		{
			name:  "empty_event",
			event: Event{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.event.GetContainerID(); got != tt.want {
				t.Errorf("GetContainerID() = %q, want %q", got, tt.want)
			}
		})
	}
}
