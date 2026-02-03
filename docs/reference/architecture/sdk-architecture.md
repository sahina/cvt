---
title: SDK Architecture
sidebar_label: SDK Architecture
sidebar_position: 5
description: Deep dive into CVT SDK design patterns and cross-language consistency
---

# SDK Architecture

This document provides a detailed look at CVT's SDK design patterns, adapter architecture, and cross-language consistency strategies.

## Overview

CVT SDKs provide:

1. **gRPC communication**: Protocol handling and connection management
2. **Configuration**: Server address, TLS, authentication
3. **Validation API**: Register schemas, validate interactions
4. **HTTP Adapters**: Automatic validation of HTTP client calls
5. **Mock Adapters**: Schema-compliant response generation

**SDKs do NOT provide:**

- Test frameworks (use Jest, Pytest, JUnit, etc.)
- HTTP clients (use Axios, requests, http.Client, etc.)
- Schema management (handled by server)

## Core Abstraction

### Validator Client

All SDKs implement a common `ContractValidator` interface:

```text
┌─────────────────────────────────────────────────────────────┐
│                    ContractValidator                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Constructor                                                │
│  ├─ address: string         (server address)                │
│  ├─ tls?: TLSConfig         (optional TLS settings)         │
│  └─ apiKey?: string         (optional authentication)       │
│                                                             │
│  Methods                                                    │
│  ├─ registerSchema(id, content, options?)                   │
│  ├─ validate(request, response)                             │
│  ├─ validateProducerResponse(method, path, response)        │
│  ├─ compareSchemas(id, oldVersion, newVersion)              │
│  ├─ generateFixture(id, method, path)                       │
│  ├─ registerConsumer(config)                                │
│  ├─ listConsumers(schemaId, environment)                    │
│  ├─ canIDeploy(schemaId, version, environment)              │
│  └─ close()                                                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Consistent API Across Languages

```typescript
// Node.js
const validator = new ContractValidator("localhost:9550");
await validator.registerSchema("my-api", "./openapi.json");
const result = await validator.validate(request, response);
```

```python
# Python
validator = ContractValidator("localhost:9550")
validator.register_schema("my-api", "./openapi.json")
result = validator.validate(request, response)
```

```go
// Go
validator, _ := cvt.NewContractValidator("localhost:9550")
validator.RegisterSchema(ctx, "my-api", schemaContent)
result, _ := validator.Validate(ctx, request, response)
```

```java
// Java
ContractValidator validator = new ContractValidator("localhost:9550");
validator.registerSchema("my-api", schemaContent);
ValidationResult result = validator.validate(request, response);
```

## Consumer Validation Patterns

### Direct Validation

Explicit validation in tests:

```text
┌──────────────────────────────────────────────────────────────┐
│                     Test Code                                │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Make HTTP call                                           │
│     ┌────────────┐     HTTP      ┌────────────┐              │
│     │ Your Code  │ ────────────► │  Real API  │              │
│     └────────────┘               └────────────┘              │
│           │                            │                     │
│           │ request                    │ response            │
│           ▼                            ▼                     │
│  2. Validate with CVT                                        │
│     ┌──────────────────────────────────────────┐             │
│     │ validator.validate(request, response)    │             │
│     └──────────────────────────────────────────┘             │
│           │                                                  │
│           ▼                                                  │
│  3. Assert result                                            │
│     expect(result.valid).toBe(true)                          │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**When to use:**

- Unit tests with explicit assertions
- Testing specific edge cases
- Full control over what gets validated

### HTTP Adapters

Automatic validation of all HTTP calls:

