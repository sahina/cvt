---
title: Use Cases
sidebar_label: Use Cases
sidebar_position: 3
description: Common CVT use cases with step-by-step guides
---

# CVT Use Cases

This guide walks you through common use cases from start to finish.

## Use Case 1: Adding Contract Tests to Your Service

**Scenario:** Your service calls an upstream API (e.g., `user-service`). You want to ensure your HTTP calls match the API contract.

### Step 1: Get the OpenAPI Schema

Obtain the OpenAPI schema from the upstream service team:

```bash
# Option A: Copy from their repo
cp ../user-service/api/openapi.json ./contracts/user-service.json

# Option B: Download from their endpoint
curl -o ./contracts/user-service.json https://user-service/openapi.json
```

### Step 2: Install the SDK

```bash
# Node.js
pnpm add @cvt/cvt-sdk

# Python
pip install cvt-sdk

# Go
go get github.com/cvt/cvt-sdk/go/cvt

# Java (add to build.gradle)
implementation 'com.cvt:cvt-sdk:1.0.0'
```

### Step 3: Start the CVT Server

```bash
# For local development
make up

# Or use Docker directly
docker run -d -p 9550:9550 cvt-server
```

### Step 4: Write Your First Contract Test

**Node.js (Jest):**

```typescript
import { ContractValidator } from "@cvt/cvt-sdk";

describe("User Service Contract", () => {
  let validator: ContractValidator;

  beforeAll(async () => {
    validator = new ContractValidator("localhost:9550");
    // Schema ID "user-service" is used to reference this schema in validate()
    await validator.registerSchema(
      "user-service",
      "./contracts/user-service.json",
    );
  });

  afterAll(() => validator.close()); // Clean up gRPC connection

  it("GET /users/{id} returns valid response", async () => {
    // Make an actual HTTP call (see Use Case 4 if you don't have API access)
    const response = await fetch("http://localhost:3000/users/123");
    const body = await response.json();

    // Validate the real response matches the OpenAPI contract
    const result = await validator.validate(
      { method: "GET", path: "/users/123" },
      { statusCode: response.status, body: JSON.stringify(body) },
    );

    expect(result.valid).toBe(true);
  });
});
```

**Python (pytest):**

```python
import pytest
from cvt_sdk import ContractValidator

@pytest.fixture(scope="module")
def validator():
    v = ContractValidator(host="localhost:9550")
    v.register_schema("user-service", "./contracts/user-service.json")
    yield v
    v.close()  # Fixture teardown closes gRPC connection

def test_get_user_returns_valid_response(validator):
    import requests
    response = requests.get("http://localhost:3000/users/123")

    # Pass actual request/response to validate against OpenAPI spec
    result = validator.validate(
        request={"method": "GET", "path": "/users/123"},
        response={"status_code": response.status_code, "body": response.text}
    )

    assert result.valid is True
```

### Step 5: Use the HTTP Adapter (Recommended)

Instead of manually validating, use the adapter for automatic validation:

```typescript
import axios from "axios";
import { ContractValidator, createAxiosAdapter } from "@cvt/cvt-sdk";

const validator = new ContractValidator("localhost:9550");
await validator.registerSchema("user-service", "./contracts/user-service.json");

const api = axios.create({ baseURL: "http://user-service" });

// Wrap axios instance - all requests/responses will be validated automatically
createAxiosAdapter({
  axios: api,
  validator,
  schemaId: "user-service",
  autoValidate: true,
  onValidationFailure: (result) => {
    throw new Error(`Contract violation: ${result.errors.join(", ")}`);
  },
});

// No manual validate() calls needed - adapter intercepts all traffic
const user = await api.get("/users/123"); // Throws if response doesn't match contract
```

### Step 6: Add to CI Pipeline

