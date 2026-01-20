---
title: Introduction
sidebar_label: Introduction
sidebar_position: 1
description: Get started with the Contract Validator Toolkit (CVT)
slug: /intro
---

# Welcome to CVT

**Contract Validator Toolkit (CVT)** is a consumer- and producer-based contract validation platform for OpenAPI v2/v3 specifications.

It helps teams ensure their API consumers and producers communicate correctly by validating HTTP interactions against published contracts.

## What is Contract Testing?

Contract testing validates API interactions against a published contract (OpenAPI specification). Unlike integration tests that require real services, contract tests verify that:

- **Consumers** send valid requests and handle responses correctly
- **Producers** return responses that match their published specification

## Key Features

| Feature                       | Description                                                        |
| ----------------------------- | ------------------------------------------------------------------ |
| **Schema Validation**         | Register OpenAPI v2/v3 schemas and validate request/response pairs |
| **Consumer Testing**          | Validate your HTTP calls against upstream API contracts            |
| **Producer Testing**          | Ensure your API implementation matches your specification          |
| **Breaking Change Detection** | Compare schema versions to detect incompatible changes             |
| **Safe Deployments**          | Use CanIDeploy to verify changes won't break consumers             |
| **Multi-language SDKs**       | Native support for Node.js, Python, Go, and Java                   |

## Quick Start

### 1. Start the CVT Server

```bash
# Using Docker (recommended)
make up

# Or run locally
make run-server
```

### 2. Install an SDK

```bash
# Node.js
pnpm add @cvt/cvt-sdk

# Python
pip install cvt-sdk

# Go
go get github.com/sahina/cvt/sdks/go/cvt
```

### 3. Validate an Interaction

```typescript
import { ContractValidator } from "@cvt/cvt-sdk";

const validator = new ContractValidator("localhost:9550");

// Register your schema
await validator.registerSchema("user-api", "./openapi.json");

// Validate an interaction
const result = await validator.validate(
  {
    method: "GET",
    path: "/users/123",
    headers: { "Content-Type": "application/json" },
  },
  {
    statusCode: 200,
    headers: { "Content-Type": "application/json" },
    body: { id: "123", name: "John" },
  },
);

console.log(result.valid); // true or false
validator.close();
```

## Architecture

CVT runs as a gRPC service that can be deployed via Docker or run locally:

```text
┌─────────────────┐     gRPC      ┌─────────────────┐
│   Your Tests    │ ────────────► │   CVT Server    │
│  (SDK Client)   │               │   (Port 9550)   │
└─────────────────┘               └─────────────────┘
                                          │
                                          ▼
                                  ┌─────────────────┐
                                  │ Schema Registry │
                                  │  + Validation   │
                                  └─────────────────┘
```

## Documentation Structure

### Getting Started

- **[Installation](./getting-started/installation.md)** - Install the server and SDKs
- **[Quick Start](./getting-started/quick-start.mdx)** - Your first contract test

### Guides

- **[Consumer Testing](./guides/consumer-testing.md)** - Test your API integrations
- **[Producer Testing](./guides/producer-testing.md)** - Validate your APIs
- **[Breaking Changes](./guides/breaking-changes.md)** - Detect schema incompatibilities
- **[Validation Modes](./guides/validation-modes.md)** - Configure validation behavior

### Reference

- **[API Reference](./reference/api.md)** - gRPC API documentation
- **[CLI Reference](./reference/cli.md)** - Command-line interface
- **[Configuration](./reference/configuration.md)** - Environment variables
- **[SDK Documentation](./reference/sdk/)** - Language-specific guides

### Operations

- **[Observability](./operations/observability.md)** - Metrics, logging, and dashboards

### Development

- **[Contributing](./development/contributing.md)** - Local development setup

## Next Steps

Choose your path based on your role:

**API Consumer?** Start with the [Consumer Testing Guide](./guides/consumer-testing.md) to learn how to validate your API integrations.

**API Producer?** Check out the [Producer Testing Guide](./guides/producer-testing.md) to ensure your API matches its specification.

**Setting up CVT?** Follow the [Installation Guide](./getting-started/installation.md) for server setup and configuration.
