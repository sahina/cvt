# Reference plugins

Two first-party plugins live in separate repositories. Together they exercise the plugin SDK from two angles — a read-heavy registry backend and an event fan-out sink — and cover the two concrete use cases the plugin system was built for.

## Which plugin do I need?

| You want to… | Plugin | Contract |
|---|---|---|
| Fetch OpenAPI specs from a REST schema registry (Apicurio, Backstage, internal API registry) | [`cvt-plugin-rest`](https://github.com/sahina/cvt-plugin-rest) | `RegistryProvider` |
| Track consumer→schema usage in a central system | [`cvt-plugin-rest`](https://github.com/sahina/cvt-plugin-rest) | `RegistryProvider` |
| Post breaking-change alerts to Slack | [`cvt-plugin-slack`](https://github.com/sahina/cvt-plugin-slack) | `EventHandler` |
| Alert the on-call channel on validation failures | [`cvt-plugin-slack`](https://github.com/sahina/cvt-plugin-slack) | `EventHandler` |
| Something else | Write your own — [authoring guide](authoring-go.md) | Custom |

Both reference plugins were extracted from the main CVT repo on 2026-04-19 and track CVT releases via the `github.com/sahina/cvt` Go module dependency. Each plugin's own repo README carries its full config surface and a quick-test recipe.

## Install (same for both)

```sh
git clone https://github.com/sahina/cvt-plugin-rest  # or cvt-plugin-slack
cd cvt-plugin-rest
go build -o cvt-plugin-rest .
cvt plugins install ./cvt-plugin-rest
```

`cvt plugins install` copies the binary to `~/.cvt/plugins/` and records its SHA256 in `~/.cvt/plugins/state.json`. Verify:

```sh
cvt plugins list
# NAME      SHA256        INSTALLED                  BINARY
# rest      abc123def456  2026-04-20 12:35:00 EDT   /Users/you/.cvt/plugins/cvt-plugin-rest
```

---

## cvt-plugin-rest

**Repo:** <https://github.com/sahina/cvt-plugin-rest> · **Contract:** `RegistryProvider` · **Hooks:** `fetch_schema`, `register_consumer_usage`

Bridges CVT to a **Central API Registry** — a service that hosts OpenAPI specs with versioning, access control, and consumer tracking (Apicurio, Backstage, SwaggerHub, or a homegrown REST API).

- `FetchSchema` → `GET {base_url}/schemas/{id}/versions/{version}/spec`. Respects `version = "latest"` and the optional `X-Schema-Version` response header that exposes the resolved version.
- `RegisterConsumerUsage` → `POST {base_url}/schemas/{id}/consumers` with consumer identity, schema version, environment, and endpoints tested. Must be an idempotent upsert on the registry side.

Design: thin bridge. No caching. No auth flows beyond an optional bearer token. One retry on 5xx, then surface failure via gRPC status.

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

| Key | Required | Description |
|---|---|---|
| `base_url` | yes | Registry API base URL |
| `token` | no | Bearer token; declare in `secrets:` to keep out of logs |

### Failure modes

| Situation | Result |
|---|---|
| Registry unreachable | gRPC `Unavailable`; with `fail_closed`, `cvt validate` fails |
| Schema not found | gRPC `NotFound`; CVT treats as schema-resolution failure |
| Registry 5xx | One retry inside the plugin, then surfaced |

### Test drive without a real registry

The repo README has a toy Python HTTP server recipe that answers both endpoints with a minimal spec — enough to prove the wiring works end-to-end. See <https://github.com/sahina/cvt-plugin-rest#quick-test-against-a-fake-registry>.

---

## cvt-plugin-slack

**Repo:** <https://github.com/sahina/cvt-plugin-slack> · **Contract:** `EventHandler` · **Hooks:** `on_breaking_change_detected`, `on_validation_failed`

Posts CVT events to a Slack webhook. Without a plugin these events are logged and audited but nobody outside CVT notices.

- `OnValidationFailed` → Slack message describing a failed validation (method, path, schema ID, first validation error). Fires from `pkg/cvt.Validator.Validate` when it returns a non-valid result (CLI `cvt validate` path and library callers). Note: the server's `ValidateInteraction` gRPC method runs validation directly via `openapi3filter` and does **not** call this hook in v1. Plugin-side dedup so a misconfigured producer throwing 10k failures/minute doesn't page your channel 10k times.
- `OnBreakingChangeDetected` → Slack message listing each breaking change (kind, method, path). Fires from `CompareSchemas` and from `RegisterSchema --check-compatibility`.

Designed **fail-open**: Slack outages should not block `cvt validate` or `cvt serve`.

### Config

```yaml
plugins:
  slack:
    binary: ~/.cvt/plugins/cvt-plugin-slack
    timeout: 3s
    on_error: fail_open   # Slack outages shouldn't fail CVT
    secrets: [webhook_url]
    config:
      webhook_url: ${SLACK_WEBHOOK}
      channel: "#contract-alerts"
      dedup_window_seconds: "60"
hooks:
  on_breaking_change_detected: slack
  on_validation_failed: slack
```

| Key | Required | Description |
|---|---|---|
| `webhook_url` | yes | Slack incoming-webhook URL; declare in `secrets:` |
| `channel` | no | Override channel; webhook default otherwise |
| `dedup_window_seconds` | no | Plugin-side dedup. Default `"60"`; `"0"` disables |

### Failure modes

| Situation | Result |
|---|---|
| Slack API unreachable | Plugin returns error; `fail_open` swallows it. Event audited, not re-sent |
| Slack rate-limit (429) | Plugin backs off, drops subsequent events in the window. Counted in audit |

### Test drive

Slack incoming webhooks are free on personal workspaces. The repo README has a 5-minute setup + a `cvt compare` recipe that posts a real message to your channel: <https://github.com/sahina/cvt-plugin-slack#quick-test-with-a-personal-webhook>.

No Slack account? Swap `webhook_url` for a request-inspector URL (`https://webhook.site/`) to see the raw JSON payloads.

---

## Why these two

The plugin system was built to unblock two deferred tracks:

1. **Issue [#83](https://github.com/sahina/cvt/issues/83)** — consumer testing with a central schema registry.
2. **Notification system design** (P2 TODO) — integrations without embedding Slack/Jira/etc. logic in core.

Building both as out-of-tree plugins validates the framework from two independent angles: one read-heavy registry backend, one event fan-out sink. Further plugins (GitHub-backed registry, Jira notifier) can follow the same pattern.

## Contributing a new reference plugin

Mechanics: [authoring-go.md](authoring-go.md). If the plugin is broadly useful (not org-specific), propose it as a new repo under `github.com/sahina/` via a CVT issue. Bar:

1. Handles a concrete use case at least two teams have asked for.
2. Ships SHA256 hashes alongside release artifacts.
3. Passes CI against the current plugin proto version.
