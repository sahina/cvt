---
name: cvt-setup
description: Set up CVT contract testing in a new project
sdk_version: "0.5.0"
---

# CVT Setup

## Prerequisites

- A running CVT server (local or remote) on port 9550, or Docker installed to start one
- An OpenAPI v2 (Swagger) or v3 schema for the API you consume
- One of: Node.js 18+, Python 3.9+, Go 1.21+, or Java 17+ with Maven

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

Install the CVT SDK for the detected language, configure the server connection, and create a minimal contract test file that registers a schema and validates one interaction.

## Steps

1. **Detect project language** using the auto-detection block above.
2. **Install the SDK** using the language-specific install command.
3. **Verify server connectivity** by running `cvt wait --server localhost:9550 --timeout 10` or by checking the server address provided by the user.
4. **Locate or obtain the OpenAPI schema** (see "No schema?" below).
5. **Create a contract test file** with a minimal example that registers the schema and validates one GET request.
6. **Run the test** to confirm everything is wired up.

## SDK-Specific Instructions

### Node.js

Install:
```bash
npm install --save-dev @sahina/cvt-sdk
```

Create `tests/contract.test.ts` (or `.js`):
```typescript
import { ContractValidator } from "@sahina/cvt-sdk";

describe("Contract Tests", () => {
  let validator: ContractValidator;

  beforeAll(async () => {
    validator = new ContractValidator("localhost:9550");
    await validator.registerSchema("my-api", "./openapi.json");
  });

  afterAll(async () => {
    await validator.close();
  });

  it("GET /health matches contract", async () => {
    const result = await validator.validate(
      { method: "GET", path: "/health", headers: {} },
      { statusCode: 200, headers: { "Content-Type": "application/json" }, body: '{"status":"ok"}' }
    );
    expect(result.valid).toBe(true);
  });
});
```

### Python

Install:
```bash
pip install cvt-sdk
# or with uv:
uv add --dev cvt-sdk
```

Create `tests/test_contract.py`:
```python
import pytest
from cvt_sdk import ContractValidator

@pytest.fixture(scope="module")
def validator():
    v = ContractValidator("localhost:9550")
    v.register_schema("my-api", "./openapi.json")
    yield v
    v.close()

def test_get_health(validator):
    result = validator.validate(
        {"method": "GET", "path": "/health", "headers": {}},
        {"status_code": 200, "headers": {"Content-Type": "application/json"}, "body": '{"status":"ok"}'}
    )
    assert result["valid"]
```

### Go

Install:
```bash
go get github.com/sahina/cvt/sdks/go/cvt
```

Create `contract_test.go`:
```go
package myapp_test

import (
    "context"
    "testing"
    "github.com/sahina/cvt/sdks/go/cvt"
)

func TestContract(t *testing.T) {
    ctx := context.Background()
    v, err := cvt.NewValidator("localhost:9550")
    if err != nil {
        t.Fatal(err)
    }
    defer v.Close()

    err = v.RegisterSchema(ctx, "my-api", "./openapi.json")
    if err != nil {
        t.Fatal(err)
    }

    result, err := v.Validate(ctx,
        &cvt.Request{Method: "GET", Path: "/health", Headers: map[string]string{}},
        &cvt.Response{StatusCode: 200, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"status":"ok"}`},
    )
    if err != nil {
        t.Fatal(err)
    }
    if !result.Valid {
        t.Errorf("validation failed: %v", result.Errors)
    }
}
```

### Java

Add to `pom.xml`:
```xml
<dependency>
  <groupId>io.github.sahina</groupId>
  <artifactId>cvt-sdk</artifactId>
  <version>0.1.0</version>
  <scope>test</scope>
</dependency>
```

Create `src/test/java/ContractTest.java`:
```java
import io.github.sahina.sdk.ContractValidator;
import org.junit.jupiter.api.*;
import static org.junit.jupiter.api.Assertions.*;

class ContractTest {
    static ContractValidator validator;

    @BeforeAll
    static void setup() {
        validator = ContractValidator.builder()
            .address("localhost:9550")
            .build();
        validator.registerSchema("my-api", "./openapi.json");
    }

    @AfterAll
    static void teardown() {
        validator.close();
    }

    @Test
    void getHealthMatchesContract() {
        var result = validator.validate(
            Map.of("method", "GET", "path", "/health", "headers", Map.of()),
            Map.of("statusCode", 200, "headers", Map.of("Content-Type", "application/json"), "body", "{\"status\":\"ok\"}")
        );
        assertTrue(result.isValid());
    }
}
```

## No Schema?

If you do not have the provider's OpenAPI schema, try these common locations:

| URL Pattern | Notes |
|---|---|
| `{base_url}/openapi.json` | OpenAPI v3 default |
| `{base_url}/openapi.yaml` | OpenAPI v3 YAML variant |
| `{base_url}/swagger.json` | Swagger / OpenAPI v2 |
| `{base_url}/v3/api-docs` | Spring Boot (Springdoc) |
| `{base_url}/v2/api-docs` | Spring Boot (Springfox) |
| `{base_url}/swagger-ui/index.html` | Swagger UI (inspect network tab for spec URL) |
| `{base_url}/api/docs` | Common custom path |

You can also ask the provider team for their schema file, or check their repository for `openapi.json`, `openapi.yaml`, or `swagger.json` at the root.

CVT accepts both v2 and v3 schemas -- v2 schemas are auto-converted to v3 during registration.

## Common Errors

| Error | Cause | Fix |
|---|---|---|
| `connection refused on :9550` | CVT server not running | Start server with `cvt serve` or `docker run -p 9550:9550 ghcr.io/sahina/cvt:latest` |
| `schema parse error` | Invalid or unreachable schema file | Verify the schema path exists and is valid JSON/YAML |
| `LANG=unknown` | No recognized project file found | Manually specify the language and create the project file first |
| `SDK not found after install` | Package not in dependency file | Verify the install command succeeded and check your lock file |

## Success Criteria

- The SDK is listed in the project's dependency/lock file
- A contract test file exists in the test directory
- Running the test produces a passing result with `valid: true`
- The CVT server shows the schema registered in its logs
