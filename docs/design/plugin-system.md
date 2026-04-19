# CVT Plugin System — v1 Design

## Context

CVT has no runtime extension mechanism. Teams wanting custom schema registries (e.g., internal Central API Registry) or event sinks (Slack, Jira, webhooks) must fork CVT. Issue #83 and the Phase 1a Enterprise Deployment work in `TODOS.md` both plan in-tree Go interfaces for this. Instead: build one small plugin framework and rewrite those tracks to consume it. Plugins live in separate repos, are installed as subprocess binaries, and communicate over gRPC via `hashicorp/go-plugin`.

## Goals

1. Third parties extend CVT without forking. Plugin code lives in its own repo.
2. Two extension points in v1: **registry providers** (fetch schema, register consumer usage) and **event handlers** (breaking change, validation failure).
3. Works in CLI and `cvt serve`.

That is the whole v1. Everything else is v1.1+.

## Non-goals (v1)

- Pipeline stages, strategies, fanout, fallback ordering. **One plugin per hook.**
- Circuit breaker layered on top of `go-plugin` restart.
- Project-vs-global config merge.
- Hot reload, `cvt plugins inspect`, `cvt plugins verify`, `cvt plugins reset`.
- Python/Node/Java plugin SDKs.
- GitHub-URL install, signed tags, Cosign.
- Windows support (Linux + macOS v1; Windows v1.1).
- Custom `Validator` extension point.
- Storage-as-plugin, command-plugin, auth-plugin.

## Architecture

```
  CVT core (binary)
    │
    │  hashicorp/go-plugin (fork + gRPC over unix socket)
    ▼
  [cvt-plugin-rest]              [cvt-plugin-slack]
       (subprocess)                  (subprocess)
```

Core forks each configured plugin at startup, speaks gRPC to it for the life of the process, tears down on shutdown. `go-plugin` handles handshake, transport, restart-on-crash, and `hclog` log forwarding natively.

`pkg/cvt` defines a `Hooks` interface. `internal/pluginclient` implements it by calling the configured plugin. `cmd/cvt` and `server/cvtservice` inject the implementation. `pkg/cvt` never imports `internal/*` (Go enforces).

## Extension points

Two services, two protos.

### `RegistryProvider`

```proto
service RegistryProvider {
  rpc FetchSchema(FetchSchemaRequest) returns (FetchSchemaResponse);
  rpc RegisterConsumerUsage(RegisterConsumerUsageRequest) returns (RegisterConsumerUsageResponse);
}
```

`RegisterConsumerUsage` MUST be idempotent upsert.

### `EventHandler`

Typed per-event RPCs (not `google.protobuf.Struct`). New events = new RPC methods. Old plugins return `Unimplemented` for unknown events; core treats that as a non-error no-op.

```proto
service EventHandler {
  rpc OnBreakingChangeDetected(BreakingChangeDetectedRequest) returns (EventResponse);
  rpc OnValidationFailed(ValidationFailedRequest) returns (EventResponse);
}
```

Event handlers rate-limit and dedup on their side. Core fires every event.

### Common handshake service

```proto
service PluginHandshake {
  rpc Info(InfoRequest) returns (InfoResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
  rpc SetConfig(SetConfigRequest) returns (SetConfigResponse);  // secrets delivered here
}

message InfoResponse {
  string name = 1;
  string version = 2;
  repeated string services = 3;        // e.g., ["registry.v1", "events.v1"]
  uint32 protocol_version = 4;
}
```

Handshake itself is `go-plugin`'s native `HandshakeConfig{MagicCookieKey, MagicCookieValue}`. Post-connection, core calls `PluginHandshake.Info` with a 5s total deadline. Mismatched `protocol_version` rejects the plugin. Log forwarding is `go-plugin`'s native `hclog` path; no custom log RPC.

## Hooks

Four hook points wired via `pkg/cvt.Hooks` (server) and `pkg/cvt.Validator.SetHooks` (CLI):

| Hook | Fires in | Maps to plugin call |
|---|---|---|
| `fetch_schema` | `server/cvtservice/validator_service.go` in `getSchemaEntry` — between cache miss and storage lookup, so a bound plugin is authoritative | `RegistryProvider.FetchSchema` |
| `register_consumer_usage` | `server/cvtservice/consumer_registry.go` at `RegisterConsumer` success | `RegistryProvider.RegisterConsumerUsage` |
| `on_breaking_change_detected` | `server/cvtservice/validator_service.go` at success returns of `CompareSchemas` and `RegisterSchema --check-compatibility` | `EventHandler.OnBreakingChangeDetected` |
| `on_validation_failed` | `pkg/cvt/validator.go` after `Validate` returns a non-valid result (CLI path) | `EventHandler.OnValidationFailed` |

