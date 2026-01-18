---
title: Consumer Testing Guide
sidebar_label: Consumer Testing
sidebar_position: 1
description: Complete guide to testing your API consumers with CVT
---

# Consumer Testing Guide

This guide covers how to test your service's API integrations using CVT. Consumer testing validates that your HTTP calls to upstream APIs match their published OpenAPI contracts.

## Overview

Consumer testing answers the question: **"Am I calling this API correctly?"**

When your service depends on external APIs, you need confidence that:
- Your requests are properly formatted
- You handle responses correctly
- You won't break when upstream APIs change

```
┌─────────────────────┐     HTTP      ┌─────────────────────┐
│   Your Service      │ ────────────► │   Upstream API      │
│   (Consumer)        │               │   (Producer)        │
└─────────────────────┘               └─────────────────────┘
         │
         │ Register & Validate
         ▼
┌─────────────────────┐
│    CVT Server       │
│    + Schema         │
└─────────────────────┘
```

---

## Quick Start

### 1. Start the CVT Server

```bash
# Using Docker (recommended)
make up

# Or run locally
make run-server
```

### 2. Write Your First Contract Test

```typescript
import { ContractValidator } from '@cvt/cvt-sdk';

describe('User Service Contract', () => {
  let validator: ContractValidator;

  beforeAll(async () => {
    validator = new ContractValidator('localhost:9550');
    await validator.registerSchema('user-service', './contracts/user-api.json');
  });

  afterAll(() => validator.close());

  it('GET /users/{id} returns valid response', async () => {
    const result = await validator.validate(
      { method: 'GET', path: '/users/123' },
      { statusCode: 200, body: JSON.stringify({ id: '123', name: 'John' }) }
    );
    expect(result.valid).toBe(true);
  });
});
```

---

## Validation Approaches

CVT supports multiple ways to validate your API interactions.

### Approach 1: Manual Validation

Build request/response objects and validate them directly:

```typescript
import { ContractValidator } from '@cvt/cvt-sdk';

const validator = new ContractValidator('localhost:9550');
await validator.registerSchema('user-api', './user-api.json');

// Validate a specific interaction
const result = await validator.validate(
  {
    method: 'GET',
    path: '/users/123',
    headers: { 'Accept': 'application/json' }
  },
  {
    statusCode: 200,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: '123', name: 'John Doe', email: 'john@example.com' })
  }
);

if (!result.valid) {
  console.error('Validation errors:', result.errors);
}
```

### Approach 2: HTTP Adapter (Recommended)

Wrap your HTTP client for automatic validation of all requests/responses:

```typescript
import axios from 'axios';
import { ContractValidator, createAxiosAdapter } from '@cvt/cvt-sdk';

const validator = new ContractValidator('localhost:9550');
await validator.registerSchema('user-service', './contracts/user-api.json');

const api = axios.create({ baseURL: 'http://user-service' });

// Wrap axios - all traffic is now validated automatically
createAxiosAdapter({
  axios: api,
  validator,
  schemaId: 'user-service',
  autoValidate: true,
  onValidationFailure: (result) => {
    throw new Error(`Contract violation: ${result.errors.join(', ')}`);
  }
});

// Use normally - validation happens transparently
const user = await api.get('/users/123');
```

### Approach 3: Mock Client

Use the MockingRoundTripper for tests without a real API:

```go
// Create mock client that auto-generates responses from schema
mock := adapters.NewMock(validator, adapters.WithCache())
mockClient := mock.Client()

// Make requests - responses are generated from OpenAPI schema
req, _ := http.NewRequest("GET", "http://mock.user-api/users/456", nil)
resp, _ := mockClient.Do(req)

// Check recorded interactions for consumer registration
interactions := mock.GetInteractions()
```

Benefits:
- No real API endpoint needed
- Responses match schema exactly
- Interactions captured for auto-registration

---

## Consumer Registration

Register your service as a consumer to enable deployment safety checks.

### Auto-Registration (Recommended)

Register from captured test interactions - endpoints and fields are extracted automatically:

```go
// Use interactions captured from MockingRoundTripper or HTTP adapter
interactions := mock.GetInteractions()

consumerInfo, _ := validator.RegisterConsumerFromInteractions(ctx, interactions, cvt.AutoRegisterConfig{
    ConsumerID:      "order-service",
    ConsumerVersion: "2.1.0",
    Environment:     "dev",
    SchemaVersion:   "1.0.0",
    // SchemaID is auto-extracted from URL: http://mock.user-api/... -> "user-api"
})
```

**Benefits:**
- No manual endpoint specification needed
- Fields extracted from actual usage
- Always in sync with test behavior
- Less maintenance overhead

### Manual Registration

For fine-grained control, specify endpoints explicitly:

```typescript
await validator.registerConsumer({
  consumerId: 'order-service',
  consumerVersion: '2.1.0',
  schemaId: 'user-api',
  schemaVersion: '1.0.0',
  environment: 'dev',
  usedEndpoints: [
    {
      method: 'GET',
      path: '/users/{id}',
      usedFields: ['id', 'name', 'email']
    },
    {
      method: 'POST',
      path: '/users',
      usedFields: ['id']
    }
  ]
});
```

This tells CVT:
- **Who you are**: `order-service` v2.1.0
- **What you depend on**: `user-api` v1.0.0
- **What you use**: Specific endpoints and fields

### Listing Consumers

Query all consumers registered for a schema:

