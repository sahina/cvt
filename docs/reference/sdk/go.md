---
title: Go SDK
sidebar_label: Go
sidebar_position: 4
description: CVT SDK for Go
---

# Go SDK

The Go SDK provides contract validation for Go applications with full type safety.

## Installation

```bash
go get github.com/sahina/cvt/sdks/go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    "github.com/sahina/cvt/sdks/go/cvt"
)

func main() {
    ctx := context.Background()

    validator, err := cvt.NewValidator("localhost:9550")
    if err != nil {
        panic(err)
    }
    defer validator.Close()

    // Register a schema
    err = validator.RegisterSchema(ctx, "user-api", schemaContent)
    if err != nil {
        panic(err)
    }

    // Validate an interaction
    result, err := validator.Validate(ctx,
        cvt.Request{Method: "GET", Path: "/users/123"},
        cvt.Response{StatusCode: 200, Body: `{"id": "123", "name": "John"}`},
    )
    if err != nil {
        panic(err)
    }

    fmt.Println("Valid:", result.Valid)
}
```

## API Reference

### NewValidator

```go
func NewValidator(address string, opts ...Option) (*Validator, error)
```

#### Options

```go
cvt.WithTLS(certFile, keyFile, caFile string)
cvt.WithAPIKey(key string)
cvt.WithTimeout(d time.Duration)
cvt.WithInsecure()  // Skip TLS verification (dev only)
```

### Validator Methods

#### RegisterSchema

```go
func (v *Validator) RegisterSchema(
    ctx context.Context,
    schemaID string,
    content string,
    opts ...RegisterOption,
) error
```

```go
// With version
err := validator.RegisterSchema(ctx, "my-api", content,
    cvt.WithSchemaVersion("1.0.0"),
    cvt.WithOwnership(cvt.Ownership{Owner: "team-a"}),
)
```

#### Validate

```go
func (v *Validator) Validate(
    ctx context.Context,
    request Request,
    response Response,
) (*ValidationResult, error)
```

#### RegisterConsumer

```go
func (v *Validator) RegisterConsumer(
    ctx context.Context,
    opts RegisterConsumerOptions,
) (*ConsumerInfo, error)
```

#### ListConsumers

```go
func (v *Validator) ListConsumers(
    ctx context.Context,
    schemaID string,
    environment string,
) ([]ConsumerInfo, error)
```

#### DeregisterConsumer

```go
func (v *Validator) DeregisterConsumer(
    ctx context.Context,
    consumerID, schemaID, environment string,
) error
```

#### CompareSchemas

```go
func (v *Validator) CompareSchemas(
    ctx context.Context,
    schemaID, oldVersion, newVersion string,
) (*CompareSchemasResponse, error)
```

#### CanIDeploy

```go
func (v *Validator) CanIDeploy(
    ctx context.Context,
    schemaID, newVersion, environment string,
) (*CanIDeployResponse, error)
```

#### GenerateFixture

```go
func (v *Validator) GenerateFixture(
    ctx context.Context,
    opts GenerateFixtureOptions,
) (*GeneratedFixture, error)
```

## HTTP Adapters

### RoundTripper Adapter

Wrap `http.Client` for automatic validation:

```go
import "github.com/sahina/cvt/sdks/go/cvt/adapters"

validator, _ := cvt.NewValidator("localhost:9550")
validator.RegisterSchema(ctx, "user-api", schema)

client := &http.Client{
    Transport: adapters.NewValidatingRoundTripper(
        http.DefaultTransport,
        validator,
        "user-api",
        adapters.WithOnValidationFailure(func(r *cvt.ValidationResult) error {
            return fmt.Errorf("contract violation: %v", r.Errors)
        }),
    ),
}

// All requests are now validated
resp, _ := client.Get("http://user-service/users/123")
```

### Mock RoundTripper

Generate responses from schema without a real API:

```go
mock := adapters.NewMock(validator, adapters.WithCache())
mockClient := mock.Client()

// Responses are auto-generated from schema
resp, _ := mockClient.Do(req)

// Get recorded interactions for consumer registration
interactions := mock.GetInteractions()
```

## Producer Middleware

