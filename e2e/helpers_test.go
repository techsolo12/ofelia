//go:build e2e
// +build e2e

// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

// Package e2e exercises the compiled ofelia binary as a real subprocess:
// build binary → spawn with INI config → observe stdout/stderr, file system
// side-effects and container logs. These tests complement the in-process
// integration tests by verifying the full pipeline (parse config → schedule
// → fire → execute → shutdown).
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

// buildBinaryOnce caches the compiled ofelia binary path across tests in the
// same `go test` process to avoid re-invoking `go build` per-test (which is
// slow and multiplies CI cost). cachedBuildErr is a captured build error —
// not a sentinel — hence the `nolint:errname` suppression.
var (
	buildBinaryOnce sync.Once
	cachedBinPath   string
	cachedBuildErr  error //nolint:errname // captured build error, not a sentinel
)

// buildBinary compiles the ofelia binary with race detection enabled into a
// temp directory and returns its absolute path. The binary is shared by all
// e2e tests in a single `go test` invocation.
//
// We intentionally compile with `-race` so that e2e tests double as real-world
// race-condition detectors — if the scheduler or shutdown logic has a data
// race, the binary itself will report it on SIGTERM.
func buildBinary(t *testing.T) string {
	t.Helper()

	buildBinaryOnce.Do(func() {
		// Resolve repo root from this test file's location (e2e/ → ..).
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			cachedBuildErr = errors.New("cannot resolve test source path")
			return
		}
		repoRoot := filepath.Dir(filepath.Dir(thisFile))

		tmpDir, err := os.MkdirTemp("", "ofelia-e2e-bin-")
		if err != nil {
			cachedBuildErr = fmt.Errorf("mkdir temp: %w", err)
			return
		}
		binPath := filepath.Join(tmpDir, "ofelia")

		// Use -race so the real binary surfaces data races during e2e runs.
		// Timeout: go build usually completes in <30s on CI.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "go", "build", "-race", "-o", binPath, ".")
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			cachedBuildErr = fmt.Errorf("build ofelia: %w\n%s", err, out)
			return
		}
		cachedBinPath = binPath
	})

	if cachedBuildErr != nil {
		t.Fatalf("build binary: %v", cachedBuildErr)
	}
	return cachedBinPath
}

// daemonProcess wraps a running ofelia daemon subprocess and provides helpers
// to observe its behavior (log scraping, stdout/stderr capture) and terminate
// it cleanly.
type daemonProcess struct {
	cmd      *exec.Cmd
	stdout   *syncBuffer
	stderr   *syncBuffer
	done     chan struct{}
	waitErr  error
	waitOnce sync.Once
}

// startDaemon launches `ofelia daemon --config=<configPath>` as a child
// process and returns a handle to it. The caller MUST defer `p.shutdown(t)`.
func startDaemon(t *testing.T, configPath string, extraArgs ...string) *daemonProcess {
	t.Helper()

	bin := buildBinary(t)

	args := append([]string{"daemon", "--config=" + configPath}, extraArgs...)
	cmd := exec.Command(bin, args...)

	// Put the child in its own process group so SIGTERM to the child does not
	// leak to the test runner, and so we can reliably kill any stragglers.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout := &syncBuffer{}
	stderr := &syncBuffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start ofelia daemon: %v", err)
	}

	dp := &daemonProcess{
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
		done:   make(chan struct{}),
	}

	go func() {
		dp.waitErr = cmd.Wait()
		close(dp.done)
	}()

	// Wait for the "Ofelia is now running" banner so we know scheduler is
	// live before returning. Time out quickly if boot fails.
	if err := dp.waitForLog("Ofelia is now running", 10*time.Second); err != nil {
		// Make sure we don't leak a child before failing.
		_ = dp.signal(syscall.SIGKILL)
		t.Fatalf("daemon did not reach 'running' state within 10s: %v\nstdout=%s\nstderr=%s",
			err, stdout.String(), stderr.String())
	}

	return dp
}

// waitForLog polls the combined stdout for a substring until it appears or
// the timeout elapses.
func (p *daemonProcess) waitForLog(needle string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if bytes.Contains(p.stdout.Bytes(), []byte(needle)) ||
			bytes.Contains(p.stderr.Bytes(), []byte(needle)) {
			return nil
		}
		if p.exited() {
			return fmt.Errorf("daemon exited before log %q appeared", needle)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for log %q", needle)
}

// countLogOccurrences counts how many times the given substring appears in
// the captured stdout.
func (p *daemonProcess) countLogOccurrences(needle string) int {
	data := p.stdout.Bytes()
	n := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), []byte(needle)) {
			n++
		}
	}
	return n
}

// exited reports whether the daemon process has already terminated.
func (p *daemonProcess) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// signal sends a signal to the daemon process.
func (p *daemonProcess) signal(sig os.Signal) error {
	if p.cmd.Process == nil {
		return errors.New("daemon process is nil")
	}
	return p.cmd.Process.Signal(sig)
}

// shutdown sends SIGTERM and waits for the process to exit. Safe to call
// multiple times. If the daemon doesn't exit within `timeout`, it is killed.
func (p *daemonProcess) shutdown(t *testing.T, timeout time.Duration) {
	t.Helper()
	p.waitOnce.Do(func() {
		if !p.exited() {
			_ = p.signal(syscall.SIGTERM)
		}
		select {
		case <-p.done:
		case <-time.After(timeout):
			t.Logf("daemon did not exit within %s after SIGTERM; sending SIGKILL", timeout)
			_ = p.signal(syscall.SIGKILL)
			<-p.done
		}
	})
}

// waitExit blocks until the daemon exits (or the timeout fires) and returns
// the exit error. Unlike shutdown(), this does not send a signal itself.
func (p *daemonProcess) waitExit(timeout time.Duration) (error, bool) {
	select {
	case <-p.done:
		return p.waitErr, true
	case <-time.After(timeout):
		return nil, false
	}
}

// syncBuffer is a concurrent-safe bytes buffer used to capture child process
// stdout/stderr without racing against the log-scraping goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]byte, s.buf.Len())
	copy(out, s.buf.Bytes())
	return out
}

func (s *syncBuffer) String() string {
	return string(s.Bytes())
}

// writeConfig writes the given INI body to a file inside the test's temp
// directory and returns the path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ofelia.ini")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// runCommand runs the ofelia binary with the given args, returning stdout,
// stderr and the exit error. Used by validation-error tests.
func runCommand(t *testing.T, args ...string) (stdout, stderr string, exitErr error) {
	t.Helper()
	bin := buildBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	exitErr = cmd.Run()
	return outBuf.String(), errBuf.String(), exitErr
}

// dockerAvailable returns true if the docker CLI is usable and the daemon is
// reachable. Tests that require docker skip cleanly when it is absent.
func dockerAvailable(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// dockerLogs returns the combined stdout+stderr log output of a named
// container.
func dockerLogs(t *testing.T, name string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "logs", name)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("docker logs %s: %v\n%s", name, err, out.String())
	}
	return out.String()
}

// dockerRemove best-effort removes a container by name.
func dockerRemove(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}
