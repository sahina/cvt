# Consumer Testing Implementation Plan

## Overview

Consumer testing in CVT enables API consumers to validate their HTTP interactions against producer OpenAPI schemas and register their dependencies for deployment safety:

- **Contract validation**: Consumers validate request/response pairs against the producer's OpenAPI spec
- **Consumer registry**: Track which consumers depend on which schemas and which endpoints/fields they use
- **Deployment safety (can-i-deploy)**: Producers check if schema changes will break registered consumers

## Architecture

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Consumer Contract Testing Flow                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  CONSUMER SIDE (during tests)              PRODUCER SIDE (before deploy)    │
│  ┌─────────────────────────────┐           ┌─────────────────────────────┐  │
│  │                             │           │                             │  │
│  │  1. Register producer's     │           │  1. Update OpenAPI schema   │  │
│  │     OpenAPI schema          │           │                             │  │
│  │              │              │           │              │              │  │
│  │              ▼              │           │              ▼              │  │
│  │  2. Validate interactions   │           │  2. Run cvt can-i-deploy    │  │
│  │     (request + response     │           │     - Check breaking changes│  │
│  │      against schema)        │           │     - Check consumer impact │  │
│  │              │              │           │              │              │  │
│  │              ▼              │           │              ▼              │  │
│  │  3. Register as consumer    │           │  3. Deploy if safe          │  │
│  │     - Which schema I use    │           │                             │  │
│  │     - Which endpoints/fields│           │                             │  │
│  │              │              │           │                             │  │
│  └──────────────┼──────────────┘           └─────────────────────────────┘  │
│                 │                                        ▲                  │
│                 │         ┌─────────────────────┐        │                  │
│                 └────────►│   CVT Server        │────────┘                  │
│                           │                     │                           │
│                           │  - Schema Store     │                           │
│                           │  - Consumer Registry│                           │
│                           │  - Compatibility    │                           │
│                           │    Analysis         │                           │
│                           └─────────────────────┘                           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Capability Summary

| Capability                   | Needs Server? | What It Answers                               |
| ---------------------------- | ------------- | --------------------------------------------- |
| **Schema registration**      | Yes           | "Store the producer's OpenAPI spec"           |
| **Interaction validation**   | Yes           | "Does my request/response match the spec?"    |
| **Consumer registration**    | Yes           | "Record that I depend on this schema"         |
| **can-i-deploy**             | Yes           | "Will this schema change break consumers?"    |
| **Consumer impact analysis** | Yes           | "Which consumers use the affected endpoints?" |

---

## Phase 1: Schema Registration & Validation

**Goal**: Consumers can register producer schemas and validate their HTTP interactions against them.

### Progress

- [x] Add `RegisterSchemaRequest` proto definition
- [x] Add `InteractionRequest` / `ValidationResult` proto definitions
- [x] Implement `RegisterSchema` server RPC
- [x] Implement `ValidateInteraction` server RPC
- [x] Support OpenAPI v2 (Swagger) with auto-conversion to v3
- [x] Support OpenAPI v3 schemas
- [x] Implement Node.js SDK
- [x] Implement Go SDK
- [x] Implement Python SDK
- [x] Implement Java SDK
- [x] Add comprehensive SDK tests

### Files Created/Modified

| File                                   | Status | Description                                  |
| -------------------------------------- | ------ | -------------------------------------------- |
| `api/protos/cvt.proto`                 | ✅     | Core proto definitions                       |
| `server/validator_service.go`          | ✅     | `RegisterSchema`, `ValidateInteraction` RPCs |
| `server/cache.go`                      | ✅     | Schema caching with Ristretto                |
| `sdks/node/src/index.ts`               | ✅     | Node.js ContractValidator                    |
| `sdks/go/cvt/validator.go`             | ✅     | Go ContractValidator                         |
| `sdks/python/cvt_sdk/__init__.py`      | ✅     | Python ContractValidator                     |
| `sdks/java/.../ContractValidator.java` | ✅     | Java ContractValidator                       |

### Proto Messages