### net/http

```go
import "github.com/sahina/cvt/sdks/go/cvt/producer/adapters"

config := producer.Config{
    SchemaID:  "my-api",
    Validator: validator,
    Mode:      producer.ModeStrict, // ModeStrict | ModeWarn | ModeShadow
}

handler := adapters.NetHTTPMiddleware(config)(myHandler)
http.Handle("/", handler)
```

### Gin

```go
import "github.com/gin-gonic/gin"
import "github.com/sahina/cvt/sdks/go/cvt/producer/adapters"

router := gin.Default()
router.Use(adapters.GinMiddleware(config))

router.GET("/users/:id", func(c *gin.Context) {
    c.JSON(200, gin.H{"id": c.Param("id"), "name": "John"})
})
```

### Chi

```go
import "github.com/go-chi/chi/v5"
import "github.com/sahina/cvt/sdks/go/cvt/producer/adapters"

router := chi.NewRouter()
router.Use(adapters.ChiMiddleware(config))
```

## Producer Test Kit

```go
import "github.com/sahina/cvt/sdks/go/cvt/producer"

func TestUserHandler(t *testing.T) {
    testKit, err := producer.NewProducerTestKit(producer.TestConfig{
        SchemaID:      "user-api",
        ServerAddress: "localhost:9550",
    })
    require.NoError(t, err)
    defer testKit.Close()

    t.Run("returns valid response", func(t *testing.T) {
        result, err := testKit.ValidateResponse(ctx, producer.ValidateResponseParams{
            Method:     "GET",
            Path:       "/users/123",
            StatusCode: 200,
            Body:       map[string]interface{}{"id": "123", "name": "John"},
        })

        require.NoError(t, err)
        assert.True(t, result.Valid)
    })
}
```

## Auto-Registration

Build consumer info from captured interactions:

```go
import "github.com/sahina/cvt/sdks/go/cvt"

// From MockingRoundTripper
interactions := mock.GetInteractions()

consumerInfo, err := validator.RegisterConsumerFromInteractions(ctx, interactions, cvt.AutoRegisterConfig{
    ConsumerID:      "order-service",
    ConsumerVersion: "2.1.0",
    Environment:     "dev",
    SchemaVersion:   "1.0.0",
})
```

## TLS Configuration

```go
// TLS
validator, _ := cvt.NewValidator("localhost:9550",
    cvt.WithTLS("./certs/ca.crt", "", ""),
)

// mTLS
validator, _ := cvt.NewValidator("localhost:9550",
    cvt.WithTLS(
        "./certs/ca.crt",
        "./certs/client.crt",
        "./certs/client.key",
    ),
)
```

## API Key Authentication

```go
validator, _ := cvt.NewValidator("localhost:9550",
    cvt.WithAPIKey("your-api-key"),
)
```

## Types

```go
type Request struct {
    Method  string
    Path    string
    Headers map[string]string
    Body    string
}

type Response struct {
    StatusCode int
    Headers    map[string]string
    Body       string
}

type ValidationResult struct {
    Valid                   bool
    Errors                  []string
    ValidatedAgainstVersion string
    ValidatedAgainstHash    string
}

type BreakingChange struct {
    Type        BreakingChangeType
    Path        string
    Method      string
    Description string
    OldValue    string
    NewValue    string
}

type ConsumerInfo struct {
    ConsumerID      string
    ConsumerVersion string
    SchemaID        string
    SchemaVersion   string
    Environment     string
    UsedEndpoints   []EndpointUsage
}
```

## Error Handling

```go
result, err := validator.Validate(ctx, request, response)
if err != nil {
    if errors.Is(err, cvt.ErrSchemaNotFound) {
        log.Println("Schema not registered")
    } else if errors.Is(err, cvt.ErrConnectionFailed) {
        log.Println("Cannot connect to CVT server")
    }
    return err
}
```

## Related Documentation

- **[Consumer Testing Guide](../../guides/consumer-testing.md)** - Testing your API integrations
- **[Producer Testing Guide](../../guides/producer-testing.md)** - Validating your APIs
- **[API Reference](../api.md)** - Full gRPC API documentation
