---
title: Python SDK
sidebar_label: Python
sidebar_position: 3
description: CVT SDK for Python
---

# Python SDK

The Python SDK provides contract validation for Python applications with async support.

## Installation

```bash
# pip
pip install cvt-sdk

# uv
uv add cvt-sdk

# poetry
poetry add cvt-sdk
```

## Quick Start

```python
from cvt_sdk import ContractValidator

validator = ContractValidator('localhost:9550')

# Register a schema
with open('openapi.json') as f:
    validator.register_schema('user-api', f.read())

# Validate an interaction
result = validator.validate(
    request={'method': 'GET', 'path': '/users/123'},
    response={'status_code': 200, 'body': '{"id": "123", "name": "John"}'}
)

print(result.valid)  # True or False
```

## API Reference

### ContractValidator

#### Constructor

```python
ContractValidator(
    address: str,
    tls_root_certs: bytes = None,
    tls_private_key: bytes = None,
    tls_cert_chain: bytes = None,
    api_key: str = None
)
```

| Parameter         | Type    | Description                             |
| ----------------- | ------- | --------------------------------------- |
| `address`         | `str`   | Server address (e.g., `localhost:9550`) |
| `tls_root_certs`  | `bytes` | CA certificate for TLS                  |
| `tls_private_key` | `bytes` | Client private key for mTLS             |
| `tls_cert_chain`  | `bytes` | Client certificate for mTLS             |
| `api_key`         | `str`   | API key for authentication              |

#### Methods

##### register_schema

```python
def register_schema(
    schema_id: str,
    content: str,
    version: str = None
) -> RegisterSchemaResponse
```

##### validate

```python
def validate(
    request: dict,
    response: dict
) -> ValidationResult
```

##### register_consumer

```python
def register_consumer(
    consumer_id: str,
    consumer_version: str,
    schema_id: str,
    schema_version: str,
    environment: str = 'dev',
    used_endpoints: List[EndpointUsage] = None
) -> RegisterConsumerResponse
```

##### list_consumers

```python
def list_consumers(
    schema_id: str,
    environment: str = None
) -> List[ConsumerInfo]
```

##### deregister_consumer

```python
def deregister_consumer(
    consumer_id: str,
    schema_id: str,
    environment: str = None
) -> None
```

##### compare_schemas

```python
def compare_schemas(
    schema_id: str,
    old_version: str,
    new_version: str
) -> CompareSchemasResponse
```

##### can_i_deploy

```python
def can_i_deploy(
    schema_id: str,
    new_version: str,
    environment: str = 'prod'
) -> CanIDeployResponse
```

##### generate_fixture

```python
def generate_fixture(
    schema_id: str,
    method: str,
    path: str,
    status_code: int = None
) -> GeneratedFixture
```

##### close

```python
def close() -> None
```

## HTTP Adapters

### Requests Adapter

```python
import requests
from cvt_sdk import ContractValidator
from cvt_sdk.adapters import create_requests_adapter

validator = ContractValidator('localhost:9550')
validator.register_schema('user-api', schema)

session = requests.Session()
create_requests_adapter(
    session=session,
    validator=validator,
    schema_id='user-api',
    auto_validate=True,
    on_validation_failure=lambda r: raise Exception(f"Contract violation: {r.errors}")
)

# All requests are now validated
response = session.get('http://user-service/users/123')
```

### HTTPX Adapter (Async)

```python
import httpx
from cvt_sdk.adapters import create_httpx_adapter

async with httpx.AsyncClient() as client:
    create_httpx_adapter(
        client=client,
        validator=validator,
        schema_id='user-api'
    )

    response = await client.get('http://user-service/users/123')
```

## Producer Middleware

### FastAPI

```python
from fastapi import FastAPI
from cvt_sdk.producer import ProducerConfig, ValidationMode
from cvt_sdk.producer.adapters import ASGIMiddleware

app = FastAPI()

config = ProducerConfig(
    schema_id='my-api',
    validator=validator,
    mode=ValidationMode.STRICT  # STRICT | WARN | SHADOW
)

app.add_middleware(ASGIMiddleware, config=config)

@app.get('/users/{user_id}')
async def get_user(user_id: str):
    return {'id': user_id, 'name': 'John'}
```

### Flask

```python
from flask import Flask
from cvt_sdk.producer.adapters import WSGIMiddleware

app = Flask(__name__)
app.wsgi_app = WSGIMiddleware(app.wsgi_app, config=config)

@app.route('/users/<user_id>')
def get_user(user_id):
    return {'id': user_id, 'name': 'John'}
```

## Producer Test Kit

```python
import pytest
from cvt_sdk.producer import ProducerTestKit, TestConfig

@pytest.fixture
def test_kit():
    kit = ProducerTestKit(TestConfig(
        schema_id='user-api',
        server_address='localhost:9550'
    ))
    yield kit
    kit.close()

def test_returns_valid_response(test_kit):
    result = test_kit.validate_response(
        method='GET',
        path='/users/123',
        status_code=200,
        body={'id': '123', 'name': 'John'}
    )

    assert result.valid
    assert len(result.errors) == 0
```

## TLS Configuration

```python
with open('./certs/ca.crt', 'rb') as f:
    root_certs = f.read()

validator = ContractValidator(
    'localhost:9550',
    tls_root_certs=root_certs
)

# For mTLS:
with open('./certs/client.key', 'rb') as f:
    private_key = f.read()
with open('./certs/client.crt', 'rb') as f:
    cert_chain = f.read()

validator = ContractValidator(
    'localhost:9550',
    tls_root_certs=root_certs,
    tls_private_key=private_key,
    tls_cert_chain=cert_chain
)
```

## API Key Authentication

```python
validator = ContractValidator(
    'localhost:9550',
    api_key='your-api-key'
)
```

## Async Support

The SDK supports both sync and async usage:

```python
# Sync
result = validator.validate(request, response)

# Async
result = await validator.validate_async(request, response)
```

## Context Manager

```python
with ContractValidator('localhost:9550') as validator:
    validator.register_schema('my-api', schema)
    result = validator.validate(request, response)
# Connection automatically closed
```

## Error Handling

```python
from cvt_sdk.exceptions import SchemaNotFoundError, InvalidSchemaError

try:
    validator.register_schema('my-api', schema)
except InvalidSchemaError as e:
    print(f'Schema is not valid OpenAPI: {e}')

try:
    result = validator.validate(request, response)
except SchemaNotFoundError as e:
    print(f'Schema not registered: {e}')
```

## Related Documentation

- **[Consumer Testing Guide](../../guides/consumer-testing.mdx)** - Testing your API integrations
- **[Producer Testing Guide](../../guides/producer-testing.mdx)** - Validating your APIs
- **[API Reference](../api.md)** - Full gRPC API documentation
