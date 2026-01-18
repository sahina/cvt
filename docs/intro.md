---
title: Introduction
sidebar_label: Introduction
sidebar_position: 1
description: Get started with the Contract Validator Toolkit (CVT)
slug: /intro
---

# Welcome to CVT

**Contract Validator Toolkit (CVT)** is a consumer-based contract validation platform for OpenAPI v2/v3 specifications. It helps teams ensure their API consumers and producers communicate correctly by validating HTTP interactions against published contracts.

## What is Contract Testing?

Contract testing validates API interactions against a published contract (OpenAPI specification). Unlike integration tests that require real services, contract tests verify that:

- **Consumers** send valid requests and handle responses correctly
- **Producers** return responses that match their published specification

## Key Features

| Feature | Description |
|---------|-------------|
| **Schema Validation** | Register OpenAPI v2/v3 schemas and validate request/response pairs |
| **Consumer Testing** | Validate your HTTP calls against upstream API contracts |
| **Producer Testing** | Ensure your API implementation matches your specification |
| **Breaking Change Detection** | Compare schema versions to detect incompatible changes |
| **Safe Deployments** | Use CanIDeploy to verify changes won't break consumers |
| **Multi-language SDKs** | Native support for Node.js, Python, Go, and Java |

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
go get github.com/sahina/cvt/sdks/go
```

### 3. Validate an Interaction

```typescript
import { CVTClient } from '@cvt/cvt-sdk';

const client = new CVTClient('localhost:9550');

// Register your schema
await client.registerSchema({
  schemaId: 'user-api',
  content: openApiSpec,
});

// Validate an interaction
const result = await client.validateInteraction({
  schemaId: 'user-api',
  request: {
    method: 'GET',
    path: '/users/123',
    headers: { 'Content-Type': 'application/json' },
  },
  response: {
    statusCode: 200,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: '123', name: 'John' }),
  },
});

console.log(result.valid); // true or false
```

## Architecture

CVT runs as a gRPC service that can be deployed via Docker or run locally:

```
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

## Next Steps

- **[Use Cases](./use-cases)** - Common scenarios with step-by-step guides
- **[Consumer Testing Guide](./consumer-testing-guide)** - Test your API consumers
- **[Producer Testing Guide](./producer-testing)** - Validate your API producers
- **[Development Guide](./DEVELOPMENT)** - Set up your local development environment
