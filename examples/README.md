# CVT Examples

Example applications demonstrating CVT (Contract Validator Toolkit) capabilities.

## Quick Start

```bash
# Terminal 1: Start server
make run-server

# Terminal 2: Run example
cd examples/go/consumer
go run main.go
```

## Available Examples

| Example          | Directory      | Description                                                      |
| ---------------- | -------------- | ---------------------------------------------------------------- |
| Consumer Testing | `go/consumer/` | Schema registration, validation, consumer registry, deploy check |

## What the Example Demonstrates

1. **Schema Registration** - Register OpenAPI schema with version enforcement
2. **Interaction Validation** - Validate request/response pairs (manual and auto-mock)
3. **Consumer Registration** - Track API dependencies (auto and manual)
4. **Deployment Safety** - Check if schema changes break consumers

## Test Schemas

Located in `examples/schemas/`:

| File               | Description                           |
| ------------------ | ------------------------------------- |
| `user-api-v1.json` | User API v1.0.0 (baseline)            |
| `user-api-v2.json` | User API v2.0.0 with breaking changes |

Breaking changes in v2.0.0:

- `DELETE /users/{id}` endpoint removed
- `email` field removed from User schema

## Related Documentation

- [Consumer Testing Guide](../docs/consumer-testing-guide.md)
- [Producer Testing Guide](../docs/producer-testing.md)
