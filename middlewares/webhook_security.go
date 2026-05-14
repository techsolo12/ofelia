// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package middlewares

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// securityMu protects global security configuration for thread-safe access
var securityMu sync.RWMutex

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// ValidateWebhookURLImpl validates basic URL requirements.
// This is the default validator that allows all hosts (consistent with local command trust model).
// For whitelist mode, use WebhookSecurityValidator with specific AllowedHosts.
func ValidateWebhookURLImpl(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTP and HTTPS
	if u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}

	// Must have a host
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	// Extract hostname (without port)
	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	return nil
}

// WebhookSecurityConfig holds security configuration for webhooks
type WebhookSecurityConfig struct {
	// AllowedHosts controls which hosts webhooks can target.
	// "*" = allow all hosts (default, consistent with local command trust model)
	// Specific list = whitelist mode, only those hosts allowed
	// Supports wildcards: "*.example.com"
	AllowedHosts []string
}

// DefaultWebhookSecurityConfig returns the default security configuration
// Default: AllowedHosts=["*"] for consistency with local command execution trust model
func DefaultWebhookSecurityConfig() *WebhookSecurityConfig {
	return &WebhookSecurityConfig{
		AllowedHosts: []string{"*"}, // Allow all by default
	}
}

// WebhookSecurityValidator validates URLs with configurable security rules
type WebhookSecurityValidator struct {
	config *WebhookSecurityConfig
}

// NewWebhookSecurityValidator creates a new security validator
func NewWebhookSecurityValidator(config *WebhookSecurityConfig) *WebhookSecurityValidator {
	if config == nil || len(config.AllowedHosts) == 0 {
		config = DefaultWebhookSecurityConfig()
	}
	return &WebhookSecurityValidator{config: config}
}