- `RegisterSchemaRequest` - Schema ID, version, content (JSON/YAML)
- `RegisterSchemaResponse` - Success status, schema hash
- `InteractionRequest` - Schema ID, request data, response data
- `ValidationResult` - Valid flag, error list, validated version/hash

---

## Phase 2: Consumer Registry

**Goal**: Track which consumers depend on which schemas for deployment safety analysis.

### Progress

- [x] Add Consumer Registry proto definitions
- [x] Extend storage interface with `ConsumerRecord`
- [x] Implement consumer storage (memory + SQLite + PostgreSQL)
- [x] Implement `RegisterConsumer` RPC
- [x] Implement `ListConsumers` RPC
- [x] Implement `DeregisterConsumer` RPC
- [x] Add consumer registry methods to all SDKs
- [x] Add database migrations for SQLite and PostgreSQL

### Files Created/Modified

| File                                                   | Status | Description                              |
| ------------------------------------------------------ | ------ | ---------------------------------------- |
| `api/protos/cvt.proto`                                 | ✅     | Consumer registry proto definitions      |
| `server/storage/storage.go`                            | ✅     | `ConsumerRecord`, `EndpointUsage` types  |
| `server/storage/memory.go`                             | ✅     | In-memory consumer storage               |
| `server/storage/sqlite/sqlite.go`                      | ✅     | SQLite consumer storage                  |
| `server/storage/sqlite/migrations/002_consumers.sql`   | ✅     | SQLite migration                         |
| `server/storage/postgres/postgres.go`                  | ✅     | PostgreSQL consumer storage              |
| `server/storage/postgres/migrations/002_consumers.sql` | ✅     | PostgreSQL migration                     |
| `server/cache.go`                                      | ✅     | `ConsumerEntry`, consumer registry cache |
| `server/validator_service.go`                          | ✅     | Consumer registry RPC implementations    |

### Proto Messages

- `ConsumerInfo` - Consumer ID, version, schema ID/version, environment, endpoints
- `EndpointUsage` - Method, path, used fields
- `RegisterConsumerRequest/Response`
- `ListConsumersRequest/Response`
- `DeregisterConsumerRequest/Response`

### Database Schema

```sql
CREATE TABLE consumers (
    id INTEGER PRIMARY KEY,
    consumer_id TEXT NOT NULL,
    consumer_version TEXT NOT NULL,
    schema_id TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    environment TEXT NOT NULL,
    registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_validated_at TIMESTAMP,
    used_endpoints TEXT,  -- JSON array
    UNIQUE(consumer_id, schema_id, environment)
);

CREATE INDEX idx_consumers_consumer_id ON consumers(consumer_id);
CREATE INDEX idx_consumers_schema_id ON consumers(schema_id);
CREATE INDEX idx_consumers_environment ON consumers(environment);
CREATE INDEX idx_consumers_schema_env ON consumers(schema_id, environment);
```

---

## Phase 3: Deployment Safety (can-i-deploy)

**Goal**: Check if a schema version can be safely deployed without breaking registered consumers.

### Progress

- [x] Add `CanIDeploy` proto definition
- [x] Implement `CanIDeploy` server logic
- [x] Add `cvt can-i-deploy` CLI command
- [x] Add `canIDeploy` to all SDKs
- [x] Support human-readable and JSON output formats

### Files Created/Modified

| File                                   | Status | Description                           |
| -------------------------------------- | ------ | ------------------------------------- |
| `api/protos/cvt.proto`                 | ✅     | `CanIDeployRequest/Response` messages |
| `server/validator_service.go`          | ✅     | `CanIDeploy` RPC implementation       |
| `cmd/cvt/can_i_deploy.go`              | ✅     | CLI command implementation            |
| `sdks/node/src/index.ts`               | ✅     | Node.js `canIDeploy` method           |
| `sdks/go/cvt/validator.go`             | ✅     | Go `CanIDeploy` method                |
| `sdks/python/cvt_sdk/__init__.py`      | ✅     | Python `can_i_deploy` method          |
| `sdks/java/.../ContractValidator.java` | ✅     | Java `canIDeploy` method              |