```yaml
# .github/workflows/test.yml
jobs:
  contract-tests:
    runs-on: ubuntu-latest
    services:
      cvt: # CVT server runs as a sidecar service
        image: cvt-server:latest
        ports:
          - 9550:9550
    steps:
      - uses: actions/checkout@v4
      - run: npm install
      - run: npm test # Tests connect to localhost:9550
```

---

## Use Case 2: Detecting Breaking Changes Before Deployment

**Scenario:** You're updating your API and want to ensure you don't break existing consumers.

### Step 1: Store Schema Versions

Keep your OpenAPI schemas versioned:

```shell
api/
├── openapi.json          # Current version
├── openapi-v1.0.0.json   # Previous release
└── openapi-v1.1.0.json   # New version (about to release)
```

### Step 2: Compare Schemas in CI

**Using the CLI:**

```bash
# Build the CLI
go build -o cvt ./cmd/cvt

# Compare schemas
cvt compare --old ./api/openapi-v1.0.0.json --new ./api/openapi-v1.1.0.json

# Exit code: 0 = compatible, 1 = breaking changes
```

**Using the SDK:**

```typescript
import { ContractValidator } from "@cvt/cvt-sdk";

const validator = new ContractValidator("localhost:9550");

// Register multiple versions of the same schema for comparison
await validator.registerSchemaWithVersion(
  "my-api",
  "./api/openapi-v1.0.0.json",
  "1.0.0",
);
await validator.registerSchemaWithVersion(
  "my-api",
  "./api/openapi-v1.1.0.json",
  "1.1.0",
);

// Detect breaking changes: removed endpoints, changed types, new required fields
const result = await validator.compareSchemas("my-api", "1.0.0", "1.1.0");

if (!result.compatible) {
  console.error("Breaking changes detected:");
  result.breakingChanges.forEach((change) => {
    console.error(`- [${change.type}] ${change.description}`);
  });
  process.exit(1); // Fail CI pipeline
}
```

### Step 3: Add to PR Checks

```yaml
# .github/workflows/api-check.yml
name: API Compatibility Check

on:
  pull_request:
    paths:
      - "api/openapi.json" # Only run when schema changes

jobs:
  check-breaking-changes:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # Need full history to compare with main

      - name: Get previous schema
        run: git show origin/main:api/openapi.json > /tmp/old-schema.json

      - name: Check for breaking changes
        run: |
          go build -o cvt ./cmd/cvt
          cvt compare --old /tmp/old-schema.json --new ./api/openapi.json
          # Non-zero exit fails the PR if breaking changes detected
```

---

## Use Case 3: Validating During Development (Local Workflow)

**Scenario:** You want fast feedback while developing without running Docker.

### Option A: Use the CLI Directly

```bash
# One-time build
go build -o cvt ./cmd/cvt

# Validate inline - useful for quick checks during development
cvt validate \
  --schema ./api/openapi.json \
  --method GET \
  --path "/users/123" \
  --status-code 200 \
  --response-body '{"id": 123, "name": "John"}'
# Exit code 0 = valid, non-zero = validation errors (printed to stderr)
```

### Option B: Use the Embedded Library (Go)

```go
import "github.com/cvt/cvt/pkg/cvt"

func TestMyHandler(t *testing.T) {
    // Local validator - no gRPC server needed, runs entirely in-process
    validator, _ := cvt.NewLocalValidator("./api/openapi.json")

    result, _ := validator.Validate(cvt.Interaction{
        Request:  cvt.Request{Method: "GET", Path: "/users/123"},
        Response: cvt.Response{StatusCode: 200, Body: `{"id": 123}`},
    })

    if !result.Valid {
        t.Errorf("Contract violation: %v", result.Errors)
    }
}
```

---

## Use Case 4: Testing Without API Access

**Scenario:** You need to validate your API integration code, but you don't have access to the actual API yet (pending onboarding, authentication setup, network access, etc.).

**Key insight:** You don't need to make HTTP calls to validate against a contract. CVT validates the _structure_ of requests and responses, not the actual API behavior.

