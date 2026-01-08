# Consumer Testing Guide

This guide walks you through testing CVT's consumer testing capabilities using the example application.

## Prerequisites

1. **Go 1.21+** installed
2. **CVT server running** - Start with `make up` from the project root

## Quick Start

```bash
# 1. Start CVT server (from project root)
make up

# 2. Run the consumer testing example
cd examples/go/consumer
go run main.go
```

## What the Example Tests

The consumer example (`examples/go/consumer/main.go`) demonstrates the full consumer testing workflow using a simple User API schema.

### Test Schemas

The example uses two schema versions located in `examples/schemas/`:

| File               | Description                             |
| ------------------ | --------------------------------------- |
| `user-api-v1.json` | User API v1.0.0 - baseline schema       |
| `user-api-v2.json` | User API v2.0.0 - with breaking changes |

**Breaking changes in v2.0.0:**

- `DELETE /users/{id}` endpoint removed
- `email` field removed from User schema

### Step 1: Schema Registration

Register a producer's OpenAPI schema that your consumer depends on:

```go
validator.RegisterSchemaWithVersion(ctx, "user-api", schemaPath, "1.0.0")
```

This registers the User API schema v1.0.0 with the CVT server.

### Step 2: Interaction Validation

Validate that your API calls match the schema:

```go
request := cvt.ValidationRequest{
    Method: "GET",
    Path:   "/users/123",
    Headers: map[string]string{"Accept": "application/json"},
}
response := cvt.ValidationResponse{
    StatusCode: 200,
    Body: map[string]interface{}{
        "id": "123", "name": "John Doe", "email": "john@example.com",
    },
}

result, _ := validator.Validate(ctx, request, response)
// result.Valid == true
```

### Step 3: Consumer Registration

Register your service as a consumer to track dependencies:

```go
validator.RegisterConsumer(ctx, cvt.RegisterConsumerOptions{
    ConsumerID:      "order-service",
    ConsumerVersion: "2.1.0",
    SchemaID:        "user-api",
    SchemaVersion:   "1.0.0",
    Environment:     "dev",
    UsedEndpoints: []cvt.EndpointUsage{
        {
            Method:     "GET",
            Path:       "/users/{id}",
            UsedFields: []string{"id", "name", "email"},
        },
    },
})
```

This tells CVT:

- **Who you are**: `order-service` v2.1.0
- **What you depend on**: `user-api` v1.0.0
- **What you use**: `GET /users/{id}` endpoint, specifically the `id`, `name`, and `email` fields

### Step 4: List Consumers

Query all consumers registered for a schema:

```go
consumers, _ := validator.ListConsumers(ctx, "user-api", "dev")
// consumers = [{ConsumerID: "order-service", ...}]
```

### Step 5: Deployment Safety Check

Before deploying a new schema version, check if it's safe:

```go
// Register the new schema version
validator.RegisterSchemaWithVersion(ctx, "user-api", schemaV2Path, "2.0.0")

// Check if it's safe to deploy
result, _ := validator.CanIDeploy(ctx, "user-api", "2.0.0", "dev")

if result.SafeToDeploy {
    fmt.Println("Safe to deploy!")
} else {
    fmt.Println("UNSAFE: Breaking changes detected")
    for _, change := range result.BreakingChanges {
        fmt.Printf("- %s: %s\n", change.Type, change.Description)
    }
    for _, consumer := range result.AffectedConsumers {
        fmt.Printf("Affected: %s v%s\n", consumer.ConsumerID, consumer.ConsumerVersion)
    }
}
```

### Step 6: Cleanup

Deregister consumers when no longer needed:

```go
validator.DeregisterConsumer(ctx, "order-service", "user-api", "dev")
```

## Expected Output

When you run the example, you should see:

```text
=== CVT Consumer Testing Example ===

This example demonstrates the full consumer testing workflow:
  1. Register producer's OpenAPI schema
  2. Validate API interactions
  3. Register as a consumer (track dependencies)
  4. List registered consumers
  5. Check deployment safety (CanIDeploy)
  6. Cleanup (deregister consumer)

============================================================

Step 1: Registering producer's OpenAPI schema (v1.0.0)...
        Schema v1.0.0 registered successfully.

Step 2: Validating API interactions against the schema...
        Valid interaction: GET /users/123

Step 3: Registering as a consumer (order-service)...
        Consumer registered: order-service v2.1.0
        Uses schema: user-api v1.0.0
        Environment: dev
        Tracked endpoints: 2

Step 4: Listing registered consumers for this schema...
        Found 1 consumer(s) in dev environment:
          - order-service v2.1.0 (uses schema v1.0.0)

Step 5: Registering schema v2.0.0 (with breaking changes)...
        Schema v2.0.0 registered successfully.

Step 6: Checking deployment safety (can-i-deploy v2.0.0 to dev)...
------------------------------------------------------------
RESULT: UNSAFE TO DEPLOY

Summary: Deployment may affect 1 consumer(s) - review required

Affected consumers: 1
  - order-service v2.1.0 (impact: BREAKING)
------------------------------------------------------------

Step 7: Cleaning up (deregistering consumer)...
        Consumer deregistered successfully.
        Remaining consumers in dev: 0

============================================================
Consumer Testing Example Complete!
```

## Testing Scenarios

### Scenario 1: Valid API Interaction

Test that your request/response pairs are valid against the schema:

```go
// Valid: all required fields present
result, _ := validator.Validate(ctx, validRequest, validResponse)
assert.True(t, result.Valid)
```

### Scenario 2: Invalid API Interaction

Test that invalid interactions are caught:

```go
// Invalid: missing required 'name' field
invalidResponse := cvt.ValidationResponse{
    StatusCode: 200,
    Body: map[string]interface{}{
        "id": 1,
        // "name" is missing
    },
}
result, _ := validator.Validate(ctx, request, invalidResponse)
assert.False(t, result.Valid)
assert.Contains(t, result.Errors[0], "name")
```

### Scenario 3: Safe Deployment

Test that compatible schema changes are allowed:

```go
// Register v1.1.0 with only additive changes
validator.RegisterSchemaWithVersion(ctx, schemaID, compatibleSchema, "1.1.0")

result, _ := validator.CanIDeploy(ctx, schemaID, "1.1.0", "dev")
assert.True(t, result.SafeToDeploy)
```

### Scenario 4: Unsafe Deployment

Test that breaking changes are blocked:

```go
// Register v2.0.0 with breaking changes
validator.RegisterSchemaWithVersion(ctx, schemaID, breakingSchema, "2.0.0")

result, _ := validator.CanIDeploy(ctx, schemaID, "2.0.0", "dev")
assert.False(t, result.SafeToDeploy)
assert.Greater(t, len(result.BreakingChanges), 0)
assert.Greater(t, len(result.AffectedConsumers), 0)
```

## CLI Alternative

You can also test deployment safety using the CVT CLI:

```bash
# Build the CLI
go build -o cvt ./cmd/cvt

# Check deployment safety
./cvt can-i-deploy --schema user-api --version 2.0.0 --env dev

# JSON output for CI/CD
./cvt can-i-deploy --schema user-api --version 2.0.0 --env dev --json
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Contract Tests

on: [push, pull_request]

jobs:
  contract-tests:
    runs-on: ubuntu-latest
    services:
      cvt:
        image: ghcr.io/cvt/cvt-server:latest
        ports:
          - 50051:50051

    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.21"

      - name: Run contract tests
        run: |
          cd examples/go/consumer
          go run main.go

      - name: Register consumer
        run: |
          # Use SDK or CLI to register consumer after successful tests
          ./cvt register-consumer \
            --id my-service \
            --version ${{ github.sha }} \
            --schema user-api \
            --env staging
```

### Pre-Deploy Safety Check

```yaml
deploy:
  needs: [contract-tests]
  steps:
    - name: Check deployment safety
      run: |
        result=$(./cvt can-i-deploy \
          --schema user-api \
          --version ${{ env.NEW_VERSION }} \
          --env prod \
          --json)

        if [ $(echo $result | jq '.safeToDeploy') != "true" ]; then
          echo "Unsafe to deploy - breaking changes detected"
          echo $result | jq '.affectedConsumers'
          exit 1
        fi
```

## Troubleshooting

### "Failed to create validator"

Make sure CVT server is running:

```bash
make up
# or
docker-compose up -d cvt-server
```

### "Failed to register schema"

Check that the schema file exists:

```bash
ls sdks/shared/openapi-v1.json
```

### Connection refused

The default server address is `localhost:50052`. If your server is on a different port:

```go
validator, _ := cvt.NewValidator("localhost:50051")
```

## Next Steps

- Review the [Producer Testing Guide](./producer-testing.md) for producer-side testing
- Check the [Consumer Testing Plan](./consumer-testing-plan.md) for implementation details
- Explore the [SDK documentation](../sdks/) for language-specific features