```typescript
const consumers = await validator.listConsumers('user-api', 'dev');
console.log(`${consumers.length} services depend on user-api`);
```

### Deregistering Consumers

Remove a consumer registration when no longer needed:

```typescript
await validator.deregisterConsumer('order-service', 'user-api', 'dev');
```

---

## Deployment Safety (can-i-deploy)

Before the upstream API deploys a new version, they can check if it's safe:

```bash
# CLI
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod
```

```typescript
// SDK
const result = await validator.canIDeploy('user-api', '2.0.0', 'dev');

if (!result.safeToDeploy) {
  console.error('UNSAFE:', result.summary);
  for (const consumer of result.affectedConsumers) {
    if (consumer.willBreak) {
      console.error(`- ${consumer.consumerId} will break`);
    }
  }
}
```

See [Breaking Changes Guide](./breaking-changes.md) for more details.

---

## SDK Examples

### Node.js

```typescript
import { ContractValidator } from '@cvt/cvt-sdk';

const validator = new ContractValidator('localhost:9550');

// Register schema
await validator.registerSchema('my-api', fs.readFileSync('openapi.json', 'utf-8'));

// Validate interaction
const result = await validator.validate(
  { method: 'GET', path: '/users/1' },
  { statusCode: 200, body: '{"id": 1}' }
);
```

### Python

```python
from cvt_sdk import ContractValidator

validator = ContractValidator('localhost:9550')

# Register schema
validator.register_schema('my-api', open('openapi.json').read())

# Validate interaction
result = validator.validate(
    request={'method': 'GET', 'path': '/users/1'},
    response={'status_code': 200, 'body': '{"id": 1}'}
)
```

### Go

```go
import "github.com/sahina/cvt/sdks/go/cvt"

client, _ := cvt.NewValidator("localhost:9550")

// Register schema
client.RegisterSchema(ctx, "my-api", schemaContent)

// Validate interaction
result, _ := client.Validate(ctx, cvt.Request{
    Method: "GET",
    Path:   "/users/1",
}, cvt.Response{
    StatusCode: 200,
    Body:       `{"id": 1}`,
})
```

### Java

```java
import com.cvt.sdk.ContractValidator;

ContractValidator validator = new ContractValidator("localhost:9550");

// Register schema
validator.registerSchema("my-api", schemaContent);

// Validate interaction
ValidationResult result = validator.validate(
    new Request("GET", "/users/1"),
    new Response(200, "{\"id\": 1}")
);
```

---

## Testing Scenarios

### Valid Interaction

```typescript
it('returns valid response with all required fields', async () => {
  const result = await validator.validate(
    { method: 'GET', path: '/users/123' },
    {
      statusCode: 200,
      body: JSON.stringify({ id: '123', name: 'John', email: 'john@example.com' })
    }
  );
  expect(result.valid).toBe(true);
});
```

### Invalid Response (Missing Fields)

```typescript
it('catches missing required fields', async () => {
  const result = await validator.validate(
    { method: 'GET', path: '/users/123' },
    {
      statusCode: 200,
      body: JSON.stringify({ id: '123' }) // missing name, email
    }
  );
  expect(result.valid).toBe(false);
  expect(result.errors).toContain('name');
});
```

### Error Responses

```typescript
it('validates 404 error response format', async () => {
  const result = await validator.validate(
    { method: 'GET', path: '/users/nonexistent' },
    {
      statusCode: 404,
      body: JSON.stringify({ error: 'User not found' })
    }
  );
  expect(result.valid).toBe(true);
});
```

---

## CI/CD Integration

### GitHub Actions

```yaml
name: Contract Tests

on: [push, pull_request]

jobs:
  contract-tests:
    runs-on: ubuntu-latest
    services:
      cvt:
        image: ghcr.io/cvt/cvt-server:latest
        ports:
          - 9550:9550

    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install dependencies
        run: npm ci

      - name: Run contract tests
        run: npm test

      - name: Register consumer
        if: github.ref == 'refs/heads/main'
        run: |
          npm run register-consumer -- \
            --id my-service \
            --version ${{ github.sha }} \
            --schema user-api \
            --env staging
```

### Pre-Deploy Safety Check

```yaml
deploy:
  needs: [contract-tests]
  steps:
    - name: Check if upstream is safe
      run: |
        # This checks if any upstream dependencies have breaking changes
        cvt can-i-deploy \
          --schema user-api \
          --version latest \
          --env prod \
          --json > check.json

        if [ $(jq '.safeToDeploy' check.json) != "true" ]; then
          echo "Upstream API has breaking changes!"
          exit 1
        fi
```

---

## Troubleshooting

### "Failed to create validator"

Make sure CVT server is running:

```bash
make up
# or
docker-compose up -d cvt-server
```

### "Schema not found"

Ensure the schema is registered before validation:

```typescript
await validator.registerSchema('my-api', schemaContent);
// Then validate...
```

### "Path not found"

Check that your path matches the OpenAPI spec:
- Use actual path values: `/users/123` not `/users/{id}`
- Ensure the HTTP method matches

### Connection refused

Default server address is `localhost:9550`. Configure if different:

```typescript
const validator = new ContractValidator('cvt.internal:9550');
```

---

## Next Steps

- **[Producer Testing Guide](./producer-testing.md)** - Validate your own APIs
- **[Breaking Changes Guide](./breaking-changes.md)** - Understand schema compatibility
- **[Validation Modes](./validation-modes.md)** - Configure validation behavior
- **[API Reference](../reference/api.md)** - Full API documentation
