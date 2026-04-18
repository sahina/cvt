# CVT Plugins

CVT has a plugin system for teams that need to extend CVT without forking
it. A plugin is a separate binary that implements one or more CVT plugin
services (registry provider, event handler) and communicates with `cvt`
over gRPC on a Unix domain socket. Plugins live in their own repositories
on their own release schedules.

## v1 call-site status

v1 wires one of the four planned hook call sites today:

| Hook | Plugin service | Status in v1 |
|---|---|---|
| `on_validation_failed` | `EventHandler` | **fires from core** after `Validate()` returns a non-valid result |
| `fetch_schema` | `RegistryProvider` | SDK + config surface only; core call site deferred to [#107](https://github.com/sahina/cvt/issues/107) (needs schema-by-ID resolution path from issue #83) |
| `register_consumer_usage` | `RegistryProvider` | SDK + config surface only; core call site deferred to [#107](https://github.com/sahina/cvt/issues/107) (blocked on P2 god-file split of `server/cvtservice/validator_service.go`) |
| `on_breaking_change_detected` | `EventHandler` | SDK + config surface only; core call site deferred to [#107](https://github.com/sahina/cvt/issues/107) (blocked on same split) |

Plugins can be written, installed, and configured against all four hook
names today. The SDK, proto contracts, and config schema are frozen.
Configured-but-not-yet-wired hooks are declarative no-ops until their
call sites land.

## When to use a plugin

Use a plugin when you want:

- **Custom schema registry.** CVT's built-in schema loading handles files
  and raw URLs. If your organization hosts schemas in a registry with its
  own API (internal Central API Registry, Backstage, Apicurio), write a
  `RegistryProvider` plugin and point `fetch_schema` at it. The plugin
  is ready to ship; the core call site is wired in #107.
- **Event integrations.** CVT fires events on breaking-change detection
  and validation failure. Write an `EventHandler` plugin that reacts to
  those events. Today only `on_validation_failed` fires; the
  `on_breaking_change_detected` call site lands in #107.

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
   cvt plugins install ./cvt-plugin-registry-rest
   ```
   The binary is copied into `~/.cvt/plugins/`; its SHA256 is recorded in
   `~/.cvt/plugins/state.json`.

2. **Declare the plugin in `~/.cvt/config.yaml`:**
   ```yaml
   config_version: 1
   plugins:
     registry:
       binary: ~/.cvt/plugins/cvt-plugin-registry-rest
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
   gRPC (not subprocess env), and is ready for the bound hooks. In v1
   only `on_validation_failed` invokes the plugin from core; the
   registry hooks activate when their call sites land (#107). See
   [config.md](config.md) for the full schema.

4. **Write your own plugin.** See [authoring-go.md](authoring-go.md).
   Plugin authors import `github.com/sahina/cvt/pkg/cvtplugin` and call
   `cvtplugin.Serve(...)` from their `main`. That's it.

## Architecture

```
  CVT (cvt serve or cvt validate)
    │
    │  hashicorp/go-plugin: fork + gRPC over Unix socket
    ▼
  [cvt-plugin-registry-rest]     [cvt-plugin-slack-events]
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

- `github.com/sahina/cvt-plugin-registry-rest` — simple REST registry
  client (covers issue #83).
- `github.com/sahina/cvt-plugin-slack-events` — posts breaking-change
  and validation-failure events to a Slack webhook.

See [reference-plugins.md](reference-plugins.md) for walkthroughs.

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
