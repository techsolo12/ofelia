//go:build e2e
// +build e2e

// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestE2E_DockerRunJob_SpawnsContainer is the canonical end-to-end docker
// scenario: ofelia runs a real alpine container, the container prints a
// marker, and the container logs contain that marker (verified via the
// docker CLI).
//
// We force `delete = false` so the container persists after exit and we can
// inspect its logs. A unique `container-name` makes cleanup deterministic.
// The test is skipped automatically when docker is not available, so it
// remains runnable on laptops without docker-in-docker.
func TestE2E_DockerRunJob_SpawnsContainer(t *testing.T) {
	t.Parallel()

	if !dockerAvailable(t) {
		t.Skip("docker not available; skipping docker e2e test")
	}

	// Pre-pull the image so the scheduled window doesn't race with registry
	// latency on cold CI runners.
	pullCtx, pullCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer pullCancel()
	if out, err := exec.CommandContext(pullCtx, "docker", "pull", "alpine:3.20").CombinedOutput(); err != nil {
		t.Fatalf("docker pull alpine:3.20: %v\n%s", err, out)
	}

	// Container name used to locate + clean up the container after the job.
	// Timestamped to avoid collisions across parallel test runs.
	containerName := fmt.Sprintf("ofelia-e2e-run-%d", time.Now().UnixNano())
	t.Cleanup(func() { dockerRemove(t, containerName) })

	configBody := fmt.Sprintf(`[global]
  log-level = info

[job-run "e2e-docker"]
  schedule = @every 1s
  image = alpine:3.20
  container-name = %q
  delete = false
  command = sh -c "echo OFELIA_E2E_DOCKER_MARKER"
`, containerName)

	configPath := writeConfig(t, configBody)
	daemon := startDaemon(t, configPath)
	t.Cleanup(func() { daemon.shutdown(t, 30*time.Second) })

	// Wait for the job to finish at least once; the log line appears after
	// image resolve + container run + log collection, so allow 30s.
	if err := daemon.waitForLog(`Job \"e2e-docker\"`, 30*time.Second); err != nil {
		t.Fatalf("no docker execution log observed: %v\nstdout=%s",
			err, daemon.stdout.String())
	}
	if err := daemon.waitForLog("Finished in", 30*time.Second); err != nil {
		t.Fatalf("docker job did not finish: %v\nstdout=%s",
			err, daemon.stdout.String())
	}

	// Now that the container exists (delete=false), fetch its logs and
	// verify the marker made it all the way through the docker runtime.
	logs := dockerLogs(t, containerName)
	if !strings.Contains(logs, "OFELIA_E2E_DOCKER_MARKER") {
		t.Fatalf("marker missing from container logs; got: %q", logs)
	}

	// Belt-and-suspenders: ofelia's stream forwarder should also have picked
	// it up (confirms the stdout-copy plumbing works).
	if !bytes.Contains(daemon.stdout.Bytes(), []byte("OFELIA_E2E_DOCKER_MARKER")) {
		t.Errorf("marker missing from ofelia's captured StdOut stream\nstdout=%s",
			daemon.stdout.String())
	}
}

// TestE2E_DockerRunJob_FailingContainerMarkedFailed asserts that a non-zero
// exit code in the container is surfaced by ofelia as a failed execution.
// Regression guard: silent failures would let broken cron jobs look healthy.
func TestE2E_DockerRunJob_FailingContainerMarkedFailed(t *testing.T) {
	t.Parallel()

	if !dockerAvailable(t) {
		t.Skip("docker not available; skipping docker e2e test")
	}

	pullCtx, pullCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer pullCancel()
	if out, err := exec.CommandContext(pullCtx, "docker", "pull", "alpine:3.20").CombinedOutput(); err != nil {
		t.Fatalf("docker pull alpine:3.20: %v\n%s", err, out)
	}

	containerName := fmt.Sprintf("ofelia-e2e-fail-%d", time.Now().UnixNano())
	t.Cleanup(func() { dockerRemove(t, containerName) })

	configBody := fmt.Sprintf(`[global]
  log-level = info

[job-run "e2e-docker-fail"]
  schedule = @every 1s
  image = alpine:3.20
  container-name = %q
  delete = false
  command = sh -c "exit 42"
`, containerName)

	configPath := writeConfig(t, configBody)
	daemon := startDaemon(t, configPath)
	t.Cleanup(func() { daemon.shutdown(t, 30*time.Second) })

	if err := daemon.waitForLog(`failed: true`, 30*time.Second); err != nil {
		t.Fatalf("expected a 'failed: true' log line within 30s but none appeared: %v\nstdout=%s",
			err, daemon.stdout.String())
	}
}
