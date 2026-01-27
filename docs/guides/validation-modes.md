---
title: Validation Modes
sidebar_label: Validation Modes
sidebar_position: 3
description: CVT producer middleware validation modes - strict, warn, and shadow
---

# Validation Modes

**CONTENT TO BE VALIDATED**

CVT producer middleware supports three validation modes that control how contract violations are handled. Choose based on your deployment stage and risk tolerance.

## Mode Reference

| Mode       | Request Violation                | Response Violation                | Metrics  | Use Case                                  |
| ---------- | -------------------------------- | --------------------------------- | -------- | ----------------------------------------- |
| **strict** | Reject with 400 Bad Request      | Log error (response already sent) | Recorded | Production enforcement after rollout      |
| **warn**   | Log warning, continue processing | Log warning, continue             | Recorded | Gradual rollout, monitoring impact        |
| **shadow** | Silent (no logging)              | Silent (no logging)               | Recorded | Initial deployment, metrics-only analysis |

## Mode Behavior Details

### Strict Mode

```text
Request → Validate → Invalid? → Return 400 (handler never executes)
                   → Valid?   → Execute handler → Validate response → Log if invalid
```

- **Request violations**: Immediately rejected with `400 Bad Request` and error details
- **Response violations**: Logged as errors but response is still sent (can't unsend)
- **Best for**: Production APIs after you've validated with warn/shadow modes

### Warn Mode

```text
Request → Validate → Log if invalid → Execute handler → Validate response → Log if invalid
```

- **Request violations**: Logged as warnings, request continues to handler
- **Response violations**: Logged as warnings, response is sent normally
- **Best for**: Transitioning from shadow to strict, identifying issues without breaking clients

### Shadow Mode

```text
Request → Validate → Execute handler → Validate response
                ↓                              ↓
          Record metrics                Record metrics
           (no logging)                  (no logging)
```

- **Request violations**: Only recorded in metrics, no logs, no impact
- **Response violations**: Only recorded in metrics, no logs, no impact
- **Best for**: Initial deployment to measure contract compliance without any risk

## Recommended Rollout Strategy

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Production Rollout Phases                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Phase 1               Phase 2               Phase 3                       │
│   ┌──────────┐          ┌──────────┐          ┌──────────┐                  │
│   │  SHADOW  │ ──────►  │   WARN   │ ──────►  │  STRICT  │                  │
│   └──────────┘          └──────────┘          └──────────┘                  │
│                                                                             │
│   • Deploy middleware    • Enable logging     • Full enforcement            │
│   • Monitor metrics      • Review violations  • Reject invalid requests     │
│   • Zero risk            • Fix issues found   • Contract is enforced        │
│   • Measure baseline     • No client impact   • Clients must comply         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Step-by-step:**

1. **Deploy with `shadow`** — Add middleware to production with zero risk. Monitor the `cvt_validation_errors_total` metric to understand your baseline.

2. **Analyze metrics** — Use Grafana dashboards to identify which endpoints have the most violations. Common issues: missing required fields, wrong types, undocumented endpoints.

3. **Switch to `warn`** — Enable logging to get detailed error messages. Review logs to understand exactly what's failing and why.

4. **Fix violations** — Either update your OpenAPI spec to match reality, or fix client/server code to match the spec. This is where you decide what the contract should be.

5. **Switch to `strict`** — Full enforcement. Invalid requests are rejected. Your API contract is now enforced.

## Mode Configuration by SDK

Each SDK uses language-appropriate naming conventions:

| SDK         | Strict                  | Warn                  | Shadow                  |
| ----------- | ----------------------- | --------------------- | ----------------------- |
| **Node.js** | `mode: "strict"`        | `mode: "warn"`        | `mode: "shadow"`        |
| **Python**  | `ValidationMode.STRICT` | `ValidationMode.WARN` | `ValidationMode.SHADOW` |
| **Go**      | `producer.ModeStrict`   | `producer.ModeWarn`   | `producer.ModeShadow`   |
| **Java**    | `ValidationMode.STRICT` | `ValidationMode.WARN` | `ValidationMode.SHADOW` |

### Node.js Example

```typescript
import { createExpressMiddleware } from "@cvt/cvt-sdk/producer";

app.use(
  createExpressMiddleware({
    schemaId: "my-api",
    validator,
    mode: "strict", // or "warn" or "shadow"
  }),
);
```

### Go Example

```go
config := producer.Config{
    SchemaID:  "my-api",
    Validator: validator,
    Mode:      producer.ModeStrict, // or ModeWarn or ModeShadow
}
```

### Python Example

```python
from cvt_sdk.producer import ProducerConfig, ValidationMode

config = ProducerConfig(
    schema_id="my-api",
    validator=validator,
    mode=ValidationMode.STRICT,  # or WARN or SHADOW
)
```

### Java Example

```java
ProducerConfig config = ProducerConfig.builder()
    .schemaId("my-api")
    .validator(myValidator)
    .mode(ValidationMode.STRICT)  // or WARN or SHADOW
    .build();
```

## Metrics Emitted

All modes emit Prometheus metrics for monitoring:

| Metric                            | Description                                         |
| --------------------------------- | --------------------------------------------------- |
| `cvt_validation_requests_total`   | Total validations by schema, endpoint, mode         |
| `cvt_validation_errors_total`     | Validation failures by schema, endpoint, error type |
| `cvt_validation_duration_seconds` | Validation latency histogram                        |

Use these metrics to:

- Track contract compliance over time
- Identify problematic endpoints before enabling strict mode
- Monitor the impact of API changes
- Alert on sudden increases in validation failures

## Response Validation Behavior

**Important:** Response validation can only log, never block.

```text
Client Request
       │
       ▼
   Validate Request ─── Can reject (strict mode)
       │
       ▼
   Your Handler
       │
       ▼
   Send Response ─── Already sent to client!
       │
       ▼
   Validate Response ─── Can only log (too late to block)
```

This is by design: by the time we validate the response, it's already been sent to the client. Response validation helps you detect implementation drift (where your code diverges from your spec) but can't prevent invalid responses from reaching clients.

**To prevent invalid responses:** Validate your response data before sending it, or use typed response builders that enforce the schema at compile time.

---

## Related Documentation

- **[Producer Testing Guide](./producer-testing.mdx)** - Full producer testing workflow
- **[Observability Guide](../operations/observability.md)** - Metrics and monitoring
- **[Configuration Reference](../reference/configuration.md)** - Environment variables