Hook fire-sites stay in `validator_service.go` (core Phase 1 methods) and `consumer_registry.go` (Phase 2 consumer lifecycle) after the file split (#107). Other post-split files (`producer_validation.go`, `deployment_safety.go`, `fixture_generator.go`) do NOT host hook fire-sites in v1.

`register_consumer_usage` carries the consumer's `used_endpoints` end-to-end including `used_fields` (added in plugin proto v1.1 alongside the hook wiring). Plugins that ignore the field stay backward-compatible with v1.0.

`on_breaking_change_detected` fires only when the comparison surfaced non-empty changes. Empty-changes calls are silently dropped at the helper layer; downstream call sites do not duplicate the guard.

The `RegisterSchema --check-compatibility` flag is server-side enforced as of issue #107 (was a silent no-op in #108): the server looks up the prior version, compares, populates `RegisterSchemaResponse.BreakingChanges`, and fires the hook. Storage errors during prior-version lookup are fail-closed — the registration is refused with a clear error so the safety check never silently passes (decision 1C, eng review 2026-04-18).

Each hook invokes **at most one plugin**, chosen by config key. Fanout = v1.1.

**Status:** All four v1 hooks are wired. `on_validation_failed` (CLI), `on_breaking_change_detected`, and `register_consumer_usage` landed with issue #107; `fetch_schema` fires from `getSchemaEntry` on cache miss before storage fallback.

## Config

One file: `~/.cvt/config.yaml`. No project override in v1.

```yaml
config_version: 1

plugins:
  registry:
    binary: ~/.cvt/plugins/cvt-plugin-rest
    timeout: 5s
    on_error: fail_closed
    secrets: [token]
    config:
      base_url: ${CVT_REGISTRY_URL:-https://registry.example.com/api/v1}
      token: ${CVT_REGISTRY_TOKEN}

  slack:
    binary: ~/.cvt/plugins/cvt-plugin-slack
    timeout: 3s
    on_error: fail_closed
    secrets: [webhook_url]
    config:
      webhook_url: ${SLACK_WEBHOOK}

hooks:
  fetch_schema: registry
  register_consumer_usage: registry
  on_breaking_change_detected: slack
  on_validation_failed: slack
```

Rules:
- Plugin name matches `^[a-z][a-z0-9-]{0,31}$`.
- `binary` path must be under `~/.cvt/plugins/`.
- `secrets` keys delivered to plugin via `SetConfig` gRPC, not subprocess env. Redacted in logs + audit.
- `${VAR}` env interpolation after load. `${VAR:-default}` supported. Unset secret without default = fail-closed at load time.
- `CVT_DISABLE_PLUGINS=1` forces empty plugin set regardless of config (safe mode).

## Lifecycle + failure semantics

| Behavior | Default | Configurable |
|---|---|---|
| Plugin fork | at core startup via `go-plugin` | n/a |
| Handshake | `go-plugin` native + `PluginHandshake.Info`; 5s total deadline | n/a |
| Restart on crash | enabled (go-plugin default) | `plugins.<name>.restart` |
| Restart backoff | exponential 1s→30s (go-plugin default) | n/a |
| Health check | gRPC `Health` every 10s | n/a |
| Per-call timeout | 5s | `plugins.<name>.timeout` |
| Per-plugin error policy | `fail_closed` | `plugins.<name>.on_error: fail_closed \| fail_open` |
| Graceful shutdown | SIGTERM, 30s grace, SIGKILL | n/a |

Every plugin call runs under a context deadline. On timeout, panic, or transport error: plugin call returns error, hook applies `on_error`. `fail_closed` surfaces error to the caller; `fail_open` treats error as no-op (for truly-best-effort hooks like a Slack notification).

## Security & trust model

**Trust boundary = install step.** Plugins run with full CVT process permissions. CVT does not sandbox plugins. `cvt plugins install` is a privileged action; operators vet plugin binaries.

**What CVT defends:** protocol mismatch (rejected at handshake), binary-path escape (must be under `~/.cvt/plugins/`), install-time SHA256 (`cvt plugins install` records hash; re-verified at each fork), secrets delivered via gRPC not env.

**What CVT does not defend:** malicious plugins reading arbitrary files or making arbitrary outbound network calls. Document clearly.

## Observability

**Logs:** structured via `go-plugin`'s native `hclog` forwarding. Core's `pkg/cvtplugin/logger.go` wires a Zap-backed `hclog.Logger` into `plugin.ClientConfig`. Plugin authors use plain `hclog.Logger`; entries reach CVT logs with fields `plugin`, `version`, `sha256` (first 12 chars), `pid`.

**Metrics** (Prometheus, existing `CVT_METRICS_PORT`):
- `cvt_plugin_call_duration_seconds{plugin, service, method}` histogram.
- `cvt_plugin_call_errors_total{plugin, service, method, code}` counter. `code` = canonical gRPC code name.
- `cvt_plugin_up{plugin, version}` gauge.
- `cvt_plugin_restarts_total{plugin}` counter.

**Audit:** `server/cvtservice/audit_logger.go` extended with plugin-call entries. Fields: `plugin`, `version`, `sha256`, `pid`, `request_id`, `service`, `method`, `duration_ms`, `outcome`, `error_code`. Secret config values redacted at emission. Writes are synchronous (small channel, block if full). Perf split deferred to v1.1 if it matters.

## `cvt plugins` CLI

Three subcommands:

- `cvt plugins list` — read `~/.cvt/plugins/state.json`, print installed plugins with `sha256` prefix + install time.
- `cvt plugins install <path>` — verify binary exists + is executable, copy into `~/.cvt/plugins/` (if outside), compute SHA256, append to `state.json` atomically.
- `cvt plugins remove <name>` — delete binary, update `state.json`. Errors if `cvt serve` is running on the local machine (checked via default socket/port); operator restarts serve afterward.

No `inspect`, no `verify`, no `reset` in v1.

**`state.json`** (install-time metadata only):

```json
{
  "version": 1,
  "plugins": {
    "registry": {
      "binary_path": "/Users/.../.cvt/plugins/cvt-plugin-rest",
      "sha256": "abc123...",
      "installed_at": "2026-04-17T19:00:00Z"
    }
  }
}
```

Runtime state (pid, up/down, restart count) never written to disk. Operators read it from `/metrics`.

## Code layout

```
api/protos/plugin/
├── handshake.proto
├── registry/v1/registry.proto
└── events/v1/events.proto

pkg/cvt/
├── hooks.go                   # Hooks interface (no impl)
└── validator.go               # two hook call sites

pkg/cvtplugin/                 # public SDK for plugin authors
├── doc.go
├── serve.go                   # wraps plugin.Serve with shared HandshakeConfig
├── registry.go                # Go interface for RegistryProvider authors
├── events.go                  # Go interface for EventHandler authors
├── logger.go                  # Zap-backed hclog adapter
└── plugintest/harness.go      # in-process test harness

internal/pluginclient/         # core-side typed adapters that implement pkg/cvt.Hooks
internal/pluginmgr/            # lifecycle via go-plugin, config load, audit wiring

cmd/cvt/plugins.go             # list/install/remove
server/cvtservice/             # post-split: consumer_registry.go, producer_validation.go host the two server-side hook call sites
```

## Documentation

Four files, shipped with v1:

- `docs/plugins/README.md` — overview, trust boundary, when to use plugins.
- `docs/plugins/config.md` — config schema, env interpolation, secrets, safe mode.
- `docs/plugins/authoring-go.md` — write a plugin using `pkg/cvtplugin`; handshake handled by SDK; use `hclog.Logger`; honor `ctx.Done()`.
- `docs/plugins/reference-plugins.md` — walkthrough of `cvt-plugin-rest` and `cvt-plugin-slack`.

Proto docs generated from the three `.proto` files.

## Reference plugins

Two, each in its own repo:

- `github.com/sahina/cvt-plugin-rest` — implements `RegistryProvider` over REST. Covers issue #83.
- `github.com/sahina/cvt-plugin-slack` — implements `EventHandler` for breaking-change + validation-failed → Slack webhook. Covers the notifications TODO.

A GitHub-backed registry plugin (`cvt-plugin-registry-github`) is tracked in `TODOS.md` but is NOT a v1 deliverable — someone needs it first.

## Verification

10 checks before merge:

1. **Framework unit tests** — `go test ./internal/pluginmgr/... -v`. Config load + validation, `on_error` behavior, timeout enforcement, safe-mode env override, plugin-name regex, binary-path policy, secret delivery via `SetConfig`.
2. **SDK unit tests** — `go test ./pkg/cvtplugin/... -v`. `Serve` round-trip, `plugintest` harness, Zap↔hclog adapter emits structured records.
3. **Real-subprocess integration test** — `go test -tags=integration ./internal/pluginmgr/... -v`. Fork a test plugin, complete handshake + `Info`, call both services, observe `go-plugin` restart on plugin crash, zombie reap after CVT SIGKILL.
4. **Handshake deadline** — plugin that never emits go-plugin handshake is killed at 5s; plugin whose `Info.protocol_version` doesn't match is rejected cleanly.
5. **Reference plugin end-to-end** — build `cvt-plugin-rest` in its repo, install, run `cvt validate --schema test-api` against a mock registry, assert schema fetched + usage registered.
6. **Metrics** — start `cvt serve` with plugins configured, trigger calls, scrape `localhost:9551/metrics`, confirm all four `cvt_plugin_*` series present.
7. **Audit** — trigger a write call, confirm audit entry with `plugin`, `sha256`, `outcome`. Confirm `${CVT_REGISTRY_TOKEN}` does NOT appear in any log or audit entry.
8. **Config** — `${VAR}` + `${VAR:-default}` interpolation; unset required secret = load-time fail-closed; bad plugin name = load-time error; binary path outside `~/.cvt/plugins/` = load-time error.
9. **`cvt plugins` commands** — `install` verifies + records sha256 in `state.json`; `list` prints installed plugins; `remove` errors if `cvt serve` running; safe mode `CVT_DISABLE_PLUGINS=1` forces empty plugin set.
10. **Docs build** — docs-site builds with new plugin section; proto reference renders.

Proto discipline: new field = new number, new method = additive, never mutate published protos. Breaking changes = bump to `v2` package. No hand-rolled CI check in v1; run `buf breaking` manually when editing protos. Automated proto-freeze CI = v1.1.

## Follow-up: Phase 1a + issue #83 reconciliation

Post-merge follow-ups:

- Issue #83: remove the `SchemaRegistryProvider` in-tree interface proposal; rewrite against `cvt.plugin.registry.v1.RegistryProvider`; REST provider code moves to `cvt-plugin-rest` repo.
- Phase 1a: `SchemaProvider` + `HTTPProvider` + `GitHubProvider` in-tree interfaces drop; `HTTPProvider` ships as `cvt-plugin-rest`, `GitHubProvider` becomes a TODO-tracked plugin.
- Closed TODOs (superseded): P2 "Notification system design document", P2 "GitHubProvider dependency investigation", P2 "DRY schema URL fetching (3 duplicate implementations)".

## Parallelization

```
Prereq: P2 god-file split of validator_service.go lands on main.
   │
   ▼
PR#1: protos + pkg/cvtplugin SDK                  (freezes proto at merge)
   │
   ▼
PR#2, PR#3 in parallel:
   PR#2: reference plugins in their own repos     (consume frozen proto)
   PR#3: internal/pluginmgr + internal/pluginclient + cvt plugins CLI + hook call sites in post-split files + audit wiring
   │
   ▼
PR#4: docs + plugin-authoring guide
   │
   ▼
Integration PR: end-to-end with reference plugin installed; all 10 verification checks pass
```

---

## GSTACK REVIEW REPORT

| Review | Runs | Status | Findings |
|---|---|---|---|
| CEO | 1 | CLEAR | HOLD_SCOPE + user picked Approach A; all amendments applied then simplified |
| Codex (outside voice) | 2 | CLEAR | 1st pass via Claude subagent fallback (codex quota); 2nd pass external reviewer flagged 5 NIH/logic issues, all applied; user then explicitly requested simplification pass |
| Eng | 1 | CLEAR | 12 issues applied; simplified out of v1: pipeline stages, circuit breaker, sync-vs-async audit split, identity-key merge protection, inspect/verify/reset, per-event lifecycle events, alert rules, multi-plugin fanout tests |
| Design | 0 | N/A | no UI scope |
| DX | 0 | — | — |

**Simplification pass** (post-reviews, user-directed): collapsed the plan from 776 → ~250 lines by removing everything not needed to deliver issue #83 + notifications. Deferred to v1.1: pipeline stages/strategies/fanout, circuit breaker (rely on `go-plugin` restart), project-vs-global config merge, `cvt plugins inspect/verify/reset`, structured lifecycle events, alert rules, Windows support, multi-plugin write pipelines, `merge` strategy. Each deferred item is a TODO.md entry or naturally re-emerges when a user asks.

**VERDICT:** ready to implement, gated on P2 god-file split landing first.
