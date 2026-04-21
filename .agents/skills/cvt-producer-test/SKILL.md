---
name: cvt-producer-test
description: Validate API responses against OpenAPI schema using ProducerTestKit
sdk_version: "0.7.0"
---

# CVT Producer Testing

## Prerequisites

- CVT SDK installed in the project (run `/cvt-setup` first if not)
- A running CVT server on port 9550
- The provider's own OpenAPI schema (the schema this API implements)
- A working API server or test harness that can handle HTTP requests

## Auto-Detection

```bash
if [ -f package.json ]; then echo "LANG=node"
elif [ -f pyproject.toml ] || [ -f setup.py ]; then echo "LANG=python"
elif [ -f go.mod ]; then echo "LANG=go"
elif [ -f pom.xml ] || [ -f build.gradle ]; then echo "LANG=java"
else echo "LANG=unknown"
fi
```

## Goal

Set up producer-side contract tests that validate your API's actual responses against the OpenAPI schema using the CVT SDK's `ProducerTestKit`. This ensures the API implementation matches the contract it advertises.

## Steps

1. **Detect project language** using the auto-detection block.
2. **Register the schema** with the CVT server using the SDK or CLI.
3. **Choose a framework adapter** appropriate to your API framework.
4. **Write producer tests** that hit actual endpoints and validate responses.
5. **Run the tests** and verify all responses conform to the schema.

## SDK-Specific Instructions

### Node.js

Supported framework adapters: **Express**, **Fastify**

```typescript
import { ProducerTestKit } from "@sahina/cvt-sdk";
import app from "../src/app"; // Your Express/Fastify app

describe("Producer Contract Tests", () => {
  let kit: ProducerTestKit;

  beforeAll(async () => {
    kit = new ProducerTestKit("localhost:9550");
    await kit.registerSchema("my-api", "./openapi.json");
  });

  afterAll(async () => {
    await kit.close();
  });

  it("GET /users/:id returns valid response", async () => {
    const result = await kit
      .forEndpoint("GET", "/users/{id}")
      .validateResponse(app, {
        pathParams: { id: "123" },
        expectedStatus: 200
      });
    expect(result.valid).toBe(true);
  });

  it("POST /users returns valid response", async () => {
    const result = await kit
      .forEndpoint("POST", "/users")
      .validateInteraction(app, {
        body: { name: "Alice", email: "alice@example.com" },
        headers: { "Content-Type": "application/json" },
        expectedStatus: 201
      });
    expect(result.valid).toBe(true);
  });

  it("GET /users/:id with invalid ID returns valid 404", async () => {
    const result = await kit
      .forEndpoint("GET", "/users/{id}")
      .validateResponse(app, {
        pathParams: { id: "nonexistent" },
        expectedStatus: 404
      });
    expect(result.valid).toBe(true);
  });
});
```

### Python

Supported framework adapters: **Flask**, **Django**

```python
import pytest
from cvt_sdk import ProducerTestKit
from myapp import create_app  # Your Flask/Django app factory

@pytest.fixture(scope="module")
def kit():
    k = ProducerTestKit("localhost:9550")
    k.register_schema("my-api", "./openapi.json")
    yield k
    k.close()

@pytest.fixture(scope="module")
def app():
    return create_app()

def test_get_user_response(kit, app):
    result = kit \
        .for_endpoint("GET", "/users/{id}") \
        .validate_response(app, path_params={"id": "123"}, expected_status=200)
    assert result["valid"]

def test_create_user_interaction(kit, app):
    result = kit \
        .for_endpoint("POST", "/users") \
        .validate_interaction(
            app,
            body={"name": "Alice", "email": "alice@example.com"},
            headers={"Content-Type": "application/json"},
            expected_status=201
        )
    assert result["valid"]

def test_get_user_not_found(kit, app):
    result = kit \
        .for_endpoint("GET", "/users/{id}") \
        .validate_response(app, path_params={"id": "nonexistent"}, expected_status=404)
    assert result["valid"]
```

### Go

Supported framework adapters: **Chi**, **Gin**, **net/http**

