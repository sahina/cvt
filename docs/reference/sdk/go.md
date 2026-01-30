---
title: Go SDK
sidebar_label: Go
sidebar_position: 4
description: CVT SDK for Go
---

# Go SDK

## What is the Go SDK?

The Go SDK provides contract validation for Go applications with full type safety. It includes HTTP client adapters for automatic validation, mock response generation, producer middleware for popular frameworks (net/http, Gin, Chi), and a test kit for schema compliance testing.

## Installation

```bash
# From local clone (SDK not published to public registries)
# Add to go.mod with replace directive
go mod edit -replace github.com/sahina/cvt/sdks/go=./path/to/cvt/sdks/go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/sahina/cvt/sdks/go/cvt"
)

func main() {
    ctx := context.Background()

    validator, err := cvt.NewValidator("localhost:9550")
    if err != nil {
        log.Fatal(err)
    }
    defer validator.Close()

    // Register a schema from file
    err = validator.RegisterSchema(ctx, "petstore", "./openapi.json")
    // Or register from URL:
    // err = validator.RegisterSchema(ctx, "petstore", "https://petstore3.swagger.io/api/v3/openapi.json")
    if err != nil {
        log.Fatal(err)
    }

    // Validate an interaction
    result, err := validator.Validate(ctx,
        cvt.ValidationRequest{Method: "GET", Path: "/pet/123"},
        cvt.ValidationResponse{StatusCode: 200, Body: map[string]any{"id": 123, "name": "doggie", "status": "available"}},
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Valid:", result.Valid)
    if !result.Valid {
        fmt.Println("Errors:", result.Errors)
    }
}
```

## API Reference

### ContractValidator

#### Constructor

```go
// Simple usage (insecure connection)
func NewValidator(address string) (*Validator, error)

// With options (TLS and API key)
func NewValidatorWithOptions(opts ValidatorOptions) (*Validator, error)
```

**Simple usage:**

```go
validator, err := cvt.NewValidator("localhost:9550")
if err != nil {
    log.Fatal(err)
}
defer validator.Close()
```

**With options:**

```go
validator, err := cvt.NewValidatorWithOptions(cvt.ValidatorOptions{
    Address: "localhost:9550",
    TLS: &cvt.TLSOptions{
        Enabled:      true,
        RootCertPath: "./certs/ca.crt",
    },
    APIKey: "your-api-key",
})
if err != nil {
    log.Fatal(err)
}
defer validator.Close()
```

| Option | Type | Description |
|--------|------|-------------|
| `Address` | `string` | Server address (default: `localhost:9550`) |
| `TLS.Enabled` | `bool` | Enable TLS |
| `TLS.RootCertPath` | `string` | Path to CA certificate |
| `TLS.CertPath` | `string` | Path to client certificate (for mTLS) |
| `TLS.KeyPath` | `string` | Path to client private key (for mTLS) |
| `APIKey` | `string` | API key for authentication |

### Validator Methods

#### RegisterSchema

Registers an OpenAPI schema from a file path or URL.

```go
func (v *Validator) RegisterSchema(ctx context.Context, schemaID, schemaPath string) error
```

```go
// From local file
err := validator.RegisterSchema(ctx, "petstore", "./openapi.json")

// From URL
err := validator.RegisterSchema(ctx, "petstore", "https://petstore3.swagger.io/api/v3/openapi.json")
```

#### RegisterSchemaWithVersion

Registers a schema with version information for comparison.

```go
func (v *Validator) RegisterSchemaWithVersion(ctx context.Context, schemaID, schemaPath, version string) error
```

```go
err := validator.RegisterSchemaWithVersion(ctx, "petstore", "./openapi.json", "1.0.0")
```

#### Validate

Validates an HTTP request/response pair against the registered schema.

```go
func (v *Validator) Validate(
    ctx context.Context,
    request ValidationRequest,
    response ValidationResponse,
) (*ValidationResult, error)
```

```go
result, err := validator.Validate(ctx,
    cvt.ValidationRequest{
        Method:  "GET",
        Path:    "/pet/123",
        Headers: map[string]string{"Accept": "application/json"},
    },
    cvt.ValidationResponse{
        StatusCode: 200,
        Body:       map[string]any{"id": 123, "name": "doggie"},
    },
)
if err != nil {
    log.Fatal(err)
}
if !result.Valid {
    fmt.Println("Errors:", result.Errors)
}
```

#### CompareSchemas

Compares two schema versions for breaking changes.

```go
func (v *Validator) CompareSchemas(
    ctx context.Context,
    schemaID, oldVersion, newVersion string,
) (*CompareResult, error)
```

```go
result, err := validator.CompareSchemas(ctx, "petstore", "1.0.0", "2.0.0")
if err != nil {
    log.Fatal(err)
}
if !result.Compatible {
    for _, change := range result.BreakingChanges {
        fmt.Printf("- %s: %s\n", change.Type, change.Description)
    }
}
```

#### GenerateFixture

Generates test fixtures from the schema.

