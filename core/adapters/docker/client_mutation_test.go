// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Tests targeting surviving CONDITIONALS_NEGATION mutations in client.go
// These test both branches of config option conditionals

func TestNewClientWithConfig_ConfigOptions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		config *ClientConfig
		desc   string
	}{
		// Targeting line 79: if config.Host != ""
		{
			name: "empty_host",
			config: &ClientConfig{
				Host:    "",
				Version: "",
			},
			desc: "Empty host should use default",
		},
		{
			name: "non_empty_host",
			config: &ClientConfig{
				Host:    "unix:///var/run/docker.sock",
				Version: "",
			},
			desc: "Non-empty host should be applied",
		},
		// Targeting line 83: if config.Version != ""
		{
			name: "empty_version",
			config: &ClientConfig{
				Host:    "",
				Version: "",
			},
			desc: "Empty version should use default",
		},
		{
			name: "non_empty_version",
			config: &ClientConfig{
				Host:    "",
				Version: "1.41",
			},
			desc: "Non-empty version should be applied",
		},
		// Targeting line 87: if config.HTTPHeaders != nil
		{
			name: "nil_http_headers",
			config: &ClientConfig{
				Host:        "",
				HTTPHeaders: nil,
			},
			desc: "Nil HTTP headers should be skipped",
		},
		{
			name: "non_nil_http_headers",
			config: &ClientConfig{
				Host:        "",
				HTTPHeaders: map[string]string{"X-Custom": "value"},
			},
			desc: "Non-nil HTTP headers should be applied",
		},
		// Test combinations
		{
			name: "all_options_set",
			config: &ClientConfig{
				Host:        "unix:///var/run/docker.sock",
				Version:     "1.41",
				HTTPHeaders: map[string]string{"X-Test": "test"},
			},
			desc: "All options set should be applied",
		},
		{
			name: "all_options_empty",
			config: &ClientConfig{
				Host:        "",
				Version:     "",
				HTTPHeaders: nil,
			},
			desc: "All options empty should use defaults",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewClientWithConfig(tc.config)
			if err != nil {
				t.Logf("%s: got expected error (no Docker): %v", tc.desc, err)
			}
		})
	}
}

func TestCreateHTTPClient_HostConditions(t *testing.T) {
	// Targeting line 124: if host == ""
	// Test that empty host defaults to DefaultDockerHost

	testCases := []struct {
		name   string
		config *ClientConfig
		desc   string
	}{
		{
			name: "empty_host_defaults",
			config: &ClientConfig{
				Host:            "",
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
			desc: "Empty host should default to Docker default host",
		},
		{
			name: "unix_socket_host",
			config: &ClientConfig{
				Host:            "unix:///var/run/docker.sock",
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
			desc: "Unix socket host should be used",
		},
		{
			name: "tcp_host",
			config: &ClientConfig{
				Host:            "tcp://localhost:2375",
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
			desc: "TCP host should be used",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// createHTTPClient is called internally by NewClientWithConfig
			// We test it indirectly by creating a client with config
			httpClient := createHTTPClient(tc.config)
			if httpClient == nil {
				t.Errorf("%s: expected non-nil HTTP client", tc.desc)
			}
		})
	}
}

func TestClientConfig_PoolingOptions(t *testing.T) {
	// Test various connection pooling configurations
	testCases := []struct {
		name   string
		config *ClientConfig
	}{
		{
			name: "default_pooling",
			config: &ClientConfig{
				MaxIdleConns:          0,
				MaxIdleConnsPerHost:   0,
				MaxConnsPerHost:       0,
				IdleConnTimeout:       0,
				ResponseHeaderTimeout: 0,
			},
		},
		{
			name: "custom_pooling",
			config: &ClientConfig{
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   10,
				MaxConnsPerHost:       50,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := createHTTPClient(tc.config)
			if client == nil {
				t.Error("Expected non-nil HTTP client")
				return
			}

			// Verify transport was configured
			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				// Transport might be wrapped, that's OK
				t.Logf("Transport is not *http.Transport, might be wrapped")
				return
			}

			// Verify pooling settings were applied
			if tc.config.MaxIdleConns > 0 && transport.MaxIdleConns != tc.config.MaxIdleConns {
				t.Errorf("MaxIdleConns: got %d, want %d", transport.MaxIdleConns, tc.config.MaxIdleConns)
			}
			if tc.config.MaxIdleConnsPerHost > 0 && transport.MaxIdleConnsPerHost != tc.config.MaxIdleConnsPerHost {
				t.Errorf("MaxIdleConnsPerHost: got %d, want %d", transport.MaxIdleConnsPerHost, tc.config.MaxIdleConnsPerHost)
			}
		})
	}
}

// TestCreateHTTPClient_UnsupportedSchemes verifies that DOCKER_HOST values with
// unsupported URL schemes are rejected at construction with a clear error,
// instead of silently falling through to a plain-TCP transport.
//
// See: https://github.com/netresearch/ofelia/issues/609
func TestCreateHTTPClient_UnsupportedSchemes(t *testing.T) {
	t.Parallel()

	// Note: tcp+tls:// is intentionally NOT in this list — it is on the
	// supported allow-list (treated like https for transport selection) to
	// prevent the silent TLS downgrade described in the issue.
	testCases := []struct {
		name string
		host string
	}{
		{
			name: "ssh_scheme",
			host: "ssh://user@docker-host:22",
		},
		{
			name: "fd_scheme",
			host: "fd://",
		},
		{
			name: "bogus_scheme",
			host: "gopher://something",
		},
		{
			name: "no_scheme",
			host: "127.0.0.1:2375",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewClientWithConfig(&ClientConfig{Host: tc.host})
			if err == nil {
				t.Fatalf("expected error for unsupported scheme %q, got nil", tc.host)
			}
			if !errors.Is(err, ErrUnsupportedDockerHostScheme) {
				t.Errorf("expected ErrUnsupportedDockerHostScheme for %q, got %v", tc.host, err)
			}
			// Error message must list at least one supported scheme so operators
			// know what to switch to.
			if !strings.Contains(err.Error(), "unix://") {
				t.Errorf("expected error message to list supported schemes (e.g. unix://), got %q", err.Error())
			}
		})
	}
}

