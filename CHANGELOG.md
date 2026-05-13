# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Docker API version negotiation at startup is now bounded by a configurable `NegotiateTimeout` (default 30s). Previously `NewClientWithConfig` called `NegotiateAPIVersion` with `context.Background()`, so a reachable-but-wedged Docker daemon (e.g. a socket proxy with a hung upstream) could hang Ofelia at startup with no diagnostic output. The deadline-exceeded path now logs a warning so operators can correlate startup slowness with daemon health ([#611](https://github.com/netresearch/ofelia/pull/611), fixes [#608](https://github.com/netresearch/ofelia/issues/608))
- `DOCKER_HOST` scheme is now validated against an allow-list (`unix://`, `tcp://`, `tcp+tls://`, `http://`, `https://`, `npipe://`) and normalized to lowercase. Unsupported schemes (`ssh://`, `fd://`, bogus values) now fail at startup with a clear error instead of silently falling through to a plain-TCP transport. Fixes silent TLS downgrade for `tcp+tls://` and case-sensitivity bug for `TCP://`/`UNIX://` ([#609](https://github.com/netresearch/ofelia/issues/609))

## [0.24.0] - 2026-05-10

### Changed

- **BREAKING:** Docker Compose service-name based job naming now works as documented. The `com.docker.compose.service` label is no longer filtered out, so the `Cross-Container Job References (Docker Compose)` feature from `docs/CONFIGURATION.md` is functional. Users who relied on the previous (incorrect) job names may see different names. ([#597](https://github.com/netresearch/ofelia/pull/597))

### Added

- End-to-end test harness running the compiled binary as a subprocess — covers scheduling, the `validate` command, SIGTERM/SIGINT graceful shutdown, and real Alpine container runs ([#581](https://github.com/netresearch/ofelia/pull/581))

### Fixed

- `log-level` invalid-value error now lists all accepted levels ([#599](https://github.com/netresearch/ofelia/pull/599))
- `make lint` works again — `golangci-lint` is now installed via the v2 module path ([#600](https://github.com/netresearch/ofelia/pull/600))
- `.envrc` hooks detection inside git worktrees ([#598](https://github.com/netresearch/ofelia/pull/598))
- `.gitignore` `/ofelia` pattern is anchored so it cannot shadow source files ([#574](https://github.com/netresearch/ofelia/pull/574))
- Stabilize flaky tests for scheduler shutdown, retry backoff, and rate limiter ([#582](https://github.com/netresearch/ofelia/pull/582), [#601](https://github.com/netresearch/ofelia/pull/601))

### Security

- Bump Go to 1.26.2 for stdlib security fixes ([#557](https://github.com/netresearch/ofelia/pull/557))

### Dependencies

- Bump `github.com/netresearch/go-cron` 0.13.1 → 0.14.0 ([#553](https://github.com/netresearch/ofelia/pull/553), [#563](https://github.com/netresearch/ofelia/pull/563))
- Bump `github.com/docker/cli` 29.3.0 → 29.4.0 ([#548](https://github.com/netresearch/ofelia/pull/548), [#559](https://github.com/netresearch/ofelia/pull/559))
- Bump `github.com/docker/go-connections` 0.6.0 → 0.7.0 ([#564](https://github.com/netresearch/ofelia/pull/564))
- Bump `github.com/go-playground/validator/v10` 10.30.1 → 10.30.2 ([#552](https://github.com/netresearch/ofelia/pull/552))
- Bump `github.com/go-viper/mapstructure/v2` 2.4.0 → 2.5.0 ([#549](https://github.com/netresearch/ofelia/pull/549))
- Bump `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` 1.42.0 → 1.43.0 ([#556](https://github.com/netresearch/ofelia/pull/556))
- Bump `golang.org/x/crypto`, `golang.org/x/term`, `golang.org/x/text` ([#558](https://github.com/netresearch/ofelia/pull/558), [#560](https://github.com/netresearch/ofelia/pull/560), [#561](https://github.com/netresearch/ofelia/pull/561))
- Bump go-dependencies group ([#596](https://github.com/netresearch/ofelia/pull/596))
- Bump `alpine` Docker base image ([#569](https://github.com/netresearch/ofelia/pull/569))
- Bump GitHub Actions groups ([#550](https://github.com/netresearch/ofelia/pull/550), [#554](https://github.com/netresearch/ofelia/pull/554), [#562](https://github.com/netresearch/ofelia/pull/562))

### CI / Build

- Adopt unified single-build release pipeline via `netresearch/.github` reusable workflows ([#566](https://github.com/netresearch/ofelia/pull/566), [#587](https://github.com/netresearch/ofelia/pull/587))
- Migrate auto-merge to org-level reusable workflow ([#567](https://github.com/netresearch/ofelia/pull/567))
- Drop `integration.yml` — superseded by `go-check` ([#579](https://github.com/netresearch/ofelia/pull/579))
- Stop Trivy FS scan from blocking PRs on pre-existing CVEs ([#555](https://github.com/netresearch/ofelia/pull/555))
- Fix auto-merge for Dependabot/Renovate PRs ([#551](https://github.com/netresearch/ofelia/pull/551))
- Use cosign `--bundle` for checksums signing ([#547](https://github.com/netresearch/ofelia/pull/547))
- Grant `security-events: write` to satisfy reusable workflow ([#585](https://github.com/netresearch/ofelia/pull/585))

### Refactor

- Extract repeated string literals flagged by `goconst` ([#599](https://github.com/netresearch/ofelia/pull/599))

## [0.23.1] - 2026-03-23

### Fixed

- Migrate release pipeline from `slsa-github-generator` to `actions/attest-build-provenance` via org-wide reusable workflow — fixes release builds blocked by SHA-pinning ruleset ([#542](https://github.com/netresearch/ofelia/pull/542))

### Security

- Migrate `go-viper/mapstructure` v1 to v2.4.0 — fixes GO-2025-3787 and GO-2025-3900 (sensitive information leak in logs) ([#544](https://github.com/netresearch/ofelia/pull/544))

## [0.23.0] - 2026-03-22

### Added

- `env-file` support: load environment variables from files for all job types, like Docker's `--env-file` ([#540](https://github.com/netresearch/ofelia/pull/540), closes [#314](https://github.com/netresearch/ofelia/issues/314))
- `env-from` support: copy environment variables from a running Docker container at job execution time ([#540](https://github.com/netresearch/ofelia/pull/540), closes [#336](https://github.com/netresearch/ofelia/issues/336), [#351](https://github.com/netresearch/ofelia/issues/351))

### Fixed

- Environment variable substitutions containing `#` or `;` were parsed as INI inline comments, truncating values like SMTP passwords ([#539](https://github.com/netresearch/ofelia/pull/539), fixes [#538](https://github.com/netresearch/ofelia/issues/538))
- Environment variable expansion now works in webhook config values (`secret`, `url`, etc.) and section names ([#539](https://github.com/netresearch/ofelia/pull/539))
- `log-level` config value now supports `${VAR}` expansion in the pre-parse path ([#539](https://github.com/netresearch/ofelia/pull/539))

### Security

- SHA-pin all GitHub Actions and add Dependabot for actions updates ([#536](https://github.com/netresearch/ofelia/pull/536))

### Dependencies

- Bump the github-actions group with 20 updates ([#537](https://github.com/netresearch/ofelia/pull/537))

## [0.22.0] - 2026-03-20

### Added

- Environment variable substitution in INI config files with `${VAR}` and `${VAR:-default}` syntax ([#532](https://github.com/netresearch/ofelia/pull/532), closes [#362](https://github.com/netresearch/ofelia/issues/362))

### Dependencies

- Bump `aquasecurity/trivy-action` from 0.28.0 to v0.35.0 ([#532](https://github.com/netresearch/ofelia/pull/532))
- Bump `step-security/harden-runner` from v2.12.0 to v2.16.0 ([#533](https://github.com/netresearch/ofelia/pull/533))
- Bump `codecov/codecov-action` from v5.5.2 to v5.5.3 ([#533](https://github.com/netresearch/ofelia/pull/533))
- Bump `go.opentelemetry.io/otel` from v1.40.0 to v1.42.0 ([#533](https://github.com/netresearch/ofelia/pull/533))
- Bump `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` from v1.38.0 to v1.42.0 ([#533](https://github.com/netresearch/ofelia/pull/533))
- Bump `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` from v0.65.0 to v0.67.0 ([#533](https://github.com/netresearch/ofelia/pull/533))
- Bump `go.opentelemetry.io/proto/otlp` from v1.9.0 to v1.10.0 ([#533](https://github.com/netresearch/ofelia/pull/533))
- Bump `google.golang.org/protobuf` from v1.36.10 to v1.36.11 ([#533](https://github.com/netresearch/ofelia/pull/533))
- Bump `google.golang.org/grpc` from v1.77.0 to v1.79.3 ([#531](https://github.com/netresearch/ofelia/pull/531))

## [0.21.5] - 2026-03-18

### Added

- `ofelia version` command and `--version` flag ([#528](https://github.com/netresearch/ofelia/pull/528))
- `job-service-run` now supports `volume` for mounting host directories and named volumes ([#529](https://github.com/netresearch/ofelia/pull/529), closes [#527](https://github.com/netresearch/ofelia/issues/527))

## [0.21.4] - 2026-03-17

### Fixed

- Fix `job-service-run` network not attached to service ([#525](https://github.com/netresearch/ofelia/pull/525), closes [#524](https://github.com/netresearch/ofelia/issues/524))
  - `convertToSwarmSpec` now reads networks from both `ServiceSpec.Networks` and `TaskTemplate.Networks`
- Complete `convertFromSwarmService` with missing field conversions: Mounts, RestartPolicy, Resources, Networks, Mode, Placement, LogDriver, EndpointSpec ([#525](https://github.com/netresearch/ofelia/pull/525))

### Added

- Swarm service adapter now converts Placement, LogDriver, and EndpointSpec in both directions ([#525](https://github.com/netresearch/ofelia/pull/525))
- 13 round-trip tests for the service adapter conversion layer ([#525](https://github.com/netresearch/ofelia/pull/525))

## [0.21.3] - 2026-03-15

### Fixed

- Wire missing container spec fields across job types ([#520](https://github.com/netresearch/ofelia/pull/520), closes [#519](https://github.com/netresearch/ofelia/issues/519))
  - `job-service-run`: add `environment`, `hostname`, `dir` support
  - `job-run`: add `working-dir` support, wire `volumes-from` (was in struct but unused)
  - `job-exec`: add `privileged` support
  - Fix misleading documentation claiming `job-service-run` inherits from `RunJob`

## [0.21.2] - 2026-03-14

### Security

- Hide `WebPasswordHash` and `WebSecretKey` from `/api/config` endpoint ([#511](https://github.com/netresearch/ofelia/pull/511))
- Remove CSRF bypass via `X-Requested-With` header ([#511](https://github.com/netresearch/ofelia/pull/511))
- Implement rate limiter cleanup to prevent memory exhaustion DoS ([#511](https://github.com/netresearch/ofelia/pull/511))
- Only trust `X-Forwarded-For` and `X-Real-IP` from trusted proxies to prevent IP spoofing ([#511](https://github.com/netresearch/ofelia/pull/511))
- Make trusted proxies configurable via `web-trusted-proxies` ([#511](https://github.com/netresearch/ofelia/pull/511))

### Fixed

- Propagate context to Docker API calls so cancellation and shutdown reach containers ([#511](https://github.com/netresearch/ofelia/pull/511))
- Prevent double-close panic on daemon done channel ([#511](https://github.com/netresearch/ofelia/pull/511))
- Add mutex to Config to prevent concurrent map access crash ([#511](https://github.com/netresearch/ofelia/pull/511))
- Execute shutdown hooks in priority groups, not all concurrently ([#511](https://github.com/netresearch/ofelia/pull/511))
- Enforce shutdown timeout even when hooks ignore context ([#511](https://github.com/netresearch/ofelia/pull/511))
- Return `NonZeroExitError` for non-zero Swarm service exit codes ([#511](https://github.com/netresearch/ofelia/pull/511))

### Dependencies

- Bump `golang.org/x/crypto` from 0.48.0 to 0.49.0 ([#512](https://github.com/netresearch/ofelia/pull/512))
- Bump `github.com/netresearch/go-cron` from 0.13.0 to 0.13.1 ([#514](https://github.com/netresearch/ofelia/pull/514))
- Bump `golang.org/x/time` from 0.14.0 to 0.15.0 ([#515](https://github.com/netresearch/ofelia/pull/515))

## [0.17.0] - 2025-12-22

### Added

- **Secure Web Authentication** ([#408](https://github.com/netresearch/ofelia/pull/408))
  - Complete bcrypt password hashing with HMAC session tokens
  - Secure cookie handling with HttpOnly, Secure, and SameSite flags
  - Support for reverse proxy HTTPS detection (X-Forwarded-Proto)
  - Password hashing utility: `ofelia hashpw`

- **Doctor Command Enhancements** ([#408](https://github.com/netresearch/ofelia/pull/408))
  - Web authentication configuration checks in `ofelia doctor`
  - Validates password hash format and token secret strength

- **ntfy-token Preset** ([#409](https://github.com/netresearch/ofelia/pull/409))
  - Bearer token authentication for self-hosted ntfy instances
  - Supports both ntfy.sh and self-hosted deployments with access tokens

- **Webhook Host Whitelist** ([#410](https://github.com/netresearch/ofelia/pull/410))
  - New `webhook-allowed-hosts` configuration option
  - Default: `*` (allow all hosts) - consistent with local command trust model
  - Whitelist mode when specific hosts are configured
  - Supports domain wildcards (e.g., `*.slack.com`)

- **CronClock Interface** ([#412](https://github.com/netresearch/ofelia/pull/412))
  - Testable time abstraction for scheduler testing
  - FakeClock implementation for instant, deterministic tests
  - go-cron compatible Timer interface

### Security

- **Cookie Security Hardening** ([#411](https://github.com/netresearch/ofelia/pull/411))
  - Secure, HttpOnly, and SameSite=Lax flags on all cookies
  - HTTPS detection for reverse proxy deployments
  - Security boundaries ADR documenting responsibility model

- **GitHub Actions Pinning** ([#411](https://github.com/netresearch/ofelia/pull/411))
  - All workflow actions pinned to SHA for supply chain security
  - CodeQL updated to v3.31.9

### Improved

- **Test Infrastructure** ([#412](https://github.com/netresearch/ofelia/pull/412))
  - Complete gocheck to stdlib+testify migration
  - Eventually pattern replacing time.Sleep-based synchronization
  - Parallel test execution with t.Parallel()
  - Race condition fixes detected by -race flag

- **Performance** ([#412](https://github.com/netresearch/ofelia/pull/412))
  - Sub-second scheduling for faster test execution
  - Optimized pre-commit and pre-push hooks
  - Test suite runtime reduced by ~80%

- **Linting** ([#413](https://github.com/netresearch/ofelia/pull/413))
  - Comprehensive golangci-lint configuration audit
  - All linting issues resolved

### Documentation

- **Security Boundaries ADR** ([#411](https://github.com/netresearch/ofelia/pull/411))
  - ADR-002 documenting security responsibility model
  - Clear separation between Ofelia and infrastructure responsibilities

- **Webhook Documentation** ([#410](https://github.com/netresearch/ofelia/pull/410))
  - Host whitelist configuration guide
  - Security model explanation

## [0.16.0] - 2025-12-10

### Fixed

- **Docker Socket HTTP/2 Compatibility**
  - Fixed Docker client connection failures on non-TLS connections introduced in v0.11.0
  - OptimizedDockerClient now only enables HTTP/2 for HTTPS (TLS) connections
  - HTTP/2 is disabled for Unix sockets, tcp://, and http:// (Docker daemon only supports HTTP/2 over TLS with ALPN)
  - Resolves "protocol error" issues when connecting to `/var/run/docker.sock` or `tcp://localhost:2375`
  - HTTP/2 enabled only for `https://` connections where Docker daemon supports ALPN negotiation
  - Added comprehensive unit tests covering all connection types (9 scenarios)
  - Technical details: Docker daemon does not implement h2c (HTTP/2 cleartext) - HTTP/2 requires TLS

## [0.11.0] - 2025-11-21

### Critical Fixes

- **Command Parsing in Swarm Services** ([#254](https://github.com/netresearch/ofelia/pull/254))
  - Fixed critical bug where `strings.Split` broke quoted arguments in Docker Swarm service commands
  - Now uses `args.GetArgs()` to properly handle commands like `sh -c "echo hello world"`
  - Prevents command execution failures in complex shell commands

- **LocalJob Empty Command Panic** ([#254](https://github.com/netresearch/ofelia/pull/254))
  - Fixed documented bug where empty commands caused runtime panic
  - Now returns proper error instead of crashing
  - Prevents service crashes from malformed job configurations

### Security

- **API Security Validation** ([#254](https://github.com/netresearch/ofelia/pull/254))
  - Added validation for LocalJob and ComposeJob API endpoints
  - Prevents command injection attacks via API
  - Validates file paths, service names, and command arguments

- **Privilege Escalation Logging** ([#244](https://github.com/netresearch/ofelia/pull/244))
  - Enhanced logging for security monitoring
  - Better detection of privilege escalation attempts

- **Dependency Updates**
  - Updated golang.org/x/crypto to v0.45.0 for CVE fixes

### Performance

- **Enhanced Buffer Pool** ([#245](https://github.com/netresearch/ofelia/pull/245))
  - Multi-tier adaptive pooling system
  - 99.97% memory usage reduction (2000 MB → 0.5 MB for 100 executions)
  - Automatic size adjustment and pool warmup

- **Optimized Docker Client** ([#245](https://github.com/netresearch/ofelia/pull/245))
  - Connection pooling for reduced overhead
  - Thread-safe concurrent operations
  - Health monitoring and automatic recovery

- **Reduced Polling** ([#254](https://github.com/netresearch/ofelia/pull/254))
  - Increased legacy polling interval from 500ms to 2s
  - 75% reduction in Docker API calls (200/min → 50/min per job)
  - Significant CPU and network usage improvement

- **Performance Metrics Framework** ([#245](https://github.com/netresearch/ofelia/pull/245))
  - Comprehensive metrics for Docker operations
  - Memory, latency, and throughput tracking
  - Real-time performance monitoring

### Added

- **Container Annotations**
  - Support for custom annotations on RunJob and RunServiceJob
  - Default Ofelia annotations for job tracking
  - User-defined metadata for containers and services

- **WorkingDir for ExecJob**
  - Support for setting working directory in exec jobs
  - Backward compatible with existing configurations

- **Opt-in Validation**
  - New `enable-strict-validation` flag
  - Allows gradual migration to strict validation
  - Prevents breaking changes for existing users

- **Git Hooks with Lefthook**
  - Go-native git hooks for better portability
  - Pre-commit, commit-msg, pre-push, post-checkout, post-merge hooks
  - Automated code quality checks and security scans

### Documentation

- **Architecture Diagrams** ([#252](https://github.com/netresearch/ofelia/pull/252))
  - System architecture overview
  - Component interaction diagrams
  - Data flow visualization

- **Complete Package Documentation** ([#247](https://github.com/netresearch/ofelia/pull/247))
  - Comprehensive package-level documentation
  - Security guides and best practices
  - Practical usage guides

- **Docker Requirements**
  - Documented minimum Docker version requirements
  - API compatibility notes

- **Exit Code Documentation** ([#254](https://github.com/netresearch/ofelia/pull/254))
  - Clear documentation of Ofelia-specific exit codes
  - Swarm service error codes (-999, -998)

### Fixed

- **Go Version Check** ([#251](https://github.com/netresearch/ofelia/pull/251))
  - Corrected inverted logic in .envrc Go version check
  - Ensures correct Go version enforcement

### Changed

- Updated go-dockerclient to v1.12.2
- Migrated from Husky to Lefthook for git hooks
- Improved CI/CD pipeline with comprehensive security scanning

### Internal

- Removed AI assistant artifacts and outdated documentation ([#246](https://github.com/netresearch/ofelia/pull/246), [#253](https://github.com/netresearch/ofelia/pull/253))
- Enhanced test suite with comprehensive integration tests
- Improved code organization and maintainability

## [0.10.2] - 2025-11-15

Previous release.

---

## Migration Guide v0.10.x → v0.11.0

### Breaking Changes

**None** - This release is backward compatible with v0.10.x

### Recommended Actions

1. **Review API Usage**: If you create jobs via API, ensure commands are properly validated
2. **Check Swarm Commands**: Verify complex shell commands in service jobs work correctly
3. **Monitor Performance**: Observe improved memory usage and reduced API calls
4. **Enable Metrics**: Consider enabling the new metrics framework for monitoring

### New Configuration Options

```ini
# Optional: Enable strict validation (default: false)
[global]
enable-strict-validation = true

# New: Container annotations
[job-run "example"]
annotations = com.example.key=value, app.version=1.0
```

### Deprecations

**None** in this release.

---

For more information, see:
- [Documentation](https://github.com/netresearch/ofelia/tree/main/docs)
- [Security Guide](https://github.com/netresearch/ofelia/blob/main/docs/SECURITY.md)
- [Configuration Guide](https://github.com/netresearch/ofelia/blob/main/docs/CONFIGURATION.md)
