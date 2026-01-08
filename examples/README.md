# CVT Examples

This directory contains example applications that demonstrate CVT (Contract Validator Toolkit) capabilities.

## Prerequisites

Before running any examples, ensure the CVT server is running:

```bash
# From the project root
make up
```

This starts the CVT gRPC server on `localhost:50052`.

## Available Examples

### Go

| Example          | Directory      | Description                                                                                               |
| ---------------- | -------------- | --------------------------------------------------------------------------------------------------------- |
| Consumer Testing | `go/consumer/` | Full consumer testing workflow: schema registration, validation, consumer registry, and deployment safety |

## Running Examples

### Go Consumer Example

```bash
cd examples/go/consumer
go run main.go
```

This example demonstrates:

1. **Schema Registration** - Register a producer's OpenAPI schema (v1.0.0)
2. **Interaction Validation** - Validate HTTP request/response pairs against the schema
3. **Consumer Registration** - Register as a consumer with endpoint tracking
4. **List Consumers** - Query registered consumers for a schema
5. **Deployment Safety** - Check if it's safe to deploy a new schema version (v2.0.0)
6. **Consumer Deregistration** - Clean up registered consumer

### Expected Output

```text
=== CVT Consumer Testing Example ===

Step 1: Registering producer's OpenAPI schema (v1.0.0)...
        Schema v1.0.0 registered successfully.

Step 2: Validating API interactions against the schema...
        Valid interaction: GET /pet?status=available

Step 3: Registering as a consumer (order-service)...
        Consumer registered: order-service v2.1.0
        Uses schema: petstore-api v1.0.0

Step 4: Listing registered consumers for this schema...
        Found 1 consumer(s) in dev environment

Step 5: Registering schema v2.0.0 (with breaking changes)...
        Schema v2.0.0 registered successfully.

Step 6: Checking deployment safety (can-i-deploy v2.0.0 to dev)...
------------------------------------------------------------
RESULT: UNSAFE TO DEPLOY
Breaking changes detected that affect registered consumers.
------------------------------------------------------------

Step 7: Cleaning up (deregistering consumer)...
        Consumer deregistered successfully.
```

## Test Schemas

The examples use test schemas located in `examples/schemas/`:

| File               | Description                           |
| ------------------ | ------------------------------------- |
| `user-api-v1.json` | User API v1.0.0 (baseline)            |
| `user-api-v2.json` | User API v2.0.0 with breaking changes |

### Breaking Changes in v2.0.0

- `DELETE /users/{id}` endpoint removed
- `email` field removed from User schema

## Adding New Examples

To add a new example:

1. Create a new directory under the appropriate language folder (e.g., `go/myexample/`)
2. Add a `go.mod` file with a replace directive for the local SDK
3. Create `main.go` with clear documentation and step-by-step output
4. Update this README with the new example

## Related Documentation

- [Consumer Testing Guide](../docs/consumer-testing-guide.md) - Detailed guide for consumer testing
- [Producer Testing Plan](../docs/producer-testing-plan.md) - Producer testing architecture
- [SDK Documentation](../sdks/) - Language-specific SDK documentation
