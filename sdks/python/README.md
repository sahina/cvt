# Contract Validator Toolkit (CVT) - Python SDK

The **CVT Python SDK** allows you to validate HTTP interactions (requests and responses) against OpenAPI schemas using the CVT gRPC service.

> **Status**: Fully Implemented

## Installation

### From PyPI (recommended)

```bash
pip install cvt-sdk
```

Or with uv:

```bash
uv add cvt-sdk
```

With the optional requests adapter:

```bash
pip install "cvt-sdk[requests]"
```

### From local source (development)

```bash
# From the project root
pip install -e sdks/python
```

## Usage

### Initialize and Register Schema

```python
from cvt_sdk import ContractValidator

validator = ContractValidator("localhost:9550")

# Register from local file
validator.register_schema("my-schema", "path/to/openapi.json")

# Register from URL
validator.register_schema("petstore", "https://petstore.swagger.io/v2/swagger.json")
```

### Validate Interactions

```python
from cvt_sdk import ContractValidator

validator = ContractValidator()

request = {
    "method": "POST",
    "path": "/users",
    "body": {"username": "alice", "email": "alice@example.com"}
}

response = {
    "status_code": 201
}

result = validator.validate(request, response)

if result["valid"]:
    print("Valid interaction")
else:
    print(f"Validation errors: {result['errors']}")
```

## HTTP Adapter (Requests)

The SDK includes a Requests adapter for automatic HTTP traffic validation:

```python
from cvt_sdk import ContractValidator
from cvt_sdk.adapters import ContractValidatingSession

validator = ContractValidator("localhost:9550")
validator.register_schema("petstore", "./openapi.json")

# Create a validating session (drop-in replacement for requests.Session)
session = ContractValidatingSession(
    validator,
    auto_validate=True,
    exclude_paths=["/health", "/metrics"],
    on_validation_failure=lambda result, req, resp: print(f"Failed: {result['errors']}")
)

# All requests are now automatically validated
response = session.post("https://api.example.com/pets", json={"name": "Fluffy"})
```

### Adapter Options

- `auto_validate`: Enable/disable automatic validation (default: True)
- `include_paths`: List of paths/regex to include
- `exclude_paths`: List of paths/regex to exclude
- `on_validation_failure`: Custom error handler
- `get_interactions()`: Retrieve captured interactions
- `clear_interactions()`: Reset captured data

## Producer Validation (Server-Side Middleware)

Validate incoming requests and outgoing responses against your OpenAPI contract on the server side.

> **Full documentation:** See [Validation Modes](../../docs/guides/validation-modes.mdx) for detailed behavior, rollout strategy, and metrics information.

### Validation Modes

| Mode                    | Request Violation | Response Violation | Use Case               |
| ----------------------- | ----------------- | ------------------ | ---------------------- |
| `ValidationMode.STRICT` | Reject with 400   | Log error          | Production enforcement |
| `ValidationMode.WARN`   | Log, continue     | Log, continue      | Gradual rollout        |
| `ValidationMode.SHADOW` | Metrics only      | Metrics only       | Initial deployment     |

