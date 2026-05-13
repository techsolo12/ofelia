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
