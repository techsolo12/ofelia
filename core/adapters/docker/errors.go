// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import "errors"

// ErrNilDockerClient is returned by any *ServiceAdapter method when the
// embedded Docker SDK client is nil. The exported `New*Adapter` constructors
// always wire a non-nil client, so this is reachable only through hand-rolled
// adapter values (`&ContainerServiceAdapter{}` and friends) — typically a
// test fixture or a wiring bug. Returning a typed error converts what would
// otherwise be a `nil pointer dereference` panic in a hot goroutine into an
// actionable, branchable failure that callers can route via `errors.Is`.
var ErrNilDockerClient = errors.New("docker adapter: nil SDK client")