**Recommended rollout:** `SHADOW` → `WARN` → `STRICT`. See [Recommended Rollout Strategy](../../docs/guides/validation-modes.mdx#recommended-rollout-strategy).

### FastAPI / ASGI Middleware

```python
from cvt_sdk import ContractValidator
from cvt_sdk.producer import ProducerConfig, ValidationMode
from cvt_sdk.producer.adapters import ASGIMiddleware

validator = ContractValidator("localhost:9550")
validator.register_schema("my-api", "./openapi.json")

config = ProducerConfig(
    schema_id="my-api",
    validator=validator,
    mode=ValidationMode.STRICT,
    exclude_paths=["/health", "/metrics"],
)

app.add_middleware(ASGIMiddleware, config=config)
```

### Flask / WSGI Middleware

```python
from cvt_sdk.producer.adapters import WSGIMiddleware

app.wsgi_app = WSGIMiddleware(app.wsgi_app, config)
```

### Configuration Options

| Option                | Type               | Description                                       |
| --------------------- | ------------------ | ------------------------------------------------- |
| `schema_id`           | `str`              | Schema ID to validate against                     |
| `validator`           | `Validator`        | ContractValidator instance                        |
| `mode`                | `ValidationMode`   | `STRICT`, `WARN`, or `SHADOW`                     |
| `validate_request`    | `bool`             | Enable request validation (default: True)         |
| `validate_response`   | `bool`             | Enable response validation (default: True)        |
| `exclude_paths`       | `list[PathFilter]` | Paths to skip validation (str or regex Pattern)   |
| `include_paths`       | `list[PathFilter]` | Only validate matching paths (str or regex Pattern)|
| `on_request_failure`  | `Callable`         | Called when request validation fails               |
| `on_response_failure` | `Callable`         | Called when response validation fails              |

## Breaking Change Detection

Detect breaking changes between OpenAPI schema versions before deployment:

```python
from cvt_sdk import ContractValidator

validator = ContractValidator("localhost:9550")

# Register both schema versions
validator.register_schema_with_version("my-api", "./openapi-v1.json", "1.0.0")
validator.register_schema_with_version("my-api", "./openapi-v2.json", "2.0.0")

# Compare versions
result = validator.compare_schemas("my-api", "1.0.0", "2.0.0")

if not result["compatible"]:
    print("Breaking changes detected:")
    for change in result["breaking_changes"]:
        print(f"- [{change['type']}] {change['description']}")
        if change["path"]:
            print(f"  Path: {change['method']} {change['path']}")
    sys.exit(1)  # Fail CI build
```

### Breaking Change Types

| Type                        | Description                                    |
| --------------------------- | ---------------------------------------------- |
| `ENDPOINT_REMOVED`          | An endpoint was removed                        |
| `REQUIRED_FIELD_ADDED`      | A required field was added to request          |
| `TYPE_CHANGED`              | A field's type was changed incompatibly        |
| `REQUIRED_PARAMETER_ADDED`  | A required query/path/header param was added   |
| `RESPONSE_SCHEMA_CHANGED`   | Response schema was changed incompatibly       |
| `ENUM_VALUE_REMOVED`        | An allowed enum value was removed              |

See `examples/breaking_changes.py` for a complete example.

## Producer Testing

Test that your API handlers return responses matching your OpenAPI specification.

### ProducerTestKit

```python
from cvt_sdk.producer import ProducerTestKit, ProducerTestConfig

test_kit = ProducerTestKit(ProducerTestConfig(
    schema_id="user-api",
    server_address="localhost:9550",
))

# Validate handler response
result = test_kit.validate_response(
    method="GET",
    path="/users/123",
    status_code=200,
    body={"id": "123", "name": "Alice", "email": "alice@example.com"},
)

assert result.valid

# Don't forget to close
test_kit.close()
```

### Consumer Registry

Track which services depend on your API:

```python
from cvt_sdk import RegisterConsumerOptions, EndpointUsage

# Register a consumer after successful contract tests
consumer = validator.register_consumer(RegisterConsumerOptions(
    consumer_id="order-service",
    consumer_version="2.1.0",
    schema_id="user-api",
    schema_version="1.0.0",
    environment="prod",
    used_endpoints=[
        EndpointUsage(method="GET", path="/users/{id}", used_fields=["id", "email"]),
    ],
))

# List all consumers of a schema
consumers = validator.list_consumers("user-api", "prod")

# Deregister a consumer
validator.deregister_consumer("order-service", "user-api", "prod")
```

### Deployment Safety (can-i-deploy)

Check if a new schema version can be safely deployed:

```python
result = validator.can_i_deploy("user-api", "2.0.0", "prod")

if not result["safe_to_deploy"]:
    print(f"Cannot deploy: {result['summary']}")
    for consumer in result["affected_consumers"]:
        if consumer["will_break"]:
            print(f"- {consumer['consumer_id']} will break")
    sys.exit(1)
```

See [Producer Testing Guide](../../docs/guides/producer-testing.mdx) for complete documentation.

## Security Configuration

### TLS

```python
from cvt_sdk import ContractValidator, ContractValidatorOptions, TLSOptions

validator = ContractValidator(ContractValidatorOptions(
    address="localhost:9550",
    tls=TLSOptions(
        enabled=True,
        root_cert_path="./certs/ca.crt",
        cert_path="./certs/client.crt",  # For mTLS
        key_path="./certs/client.key",   # For mTLS
    ),
))
```

### API Key Authentication

```python
from cvt_sdk import ContractValidator, ContractValidatorOptions

validator = ContractValidator(ContractValidatorOptions(
    address="localhost:9550",
    api_key="your-api-key-here",
))
```

## Prerequisites

Ensure the CVT gRPC server is running (default: `localhost:9550`).

## Testing

The Python SDK includes tests covering:

- Client initialization and configuration
- Schema registration
- Validation requests and responses
- Error handling

### Running Tests

```bash
# Install dependencies
uv sync

# Run all tests
uv run pytest

# Run tests with coverage
uv run pytest --cov=cvt_sdk --cov-report=html

# View coverage report
open htmlcov/index.html

# Run specific test file
uv run pytest tests/test_validator.py

# Run with verbose output
uv run pytest -v -s
```

### Test Structure

```shell
tests/
├── test_validator.py      # Main SDK test suite
├── test_registration.py   # Schema registration tests
└── conftest.py           # Test fixtures
```

### Writing Tests

Example test using pytest:

```python
import pytest
from cvt_sdk import ContractValidator

@pytest.fixture
def validator():
    """Create validator instance for testing."""
    v = ContractValidator("localhost:9550")
    yield v
    v.close()

def test_validate_correct_interaction(validator):
    """Test validation of a correct interaction."""
    validator.register_schema("test", "tests/fixtures/openapi.json")

    result = validator.validate(
        request={"method": "GET", "path": "/users"},
        response={"status_code": 200, "body": []}
    )

    assert result["valid"] is True

def test_validate_incorrect_interaction(validator):
    """Test validation of an incorrect interaction."""
    validator.register_schema("test", "tests/fixtures/openapi.json")

    result = validator.validate(
        request={"method": "GET", "path": "/users"},
        response={"status_code": 500}  # Should be 200
    )

    assert result["valid"] is False
    assert len(result["errors"]) > 0
```

### Coverage

The SDK targets 60%+ test coverage.

## Development

```bash
# Install development dependencies
uv sync --all-extras

# Run linter
uv run ruff check cvt_sdk

# Format code
uv run ruff format cvt_sdk

# Type checking
uv run mypy cvt_sdk

# Build package
uv build
```

## Contributing

Contributions are welcome!

## License

MIT License
