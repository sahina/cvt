# Contract Validator Toolkit (CVT) - Go SDK

The **CVT Go SDK** allows you to validate HTTP interactions (requests and responses) against OpenAPI schemas using the CVT gRPC service.

> **Status**: Fully Implemented

## Installation

**Note**: This package is currently for internal/development use.

To use locally, add a `replace` directive to your `go.mod`:

```go
replace github.com/sahina/cvt/sdks/go => ../path/to/cvt/sdks/go
```

Then require it:

```bash
go get github.com/sahina/cvt/sdks/go/cvt
```

## Usage

### Initialize and Register Schema

```go
package main

import (
    "context"
    "log"
    "github.com/sahina/cvt/sdks/go/cvt"
)

func main() {
    validator, err := cvt.NewValidator(cvt.Config{
        Host: "localhost:9550",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer validator.Close()

    // Register from local file
    err = validator.RegisterSchema(context.Background(), "my-schema", "path/to/openapi.json")
    if err != nil {
        log.Fatal(err)
    }

    // Register from URL
    err = validator.RegisterSchema(context.Background(), "petstore",
        "https://petstore.swagger.io/v2/swagger.json")
    if err != nil {
        log.Fatal(err)
    }
}
```

### Validate Interactions

```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/sahina/cvt/sdks/go/cvt"
)

func main() {
    validator, _ := cvt.NewValidator(cvt.Config{Host: "localhost:9550"})
    defer validator.Close()

    request := cvt.Request{
        Method: "POST",
        Path:   "/users",
        Body:   map[string]any{"username": "alice", "email": "alice@example.com"},
    }

    response := cvt.Response{
        StatusCode: 201,
    }

    result, err := validator.Validate(context.Background(), request, response)
    if err != nil {
        log.Fatal(err)
    }

    if result.Valid {
        fmt.Println("✅ Valid interaction")
    } else {
        fmt.Printf("❌ Validation errors: %v\n", result.Errors)
    }
}
```

## HTTP Adapters

The SDK includes adapters for automatic HTTP traffic validation.

### Client-Side (RoundTripper)

```go
import (
    "net/http"
    "github.com/sahina/cvt/sdks/go/cvt"
    "github.com/sahina/cvt/sdks/go/cvt/adapters"
)

validator, _ := cvt.NewValidator(cvt.Config{Host: "localhost:9550"})
validator.RegisterSchema(ctx, "petstore", "./openapi.json")

// Wrap http.Client transport
rt := adapters.NewValidatingRoundTripper(adapters.RoundTripperConfig{
    Validator:       validator,
    SchemaID:        "petstore",
    AutoValidate:    true,
    ExcludePaths:    []string{"/health", "/metrics"},
    OnValidationFailure: func(result *cvt.ValidationResult, interaction *cvt.Interaction) {
        log.Printf("Validation failed: %v", result.Errors)
    },
})

client := &http.Client{Transport: rt}
resp, _ := client.Post("https://api.example.com/pets", "application/json", body)
```

## Producer Validation (Server-Side Middleware)

Validate incoming requests and outgoing responses against your OpenAPI contract on the server side.

> **Full documentation:** See [Validation Modes](../../docs/guides/validation-modes.md) for detailed behavior, rollout strategy, and metrics information.

### Validation Modes

| Mode                  | Request Violation | Response Violation | Use Case               |
| --------------------- | ----------------- | ------------------ | ---------------------- |
| `producer.ModeStrict` | Reject with 400   | Log error          | Production enforcement |
| `producer.ModeWarn`   | Log, continue     | Log, continue      | Gradual rollout        |
| `producer.ModeShadow` | Metrics only      | Metrics only       | Initial deployment     |