### What You Can Validate Without API Access

| What                 | How                                  | Value                                              |
| -------------------- | ------------------------------------ | -------------------------------------------------- |
| Request construction | Validate what your code _would_ send | Catch malformed requests before you get API access |
| Response handling    | Validate expected response shapes    | Ensure your code handles responses correctly       |
| Error responses      | Validate error response structures   | Test error handling paths                          |

### Example: Validate Your API Client Code

```typescript
import { ContractValidator } from "@cvt/cvt-sdk";

// Your actual production code that builds requests
function buildCreateUserRequest(name: string, email: string) {
  return {
    method: "POST" as const,
    path: "/users",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, email }),
  };
}

// Your actual production code that parses responses
function parseUserResponse(body: string): {
  id: number;
  name: string;
  email: string;
} {
  return JSON.parse(body);
}

describe("User API Client", () => {
  let validator: ContractValidator;

  beforeAll(async () => {
    validator = new ContractValidator("localhost:9550");
    await validator.registerSchema("user-api", "./contracts/user-api.json");
  });

  afterAll(() => validator.close());

  it("creates valid requests and handles expected responses", async () => {
    // Test your request-building code against the contract
    const request = buildCreateUserRequest("John Doe", "john@example.com");

    // Auto-generate a valid response from the schema - no need to handcraft JSON
    const generatedResponse = await validator.generateResponse(
      "POST",
      "/users",
      { statusCode: 201 },
    );

    // Validate both request AND response are schema-compliant (no HTTP call!)
    const result = await validator.validate(request, {
      statusCode: generatedResponse.statusCode,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(generatedResponse.body),
    });
    expect(result.valid).toBe(true);

    // Verify your parser handles schema-compliant responses correctly
    const user = parseUserResponse(JSON.stringify(generatedResponse.body));
    expect(user.id).toBeDefined();
  });

  it("handles error responses correctly", async () => {
    const request = buildCreateUserRequest("", "");

    // Test that error responses also conform to the schema
    const errorResponse = {
      statusCode: 400,
      body: JSON.stringify({
        error: "Validation failed",
        details: ["name is required"],
      }),
    };

    const result = await validator.validate(request, errorResponse);
    expect(result.valid).toBe(true);
  });
});
```

### Example: Testing Multiple Endpoints

```typescript
// Table-driven tests for comprehensive API coverage
const testCases = [
  {
    name: "GET /users/:id",
    request: { method: "GET", path: "/users/123" },
    response: {
      statusCode: 200,
      body: JSON.stringify({ id: 123, name: "John" }),
    },
  },
  {
    name: "POST /users",
    request: {
      method: "POST",
      path: "/users",
      body: JSON.stringify({ name: "Jane", email: "jane@test.com" }),
    },
    response: {
      statusCode: 201,
      body: JSON.stringify({ id: 456, name: "Jane", email: "jane@test.com" }),
    },
  },
  {
    name: "DELETE /users/:id",
    request: { method: "DELETE", path: "/users/123" },
    response: { statusCode: 204 }, // No body for 204 responses
  },
];

// Each test case validates request + response structure against schema
testCases.forEach(({ name, request, response }) => {
  it(`${name} is schema-compliant`, async () => {
    const result = await validator.validate(request, response);
    expect(result.valid).toBe(true);
  });
});
```

### Benefits of This Approach

1. **Start testing immediately** - Don't wait for API access
2. **Catch issues early** - Find request construction bugs before integration
3. **Document expectations** - Tests show what you expect from the API
4. **Faster feedback** - No network calls means faster test runs
5. **Works offline** - Test on planes, in restricted networks, etc.

### When to Add Real HTTP Calls

Once you have API access, you can:

