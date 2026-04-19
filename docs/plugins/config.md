# Plugin config

CVT reads plugin configuration from a single YAML file at
`~/.cvt/config.yaml`. If the file doesn't exist, the plugin system stays
dormant (no plugins run, no overhead).

## Minimal example

```yaml
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
```

Run CVT; the `registry` plugin forks at startup. All four hooks fire
from core today — see the hook status table in `README.md`.

## Full schema

```yaml
config_version: 1                # required; rejects unknown versions

plugins:
  <name>:                         # name matches ^[a-z][a-z0-9-]{0,31}$
    binary: <path>                # required; must be under ~/.cvt/plugins/
    timeout: 5s                   # per-call gRPC deadline; default 5s
    on_error: fail_closed         # fail_closed (default) or fail_open
    secrets: [key1, key2]         # keys in config: that should be
                                  # redacted and delivered via SetConfig
    config:                       # free-form map delivered to the plugin
      key: value                  # via SetConfig on startup
      token: ${ENV_VAR}           # ${VAR} interpolation
      base_url: ${OVERRIDE:-https://default.example.com}  # with default

hooks:
  fetch_schema: <plugin-name>                 # optional
  register_consumer_usage: <plugin-name>      # optional
  on_breaking_change_detected: <plugin-name>  # optional
  on_validation_failed: <plugin-name>         # optional
```

## Field reference

### `config_version`

Schema version. CVT accepts `1` in this release. Unknown versions fail
at load time with an upgrade hint. Bump only when the config shape
changes incompatibly.

### `plugins.<name>`

The map key is the plugin's CVT-visible name (not the plugin's
self-reported name — that's recorded separately as `reported_version`).
Plugin names must match `^[a-z][a-z0-9-]{0,31}$`: start with a lowercase
letter, contain only lowercase letters, digits, and hyphens, 1 to 32
chars. Invalid names are load-time errors.

### `binary`

Absolute path to the plugin binary. Must be under `~/.cvt/plugins/`. A
leading `~/` is expanded. Any other location is rejected as a load-time
error; this is the primary defense against a config file pointing at an
arbitrary binary.

### `timeout`

Per-call gRPC deadline applied to every plugin RPC. Default `5s`.
Plugin calls that exceed the deadline return `DeadlineExceeded`; the
hook adapter applies `on_error` and records the call as `outcome=timeout`
in audit.

### `on_error`

Either `fail_closed` (default) or `fail_open`.

- `fail_closed`: plugin errors propagate to the caller. Use for plugins
  the caller depends on (e.g., the primary schema registry).
- `fail_open`: plugin errors are logged + audited but swallowed. Use for
  truly-best-effort plugins (e.g., a Slack notifier — if Slack is down,
  you don't want CI to fail).

### `secrets`

List of keys in `config:` that should be treated as secret. Secret keys
are:

- Delivered to the plugin via the `SetConfig` gRPC call, NOT via
  subprocess environment variables (env is readable from
  `/proc/<pid>/environ`).
- Redacted from CVT logs and audit entries.
- Required to have a value after env interpolation. A secret declared
  in `secrets:` but missing from `config:` is a load-time error.

### `config`

Free-form string-to-string map. CVT delivers every entry to the plugin
via `SetConfig` at startup, before any extension-point RPC runs.
Environment-variable interpolation (`${VAR}`, `${VAR:-default}`) applies
to values. Unset `${VAR}` without a default fails load-time.

### `hooks`

Maps each of the four v1 hook points to exactly one plugin name. An
unset hook means CVT falls back to its built-in behavior (no plugin
runs for that hook).

| Hook | When it fires | Plugin service | Status |
|---|---|---|---|
| `on_validation_failed` | After `ValidateInteraction` returns a non-valid result | `EventHandler` | **wired** |
| `on_breaking_change_detected` | After `CompareSchemas`, or after `RegisterSchema` when `check_compatibility=true` and breaking changes are detected | `EventHandler` | **wired** |
| `register_consumer_usage` | After `RegisterConsumer` succeeds | `RegistryProvider` | **wired** |
| `fetch_schema` | Before schema-by-ID resolution (on cache miss, before storage) | `RegistryProvider` | **wired** |

A hook referencing a plugin that isn't declared under `plugins:` is a
load-time error.

## Environment variable interpolation

Values under `config:` support two POSIX-style expressions:

- `${VAR}` — substitute the value of `$VAR`. Unset = load-time error.
- `${VAR:-default}` — substitute `$VAR`; if unset or empty, use `default`.

Interpolation runs after YAML parse, before secret delivery. Nested
expressions are not supported.

Unterminated `${` sequences are load-time errors.

## Safe mode

Set `CVT_DISABLE_PLUGINS=1` in the environment and CVT behaves as if
no plugins were configured, regardless of what the file says. Use this
to recover from a broken plugin that prevents `cvt serve` from
starting:

```sh
CVT_DISABLE_PLUGINS=1 cvt serve
```

## Config precedence

v1 loads exactly one file: `~/.cvt/config.yaml`. There is no project-level
override in this release. Teams that want per-project config check the
file into their repo and symlink it at setup, or use a wrapper that
points `HOME` at a project-specific directory.

## Recovering from a stuck plugin

1. `CVT_DISABLE_PLUGINS=1 cvt serve` — bring CVT back up without plugins.
2. Investigate via `cvt plugins list`, logs, and `/metrics`.
3. `cvt plugins remove <name>` if the plugin should go.
4. Edit config; restart CVT.

There is no live hot reload or admin IPC in v1. Restart is required for
every config change.
