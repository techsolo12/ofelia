// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

// Package docker provides an adapter for the official Docker SDK.
package docker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/client"

	"github.com/netresearch/ofelia/core/ports"
)

// defaultNegotiateTimeout bounds the initial Docker API version negotiation.
// Used both as the DefaultConfig value and as the fallback when callers pass
// a non-positive NegotiateTimeout.
const defaultNegotiateTimeout = 30 * time.Second

// ErrUnsupportedDockerHostScheme is returned when a DOCKER_HOST value uses
// a URL scheme that this adapter does not support. The error message lists
// the supported schemes so operators get an actionable failure at startup
// instead of an opaque dial error later.
//
// Supported schemes (case-insensitive): unix, tcp, tcp+tls, http, https, npipe.
// Notably unsupported: ssh, fd. See docs/TROUBLESHOOTING.md.
//
// Tracking: https://github.com/netresearch/ofelia/issues/609
var ErrUnsupportedDockerHostScheme = errors.New("unsupported DOCKER_HOST scheme")

// supportedDockerHostSchemes is the allow-list of URL schemes accepted by
// NewClientWithConfig. Schemes are compared case-insensitively.
//
//   - unix:    Unix domain socket (default on Linux/macOS).
//   - tcp:     Plain TCP (HTTP/1.1, no HTTP/2).
//   - tcp+tls: TLS over TCP; treated like https for transport selection.
//   - http:    Plain HTTP over TCP (HTTP/1.1, no HTTP/2).
//   - https:   HTTPS; HTTP/2 negotiated via ALPN.
//   - npipe:   Windows named pipe (only usable on Windows builds; the actual
//     dialer lives in the SDK and is build-tagged). The scheme is on the
//     allow-list so Windows configurations work transparently.
//
// ssh:// and fd:// are deliberately not supported — they require dialers
// (SSH tunnel, systemd socket activation) that this adapter does not wire up.
var supportedDockerHostSchemes = []string{"unix", "tcp", "tcp+tls", "http", "https", "npipe"}

// Client implements ports.DockerClient using the official Docker SDK.
type Client struct {
	sdk *client.Client

	containers *ContainerServiceAdapter
	exec       *ExecServiceAdapter
	images     *ImageServiceAdapter
	events     *EventServiceAdapter
	services   *SwarmServiceAdapter
	networks   *NetworkServiceAdapter
	system     *SystemServiceAdapter
}

// ClientConfig contains configuration for the Docker client.
type ClientConfig struct {
	// Host is the Docker host address (e.g., "unix:///var/run/docker.sock")
	Host string

	// Version is the API version (empty for auto-negotiation)
	Version string

	// HTTPClient is a custom HTTP client (optional)
	HTTPClient *http.Client

	// HTTPHeaders are custom HTTP headers (optional)
	HTTPHeaders map[string]string

	// Connection pool settings
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration

	// Timeout settings
	DialTimeout           time.Duration
	ResponseHeaderTimeout time.Duration

	// NegotiateTimeout bounds the initial Docker API version negotiation that
	// runs once at client creation. Without a bound, NegotiateAPIVersion uses
	// context.Background() and can block forever when the Docker daemon is
	// reachable but unresponsive (e.g. a socket proxy whose upstream is wedged),
	// hanging Ofelia at startup with no diagnostic output.
	//
	// The Docker SDK swallows ping errors silently, so this timeout does not
	// change correctness on successful paths - it only bounds the failure case.
	//
	// Set to 0 to inherit the default (see DefaultConfig). Use a small value
	// to fail fast in tests; production typically wants 10-30s.
	NegotiateTimeout time.Duration
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       90 * time.Second,
		DialTimeout:           30 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
		NegotiateTimeout:      defaultNegotiateTimeout,
	}
}

// NewClient creates a new Docker client from environment variables.
func NewClient() (*Client, error) {
	return NewClientWithConfig(DefaultConfig())
}

