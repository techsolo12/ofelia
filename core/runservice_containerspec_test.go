// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"testing"
)

// TestRunServiceJob_BuildService_Environment verifies that environment variables
// set on RunServiceJob are passed through to the Docker Swarm ContainerSpec.
func TestRunServiceJob_BuildService_Environment(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.Environment = []string{"FOO=bar", "BAZ=qux"}

	svcID, err := k.job.buildService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svcID == "" {
		t.Fatal("expected non-empty service ID")
	}

	if len(k.services.CreateCalls) == 0 {
		t.Fatal("expected Create to be called")
	}
	spec := k.services.CreateCalls[0].Spec
	env := spec.TaskTemplate.ContainerSpec.Env

	if len(env) != 2 {
		t.Fatalf("expected 2 env vars, got %d: %v", len(env), env)
	}
	if env[0] != "FOO=bar" {
		t.Errorf("expected env[0]=%q, got %q", "FOO=bar", env[0])
	}
	if env[1] != "BAZ=qux" {
		t.Errorf("expected env[1]=%q, got %q", "BAZ=qux", env[1])
	}
}

// TestRunServiceJob_BuildService_Hostname verifies that hostname
// set on RunServiceJob is passed through to the Docker Swarm ContainerSpec.
func TestRunServiceJob_BuildService_Hostname(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.Hostname = "my-service-host"

	svcID, err := k.job.buildService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svcID == "" {
		t.Fatal("expected non-empty service ID")
	}

	if len(k.services.CreateCalls) == 0 {
		t.Fatal("expected Create to be called")
	}
	spec := k.services.CreateCalls[0].Spec

	if spec.TaskTemplate.ContainerSpec.Hostname != "my-service-host" {
		t.Errorf("expected hostname %q, got %q", "my-service-host", spec.TaskTemplate.ContainerSpec.Hostname)
	}
}

// TestRunServiceJob_BuildService_Dir verifies that working directory
// set on RunServiceJob is passed through to the Docker Swarm ContainerSpec.
func TestRunServiceJob_BuildService_Dir(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.Dir = "/app/data"

	svcID, err := k.job.buildService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svcID == "" {
		t.Fatal("expected non-empty service ID")
	}

	if len(k.services.CreateCalls) == 0 {
		t.Fatal("expected Create to be called")
	}
	spec := k.services.CreateCalls[0].Spec

	if spec.TaskTemplate.ContainerSpec.Dir != "/app/data" {
		t.Errorf("expected dir %q, got %q", "/app/data", spec.TaskTemplate.ContainerSpec.Dir)
	}
}

// TestRunServiceJob_BuildService_AllContainerSpecFields verifies that all three
// new fields (Environment, Hostname, Dir) work together.
func TestRunServiceJob_BuildService_AllContainerSpecFields(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.Environment = []string{"DB_HOST=postgres", "DB_PORT=5432"}
	k.job.Hostname = "worker-1"
	k.job.Dir = "/opt/app"

	svcID, err := k.job.buildService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svcID == "" {
		t.Fatal("expected non-empty service ID")
	}

	spec := k.services.CreateCalls[0].Spec
	cs := spec.TaskTemplate.ContainerSpec

	if len(cs.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(cs.Env))
	}
	if cs.Hostname != "worker-1" {
		t.Errorf("expected hostname %q, got %q", "worker-1", cs.Hostname)
	}
	if cs.Dir != "/opt/app" {
		t.Errorf("expected dir %q, got %q", "/opt/app", cs.Dir)
	}
}
