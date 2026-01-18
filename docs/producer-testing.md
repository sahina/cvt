---
title: Producer Testing Guide
sidebar_label: Testing Guide
sidebar_position: 1
description: How to use CVT for producer-side contract testing
---

# Producer Testing Guide

This guide covers how to use CVT for producer-side contract testing, including schema compliance testing, consumer registry, and deployment safety checks.

## Overview

CVT supports two complementary testing approaches:

| Approach | Who Uses It | What It Tests |
|----------|-------------|---------------|
| **Consumer Testing** | API consumers | "Can I call this API correctly?" |
| **Producer Testing** | API producers | "Does my API match my spec?" |

Producer testing ensures your API implementation matches your OpenAPI specification before deployment, without needing real consumers.

## Capabilities

| Capability | Needs Server? | What It Answers |
|------------|---------------|-----------------|
| **Schema compliance tests** | Yes | "Does my handler return spec-compliant responses?" |
| **Breaking change detection** | No (CLI) | "What changed between v1 and v2 of my spec?" |
| **Consumer registry** | Yes | "Which services depend on my API?" |
| **can-i-deploy** | Yes | "Will this change break real consumers?" |

---

## Schema Compliance Testing

Schema compliance testing validates that your API handlers return responses matching your OpenAPI specification.

### How It Works

1. Register your OpenAPI schema with CVT server
2. Call your handler with test data
3. Validate the response against the schema
4. Get detailed error messages for any mismatches

### Node.js Example

```typescript
import { ProducerTestKit } from '@cvt/cvt-sdk/producer';

describe('User API', () => {
  let testKit: ProducerTestKit;

  beforeAll(async () => {
    testKit = new ProducerTestKit({
      schemaId: 'user-api',
      serverAddress: 'localhost:9550',
    });
  });

  afterAll(async () => {
    await testKit.close();
  });

  it('GET /users/:id returns valid response', async () => {
    // Call your actual handler
    const response = await userHandler.getUser('123');

    // Validate against schema
    const result = await testKit.validateResponse({
      method: 'GET',
      path: '/users/123',
      statusCode: 200,
      body: response,
    });

    expect(result.valid).toBe(true);
    expect(result.errors).toHaveLength(0);
  });

  it('detects missing required fields', async () => {
    const result = await testKit.validateResponse({
      method: 'GET',
      path: '/users/123',
      statusCode: 200,
      body: { id: '123' }, // missing 'name' field
    });

    expect(result.valid).toBe(false);
    expect(result.errors[0]).toContain('name');
  });
});
```

### Go Example

```go
func TestUserHandler(t *testing.T) {
    testKit, err := producer.NewProducerTestKit(producer.TestConfig{
        SchemaID:      "user-api",
        ServerAddress: "localhost:9550",
    })
    require.NoError(t, err)
    defer func() { _ = testKit.Close() }()

    t.Run("GET /users/:id returns valid response", func(t *testing.T) {
        // Call your handler
        resp := userHandler.GetUser(ctx, "123")

        // Validate
        result, err := testKit.ValidateResponse(ctx, producer.ValidateResponseParams{
            Method:     "GET",
            Path:       "/users/123",
            StatusCode: 200,
            Body:       resp,
        })

        require.NoError(t, err)
        assert.True(t, result.Valid)
    })
}
```

### Python Example

```python
import pytest
from cvt_sdk.producer import ProducerTestKit, TestConfig

@pytest.fixture
def test_kit():
    kit = ProducerTestKit(TestConfig(
        schema_id="user-api",
        server_address="localhost:9550",
    ))
    yield kit
    kit.close()

def test_get_user_returns_valid_response(test_kit, user_handler):
    # Call your handler
    response = user_handler.get_user("123")

    # Validate
    result = test_kit.validate_response(
        method="GET",
        path="/users/123",
        status_code=200,
        body=response,
    )

    assert result.valid
    assert len(result.errors) == 0
```

### Java Example

```java
public class UserHandlerTest {
    private ProducerTestKit testKit;

    @BeforeEach
    void setup() {
        testKit = ProducerTestKit.builder()
            .schemaId("user-api")
            .serverAddress("localhost:9550")
            .build();
    }

    @AfterEach
    void teardown() {
        testKit.close();
    }

    @Test
    void getUserReturnsValidResponse() {
        // Call your handler
        var response = userHandler.getUser("123");

        // Validate
        var result = testKit.validateResponse(
            "GET",
            "/users/123",
            TestResponseData.builder()
                .statusCode(200)
                .body(response)
                .build()
        );

        assertTrue(result.isValid());
        assertTrue(result.getErrors().isEmpty());
    }
}
```

---

## Consumer Registry

The consumer registry tracks which services depend on your API and which endpoints/fields they use.

### Registering Consumers

Consumers register themselves after successful contract tests:

```typescript
// Node.js
const validator = new ContractValidator('localhost:9550');

await validator.registerConsumer({
  consumerId: 'order-service',
  consumerVersion: '2.1.0',
  schemaId: 'user-api',
  schemaVersion: '1.0.0',
  environment: 'prod',
  usedEndpoints: [
    {
      method: 'GET',
      path: '/users/{id}',
      usedFields: ['id', 'email', 'name'],
    },
  ],
});
```

### Listing Consumers

Producers can see who depends on their API:

```typescript
const consumers = await validator.listConsumers({
  schemaId: 'user-api',
  environment: 'prod',
});

console.log(`${consumers.length} services depend on user-api in prod`);
for (const consumer of consumers) {
  console.log(`- ${consumer.consumerId} v${consumer.consumerVersion}`);
}
```

### Deregistering Consumers

When a service stops using an API:

