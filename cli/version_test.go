// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"strings"
	"testing"
)

func TestVersionString_WithLdflags(t *testing.T) {
	t.Parallel()

	// Save and restore
	origV, origB := Version, Build
	t.Cleanup(func() { Version, Build = origV, origB })

	Version = "v0.21.4"
	Build = "abc1234"

	result := VersionString()
	if result != "ofelia v0.21.4 (abc1234)" {
		t.Errorf("expected %q, got %q", "ofelia v0.21.4 (abc1234)", result)
	}
}

func TestVersionString_VersionOnly(t *testing.T) {
	t.Parallel()

	origV, origB := Version, Build
	t.Cleanup(func() { Version, Build = origV, origB })

	Version = "v0.21.4"
	Build = ""

	result := VersionString()
	if result != "ofelia v0.21.4" {
		t.Errorf("expected %q, got %q", "ofelia v0.21.4", result)
	}
}

func TestVersionString_DevBuild(t *testing.T) {
	t.Parallel()

	origV, origB := Version, Build
	t.Cleanup(func() { Version, Build = origV, origB })

	Version = ""
	Build = ""

	result := VersionString()
	if !strings.HasPrefix(result, "ofelia dev") {
		t.Errorf("expected dev build string starting with %q, got %q", "ofelia dev", result)
	}
}
