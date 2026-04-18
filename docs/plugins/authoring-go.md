# Writing a CVT plugin in Go

A CVT plugin is a standalone Go binary. It implements one or both of the
CVT plugin interfaces, calls `cvtplugin.Serve(...)` from its `main`, and
`hashicorp/go-plugin` handles the subprocess lifecycle + gRPC transport
for you.

This guide walks through a minimal registry plugin. The same pattern
applies to event handlers.

## 1. Start a new Go module

```sh
mkdir cvt-plugin-my-registry
cd cvt-plugin-my-registry
go mod init github.com/you/cvt-plugin-my-registry
go get github.com/sahina/cvt/pkg/cvtplugin
```

## 2. Implement `RegistryProvider`

Two methods. Both safe for concurrent use (CVT will send concurrent RPCs).

```go
// main.go
package main

import (
    "context"

    "github.com/sahina/cvt/pkg/cvtplugin"
    registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
)

type myRegistry struct {
    baseURL string
    token   string
}

func (r *myRegistry) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
    // ... HTTP GET to r.baseURL ... respect ctx deadline ...
    return &registrypb.FetchSchemaResponse{
        Spec:            specBytes,
        ResolvedVersion: "1.0.0",
    }, nil
}

func (r *myRegistry) RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
    // MUST be idempotent upsert. CVT provides at-most-one-plugin-per-hook
    // semantics in v1, but your plugin's own dedup protects against
    // retries, replays, and future v1.1 multi-sink fanout.
    // ... HTTP POST to r.baseURL ...
    return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
}
```

## 3. Receive config via `SetConfig` (optional but recommended)

CVT delivers every `config:` entry from `~/.cvt/config.yaml` via a gRPC
`SetConfig` call at plugin startup, before any extension-point RPC runs.
Secret keys (declared in `secrets:`) are delivered this way too —
**never** via subprocess environment variables.

Implement `cvtplugin.ConfigReceiver` to receive the calls:

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

Unknown keys should be accepted silently — CVT versions newer than your
plugin may add keys your code doesn't know yet.

## 4. Wire up `main`

```go
func main() {
    r := &myRegistry{}
    cvtplugin.Serve(
        cvtplugin.PluginInfo{
            Name:    "my-registry",
            Version: "0.1.0",
        },
        cvtplugin.WithRegistryProvider(r),
        cvtplugin.WithConfigReceiver(r),
    )
}
```

`Serve` blocks until CVT disconnects. Any return from `Serve` causes
`go-plugin` to exit the process, which is the correct behavior on
shutdown.

## 5. Build with the expected name

CVT plugin names (the config-file key under `plugins:`) default to the
binary basename with a `cvt-plugin-` prefix stripped. Install the binary
as `cvt-plugin-my-registry` and it becomes the plugin named `my-registry`:

```sh
go build -o cvt-plugin-my-registry .
cvt plugins install ./cvt-plugin-my-registry
```

## 6. Add to `~/.cvt/config.yaml`

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

Run CVT. Your plugin handles both hooks.

## Logging

`go-plugin` forwards structured logs from plugins to CVT's host logger.
Use `hclog.Logger` (or just `log` — `go-plugin` intercepts standard
library log output too):

```go
import "github.com/hashicorp/go-hclog"

logger := hclog.Default()
logger.Info("fetched schema", "schema_id", id, "duration_ms", dur.Milliseconds())
```

Log entries arrive in CVT logs with a `plugin=<name>` field
automatically added. No need to include the plugin name yourself.

## Context deadlines

Every plugin RPC runs under a context deadline derived from the
plugin's `timeout` config (default 5s). Your handler MUST respect
`ctx.Done()`:

```go
func (r *myRegistry) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
    httpReq, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := http.DefaultClient.Do(httpReq)
    // ...
}
```

Plugins that ignore `ctx.Done()` eventually get killed by the supervisor,
but they burn subprocess time in the meantime.

## Connection pooling

Reuse a single `http.Client` across calls. The default client opens a
new connection per request, which is wasteful when CVT sends many
consecutive `FetchSchema` calls.

```go
type myRegistry struct {
    hc *http.Client
}

func newRegistry() *myRegistry {
    return &myRegistry{hc: &http.Client{Timeout: 10 * time.Second}}
}
```

## Unit testing with `plugintest`

The SDK ships an in-process test harness so you can exercise your
implementation without forking a subprocess:

```go
import "github.com/sahina/cvt/pkg/cvtplugin/plugintest"

func TestFetchSchema(t *testing.T) {
    r := newRegistry()
    h := plugintest.New().WithRegistry(r)
    h.SetConfig(context.Background(), "base_url", "http://127.0.0.1:8080")

    resp, err := h.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "pet-api"})
    // ... assertions ...
}
```

## Implementing only part of an interface

`EventHandler` has multiple RPC methods (one per event type) but you may
only care about some. Embed `cvtplugin.UnimplementedEventHandler` to
auto-return `Unimplemented` for events you don't handle. CVT treats
`Unimplemented` as a no-op, not an error.

```go
type mySlack struct {
    cvtplugin.UnimplementedEventHandler
}

// Implement only the event you care about.
func (s *mySlack) OnBreakingChangeDetected(ctx context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
    // ... post to Slack ...
    return &eventspb.EventResponse{Acknowledged: true}, nil
}
```

## Releasing

Ship SHA256 hashes alongside your binaries so operators can verify them
against what `cvt plugins install` records. The install-time hash is the
trusted identity in CVT audit logs.

Tag releases against the proto version you're compatible with. Changes
to the plugin proto bump to `v2`; plugins built against `v1` won't load
in a core that expects `v2`.

## Where to go next

- [Config reference](config.md) — the full config.yaml schema.
- [Reference plugins](reference-plugins.md) — walkthroughs of the two
  first-party reference plugins.
- Proto files live at `api/protos/plugin/` in the CVT repo — the wire
  contract.
