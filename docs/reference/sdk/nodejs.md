---
title: Node.js SDK
sidebar_label: Node.js
sidebar_position: 2
description: CVT SDK for Node.js and TypeScript
---

# Node.js SDK

## What is the Node.js SDK?

The Node.js SDK provides TypeScript-first contract validation for Node.js applications. It includes HTTP client adapters for automatic validation, producer middleware for Express and Fastify, and a test kit for schema compliance testing.

:::tip SDK Architecture
For information about SDK design patterns, adapter architecture, and cross-language consistency, see [SDK Architecture](../architecture/sdk-architecture.md).
:::

## Installation

```bash
# From local clone (SDK not published to npm)
npm install ./cvt/sdks/node

# Or with pnpm
pnpm add ./cvt/sdks/node
```

## Quick Start

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";

const validator = new ContractValidator("localhost:9550");

// Register a schema from file
await validator.registerSchema("petstore", "./openapi.json");
// Or register from URL:
// await validator.registerSchema("petstore", "https://petstore3.swagger.io/api/v3/openapi.json");

// Validate an interaction
const result = await validator.validate(
  { method: "GET", path: "/pet/123" },
  { statusCode: 200, body: { id: 123, name: "doggie", status: "available" } },
);

console.log(result.valid); // true or false
if (!result.valid) {
  console.error("Errors:", result.errors);
}

// Clean up
validator.close();
```

## API Reference

### ContractValidator

#### Constructor

```typescript
// Simple usage (insecure connection)
new ContractValidator(address?: string)

// With options (TLS and API key)
new ContractValidator(options: ContractValidatorOptions)
```

**Simple usage:**

```typescript
const validator = new ContractValidator("localhost:9550");
```

**With options:**

```typescript
const validator = new ContractValidator({
  address: "localhost:9550",
  tls: {
    enabled: true,
    rootCertPath: "./certs/ca.crt",
  },
  apiKey: "your-api-key",
});
```

| Option             | Type      | Description                                |
| ------------------ | --------- | ------------------------------------------ |
| `address`          | `string`  | Server address (default: `localhost:9550`) |
| `tls.enabled`      | `boolean` | Enable TLS                                 |
| `tls.rootCertPath` | `string`  | Path to CA certificate                     |
| `tls.certPath`     | `string`  | Path to client certificate (for mTLS)      |
| `tls.keyPath`      | `string`  | Path to client private key (for mTLS)      |
| `apiKey`           | `string`  | API key for authentication                 |

#### Methods

##### registerSchema

Registers an OpenAPI schema from a file path or URL.

```typescript
registerSchema(schemaId: string, schemaPath: string): Promise<void>
```

```typescript
// From local file
await validator.registerSchema("petstore", "./openapi.json");

// From URL
await validator.registerSchema(
  "petstore",
  "https://petstore3.swagger.io/api/v3/openapi.json",
);
```

##### registerSchemaWithVersion

Registers a schema with version information for comparison.

```typescript
registerSchemaWithVersion(schemaId: string, schemaPath: string, version: string): Promise<void>
```

##### validate

Validates an HTTP request/response pair against the registered schema.

```typescript
validate(request: ValidationRequest, response: ValidationResponse): Promise<ValidationResult>
```

##### compareSchemas

Compares two schema versions for breaking changes.

```typescript
compareSchemas(schemaId: string, oldVersion?: string, newVersion?: string): Promise<CompareResult>
```

##### generateFixture

Generates test fixtures from the schema.

```typescript
generateFixture(method: string, path: string, options?: GenerateOptions): Promise<GeneratedFixture>
```

##### generateResponse

Generates a response fixture only.

```typescript
generateResponse(method: string, path: string, options?: GenerateOptions): Promise<GeneratedResponse>
```

##### generateRequestBody

Generates a request body fixture for an endpoint.

```typescript
generateRequestBody(method: string, path: string, options?: GenerateOptions): Promise<any>
```

```typescript
const body = await validator.generateRequestBody("POST", "/pet");
console.log(body); // { name: "string", status: "available" }
```

##### listEndpoints

Lists all endpoints in the registered schema.

```typescript
listEndpoints(): Promise<EndpointInfo[]>
```

##### registerConsumer

Registers a consumer with expected interactions.

```typescript
registerConsumer(options: RegisterConsumerOptions): Promise<ConsumerInfo>
```

##### listConsumers

Lists all consumers for a schema.

```typescript
listConsumers(schemaId: string, environment?: string): Promise<ConsumerInfo[]>
```

##### deregisterConsumer

Removes a consumer registration.

```typescript
deregisterConsumer(consumerId: string, schemaId: string, environment?: string): Promise<void>
```

##### canIDeploy

Checks if a schema version can be safely deployed.

```typescript
canIDeploy(schemaId: string, newVersion: string, environment?: string): Promise<CanIDeployResult>
```

##### close

Closes the gRPC connection.

```typescript
close(): void
```

## HTTP Adapters

### Axios Adapter

Automatically validate all Axios requests:

```typescript
import axios from "axios";
import { ContractValidator } from "@sahina/cvt-sdk";
import { createAxiosAdapter } from "@sahina/cvt-sdk/adapters";