```text
┌─────────────────────────────────────────────────────────────┐
│                  HTTP Client with Adapter                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│     ┌────────────┐                                          │
│     │ Your Code  │                                          │
│     └─────┬──────┘                                          │
│           │ api.get('/users/123')                           │
│           ▼                                                 │
│     ┌────────────┐                                          │
│     │   HTTP     │                                          │
│     │  Client    │                                          │
│     │  (Axios)   │                                          │
│     └─────┬──────┘                                          │
│           │                                                 │
│           ▼                                                 │
│     ┌────────────────────────────────────────┐              │
│     │          CVT Adapter                   │              │
│     │  ┌──────────────────────────────────┐  │              │
│     │  │ 1. Intercept request             │  │              │
│     │  │ 2. Make actual HTTP call         │──┼──► Real API  │
│     │  │ 3. Intercept response            │◄─┼──            │
│     │  │ 4. Validate both with CVT        │  │              │
│     │  │ 5. Return response (or throw)    │  │              │
│     │  └──────────────────────────────────┘  │              │
│     └────────────────────────────────────────┘              │
│           │                                                 │
│           ▼                                                 │
│     Response (or validation error)                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**When to use:**

- Integration tests
- Zero-touch validation
- Catch all contract violations

### Mock Adapters

Schema-compliant response generation:

```text
┌──────────────────────────────────────────────────────────────┐
│                    Mock Adapter                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│     ┌────────────┐                                           │
│     │ Your Code  │                                           │
│     └─────┬──────┘                                           │
│           │ mock.get('/users/123')                           │
│           ▼                                                  │
│     ┌────────────────────────────────────────┐               │
│     │          Mock Adapter                  │               │
│     │  ┌──────────────────────────────────┐  │               │
│     │  │ 1. Parse request                 │  │               │
│     │  │ 2. Find matching operation       │  │               │
│     │  │ 3. Generate response from schema │  │   No network  │
│     │  │ 4. Validate generated response   │  │   call!       │
│     │  │ 5. Return mock response          │  │               │
│     │  └──────────────────────────────────┘  │               │
│     └────────────────────────────────────────┘               │
│           │                                                  │
│           ▼                                                  │
│     Generated Response                                       │
│     { id: 123, name: "string", email: "user@example.com" }   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**When to use:**

- API not yet available
- Testing without network access
- Fast, deterministic tests

## Producer Validation Patterns

### Middleware (Runtime)

Validate requests/responses at runtime:

```text
┌──────────────────────────────────────────────────────────────┐
│                    API Server with Middleware                │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│     Client Request                                           │
│           │                                                  │
│           ▼                                                  │
│     ┌────────────────────────────────────────┐               │
│     │          CVT Middleware                │               │
│     │  ┌──────────────────────────────────┐  │               │
│     │  │ 1. Validate incoming request     │  │               │
│     │  │                                  │  │               │
│     │  │    mode: strict → reject 400     │  │               │
│     │  │    mode: warn   → log, continue  │  │               │
│     │  │    mode: shadow → metrics only   │  │               │
│     │  └──────────────────────────────────┘  │               │
│     └────────────────────────────────────────┘               │
│           │                                                  │
│           ▼                                                  │
│     ┌────────────────────────────────────────┐               │
│     │          Your Handler                  │               │
│     │     return { id: 123, name: "Alice" }  │               │
│     └────────────────────────────────────────┘               │
│           │                                                  │
│           ▼                                                  │
│     ┌────────────────────────────────────────┐               │
│     │          CVT Middleware                │               │
│     │  ┌──────────────────────────────────┐  │               │
│     │  │ 2. Validate outgoing response    │  │               │
│     │  │    (log violations, metrics)     │  │               │
│     │  └──────────────────────────────────┘  │               │
│     └────────────────────────────────────────┘               │
│           │                                                  │
│           ▼                                                  │
│     Client Response                                          │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Validation modes:**

| Mode     | Request Violation | Response Violation | Use Case               |
| -------- | ----------------- | ------------------ | ---------------------- |
| `strict` | Reject (400)      | Log error          | Production enforcement |
| `warn`   | Log, continue     | Log warning        | Gradual rollout        |
| `shadow` | Metrics only      | Metrics only       | Initial deployment     |

### ProducerTestKit (Test-time)

Validate handler output in tests:

```text
┌──────────────────────────────────────────────────────────────┐
│                    Producer Test                             │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Call handler directly                                    │
│     ┌──────────────────────────────────────────┐             │
│     │ const response = await myHandler(request)│             │
│     └──────────────────────────────────────────┘             │
│           │                                                  │
│           ▼                                                  │
│  2. Validate with ProducerTestKit                            │
│     ┌──────────────────────────────────────────┐             │
│     │ testKit.validateResponse({               │             │
│     │   method: "GET",                         │             │
│     │   path: "/users/123",                    │             │
│     │   statusCode: 200,                       │             │
│     │   body: response                         │             │
│     │ })                                       │             │
│     └──────────────────────────────────────────┘             │
│           │                                                  │
│           ▼                                                  │
│  3. Assert compliance                                        │
│     expect(result.valid).toBe(true)                          │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**When to use:**

- Unit testing handlers
- Pre-deployment validation
- Faster than middleware approach

## Adapter Pattern

All SDK adapters follow a consistent pattern:

```text
┌─────────────────────────────────────────────────────────────┐
│                     Adapter Interface                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Configuration                                              │
│  ├─ validator: ContractValidator                            │
│  ├─ schemaId: string                                        │
│  ├─ onViolation: "throw" | "log" | "ignore"                 │
│  └─ recordUsedEndpoints: boolean                            │
│                                                             │
│  Lifecycle                                                  │
│  ├─ intercept(request) → modifiedRequest                    │
│  ├─ intercept(response) → validatedResponse                 │
│  └─ dispose()                                               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Language-Specific Implementations

#### Node.js (Axios Adapter)

```typescript
const adapter = createAxiosAdapter({
  axios: axiosInstance,
  validator,
  schemaId: "user-api",
  onViolation: "throw", // or "log", "ignore"
});