### Proto Messages

- `CanIDeployRequest` - Schema ID, version, environment
- `CanIDeployResponse` - Safe flag, breaking changes, affected consumers
- `ConsumerImpact` - Consumer ID/version, affected endpoints, impact level

### CLI Usage

```bash
# Check if safe to deploy
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod

# JSON output for CI/CD integration
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod --json
```

### Expected Output

```text
❌ UNSAFE TO DEPLOY

Breaking changes in v2.0.0:
  - FIELD_REMOVED: GET /users/{id} response removed 'email'

Affected consumers in production:
  ├── order-service v2.1.0
  │   Schema version: 1.0.0
  │   Impact: BREAKING
  │   Affected by:
  │     - GET /users/{id} (uses 'email' field)
  │
  └── billing-service v1.0.0
      Uses: POST /users only
      Impact: None (doesn't use affected endpoint)

Safe consumers:     1/2
Affected consumers: 1/2

Recommendation: Coordinate with order-service team before deploying.
```

---

## SDK Usage Examples

### Node.js

```typescript
import { ContractValidator } from "cvt-sdk";

const validator = new ContractValidator({
  serverAddress: "localhost:9550",
});

// 1. Register producer's schema
await validator.registerSchema("user-api", userApiSpec);

// 2. Validate an interaction
const result = await validator.validateInteraction({
  schemaId: "user-api",
  request: {
    method: "GET",
    path: "/users/123",
    headers: { "Content-Type": "application/json" },
  },
  response: {
    statusCode: 200,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      id: "123",
      name: "John",
      email: "john@example.com",
    }),
  },
});

expect(result.valid).toBe(true);

// 3. Register as a consumer
await validator.registerConsumer({
  consumerId: "order-service",
  consumerVersion: "2.1.0",
  schemaId: "user-api",
  schemaVersion: "1.0.0",
  environment: "prod",
  usedEndpoints: [
    { method: "GET", path: "/users/{id}", usedFields: ["id", "name", "email"] },
  ],
});
```

### Go

```go
import "github.com/cvt/cvt-go-sdk/cvt"

validator, err := cvt.NewContractValidator(cvt.Config{
    ServerAddress: "localhost:9550",
})

// 1. Register producer's schema
err = validator.RegisterSchema(ctx, "user-api", schemaContent)

// 2. Validate an interaction
result, err := validator.ValidateInteraction(ctx, cvt.InteractionRequest{
    SchemaID: "user-api",
    Request: cvt.RequestData{
        Method:  "GET",
        Path:    "/users/123",
        Headers: map[string]string{"Content-Type": "application/json"},
    },
    Response: cvt.ResponseData{
        StatusCode: 200,
        Headers:    map[string]string{"Content-Type": "application/json"},
        Body:       `{"id": "123", "name": "John", "email": "john@example.com"}`,
    },
})

assert.True(t, result.Valid)

// 3. Register as a consumer
err = validator.RegisterConsumer(ctx, cvt.ConsumerInfo{
    ConsumerID:      "order-service",
    ConsumerVersion: "2.1.0",
    SchemaID:        "user-api",
    SchemaVersion:   "1.0.0",
    Environment:     "prod",
    UsedEndpoints: []cvt.EndpointUsage{
        {Method: "GET", Path: "/users/{id}", UsedFields: []string{"id", "name", "email"}},
    },
})
```

### Python

```python
from cvt_sdk import ContractValidator

validator = ContractValidator(server_address="localhost:9550")

# 1. Register producer's schema
validator.register_schema("user-api", schema_content)

# 2. Validate an interaction
result = validator.validate_interaction(
    schema_id="user-api",
    request={
        "method": "GET",
        "path": "/users/123",
        "headers": {"Content-Type": "application/json"},
    },
    response={
        "status_code": 200,
        "headers": {"Content-Type": "application/json"},
        "body": '{"id": "123", "name": "John", "email": "john@example.com"}',
    },
)

assert result.valid

# 3. Register as a consumer
validator.register_consumer(
    consumer_id="order-service",
    consumer_version="2.1.0",
    schema_id="user-api",
    schema_version="1.0.0",
    environment="prod",
    used_endpoints=[
        {"method": "GET", "path": "/users/{id}", "used_fields": ["id", "name", "email"]},
    ],
)
```

