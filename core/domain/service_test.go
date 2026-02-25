// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package domain

import "testing"

func TestTaskState_IsTerminalState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state TaskState
		want  bool
	}{
		// Terminal states
		{TaskStateComplete, true},
		{TaskStateFailed, true},
		{TaskStateRejected, true},
		{TaskStateShutdown, true},
		{TaskStateOrphaned, true},

		// Non-terminal states
		{TaskStateNew, false},
		{TaskStatePending, false},
		{TaskStateAssigned, false},
		{TaskStateAccepted, false},
		{TaskStatePreparing, false},
		{TaskStateReady, false},
		{TaskStateStarting, false},
		{TaskStateRunning, false},
		{TaskStateRemove, false},

		// Unknown state
		{TaskState("unknown"), false},
		{TaskState(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			t.Parallel()
			if got := tt.state.IsTerminalState(); got != tt.want {
				t.Errorf("TaskState(%q).IsTerminalState() = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}
