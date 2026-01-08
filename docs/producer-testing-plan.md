# Producer Testing Implementation Plan

## Overview

Comprehensive producer testing for CVT that mirrors consumer testing capabilities:

- **Schema compliance testing**: Producers test their handlers against their OpenAPI spec (no consumers needed)
- **Consumer registry**: Track which consumers depend on which schemas
- **Deployment safety (can-i-deploy)**: Prevent breaking changes from reaching production

## Architecture

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Consumer Registry & Producer Testing                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  CONSUMER SIDE (during tests)              PRODUCER SIDE (before deploy)    │
│  ┌─────────────────────────────┐           ┌─────────────────────────────┐  │
│  │                             │           │                             │  │
│  │  1. Run contract tests      │           │  1. Run schema compliance   │  │
│  │     (validates against      │           │     tests (handler output   │  │
│  │      producer's spec)       │           │      matches spec)          │  │
│  │              │              │           │              │              │  │
│  │              ▼              │           │              ▼              │  │
│  │  2. Register with CVT       │           │  2. Run cvt can-i-deploy    │  │
│  │     - Which schema I use    │           │     - Check breaking changes│  │
│  │     - Which version         │           │     - Check consumer impact │  │
│  │     - Which endpoints/fields│           │              │              │  │
│  │              │              │           │              ▼              │  │
│  │              ▼              │           │  3. Deploy if safe          │  │
│  └──────────────┼──────────────┘           └─────────────────────────────┘  │
│                 │                                        ▲                  │
│                 │         ┌─────────────────────┐        │                  │
│                 └────────►│   CVT Server        │────────┘                  │
│                           │                     │                           │
│                           │  - Consumer Registry│                           │
│                           │  - Schema Store     │                           │
│                           │  - Compatibility    │                           │
│                           │    Matrix           │                           │
│                           └─────────────────────┘                           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Capability Summary

| Capability                    | Needs Registry? | What It Answers                          |
| ----------------------------- | --------------- | ---------------------------------------- |
| **Schema compliance tests**   | No              | "Does my code match my spec?"            |
| **Breaking change detection** | No              | "What changed between v1 and v2?"        |
| **can-i-deploy**              | **Yes**         | "Will this change break real consumers?" |
| **Consumer impact analysis**  | **Yes**         | "Which teams do I need to notify?"       |

---

## Phase 1: Schema Compliance Testing

**Goal**: Producers can test their handlers return spec-compliant responses without needing real consumers.

### Progress

- [x] Add `ValidateProducerRequest` proto definition
- [x] Run `make generate` for all languages
- [x] Implement `ValidateProducerResponse` server RPC
- [x] Add server tests for `ValidateProducerResponse`
- [x] Implement Node.js ProducerTestKit
- [x] Implement Go ProducerTestKit
- [x] Implement Python ProducerTestKit
- [x] Implement Java ProducerTestKit
- [x] Add SDK tests for all languages

### Files Created/Modified

| File                                               | Status | Description                                     |
| -------------------------------------------------- | ------ | ----------------------------------------------- |
| `api/protos/cvt.proto`                             | ✅     | Added `ValidateProducerRequest` message and RPC |
| `server/validator_service.go`                      | ✅     | Implemented `ValidateProducerResponse` RPC      |
| `server/validator_service_test.go`                 | ✅     | Added 5 test cases                              |
| `sdks/node/src/producer/testing.ts`                | ✅     | Node.js ProducerTestKit                         |
| `sdks/node/tests/producer/testing.test.ts`         | ✅     | Node.js tests (all passing)                     |
| `sdks/go/cvt/producer/testing.go`                  | ✅     | Go ProducerTestKit                              |
| `sdks/go/cvt/producer/testing_test.go`             | ✅     | Go tests (all passing)                          |
| `sdks/python/cvt_sdk/producer/testing.py`          | ✅     | Python ProducerTestKit                          |
| `sdks/python/tests/test_producer_testing.py`       | ✅     | Python tests (all passing)                      |
| `sdks/java/.../producer/ProducerTestKit.java`      | ✅     | Java ProducerTestKit                            |
| `sdks/java/.../producer/TestResponseData.java`     | ✅     | Java response data builder                      |
| `sdks/java/.../producer/TestRequestContext.java`   | ✅     | Java request context builder                    |
| `sdks/java/.../producer/TestValidationResult.java` | ✅     | Java validation result                          |
| `sdks/java/.../producer/ProducerTestKitTest.java`  | ✅     | Java tests (24 tests passing)                   |

---

## Phase 2: Consumer Registry

**Goal**: Track which consumers depend on which schemas for deployment safety analysis.

### Progress

- [x] Add Consumer Registry proto definitions
- [x] Extend storage interface with `ConsumerRecord`
- [x] Implement consumer storage (memory + SQLite + PostgreSQL)
- [x] Implement `RegisterConsumer`/`ListConsumers`/`DeregisterConsumer` RPCs
- [x] Implement `CanIDeploy` RPC (basic version)
- [x] Add consumer registry to SDKs (Node.js, Go, Python, Java)

### Files Created/Modified

| File                                                   | Status | Description                                                          |
| ------------------------------------------------------ | ------ | -------------------------------------------------------------------- |
| `server/storage/storage.go`                            | ✅     | Added ConsumerRecord, EndpointUsage, ListConsumersFilter             |
| `server/storage/memory.go`                             | ✅     | In-memory consumer storage                                           |
| `server/storage/sqlite/sqlite.go`                      | ✅     | SQLite consumer storage                                              |
| `server/storage/sqlite/migrations/002_consumers.sql`   | ✅     | SQLite migration                                                     |
| `server/storage/postgres/postgres.go`                  | ✅     | PostgreSQL consumer storage                                          |
| `server/storage/postgres/migrations/002_consumers.sql` | ✅     | PostgreSQL migration                                                 |
| `server/cache.go`                                      | ✅     | Added ConsumerEntry, EndpointUsage, consumer registry methods        |
| `server/validator_service.go`                          | ✅     | RegisterConsumer, ListConsumers, DeregisterConsumer, CanIDeploy RPCs |

### Proto Messages Added

- `ConsumerInfo` - Consumer dependency information
- `EndpointUsage` - Which endpoints/fields a consumer uses
- `RegisterConsumerRequest/Response`
- `ListConsumersRequest/Response`
- `DeregisterConsumerRequest/Response`

---

## Phase 3: Deployment Safety (can-i-deploy)

**Goal**: Check if a schema version can be safely deployed without breaking registered consumers.

### Progress

- [x] Add `CanIDeploy` proto definition
- [x] Implement `CanIDeploy` server logic (basic version - checks consumer version compatibility)
- [x] Add `cvt can-i-deploy` CLI command
- [x] Add `CanIDeploy` to SDKs (Node.js, Go, Python, Java)

### Proto Messages Added

- `CanIDeployRequest`
- `CanIDeployResponse`
- `ConsumerImpact`

### Expected CLI Output

```bash
# Usage
cvt can-i-deploy --schema my-api --version 2.0.0 --env prod

# Output
❌ UNSAFE TO DEPLOY

Breaking changes in v2.0.0:
  - FIELD_REMOVED: GET /users/{id} response removed 'email'

Affected consumers in production:
  ├── order-service v2.1.0
  │   Uses: GET /users/{id} (expects 'email' field)
  │   Impact: BREAKING
  │
  └── billing-service v1.0.0
      Uses: POST /users only
      Impact: None (doesn't use affected endpoint)

Safe consumers: 1/2
Affected consumers: 1/2

Recommendation: Coordinate with order-service team before deploying.
```

---

## Phase 4: Documentation

**Goal**: Comprehensive documentation for all features.

### Progress

- [x] Update README with architecture diagram and capability table
- [x] Create `docs/producer-testing-plan.md` (this file)
- [x] Create `docs/producer-testing.md` (usage guide)
- [x] Update SDK READMEs with producer testing examples

---

## SDK Usage Examples

### Node.js

```typescript
import { ProducerTestKit } from "cvt-sdk/producer";

const testKit = new ProducerTestKit({
  schemaId: "user-api",
  serverAddress: "localhost:50051",
});

// In your test
const result = await testKit.validateResponse({
  method: "GET",
  path: "/users/123",
  statusCode: 200,
  body: { id: "123", name: "John", email: "john@example.com" },
});

expect(result.valid).toBe(true);
```

### Go

```go
testKit, err := producer.NewProducerTestKit(producer.TestConfig{
    SchemaID:      "user-api",
    ServerAddress: "localhost:50051",
})

result, err := testKit.ValidateResponse(ctx, producer.ValidateResponseParams{
    Method:     "GET",
    Path:       "/users/123",
    Response: producer.TestResponseData{
        StatusCode: 200,
        Body:       map[string]interface{}{"id": "123", "name": "John"},
    },
})

assert.True(t, result.Valid)
```

### Python

```python
from cvt_sdk.producer import ProducerTestKit, TestConfig, TestResponseData

test_kit = ProducerTestKit(TestConfig(
    schema_id="user-api",
    server_address="localhost:50051",
))

result = test_kit.validate_response(
    method="GET",
    path="/users/123",
    response=TestResponseData(
        status_code=200,
        body={"id": "123", "name": "John"},
    ),
)

assert result.valid
```

### Java

```java
ProducerTestKit testKit = ProducerTestKit.builder()
    .schemaId("user-api")
    .serverAddress("localhost:50051")
    .build();

try {
    TestValidationResult result = testKit.validateResponse(
        "GET",
        "/users/123",
        TestResponseData.builder()
            .statusCode(200)
            .body(Map.of("id", "123", "name", "John"))
            .build()
    );

    assertTrue(result.isValid());
} finally {
    testKit.close();
}
```
