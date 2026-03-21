// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/netresearch/ofelia/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseEnvFile tests ---

func TestParseEnvFile_BasicKeyValue(t *testing.T) {
	f := writeEnvFile(t, "FOO=bar\nBAZ=qux\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, got)
}

func TestParseEnvFile_Comments(t *testing.T) {
	f := writeEnvFile(t, "# this is a comment\nFOO=bar\n# another\nBAZ=qux\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, got)
}

func TestParseEnvFile_BlankLines(t *testing.T) {
	f := writeEnvFile(t, "\nFOO=bar\n\n\nBAZ=qux\n\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, got)
}

func TestParseEnvFile_QuotedValues(t *testing.T) {
	f := writeEnvFile(t, `FOO="hello world"`+"\n"+`BAZ='single quoted'`+"\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=hello world", "BAZ=single quoted"}, got)
}

func TestParseEnvFile_EmptyValue(t *testing.T) {
	f := writeEnvFile(t, "FOO=\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO="}, got)
}

func TestParseEnvFile_ValueContainsEquals(t *testing.T) {
	f := writeEnvFile(t, "DATABASE_URL=postgres://user:pass@host/db?opt=val\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"DATABASE_URL=postgres://user:pass@host/db?opt=val"}, got)
}

func TestParseEnvFile_ValueContainsHash(t *testing.T) {
	f := writeEnvFile(t, "PASSWORD=p@ss#w0rd\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"PASSWORD=p@ss#w0rd"}, got)
}

func TestParseEnvFile_ExportPrefix(t *testing.T) {
	f := writeEnvFile(t, "export FOO=bar\nexport BAZ=qux\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, got)
}

func TestParseEnvFile_FileNotFound(t *testing.T) {
	_, err := ParseEnvFile("/nonexistent/path/file.env")
	require.Error(t, err)
}

func TestParseEnvFile_EmptyKey(t *testing.T) {
	// Lines like "=value" should be skipped (empty key)
	f := writeEnvFile(t, "=value\nFOO=bar\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar"}, got)
}

func TestParseEnvFile_LineWithoutEquals(t *testing.T) {
	// Lines without = are skipped (just a key name, no value assignment)
	f := writeEnvFile(t, "FOO=bar\nINVALID_LINE\nBAZ=qux\n")
	got, err := ParseEnvFile(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, got)
}

// --- MergeEnvironments tests ---

func TestMergeEnvironments_EmptyAll(t *testing.T) {
	got := MergeEnvironments(nil, nil, nil)
	assert.Empty(t, got)
}

func TestMergeEnvironments_OnlyExplicit(t *testing.T) {
	explicit := []string{"FOO=bar", "BAZ=qux"}
	got := MergeEnvironments(nil, nil, explicit)
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, got)
}

func TestMergeEnvironments_EnvFileOverriddenByExplicit(t *testing.T) {
	envFile := []string{"FOO=from-file", "EXTRA=from-file"}
	explicit := []string{"FOO=explicit"}
	got := MergeEnvironments(envFile, nil, explicit)
	assert.Contains(t, got, "FOO=explicit")
	assert.Contains(t, got, "EXTRA=from-file")
	// FOO should NOT appear twice
	count := 0
	for _, v := range got {
		if len(v) >= 4 && v[:4] == "FOO=" {
			count++
		}
	}
	assert.Equal(t, 1, count, "FOO should appear exactly once")
}

func TestMergeEnvironments_EnvFromOverriddenByExplicit(t *testing.T) {
	envFrom := []string{"DB_HOST=from-container", "DB_PORT=5432"}
	explicit := []string{"DB_HOST=explicit"}
	got := MergeEnvironments(nil, envFrom, explicit)
	assert.Contains(t, got, "DB_HOST=explicit")
	assert.Contains(t, got, "DB_PORT=5432")
}

func TestMergeEnvironments_EnvFromOverridesEnvFile(t *testing.T) {
	envFile := []string{"FOO=from-file"}
	envFrom := []string{"FOO=from-container"}
	got := MergeEnvironments(envFile, envFrom, nil)
	assert.Equal(t, []string{"FOO=from-container"}, got)
}

func TestMergeEnvironments_FullMergeOrder(t *testing.T) {
	envFile := []string{"A=file", "B=file", "C=file"}
	envFrom := []string{"B=container", "D=container"}
	explicit := []string{"C=explicit", "E=explicit"}
	got := MergeEnvironments(envFile, envFrom, explicit)
	assert.Contains(t, got, "A=file")
	assert.Contains(t, got, "B=container")
	assert.Contains(t, got, "C=explicit")
	assert.Contains(t, got, "D=container")
	assert.Contains(t, got, "E=explicit")
	assert.Len(t, got, 5)
}

func TestMergeEnvironments_DuplicateKeysWithinSource(t *testing.T) {
	envFile := []string{"FOO=first", "FOO=second"}
	got := MergeEnvironments(envFile, nil, nil)
	assert.Equal(t, []string{"FOO=second"}, got)
}

// --- ResolveEnvFrom tests ---

func TestResolveEnvFrom_HappyPath(t *testing.T) {
	provider := &mockEnvProvider{
		env: []string{"DB_HOST=postgres", "DB_PORT=5432"},
	}
	got, err := ResolveEnvFrom(context.Background(), provider, "my-container")
	require.NoError(t, err)
	assert.Equal(t, []string{"DB_HOST=postgres", "DB_PORT=5432"}, got)
}

func TestResolveEnvFrom_ContainerNotFound(t *testing.T) {
	provider := &mockEnvProvider{
		err: assert.AnError,
	}
	_, err := ResolveEnvFrom(context.Background(), provider, "nonexistent")
	require.Error(t, err)
}

func TestResolveEnvFrom_NilConfig(t *testing.T) {
	provider := &mockEnvProvider{
		nilConfig: true,
	}
	got, err := ResolveEnvFrom(context.Background(), provider, "my-container")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestResolveEnvFrom_NilProvider(t *testing.T) {
	_, err := ResolveEnvFrom(context.Background(), nil, "my-container")
	require.Error(t, err)
}

// --- ResolveJobEnvironment tests ---

func TestResolveJobEnvironment_NoSources(t *testing.T) {
	got, err := ResolveJobEnvironment(context.Background(), nil, nil, []string{"FOO=bar"}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar"}, got)
}

func TestResolveJobEnvironment_EnvFileAndExplicit(t *testing.T) {
	f := writeEnvFile(t, "FROM_FILE=yes\nFOO=file\n")
	got, err := ResolveJobEnvironment(context.Background(), []string{f}, nil, []string{"FOO=explicit"}, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, got, "FROM_FILE=yes")
	assert.Contains(t, got, "FOO=explicit")
}

func TestResolveJobEnvironment_EnvFromWithProvider(t *testing.T) {
	provider := &mockEnvProvider{
		env: []string{"FROM_CONTAINER=yes"},
	}
	got, err := ResolveJobEnvironment(context.Background(), nil, []string{"app"}, []string{"FOO=explicit"}, provider, nil)
	require.NoError(t, err)
	assert.Contains(t, got, "FROM_CONTAINER=yes")
	assert.Contains(t, got, "FOO=explicit")
}

func TestResolveJobEnvironment_EnvFromWithoutProvider(t *testing.T) {
	var warnings []string
	warnFn := func(msg string) { warnings = append(warnings, msg) }
	got, err := ResolveJobEnvironment(context.Background(), nil, []string{"app"}, []string{"FOO=bar"}, nil, warnFn)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar"}, got)
	assert.Len(t, warnings, 1, "should have warned about env-from without provider")
}

func TestResolveJobEnvironment_EnvFileNotFound(t *testing.T) {
	_, err := ResolveJobEnvironment(context.Background(), []string{"/nonexistent.env"}, nil, nil, nil, nil)
	require.Error(t, err)
}

// --- test helpers ---

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "test.env")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))
	return f
}

// mockEnvProvider implements just enough of DockerProvider for ResolveEnvFrom tests.
type mockEnvProvider struct {
	env       []string
	err       error
	nilConfig bool
}

func (m *mockEnvProvider) InspectContainer(_ context.Context, _ string) (*domain.Container, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.nilConfig {
		return &domain.Container{ID: "abc123"}, nil
	}
	return &domain.Container{
		ID: "abc123",
		Config: &domain.ContainerConfig{
			Env: m.env,
		},
	}, nil
}