// All calls through this instance are validated
const response = await axiosInstance.get("/users/123");
```

**Implementation:** Uses Axios interceptors for request/response interception.

#### Python (Session Adapter)

```python
session = ContractValidatingSession(
    validator=validator,
    schema_id="user-api",
    on_violation="throw",
)

# All calls through this session are validated
response = session.get("http://api/users/123")
```

**Implementation:** Wraps `requests.Session` with validation hooks.

#### Go (RoundTripper)

```go
rt := adapters.NewValidatingRoundTripper(adapters.RoundTripperConfig{
    Validator: validator,
    SchemaID:  "user-api",
    OnViolation: adapters.OnViolationThrow,
})

client := &http.Client{Transport: rt}

// All calls through this client are validated
resp, _ := client.Get("http://api/users/123")
```

**Implementation:** Implements `http.RoundTripper` interface for interception.

#### Java (Interceptor)

```java
OkHttpClient client = new OkHttpClient.Builder()
    .addInterceptor(new CVTInterceptor(validator, "user-api"))
    .build();

// All calls through this client are validated
Response response = client.newCall(request).execute();
```

**Implementation:** Uses OkHttp Interceptor for request/response interception.

## Error Handling

### Validation Result Structure

All SDKs return a consistent validation result:

```typescript
interface ValidationResult {
  valid: boolean;
  errors?: string[];
  validatedAgainstVersion?: string;
  validatedAgainstHash?: string;
}
```

### Error Handling Strategies

| Strategy | Behavior                     | Use Case        |
| -------- | ---------------------------- | --------------- |
| `throw`  | Throw exception on violation | Strict testing  |
| `log`    | Log warning, return result   | Observability   |
| `ignore` | Silent, return result        | Gradual rollout |

```typescript
// Throw on violation
const adapter = createAxiosAdapter({
  onViolation: "throw",
});

try {
  await api.get("/users/123");
} catch (error) {
  // ContractValidationError
  console.log(error.violations);
}
```

```typescript
// Log on violation
const adapter = createAxiosAdapter({
  onViolation: "log",
  logger: console,
});

// Violations logged, response returned
const response = await api.get("/users/123");
```

## Language-Specific Notes

### Node.js

- **Proto loading**: Dynamic loading via `@grpc/proto-loader`
- **Connection management**: gRPC channel reuse
- **Async/await**: All methods return Promises
- **Types**: Full TypeScript support

### Python

- **Package manager**: uv (recommended) or pip
- **Proto generation**: Pre-generated with `grpcio-tools`
- **Async support**: Both sync and async APIs
- **Type hints**: Full typing support

### Go

- **Proto generation**: Pre-generated with protoc-gen-go
- **Context**: All methods accept `context.Context`
- **Error handling**: Go-style `(result, error)` returns
- **HTTP integration**: Native `http.RoundTripper`

### Java

- **Build system**: Gradle with protobuf plugin
- **Proto generation**: Gradle task `generateProto`
- **Async support**: CompletableFuture API available
- **Spring integration**: Auto-configuration support

## Implementation Notes

Key implementation files:

| SDK     | Main Files                                                              |
| ------- | ----------------------------------------------------------------------- |
| Node.js | `sdks/node/src/client.ts`, `sdks/node/src/adapters/`                    |
| Python  | `sdks/python/cvt_sdk/client.py`, `sdks/python/cvt_sdk/adapters/`        |
| Go      | `sdks/go/cvt/client.go`, `sdks/go/cvt/adapters/`                        |
| Java    | `sdks/java/src/main/java/cvt/`, `sdks/java/src/main/java/cvt/adapters/` |

## In Roadmap

The following SDK features are planned but not yet implemented:

- **Automatic endpoint tracking**: Auto-detect used endpoints for consumer registration
- **Batch validation**: Validate multiple interactions in single RPC
- **Connection pooling**: Improved connection management for high throughput
- **Metrics export**: SDK-side metrics for client-side monitoring

---

## Related Documentation

- **[Architecture Overview](./index.md)** - System architecture
- **[SDK Documentation](../sdk/index.mdx)** - Language-specific SDK guides
- **[Consumer Testing Guide](../../guides/consumer-testing.mdx)** - Consumer validation patterns
- **[Producer Testing Guide](../../guides/producer-testing.mdx)** - Producer validation patterns
