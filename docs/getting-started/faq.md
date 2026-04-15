---
title: FAQ
sidebar_label: FAQ
sidebar_position: 3
description: Non-obvious questions about CVT
---

## Does the CVT server need to run in production?

No. CVT is a test-time tool — the server runs in CI or on your local machine during development. Your production services never talk to it. Think of it like a test database: it exists to support your test suite, not your live traffic.

## How is this different from Pact?

Pact is broker-centric and generates contracts from recorded interactions. CVT is schema-first — you bring an existing OpenAPI spec and validate against it directly. There's no broker to host, no contract generation step, and consumers don't need to coordinate with producers during test authoring. If you already have OpenAPI specs (most teams do), CVT lets you use them as the contract without an additional layer.

## What happens when my API returns fields not in the schema?

By default, extra fields pass validation. This is because OpenAPI's `additionalProperties` defaults to `true`, so undocumented fields are technically allowed. If you want strict validation that rejects unexpected fields, set `additionalProperties: false` on your response schemas.

## Can I validate against a Swagger 2.0 spec?

Yes. CVT auto-converts Swagger 2.0 to OpenAPI 3.x at registration time. You don't need to convert your spec manually — just pass it to `registerSchema` as-is.

## Do I need to register the schema before every test run?

`registerSchema` is idempotent — calling it with the same ID and content is a no-op. Most teams call it once in `beforeAll` or test setup. If you enable [persistent storage](../reference/configuration.mdx), schemas also survive server restarts, so re-registration on startup is fast (it just confirms the schema is already there).

## Should the producer provide mocks to consumers?

Yes — and CVT already does this. Every SDK ships a **[mock adapter](../reference/architecture/sdk-architecture.md#mock-adapters)** that generates HTTP responses directly from the producer's OpenAPI schema, with no real API call needed. The producer publishes their schema; the consumer points CVT's mock adapter at it and gets back spec-compliant responses automatically.

This is sometimes called "provider-generated stubs." The key advantage is that the mock can't drift from the contract — it's generated from the same schema CVT validates against. Consumers don't need to hand-write test doubles or guess at response shapes.

| Language | Mock Adapter            | Drop-in for          |
| -------- | ----------------------- | -------------------- |
| Node.js  | `createMockFetch()`     | `fetch()`            |
| Python   | `create_mock_session()` | `requests.Session`   |
| Go       | `NewMockClient()`       | `http.Client`        |
| Java     | `MockInterceptor`       | OkHttp `Interceptor` |

See [Approach 3: Mock Client](../guides/consumer-testing.mdx#approach-3-mock-client) in the Consumer Testing Guide for full setup examples.

## Are CVT mock adapters a full mock server?

No — they're in-process test doubles, not a standalone HTTP server. The mock adapter intercepts HTTP calls inside your test and returns schema-generated responses without network overhead. This makes tests fast and deterministic. See the [adapter pattern](../reference/architecture/sdk-architecture.md#adapter-pattern) docs for how this works under the hood.

If you need an actual running HTTP mock server (e.g., for frontend development, manual testing, or non-SDK languages), use the [`cvt mock`](../reference/cli.mdx#mock) CLI command:

```bash
cvt mock --schema ./openapi.json
# Mock server running on http://localhost:8080
```

It supports multiple schemas, request validation, hot-reload, latency simulation, and CORS headers out of the box. See the [CLI Reference](../reference/cli.mdx#mock) for all options.

For automated testing, the in-process mock adapters remain the recommended approach — they're simpler to set up, faster to run, and tightly integrated with CVT's validation pipeline.

## How can I trust mock responses to match the real API?

Mock responses are structurally correct — they match the schema's types, required fields, and response codes — but they don't capture business logic. A mock for `GET /pet/123` will return a valid pet object, but it won't know that pet 123 is a dog named Max.

This is by design. Mock adapters solve the "does my consumer code handle the response shape correctly?" question. For **"does the producer actually behave this way?"** you need [producer testing](../guides/producer-testing.mdx#schema-compliance-testing) and [contract validation](../guides/consumer-testing.mdx#approach-2-http-adapter-recommended).

The combination is powerful: mock adapters give consumers fast, offline feedback during development, and contract validation catches real integration issues in CI. Neither replaces the other.

## Can I use CVT without a running server?

For some workflows, yes:

- **CLI offline commands** (**server not needed**) — [`cvt validate`](../reference/cli.mdx#validate), [`cvt compare`](../reference/cli.mdx#compare), [`cvt generate`](../reference/cli.mdx#generate), and [`cvt mock`](../reference/cli.mdx#mock) work without a CVT server. They operate on local schema files directly (or fetch from a URL if one is provided).
- **Embedded Go library** (**server not needed**) — [`pkg/cvt`](https://pkg.go.dev/github.com/sahina/cvt/pkg/cvt) can be imported directly into Go code for in-process validation without any network calls.
- **SDK-based workflow** (**server needed**) — [`registerSchema`](../reference/sdk/index.mdx#initialization), [`validate`](../reference/sdk/index.mdx#validation), [consumer registration](../guides/consumer-testing.mdx), and [`can-i-deploy`](../reference/cli.mdx#can-i-deploy) require a running server since SDKs communicate over [gRPC](../reference/api.mdx#service-methods).
