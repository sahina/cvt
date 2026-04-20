# Writing a CVT plugin in Go

A CVT plugin is a standalone Go binary that implements one or both of the CVT plugin interfaces and calls `cvtplugin.Serve(...)` from its `main`. `hashicorp/go-plugin` handles subprocess lifecycle and gRPC transport.

Go is the only supported SDK in v1. Python / Node / Java are deferred to v1.1+.

## TL;DR — a minimum viable plugin in ~30 lines

```go
package main

import (
    "context"

    "github.com/sahina/cvt/pkg/cvtplugin"
    registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
)

type myRegistry struct{}

func (r *myRegistry) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
    // fetch OpenAPI spec bytes by req.SchemaId / req.Version, respect ctx.Done()
    return &registrypb.FetchSchemaResponse{Spec: specBytes, ResolvedVersion: "1.0.0"}, nil
}

func (r *myRegistry) RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
    // idempotent upsert
    return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
}

func main() {
    cvtplugin.Serve(
        cvtplugin.PluginInfo{Name: "my-registry", Version: "0.1.0"},
        cvtplugin.WithRegistryProvider(&myRegistry{}),
    )
}
```

Build it as `cvt-plugin-my-registry` (the `cvt-plugin-` prefix becomes the CVT-visible name), install with `cvt plugins install`, declare in `~/.cvt/config.yaml`, bind to hooks. Done.

## Walkthrough

### 1. New Go module

```sh
mkdir cvt-plugin-my-registry && cd cvt-plugin-my-registry
go mod init github.com/you/cvt-plugin-my-registry
go get github.com/sahina/cvt/pkg/cvtplugin
```

### 2. Implement the interface

`RegistryProvider` has two methods. Both must be safe for concurrent use — CVT sends concurrent RPCs. See TL;DR above for the full skeleton.

Key contract:
- `FetchSchema` — return raw OpenAPI bytes for `(SchemaId, Version)`. Respect `ctx.Done()` (see below).
- `RegisterConsumerUsage` — **must be idempotent upsert** on your side. CVT guarantees at-most-one-plugin-per-hook in v1, but your dedup protects against retries, replays, and future v1.1 multi-sink fanout.

### 3. Receive config

CVT delivers every `config:` entry from `~/.cvt/config.yaml` via `SetConfig` gRPC at plugin startup, before any extension-point RPC runs. Secret keys are delivered this way too — **never** via subprocess env.

Implement `cvtplugin.ConfigReceiver`:

```go
func (r *myRegistry) SetConfig(_ context.Context, key, value string) error {
    switch key {
    case "base_url":
        r.baseURL = value
    case "token":
        r.token = value
    }
    return nil
}
```

Accept unknown keys silently — a future CVT may deliver keys your plugin doesn't know yet.

Register the receiver alongside the provider:

```go
cvtplugin.Serve(
    cvtplugin.PluginInfo{Name: "my-registry", Version: "0.1.0"},
    cvtplugin.WithRegistryProvider(r),
    cvtplugin.WithConfigReceiver(r),
)
```

### 4. Build with the expected name

Plugin name in config defaults to the binary basename with the `cvt-plugin-` prefix stripped. Build the binary as `cvt-plugin-my-registry` → CVT-visible name is `my-registry`:

```sh
go build -o cvt-plugin-my-registry .
cvt plugins install ./cvt-plugin-my-registry
```

### 5. Declare in config

```yaml
config_version: 1
plugins:
  my-registry:
    binary: ~/.cvt/plugins/cvt-plugin-my-registry
    on_error: fail_closed
    secrets: [token]
    config:
      base_url: https://registry.example.com
      token: ${MY_REGISTRY_TOKEN}
hooks:
  fetch_schema: my-registry
  register_consumer_usage: my-registry
```

Run CVT; your plugin handles both hooks. Verify with `cvt plugins list` + the `cvt_plugin_up{plugin="my-registry"}` metric on `:9551/metrics`.

---

## Next

- [Config reference](config.md) — full config.yaml schema.
- [Reference plugins](reference-plugins.md) — walkthroughs of the two first-party plugins.
- Proto contracts live at `api/protos/plugin/` in the CVT repo.

<details>
<summary>Respecting context deadlines (mandatory)</summary>

Every plugin RPC runs under a context deadline derived from the plugin's `timeout` config (default 5s). Your handler **must** respect `ctx.Done()`:

```go
func (r *myRegistry) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
    httpReq, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := http.DefaultClient.Do(httpReq)
    // ...
}
```

Plugins that ignore `ctx.Done()` eventually get killed by the supervisor, but they burn subprocess time in the meantime.

</details>

<details>
<summary>Connection pooling (reuse one http.Client)</summary>

Reuse a single `http.Client` across calls. Go's `http.DefaultClient` pools connections via HTTP/1.1 keep-alive / HTTP/2 multiplexing, but only if you reuse the same client — constructing a new `http.Client` or `http.Transport` per call throws away the pool and forces fresh connections. Store one `http.Client` on your plugin struct and reuse it.

```go
type myRegistry struct {
    hc *http.Client
}

func newRegistry() *myRegistry {
    return &myRegistry{hc: &http.Client{Timeout: 10 * time.Second}}
}
```

Also: fully drain and `Close()` response bodies so keep-alive connections actually return to the pool.

</details>

<details>
<summary>Implementing only some event methods</summary>

`EventHandler` has one RPC per event type. If you only care about some, embed `cvtplugin.UnimplementedEventHandler` — CVT treats `Unimplemented` as a non-error no-op.

```go
type mySlack struct {
    cvtplugin.UnimplementedEventHandler
}

// Only implement the event you care about.
func (s *mySlack) OnBreakingChangeDetected(ctx context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
    // post to Slack
    return &eventspb.EventResponse{Acknowledged: true}, nil
}
```

</details>

<details>
<summary>Logging</summary>

`go-plugin` forwards structured logs from plugins to CVT's host logger. Use `hclog.Logger` (or stdlib `log` — `go-plugin` intercepts it):

```go
import "github.com/hashicorp/go-hclog"

logger := hclog.Default()
logger.Info("fetched schema", "schema_id", id, "duration_ms", dur.Milliseconds())
```

Entries arrive in CVT logs with a `plugin=<name>` field added automatically. Don't include the plugin name yourself.

Sample log line on CVT's side:

```text
INFO plugin-log plugin=my-registry version=0.1.0 msg="fetched schema" schema_id=pet-api duration_ms=42
```

</details>

<details>
<summary>Unit testing with plugintest</summary>

The SDK ships an in-process test harness — exercise your implementation without forking a subprocess:

```go
import "github.com/sahina/cvt/pkg/cvtplugin/plugintest"

func TestFetchSchema(t *testing.T) {
    r := newRegistry()
    h := plugintest.New().WithRegistry(r)
    h.SetConfig(context.Background(), "base_url", "http://127.0.0.1:8080")

    resp, err := h.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "pet-api"})
    // assertions...
}
```

</details>

<details>
<summary>Releasing</summary>

- Ship SHA256 hashes alongside your binaries. `cvt plugins install` records the hash; operators should verify against your published hash. Hash is the trusted identity in CVT audit logs.
- Tag releases against the proto version you're compatible with. Plugin proto bumps to `v2` would break `v1` plugins at handshake — reject cleanly.
- Linux + macOS only in v1. Windows plugin support is v1.1+.

</details>
