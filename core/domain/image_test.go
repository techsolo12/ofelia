// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package domain

import "testing"

func TestParseRepositoryTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantRepo   string
		wantTag    string
		wantDigest string
	}{
		{
			name:     "image_with_tag",
			input:    "nginx:1.25",
			wantRepo: "nginx",
			wantTag:  "1.25",
		},
		{
			name:     "image_latest_explicit",
			input:    "alpine:latest",
			wantRepo: "alpine",
			wantTag:  "latest",
		},
		{
			name:     "bare_image_defaults_to_latest",
			input:    "ubuntu",
			wantRepo: "ubuntu",
			wantTag:  "latest",
		},
		{
			name:       "image_with_digest",
			input:      "nginx@sha256:abcdef1234567890",
			wantRepo:   "nginx",
			wantTag:    "latest",
			wantDigest: "sha256:abcdef1234567890",
		},
		{
			name:     "registry_with_port_and_tag",
			input:    "localhost:5000/myimage:v2",
			wantRepo: "localhost:5000/myimage",
			wantTag:  "v2",
		},
		{
			name:     "registry_with_port_no_tag",
			input:    "localhost:5000/myimage",
			wantRepo: "localhost:5000/myimage",
			wantTag:  "latest",
		},
		{
			name:     "full_registry_path",
			input:    "registry.example.com/org/image:v1.0.0",
			wantRepo: "registry.example.com/org/image",
			wantTag:  "v1.0.0",
		},
		{
			name:       "full_registry_with_digest",
			input:      "registry.example.com/org/image@sha256:deadbeef",
			wantRepo:   "registry.example.com/org/image",
			wantTag:    "latest",
			wantDigest: "sha256:deadbeef",
		},
		{
			name:     "docker_hub_library_image",
			input:    "docker.io/library/nginx:stable",
			wantRepo: "docker.io/library/nginx",
			wantTag:  "stable",
		},
		{
			name:     "empty_string",
			input:    "",
			wantRepo: "",
			wantTag:  "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseRepositoryTag(tt.input)

			if got.Repository != tt.wantRepo {
				t.Errorf("Repository = %q, want %q", got.Repository, tt.wantRepo)
			}
			if got.Digest != "" {
				// When digest is present, tag keeps its default "latest" value
				if got.Digest != tt.wantDigest {
					t.Errorf("Digest = %q, want %q", got.Digest, tt.wantDigest)
				}
			} else {
				if got.Tag != tt.wantTag {
					t.Errorf("Tag = %q, want %q", got.Tag, tt.wantTag)
				}
			}
		})
	}
}