const validator = new ContractValidator("localhost:9550");
await validator.registerSchema("petstore", "./openapi.json");

const api = axios.create({ baseURL: "http://petstore-service" });

const adapter = createAxiosAdapter({
  axios: api,
  validator,
  autoValidate: true,
  onValidationFailure: (result) => {
    throw new Error(`Contract violation: ${result.errors.join(", ")}`);
  },
});

// All requests are now validated
const response = await api.get("/pet/123");

// Check captured interactions
const interactions = adapter.getInteractions();
console.log(interactions[0].validationResult?.valid);

// Clean up
adapter.detach();
```

### Fetch Adapter

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";
import { createFetchAdapter } from "@sahina/cvt-sdk/adapters";

const validator = new ContractValidator("localhost:9550");
await validator.registerSchema("petstore", "./openapi.json");

const adapter = createFetchAdapter({
  validator,
  baseURL: "http://petstore-service",
});

// Use the adapter's fetch method
const response = await adapter.fetch("/pet/123");
const data = await response.json();

// Check captured interactions
const interactions = adapter.getInteractions();
```

## Producer Middleware

### Express

```typescript
import express from "express";
import { ContractValidator } from "@sahina/cvt-sdk";
import { createExpressMiddleware } from "@sahina/cvt-sdk/producer";

const app = express();
app.use(express.json());

const validator = new ContractValidator("localhost:9550");
await validator.registerSchema("petstore", "./openapi.json");

app.use(
  createExpressMiddleware({
    schemaId: "petstore",
    validator,
    mode: "strict", // "strict" | "warn" | "shadow"
    excludePaths: ["/health", "/metrics"],
  }),
);

app.get("/pet/:petId", (req, res) => {
  res.json({
    id: parseInt(req.params.petId),
    name: "doggie",
    status: "available",
  });
});
```

### Fastify

```typescript
import Fastify from "fastify";
import { ContractValidator } from "@sahina/cvt-sdk";
import { fastifyProducerPlugin } from "@sahina/cvt-sdk/producer";

const fastify = Fastify();
const validator = new ContractValidator("localhost:9550");
await validator.registerSchema("petstore", "./openapi.json");

fastify.register(fastifyProducerPlugin, {
  schemaId: "petstore",
  validator,
  mode: "strict",
});
```

## Producer Test Kit

Test your API responses against your schema without real consumers:

```typescript
import { ProducerTestKit } from "@sahina/cvt-sdk/producer";

describe("Pet API", () => {
  let testKit: ProducerTestKit;

  beforeAll(async () => {
    testKit = new ProducerTestKit({
      schemaId: "petstore",
      serverAddress: "localhost:9550",
    });
  });

  afterAll(() => testKit.close());

  it("returns valid response for GET /pet/{petId}", async () => {
    const result = await testKit.validateResponse({
      method: "GET",
      path: "/pet/123",
      response: {
        statusCode: 200,
        body: { id: 123, name: "doggie", status: "available" },
      },
    });

    expect(result.valid).toBe(true);
  });

  it("detects invalid response", async () => {
    const result = await testKit.validateResponse({
      method: "GET",
      path: "/pet/123",
      response: {
        statusCode: 200,
        body: { id: "not-a-number" }, // Missing required fields
      },
    });

    expect(result.valid).toBe(false);
    expect(result.errors.length).toBeGreaterThan(0);
  });
});
```

## Auto-Registration

Build consumer registrations from captured mock interactions:

```typescript
import { createMockAdapter } from "@sahina/cvt-sdk/adapters";

// Capture interactions during tests
const mock = createMockAdapter({ validator, cache: true });
await mock.fetch("http://mock.petstore/pet/123");
await mock.fetch("http://mock.petstore/pet", { method: "POST", body: "{}" });

// Build registration options (preview)
const opts = validator.buildConsumerFromInteractions(mock.getInteractions(), {
  consumerId: "order-service",
  consumerVersion: "2.1.0",
  environment: "dev",
  schemaVersion: "1.0.0",
});
console.log(`Would register ${opts.usedEndpoints?.length} endpoints`);

// Or register directly
const info = await validator.registerConsumerFromInteractions(
  mock.getInteractions(),
  {
    consumerId: "order-service",
    consumerVersion: "2.1.0",
    environment: "dev",
    schemaVersion: "1.0.0",
  },
);
```

### buildConsumerFromInteractions

Builds consumer registration options from captured interactions without registering.

```typescript
buildConsumerFromInteractions(
  interactions: CapturedInteraction[],
  config: AutoRegisterConfig
): RegisterConsumerOptions
```

### registerConsumerFromInteractions

Registers a consumer from captured interactions (combines `buildConsumerFromInteractions` + `registerConsumer`).

```typescript
registerConsumerFromInteractions(
  interactions: CapturedInteraction[],
  config: AutoRegisterConfig
): Promise<ConsumerInfo>
```

## TLS Configuration

```typescript
const validator = new ContractValidator({
  address: "localhost:9550",
  tls: {
    enabled: true,
    rootCertPath: "./certs/ca.crt",
  },
});
```

### Mutual TLS (mTLS)

```typescript
const validator = new ContractValidator({
  address: "localhost:9550",
  tls: {
    enabled: true,
    rootCertPath: "./certs/ca.crt",
    certPath: "./certs/client.crt",
    keyPath: "./certs/client.key",
  },
});
```

## API Key Authentication

```typescript
const validator = new ContractValidator({
  address: "localhost:9550",
  apiKey: "your-api-key",
});
```

## TypeScript Types

The SDK exports all TypeScript types:

```typescript
import {
  ContractValidator,
  ContractValidatorOptions,
  TLSOptions,
  ValidationRequest,
  ValidationResponse,
  ValidationResult,
  BreakingChange,
  CompareResult,
  GenerateOptions,
  GeneratedFixture,
  GeneratedRequest,
  GeneratedResponse,
  EndpointInfo,
  RegisterConsumerOptions,
  ConsumerInfo,
  CanIDeployResult,
} from "@sahina/cvt-sdk";
```

## Error Handling

```typescript
try {
  await validator.registerSchema("petstore", "./openapi.json");
} catch (error) {
  if (error.code === "UNAVAILABLE") {
    console.error("CVT server is not reachable");
  } else {
    console.error("Registration failed:", error.message);
  }
}

// Validation errors are returned in the result, not thrown
const result = await validator.validate(request, response);
if (!result.valid) {
  console.error("Validation errors:", result.errors);
}
```

## Related Documentation

- **[Consumer Testing Guide](../../guides/consumer-testing.mdx)** - Testing your API integrations
- **[Producer Testing Guide](../../guides/producer-testing.mdx)** - Validating your APIs
- **[API Reference](../api.mdx)** - Full gRPC API documentation