### Java

```java
import com.cvt.sdk.ContractValidator;

ContractValidator validator = ContractValidator.builder()
    .serverAddress("localhost:9550")
    .build();

try {
    // 1. Register producer's schema
    validator.registerSchema("user-api", schemaContent);

    // 2. Validate an interaction
    ValidationResult result = validator.validateInteraction(
        "user-api",
        RequestData.builder()
            .method("GET")
            .path("/users/123")
            .header("Content-Type", "application/json")
            .build(),
        ResponseData.builder()
            .statusCode(200)
            .header("Content-Type", "application/json")
            .body("{\"id\": \"123\", \"name\": \"John\", \"email\": \"john@example.com\"}")
            .build()
    );

    assertTrue(result.isValid());

    // 3. Register as a consumer
    validator.registerConsumer(ConsumerInfo.builder()
        .consumerId("order-service")
        .consumerVersion("2.1.0")
        .schemaId("user-api")
        .schemaVersion("1.0.0")
        .environment("prod")
        .usedEndpoint(EndpointUsage.builder()
            .method("GET")
            .path("/users/{id}")
            .usedFields(List.of("id", "name", "email"))
            .build())
        .build());
} finally {
    validator.close();
}
```

---

## Testing Strategy

### Unit Tests

Each SDK includes comprehensive unit tests for consumer functionality:

| SDK     | Test File                                           | Coverage |
| ------- | --------------------------------------------------- | -------- |
| Node.js | `sdks/node/tests/contract-validator.test.ts`        | 70%+     |
| Go      | `sdks/go/cvt/validator_test.go`                     | 70%+     |
| Python  | `sdks/python/tests/test_contract_validator.py`      | 70%+     |
| Java    | `sdks/java/src/test/.../ContractValidatorTest.java` | 70%+     |

### Integration Tests

Server-side integration tests verify end-to-end consumer testing flow:

```bash
# Run all tests
make test

# Run server tests only
make test-server

# Run with coverage
make test-coverage
```

### Test Scenarios

1. **Schema Registration**
   - Valid OpenAPI v3 schema
   - Valid Swagger v2 schema (auto-converted)
   - Invalid schema (should fail)
   - Schema versioning

2. **Interaction Validation**
   - Valid request/response
   - Invalid request (missing required field)
   - Invalid response (wrong status code)
   - Invalid response body (schema mismatch)
   - Path parameter validation
   - Query parameter validation
   - Header validation

3. **Consumer Registration**
   - Register new consumer
   - Update existing consumer
   - List consumers by schema
   - List consumers by environment
   - Deregister consumer

4. **Can-I-Deploy**
   - Safe deployment (no breaking changes)
   - Unsafe deployment (breaking changes)
   - No registered consumers
   - Field-level impact analysis

---

## CI/CD Integration

### GitHub Actions Example

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

      - name: Run contract tests
        run: npm test

      - name: Register as consumer
        run: |
          npm run register-consumer -- \
            --schema user-api \
            --version ${{ github.sha }} \
            --env ${{ github.ref == 'refs/heads/main' && 'prod' || 'staging' }}
```

### Pre-Deploy Check

```yaml
deploy:
  needs: [contract-tests]
  steps:
    - name: Check deployment safety
      run: |
        cvt can-i-deploy \
          --schema user-api \
          --version ${{ env.NEW_VERSION }} \
          --env prod \
          --json > deploy-check.json

        if [ $(jq '.safeToDeploy' deploy-check.json) != "true" ]; then
          echo "❌ Unsafe to deploy - breaking changes detected"
          jq '.affectedConsumers' deploy-check.json
          exit 1
        fi
```
