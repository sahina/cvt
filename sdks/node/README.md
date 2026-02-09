# Contract Validator Toolkit (CVT) - Node.js SDK

The **CVT Node.js SDK** allows you to validate HTTP interactions (requests and responses) against OpenAPI schemas using the CVT gRPC service.

> **Status**: Fully Implemented

## Installation

**Note**: This package is currently for internal/development use.

To install from the local source:

```bash
cd sdks/node
npm install
npm run build
cd ../your-project
npm install ../sdks/node
```

For the published version via GitHub Packages:

```bash
npm install @sahina/cvt-sdk
```

## Usage

### 1. Initialize and Register Schema

You can register a schema from a local file or a URL.

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";
import * as path from "path";

const validator = new ContractValidator();

// Register from local file
await validator.registerSchema(
  "my-schema",
  path.resolve(__dirname, "openapi.json"),
);

// OR Register from URL
await validator.registerSchema(
  "petstore",
  "https://petstore.swagger.io/v2/swagger.json",
);
```

### 2. Validate Interactions

You can validate requests and responses. The `validate` method supports generic types for strong typing, or you can use it without types.

#### Strong Typing (Recommended)

```typescript
import {
  ContractValidator,
  ValidationRequest,
  ValidationResponse,
} from "@sahina/cvt-sdk";

interface User {
  username: string;
  email: string;
}

const request: ValidationRequest<User> = {
  method: "POST",
  path: "/users",
  headers: { "content-type": "application/json" },
  body: { username: "alice", email: "alice@example.com" },
};

const response: ValidationResponse = {
  statusCode: 201,
};

const result = await validator.validate<User>(request, response);

if (result.valid) {
  console.log("✅ Valid interaction");
} else {
  console.error("❌ Validation errors:", result.errors);
}
```

#### Untyped Usage

```typescript
const result = await validator.validate(
  {
    method: "GET",
    path: "/pet/123",
  },
  {
    statusCode: 200,
    body: { id: 123, name: "Fluffy" },
  },
);
```

## HTTP Adapter (Axios)

The SDK includes an Axios adapter for automatic HTTP traffic validation:

```typescript
import axios from "axios";
import { ContractValidator, createAxiosAdapter } from "@sahina/cvt-sdk";

const validator = new ContractValidator();
await validator.registerSchema("petstore", "./openapi.json");

const api = axios.create({ baseURL: "https://api.example.com" });

// Auto-validate all requests
const adapter = createAxiosAdapter({
  axios: api,
  validator,
  schemaId: "petstore",
  autoValidate: true,
  excludePaths: ["/health", "/metrics"],
  onValidationFailure: (result, interaction) => {
    console.error("Validation failed:", result.errors);
  },
});

// All requests are now automatically validated
const response = await api.post("/pets", { name: "Fluffy" });
```

### Adapter Options

- `autoValidate`: Enable/disable automatic validation (default: true)
- `includePaths`: Array of paths/regex to include
- `excludePaths`: Array of paths/regex to exclude
- `onValidationFailure`: Custom error handler
- `getInteractions()`: Retrieve captured interactions
- `clearInteractions()`: Reset captured data

## Producer Validation (Server-Side Middleware)

Validate incoming requests and outgoing responses against your OpenAPI contract on the server side.

> **Full documentation:** See [Validation Modes](../../docs/guides/validation-modes.mdx) for detailed behavior, rollout strategy, and metrics information.

### Validation Modes

| Mode       | Request Violation | Response Violation | Use Case               |
| ---------- | ----------------- | ------------------ | ---------------------- |
| `"strict"` | Reject with 400   | Log error          | Production enforcement |
| `"warn"`   | Log, continue     | Log, continue      | Gradual rollout        |
| `"shadow"` | Metrics only      | Metrics only       | Initial deployment     |

**Recommended rollout:** `shadow` → `warn` → `strict`. See [Recommended Rollout Strategy](../../docs/guides/validation-modes.mdx#recommended-rollout-strategy).

### Express Middleware

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";
import { createExpressMiddleware } from "@sahina/cvt-sdk/producer";

const validator = new ContractValidator();
await validator.registerSchema("my-api", "./openapi.json");

app.use(
  createExpressMiddleware({
    schemaId: "my-api",
    validator,
    mode: "strict",
    excludePaths: ["/health", "/metrics"],
  }),
);
```

### Fastify Plugin

```typescript
import { fastifyProducerPlugin } from "@sahina/cvt-sdk/producer";

fastify.register(fastifyProducerPlugin, {
  schemaId: "my-api",
  validator,
  mode: "strict",
});
```

### Configuration Options

| Option              | Type        | Description                                |
| ------------------- | ----------- | ------------------------------------------ |
| `schemaId`          | `string`    | Schema ID to validate against              |
| `validator`         | `Validator` | ContractValidator instance                 |
| `mode`              | `string`    | `strict`, `warn`, or `shadow`              |
| `excludePaths`      | `string[]`  | Paths to skip validation (e.g., `/health`) |
| `includePaths`      | `string[]`  | Only validate matching paths               |
| `validateResponse`  | `boolean`   | Enable response validation (default: true) |
| `onValidationError` | `function`  | Custom error handler callback              |

