---
title: Node.js SDK
sidebar_label: Node.js
sidebar_position: 2
description: CVT SDK for Node.js and TypeScript
---

# Node.js SDK

The Node.js SDK provides TypeScript-first contract validation for Node.js applications.

## Installation

```bash
# npm
npm install @cvt/cvt-sdk

# yarn
yarn add @cvt/cvt-sdk
```

## Quick Start

```typescript
import { ContractValidator } from '@cvt/cvt-sdk';

const validator = new ContractValidator('localhost:9550');

// Register a schema
await validator.registerSchema('user-api', fs.readFileSync('openapi.json', 'utf-8'));

// Validate an interaction
const result = await validator.validate(
  { method: 'GET', path: '/users/123' },
  { statusCode: 200, body: JSON.stringify({ id: '123', name: 'John' }) }
);

console.log(result.valid); // true or false
```

## API Reference

### ContractValidator

#### Constructor

```typescript
new ContractValidator(address: string, options?: ValidatorOptions)
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `address` | `string` | Server address (e.g., `localhost:9550`) |
| `options.tls` | `TLSConfig` | TLS configuration |
| `options.metadata` | `Record<string, string>` | gRPC metadata (e.g., API key) |

#### Methods

##### registerSchema

```typescript
registerSchema(schemaId: string, content: string, version?: string): Promise<RegisterSchemaResponse>
```

##### validate

```typescript
validate(request: RequestData, response: ResponseData): Promise<ValidationResult>
```

##### registerConsumer

```typescript
registerConsumer(options: RegisterConsumerOptions): Promise<RegisterConsumerResponse>
```

##### listConsumers

```typescript
listConsumers(schemaId: string, environment?: string): Promise<ConsumerInfo[]>
```

##### deregisterConsumer

```typescript
deregisterConsumer(consumerId: string, schemaId: string, environment?: string): Promise<void>
```

##### compareSchemas

```typescript
compareSchemas(schemaId: string, oldVersion: string, newVersion: string): Promise<CompareSchemasResponse>
```

##### canIDeploy

```typescript
canIDeploy(options: CanIDeployOptions): Promise<CanIDeployResponse>
```

##### generateFixture

```typescript
generateFixture(options: GenerateFixtureOptions): Promise<GeneratedFixture>
```

##### close

```typescript
close(): void
```

## HTTP Adapters

### Axios Adapter

Automatically validate all Axios requests:

```typescript
import axios from 'axios';
import { ContractValidator, createAxiosAdapter } from '@cvt/cvt-sdk';

const validator = new ContractValidator('localhost:9550');
await validator.registerSchema('user-api', schema);

const api = axios.create({ baseURL: 'http://user-service' });

createAxiosAdapter({
  axios: api,
  validator,
  schemaId: 'user-api',
  autoValidate: true,
  onValidationFailure: (result) => {
    throw new Error(`Contract violation: ${result.errors.join(', ')}`);
  }
});

// All requests are now validated
const user = await api.get('/users/123');
```

### Fetch Adapter

```typescript
import { createFetchAdapter } from '@cvt/cvt-sdk';

const validatedFetch = createFetchAdapter({
  validator,
  schemaId: 'user-api',
  baseUrl: 'http://user-service'
});

const response = await validatedFetch('/users/123');
```

## Producer Middleware

### Express

```typescript
import express from 'express';
import { createExpressMiddleware } from '@cvt/cvt-sdk/producer';

const app = express();

app.use(createExpressMiddleware({
  schemaId: 'my-api',
  validator,
  mode: 'strict', // 'strict' | 'warn' | 'shadow'
  excludePaths: ['/health', '/metrics']
}));

app.get('/users/:id', (req, res) => {
  res.json({ id: req.params.id, name: 'John' });
});
```

### Fastify

```typescript
import Fastify from 'fastify';
import { createFastifyPlugin } from '@cvt/cvt-sdk/producer';

const fastify = Fastify();

fastify.register(createFastifyPlugin({
  schemaId: 'my-api',
  validator,
  mode: 'strict'
}));
```

## Producer Test Kit

```typescript
import { ProducerTestKit } from '@cvt/cvt-sdk/producer';

describe('User API', () => {
  let testKit: ProducerTestKit;

  beforeAll(async () => {
    testKit = new ProducerTestKit({
      schemaId: 'user-api',
      serverAddress: 'localhost:9550'
    });
  });

  afterAll(() => testKit.close());

  it('returns valid response', async () => {
    const result = await testKit.validateResponse({
      method: 'GET',
      path: '/users/123',
      statusCode: 200,
      body: { id: '123', name: 'John' }
    });

    expect(result.valid).toBe(true);
  });
});
```

## TLS Configuration

```typescript
import * as fs from 'fs';

const validator = new ContractValidator('localhost:9550', {
  tls: {
    rootCerts: fs.readFileSync('./certs/ca.crt'),
    // For mTLS:
    privateKey: fs.readFileSync('./certs/client.key'),
    certChain: fs.readFileSync('./certs/client.crt')
  }
});
```

## API Key Authentication

```typescript
const validator = new ContractValidator('localhost:9550', {
  metadata: {
    'x-api-key': 'your-api-key'
  }
});
```

## TypeScript Types

The SDK exports all TypeScript types:

```typescript
import {
  ContractValidator,
  RequestData,
  ResponseData,
  ValidationResult,
  RegisterConsumerOptions,
  ConsumerInfo,
  BreakingChange,
  CanIDeployResponse
} from '@cvt/cvt-sdk';
```

## Error Handling

```typescript
try {
  await validator.registerSchema('my-api', schema);
} catch (error) {
  if (error.code === 'INVALID_SCHEMA') {
    console.error('Schema is not valid OpenAPI');
  } else if (error.code === 'UNAVAILABLE') {
    console.error('CVT server is not reachable');
  }
}
```

## Related Documentation

- **[Consumer Testing Guide](../../guides/consumer-testing.mdx)** - Testing your API integrations
- **[Producer Testing Guide](../../guides/producer-testing.md)** - Validating your APIs
- **[API Reference](../api.md)** - Full gRPC API documentation
