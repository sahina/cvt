---
name: cvt-write-contract
description: Write a consumer contract test against an OpenAPI schema
sdk_version: "0.5.0"
---

# Write Consumer Contract Test

## Prerequisites

- CVT SDK installed in the project (run `/cvt-setup` first if not)
- A CVT server running on port 9550 (or a custom address)
- The provider's OpenAPI schema registered or available as a file/URL
- A specific API endpoint the consumer depends on

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

Write a consumer contract test that validates a specific HTTP request/response interaction against the provider's OpenAPI schema using the CVT SDK `validate()` method.

## Steps

1. **Detect project language** using the auto-detection block.
2. **Identify the endpoint** the consumer depends on (method, path, expected request body, expected response).
3. **Review the schema** to understand required fields, status codes, and content types for that endpoint.
4. **Write the contract test** that constructs realistic request and response objects and calls `validate()`.
5. **Include edge cases** -- test both the happy path (2xx) and at least one error path (4xx) if the consumer handles errors.
6. **Run the test** and verify all validations pass.

## SDK-Specific Instructions

### Node.js

File: `tests/contract/<endpoint>.test.ts`

```typescript
import { ContractValidator } from "@sahina/cvt-sdk";

describe("POST /users contract", () => {
  let validator: ContractValidator;

  beforeAll(async () => {
    validator = new ContractValidator("localhost:9550");
    await validator.registerSchema("user-api", "./openapi.json");
  });

  afterAll(async () => {
    await validator.close();
  });

  it("validates successful user creation", async () => {
    const result = await validator.validate(
      {
        method: "POST",
        path: "/users",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Alice", email: "alice@example.com" })
      },
      {
        statusCode: 201,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: 1, name: "Alice", email: "alice@example.com" })
      }
    );
    expect(result.valid).toBe(true);
    expect(result.errors).toHaveLength(0);
  });

  it("validates validation error response", async () => {
    const result = await validator.validate(
      {
        method: "POST",
        path: "/users",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "" })
      },
      {
        statusCode: 400,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ error: "validation_error", message: "name is required" })
      }
    );
    expect(result.valid).toBe(true);
  });
});
```

### Python

File: `tests/contract/test_<endpoint>.py`

```python
import pytest
from cvt_sdk import ContractValidator

@pytest.fixture(scope="module")
def validator():
    v = ContractValidator("localhost:9550")
    v.register_schema("user-api", "./openapi.json")
    yield v
    v.close()

def test_create_user_success(validator):
    result = validator.validate(
        {
            "method": "POST",
            "path": "/users",
            "headers": {"Content-Type": "application/json"},
            "body": '{"name": "Alice", "email": "alice@example.com"}'
        },
        {
            "status_code": 201,
            "headers": {"Content-Type": "application/json"},
            "body": '{"id": 1, "name": "Alice", "email": "alice@example.com"}'
        }
    )
    assert result["valid"]
    assert len(result["errors"]) == 0

def test_create_user_validation_error(validator):
    result = validator.validate(
        {
            "method": "POST",
            "path": "/users",
            "headers": {"Content-Type": "application/json"},
            "body": '{"name": ""}'
        },
        {
            "status_code": 400,
            "headers": {"Content-Type": "application/json"},
            "body": '{"error": "validation_error", "message": "name is required"}'
        }
    )
    assert result["valid"]
```

### Go

File: `contract/<endpoint>_test.go`