// NewClientWithConfig creates a new Docker client with custom configuration.
//
// The effective Docker host is resolved in this order:
//  1. config.Host (if non-empty)
//  2. DOCKER_HOST environment variable
//  3. client.DefaultDockerHost
//
// The host's URL scheme is validated against the allow-list (see
// supportedDockerHostSchemes) and normalized to lowercase. Unsupported schemes
// (ssh://, fd://, etc.) return ErrUnsupportedDockerHostScheme.
func NewClientWithConfig(config *ClientConfig) (*Client, error) {
	// Resolve the effective host and validate/normalize its scheme up front,
	// so operators get a clear error at startup instead of an opaque dial
	// error later. See https://github.com/netresearch/ofelia/issues/609.
	effectiveHost := config.Host
	if effectiveHost == "" {
		effectiveHost = os.Getenv("DOCKER_HOST")
	}
	normalizedHost, err := validateAndNormalizeHost(effectiveHost)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	// Apply the normalized host back to the config so createHTTPClient sees
	// the same lowercase scheme the SDK will see.
	if normalizedHost != "" {
		config.Host = normalizedHost
	}

	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	if normalizedHost != "" {
		opts = append(opts, client.WithHost(normalizedHost))
	}

	if config.Version != "" {
		opts = append(opts, client.WithVersion(config.Version))
	}

	if config.HTTPHeaders != nil {
		opts = append(opts, client.WithHTTPHeaders(config.HTTPHeaders))
	}

	// Create custom HTTP client with connection pooling
	httpClient := createHTTPClient(config)
	if httpClient != nil {
		opts = append(opts, client.WithHTTPClient(httpClient))
	}

	sdk, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	// Force early API version negotiation to prevent race conditions.
	// Without this, concurrent goroutines (e.g., Events and ContainerList) making their
	// first API calls simultaneously will race on the lazy version negotiation.
	//
	// Bound the call so a reachable-but-wedged daemon (e.g. socket proxy with a
	// hung upstream) cannot hang Ofelia at startup. NegotiateAPIVersion swallows
	// ping errors silently, so a timeout only bounds the failure case; the
	// successful path is unaffected. See https://github.com/netresearch/ofelia/issues/608.
	negotiateTimeout := config.NegotiateTimeout
	if negotiateTimeout <= 0 {
		negotiateTimeout = defaultNegotiateTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), negotiateTimeout)
	defer cancel()
	sdk.NegotiateAPIVersion(ctx)
	// NegotiateAPIVersion swallows ping errors silently. Surface a deadline
	// hit so operators can correlate startup slowness with daemon health
	// rather than chasing phantom bugs - context cancellation is the only
	// observable signal the SDK leaves us.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		slog.Default().Warn(
			"Docker API version negotiation timed out; continuing with default API version",
			"timeout", negotiateTimeout,
			"hint", "check Docker daemon health and DOCKER_HOST reachability",
		)
	}

	return newClientFromSDK(sdk), nil
}

// newClientFromSDK wraps an existing SDK client.
func newClientFromSDK(sdk *client.Client) *Client {
	c := &Client{sdk: sdk}
	c.containers = &ContainerServiceAdapter{client: sdk}
	c.exec = &ExecServiceAdapter{client: sdk}
	c.images = &ImageServiceAdapter{client: sdk}
	c.events = &EventServiceAdapter{client: sdk}
	c.services = &SwarmServiceAdapter{client: sdk}
	c.networks = &NetworkServiceAdapter{client: sdk}
	c.system = &SystemServiceAdapter{client: sdk}
	return c
}

