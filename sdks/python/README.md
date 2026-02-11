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

### From GitHub Releases

Install directly from a GitHub Release asset:

```bash
pip install "cvt-sdk @ https://github.com/sahina/cvt/releases/download/v0.1.0/cvt_sdk-0.1.0-py3-none-any.whl"
```

Replace `v0.1.0` in the URL and `0.1.0` in the filename with the desired version. Available versions can be found on the [Releases](https://github.com/sahina/cvt/releases) page.

### From local source (development)

```bash
# From the project root
pip install -e sdks/python
```

## Usage

### Initialize and Register Schema

```python
from cvt_sdk import ContractValidator

validator = ContractValidator(host="localhost:9550")

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

if result.valid:
    print("✅ Valid interaction")
else:
    print(f"❌ Validation errors: {result.errors}")
```

## HTTP Adapter (Requests)

The SDK includes a Requests adapter for automatic HTTP traffic validation:

```python
from cvt_sdk import ContractValidator
from cvt_sdk.adapters import ContractValidatingSession

validator = ContractValidator(host="localhost:9550")
validator.register_schema("petstore", "./openapi.json")

# Create a validating session (drop-in replacement for requests.Session)
session = ContractValidatingSession(
    validator=validator,
    schema_id="petstore",
    auto_validate=True,
    exclude_paths=["/health", "/metrics"],
    on_validation_failure=lambda result, interaction: print(f"Failed: {result.errors}")
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

validator = ContractValidator(host="localhost:9550")
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

app.wsgi_app = WSGIMiddleware(app.wsgi_app, config=config)
```

### Configuration Options

| Option                | Type             | Description                                |
| --------------------- | ---------------- | ------------------------------------------ |
| `schema_id`           | `str`            | Schema ID to validate against              |
| `validator`           | `Validator`      | ContractValidator instance                 |
| `mode`                | `ValidationMode` | `STRICT`, `WARN`, or `SHADOW`              |
| `exclude_paths`       | `list[str]`      | Paths to skip validation (e.g., `/health`) |
| `include_paths`       | `list[str]`      | Only validate matching paths               |
| `validate_response`   | `bool`           | Enable response validation (default: True) |
| `on_validation_error` | `Callable`       | Custom error handler callback              |

## Breaking Change Detection

Detect breaking changes between OpenAPI schema versions before deployment:

```python
from cvt_sdk import ContractValidator

validator = ContractValidator(host="localhost:9550")

# Register both schema versions
validator.register_schema_with_version("my-api", "./openapi-v1.json", "1.0.0")
validator.register_schema_with_version("my-api", "./openapi-v2.json", "2.0.0")

# Compare versions
result = validator.compare_schemas("my-api", "1.0.0", "2.0.0")

if not result.compatible:
    print("Breaking changes detected:")
    for change in result.breaking_changes:
        print(f"- [{change.type}] {change.description}")
        if change.path:
            print(f"  Path: {change.method} {change.path}")
    sys.exit(1)  # Fail CI build
```

### Breaking Change Types

| Type                   | Description                           |
| ---------------------- | ------------------------------------- |
| `ENDPOINT_REMOVED`     | An endpoint was removed               |
| `REQUIRED_FIELD_ADDED` | A required field was added to request |
| `FIELD_TYPE_CHANGED`   | A field's type was changed            |
| `ENUM_VALUE_REMOVED`   | An allowed enum value was removed     |

See `examples/breaking_changes.py` for a complete example.

## Producer Testing

Test that your API handlers return responses matching your OpenAPI specification.

### ProducerTestKit

```python
from cvt_sdk.producer import ProducerTestKit, TestConfig, TestResponseData

test_kit = ProducerTestKit(TestConfig(
    schema_id="user-api",
    server_address="localhost:9550",
))

# Validate handler response
result = test_kit.validate_response(
    method="GET",
    path="/users/123",
    response=TestResponseData(
        status_code=200,
        body={"id": "123", "name": "Alice", "email": "alice@example.com"},
    ),
)

assert result.valid

# Don't forget to close
test_kit.close()
```

### Consumer Registry

Track which services depend on your API:

```python
# Register a consumer after successful contract tests
consumer = validator.register_consumer(
    consumer_id="order-service",
    consumer_version="2.1.0",
    schema_id="user-api",
    schema_version="1.0.0",
    environment="prod",
    used_endpoints=[
        {"method": "GET", "path": "/users/{id}", "used_fields": ["id", "email"]},
    ],
)

# List all consumers of a schema
consumers = validator.list_consumers(schema_id="user-api", environment="prod")

# Deregister a consumer
validator.deregister_consumer("order-service", "user-api", "prod")
```

### Deployment Safety (can-i-deploy)

Check if a new schema version can be safely deployed:

```python
result = validator.can_i_deploy(
    schema_id="user-api",
    new_version="2.0.0",
    environment="prod",
)

if not result.safe_to_deploy:
    print(f"Cannot deploy: {result.summary}")
    for consumer in result.affected_consumers:
        if consumer.will_break:
            print(f"- {consumer.consumer_id} will break")
    sys.exit(1)
```

See [Producer Testing Guide](../../docs/guides/producer-testing.mdx) for complete documentation.

## Security Configuration

### TLS

```python
validator = ContractValidator(
    host="localhost:9550",
    tls_enabled=True,
    tls_root_cert="./certs/ca.crt",
    tls_client_cert="./certs/client.crt",  # For mTLS
    tls_client_key="./certs/client.key"    # For mTLS
)
```

### API Key Authentication

```python
validator = ContractValidator(
    host="localhost:9550",
    api_key="your-api-key-here"
)
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
    v = ContractValidator(host="localhost:9550")
    yield v
    v.close()

def test_validate_correct_interaction(validator):
    """Test validation of a correct interaction."""
    validator.register_schema("test", "tests/fixtures/openapi.json")

    result = validator.validate(
        request={"method": "GET", "path": "/users"},
        response={"status_code": 200, "body": []}
    )

    assert result.valid is True

def test_validate_incorrect_interaction(validator):
    """Test validation of an incorrect interaction."""
    validator.register_schema("test", "tests/fixtures/openapi.json")

    result = validator.validate(
        request={"method": "GET", "path": "/users"},
        response={"status_code": 500}  # Should be 200
    )

    assert result.valid is False
    assert len(result.errors) > 0
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
