// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/netresearch/ofelia/core/domain"
)

func TestParseVolumeMount(t *testing.T) {
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
		},
		{
			name:       "named volume",
			input:      "myvolume:/app/data",
			wantType:   domain.MountTypeVolume,
			wantSource: "myvolume",
			wantTarget: "/app/data",
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
		},
		{
			name:       "relative path is bind mount",
			input:      "./data:/data",
			wantType:   domain.MountTypeBind,
			wantSource: "./data",
			wantTarget: "/data",
		},
		{
			name:       "dot-dot relative path is bind mount",
			input:      "../config:/config:ro",
			wantType:   domain.MountTypeBind,
			wantSource: "../config",
			wantTarget: "/config",
			wantRO:     true,
		},
		{
			name:       "ro is exact token not substring",
			input:      "/prometheus:/data:rw",
			wantType:   domain.MountTypeBind,
			wantSource: "/prometheus",
			wantTarget: "/data",
			wantRO:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, err := parseVolumeMount(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
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

func TestParseVolumeMount_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"no target", "/host/path"},
		{"empty source", ":/container"},
		{"empty target", "/host:"},
		{"only colon", ":"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseVolumeMount(tc.input)
			if err == nil {
				t.Errorf("expected error for invalid volume %q, got nil", tc.input)
			}
		})
	}
}
