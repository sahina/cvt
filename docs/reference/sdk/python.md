---
title: Python SDK
sidebar_label: Python
sidebar_position: 3
description: CVT SDK for Python
---

# Python SDK

## What is the Python SDK?

The Python SDK provides contract validation for Python applications. It includes HTTP client adapters for automatic validation with the `requests` library, producer middleware for FastAPI and Flask, and a test kit for schema compliance testing.

:::tip SDK Architecture
For information about SDK design patterns, adapter architecture, and cross-language consistency, see [SDK Architecture](../architecture/sdk-architecture.md).
:::

## Installation

Install from a GitHub Release asset:

```bash
pip install "cvt-sdk @ https://github.com/sahina/cvt/releases/download/v0.1.0/cvt_sdk-0.1.0-py3-none-any.whl"

# Or with uv
uv add "cvt-sdk @ https://github.com/sahina/cvt/releases/download/v0.1.0/cvt_sdk-0.1.0-py3-none-any.whl"
```

Replace `v0.1.0` in the URL and `0.1.0` in the filename with the desired version. Available versions can be found on the [Releases](https://github.com/sahina/cvt/releases) page.

## Quick Start

```python
from cvt_sdk import ContractValidator

validator = ContractValidator("localhost:9550")

# Register a schema from file
validator.register_schema("petstore", "./openapi.json")
# Or register from URL:
# validator.register_schema("petstore", "https://petstore3.swagger.io/api/v3/openapi.json")

# Validate an interaction
result = validator.validate(
    request={"method": "GET", "path": "/pet/123"},
    response={"status_code": 200, "body": {"id": 123, "name": "doggie", "status": "available"}}
)

print(result["valid"])  # True or False
if not result["valid"]:
    print("Errors:", result["errors"])

# Clean up
validator.close()
```

## API Reference

### ContractValidator

#### Constructor

```python
# Simple usage (insecure connection)
ContractValidator(address: str = "localhost:9550")

# With options (TLS and API key)
ContractValidator(options: ContractValidatorOptions)
```

**Simple usage:**

```python
validator = ContractValidator("localhost:9550")
```

**With options:**

```python
from cvt_sdk import ContractValidator, ContractValidatorOptions, TLSOptions

options = ContractValidatorOptions(
    address="localhost:9550",
    tls=TLSOptions(
        enabled=True,
        root_cert_path="./certs/ca.crt",
    ),
    api_key="your-api-key",
)
validator = ContractValidator(options)
```

| Option               | Type   | Description                                |
| -------------------- | ------ | ------------------------------------------ |
| `address`            | `str`  | Server address (default: `localhost:9550`) |
| `tls.enabled`        | `bool` | Enable TLS                                 |
| `tls.root_cert_path` | `str`  | Path to CA certificate                     |
| `tls.cert_path`      | `str`  | Path to client certificate (for mTLS)      |
| `tls.key_path`       | `str`  | Path to client private key (for mTLS)      |
| `api_key`            | `str`  | API key for authentication                 |

#### Methods

##### register_schema

Registers an OpenAPI schema from a file path or URL.

```python
def register_schema(schema_id: str, schema_path: str) -> None
```

```python
# From local file
validator.register_schema("petstore", "./openapi.json")

# From URL
validator.register_schema("petstore", "https://petstore3.swagger.io/api/v3/openapi.json")
```

##### register_schema_with_version

Registers a schema with version information for comparison.

```python
def register_schema_with_version(schema_id: str, schema_path: str, version: str) -> None
```

##### validate

Validates an HTTP request/response pair against the registered schema.

```python
def validate(request: dict, response: dict) -> ValidationResult
```

Returns a dict with `valid` (bool) and `errors` (list of strings).

##### compare_schemas

Compares two schema versions for breaking changes.

```python
def compare_schemas(schema_id: str, old_version: str = "", new_version: str = "") -> CompareResult
```

##### generate_fixture

Generates test fixtures from the schema.

```python
def generate_fixture(method: str, path: str, options: GenerateOptions = None) -> GeneratedFixture
```

##### generate_response

Generates a response fixture only.

```python
def generate_response(method: str, path: str, options: GenerateOptions = None) -> GeneratedResponse
```

##### generate_request_body

Generates a request body fixture for an endpoint.

```python
def generate_request_body(method: str, path: str, options: GenerateOptions = None) -> Any
```

```python
body = validator.generate_request_body("POST", "/pet")
print(body)  # {"name": "string", "status": "available"}
```

##### list_endpoints

Lists all endpoints in the registered schema.

```python
def list_endpoints() -> list[EndpointInfo]
```

##### register_consumer

Registers a consumer with expected interactions.

```python
def register_consumer(options: RegisterConsumerOptions) -> ConsumerInfo
```

##### list_consumers

Lists all consumers for a schema.

```python
def list_consumers(schema_id: str, environment: str = None) -> list[ConsumerInfo]
```

##### deregister_consumer

Removes a consumer registration.

```python
def deregister_consumer(consumer_id: str, schema_id: str, environment: str = None) -> None
```

##### can_i_deploy

Checks if a schema version can be safely deployed.

```python
def can_i_deploy(schema_id: str, new_version: str, environment: str = "prod") -> CanIDeployResult
```

##### close

Closes the gRPC connection.

```python
def close() -> None
```

## HTTP Adapters

### Requests Adapter

Use `ContractValidatingSession` as a drop-in replacement for `requests.Session`:

```python
from cvt_sdk import ContractValidator
from cvt_sdk.adapters import ContractValidatingSession

validator = ContractValidator("localhost:9550")
validator.register_schema("petstore", "./openapi.json")

# Create a validating session
session = ContractValidatingSession(
    validator,
    auto_validate=True,
    on_validation_failure=lambda result, req, resp: print(f"Contract violation: {result['errors']}"),
)

# All requests are now validated
response = session.get("http://petstore-service/pet/123")

# Check captured interactions
interactions = session.get_interactions()
print(interactions[0].validation_result["valid"])
```

Or use the factory function:

```python
from cvt_sdk.adapters import create_validating_session

session = create_validating_session(
    validator,
    auto_validate=True,
)
```

## Producer Middleware

### FastAPI

```python
from fastapi import FastAPI
from cvt_sdk import ContractValidator
from cvt_sdk.producer import ProducerConfig, ValidationMode
from cvt_sdk.producer.adapters import ASGIMiddleware

app = FastAPI()

validator = ContractValidator("localhost:9550")
validator.register_schema("petstore", "./openapi.json")

config = ProducerConfig(
    schema_id="petstore",
    validator=validator,
    mode=ValidationMode.STRICT,  # STRICT | WARN | SHADOW
)

app.add_middleware(ASGIMiddleware, config=config)

@app.get("/pet/{pet_id}")
async def get_pet(pet_id: int):
    return {"id": pet_id, "name": "doggie", "status": "available"}
```

### Flask

```python
from flask import Flask, jsonify
from cvt_sdk import ContractValidator
from cvt_sdk.producer import ProducerConfig, ValidationMode
from cvt_sdk.producer.adapters import WSGIMiddleware

app = Flask(__name__)

validator = ContractValidator("localhost:9550")
validator.register_schema("petstore", "./openapi.json")

config = ProducerConfig(
    schema_id="petstore",
    validator=validator,
    mode=ValidationMode.WARN,
)

app.wsgi_app = WSGIMiddleware(app.wsgi_app, config)

@app.route("/pet/<int:pet_id>")
def get_pet(pet_id):
    return jsonify({"id": pet_id, "name": "doggie", "status": "available"})
```

## Producer Test Kit

Test your API responses against your schema without real consumers:

```python
import pytest
from cvt_sdk.producer import ProducerTestKit, ProducerTestConfig

@pytest.fixture
def test_kit():
    kit = ProducerTestKit(ProducerTestConfig(
        schema_id="petstore",
        server_address="localhost:9550",
    ))
    yield kit
    kit.close()

def test_get_pet_returns_valid_response(test_kit):
    result = test_kit.validate_response(
        method="GET",
        path="/pet/123",
        status_code=200,
        body={"id": 123, "name": "doggie", "status": "available"},
    )

    assert result.valid
    assert len(result.errors) == 0

def test_detects_invalid_response(test_kit):
    result = test_kit.validate_response(
        method="GET",
        path="/pet/123",
        status_code=200,
        body={"id": "not-a-number"},  # Missing required fields
    )

    assert not result.valid
    assert len(result.errors) > 0
```

## Auto-Registration

Build consumer registrations from captured mock interactions:

```python
from cvt_sdk.adapters import create_validating_session

# Capture interactions during tests
session = create_validating_session(validator, auto_validate=True)
session.get("http://mock.petstore/pet/123")
session.post("http://mock.petstore/pet", json={"name": "doggie"})

# Build registration options (preview)
opts = validator.build_consumer_from_interactions(
    session.get_interactions(),
    {
        "consumer_id": "order-service",
        "consumer_version": "2.1.0",
        "environment": "dev",
        "schema_version": "1.0.0",
    }
)
print(f"Would register {len(opts.get('used_endpoints', []))} endpoints")

# Or register directly
info = validator.register_consumer_from_interactions(
    session.get_interactions(),
    {
        "consumer_id": "order-service",
        "consumer_version": "2.1.0",
        "environment": "dev",
        "schema_version": "1.0.0",
    }
)
```

### build_consumer_from_interactions

Builds consumer registration options from captured interactions without registering.

```python
def build_consumer_from_interactions(
    interactions: list[CapturedInteraction],
    config: AutoRegisterConfig
) -> RegisterConsumerOptions
```

### register_consumer_from_interactions

Registers a consumer from captured interactions (combines `build_consumer_from_interactions` + `register_consumer`).

```python
def register_consumer_from_interactions(
    interactions: list[CapturedInteraction],
    config: AutoRegisterConfig
) -> ConsumerInfo
```

## TLS Configuration

```python
from cvt_sdk import ContractValidator, ContractValidatorOptions, TLSOptions

validator = ContractValidator(ContractValidatorOptions(
    address="localhost:9550",
    tls=TLSOptions(
        enabled=True,
        root_cert_path="./certs/ca.crt",
    ),
))
```

### Mutual TLS (mTLS)

```python
validator = ContractValidator(ContractValidatorOptions(
    address="localhost:9550",
    tls=TLSOptions(
        enabled=True,
        root_cert_path="./certs/ca.crt",
        cert_path="./certs/client.crt",
        key_path="./certs/client.key",
    ),
))
```

## API Key Authentication

```python
from cvt_sdk import ContractValidator, ContractValidatorOptions

validator = ContractValidator(ContractValidatorOptions(
    address="localhost:9550",
    api_key="your-api-key",
))
```

## Error Handling

```python
try:
    validator.register_schema("petstore", "./openapi.json")
except FileNotFoundError:
    print("Schema file not found")
except ValueError as e:
    print(f"Schema registration failed: {e}")

# Validation errors are returned in the result, not thrown
result = validator.validate(request, response)
if not result["valid"]:
    print("Validation errors:", result["errors"])
```

## Type Definitions

The SDK uses TypedDict for request/response types:

```python
from cvt_sdk import (
    ContractValidator,
    ContractValidatorOptions,
    TLSOptions,
    ValidationRequest,
    ValidationResponse,
    ValidationResult,
    BreakingChange,
    CompareResult,
    GenerateOptions,
    GeneratedFixture,
    GeneratedRequest,
    GeneratedResponse,
    EndpointInfo,
    RegisterConsumerOptions,
    ConsumerInfo,
    CanIDeployResult,
)
```

## Related Documentation

- **[Consumer Testing Guide](../../guides/consumer-testing.mdx)** - Testing your API integrations
- **[Producer Testing Guide](../../guides/producer-testing.mdx)** - Validating your APIs
- **[API Reference](../api.mdx)** - Full gRPC API documentation
