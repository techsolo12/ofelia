# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `tcp+tls://` is back on the `DOCKER_HOST` allow-list now that the TLS plumbing from [#613](https://github.com/netresearch/ofelia/pull/613) wires `DOCKER_CERT_PATH` / `DOCKER_TLS_VERIFY` (and the equivalent `ClientConfig.TLSCertPath` / `TLSVerify` overrides) into the custom HTTP transport. PR [#612](https://github.com/netresearch/ofelia/pull/612) had withheld it to avoid a silent plain-TCP downgrade; that risk is now closed by the existing `TestCreateHTTPClient_TCPPlusTLSEnablesTLS` regression test plus the new `TestNewClientWithConfig_TCPPlusTLSScheme` allow-list assertion. ([#625](https://github.com/netresearch/ofelia/pull/625), fixes [#616](https://github.com/netresearch/ofelia/issues/616))

### Changed

- **BREAKING:** `DOCKER_HOST` scheme is now validated against an allow-list (`unix://`, `tcp://`, `tcp+tls://`, `http://`, `https://`, `npipe://`) and normalized to lowercase. Unsupported schemes (`ssh://`, `fd://`, bogus values) now fail at startup with a clear error instead of silently falling through to a plain-TCP transport. Fixes case-sensitivity bug for `TCP://` / `UNIX://`. Configurations that previously relied on the silent fallthrough will now fail loudly — operators using `ssh://` should switch to an SSH-forwarded socket and point `DOCKER_HOST` at the forwarded `unix://` path. ([#612](https://github.com/netresearch/ofelia/pull/612), fixes [#609](https://github.com/netresearch/ofelia/issues/609))
- Webhook global config now lives at a single source of truth: `c.WebhookConfigs.Global` aliases `&c.Global.WebhookGlobalConfig`, eliminating the dual-store antipattern that PR [#618](https://github.com/netresearch/ofelia/pull/618) papered over with a hand-rolled `syncGlobalWebhookConfig` copy. Every entry point that mutates the embedded struct from the INI side (initial INI parse, INI live-reload) is automatically visible to `WebhookManager` without an explicit sync call. INI live-reload of `webhook-allowed-hosts` now also re-runs `WebhookManager.InitManager()` so the URL validator picks up the new whitelist at runtime — previously the data store was refreshed but the security validator stayed snapshotted at startup, so tightening the whitelist via live-reload had no enforcement effect until restart. The Docker label sync path still parses into a scratch Config and merges back via `mergeWebhookConfigs`/`syncWebhookConfigs`, which only forward the `webhook-webhooks` selector and per-webhook definitions — see follow-up. ([#620](https://github.com/netresearch/ofelia/issues/620))
- `PresetLoader` now caches a single `*http.Client` (constructed in `NewPresetLoader` from `TransportFactory()`) instead of building a fresh client and `*http.Transport` on every `loadFromURL` call. Bursty preset fetches now share the underlying connection pool / idle-conn reuse. Test ordering constraint: tests overriding the transport factory via `SetTransportFactoryForTest` MUST install the override BEFORE calling `NewPresetLoader` — replacing the factory afterwards has no effect on the cached client. The deprecated Slack middleware (`middlewares/slack.go`) also routes its fallback `*http.Client` through `TransportFactory()` so notifications inherit the webhook stack's TLS / proxy posture instead of `http.DefaultTransport`; defense-in-depth for the deprecation window. ([#630](https://github.com/netresearch/ofelia/issues/630))

### Deprecated

- Docker label key `ofelia.webhooks` is renamed to `ofelia.webhook-webhooks` to match the documented INI `[global]` key name. A user copying their INI `webhook-webhooks` value verbatim into Docker labels previously hit an "Unknown global label keys" warning and silently lost the value. The legacy `ofelia.webhooks` form still works for backward compatibility but logs a one-shot deprecation warning per process — migrate to `ofelia.webhook-webhooks` before the next major release. The other unprefixed legacy forms (`ofelia.allow-remote-presets`, `ofelia.trusted-preset-sources`, `ofelia.preset-cache-ttl`, `ofelia.preset-cache-dir`) were never accepted from labels because their canonical forms remain INI-only for SSRF reasons (see [#486](https://github.com/netresearch/ofelia/issues/486)). ([#620](https://github.com/netresearch/ofelia/issues/620))

  **Before:**
  ```yaml
  labels:
    ofelia.webhooks: "slack-alerts"  # legacy — emits one-shot deprecation warning
  ```

  **After:**
  ```yaml
  labels:
    ofelia.webhook-webhooks: "slack-alerts"  # canonical — matches INI [global]
  ```

### Security

- Remote preset fetches (`webhook-allow-remote-presets = true`) now route through the same `TransportFactory()` used by the webhook stack instead of `http.DefaultClient`. The previous code relied implicitly on Go stdlib defaults for TLS verification — safe today, but untested and easy to regress if a future change mutates `http.DefaultTransport`. The TLS posture is now explicit, centrally configurable alongside webhook delivery, and pinned by regression tests (self-signed cert rejection + `InsecureSkipVerify` posture check). No behavior change for operators on default config. ([#615](https://github.com/netresearch/ofelia/issues/615))
- Docker SDK adapter now honors `DOCKER_TLS_VERIFY` and `DOCKER_CERT_PATH` for HTTPS / `tcp+tls` hosts. The custom HTTP client previously replaced the SDK's `FromEnv`-configured TLS transport wholesale, silently discarding the client cert and pinned CA. Connections to mTLS-protected Docker daemons proceeded without a client cert and against the system CA pool — operators believing they had mTLS were getting unauthenticated connections. New `ClientConfig.TLSCertPath` / `TLSVerify` fields allow explicit override with config > env precedence. **Upgrade impact:** if your `https://` Docker daemon previously accepted Ofelia connections without verifying client certs, upgrading will cause the dial to fail until valid `ca.pem` / `cert.pem` / `key.pem` exist at `DOCKER_CERT_PATH`. ([#613](https://github.com/netresearch/ofelia/pull/613), fixes [#607](https://github.com/netresearch/ofelia/issues/607))
- Docker SDK adapter now fails closed when `DOCKER_HOST=tcp+tls://...` is set without TLS material (`DOCKER_CERT_PATH` / `DOCKER_TLS_VERIFY` env vars or `ClientConfig.TLSCertPath` / `TLSVerify` overrides). Previously `resolveTLSConfig` returned `(nil, nil)` and the SDK dialed TLS using Go's stdlib defaults — system CA bundle, **no** client cert — silently downgrading the operator's declared mTLS into an unauthenticated TLS handshake against any daemon that did not strictly require client auth. `tcp+tls://` is an *explicit* TLS opt-in (unlike the ambiguous `tcp://`), so the new typed sentinel `ErrTCPTLSRequiresCertMaterial` makes the misconfiguration loud at startup rather than silent at runtime. `tcp://` and `https://` remain fail-open. **Upgrade impact:** if you set `DOCKER_HOST=tcp+tls://...` without configuring TLS material, Ofelia will now refuse to start — set `DOCKER_CERT_PATH` (and optionally `DOCKER_TLS_VERIFY`) to a directory containing readable `ca.pem` / `cert.pem` / `key.pem`, or switch to `https://` if you genuinely want fail-open-with-warning. ([#627](https://github.com/netresearch/ofelia/issues/627), surfaced during review of [#625](https://github.com/netresearch/ofelia/pull/625))

### Fixed

- Bound the seven remaining unbounded `context.Background()` / `context.TODO()` sites in `core/` so a wedged Docker upstream can no longer stall job execution indefinitely. Sibling-hunt during PR [#636](https://github.com/netresearch/ofelia/pull/636) (which fixed [#614](https://github.com/netresearch/ofelia/issues/614) by bounding the cli/web Docker pings) flagged seven more in the job-execution path: `core/common.go:NewContext` (foundational — every job inherited the unbounded default), `core/scheduler.go:maxConcurrentSkipJob.Run` and `jobWrapper.Run` (defensive `cron.Job` fallbacks), and the four `if runCtx == nil { runCtx = context.Background() }` blocks in `execjob.go`, `runjob.go`, `runservice.go`, `localjob.go`. The fix introduces a single `(*Context).RunContext()` helper that centralizes the nil-fallback, plus a scheduler-level `boundJobContext` that wraps every per-run context with `context.WithTimeout(parent, MaxRuntime)` derived from the new `MaxRuntimeProvider` interface (implemented by `RunJob` and `RunServiceJob` — the two job types that already carry a `MaxRuntime` field), or with `defaultJobMaxRuntime = 24h` for `ExecJob`, `LocalJob`, and `ComposeJob`. Note that `[global] max-runtime` only inherits into `RunJob`/`ServiceJob` at config-load time today (`cli/config.go:449,471`), so operators who want a sub-day ceiling on `ExecJob`/`LocalJob`/`ComposeJob` still need to set a per-job `max-runtime` (or wait for the inheritance to be widened in a follow-up). Previously `RunJob` was the only job type whose internal `startAndWait` re-wrapped with `context.WithTimeout(ctx, MaxRuntime)`; `ExecJob` and `RunServiceJob` could hang against a slow `docker exec`/`service create` with no upper bound, and `LocalJob`'s `ResolveJobEnvironment` call (env-from-container) had no timeout either (now bounded with a 10s `localJobEnvResolveTimeout`). The scheduler-level bound is applied once in `jobWrapper.runWithCtx` so all job types benefit uniformly. ([#638](https://github.com/netresearch/ofelia/issues/638), refs [#614](https://github.com/netresearch/ofelia/issues/614), [#636](https://github.com/netresearch/ofelia/pull/636))
- Docker API version negotiation at startup is now bounded by a configurable `NegotiateTimeout` (default 30s). Previously `NewClientWithConfig` called `NegotiateAPIVersion` with `context.Background()`, so a reachable-but-wedged Docker daemon (e.g. a socket proxy with a hung upstream) could hang Ofelia at startup with no diagnostic output. The deadline-exceeded path now logs a warning so operators can correlate startup slowness with daemon health ([#611](https://github.com/netresearch/ofelia/pull/611), fixes [#608](https://github.com/netresearch/ofelia/issues/608))
- Remaining unbounded Docker SDK calls are now wrapped in `context.WithTimeout`, so a reachable-but-wedged daemon can no longer stall the periodic `/health` and `/ready` checker (5s per call), the daemon startup sanity Pings in `NewDockerHandler` and `buildSDKProvider` (10s each, derived from the handler's own context so SIGINT during startup also cancels), or the `ofelia doctor` diagnostic (5s per Ping and per `HasImageLocally` call — per-call rather than overall to avoid falsely failing slow daemons with many images). `web/health.go` was the most visible regression because monitoring agents would never observe a non-2xx response when the daemon wedged. The three timeout values are unexported constants — see code comments for rationale; file an issue if your environment needs different bounds. Also adds `SDKDockerProviderConfig.NegotiateTimeout` to plumb the test-friendly negotiation bound from [#611](https://github.com/netresearch/ofelia/pull/611) one layer up. ([#636](https://github.com/netresearch/ofelia/pull/636), fixes [#614](https://github.com/netresearch/ofelia/issues/614))
- `[global]` section now recognizes the documented `webhook-*` keys (`webhook-allow-remote-presets`, `webhook-preset-cache-ttl`, `webhook-trusted-preset-sources`, `webhook-preset-cache-dir`, `webhook-allowed-hosts`, `webhook-webhooks`) without emitting "Unknown configuration key" warnings, and the values are now applied to the webhook subsystem. Live-reload also re-syncs into `WebhookConfigs.Global` so runtime edits to `webhook-allowed-hosts` take effect without a restart. **Upgrade note:** if you previously used the unprefixed forms (`allow-remote-presets`, `webhooks`, `preset-cache-ttl`, etc.) under `[global]` — they were never documented but were tolerated by the old hand-rolled parser — rename them to the documented `webhook-*` form. The old keys now produce "Unknown configuration key" warnings and the values silently fall back to defaults. ([#618](https://github.com/netresearch/ofelia/pull/618), fixes [#604](https://github.com/netresearch/ofelia/issues/604))
- `DOCKER_HOST=tcp://...` now correctly drives the HTTP transport's dialer when `ClientConfig.Host` is empty. Previously the dialer was hard-pinned to `unix:///var/run/docker.sock` while the SDK was directed at the env-supplied TCP host, so every request silently routed to a non-existent unix socket and surfaced as a misleading "Cannot connect to the Docker daemon at tcp://..." error. Most commonly hit with Docker socket proxies (e.g. tecnativa/docker-socket-proxy). The actual code change cascaded in via [#613](https://github.com/netresearch/ofelia/pull/613); this entry documents the original report and adds the troubleshooting recipe. ([#606](https://github.com/netresearch/ofelia/pull/606), fixes [#605](https://github.com/netresearch/ofelia/issues/605))
- `ExecServiceAdapter.Create` and `.Run` no longer panic on a nil `ExecConfig` or on nil `stdout`/`stderr` writers in non-TTY mode. Both paths now return typed sentinel errors (`ErrNilExecConfig`, `ErrNoExecOutputWriter`) that callers can branch on via `errors.Is`. Previously the SDK would dereference the nil config (`config.User`, etc.) or `stdcopy.StdCopy` would panic on `(nil, nil)` writers when there was output to demultiplex. ([#619](https://github.com/netresearch/ofelia/pull/619), refs [#610](https://github.com/netresearch/ofelia/issues/610))
- Defense-in-depth: every public method on every `*ServiceAdapter` in `core/adapters/docker/` (`Container`, `Exec`, `Image`, `Event`, `Network`, `Swarm`, `System`) now returns the new sentinel `ErrNilDockerClient` instead of panicking with a nil-pointer dereference if the embedded SDK client is nil. The `newClientFromSDK` constructor always wires a non-nil client, so this is only reachable through hand-rolled adapter values (test fixtures or wiring bugs) — but the guards convert what would otherwise be a panic in a hot goroutine into a branchable, actionable failure. `Subscribe` and `Wait` (channel-returning) push the sentinel to `errCh` and close both channels synchronously without launching a goroutine. ([#639](https://github.com/netresearch/ofelia/pull/639), fixes [#623](https://github.com/netresearch/ofelia/issues/623))
- The Docker label sync path now mirrors the `WebhookConfigs.Global → &Global.WebhookGlobalConfig` pointer alias that `NewConfig` set up for the live config in [#637](https://github.com/netresearch/ofelia/pull/637) / [#620](https://github.com/netresearch/ofelia/issues/620). The scratch `Config` built by `dockerContainersUpdate` and `mergeJobsFromDockerContainers` previously used a struct literal with `WebhookConfigs: NewWebhookConfigs()`, which left `parsed.WebhookConfigs.Global` pointing at a fresh `*WebhookGlobalConfig` disjoint from `parsed.Global.WebhookGlobalConfig`. Any future `mergeWebhookConfigs` field that reads from the parsed `WebhookConfigs.Global` (notably the `PresetCacheTTL` forwarding planned in [#640](https://github.com/netresearch/ofelia/pull/640)) would silently observe the 24h default instead of the just-decoded label value. Both call sites now go through a `newScratchConfig(c)` helper that re-establishes the alias. ([#641](https://github.com/netresearch/ofelia/issues/641))

### Tests

- Stabilize `TestHealthStatus` race against the `NewHealthChecker` background goroutine — build the `HealthChecker` directly in the test so the auto-injected `docker=Unhealthy` check cannot leak into the aggregated status before `GetHealth()` runs. ([#606](https://github.com/netresearch/ofelia/pull/606))
- New `TestConfigGlobalKeysAreDocumented` walks the embedded middleware structs in `Config.Global` via reflection and asserts each `mapstructure` key is mentioned in at least one operator-facing docs file (`docs/CONFIGURATION.md`, `docs/webhooks.md`, `docs/QUICK_REFERENCE.md`, `docs/TROUBLESHOOTING.md`, `README.md`). Catches the same drift class as #604 / #621 mechanically. ([#621](https://github.com/netresearch/ofelia/issues/621))
- Per-handler unit tests for the Docker scheme dispatch table (`TestSchemeHandlers_ApplyDirect`) invoke each `apply*` function directly with a fresh `*http.Transport` and assert the per-scheme `ForceAttemptHTTP2` / `DialContext` shape. Catches a refactor that breaks one scheme without breaking the others — previously the `apply*` functions were only covered transitively. New `TestCreateHTTPClient_UnknownSchemeFallback` pins the defensive plain-HTTP/1.1 fallback for unrecognized schemes (production rejects upstream via `NewClientWithConfig`; this exercises the seam, not the production gate). Tightened `TestNewClientWithConfig_ReadsDOCKERHOSTOnce` `env_only` branch from `<= 1` to `== 1` so a regression that drops the env read entirely is caught. ([#633](https://github.com/netresearch/ofelia/issues/633), follow-up to [#629](https://github.com/netresearch/ofelia/pull/629))

### Documentation

- Reconcile the `tcp://` Docker host scheme docs with reality: the godoc in `core/adapters/docker/client.go` (the `schemeHandlers` table) and the scheme table in `docs/TROUBLESHOOTING.md` previously claimed `tcp://` "auto-upgrades to TLS when `DOCKER_TLS_VERIFY` / `DOCKER_CERT_PATH` are set", mirroring the docker CLI. The transport-layer half of that upgrade was wired in [#613](https://github.com/netresearch/ofelia/pull/613), but Go's `http.Transport` only performs TLS on `https://` URLs — so the `TLSClientConfig` was loaded with cert material that the SDK never offered on the wire. Operators following the docker CLI mental model believed they had mTLS while their connections went out as plain TCP. The fix removes the `applyDockerTLS` call from `applyTCPTransport` (silent ineffective wiring is worse than failing loud), updates the godoc and the troubleshooting table to point operators at `tcp+tls://` ([#616](https://github.com/netresearch/ofelia/issues/616)) or `https://` for TLS over TCP, and replaces the misleading `TestCreateHTTPClient_TCPWithTLSEnvUpgrades` regression test with `TestCreateHTTPClient_TCPDoesNotWireTLSEvenWithEnv` to pin the contract going forward. The deeper docker-CLI parity story — automatic `tcp://` to `https://` URL rewriting at the SDK layer — is tracked separately in [#634](https://github.com/netresearch/ofelia/issues/634) and intentionally not addressed here. ([#628](https://github.com/netresearch/ofelia/issues/628))
- Reconcile **Slack** middleware key documentation with the actual struct fields in `middlewares.SlackConfig`. Removed the documented-but-rejected `slack-url` (typo for `slack-webhook`), `slack-channel`, `slack-mentions`, `slack-icon-emoji`, and `slack-username` keys from `docs/CONFIGURATION.md` and `docs/QUICK_REFERENCE.md`. The legacy Slack middleware (deprecated, scheduled for removal in v1.0.0) only accepts `slack-webhook` and `slack-only-on-error`; for channel routing, mentions, custom username/avatar, etc., migrate to a `[webhook "name"]` section with `preset = slack` ([webhook docs](docs/webhooks.md)). ([#621](https://github.com/netresearch/ofelia/issues/621))
- Reconcile **Save** middleware key documentation with the actual struct fields in `middlewares.SaveConfig`. Documented the existing `restore-history` and `restore-history-max-age` global keys (supported by the parser but undocumented) and removed the unimplemented `save-format` and `save-retention` keys from `docs/CONFIGURATION.md`. ([#621](https://github.com/netresearch/ofelia/issues/621))
- Document the previously-undocumented (or only partially-documented) `[global]` keys surfaced by the docs-vs-code drift sweep: `notification-cooldown` (notification deduplication window) and `smtp-tls-skip-verify` (with a dedicated security trade-off section in `docs/TROUBLESHOOTING.md` covering when it's acceptable, when it's not, and recommended alternatives) in `docs/CONFIGURATION.md`; and the webhook globals `webhook-webhooks`, `webhook-trusted-preset-sources`, `webhook-preset-cache-dir`, plus an explicit INI-vs-Docker-labels callout (only `webhook-webhooks` is exposed via labels; the SSRF-sensitive globals are INI-only) in `docs/webhooks.md`. ([#635](https://github.com/netresearch/ofelia/issues/635), refs [#621](https://github.com/netresearch/ofelia/issues/621), [#604](https://github.com/netresearch/ofelia/issues/604))

### Refactor

- Unify Docker host / scheme resolution in `core/adapters/docker/client.go` into a single `resolveDockerHost` seam. `NewClientWithConfig` and `createHTTPClient` now agree on the resolved host without re-reading `DOCKER_HOST`, eliminating the dual-reader anti-pattern that produced [#605](https://github.com/netresearch/ofelia/issues/605) / [#607](https://github.com/netresearch/ofelia/issues/607) / [#609](https://github.com/netresearch/ofelia/issues/609). The dispatch `switch` and the separate `supportedDockerHostSchemes` slice collapsed into a single `schemeHandlers` map (allow-list + dispatch derived from the same data); scheme spelling lives in named constants; `formatSupportedSchemes` is cached in a package var. `client.FromEnv` is dropped from the SDK options chain (host + TLS are mirrored explicitly; `DOCKER_API_VERSION` is preserved via `client.WithVersionFromEnv()`). New contract test asserts `DOCKER_HOST` is read at most once per `NewClientWithConfig` call; new parity test asserts the public allow-list cannot drift from the dispatch table. Pure refactor with one minor operator-visible side effect: the `unsupported DOCKER_HOST scheme` error now lists the supported schemes in alphabetical order (`http://, https://, npipe://, tcp://, unix://`) rather than the previous curated `unix, tcp, http, https, npipe` order — the new map-derived list sorts deterministically. ([#617](https://github.com/netresearch/ofelia/issues/617))

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