**Recommended rollout:** `ModeShadow` → `ModeWarn` → `ModeStrict`. See [Recommended Rollout Strategy](../../docs/guides/validation-modes.md#recommended-rollout-strategy).

### net/http Middleware

```go
import (
    "net/http"
    "github.com/sahina/cvt/sdks/go/cvt/producer"
    "github.com/sahina/cvt/sdks/go/cvt/producer/adapters"
)

config := producer.Config{
    SchemaID:     "my-api",
    Validator:    validator,
    Mode:         producer.ModeStrict,
    ExcludePaths: []string{"/health", "/metrics"},
}

handler := adapters.NetHTTPMiddleware(config)(yourHandler)
http.ListenAndServe(":8080", handler)
```

### Gin Middleware

```go
import "github.com/sahina/cvt/sdks/go/cvt/producer/adapters"

router := gin.Default()
router.Use(adapters.GinMiddleware(config))
```

### Chi Middleware

```go
import "github.com/sahina/cvt/sdks/go/cvt/producer/adapters"

r := chi.NewRouter()
r.Use(adapters.ChiMiddleware(config))
```

### Configuration Options

| Option              | Type        | Description                                |
| ------------------- | ----------- | ------------------------------------------ |
| `SchemaID`          | `string`    | Schema ID to validate against              |
| `Validator`         | `Validator` | ContractValidator instance                 |
| `Mode`              | `Mode`      | `ModeStrict`, `ModeWarn`, or `ModeShadow`  |
| `ExcludePaths`      | `[]string`  | Paths to skip validation (e.g., `/health`) |
| `IncludePaths`      | `[]string`  | Only validate matching paths               |
| `ValidateResponse`  | `bool`      | Enable response validation (default: true) |
| `OnValidationError` | `func`      | Custom error handler callback              |

## HTTP Adapter Options

- `AutoValidate`: Enable/disable automatic validation (default: true)
- `IncludePaths`: Slice of paths/regex to include
- `ExcludePaths`: Slice of paths/regex to exclude
- `OnValidationFailure`: Custom error handler
- `GetInteractions()`: Retrieve captured interactions
- `ClearInteractions()`: Reset captured data

## Breaking Change Detection

Detect breaking changes between OpenAPI schema versions before deployment:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "github.com/sahina/cvt/sdks/go/cvt"
)

func main() {
    validator, _ := cvt.NewValidator(cvt.Config{Host: "localhost:9550"})
    defer validator.Close()

    ctx := context.Background()

    // Register both schema versions
    validator.RegisterSchemaWithVersion(ctx, "my-api", "./openapi-v1.json", "1.0.0")
    validator.RegisterSchemaWithVersion(ctx, "my-api", "./openapi-v2.json", "2.0.0")

    // Compare versions
    result, err := validator.CompareSchemas(ctx, "my-api", "1.0.0", "2.0.0")
    if err != nil {
        log.Fatal(err)
    }

    if !result.Compatible {
        fmt.Println("Breaking changes detected:")
        for _, change := range result.BreakingChanges {
            fmt.Printf("- [%s] %s\n", change.Type, change.Description)
            if change.Path != "" {
                fmt.Printf("  Path: %s %s\n", change.Method, change.Path)
            }
        }
        os.Exit(1) // Fail CI build
    }
}
```

### Breaking Change Types

| Type                   | Description                           |
| ---------------------- | ------------------------------------- |
| `ENDPOINT_REMOVED`     | An endpoint was removed               |
| `REQUIRED_FIELD_ADDED` | A required field was added to request |
| `FIELD_TYPE_CHANGED`   | A field's type was changed            |
| `ENUM_VALUE_REMOVED`   | An allowed enum value was removed     |

See `examples/breaking/main.go` for a complete example.

## Producer Testing

Test that your API handlers return responses matching your OpenAPI specification.

### ProducerTestKit

```go
import "github.com/sahina/cvt/sdks/go/cvt/producer"

testKit, err := producer.NewProducerTestKit(producer.TestConfig{
    SchemaID:      "user-api",
    ServerAddress: "localhost:9550",
})
if err != nil {
    log.Fatal(err)
}
defer testKit.Close()

// Validate handler response
result, err := testKit.ValidateResponse(ctx, producer.ValidateResponseParams{
    Method:     "GET",
    Path:       "/users/123",
    Response: producer.TestResponseData{
        StatusCode: 200,
        Body:       map[string]any{"id": "123", "name": "Alice", "email": "alice@example.com"},
    },
})

