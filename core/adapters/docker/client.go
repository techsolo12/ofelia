// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

// Package docker provides an adapter for the official Docker SDK.
package docker

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"

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
// Supported schemes (case-insensitive): unix, tcp, http, https, npipe.
// Notably unsupported: ssh, fd, tcp+tls. See docs/TROUBLESHOOTING.md.
//
// Tracking: https://github.com/netresearch/ofelia/issues/609
var ErrUnsupportedDockerHostScheme = errors.New("unsupported DOCKER_HOST scheme")

// ErrMissingDockerHostScheme is returned when DOCKER_HOST is non-empty but
// has no "://" separator (e.g. "127.0.0.1:2375"). Distinct from
// ErrUnsupportedDockerHostScheme so operators can tell "I forgot the scheme"
// from "I used a scheme that isn't supported."
var ErrMissingDockerHostScheme = errors.New("missing DOCKER_HOST scheme")

// supportedDockerHostSchemes is the allow-list of URL schemes accepted by
// NewClientWithConfig. Schemes are compared case-insensitively.
//
//   - unix:    Unix domain socket (default on Linux/macOS).
//   - tcp:     Plain TCP (HTTP/1.1, no HTTP/2).
//   - http:    Plain HTTP over TCP (HTTP/1.1, no HTTP/2).
//   - https:   HTTPS; HTTP/2 negotiated via ALPN.
//   - npipe:   Windows named pipe (only usable on Windows builds; the actual
//     dialer lives in the SDK and is build-tagged). The scheme is on the
//     allow-list so Windows configurations work transparently.
//
// Deliberately not supported:
//
//   - ssh:    Requires an SSH tunnel dialer this adapter does not wire up.
//   - fd:     Requires systemd socket activation we do not handle.
//   - tcp+tls: Withheld pending PR #613 (issue #607). Without TLS material
//     wired into the custom transport, accepting tcp+tls would silently
//     downgrade to plain TCP — exactly the silent-downgrade class this PR
//     is supposed to prevent. Re-enable in a follow-up once TLS plumbing
//     lands.
var supportedDockerHostSchemes = []string{"unix", "tcp", "http", "https", "npipe"}

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

	// TLSCertPath is the directory containing TLS material (ca.pem, cert.pem,
	// key.pem) for HTTPS Docker hosts. When empty, the DOCKER_CERT_PATH
	// environment variable is consulted instead. If both are empty, no TLS
	// configuration is applied (mirroring docker/docker's
	// WithTLSClientConfigFromEnv).
	TLSCertPath string

	// TLSVerify controls whether the server certificate is verified for HTTPS
	// Docker hosts. When nil, the DOCKER_TLS_VERIFY environment variable is
	// consulted: any non-empty value enables verification (mirroring
	// docker/docker's WithTLSClientConfigFromEnv).
	TLSVerify *bool
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
	// Tolerate a nil config the same way DefaultConfig() would: callers
	// constructing through the public surface should not be able to panic
	// the daemon at startup with a nil-pointer deref.
	if config == nil {
		config = DefaultConfig()
	}

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

	// Build a local copy of config with the normalized host for createHTTPClient
	// so the dialer-selection switch sees the lowercase scheme. We deliberately
	// avoid mutating the caller's struct - reusing a *ClientConfig across
	// constructions is a reasonable pattern and silent mutation is surprising.
	cfgForTransport := *config
	if normalizedHost != "" {
		cfgForTransport.Host = normalizedHost
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
	httpClient := createHTTPClient(&cfgForTransport)
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
	//
	// Mirror the SDK's host-resolution precedence locally so callers that
	// invoke createHTTPClient directly (most tests, plus future internal
	// callers) get the same dialer/TLS choice as NewClientWithConfig:
	// explicit config.Host > DOCKER_HOST env > client.DefaultDockerHost.
	host := config.Host
	if host == "" {
		host = os.Getenv(client.EnvOverrideHost)
	}
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
		// HTTPS / tcp+tls connections can use HTTP/2 via ALPN.
		// tcp+tls is matched here defensively even though PR #612 currently
		// rejects it at validateAndNormalizeHost; if that allow-list is
		// re-enabled, this branch ensures the connection actually gets TLS.
		transport.ForceAttemptHTTP2 = true
		applyDockerTLS(transport, config)
	case "tcp":
		// Plain tcp:// is the legacy way to talk TLS to a remote daemon:
		// the docker CLI / SDK upgrade the connection to HTTPS when
		// DOCKER_TLS_VERIFY / DOCKER_CERT_PATH are set, even with a tcp://
		// URL. Mirror that here so users following Docker's standard mTLS
		// setup don't see a silent plaintext downgrade. When no TLS env /
		// config is present, applyDockerTLS is a no-op and we stay on
		// plain TCP / HTTP/1.1.
		if hasTLSMaterial(config) {
			transport.ForceAttemptHTTP2 = true
		}
		applyDockerTLS(transport, config)
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
	scheme, _, ok := strings.Cut(host, "://")
	if !ok {
		return ""
	}
	return strings.ToLower(scheme)
}

