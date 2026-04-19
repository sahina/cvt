# CVT Plugins

CVT has a plugin system for teams that need to extend CVT without forking
it. A plugin is a separate binary that implements one or more CVT plugin
services (registry provider, event handler) and communicates with `cvt`
over gRPC on a Unix domain socket. Plugins live in their own repositories
on their own release schedules.

## Hook call-site status

Three of the four v1 hook call sites are wired today:

| Hook | Plugin service | Status |
|---|---|---|
| `on_validation_failed` | `EventHandler` | **wired** — fires from `ValidateInteraction` after a non-valid result |
| `on_breaking_change_detected` | `EventHandler` | **wired** — fires from `CompareSchemas` and from `RegisterSchema` when `--check-compatibility` is set |
| `register_consumer_usage` | `RegistryProvider` | **wired** — fires from `RegisterConsumer` on success |
| `fetch_schema` | `RegistryProvider` | SDK + config surface only; core call site deferred pending the schema-by-ID resolution path from [issue #83](https://github.com/sahina/cvt/issues/83) |

Plugins can be written, installed, and configured against all four hook
names today. The SDK, proto contracts, and config schema are frozen.
The unwired `fetch_schema` binding is a declarative no-op until its
call site lands.

## When to use a plugin

Use a plugin when you want:

- **Custom schema registry.** CVT's built-in schema loading handles files
  and raw URLs. If your organization hosts schemas in a registry with its
  own API (internal Central API Registry, Backstage, Apicurio), write a
  `RegistryProvider` plugin and point `fetch_schema` / `register_consumer_usage`
  at it. `register_consumer_usage` fires on every `RegisterConsumer`
  today; `fetch_schema` is the one hook still awaiting its call site.
- **Event integrations.** CVT fires events on breaking-change detection
  and validation failure. Write an `EventHandler` plugin that reacts to
  those events. Both `on_validation_failed` and `on_breaking_change_detected`
  fire from core today.

Don't write a plugin when:

- You just want to customize a single CVT build for your org — fork or
  vendor instead.
- You need to swap the storage backend — that's in-tree via
  `server/storage/factory.go`, not a plugin.

## Trust boundary

**`cvt plugins install` is a privileged action.** Once installed, a plugin
runs with the same permissions as the CVT process: it can read any file
the user can, make any outbound network call, and use any environment
variable. CVT does not sandbox plugins; it does not defend against
malicious plugin code.

Treat plugin installation like installing any other binary on your
system. Vet the source. Pin versions. Review SHA256 hashes against what
the plugin author publishes.

## Quick tour

1. **Install a plugin:**
   ```sh
   cvt plugins install ./cvt-plugin-rest
   ```
   The binary is copied into `~/.cvt/plugins/`; its SHA256 is recorded in
   `~/.cvt/plugins/state.json`.

2. **Declare the plugin in `~/.cvt/config.yaml`:**
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
     register_consumer_usage: registry
   ```

3. **Run CVT.** The plugin forks at startup, receives its secret via
   gRPC (not subprocess env), and is ready for the bound hooks. Three
   of the four hooks fire from core today; see the status table above.
   See [config.md](config.md) for the full schema.

4. **Write your own plugin.** See [authoring-go.md](authoring-go.md).
   Plugin authors import `github.com/sahina/cvt/pkg/cvtplugin` and call
   `cvtplugin.Serve(...)` from their `main`. That's it.

## Architecture

```
  CVT (cvt serve or cvt validate)
    │
    │  hashicorp/go-plugin: fork + gRPC over Unix socket
    ▼
  [cvt-plugin-rest]     [cvt-plugin-slack]
       (subprocess)                  (subprocess)
```

The plugin manager forks each configured plugin once at CVT startup,
performs the native `go-plugin` handshake, calls the CVT-specific
`PluginHandshake.Info` RPC to negotiate the protocol version, delivers
configured values (including secrets) via `SetConfig`, and keeps the
typed gRPC clients alive for the life of the CVT process. When CVT
shuts down, plugins receive SIGTERM with a 30-second grace period, then
SIGKILL.

## Reference plugins

Two first-party reference plugins ship in separate repos:

- **[cvt-plugin-rest](https://github.com/sahina/cvt-plugin-rest)** —
  `RegistryProvider` backed by any REST schema registry. Fetches
  OpenAPI specs by ID and records consumer usage via HTTP. Covers
  [issue #83](https://github.com/sahina/cvt/issues/83).
- **[cvt-plugin-slack](https://github.com/sahina/cvt-plugin-slack)** —
  `EventHandler` that posts breaking-change and validation-failure
  events to a Slack webhook, with plugin-side dedup so you don't get
  paged 10,000 times per bad deploy.

See [reference-plugins.md](reference-plugins.md) for walkthroughs and
install recipes. Each plugin's own repo README covers its full config
surface and a quick-test recipe.

## Scope: what's in v1, what isn't

**In v1:**
- Two extension points: `RegistryProvider` and `EventHandler`.
- One plugin per hook (no fanout).
- Go SDK.
- Linux + macOS.
- Three CLI commands: `cvt plugins list`, `install`, `remove`.
- Four Prometheus metrics + synchronous audit.

**Deferred to v1.1+:**
- Multi-plugin fanout per hook, pipeline stages, `race`/`first_success`
  strategies.
- Custom `Validator` extension point.
- Hot reload, circuit-breaker admin (`cvt plugins reset`), inspect/verify.
- Python/Node/Java plugin SDKs.
- Windows support.
- GitHub-URL install, signed plugin tags.

See the design doc at `docs/design/plugin-system.md` for the full scope
reasoning.