## Breaking Change Detection

Detect breaking changes between OpenAPI schema versions before deployment:

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";

const validator = new ContractValidator();

// Register both schema versions
await validator.registerSchemaWithVersion(
  "my-api",
  "./openapi-v1.json",
  "1.0.0",
);
await validator.registerSchemaWithVersion(
  "my-api",
  "./openapi-v2.json",
  "2.0.0",
);

// Compare versions
const result = await validator.compareSchemas("my-api", "1.0.0", "2.0.0");

if (!result.compatible) {
  console.log("Breaking changes detected:");
  result.breakingChanges.forEach((change) => {
    console.log(`- [${change.type}] ${change.description}`);
    if (change.path) {
      console.log(`  Path: ${change.method} ${change.path}`);
    }
  });
  process.exit(1); // Fail CI build
}
```

### Breaking Change Types

| Type                   | Description                           |
| ---------------------- | ------------------------------------- |
| `ENDPOINT_REMOVED`     | An endpoint was removed               |
| `REQUIRED_FIELD_ADDED` | A required field was added to request |
| `FIELD_TYPE_CHANGED`   | A field's type was changed            |
| `ENUM_VALUE_REMOVED`   | An allowed enum value was removed     |

See `examples/breaking-changes.ts` for a complete example.

## Producer Testing

Test that your API handlers return responses matching your OpenAPI specification.

### ProducerTestKit

```typescript
import { ProducerTestKit } from "@sahina/cvt-sdk/producer";

const testKit = new ProducerTestKit({
  schemaId: "user-api",
  serverAddress: "localhost:9550",
});

// Validate handler response
const result = await testKit.validateResponse({
  method: "GET",
  path: "/users/123",
  statusCode: 200,
  body: { id: "123", name: "Alice", email: "alice@example.com" },
});

expect(result.valid).toBe(true);

// Don't forget to close
await testKit.close();
```

### Consumer Registry

Track which services depend on your API:

```typescript
// Register a consumer after successful contract tests
await validator.registerConsumer({
  consumerId: "order-service",
  consumerVersion: "2.1.0",
  schemaId: "user-api",
  schemaVersion: "1.0.0",
  environment: "prod",
  usedEndpoints: [
    { method: "GET", path: "/users/{id}", usedFields: ["id", "email"] },
  ],
});

// List all consumers of a schema
const consumers = await validator.listConsumers({
  schemaId: "user-api",
  environment: "prod",
});

// Deregister a consumer
await validator.deregisterConsumer("order-service", "user-api", "prod");
```

### Deployment Safety (can-i-deploy)

Check if a new schema version can be safely deployed:

```typescript
const result = await validator.canIDeploy({
  schemaId: "user-api",
  newVersion: "2.0.0",
  environment: "prod",
});

if (!result.safeToDeploy) {
  console.error("Cannot deploy:", result.summary);
  for (const consumer of result.affectedConsumers) {
    if (consumer.willBreak) {
      console.error(`- ${consumer.consumerId} will break`);
    }
  }
  process.exit(1);
}
```

See [Producer Testing Guide](../../docs/guides/producer-testing.mdx) for complete documentation.

## Security Configuration

### TLS

```typescript
const validator = new ContractValidator({
  address: "localhost:9550",
  tls: {
    enabled: true,
    rootCertPath: "./certs/ca.crt", // CA certificate
    clientCertPath: "./certs/client.crt", // For mTLS
    clientKeyPath: "./certs/client.key", // For mTLS
  },
});
```

### API Key Authentication

```typescript
const validator = new ContractValidator({
  address: "localhost:9550",
  apiKey: "your-api-key-here",
});
```

## Prerequisites

Ensure the CVT gRPC server is running (default: `localhost:9550`).

## Testing

The Node.js SDK includes comprehensive tests covering:

- Client initialization and configuration
- Schema registration (local files and URLs)
- Validation requests and responses
- Error handling
- gRPC communication

### Running Tests

```bash
# Install dependencies
npm install

# Run all tests
npm test

# Run tests with coverage
npm test -- --coverage

# Run specific test file
npm test -- ContractValidator.test.ts

# Run tests in watch mode
npm test -- --watch
```

### Test Structure

```shell
tests/
└── ContractValidator.test.ts  # Main SDK test suite
```

### Writing Tests

Example test using Jest:

```typescript
import { ContractValidator } from "../src";

describe("ContractValidator", () => {
  let validator: ContractValidator;

  beforeEach(() => {
    validator = new ContractValidator({ host: "localhost:9550" });
  });

  afterEach(async () => {
    await validator.close();
  });

  it("should validate a correct interaction", async () => {
    await validator.registerSchema("test", "./openapi.json");

    const result = await validator.validate(
      { method: "GET", path: "/users" },
      { statusCode: 200, body: [] },
    );

    expect(result.valid).toBe(true);
  });
});
```

### Coverage

The SDK maintains 60%+ test coverage. Generate coverage reports with:

```bash
npm test -- --coverage
open coverage/lcov-report/index.html
```

## Development

```bash
# Install dependencies
npm install

# Build the SDK
npm run build

# Run linter
npm run lint

# Format code
npm run format:check

# Run example
npm run example
```