// validateAndNormalizeHost validates that host uses a supported scheme and
// returns the host with the scheme lowercased.
func validateAndNormalizeHost(host string) (string, error) {
	if host == "" {
		return "", nil
	}

	rawScheme, rest, hasScheme := strings.Cut(host, "://")
	if !hasScheme {
		return "", fmt.Errorf("%w: %q (e.g. \"unix://\", \"tcp://\"); supported schemes: %s",
			ErrMissingDockerHostScheme, host, formatSupportedSchemes())
	}

	scheme := strings.ToLower(rawScheme)
	if slices.Contains(supportedDockerHostSchemes, scheme) {
		return scheme + "://" + rest, nil
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

// hasTLSMaterial reports whether either the explicit ClientConfig fields or
// the DOCKER_CERT_PATH env var would cause resolveTLSConfig to produce a
// non-nil *tls.Config. Used to decide whether tcp:// should be upgraded to
// HTTPS (mirroring the docker CLI's behavior).
func hasTLSMaterial(config *ClientConfig) bool {
	if config.TLSCertPath != "" {
		return true
	}
	return os.Getenv(client.EnvOverrideCertPath) != ""
}

// applyDockerTLS resolves TLS material and assigns it to the transport,
// surfacing misconfiguration via slog rather than silently downgrading.
// No-op when no TLS material is configured.
func applyDockerTLS(transport *http.Transport, config *ClientConfig) {
	tlsCfg, err := resolveTLSConfig(config)
	if err != nil {
		slog.Default().Warn(
			"Docker TLS config could not be loaded; falling back to default TLS without client cert / pinned CA",
			"error", err,
			"hint", "verify DOCKER_CERT_PATH points at a directory containing readable ca.pem, cert.pem, key.pem",
		)
		return
	}
	if tlsCfg != nil {
		transport.TLSClientConfig = tlsCfg
	}
}

// resolveTLSConfig builds a *tls.Config for an HTTPS Docker host using the
// ClientConfig fields with fallback to DOCKER_CERT_PATH / DOCKER_TLS_VERIFY
// environment variables. Returns a nil *tls.Config (with a nil error) when
// no cert path is configured — mirroring docker/docker's
// WithTLSClientConfigFromEnv: empty cert path means no TLS modification.
//
// Precedence:
//   - ClientConfig.TLSCertPath > DOCKER_CERT_PATH env > none
//   - ClientConfig.TLSVerify   > DOCKER_TLS_VERIFY env (any non-empty value
//     enables verification, matching the SDK's semantics)
func resolveTLSConfig(config *ClientConfig) (*tls.Config, error) {
	certPath := config.TLSCertPath
	if certPath == "" {
		certPath = os.Getenv(client.EnvOverrideCertPath)
	}
	if certPath == "" {
		//nolint:nilnil // sentinel "no TLS configured" matches SDK semantics
		return nil, nil
	}

	var verify bool
	if config.TLSVerify != nil {
		verify = *config.TLSVerify
	} else {
		// Mirror docker/docker: InsecureSkipVerify = (DOCKER_TLS_VERIFY == "").
		// Any non-empty value (including "0") implies verify.
		verify = os.Getenv(client.EnvTLSVerify) != ""
	}

	tlsCfg, err := tlsconfig.Client(tlsconfig.Options{
		CAFile:             filepath.Join(certPath, "ca.pem"),
		CertFile:           filepath.Join(certPath, "cert.pem"),
		KeyFile:            filepath.Join(certPath, "key.pem"),
		InsecureSkipVerify: !verify,
	})
	if err != nil {
		return nil, fmt.Errorf("loading docker TLS config from %s (expected ca.pem, cert.pem, key.pem): %w", certPath, err)
	}
	return tlsCfg, nil
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