// Validate checks if a URL is safe to access based on the allowed hosts configuration.
// If AllowedHosts contains "*", all hosts are allowed (default behavior).
// Otherwise, only hosts in the whitelist are allowed.
func (v *WebhookSecurityValidator) Validate(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTP and HTTPS
	if u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS {
		return fmt.Errorf("URL scheme must be http or https")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Check if all hosts are allowed (default behavior)
	if v.isAllowAll() {
		return nil
	}

	// Whitelist mode: only allow hosts in the list
	if !v.isAllowedHost(hostname) {
		return fmt.Errorf("host %q is not in allowed hosts list", hostname)
	}

	return nil
}

// isAllowAll checks if the configuration allows all hosts
func (v *WebhookSecurityValidator) isAllowAll() bool {
	return slices.Contains(v.config.AllowedHosts, "*")
}

// isAllowedHost checks if a hostname matches the allowed hosts list
func (v *WebhookSecurityValidator) isAllowedHost(hostname string) bool {
	lowerHost := strings.ToLower(hostname)
	for _, allowed := range v.config.AllowedHosts {
		lowerAllowed := strings.ToLower(allowed)

		// Exact match
		if lowerHost == lowerAllowed {
			return true
		}

		// Wildcard match (e.g., "*.example.com")
		if strings.HasPrefix(lowerAllowed, "*.") {
			suffix := lowerAllowed[1:] // Keep the dot
			if strings.HasSuffix(lowerHost, suffix) {
				return true
			}
		}
	}
	return false
}

// SetGlobalSecurityConfig sets the global security configuration for webhooks
// This should be called during initialization with the parsed configuration.
//
// Emits a single startup-time slog.Warn when the resolved AllowedHosts
// collapses to ["*"] (whether explicitly configured or by default). A typo
// in the `webhook-allowed-hosts` INI key would otherwise yield wide-open
// egress with no operator-visible signal that the allow-list they thought
// they had configured is actually empty. Tracked in
// https://github.com/netresearch/ofelia/issues/653.
//
// Passing nil restores the package defaults silently — used by tests and
// reload paths that revert state. The warning is reserved for
// operator-meaningful startup state.
func SetGlobalSecurityConfig(config *WebhookSecurityConfig) {
	securityMu.Lock()
	defer securityMu.Unlock()

	// Update the global validator to use the new config
	if config != nil {
		warnIfWideOpenEgress(config)
		validator := NewWebhookSecurityValidator(config)
		validateWebhookURLFunc = validator.Validate
		transportFactoryFunc = func() *http.Transport {
			return NewConfigurableTransport(config)
		}
	} else {
		validateWebhookURLFunc = ValidateWebhookURLImpl
		transportFactoryFunc = NewSafeTransport
	}
}

// warnIfWideOpenEgress emits a single slog.Warn when the resolved
// AllowedHosts admits everything (empty list or contains the "*" wildcard).
// Mirrors the silent-downgrade pattern: operator INTENT was to restrict
// egress; the resolved STATE is wide open. A surfaced warning at startup
// converts a silent misconfiguration into a visible one.
//
// One emission per SetGlobalSecurityConfig call by design. The function
// is invoked from two places: (1) NewWebhookManager at daemon startup, and
// (2) the live-reload path that re-initializes the manager when
// `webhook-allowed-hosts` changes (cli/config.go's
// refreshWebhookManagerOnGlobalChange). Both paths are operator-driven,
// so re-warning on each is the correct cadence — operators who tighten
// the allow-list at runtime then loosen it back to "*" should hear about
// the regression. Per-validator construction (NewWebhookSecurityValidator)
// does NOT warn because it is invoked per-request and would be log-spam.
func warnIfWideOpenEgress(config *WebhookSecurityConfig) {
	if config == nil {
		return
	}
	allowAll := len(config.AllowedHosts) == 0 || slices.Contains(config.AllowedHosts, "*")
	if !allowAll {
		return
	}
	slog.Default().Warn(
		"webhook AllowedHosts admits all hosts; webhook egress is wide open",
		"allowed_hosts", config.AllowedHosts,
		"hint", "set webhook-allowed-hosts in [global] to a "+
			"comma-separated list of hostnames or wildcards "+
			"(e.g. *.slack.com) to restrict egress",
		"see", "https://github.com/netresearch/ofelia/issues/653",
	)
}

// getValidateWebhookURL returns the current URL validator with thread-safe access
func getValidateWebhookURL() func(string) error {
	securityMu.RLock()
	defer securityMu.RUnlock()
	return validateWebhookURLFunc
}

// getTransportFactory returns the current transport factory with thread-safe access
func getTransportFactory() func() *http.Transport {
	securityMu.RLock()
	defer securityMu.RUnlock()
	return transportFactoryFunc
}

// SetValidateWebhookURLForTest allows tests to override the URL validator (thread-safe)
func SetValidateWebhookURLForTest(fn func(string) error) {
	securityMu.Lock()
	defer securityMu.Unlock()
	validateWebhookURLFunc = fn
}

// SetTransportFactoryForTest allows tests to override the transport factory (thread-safe)
func SetTransportFactoryForTest(fn func() *http.Transport) {
	securityMu.Lock()
	defer securityMu.Unlock()
	transportFactoryFunc = fn
}

// SecurityConfigFromGlobal creates a WebhookSecurityConfig from WebhookGlobalConfig
func SecurityConfigFromGlobal(global *WebhookGlobalConfig) *WebhookSecurityConfig {
	if global == nil {
		return DefaultWebhookSecurityConfig()
	}

	config := &WebhookSecurityConfig{}

	// Parse allowed hosts from comma-separated string
	// Default is "*" (allow all) if not specified
	allowedHosts := global.AllowedHosts
	if allowedHosts == "" {
		allowedHosts = "*"
	}

	hosts := strings.SplitSeq(allowedHosts, ",")
	for h := range hosts {
		h = strings.TrimSpace(h)
		if h != "" {
			config.AllowedHosts = append(config.AllowedHosts, h)
		}
	}

	return config
}

// validateWebhookURLFunc is the internal storage for the URL validator
var validateWebhookURLFunc = ValidateWebhookURLImpl

// transportFactoryFunc is the internal storage for the transport factory
var transportFactoryFunc = NewSafeTransport

// ValidateWebhookURL validates a URL for webhook requests (thread-safe access)
var ValidateWebhookURL = func(rawURL string) error {
	fn := getValidateWebhookURL()
	return fn(rawURL)
}

// TransportFactory creates HTTP transports for webhook requests (thread-safe access)
var TransportFactory = func() *http.Transport {
	fn := getTransportFactory()
	return fn()
}

// NewSafeTransport creates a standard HTTP transport.
// URL validation is handled by the security validator before requests are made.
func NewSafeTransport() *http.Transport {
	return NewConfigurableTransport(DefaultWebhookSecurityConfig())
}

// NewConfigurableTransport creates a standard HTTP transport.
// Security validation is handled by WebhookSecurityValidator before requests are made.
// The transport itself doesn't need additional restrictions since we follow
// the "trust the config" model - if users can run arbitrary commands, they can
// send webhooks to any configured destination.
func NewConfigurableTransport(config *WebhookSecurityConfig) *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
