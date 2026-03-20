// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/netresearch/ofelia/test"
)

func TestBuildFromString_EnvSubstitution(t *testing.T) {
	t.Setenv("TEST_IMAGE", "alpine:3.20")
	t.Setenv("TEST_SCHEDULE", "@every 10s")

	configStr := `
[job-run "env-test"]
schedule = ${TEST_SCHEDULE}
image = ${TEST_IMAGE}
command = echo hello
`
	cfg, err := BuildFromString(configStr, test.NewTestLogger())
	if err != nil {
		t.Fatalf("BuildFromString failed: %v", err)
	}

	job, ok := cfg.RunJobs["env-test"]
	if !ok {
		t.Fatal("expected job 'env-test' to exist")
	}
	if job.Image != "alpine:3.20" {
		t.Errorf("expected image %q, got %q", "alpine:3.20", job.Image)
	}
	if job.Schedule != "@every 10s" {
		t.Errorf("expected schedule %q, got %q", "@every 10s", job.Schedule)
	}
}

func TestBuildFromString_EnvSubstitutionDefault(t *testing.T) {
	// Ensure var is unset so default is used
	t.Setenv("OFELIA_TEST_DFLT_IMG_362", "")
	configStr := `
[job-run "default-test"]
schedule = @daily
image = ${OFELIA_TEST_DFLT_IMG_362:-postgres:15}
command = pg_dump
`
	cfg, err := BuildFromString(configStr, test.NewTestLogger())
	if err != nil {
		t.Fatalf("BuildFromString failed: %v", err)
	}

	job, ok := cfg.RunJobs["default-test"]
	if !ok {
		t.Fatal("expected job 'default-test' to exist")
	}
	if job.Image != "postgres:15" {
		t.Errorf("expected image %q (default), got %q", "postgres:15", job.Image)
	}
}

func TestBuildFromString_EnvSubstitutionUndefined(t *testing.T) {
	// Use a var name that will never be set in any environment
	const varName = "OFELIA_TEST_UNDEF_362_XYZZY"
	_ = os.Unsetenv(varName)
	configStr := `
[job-run "undef-test"]
schedule = @daily
image = ${` + varName + `}
command = echo test
`
	cfg, err := BuildFromString(configStr, test.NewTestLogger())
	if err != nil {
		t.Fatalf("BuildFromString failed: %v", err)
	}

	job, ok := cfg.RunJobs["undef-test"]
	if !ok {
		t.Fatal("expected job 'undef-test' to exist")
	}
	if job.Image != "${"+varName+"}" {
		t.Errorf("expected undefined var to stay literal, got %q", job.Image)
	}
}

func TestBuildFromString_EnvSubstitutionSmtpPassword(t *testing.T) {
	// Real-world use case from issue #362 comment
	t.Setenv("SCHEDULER_MAIL_PASS", "s3cret!")

	configStr := `
[global]
smtp-host = mail.example.com
smtp-port = 587
smtp-password = ${SCHEDULER_MAIL_PASS}

[job-exec "test"]
schedule = @daily
container = app
command = echo ok
`
	cfg, err := BuildFromString(configStr, test.NewTestLogger())
	if err != nil {
		t.Fatalf("BuildFromString failed: %v", err)
	}

	if cfg.Global.SMTPPassword != "s3cret!" {
		t.Errorf("expected SMTP password %q, got %q", "s3cret!", cfg.Global.SMTPPassword)
	}
}

func TestBuildFromFile_EnvSubstitution(t *testing.T) {
	t.Setenv("FILE_TEST_IMAGE", "nginx:latest")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.ini")
	err := os.WriteFile(configPath, []byte(`
[job-run "file-env-test"]
schedule = @hourly
image = ${FILE_TEST_IMAGE}
command = echo ok
`), 0o644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := BuildFromFile(configPath, test.NewTestLogger())
	if err != nil {
		t.Fatalf("BuildFromFile failed: %v", err)
	}

	job, ok := cfg.RunJobs["file-env-test"]
	if !ok {
		t.Fatal("expected job 'file-env-test' to exist")
	}
	if job.Image != "nginx:latest" {
		t.Errorf("expected image %q, got %q", "nginx:latest", job.Image)
	}
}
