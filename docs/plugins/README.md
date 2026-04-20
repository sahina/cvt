# CVT Plugins

CVT plugins extend the server without forking it. A plugin is a separate binary that implements one or more CVT plugin services (registry provider, event handler) and talks to `cvt` over gRPC via a Unix domain socket. Plugins live in their own repositories on their own release schedules.

## TL;DR

```sh
# 1. install a plugin binary
cvt plugins install ./cvt-plugin-rest

# 2. declare it in ~/.cvt/config.yaml
cat > ~/.cvt/config.yaml <<'EOF'
config_version: 1
plugins:
  registry:
    binary: ~/.cvt/plugins/cvt-plugin-rest
    on_error: fail_closed
    secrets: [token]
    config:
      base_url: https://registry.example.com/api/v1
      token: ${CVT_REGISTRY_TOKEN}
hooks:
  fetch_schema: registry
  register_consumer_usage: registry
EOF

# 3. run CVT — plugin forks at startup and is ready for bound hooks
cvt serve
```

`cvt plugins list` confirms the install; plugin logs show up in CVT's output with a `plugin=registry` field. Broken plugin? `CVT_DISABLE_PLUGINS=1 cvt serve` bypasses the plugin system entirely.

## Pick your path

- **Installing and running plugins?** Read [Config reference](config.md) and the [Reference plugins](reference-plugins.md) for turn-key installs.
- **Writing a plugin?** [Authoring guide (Go)](authoring-go.md). Go is the only SDK in v1.
- **Operating CVT with plugins in prod?** Trust model, metrics, and recovery are all covered below.

## Hook status

All four v1 hook call-sites are wired today. A plugin bound to any of them gets called from core.

| Hook | Plugin service | Fires when |
|---|---|---|
| `fetch_schema` | `RegistryProvider` | Schema cache miss, before storage fallback, on every RPC that resolves a schema by ID |
| `register_consumer_usage` | `RegistryProvider` | `RegisterConsumer` succeeds |
| `on_breaking_change_detected` | `EventHandler` | `CompareSchemas` returns breaking changes, or `RegisterSchema --check-compatibility` detects them |
| `on_validation_failed` | `EventHandler` | `pkg/cvt.Validator.Validate` returns a non-valid result (CLI/library path) |

Plugins can be written, installed, and configured against all four hook names today. SDK, proto contracts, and config schema are frozen.

## When to use a plugin

- **Custom schema registry.** Your org hosts OpenAPI specs in a registry with its own API (internal Central API Registry, Backstage, Apicurio). Write a `RegistryProvider` plugin and bind `fetch_schema` + `register_consumer_usage`. CVT's built-in loader only handles files and raw URLs; a bound plugin is authoritative.
- **Event integrations.** Post breaking-change or validation-failure events to Slack, Jira, a webhook, your incident tool. Write an `EventHandler`.

**Don't** write a plugin if you just want to patch CVT for your org — fork or vendor. Don't use a plugin to swap storage backends — that's in-tree in `server/storage/factory.go`.

## Verifying your plugin is running

Four signals, any of which confirms the plugin is loaded and being called:

```sh
# 1. state.json knows about it
cvt plugins list
# NAME      SHA256        INSTALLED                  BINARY
# registry  abc123def456  2026-04-20 12:35:00 EDT   /Users/you/.cvt/plugins/cvt-plugin-rest

# 2. CVT logs show the plugin forked and handshake completed
# (sample line — your log format may differ)
# INFO plugin registered plugin=registry version=0.1.0 protocol_version=1 pid=12345

# 3. /metrics exposes per-plugin counters
curl -s http://localhost:9551/metrics | grep cvt_plugin_up
# cvt_plugin_up{plugin="registry",version="0.1.0"} 1

# 4. trigger the bound hook (e.g., `cvt validate --schema some-id`) and watch
#    cvt_plugin_call_duration_seconds_count increment for your plugin
curl -s http://localhost:9551/metrics | grep cvt_plugin_call_duration_seconds_count
```

Four plugin metrics are always emitted: `cvt_plugin_call_duration_seconds`, `cvt_plugin_call_errors_total`, `cvt_plugin_up`, `cvt_plugin_restarts_total`.

## Trust boundary

**`cvt plugins install` is a privileged action.** A plugin runs with the same permissions as the CVT process: reads any file the user can, makes any outbound network call, sees any environment variable. CVT does not sandbox plugins.

Treat plugin installation like installing any other binary on your system: vet the source, pin versions, verify SHA256 against what the author publishes. CVT records the install-time SHA256 in `~/.cvt/plugins/state.json`; `cvt plugins list` surfaces the first 12 chars so you can spot-check.

## Recovering from a broken plugin

1. `CVT_DISABLE_PLUGINS=1 cvt serve` — bring CVT back up, no plugins. Safe mode ignores `~/.cvt/config.yaml` entirely.
2. Investigate via `cvt plugins list`, logs, and `/metrics`.
3. `cvt plugins remove <name>` if the plugin should go (requires CVT not running).
4. Fix config, restart CVT.

There is no hot reload in v1. Every config change requires a CVT restart.

## Architecture

```
  CVT (cvt serve or cvt validate)
    │
    │  hashicorp/go-plugin: fork + gRPC over Unix socket
    ▼
  [cvt-plugin-rest]     [cvt-plugin-slack]
    (subprocess)          (subprocess)
```

Plugin manager forks each configured plugin once at CVT startup, completes the `go-plugin` native handshake, calls `PluginHandshake.Info` to negotiate the protocol version, delivers configured values (secrets included) via `SetConfig`, and keeps the typed gRPC clients alive for the life of the CVT process. On shutdown: SIGTERM, 30-second grace, SIGKILL.

## Reference plugins

Two first-party plugins ship in separate repos:

- **[cvt-plugin-rest](https://github.com/sahina/cvt-plugin-rest)** — `RegistryProvider` backed by any REST schema registry. Covers [issue #83](https://github.com/sahina/cvt/issues/83).
- **[cvt-plugin-slack](https://github.com/sahina/cvt-plugin-slack)** — `EventHandler` that posts breaking-change and validation-failure events to a Slack webhook, with plugin-side dedup.

See [reference-plugins.md](reference-plugins.md) for decision table + walkthroughs.

## Scope

**In v1:**
- Two extension points: `RegistryProvider` and `EventHandler`.
- One plugin per hook (no fanout).
- Go SDK only.
- Linux + macOS.
- `cvt plugins {list, install, remove}`.
- Four plugin metrics + synchronous audit.

<details>
<summary>Deferred to v1.1+</summary>

- Multi-plugin fanout per hook, pipeline stages, `race`/`first_success` strategies.
- Custom `Validator` extension point.
- Hot reload, circuit-breaker admin (`cvt plugins reset`), `cvt plugins inspect/verify`.
- Python / Node / Java plugin SDKs.
- Windows support.
- GitHub-URL install, signed plugin tags.

Full scope reasoning: [design/plugin-system.md](../design/plugin-system.md).

</details>
