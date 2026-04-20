# Plugin config

CVT reads plugin configuration from a single YAML file at `~/.cvt/config.yaml`. If the file doesn't exist, the plugin system stays dormant — no plugins run, no overhead.

## TL;DR — minimum viable config

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

This binds the `registry` plugin to `fetch_schema`. Run `cvt serve`; the plugin forks at startup, receives `base_url` and `token` via gRPC (not subprocess env), and handles every schema-by-ID resolution.

## Hook reference

Four hook points. Each binds to at most one plugin; an unset hook falls back to CVT's built-in behavior.

| Hook | Plugin service | Fires when | Status |
|---|---|---|---|
| `fetch_schema` | `RegistryProvider` | Schema-by-ID resolution, on cache miss, before storage fallback | **wired** |
| `register_consumer_usage` | `RegistryProvider` | `RegisterConsumer` succeeds | **wired** |
| `on_breaking_change_detected` | `EventHandler` | `CompareSchemas`, or `RegisterSchema` when `--check-compatibility` detects breaks | **wired** |
| `on_validation_failed` | `EventHandler` | `ValidateInteraction` returns a non-valid result | **wired** |

Binding a hook to a plugin that isn't declared under `plugins:` is a load-time error.

## Recovering from a broken config

```sh
CVT_DISABLE_PLUGINS=1 cvt serve
```

Safe mode ignores `~/.cvt/config.yaml` entirely. Use it if a misconfigured plugin prevents CVT from starting. Then edit the config or `cvt plugins remove <name>` and restart.

---

<details>
<summary>Full config schema</summary>

```yaml
config_version: 1                # required; unknown versions rejected

plugins:
  <name>:                         # name matches ^[a-z][a-z0-9-]{0,31}$
    binary: <path>                # required; must be under ~/.cvt/plugins/
    timeout: 5s                   # per-call gRPC deadline; default 5s
    on_error: fail_closed         # fail_closed (default) | fail_open
    secrets: [key1, key2]         # keys redacted + delivered via SetConfig
    config:                       # free-form map, delivered via SetConfig
      key: value
      token: ${ENV_VAR}           # ${VAR} interpolation
      base_url: ${OVERRIDE:-https://default.example.com}  # with default

hooks:
  fetch_schema: <plugin-name>                 # optional
  register_consumer_usage: <plugin-name>      # optional
  on_breaking_change_detected: <plugin-name>  # optional
  on_validation_failed: <plugin-name>         # optional
```

</details>

<details>
<summary>Field reference</summary>

### `config_version`

Schema version. Current release accepts `1`. Unknown versions fail at load time with an upgrade hint. Bumps only on incompatible config-shape changes.

### `plugins.<name>`

Map key = plugin's CVT-visible name (not the plugin's self-reported name, which is recorded separately as `reported_version`). Must match `^[a-z][a-z0-9-]{0,31}$`: lowercase letters, digits, hyphens, starting with a letter, 1–32 chars. Invalid name is a load-time error.

### `binary`

Absolute path to the plugin binary. Must be under `~/.cvt/plugins/`; leading `~/` is expanded. Any other location is rejected — primary defense against a config pointing at an arbitrary binary.

### `timeout`

Per-call gRPC deadline for every plugin RPC. Default `5s`. Exceeded calls return `DeadlineExceeded`; the hook adapter applies `on_error` and records `outcome=timeout` in audit.

### `on_error`

- `fail_closed` (default): plugin errors propagate to the caller. Use for plugins the caller depends on (a primary schema registry).
- `fail_open`: plugin errors are logged and audited but swallowed. Use for best-effort sinks — a Slack notifier shouldn't fail CI.

### `secrets`

List of keys in `config:` treated as secret. Secret keys are:

- Delivered via `SetConfig` gRPC, **never** via subprocess env (env is readable from `/proc/<pid>/environ`).
- Redacted from CVT logs and audit entries.
- Required to have a value after env interpolation. A secret in `secrets:` missing from `config:` is a load-time error.

### `config`

Free-form string-to-string map, delivered to the plugin via `SetConfig` at startup, before any extension-point RPC runs. POSIX-style env interpolation applies (see below).

### `hooks`

Maps each hook name to a declared plugin. Unset hook = built-in CVT behavior. See the hook reference table above.

</details>

<details>
<summary>Environment-variable interpolation</summary>

Two POSIX-style forms, applied to `config:` values after YAML parse, before secret delivery:

- `${VAR}` — substitute `$VAR`. Unset `$VAR` = load-time error.
- `${VAR:-default}` — substitute `$VAR`; use `default` if unset or empty.

Unterminated `${` is a load-time error. Nested expressions are not supported.

</details>

<details>
<summary>Config precedence and per-project config</summary>

v1 loads exactly one file: `~/.cvt/config.yaml`. There is no project-level override.

If you need per-project config today, two workarounds:
- Check the config into your repo and symlink at setup.
- Use a wrapper that sets `HOME` to a project-specific directory before invoking `cvt`.

Project overrides are tracked for v1.1+.

</details>
