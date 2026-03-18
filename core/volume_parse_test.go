// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/netresearch/ofelia/core/domain"
)

func TestParseBindMount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantType   domain.MountType
		wantSource string
		wantTarget string
		wantRO     bool
	}{
		{
			name:       "bind mount read-write",
			input:      "/host/path:/container/path",
			wantType:   domain.MountTypeBind,
			wantSource: "/host/path",
			wantTarget: "/container/path",
			wantRO:     false,
		},
		{
			name:       "bind mount read-only",
			input:      "/host/path:/container/path:ro",
			wantType:   domain.MountTypeBind,
			wantSource: "/host/path",
			wantTarget: "/container/path",
			wantRO:     true,
		},
		{
			name:       "bind mount explicit rw",
			input:      "/host/path:/container/path:rw",
			wantType:   domain.MountTypeBind,
			wantSource: "/host/path",
			wantTarget: "/container/path",
			wantRO:     false,
		},
		{
			name:       "named volume",
			input:      "myvolume:/app/data",
			wantType:   domain.MountTypeVolume,
			wantSource: "myvolume",
			wantTarget: "/app/data",
			wantRO:     false,
		},
		{
			name:       "named volume read-only",
			input:      "myvolume:/app/data:ro",
			wantType:   domain.MountTypeVolume,
			wantSource: "myvolume",
			wantTarget: "/app/data",
			wantRO:     true,
		},
		{
			name:       "single file bind",
			input:      "/host/script.sh:/script.sh",
			wantType:   domain.MountTypeBind,
			wantSource: "/host/script.sh",
			wantTarget: "/script.sh",
			wantRO:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := parseBindMount(tc.input)
			if m.Type != tc.wantType {
				t.Errorf("Type: want %q, got %q", tc.wantType, m.Type)
			}
			if m.Source != tc.wantSource {
				t.Errorf("Source: want %q, got %q", tc.wantSource, m.Source)
			}
			if m.Target != tc.wantTarget {
				t.Errorf("Target: want %q, got %q", tc.wantTarget, m.Target)
			}
			if m.ReadOnly != tc.wantRO {
				t.Errorf("ReadOnly: want %v, got %v", tc.wantRO, m.ReadOnly)
			}
		})
	}
}