1. Keep the schema validation tests (they're still valuable)
2. Add integration tests that make real calls
3. Use the HTTP adapter for automatic validation of real traffic

---

## Use Case 5: Producer-Side API Validation

**Scenario:** You own an API and want to ensure incoming requests conform to your OpenAPI contract before they reach your business logic, and outgoing responses match the contract.

### Why Producer Validation?

| Benefit              | Description                                            |
| -------------------- | ------------------------------------------------------ |
| Early rejection      | Invalid requests fail fast with clear error messages   |
| Contract enforcement | Ensure all clients follow your published API contract  |
| Drift detection      | Catch when your implementation diverges from the spec  |
| Gradual rollout      | Start with shadow mode, progress to strict enforcement |

### Step 1: Choose Your Validation Mode

CVT supports three validation modes: **strict**, **warn**, and **shadow**. See [modes.md](modes.md) for detailed behavior, rollout strategy, and configuration examples.

**Quick reference:**

- `strict` — Reject invalid requests (production enforcement)
- `warn` — Log violations but continue (gradual rollout)
- `shadow` — Metrics only, zero impact (initial deployment)

### Step 2: Add Middleware to Your Framework

**Go (net/http):**

```go
import "github.com/cvt/cvt-sdk/go/cvt/producer"
import "github.com/cvt/cvt-sdk/go/cvt/producer/adapters"

config := producer.Config{
    SchemaID:  "my-api",
    Validator: myValidator,
    Mode:      producer.ModeStrict, // ModeWarn or ModeShadow for gradual rollout
}
// Wrap your handler - requests are validated before reaching myHandler
http.Handle("/", adapters.NetHTTPMiddleware(config)(myHandler))
```

**Go (Gin):**

```go
router := gin.Default()
router.Use(adapters.GinMiddleware(config))
```

**Node.js (Express):**

```typescript
import { createExpressMiddleware } from "@cvt/cvt-sdk/producer";

app.use(
  createExpressMiddleware({
    schemaId: "my-api",
    validator,
    mode: "strict",
  }),
);
```

**Node.js (Fastify):**

```typescript
import { createFastifyPlugin } from "@cvt/cvt-sdk/producer";

fastify.register(createFastifyPlugin({ schemaId: "my-api", validator }));
```

**Python (FastAPI):**

```python
from cvt_sdk.producer import ProducerConfig, ValidationMode
from cvt_sdk.producer.adapters import ASGIMiddleware

config = ProducerConfig(
    schema_id="my-api",
    validator=validator,
    mode=ValidationMode.STRICT,  # WARN or SHADOW for gradual rollout
)
app.add_middleware(ASGIMiddleware, config=config)
```

**Python (Flask):**

```python
from cvt_sdk.producer.adapters import WSGIMiddleware

# Wrap the WSGI app - works with any WSGI framework
app.wsgi_app = WSGIMiddleware(app.wsgi_app, config=config)
```

**Java (Spring):**

```java
ProducerConfig config = ProducerConfig.builder()
    .schemaId("my-api")
    .validator(myValidator)
    .mode(ValidationMode.STRICT)  // WARN or SHADOW available
    .build();

// Interceptor runs before @Controller methods
registry.addInterceptor(new SpringInterceptor(config))
    .addPathPatterns("/api/**");
```

**Java (Servlet Filter):**

```java
// Works with any Servlet-based framework (Tomcat, Jetty, etc.)
FilterRegistrationBean<ServletFilter> registration = new FilterRegistrationBean<>();
registration.setFilter(new ServletFilter(config));
registration.addUrlPatterns("/api/*");
```

### Step 3: Gradual Production Rollout

Follow the recommended rollout strategy: `shadow` → `warn` → `strict`. See [modes.md#recommended-rollout-strategy](modes.md#recommended-rollout-strategy) for the detailed step-by-step guide.

### Step 4: Path Filtering (Optional)

Exclude health checks, metrics, or other paths from validation:

```typescript
createExpressMiddleware({
  schemaId: "my-api",
  validator,
  mode: "strict",
  excludePaths: ["/health", "/metrics", "/ready"], // Skip internal endpoints
  includePaths: ["/api/**"], // Only validate paths under /api/
});
```

### Step 5: Migrating from Consumer-Only to Full Contract Testing

If your team already uses CVT for consumer validation (Use Cases 1-4) and you also own APIs, here's how to adopt full contract testing:

**Understanding the Two Sides:**

```text
┌────────────────────────────────────────────────────────────────┐
│                        Your Service                            │
│                                                                │
│   ┌─────────────────┐                    ┌─────────────────┐   │
│   │ Consumer Tests  │                    │ Producer Tests  │   │
│   │ (Use Cases 1-4) │                    │ (Use Case 5)    │   │
│   │                 │                    │                 │   │
│   │ Validates YOUR  │                    │ Validates       │   │
│   │ calls TO other  │                    │ THEIR calls TO  │   │
│   │ APIs            │                    │ your API        │   │
│   └────────┬────────┘                    └────────┬────────┘   │
│            │                                      │            │
│            ▼                                      ▼            │
│   ┌─────────────────┐                    ┌─────────────────┐   │
│   │  HTTP Client    │                    │  HTTP Server    │   │
│   │  (axios, fetch) │                    │  (express, gin) │   │
│   └────────┬────────┘                    └────────┬────────┘   │
└────────────┼──────────────────────────────────────┼────────────┘
             │                                      │
             ▼                                      ▼
      ┌──────────────┐                      ┌──────────────┐
      │ Upstream API │                      │   Clients    │
      │ (user-svc)   │                      │ (mobile app) │
      └──────────────┘                      └──────────────┘
```

**Migration Checklist:**

| Step | Action                                   | Why                                          |
| ---- | ---------------------------------------- | -------------------------------------------- |
| 1    | Keep existing consumer tests             | They validate your outbound calls still work |
| 2    | Identify APIs you own                    | Which endpoints do other teams call?         |
| 3    | Ensure OpenAPI spec is up-to-date        | Producer validation requires accurate specs  |
| 4    | Add producer middleware in `shadow` mode | Monitor without breaking anything            |
| 5    | Review validation failures               | Fix spec or implementation mismatches        |
| 6    | Promote to `strict` mode                 | Full contract enforcement                    |

**Example: Service That Both Consumes and Produces:**

```typescript
import { ContractValidator } from "@cvt/cvt-sdk";
import { createAxiosAdapter } from "@cvt/cvt-sdk/adapters";
import { createExpressMiddleware } from "@cvt/cvt-sdk/producer";

// Single validator instance handles both consumer and producer validation
const validator = new ContractValidator("localhost:9550");

// Register schemas for APIs you CONSUME (other teams' APIs)
await validator.registerSchema("user-service", "./contracts/user-service.json");
await validator.registerSchema(
  "payment-service",
  "./contracts/payment-service.json",
);

// Register schema for API you PRODUCE (your own API)
await validator.registerSchema("order-service", "./api/openapi.json");

// CONSUMER: Validate your outbound calls to other services
const userApi = axios.create({ baseURL: "http://user-service" });
createAxiosAdapter({ axios: userApi, validator, schemaId: "user-service" });

const paymentApi = axios.create({ baseURL: "http://payment-service" });
createAxiosAdapter({
  axios: paymentApi,
  validator,
  schemaId: "payment-service",
});

// PRODUCER: Validate inbound calls from your clients
app.use(
  createExpressMiddleware({
    schemaId: "order-service",
    validator,
    mode: "strict",
  }),
);
```

**Benefits of Full Contract Testing:**

| Aspect                            | Consumer Only | Full Contract Testing |
| --------------------------------- | ------------- | --------------------- |
| Validates your calls to others    | ✅            | ✅                    |
| Validates others' calls to you    | ❌            | ✅                    |
| Catches your breaking changes     | ❌            | ✅                    |
| Catches upstream breaking changes | ✅            | ✅                    |
| Complete API coverage             | Partial       | Complete              |