// createHTTPClient creates an HTTP client with connection pooling.
//
// The caller is responsible for ensuring config.Host has already been
// validated and normalized via validateAndNormalizeHost (NewClientWithConfig
// does this). The switch below relies on the scheme being lowercase.
func createHTTPClient(config *ClientConfig) *http.Client {
	// Determine if we should use HTTP/2
	// Docker daemon only supports HTTP/2 over TLS (ALPN negotiation)
	// For Unix sockets and plain TCP, we use HTTP/1.1
	host := config.Host
	if host == "" {
		host = client.DefaultDockerHost
	}
	scheme := schemeOf(host)

	transport := &http.Transport{
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
	}

	// Configure dialer based on scheme. Schemes are lowercase at this point
	// (validateAndNormalizeHost guarantees this for caller-supplied hosts;
	// client.DefaultDockerHost is already lowercase).
	switch scheme {
	case "unix":
		socketPath := strings.TrimPrefix(host, "unix://")
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: config.DialTimeout}
			return dialer.DialContext(ctx, "unix", socketPath)
		}
		// HTTP/2 not supported on Unix sockets
		transport.ForceAttemptHTTP2 = false
	case "https", "tcp+tls":
		// TLS-backed connections can use HTTP/2 via ALPN.
		transport.ForceAttemptHTTP2 = true
	default:
		// tcp, http, npipe, or empty: plain transport, HTTP/1.1.
		// (npipe on Windows relies on the SDK's build-tagged dialer; on other
		// platforms the connection will fail at dial time — see the package
		// docs and docs/TROUBLESHOOTING.md.)
		transport.ForceAttemptHTTP2 = false
	}

	return &http.Client{
		Transport: transport,
		Timeout:   0, // No overall timeout; individual operations have timeouts
	}
}

// schemeOf returns the lowercase scheme portion of a Docker host URL, or ""
// if the input has no scheme separator. It does NOT validate the scheme.
func schemeOf(host string) string {
	idx := strings.Index(host, "://")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(host[:idx])
}

// validateAndNormalizeHost validates that host uses a supported scheme and
// returns the host with the scheme lowercased. The path/authority portion
// is preserved as-is (Unix socket paths are case-sensitive on most
// filesystems, so lowercasing the whole string would break valid
// configurations like "unix:///Var/Run/docker.sock").
//
// An empty host is returned unchanged — callers fall back to DOCKER_HOST or
// client.DefaultDockerHost downstream.
//
// Unsupported schemes (ssh, fd, tcp+tls used incorrectly, etc.) return
// ErrUnsupportedDockerHostScheme wrapped with the offending value and a list
// of supported schemes.
func validateAndNormalizeHost(host string) (string, error) {
	if host == "" {
		return "", nil
	}

	idx := strings.Index(host, "://")
	if idx < 0 {
		return "", fmt.Errorf("%w: %q has no scheme; supported schemes: %s",
			ErrUnsupportedDockerHostScheme, host, formatSupportedSchemes())
	}

	scheme := strings.ToLower(host[:idx])
	for _, allowed := range supportedDockerHostSchemes {
		if scheme == allowed {
			// Reassemble with the lowercase scheme; preserve the rest verbatim.
			return scheme + host[idx:], nil
		}
	}

	return "", fmt.Errorf("%w: %q; supported schemes: %s",
		ErrUnsupportedDockerHostScheme, scheme+"://", formatSupportedSchemes())
}

// formatSupportedSchemes returns a comma-separated, suffixed list of the
// supported Docker host schemes for inclusion in error messages.
func formatSupportedSchemes() string {
	parts := make([]string, len(supportedDockerHostSchemes))
	for i, s := range supportedDockerHostSchemes {
		parts[i] = s + "://"
	}
	return strings.Join(parts, ", ")
}

// Containers returns the container service.
func (c *Client) Containers() ports.ContainerService {
	return c.containers
}

// Exec returns the exec service.
func (c *Client) Exec() ports.ExecService {
	return c.exec
}

// Images returns the image service.
func (c *Client) Images() ports.ImageService {
	return c.images
}

// Events returns the event service.
func (c *Client) Events() ports.EventService {
	return c.events
}

// Services returns the Swarm service.
func (c *Client) Services() ports.SwarmService {
	return c.services
}

// Networks returns the network service.
func (c *Client) Networks() ports.NetworkService {
	return c.networks
}

// System returns the system service.
func (c *Client) System() ports.SystemService {
	return c.system
}

// Close closes the client.
func (c *Client) Close() error {
	if err := c.sdk.Close(); err != nil {
		return fmt.Errorf("closing docker client: %w", err)
	}
	return nil
}

// SDK returns the underlying Docker SDK client.
// This should only be used for operations not covered by the ports interface.
func (c *Client) SDK() *client.Client {
	return c.sdk
}
