---
title: SDK Reference
sidebar_label: Overview
sidebar_position: 1
description: CVT SDK documentation for all supported languages
---

# SDK Reference

CVT provides native SDKs for the following languages:

| Language | Package | Status |
|----------|---------|--------|
| [Node.js](./nodejs.md) | `@cvt/cvt-sdk` | Production-ready |
| [Python](./python.md) | `cvt-sdk` | Production-ready |
| [Go](./go.md) | `github.com/sahina/cvt/sdks/go` | Production-ready |
| [Java](./java.md) | `com.cvt:cvt-sdk` | Production-ready |

## Common Features

All SDKs provide:

- **Schema Registration** - Register OpenAPI v2/v3 schemas
- **Interaction Validation** - Validate request/response pairs
- **Consumer Registration** - Register as a consumer for deployment safety
- **Schema Comparison** - Detect breaking changes between versions
- **Fixture Generation** - Generate test data from schemas
- **HTTP Adapters** - Automatic validation for HTTP clients
- **Producer Middleware** - Automatic validation for HTTP servers

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Your Application                        │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐ │
│  │  Node.js  │  │  Python   │  │    Go     │  │   Java    │ │
│  │    SDK    │  │    SDK    │  │    SDK    │  │    SDK    │ │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘ │
└────────┼──────────────┼──────────────┼──────────────┼───────┘
         │              │              │              │
         └──────────────┴──────────────┴──────────────┘
                                │
                          gRPC Protocol
                                │
                                ▼
                   ┌────────────────────────┐
                   │      CVT Server        │
                   │     (Port 9550)        │
                   └────────────────────────┘
```

## Quick Comparison

### Initialization

```typescript
// Node.js
const validator = new ContractValidator('localhost:9550');
```

```python
# Python
validator = ContractValidator('localhost:9550')
```

```go
// Go
validator, _ := cvt.NewValidator("localhost:9550")
```

```java
// Java
ContractValidator validator = new ContractValidator("localhost:9550");
```

### Validation

```typescript
// Node.js
const result = await validator.validate(request, response);
```

```python
# Python
result = validator.validate(request, response)
```

```go
// Go
result, _ := validator.Validate(ctx, request, response)
```

```java
// Java
ValidationResult result = validator.validate(request, response);
```

## Choosing an SDK

All SDKs have equivalent functionality. Choose based on your application's language.

For detailed documentation, see the individual SDK guides.
