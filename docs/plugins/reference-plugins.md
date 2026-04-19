# Reference plugins

Two first-party plugins live in separate repositories. They validate
the SDK from a plugin-author perspective and cover the two concrete
use cases the plugin system was built for.

Both were extracted from this monorepo on 2026-04-19 and track CVT
releases via the `github.com/sahina/cvt` Go module dependency.

## Installation

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

## cvt-plugin-rest

**Repo:** <https://github.com/sahina/cvt-plugin-rest>
**Contract:** `RegistryProvider`

A `RegistryProvider` implementation that speaks to a generic REST
schema registry. It covers the design described in
[issue #83 (Consumer Testing with Central API Registry Integration)](https://github.com/sahina/cvt/issues/83)
and supersedes the in-tree `HTTPProvider` that was originally planned
under the Phase 1a Enterprise Deployment work.

### What it implements

- `FetchSchema`: HTTP `GET {base_url}/schemas/{id}/versions/{version}/spec`.
  Returns the raw OpenAPI spec bytes.
- `RegisterConsumerUsage`: HTTP `POST {base_url}/schemas/{id}/consumers`
  with a JSON body containing consumer identity, schema version,
  environment, and endpoints tested. Idempotent upsert per spec.

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

- Registry unreachable → returns gRPC `Unavailable`. With `on_error:
  fail_closed`, `cvt validate` fails.
- Schema not found → returns gRPC `NotFound`. CVT treats as
  schema-resolution failure.
- Registry returns 5xx → retried once by the plugin's internal HTTP
  client, then surfaced.

### When to use this

- Your organization runs a Central API Registry with a REST API and
  wants CVT to fetch schemas from there instead of files/URLs.
- You want consumer-usage tracking in CI — every `cvt validate` in your
  pipeline records a consumer→schema dependency.

## cvt-plugin-slack

**Repo:** <https://github.com/sahina/cvt-plugin-slack>
**Contract:** `EventHandler`

An `EventHandler` implementation that posts CVT events to a Slack
webhook. Supersedes the "P2: Notification system design document" TODO.

### What it does

- `OnBreakingChangeDetected`: formats a Slack message summarizing the
  breaking changes and posts to the configured webhook URL. Fires from
  the `CompareSchemas` gRPC method and from `RegisterSchema` when
  compatibility checking is enabled (`cvt register-schema --check-compatibility`).
- `OnValidationFailed`: formats and posts a message describing the
  failed interaction. Fires from `ValidateInteraction` when a
  request/response pair fails validation. Includes per-plugin dedup
  + rate limiting so a broken deploy producing 10k failures/minute
  doesn't translate to 10k Slack messages.

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
| `dedup_window_seconds` | no | Plugin-side dedup window for repeated events. Default 60s. |

### Failure modes

- Slack API unreachable → plugin returns error; `on_error: fail_open`
  swallows it. Event is audited but not re-sent.
- Rate-limited by Slack (`429`) → plugin backs off, drops subsequent
  events in the window. Counted in audit.

### When to use this

- Ops team wants real-time notification on breaking-change detection
  in CI.
- Validation failures in production should page the on-call channel.

## Why these two

The plugin system exists to unblock two specific pieces of work that
had been deferred as "needs design" TODOs:

1. **Issue #83** — consumer testing with a central schema registry.
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
