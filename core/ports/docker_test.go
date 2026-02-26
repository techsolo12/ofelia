// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package ports

import "testing"

func TestWithHost(t *testing.T) {
	t.Parallel()

	opts := &ClientOptions{}
	WithHost("unix:///var/run/docker.sock")(opts)

	if opts.Host != "unix:///var/run/docker.sock" {
		t.Errorf("Host = %q, want %q", opts.Host, "unix:///var/run/docker.sock")
	}
}

func TestWithVersion(t *testing.T) {
	t.Parallel()

	opts := &ClientOptions{}
	WithVersion("1.45")(opts)

	if opts.Version != "1.45" {
		t.Errorf("Version = %q, want %q", opts.Version, "1.45")
	}
}

func TestWithTLSConfig(t *testing.T) {
	t.Parallel()

	tlsCfg := &TLSConfig{
		CAFile:   "/certs/ca.pem",
		CertFile: "/certs/cert.pem",
		KeyFile:  "/certs/key.pem",
		Insecure: false,
	}

	opts := &ClientOptions{}
	WithTLSConfig(tlsCfg)(opts)

	if opts.TLSConfig != tlsCfg {
		t.Error("TLSConfig was not set correctly")
	}
	if opts.TLSConfig.CAFile != "/certs/ca.pem" {
		t.Errorf("CAFile = %q, want %q", opts.TLSConfig.CAFile, "/certs/ca.pem")
	}
	if opts.TLSConfig.CertFile != "/certs/cert.pem" {
		t.Errorf("CertFile = %q, want %q", opts.TLSConfig.CertFile, "/certs/cert.pem")
	}
	if opts.TLSConfig.KeyFile != "/certs/key.pem" {
		t.Errorf("KeyFile = %q, want %q", opts.TLSConfig.KeyFile, "/certs/key.pem")
	}
	if opts.TLSConfig.Insecure {
		t.Error("Insecure should be false")
	}
}

func TestWithTLSConfig_Nil(t *testing.T) {
	t.Parallel()

	opts := &ClientOptions{}
	WithTLSConfig(nil)(opts)

	if opts.TLSConfig != nil {
		t.Error("TLSConfig should be nil")
	}
}

func TestWithHTTPHeaders(t *testing.T) {
	t.Parallel()

	headers := map[string]string{
		"X-Custom":    "value1",
		"X-Tenant-ID": "tenant-42",
	}

	opts := &ClientOptions{}
	WithHTTPHeaders(headers)(opts)

	if len(opts.HTTPHeaders) != 2 {
		t.Errorf("HTTPHeaders length = %d, want 2", len(opts.HTTPHeaders))
	}
	if opts.HTTPHeaders["X-Custom"] != "value1" {
		t.Errorf("HTTPHeaders[X-Custom] = %q, want %q", opts.HTTPHeaders["X-Custom"], "value1")
	}
	if opts.HTTPHeaders["X-Tenant-ID"] != "tenant-42" {
		t.Errorf("HTTPHeaders[X-Tenant-ID] = %q, want %q", opts.HTTPHeaders["X-Tenant-ID"], "tenant-42")
	}
}

func TestClientOptions_CombinedOptions(t *testing.T) {
	t.Parallel()

	opts := &ClientOptions{}

	options := []ClientOption{
		WithHost("tcp://localhost:2376"),
		WithVersion("1.44"),
		WithTLSConfig(&TLSConfig{Insecure: true}),
		WithHTTPHeaders(map[string]string{"Authorization": "Bearer token"}),
	}

	for _, opt := range options {
		opt(opts)
	}

	if opts.Host != "tcp://localhost:2376" {
		t.Errorf("Host = %q, want %q", opts.Host, "tcp://localhost:2376")
	}
	if opts.Version != "1.44" {
		t.Errorf("Version = %q, want %q", opts.Version, "1.44")
	}
	if !opts.TLSConfig.Insecure {
		t.Error("TLSConfig.Insecure should be true")
	}
	if opts.HTTPHeaders["Authorization"] != "Bearer token" {
		t.Errorf("HTTPHeaders[Authorization] = %q, want %q", opts.HTTPHeaders["Authorization"], "Bearer token")
	}
}
