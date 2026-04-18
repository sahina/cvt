# cvt-plugin-slack-events

Slack-webhook event sink for CVT. Implements the `EventHandler` plugin
contract: posts breaking-change and validation-failure events to a
Slack channel with plugin-side dedup to avoid flooding.

## Build

```sh
go build -o cvt-plugin-slack-events .
cvt plugins install ./cvt-plugin-slack-events
```

## Config

```yaml
plugins:
  slack:
    binary: ~/.cvt/plugins/cvt-plugin-slack-events
    timeout: 3s
    on_error: fail_open   # don't fail CVT if Slack is down
    secrets: [webhook_url]
    config:
      webhook_url: ${SLACK_WEBHOOK}
      channel: "#contract-alerts"
      dedup_window_seconds: "60"
hooks:
  on_breaking_change_detected: slack
  on_validation_failed: slack
```

## Dedup

Repeat events within `dedup_window_seconds` are silently dropped. Dedup
key:

- Breaking change: `(schema_id, old_version, new_version)`.
- Validation failure: `(schema_id, method, path, first_error_kind)`.

Set `dedup_window_seconds: "0"` to disable dedup and forward every event.

## gRPC status codes

| Code | When |
|---|---|
| `FailedPrecondition` | `webhook_url` not configured |
| `ResourceExhausted` | Slack returned 429 |
| `Unavailable` | Slack unreachable or 4xx/5xx |