```go
func (v *Validator) GenerateFixture(
    ctx context.Context,
    method, path string,
    opts *GenerateOptions,
) (*GeneratedFixture, error)
```

```go
fixture, err := validator.GenerateFixture(ctx, "GET", "/pet/{petId}", nil)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Request: %+v\n", fixture.Request)
fmt.Printf("Response: %+v\n", fixture.Response)
```

#### GenerateResponse

Generates a response fixture only.

```go
func (v *Validator) GenerateResponse(
    ctx context.Context,
    method, path string,
    opts *GenerateOptions,
) (*GeneratedResponse, error)
```

#### ListEndpoints

Lists all endpoints in the registered schema.

```go
func (v *Validator) ListEndpoints(ctx context.Context) ([]EndpointInfo, error)
```

```go
endpoints, err := validator.ListEndpoints(ctx)
for _, ep := range endpoints {
    fmt.Printf("%s %s - %s\n", ep.Method, ep.Path, ep.Summary)
}
```

#### RegisterConsumer

Registers a consumer with expected interactions.

```go
func (v *Validator) RegisterConsumer(
    ctx context.Context,
    opts RegisterConsumerOptions,
) (*ConsumerInfo, error)
```

```go
consumer, err := validator.RegisterConsumer(ctx, cvt.RegisterConsumerOptions{
    ConsumerID:      "order-service",
    ConsumerVersion: "2.1.0",
    SchemaID:        "petstore",
    SchemaVersion:   "1.0.0",
    Environment:     "prod",
    UsedEndpoints: []cvt.EndpointUsage{
        {Method: "GET", Path: "/pet/{petId}", UsedFields: []string{"id", "name", "status"}},
    },
})
```

#### ListConsumers

Lists all consumers for a schema.

```go
func (v *Validator) ListConsumers(
    ctx context.Context,
    schemaID, environment string,
) ([]ConsumerInfo, error)
```

```go
consumers, err := validator.ListConsumers(ctx, "petstore", "prod")
for _, c := range consumers {
    fmt.Printf("%s v%s\n", c.ConsumerID, c.ConsumerVersion)
}
```

#### DeregisterConsumer

Removes a consumer registration.

```go
func (v *Validator) DeregisterConsumer(
    ctx context.Context,
    consumerID, schemaID, environment string,
) error
```

#### CanIDeploy

Checks if a schema version can be safely deployed.

```go
func (v *Validator) CanIDeploy(
    ctx context.Context,
    schemaID, newVersion, environment string,
) (*CanIDeployResult, error)
```

```go
result, err := validator.CanIDeploy(ctx, "petstore", "2.0.0", "prod")
if err != nil {
    log.Fatal(err)
}
if !result.SafeToDeploy {
    fmt.Println("Unsafe:", result.Summary)
    for _, c := range result.AffectedConsumers {
        fmt.Printf("- %s will break\n", c.ConsumerID)
    }
}
```

## HTTP Adapters

### ValidatingRoundTripper

Wrap `http.Client` for automatic validation:

```go
import "github.com/sahina/cvt/sdks/go/cvt/adapters"

ctx := context.Background()
validator, _ := cvt.NewValidator("localhost:9550")
validator.RegisterSchema(ctx, "petstore", "./openapi.json")

rt := adapters.NewValidatingRoundTripper(adapters.RoundTripperConfig{
    Validator:    validator,
    AutoValidate: true,
    OnValidationFailure: func(result *cvt.ValidationResult, req *http.Request, resp *http.Response) error {
        return fmt.Errorf("contract violation: %v", result.Errors)
    },
})

client := &http.Client{Transport: rt}

// All requests are now validated
resp, err := client.Get("http://petstore-service/pet/123")

// Check captured interactions
interactions := rt.GetInteractions()
fmt.Println(interactions[0].ValidationResult.Valid)
```

### MockingRoundTripper

Generate responses from schema without a real API:

```go
import "github.com/sahina/cvt/sdks/go/cvt/adapters"

ctx := context.Background()
validator, _ := cvt.NewValidator("localhost:9550")
validator.RegisterSchema(ctx, "petstore", "./openapi.json")

// Simple API
mock := adapters.NewMock(validator, adapters.WithCache())
client := mock.Client()

// Responses are auto-generated from schema
resp, _ := client.Get("http://mock.petstore/pet/123")

// Get recorded interactions for consumer registration
interactions := mock.GetInteractions()
```

Or use the direct constructor for more control:

```go
rt := adapters.NewMockingRoundTripper(adapters.MockingRoundTripperConfig{
    Validator:        validator,
    ValidateRequests: true,
    CacheResponses:   true,
})
client := &http.Client{Transport: rt}
```

## Producer Middleware

### net/http

```go
import "github.com/sahina/cvt/sdks/go/cvt/producer/adapters"

config := producer.Config{
    SchemaID:  "petstore",
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

router.GET("/pet/:petId", func(c *gin.Context) {
    c.JSON(200, gin.H{"id": c.Param("petId"), "name": "doggie", "status": "available"})
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

Test your API responses against your schema without real consumers:

```go
import "github.com/sahina/cvt/sdks/go/cvt/producer"

