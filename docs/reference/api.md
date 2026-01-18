---
title: API Reference
sidebar_label: API Reference
sidebar_position: 1
description: CVT gRPC API reference documentation
---

# API Reference

CVT exposes a gRPC API defined in the protocol buffer specification. This page provides a comprehensive reference for all available RPC methods and their message types.

## Proto Definition

The complete API is defined in [`api/protos/cvt.proto`](https://github.com/sahina/cvt/blob/main/api/protos/cvt.proto).

## Service Methods

### Phase 1: Schema & Validation

| Method | Description |
|--------|-------------|
| `RegisterSchema` | Register an OpenAPI v2/v3 schema for validation |
| `ValidateInteraction` | Validate an HTTP request/response pair against a registered schema |
| `GetSchema` | Get metadata and content for a registered schema |
| `ListSchemas` | List all registered schemas with optional filtering |
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

---

## Message Types

### Schema Registration

#### RegisterSchemaRequest

```protobuf
message RegisterSchemaRequest {
  string schema_id = 1;            // Unique identifier for the schema
  string schema_content = 2;       // The OpenAPI spec content (YAML or JSON)
  string schema_version = 3;       // Optional semantic version
  SchemaOwnership ownership = 4;   // Optional ownership information
  bool check_compatibility = 5;    // If true, check for breaking changes
}
```

#### RegisterSchemaResponse

```protobuf
message RegisterSchemaResponse {
  bool success = 1;
  string message = 2;
  SchemaMetadata metadata = 3;                   // Metadata for the registered schema
  repeated BreakingChange breaking_changes = 4;  // Breaking changes if any detected
}
```

#### SchemaMetadata

```protobuf
message SchemaMetadata {
  string schema_id = 1;       // Unique identifier
  string schema_version = 2;  // Semantic version (e.g., "1.2.3")
  string schema_hash = 3;     // SHA256 hash of schema content
  int64 registered_at = 4;    // Unix timestamp of initial registration
  int64 updated_at = 5;       // Unix timestamp of last update
  SchemaOwnership ownership = 6;
  string openapi_version = 7; // Detected OpenAPI version (e.g., "3.0.0")
  int32 endpoint_count = 8;   // Number of endpoints in schema
}
```

#### SchemaOwnership

```protobuf
message SchemaOwnership {
  string owner = 1;         // Owner name or identifier
  string team = 2;          // Team responsible for the schema
  string contact_email = 3; // Contact email for schema issues
  bool read_only = 4;       // If true, schema cannot be updated
}
```

---

### Schema Retrieval

#### GetSchemaRequest

```protobuf
message GetSchemaRequest {
  string schema_id = 1;
  string schema_version = 2;  // Optional: specific version (empty = latest)
}
```

#### GetSchemaResponse

```protobuf
message GetSchemaResponse {
  bool found = 1;
  SchemaMetadata metadata = 2;
  string schema_content = 3;  // The actual schema content
}
```

#### ListSchemasRequest

```protobuf
message ListSchemasRequest {
  int32 page_size = 1;   // Max results per page (default 100)
  string page_token = 2; // Token for pagination
  string owner = 3;      // Optional: filter by owner
  string team = 4;       // Optional: filter by team
}
```

#### ListSchemasResponse

```protobuf
message ListSchemasResponse {
  repeated SchemaMetadata schemas = 1;
  string next_page_token = 2;  // Token for next page (empty if no more)
  int32 total_count = 3;       // Total number of schemas matching filter
}
```

---

### Interaction Validation

#### InteractionRequest

```protobuf
message InteractionRequest {
  string schema_id = 1;
  RequestData request = 2;
  ResponseData response = 3;
  string schema_version = 4;  // Optional: validate against specific version
}
```

#### RequestData

```protobuf
message RequestData {
  string method = 1; // GET, POST, etc.
  string path = 2;   // /users/123
  map<string, string> headers = 3;
  string body = 4;   // JSON body as string
}
```

#### ResponseData

```protobuf
message ResponseData {
  int32 status_code = 1;
  map<string, string> headers = 2;
  string body = 3; // JSON body as string
}
```

#### ValidationResult

```protobuf
message ValidationResult {
  bool valid = 1;
  repeated string errors = 2;
  string validated_against_version = 3; // Version of schema used for validation
  string validated_against_hash = 4;    // Hash of schema used for validation
}
```

---

### Producer Validation

#### ValidateProducerRequest

```protobuf
message ValidateProducerRequest {
  string schema_id = 1;            // Schema to validate against
  string schema_version = 2;       // Optional: specific schema version
  string method = 3;               // HTTP method (GET, POST, etc.)
  string path = 4;                 // API path with actual values (e.g., /users/123)
  ResponseData response = 5;       // The response to validate
  RequestData request = 6;         // Optional: request context (for path param extraction)
}
```

---

### Schema Comparison

#### CompareSchemasRequest

```protobuf
message CompareSchemasRequest {
  string schema_id = 1;
  string old_version = 2;     // Version to compare from (empty = previous)
  string new_version = 3;     // Version to compare to (empty = latest)
}
```

#### CompareSchemasResponse

```protobuf
message CompareSchemasResponse {
  bool compatible = 1;                        // True if no breaking changes
  repeated BreakingChange breaking_changes = 2;
  SchemaMetadata old_schema = 3;
  SchemaMetadata new_schema = 4;
}
```

#### BreakingChange

```protobuf
message BreakingChange {
  BreakingChangeType type = 1;
  string path = 2;           // API path affected (e.g., "/users/{id}")
  string method = 3;         // HTTP method affected (e.g., "POST")
  string description = 4;    // Human-readable description
  string old_value = 5;      // Previous value (for context)
  string new_value = 6;      // New value (for context)
}

enum BreakingChangeType {
  BREAKING_CHANGE_UNSPECIFIED = 0;
  ENDPOINT_REMOVED = 1;           // Endpoint path+method was removed
  REQUIRED_FIELD_ADDED = 2;       // Required field added to request body
  TYPE_CHANGED = 3;               // Field type changed incompatibly
  REQUIRED_PARAMETER_ADDED = 4;   // Required query/path/header param added
  RESPONSE_SCHEMA_CHANGED = 5;    // Response schema changed incompatibly
  ENUM_VALUE_REMOVED = 6;         // Enum value was removed
}
```

---

### Fixture Generation

#### GenerateFixtureRequest

```protobuf
message GenerateFixtureRequest {
  string schema_id = 1;         // Schema to generate fixtures from
  string method = 2;            // HTTP method (GET, POST, etc.)
  string path = 3;              // API path (e.g., /users/{id})
  int32 status_code = 4;        // Response status code (0 = auto-select)
  bool use_examples = 5;        // Use schema examples when available
  string content_type = 6;      // Content type (default: application/json)
  OutputType output_type = 7;   // What to generate
}

enum OutputType {
  OUTPUT_FIXTURE = 0;   // Complete request/response pair (default)
  OUTPUT_REQUEST = 1;   // Request body only
  OUTPUT_RESPONSE = 2;  // Response only
}
```

#### GenerateFixtureResponse

```protobuf
message GenerateFixtureResponse {
  bool success = 1;
  string message = 2;
  GeneratedFixture fixture = 3;     // Full fixture (if output_type = FIXTURE)
  string request_body = 4;          // Request body JSON (if output_type = REQUEST)
  GeneratedResponse response = 5;   // Response (if output_type = RESPONSE)
}
```

#### GeneratedFixture

```protobuf
message GeneratedFixture {
  GeneratedRequest request = 1;
  GeneratedResponse response = 2;
}

message GeneratedRequest {
  string method = 1;
  string path = 2;
  map<string, string> headers = 3;
  string body = 4;  // JSON body as string
}

message GeneratedResponse {
  int32 status_code = 1;
  map<string, string> headers = 2;
  string body = 3;  // JSON body as string
}
```

---

### Endpoint Listing

#### ListEndpointsRequest

```protobuf
message ListEndpointsRequest {
  string schema_id = 1;
}
```

#### ListEndpointsResponse

```protobuf
message ListEndpointsResponse {
  repeated EndpointInfo endpoints = 1;
}

message EndpointInfo {
  string method = 1;       // HTTP method
  string path = 2;         // API path
  string operation_id = 3; // OpenAPI operationId (if defined)
  string summary = 4;      // OpenAPI summary (if defined)
}
```

---

### Consumer Registry

#### RegisterConsumerRequest

```protobuf
message RegisterConsumerRequest {
  string consumer_id = 1;
  string consumer_version = 2;
  string schema_id = 3;
  string schema_version = 4;
  string environment = 5;
  repeated EndpointUsage used_endpoints = 6;
}
```

#### RegisterConsumerResponse

```protobuf
message RegisterConsumerResponse {
  bool success = 1;
  string message = 2;
  ConsumerInfo consumer = 3;
}
```

#### ConsumerInfo

```protobuf
message ConsumerInfo {
  string consumer_id = 1;              // Unique consumer identifier (e.g., "order-service")
  string consumer_version = 2;         // Consumer's version (e.g., "2.1.0")
  string schema_id = 3;                // Schema this consumer depends on
  string schema_version = 4;           // Schema version consumer was tested against
  string environment = 5;              // Environment (dev, staging, prod)
  int64 registered_at = 6;             // Unix timestamp of registration
  int64 last_validated_at = 7;         // Unix timestamp of last successful validation
  repeated EndpointUsage used_endpoints = 8;  // Which endpoints the consumer uses
}
```

#### EndpointUsage

```protobuf
message EndpointUsage {
  string method = 1;                   // HTTP method
  string path = 2;                     // API path
  repeated string used_fields = 3;    // Fields used in response (e.g., ["email", "name"])
}
```

#### ListConsumersRequest

```protobuf
message ListConsumersRequest {
  string schema_id = 1;      // Required: schema to query
  string environment = 2;    // Optional: filter by environment
}
```

#### ListConsumersResponse

```protobuf
message ListConsumersResponse {
  repeated ConsumerInfo consumers = 1;
}
```

#### DeregisterConsumerRequest

```protobuf
message DeregisterConsumerRequest {
  string consumer_id = 1;
  string schema_id = 2;     // Required: which schema dependency to remove
  string environment = 3;
}
```

#### DeregisterConsumerResponse

```protobuf
message DeregisterConsumerResponse {
  bool success = 1;
  string message = 2;
}
```

---

### Deployment Safety

#### CanIDeployRequest

```protobuf
message CanIDeployRequest {
  string schema_id = 1;        // Schema to deploy
  string new_version = 2;      // New version to deploy
  string environment = 3;      // Target environment (dev, staging, prod)
}
```

#### CanIDeployResponse

```protobuf
message CanIDeployResponse {
  bool safe_to_deploy = 1;                       // True if safe to deploy
  string summary = 2;                            // Human-readable summary
  repeated BreakingChange breaking_changes = 3;  // All breaking changes in new version
  repeated ConsumerImpact affected_consumers = 4; // Impact on each consumer
}
```

#### ConsumerImpact

```protobuf
message ConsumerImpact {
  string consumer_id = 1;
  string consumer_version = 2;
  string current_schema_version = 3;   // Version consumer was tested against
  string environment = 4;
  bool will_break = 5;                 // True if consumer will be affected
  repeated BreakingChange relevant_changes = 6;  // Breaking changes affecting this consumer
}
```

---

## Error Codes

| Code | Description |
|------|-------------|
| `SCHEMA_NOT_FOUND` | The requested schema ID is not registered |
| `INVALID_SCHEMA` | The provided schema is not valid OpenAPI |
| `VALIDATION_FAILED` | The interaction does not match the schema |
| `PATH_NOT_FOUND` | The request path does not match any schema endpoint |
| `METHOD_NOT_ALLOWED` | The HTTP method is not allowed for this endpoint |
| `VERSION_NOT_FOUND` | The requested schema version does not exist |
| `CONSUMER_NOT_FOUND` | The requested consumer is not registered |

---

## Default Values

Some fields have default values when not explicitly specified:

| Field | Default |
|-------|---------|
| `RegisterConsumerRequest.environment` | `"dev"` |
| `DeregisterConsumerRequest.environment` | `"dev"` |
| `CanIDeployRequest.environment` | `"prod"` |
| `ListSchemasRequest.page_size` | `100` |
| `GenerateFixtureRequest.content_type` | `"application/json"` |

---

## Related Documentation

- **[CLI Reference](./cli.md)** - Command-line interface for CVT
- **[Configuration Reference](./configuration.md)** - Environment variables and settings
- **[SDK Documentation](./sdk/)** - Language-specific SDK guides