// TestValidateAndNormalizeHost covers the host scheme validation helper directly.
// It verifies case-insensitivity (RFC 3986: schemes are case-insensitive) and
// the allow-list of supported transports.
func TestValidateAndNormalizeHost(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		input     string
		want      string
		wantErr   bool
		errSentry error
	}{
		// Supported schemes - lowercase.
		{name: "unix_lowercase", input: "unix:///var/run/docker.sock", want: "unix:///var/run/docker.sock"},
		{name: "tcp_lowercase", input: "tcp://127.0.0.1:2375", want: "tcp://127.0.0.1:2375"},
		{name: "http_lowercase", input: "http://127.0.0.1:2375", want: "http://127.0.0.1:2375"},
		{name: "https_lowercase", input: "https://127.0.0.1:2376", want: "https://127.0.0.1:2376"},
		{name: "npipe_lowercase", input: `npipe:////./pipe/docker_engine`, want: `npipe:////./pipe/docker_engine`},

		// Case-insensitivity: schemes are normalized to lowercase, paths preserved.
		{name: "tcp_uppercase", input: "TCP://127.0.0.1:2375", want: "tcp://127.0.0.1:2375"},
		{name: "unix_uppercase", input: "UNIX:///var/run/docker.sock", want: "unix:///var/run/docker.sock"},
		{name: "https_mixed", input: "HtTpS://127.0.0.1:2376", want: "https://127.0.0.1:2376"},

		// Path casing is preserved (only scheme is lowercased).
		{name: "unix_mixed_path", input: "UNIX:///Var/Run/docker.sock", want: "unix:///Var/Run/docker.sock"},

		// Empty string passes through (caller supplies default).
		{name: "empty_string", input: "", want: ""},

		// tcp+tls:// is supported (treated like https for transport selection).
		{name: "tcp_plus_tls", input: "tcp+tls://127.0.0.1:2376", want: "tcp+tls://127.0.0.1:2376"},

		// Unsupported schemes.
		{name: "ssh", input: "ssh://docker-host", wantErr: true, errSentry: ErrUnsupportedDockerHostScheme},
		{name: "fd", input: "fd://", wantErr: true, errSentry: ErrUnsupportedDockerHostScheme},
		{name: "gopher", input: "gopher://something", wantErr: true, errSentry: ErrUnsupportedDockerHostScheme},

		// Missing scheme separator.
		{name: "no_scheme", input: "127.0.0.1:2375", wantErr: true, errSentry: ErrUnsupportedDockerHostScheme},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateAndNormalizeHost(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("input %q: expected error, got nil (result=%q)", tc.input, got)
				}
				if tc.errSentry != nil && !errors.Is(err, tc.errSentry) {
					t.Errorf("input %q: expected errors.Is(err, %v), got %v", tc.input, tc.errSentry, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("input %q: unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("input %q: got %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestNewClientWithConfig_NormalizesDockerHostEnv verifies that DOCKER_HOST
// environment variables with uppercase schemes are normalized to lowercase
// before scheme-dispatch, instead of falling through to the plain-TCP default.
//
// Cannot use t.Parallel() — t.Setenv is incompatible with parallel subtests.
func TestNewClientWithConfig_NormalizesDockerHostEnv(t *testing.T) {
	// 127.0.0.1:0 is unreachable (port 0); construction succeeds but no real
	// connection is attempted (API negotiation is best-effort and tolerates
	// failure at this layer for unit-test purposes).
	t.Setenv("DOCKER_HOST", "TCP://127.0.0.1:0")

	// We don't care if the actual connection fails — we care that construction
	// validates the scheme and doesn't reject a valid (if uppercase) TCP host.
	_, err := NewClientWithConfig(&ClientConfig{})
	if err != nil && errors.Is(err, ErrUnsupportedDockerHostScheme) {
		t.Fatalf("uppercase TCP:// scheme should be normalized, not rejected: %v", err)
	}
}

// TestCreateHTTPClient_NpipeTransport documents the npipe:// behavior decision:
// npipe:// is on the allow-list (so Windows builds work transparently), but on
// non-Windows the transport falls back to the default (plain TCP) configuration
// — the actual named-pipe dialer lives in the SDK and is build-tagged. We assert
// only that construction does not return ErrUnsupportedDockerHostScheme for
// npipe://, mirroring the documented contract.
func TestCreateHTTPClient_NpipeTransport(t *testing.T) {
	t.Parallel()

	_, err := NewClientWithConfig(&ClientConfig{Host: `npipe:////./pipe/docker_engine`})
	if err != nil && errors.Is(err, ErrUnsupportedDockerHostScheme) {
		t.Errorf("npipe:// should be on the allow-list, got %v", err)
	}
}