```go
package contract_test

import (
    "context"
    "testing"
    "github.com/sahina/cvt/sdks/go/cvt"
)

func TestCreateUser(t *testing.T) {
    ctx := context.Background()
    v, err := cvt.NewValidator("localhost:9550")
    if err != nil {
        t.Fatal(err)
    }
    defer v.Close()

    if err := v.RegisterSchema(ctx, "user-api", "./openapi.json"); err != nil {
        t.Fatal(err)
    }

    t.Run("success", func(t *testing.T) {
        result, err := v.Validate(ctx,
            &cvt.Request{
                Method:  "POST",
                Path:    "/users",
                Headers: map[string]string{"Content-Type": "application/json"},
                Body:    `{"name":"Alice","email":"alice@example.com"}`,
            },
            &cvt.Response{
                StatusCode: 201,
                Headers:    map[string]string{"Content-Type": "application/json"},
                Body:       `{"id":1,"name":"Alice","email":"alice@example.com"}`,
            },
        )
        if err != nil {
            t.Fatal(err)
        }
        if !result.Valid {
            t.Errorf("expected valid, got errors: %v", result.Errors)
        }
    })

    t.Run("validation error", func(t *testing.T) {
        result, err := v.Validate(ctx,
            &cvt.Request{
                Method:  "POST",
                Path:    "/users",
                Headers: map[string]string{"Content-Type": "application/json"},
                Body:    `{"name":""}`,
            },
            &cvt.Response{
                StatusCode: 400,
                Headers:    map[string]string{"Content-Type": "application/json"},
                Body:       `{"error":"validation_error","message":"name is required"}`,
            },
        )
        if err != nil {
            t.Fatal(err)
        }
        if !result.Valid {
            t.Errorf("expected valid, got errors: %v", result.Errors)
        }
    })
}
```

### Java

File: `src/test/java/contract/CreateUserContractTest.java`

```java
import io.github.sahina.sdk.ContractValidator;
import org.junit.jupiter.api.*;
import java.util.Map;
import static org.junit.jupiter.api.Assertions.*;

class CreateUserContractTest {
    static ContractValidator validator;

    @BeforeAll
    static void setup() {
        validator = ContractValidator.builder()
            .address("localhost:9550")
            .build();
        validator.registerSchema("user-api", "./openapi.json");
    }

    @AfterAll
    static void teardown() {
        validator.close();
    }

    @Test
    void createUserSuccess() {
        var result = validator.validate(
            Map.of(
                "method", "POST",
                "path", "/users",
                "headers", Map.of("Content-Type", "application/json"),
                "body", "{\"name\":\"Alice\",\"email\":\"alice@example.com\"}"
            ),
            Map.of(
                "statusCode", 201,
                "headers", Map.of("Content-Type", "application/json"),
                "body", "{\"id\":1,\"name\":\"Alice\",\"email\":\"alice@example.com\"}"
            )
        );
        assertTrue(result.isValid());
        assertEquals(0, result.getErrors().size());
    }

    @Test
    void createUserValidationError() {
        var result = validator.validate(
            Map.of(
                "method", "POST",
                "path", "/users",
                "headers", Map.of("Content-Type", "application/json"),
                "body", "{\"name\":\"\"}"
            ),
            Map.of(
                "statusCode", 400,
                "headers", Map.of("Content-Type", "application/json"),
                "body", "{\"error\":\"validation_error\",\"message\":\"name is required\"}"
            )
        );
        assertTrue(result.isValid());
    }
}
```

## Writing Good Contract Tests

- **Test what you consume**: only validate the fields your consumer actually reads from the response.
- **Use realistic data**: avoid placeholder values like "string" or 0 -- use data that resembles production.
- **Test each status code**: if your consumer handles 200, 400, and 404 differently, write a test for each.
- **Include required headers**: always set `Content-Type` on requests with bodies and on responses.
- **Path parameters**: use actual values, not templates (e.g., `/users/123` not `/users/{id}`).
- **Body as string**: the request and response body fields expect serialized JSON strings, not objects.

## Common Errors

| Error | Cause | Fix |
|---|---|---|
| `no route matched` | Path does not exist in the schema | Check the schema for the exact path, including prefix (e.g., `/api/v1/users`) |
| `request body has an error` | Request body does not match the schema | Ensure all required fields are present and types match |
| `response body has an error` | Response body does not match the schema | Check field names, types, and required properties against the schema |
| `status code not defined` | Response status code not in the schema | Verify the schema defines the status code you are testing (200, 201, 400, etc.) |
| `header Content-Type has unexpected value` | Wrong or missing Content-Type | Set Content-Type to match the schema's content type (usually `application/json`) |

## Success Criteria

- Contract test file exists with at least one happy-path and one error-path test
- All tests pass with `valid: true` from the CVT server
- Tests cover the specific endpoints the consumer depends on
- Request and response structures match the OpenAPI schema exactly