```typescript
await validator.deregisterConsumer('order-service', 'user-api', 'prod');
```

---

## Deployment Safety (can-i-deploy)

Before deploying a new schema version, check if it will break any consumers.

### CLI Usage

```bash
# Check if v2.0.0 can be deployed to production
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod

# Output as JSON for CI/CD
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod --json

# Use a specific server
cvt can-i-deploy --schema user-api --version 2.0.0 --env prod --server cvt.internal:9550
```

### Example Output

When deployment is safe:

```
Deployment Safety Check
=======================
Schema:      user-api
Version:     2.0.0
Environment: prod

✅ SAFE TO DEPLOY

No breaking changes detected that would affect registered consumers.
```

When deployment is unsafe:

```
Deployment Safety Check
=======================
Schema:      user-api
Version:     2.0.0
Environment: prod

❌ UNSAFE TO DEPLOY

Breaking changes in v2.0.0:
  - FIELD_REMOVED: GET /users/{id} response removed 'email'

Affected consumers in production:
  ├── order-service v2.1.0
  │   Schema version: 1.0.0
  │   Impact: BREAKING
  │   Affected by:
  │     - GET /users/{id}
  │
  └── billing-service v1.0.0
      Schema version: 1.0.0
      Impact: None

Safe consumers:     1/2
Affected consumers: 1/2

Recommendation: Coordinate with order-service team before deploying.
```

### SDK Usage

```typescript
// Node.js
const result = await validator.canIDeploy({
  schemaId: 'user-api',
  newVersion: '2.0.0',
  environment: 'prod',
});

if (!result.safeToDeploy) {
  console.error('Cannot deploy:', result.summary);
  for (const consumer of result.affectedConsumers) {
    if (consumer.willBreak) {
      console.error(`- ${consumer.consumerId} will break`);
    }
  }
  process.exit(1);
}
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Deploy API

on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run contract tests
        run: npm test

      - name: Check deployment safety
        run: |
          cvt can-i-deploy \
            --schema ${{ env.SCHEMA_ID }} \
            --version ${{ env.VERSION }} \
            --env prod \
            --server ${{ secrets.CVT_SERVER }}

      - name: Deploy
        if: success()
        run: ./deploy.sh
```

### GitLab CI Example

```yaml
stages:
  - test
  - safety-check
  - deploy

contract-tests:
  stage: test
  script:
    - npm test

deployment-safety:
  stage: safety-check
  script:
    - cvt can-i-deploy --schema $SCHEMA_ID --version $VERSION --env prod --json
  allow_failure: false

deploy:
  stage: deploy
  script:
    - ./deploy.sh
  only:
    - main
```

---

## Best Practices

### 1. Test All Response Scenarios

Don't just test the happy path:

```typescript
it('validates 404 response', async () => {
  const result = await testKit.validateResponse({
    method: 'GET',
    path: '/users/nonexistent',
    statusCode: 404,
    body: { error: 'User not found' },
  });
  expect(result.valid).toBe(true);
});

it('validates 400 response for bad request', async () => {
  const result = await testKit.validateResponse({
    method: 'POST',
    path: '/users',
    statusCode: 400,
    body: { errors: [{ field: 'email', message: 'Invalid format' }] },
  });
  expect(result.valid).toBe(true);
});
```

### 2. Register Endpoint Usage Accurately

Track which fields consumers actually use:

```typescript
await validator.registerConsumer({
  consumerId: 'order-service',
  schemaId: 'user-api',
  usedEndpoints: [
    {
      method: 'GET',
      path: '/users/{id}',
      usedFields: ['id', 'email'],  // Only fields you actually use
    },
  ],
});
```

### 3. Run can-i-deploy in CI

Make deployment safety checks a required gate:

```bash
# In your CI pipeline
cvt can-i-deploy --schema my-api --version $NEW_VERSION --env prod || exit 1
```

### 4. Use Environment-Specific Checks

Check each environment before promoting:

```bash
# Check staging first
cvt can-i-deploy --schema my-api --version 2.0.0 --env staging

# Then production
cvt can-i-deploy --schema my-api --version 2.0.0 --env prod
```

---

## Comparison with Pact

| Aspect | CVT Producer Testing | Pact |
|--------|---------------------|------|
| **Schema format** | OpenAPI (existing specs) | Pact-specific contracts |
| **Contract generation** | Use existing OpenAPI | Generated from consumer tests |
| **Breaking change detection** | Schema diff analysis | Consumer re-verification |
| **Deployment safety** | can-i-deploy with registry | Pact broker can-i-deploy |
| **Setup complexity** | Single CVT server | Pact broker + per-language setup |

CVT is ideal when you already have OpenAPI specifications and want schema-first contract testing. Pact is better for consumer-driven contract testing where contracts are generated from consumer tests.

---

## Troubleshooting

### "Schema not found" Error

Ensure the schema is registered before running producer tests:

```typescript
// Register schema first
await validator.registerSchema({
  schemaId: 'user-api',
  schemaVersion: '1.0.0',
  format: 'openapi_v3',
  content: fs.readFileSync('./openapi.yaml', 'utf-8'),
});
```

### "Path not found" Error

Check that the path in your test matches the OpenAPI spec exactly:

```typescript
// If spec has: /users/{userId}
// Use path params, not the literal path
const result = await testKit.validateResponse({
  method: 'GET',
  path: '/users/123',  // Actual path with ID
  // ...
});
```

### "No consumers registered" Warning

This is normal if you're the first to deploy or if no consumers have registered:

```bash
cvt can-i-deploy --schema new-api --version 1.0.0 --env prod

# Output:
# ✅ SAFE TO DEPLOY
# No consumers registered for this schema in prod.
```
