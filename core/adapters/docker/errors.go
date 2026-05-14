// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import "errors"

// ErrNilDockerClient is returned by any *ServiceAdapter method when the
// embedded Docker SDK client is nil. The supported wiring (`NewClient` /
// `NewClientWithConfig`, then accessing the per-service adapters via the
// `*Client` accessor methods) always returns adapters with a non-nil
// embedded client, so this is reachable only through hand-rolled adapter
// values (`&ContainerServiceAdapter{}` and friends) — typically a test
// fixture or a wiring bug. Returning a typed error converts what would
// otherwise be a `nil pointer dereference` panic in a hot goroutine into an
// actionable, branchable failure that callers can route via `errors.Is`.
var ErrNilDockerClient = errors.New("docker adapter: nil SDK client")

// ErrTCPTLSRequiresCertMaterial is returned by NewClientWithConfig when
// DOCKER_HOST uses the explicit-TLS scheme `tcp+tls://` but no TLS material
// is configured (no DOCKER_CERT_PATH / DOCKER_TLS_VERIFY env vars and no
// ClientConfig.TLSCertPath / TLSVerify overrides).
//
// Without this gate the SDK would dial TLS using Go's stdlib defaults —
// system CA bundle, NO client certificate — silently downgrading what the
// operator declared as mTLS into an unauthenticated TLS handshake. This
// is the analog, for tcp+tls://, of the silent plain-TCP downgrade
// closed by [#612] / [#625]; tracked in [#627].
//
// `tcp://` and `https://` remain fail-open (tcp:// is ambiguous; https://
// follows the upstream SDK's documented fail-open-with-warning posture).
//
// [#612]: https://github.com/netresearch/ofelia/pull/612
// [#625]: https://github.com/netresearch/ofelia/pull/625
// [#627]: https://github.com/netresearch/ofelia/issues/627
var ErrTCPTLSRequiresCertMaterial = errors.New("tcp+tls:// requires TLS material")

// ErrHTTPSRequiresUsableCertMaterial is returned by NewClientWithConfig when
// DOCKER_HOST uses the explicit-TLS scheme `https://` AND TLS material is
// configured (DOCKER_CERT_PATH or ClientConfig.TLSCertPath) but
// resolveTLSConfig fails to load it (typo in path, unreadable cert.pem,
// malformed ca.pem, secrets not yet populated, broken volume mount).
//
// Without this gate, applyDockerTLS would emit slog.Warn and leave
// TLSClientConfig nil; the SDK would then dial with Go's default TLS — the
// system CA pool, NO client certificate — silently downgrading what the
// operator declared as mTLS into an unauthenticated TLS handshake.
//
// Asymmetry vs ErrTCPTLSRequiresCertMaterial: tcp+tls:// REQUIRES material
// even when nothing is set (the scheme is an explicit mTLS opt-in). https://
// is fail-open when no material is configured (operator legitimately relies
// on the system CA bundle); we only fail closed when material IS configured
// but unloadable. This is the silent-downgrade vector closed by [#653].
//
// [#653]: https://github.com/netresearch/ofelia/issues/653
var ErrHTTPSRequiresUsableCertMaterial = errors.New("https:// has TLS material configured but it is unreadable or invalid")

// ErrNilContainerConfig is returned by ContainerServiceAdapter.Create when
// the supplied *domain.ContainerConfig is nil. Production callers always
// pass a non-nil config (the executor builds it eagerly), so this is
// defense-in-depth — but the previous code dereferenced
// `config.HostConfig` / `config.NetworkConfig` unconditionally and would
// panic on a hand-rolled nil. Returning a typed sentinel keeps the failure
// branchable via errors.Is and consistent with ErrNilExecConfig.
var ErrNilContainerConfig = errors.New("container: nil ContainerConfig")
