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

## Can I use CVT without a running server?

For some workflows, yes. The CLI commands `cvt validate` and `cvt compare` work entirely offline against local schema files or URLs, and the embedded Go library ([`pkg/cvt`](https://pkg.go.dev/github.com/sahina/cvt/pkg/cvt)) can be used directly in Go code. However, the SDK-based workflow — including `registerSchema`, `validate`, consumer registration, and `can-i-deploy` — requires a running server since SDKs communicate over gRPC.