```go
package producer_test

import (
    "context"
    "net/http"
    "testing"
    "github.com/sahina/cvt/sdks/go/cvt"
)

func TestProducerContract(t *testing.T) {
    ctx := context.Background()
    kit, err := cvt.NewProducerTestKit("localhost:9550")
    if err != nil {
        t.Fatal(err)
    }
    defer kit.Close()

    if err := kit.RegisterSchema(ctx, "my-api", "./openapi.json"); err != nil {
        t.Fatal(err)
    }

    // Assumes your app implements http.Handler
    handler := setupRouter() // Your Chi/Gin/http.ServeMux router

    t.Run("GET /users/{id} returns valid response", func(t *testing.T) {
        result, err := kit.ForEndpoint("GET", "/users/{id}").
            ValidateResponse(ctx, handler, cvt.ProducerOpts{
                PathParams:     map[string]string{"id": "123"},
                ExpectedStatus: http.StatusOK,
            })
        if err != nil {
            t.Fatal(err)
        }
        if !result.Valid {
            t.Errorf("response invalid: %v", result.Errors)
        }
    })

    t.Run("POST /users returns valid response", func(t *testing.T) {
        result, err := kit.ForEndpoint("POST", "/users").
            ValidateInteraction(ctx, handler, cvt.ProducerOpts{
                Body:           `{"name":"Alice","email":"alice@example.com"}`,
                Headers:        map[string]string{"Content-Type": "application/json"},
                ExpectedStatus: http.StatusCreated,
            })
        if err != nil {
            t.Fatal(err)
        }
        if !result.Valid {
            t.Errorf("interaction invalid: %v", result.Errors)
        }
    })
}
```

### Java

Supported framework adapters: **Spring**, **Servlet**

```java
import io.github.sahina.sdk.ProducerTestKit;
import org.junit.jupiter.api.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import java.util.Map;
import static org.junit.jupiter.api.Assertions.*;

@SpringBootTest
@AutoConfigureMockMvc
class ProducerContractTest {
    static ProducerTestKit kit;

    @Autowired
    MockMvc mockMvc;

    @BeforeAll
    static void setup() {
        kit = ProducerTestKit.builder()
            .address("localhost:9550")
            .build();
        kit.registerSchema("my-api", "./openapi.json");
    }

    @AfterAll
    static void teardown() {
        kit.close();
    }

    @Test
    void getUserReturnsValidResponse() {
        var result = kit
            .forEndpoint("GET", "/users/{id}")
            .validateResponse(mockMvc, Map.of(
                "pathParams", Map.of("id", "123"),
                "expectedStatus", 200
            ));
        assertTrue(result.isValid());
    }

    @Test
    void createUserReturnsValidResponse() {
        var result = kit
            .forEndpoint("POST", "/users")
            .validateInteraction(mockMvc, Map.of(
                "body", "{\"name\":\"Alice\",\"email\":\"alice@example.com\"}",
                "headers", Map.of("Content-Type", "application/json"),
                "expectedStatus", 201
            ));
        assertTrue(result.isValid());
    }

    @Test
    void getUserNotFoundReturnsValid404() {
        var result = kit
            .forEndpoint("GET", "/users/{id}")
            .validateResponse(mockMvc, Map.of(
                "pathParams", Map.of("id", "nonexistent"),
                "expectedStatus", 404
            ));
        assertTrue(result.isValid());
    }
}
```

## Producer Testing Patterns

### Test Every Documented Status Code

If your schema defines 200, 400, and 404 for an endpoint, write a producer test for each:

```
GET /users/{id} -> 200 (valid user)
GET /users/{id} -> 404 (user not found)
POST /users    -> 201 (created)
POST /users    -> 400 (validation error)
```

### Use Realistic Test Data

Seed your test database or use fixtures that produce realistic responses. Avoid empty objects or placeholder values -- the schema validation checks field types and required properties.

### Test with the Actual Application

Producer tests should exercise the real application handler (not mocks). The point is to catch drift between the schema and the implementation.

### Validate Error Responses Too

Error responses (4xx, 5xx) must also conform to the schema. If your schema defines an error response format, make sure your API returns it consistently.

## Common Errors

| Error | Cause | Fix |
|---|---|---|
| `response body has an error: additionalProperties` | API returns fields not in the schema | Add the fields to the schema or set `additionalProperties: true` |
| `response status code not defined` | API returns a status code not in the schema | Add the status code to the schema for this endpoint |
| `response body has an error: value is required` | API omits a required field | Fix the API to return the required field, or make it optional in the schema |
| `response body has an error: type mismatch` | Field type differs (e.g., returning string for integer) | Fix the API serialization or update the schema type |
| `no route matched` | The test path does not match any schema endpoint | Verify the path template matches the schema exactly |
| `handler returned unexpected status` | App returned a different status than expected | Check test data setup; ensure the test scenario triggers the expected status |

## Success Criteria

- Producer test file exists with tests for primary endpoints
- Tests exercise the real application handler (not mocks)
- All documented status codes for each endpoint have a test
- All tests pass with `valid: true` from the CVT server
- Tests run as part of the standard test suite (`npm test`, `pytest`, `go test`, `mvn test`)