assert.True(t, result.Valid)
```

### Consumer Registry

Track which services depend on your API:

```go
// Register a consumer after successful contract tests
consumer, err := validator.RegisterConsumer(ctx, cvt.RegisterConsumerOptions{
    ConsumerID:      "order-service",
    ConsumerVersion: "2.1.0",
    SchemaID:        "user-api",
    SchemaVersion:   "1.0.0",
    Environment:     "prod",
    UsedEndpoints: []cvt.EndpointUsage{
        {Method: "GET", Path: "/users/{id}", UsedFields: []string{"id", "email"}},
    },
})

// List all consumers of a schema
consumers, err := validator.ListConsumers(ctx, "user-api", "prod")

// Deregister a consumer
err = validator.DeregisterConsumer(ctx, "order-service", "user-api", "prod")
```

### Deployment Safety (can-i-deploy)

Check if a new schema version can be safely deployed:

```go
result, err := validator.CanIDeploy(ctx, "user-api", "2.0.0", "prod")

if !result.SafeToDeploy {
    log.Printf("Cannot deploy: %s", result.Summary)
    for _, consumer := range result.AffectedConsumers {
        if consumer.WillBreak {
            log.Printf("- %s will break", consumer.ConsumerID)
        }
    }
    os.Exit(1)
}
```

See [Producer Testing Guide](../../docs/guides/producer-testing.md) for complete documentation.

## Security Configuration

### TLS

```go
validator, _ := cvt.NewValidator(cvt.Config{
    Host: "localhost:9550",
    TLS: &cvt.TLSConfig{
        Enabled:        true,
        RootCertPath:   "./certs/ca.crt",
        ClientCertPath: "./certs/client.crt",  // For mTLS
        ClientKeyPath:  "./certs/client.key",  // For mTLS
    },
})
```

### API Key Authentication

```go
validator, _ := cvt.NewValidator(cvt.Config{
    Host:   "localhost:9550",
    APIKey: "your-api-key-here",
})
```

## Prerequisites

Ensure the CVT gRPC server is running (default: `localhost:9550`).

## Testing

The Go SDK includes tests covering:

- Client initialization and configuration
- Schema registration
- Validation requests and responses
- Error handling

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html

# Run tests with verbose output
go test -v ./...

# Run specific test
go test -run TestValidateCorrectInteraction ./...
```

### Test Structure

```shell
cvt/
├── validator_test.go       # Main SDK test suite
├── registration_test.go    # Schema registration tests
└── integration_test.go     # Integration tests
```

### Writing Tests

Example test using Go's testing package:

```go
package cvt_test

import (
    "context"
    "testing"
    "github.com/sahina/cvt/sdks/go/cvt"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestValidateCorrectInteraction(t *testing.T) {
    validator, err := cvt.NewValidator(cvt.Config{
        Host: "localhost:9550",
    })
    require.NoError(t, err)
    defer validator.Close()

    err = validator.RegisterSchema(context.Background(),
        "test", "testdata/openapi.json")
    require.NoError(t, err)

    result, err := validator.Validate(
        context.Background(),
        cvt.Request{Method: "GET", Path: "/users"},
        cvt.Response{StatusCode: 200, Body: []any{}},
    )

    require.NoError(t, err)
    assert.True(t, result.Valid)
}

func TestValidateIncorrectInteraction(t *testing.T) {
    validator, err := cvt.NewValidator(cvt.Config{
        Host: "localhost:9550",
    })
    require.NoError(t, err)
    defer validator.Close()

    err = validator.RegisterSchema(context.Background(),
        "test", "testdata/openapi.json")
    require.NoError(t, err)

    result, err := validator.Validate(
        context.Background(),
        cvt.Request{Method: "GET", Path: "/users"},
        cvt.Response{StatusCode: 500}, // Should be 200
    )

    require.NoError(t, err)
    assert.False(t, result.Valid)
    assert.NotEmpty(t, result.Errors)
}
```

### Coverage

The SDK targets 60%+ test coverage.

## Development

```bash
# Install dependencies
go mod download

# Run linter
golangci-lint run

# Format code
go fmt ./...

# Run vet
go vet ./...

# Build
go build ./...

# Update dependencies
go mod tidy
```

## Contributing

Contributions are welcome!

## License

MIT License
