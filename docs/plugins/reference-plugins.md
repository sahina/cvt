# Reference plugins

Two first-party plugins live in separate repositories. Together they
validate the plugin SDK from two angles — a read-heavy registry backend
and an event fan-out sink — and cover the two concrete use cases the
plugin system was built for.

| Plugin | Contract | Repo | Covers |
|---|---|---|---|
| [`cvt-plugin-rest`](https://github.com/sahina/cvt-plugin-rest) | `RegistryProvider` | <https://github.com/sahina/cvt-plugin-rest> | REST-backed schema registries (issue [#83](https://github.com/sahina/cvt/issues/83)) |
| [`cvt-plugin-slack`](https://github.com/sahina/cvt-plugin-slack) | `EventHandler` | <https://github.com/sahina/cvt-plugin-slack> | Slack notifications for breaking changes + validation failures |

Both were extracted from this monorepo on 2026-04-19 and track CVT
releases via the `github.com/sahina/cvt` Go module dependency.
Each plugin's own repo README carries the full config surface and a
quick-test recipe; this page is the walkthrough.

## Install

Clone the plugin repo, build the binary, install with `cvt plugins install`:

```bash
git clone https://github.com/sahina/cvt-plugin-rest
cd cvt-plugin-rest
go build -o cvt-plugin-rest .
cvt plugins install ./cvt-plugin-rest
```

To install the Slack event sink, repeat the same steps against its repo:

```bash
git clone https://github.com/sahina/cvt-plugin-slack
cd cvt-plugin-slack
go build -o cvt-plugin-slack .
cvt plugins install ./cvt-plugin-slack
```

`cvt plugins install` copies the binary to `~/.cvt/plugins/` and
records its SHA256 in `~/.cvt/plugins/state.json`. Verify with
`cvt plugins list`.

## cvt-plugin-rest

**Repo:** <https://github.com/sahina/cvt-plugin-rest> ([README](https://github.com/sahina/cvt-plugin-rest#readme))
**Contract:** `RegistryProvider`
**Hooks it serves:** `fetch_schema`, `register_consumer_usage`

### What it does

CVT's built-in schema loader handles files and raw URLs. Many
organizations run a **Central API Registry** instead — a service that
hosts OpenAPI specs with versioning, access control, and consumer
tracking (Apicurio, Backstage, SwaggerHub, or a homegrown REST API).
`cvt-plugin-rest` is the bridge.

- `FetchSchema`: translates "give me schema X at version Y" into
  `GET {base_url}/schemas/{id}/versions/{version}/spec` and returns
  the raw OpenAPI bytes. Respects `version = "latest"` and the
  optional `X-Schema-Version` response header that exposes the
  resolved version.
- `RegisterConsumerUsage`: translates consumer-usage reporting into
  `POST {base_url}/schemas/{id}/consumers` with a JSON body containing
  consumer identity, schema version, environment, and endpoints
  tested. MUST be an idempotent upsert on the server side. Your
  registry can use this feed to answer "who depends on this schema?",
  gate deployments on consumer coverage, or build a contract map.

Design goal: stay thin. No caching. No auth flows beyond an optional
bearer token. One retry on 5xx, then surface the failure via gRPC
status.

### Config

```yaml
plugins:
  registry:
    binary: ~/.cvt/plugins/cvt-plugin-rest
    timeout: 5s
    on_error: fail_closed
    secrets: [token]
    config:
      base_url: https://registry.example.com/api/v1
      token: ${CVT_REGISTRY_TOKEN}
hooks:
  fetch_schema: registry
  register_consumer_usage: registry
```

| Config key | Required | Description |
|---|---|---|
| `base_url` | yes | Registry API base URL. |
| `token` | no | Bearer token sent as `Authorization: Bearer <token>`. Declare in `secrets:` to keep it out of logs. |

### Failure modes

- Registry unreachable → plugin returns gRPC `Unavailable`. With
  `on_error: fail_closed`, `cvt validate` fails.
- Schema not found → plugin returns gRPC `NotFound`. CVT treats as
  schema-resolution failure.
- Registry returns 5xx → retried once by the plugin's internal HTTP
  client, then surfaced.

### When to use this

- Your organization runs a Central API Registry with a REST API and
  wants CVT to fetch schemas from there instead of files/URLs.
- You want consumer-usage tracking in CI — every `cvt validate` in your
  pipeline records a consumer→schema dependency.

### Test drive

Don't have a real registry? The repo README has a toy Python HTTP
server recipe that answers both endpoints with a minimal spec — enough
to prove the wiring works end-to-end. See
<https://github.com/sahina/cvt-plugin-rest#quick-test-against-a-fake-registry>.

## cvt-plugin-slack

**Repo:** <https://github.com/sahina/cvt-plugin-slack> ([README](https://github.com/sahina/cvt-plugin-slack#readme))
**Contract:** `EventHandler`
**Hooks it serves:** `on_breaking_change_detected`, `on_validation_failed`

### What it does

CVT fires two categories of event during its normal operation. Without
a plugin they're logged and audited but nobody outside CVT notices.
`cvt-plugin-slack` turns those events into Slack messages.

- `OnValidationFailed`: formats and posts a message describing the
  failed interaction (method, path, schema ID, first validation
  error). Fires from `ValidateInteraction` whenever a request/response
  pair fails validation — in CI, from `cvt validate`, or in a running
  `cvt serve`. Includes per-plugin dedup so a misconfigured producer
  throwing 10k failures/minute doesn't page your channel 10k times.
- `OnBreakingChangeDetected`: formats a Slack message listing each
  breaking change (kind, method, path) and posts to the webhook.
  Fires from two paths: the `CompareSchemas` gRPC method (diff-tool
  / PR-comment-bot pattern) and from `RegisterSchema` when
  `check_compatibility=true` (CLI: `cvt register-schema --check-compatibility`).

**Fail-open by default.** Slack outages should not block `cvt validate`
or `cvt serve`. Configure `on_error: fail_open` and transient Slack
failures are audited but swallowed.

### Config

```yaml
plugins:
  slack:
    binary: ~/.cvt/plugins/cvt-plugin-slack
    timeout: 3s
    on_error: fail_open   # don't let Slack outages fail CVT
    secrets: [webhook_url]
    config:
      webhook_url: ${SLACK_WEBHOOK}
      channel: "#contract-alerts"
      dedup_window_seconds: "60"
hooks:
  on_breaking_change_detected: slack
  on_validation_failed: slack
```

| Config key | Required | Description |
|---|---|---|
| `webhook_url` | yes | Slack incoming-webhook URL. Declare in `secrets:`. |
| `channel` | no | Override channel (the webhook's default is used if unset). |
| `dedup_window_seconds` | no | Plugin-side dedup window for repeated events. Default `"60"`. Set `"0"` to disable. |

### Failure modes

- Slack API unreachable → plugin returns error; `on_error: fail_open`
  swallows it. Event is audited but not re-sent.
- Rate-limited by Slack (`429`) → plugin backs off, drops subsequent
  events in the window. Counted in audit.

### When to use this

- Ops team wants real-time notification on breaking-change detection
  in CI.
- Validation failures in production should page the on-call channel.

### Test drive

Slack incoming webhooks are free for personal workspaces. The repo
README has a 5-minute setup + a `cvt compare` recipe that posts a real
message to your channel. See
<https://github.com/sahina/cvt-plugin-slack#quick-test-with-a-personal-webhook>.

No Slack? Swap `webhook_url` for a request-inspector URL
(`https://webhook.site/`) to see the raw JSON payloads.

## Why these two

The plugin system exists to unblock two pieces of work that had been
deferred as "needs design" TODOs:

1. **Issue [#83](https://github.com/sahina/cvt/issues/83)** — consumer
   testing with a central schema registry.
2. **P2: Notification system design** — integrations without embedding
   Slack/Jira/etc. logic in core.

Building both as out-of-tree plugins validates the framework from two
independent angles: one read-heavy registry backend, one event fan-out
sink. Further plugins (GitHub-backed registry, Jira notifier, etc.) can
follow the same pattern.

## Contributing a new reference plugin

See [authoring-go.md](authoring-go.md) for the mechanics. If the plugin
is broadly useful (not org-specific), propose it as a new repo under
`github.com/sahina/` via a CVT issue. The bar is:

1. Handles a concrete use case at least two teams have asked for.
2. Ships SHA256 hashes alongside release artifacts.
3. Passes CI against the current plugin proto version.
