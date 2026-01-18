---
title: API Reference
sidebar_label: API Reference
sidebar_position: 100
description: CVT gRPC API reference documentation
---

# API Reference

CVT exposes a gRPC API defined in the protocol buffer specification. This page provides an overview of available RPC methods and their usage.

## Proto Definition

The complete API is defined in [`api/protos/cvt.proto`](https://github.com/sahina/cvt/blob/main/api/protos/cvt.proto).

## Service Methods

### Phase 1: Schema & Validation

| Method | Description |
|--------|-------------|
| `RegisterSchema` | Register an OpenAPI v2/v3 schema for validation |
| `ValidateInteraction` | Validate an HTTP request/response pair against a registered schema |
| `ValidateProducerResponse` | Producer-side response validation |
| `CompareSchemas` | Compare two schema versions for breaking changes |
| `GenerateFixture` | Generate test fixtures from schemas |
| `ListEndpoints` | List all endpoints in a registered schema |

### Phase 2: Consumer Registry

| Method | Description |
|--------|-------------|
| `RegisterConsumer` | Register a consumer with expected interactions |
| `ListConsumers` | List all registered consumers for a schema |
| `DeregisterConsumer` | Remove a consumer registration |

### Phase 3: Deployment Safety

| Method | Description |
|--------|-------------|
| `CanIDeploy` | Check if schema changes will break registered consumers |

## Request/Response Types

### RegisterSchema

Register an OpenAPI schema for validation.

```protobuf
message RegisterSchemaRequest {
  string schema_id = 1;      // Unique identifier for the schema
  string content = 2;        // OpenAPI schema content (JSON or YAML)
  string format = 3;         // Format: "json" or "yaml"
  string version = 4;        // Optional version string
}

message RegisterSchemaResponse {
  bool success = 1;
  string message = 2;
  string schema_id = 3;
}
```

### ValidateInteraction

Validate an HTTP request/response pair against a registered schema.

```protobuf
message InteractionRequest {
  string schema_id = 1;
  HttpRequest request = 2;
  HttpResponse response = 3;
}

message HttpRequest {
  string method = 1;         // HTTP method (GET, POST, etc.)
  string path = 2;           // Request path
  map<string, string> headers = 3;
  map<string, string> query = 4;
  string body = 5;
}

message HttpResponse {
  int32 status_code = 1;
  map<string, string> headers = 2;
  string body = 3;
}

message ValidationResult {
  bool valid = 1;
  repeated ValidationError errors = 2;
}

message ValidationError {
  string path = 1;           // JSONPath to the error location
  string message = 2;        // Error description
  string code = 3;           // Error code
}
```

### CompareSchemas

Compare two schema versions for breaking changes.

```protobuf
message CompareRequest {
  string old_schema = 1;     // Previous schema content
  string new_schema = 2;     // New schema content
}

message CompareResponse {
  bool has_breaking_changes = 1;
  repeated SchemaChange changes = 2;
}

message SchemaChange {
  string type = 1;           // "breaking" or "non-breaking"
  string path = 2;           // Path to the changed element
  string description = 3;    // Human-readable description
}
```

## SDK Examples

### Node.js

```typescript
import { CVTClient } from '@cvt/cvt-sdk';

const client = new CVTClient('localhost:9550');

// Register a schema
await client.registerSchema({
  schemaId: 'my-api',
  content: fs.readFileSync('openapi.json', 'utf-8'),
  format: 'json',
});

// Validate an interaction
const result = await client.validateInteraction({
  schemaId: 'my-api',
  request: { method: 'GET', path: '/users/1' },
  response: { statusCode: 200, body: '{"id": 1}' },
});
```

### Python

```python
from cvt_sdk import CVTClient

client = CVTClient('localhost:9550')

# Register a schema
client.register_schema(
    schema_id='my-api',
    content=open('openapi.json').read(),
    format='json'
)

# Validate an interaction
result = client.validate_interaction(
    schema_id='my-api',
    request={'method': 'GET', 'path': '/users/1'},
    response={'status_code': 200, 'body': '{"id": 1}'}
)
```

### Go

```go
import "github.com/sahina/cvt/sdks/go/cvt"

client, _ := cvt.NewClient("localhost:9550")

// Register a schema
client.RegisterSchema(ctx, &cvt.RegisterSchemaRequest{
    SchemaId: "my-api",
    Content:  openAPIContent,
    Format:   "json",
})

// Validate an interaction
result, _ := client.ValidateInteraction(ctx, &cvt.InteractionRequest{
    SchemaId: "my-api",
    Request:  &cvt.HttpRequest{Method: "GET", Path: "/users/1"},
    Response: &cvt.HttpResponse{StatusCode: 200, Body: `{"id": 1}`},
})
```

## CLI Commands

CVT also provides a CLI for local validation without a running server:

```bash
# Validate an interaction
cvt validate --schema openapi.json --request req.json --response resp.json

# Compare schemas for breaking changes
cvt compare --old v1/openapi.json --new v2/openapi.json

# Generate test fixtures
cvt generate --schema openapi.json --endpoint "GET /users/{id}"

# Check deployment safety
cvt can-i-deploy --schema openapi.json --server localhost:9550
```

See the [Development Guide](./DEVELOPMENT) for more CLI details and local development setup.

## Error Codes

| Code | Description |
|------|-------------|
| `SCHEMA_NOT_FOUND` | The requested schema ID is not registered |
| `INVALID_SCHEMA` | The provided schema is not valid OpenAPI |
| `VALIDATION_FAILED` | The interaction does not match the schema |
| `PATH_NOT_FOUND` | The request path does not match any schema endpoint |
| `METHOD_NOT_ALLOWED` | The HTTP method is not allowed for this endpoint |
