---
title: Introduction
sidebar_label: Introduction
sidebar_position: 1
description: Get started with the Contract Validator Toolkit (CVT)
slug: /intro
---

# Welcome to CVT

**CVT (Contract Validator Toolkit)** catches API breakage before it reaches production by validating HTTP traffic against the API's OpenAPI spec.

## The problem (Alice, Bob, and last Tuesday)

Alice's **checkout service** calls Bob's **Pets API** to show pet inventory at checkout.

Last Tuesday Bob renamed `status` to `availability` in the Pets API response. Alice's checkout crashed in production because her code still read `status`. Nobody noticed until orders started failing.

CVT would have caught the rename at PR time — before Bob's change ever merged.

## How CVT fixes it

The fix is a two-sided handshake over the **OpenAPI spec** Bob already publishes.

**Bob's side (the producer)**: Bob registers his OpenAPI spec with a shared CVT server as part of his CI. CVT knows exactly what his API promises to return.

**Alice's side (the consumer)**: Alice's tests call CVT to validate every HTTP interaction her service makes against Bob's registered spec. When Alice's CI is green, she also registers with CVT to say "I depend on these endpoints and fields."

**The payoff**: before Bob merges a change, his CI runs `can-i-deploy`. CVT compares Bob's new spec against every registered consumer. Alice reads `status`? Bob renamed it? The deploy is blocked. The rename never ships.

```text
      Alice's checkout                      Bob's Pets API
       (consumer)                            (producer)
            │                                    │
            │ validate interactions              │ register schema
            │ register as consumer               │ run can-i-deploy
            ▼                                    ▼
       ┌───────────────────────────────────────────────┐
       │              CVT Server (shared)              │
       │         Bob's schema = source of truth        │
       │      Registry of Alice's dependencies         │
       └───────────────────────────────────────────────┘
```

## Contract testing in 30 seconds

- **Contract** — the OpenAPI spec an API publishes. Machine-readable promise about paths, params, and response shapes.
- **Producer** — the team that owns an API and its spec. Bob.
- **Consumer** — a service that calls that API. Alice.
- **Contract test** — a test that checks an actual HTTP interaction matches the contract.
- **Breaking change** — a spec change that would make existing consumers fail (removed fields, new required fields, renamed keys, tightened types).
- **`can-i-deploy`** — CVT's safety gate. Compares a new spec version against registered consumers and returns a verdict.

## Pick your starting point

- **Consuming an API?** → [Quick Start](./getting-started/quick-start.mdx) — write your first consumer test in 5 minutes.
- **Own an API?** → [Producer Testing Guide](./guides/producer-testing.mdx) — register your schema and verify your handlers match it.
- **Wiring CI?** → [CanIDeploy](./guides/breaking-changes.mdx#deployment-safety-can-i-deploy) — the safety gate that ties both sides together.

<details>
<summary>New to OpenAPI? (1 minute)</summary>

OpenAPI is a machine-readable format for describing HTTP APIs — paths, methods, parameters, and response shapes. If your API has a Swagger UI page, there's an OpenAPI spec behind it. CVT accepts OpenAPI v3 natively and auto-converts Swagger 2.0.

Learn more: [openapi.io](https://www.openapis.org/).

</details>

<details>
<summary>How is CVT different from Pact and other contract tools?</summary>

Pact is broker-centric and generates contracts from recorded interactions. CVT is **schema-first** — you bring an existing OpenAPI spec and validate against it directly. No broker to host, no contract generation step, no coordination between consumer and producer to author the contract.

If your team already maintains OpenAPI specs (most do), CVT uses them as the contract without an extra layer.

</details>

<details>
<summary>Architecture deep dive</summary>

For system architecture, component design, validation engine internals, and storage layer details, see the [Architecture Documentation](./reference/architecture/index.md).

</details>

## Install

```bash
# Server (Docker — recommended)
docker run -d -p 9550:9550 -p 9551:9551 ghcr.io/sahina/cvt:latest

# SDKs
npm install @sahina/cvt-sdk        # Node.js
pip install cvt-sdk                # Python
go get github.com/sahina/cvt/sdks/go  # Go
# Java — see Installation guide for Maven/Gradle
```

See the [Installation Guide](./getting-started/installation.mdx) for other install methods, verification, and Java setup.