func TestPetHandler(t *testing.T) {
    testKit, err := producer.NewProducerTestKit(producer.TestConfig{
        SchemaID:      "petstore",
        ServerAddress: "localhost:9550",
    })
    require.NoError(t, err)
    defer testKit.Close()

    t.Run("returns valid response", func(t *testing.T) {
        result, err := testKit.ValidateResponse(ctx, producer.ValidateResponseParams{
            Method: "GET",
            Path:   "/pet/123",
            Response: producer.TestResponseData{
                StatusCode: 200,
                Body:       map[string]any{"id": 123, "name": "doggie", "status": "available"},
            },
        })

        require.NoError(t, err)
        assert.True(t, result.Valid)
    })
}
```

### ForEndpoint Helper

Test multiple scenarios for the same endpoint:

```go
getPetEndpoint := testKit.ForEndpoint("GET", "/pet/{petId}")

// Test valid response
result, err := getPetEndpoint.ValidateResponse(ctx, producer.TestResponseData{
    StatusCode: 200,
    Body:       map[string]any{"id": 123, "name": "doggie", "status": "available"},
}, map[string]string{"petId": "123"})

// Test not found
result, err = getPetEndpoint.ValidateResponse(ctx, producer.TestResponseData{
    StatusCode: 404,
    Body:       map[string]any{"message": "Pet not found"},
}, map[string]string{"petId": "999"})
```

## Auto-Registration

Build consumer info from captured interactions:

```go
import "github.com/sahina/cvt/sdks/go/cvt"

// From MockingRoundTripper
mock := adapters.NewMock(validator)
client := mock.Client()

// Run your tests
resp, _ := client.Get("http://mock.petstore/pet/123")

// Auto-register consumer from captured interactions
consumerInfo, err := validator.RegisterConsumerFromInteractions(ctx, mock.GetInteractions(), cvt.AutoRegisterConfig{
    ConsumerID:      "order-service",
    ConsumerVersion: "2.1.0",
    Environment:     "dev",
    SchemaVersion:   "1.0.0",
})
```

Or preview before registering:

```go
opts, err := validator.BuildConsumerFromInteractions(ctx, mock.GetInteractions(), cvt.AutoRegisterConfig{
    ConsumerID:      "order-service",
    ConsumerVersion: "2.1.0",
    Environment:     "dev",
    SchemaVersion:   "1.0.0",
})
fmt.Printf("Would register %d endpoints\n", len(opts.UsedEndpoints))
```

## TLS Configuration

```go
// TLS with CA certificate
validator, err := cvt.NewValidatorWithOptions(cvt.ValidatorOptions{
    Address: "localhost:9550",
    TLS: &cvt.TLSOptions{
        Enabled:      true,
        RootCertPath: "./certs/ca.crt",
    },
})

// Mutual TLS (mTLS)
validator, err := cvt.NewValidatorWithOptions(cvt.ValidatorOptions{
    Address: "localhost:9550",
    TLS: &cvt.TLSOptions{
        Enabled:      true,
        RootCertPath: "./certs/ca.crt",
        CertPath:     "./certs/client.crt",
        KeyPath:      "./certs/client.key",
    },
})
```

## API Key Authentication

```go
validator, err := cvt.NewValidatorWithOptions(cvt.ValidatorOptions{
    Address: "localhost:9550",
    APIKey:  "your-api-key",
})
```

## Types

```go
type ValidationRequest struct {
    Method  string
    Path    string
    Headers map[string]string
    Body    any
}

type ValidationResponse struct {
    StatusCode int
    Headers    map[string]string
    Body       any
}

type ValidationResult struct {
    Valid  bool
    Errors []string
}

type BreakingChange struct {
    Type        string
    Path        string
    Method      string
    Description string
    OldValue    string
    NewValue    string
}

type CompareResult struct {
    Compatible      bool
    BreakingChanges []BreakingChange
}

type ConsumerInfo struct {
    ConsumerID      string
    ConsumerVersion string
    SchemaID        string
    SchemaVersion   string
    Environment     string
    RegisteredAt    int64
    LastValidatedAt int64
    UsedEndpoints   []EndpointUsage
}

type EndpointUsage struct {
    Method     string
    Path       string
    UsedFields []string
}

type CanIDeployResult struct {
    SafeToDeploy      bool
    Summary           string
    BreakingChanges   []BreakingChange
    AffectedConsumers []ConsumerImpact
}
```

## Error Handling

```go
result, err := validator.Validate(ctx, request, response)
if err != nil {
    // Connection or transport errors
    log.Printf("Validation failed: %v", err)
    return err
}

// Validation errors are returned in the result, not as errors
if !result.Valid {
    log.Printf("Contract violations: %v", result.Errors)
}
```

## Related Documentation

- **[Consumer Testing Guide](../../guides/consumer-testing.mdx)** - Testing your API integrations
- **[Producer Testing Guide](../../guides/producer-testing.mdx)** - Validating your APIs
- **[API Reference](../api.mdx)** - Full gRPC API documentation
