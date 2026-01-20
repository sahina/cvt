// Package producer provides server-side HTTP middleware for validating
// incoming requests and outgoing responses against OpenAPI schemas.
//
// This file contains the ProducerTestKit for schema compliance testing.
package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	cvt "github.com/sahina/cvt/sdks/go/cvt"
	pb "github.com/sahina/cvt/sdks/go/cvt/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// TestConfig configures the ProducerTestKit.
type TestConfig struct {
	// SchemaID is the schema identifier to validate against (required).
	SchemaID string

	// SchemaVersion is an optional specific version to validate against.
	SchemaVersion string

	// ServerAddress is the CVT gRPC server address (default: "localhost:9550").
	ServerAddress string

	// TLS configures TLS for secure connections.
	// If nil or TLS.Enabled is false, an insecure connection is used.
	TLS *cvt.TLSOptions

	// APIKey is an optional API key for authentication.
	APIKey string
}

// TestResponseData represents HTTP response data for validation.
type TestResponseData struct {
	// StatusCode is the HTTP status code.
	StatusCode int

	// Headers are the response headers.
	Headers map[string]string

	// Body is the response body (can be any JSON-serializable type).
	Body any
}

// TestRequestContext provides optional request context for path parameter extraction.
type TestRequestContext struct {
	// Method is the HTTP method.
	Method string

	// Path is the request path.
	Path string

	// Headers are the request headers.
	Headers map[string]string

	// Body is the request body (can be any JSON-serializable type).
	Body any
}

// TestValidationResult represents the result of producer response validation.
type TestValidationResult struct {
	// Valid indicates whether validation passed.
	Valid bool

	// Errors contains validation error messages (empty if Valid is true).
	Errors []string

	// ValidatedAgainstVersion is the schema version used for validation.
	ValidatedAgainstVersion string

	// ValidatedAgainstHash is the schema hash used for validation.
	ValidatedAgainstHash string
}

// ValidateResponseParams contains parameters for validating a producer response.
type ValidateResponseParams struct {
	// Method is the HTTP method (GET, POST, etc.).
	Method string

	// Path is the API path with actual values (e.g., /users/123).
	Path string

	// Response is the response data to validate.
	Response TestResponseData

	// Request is optional request context for path parameter extraction.
	Request *TestRequestContext
}

// ProducerTestKit enables schema compliance testing for producers.
//
// It allows producers to validate their API responses against their OpenAPI
// schema without needing real consumers. Use it in your test suite to ensure
// your handler output matches your contract.
//
// Example usage:
//
//	func TestUserAPI(t *testing.T) {
//	    testKit, err := producer.NewProducerTestKit(producer.TestConfig{
//	        SchemaID:      "user-api",
//	        ServerAddress: "localhost:9550",
//	    })
//	    if err != nil {
//	        t.Fatal(err)
//	    }
//	    defer testKit.Close()
//
//	    // Call your handler
//	    response := myHandler.GetUser("123")
//
//	    // Validate against schema
//	    result, err := testKit.ValidateResponse(ctx, producer.ValidateResponseParams{
//	        Method: "GET",
//	        Path:   "/users/123",
//	        Response: producer.TestResponseData{
//	            StatusCode: 200,
//	            Body:       response,
//	        },
//	    })
//	    if err != nil {
//	        t.Fatal(err)
//	    }
//	    if !result.Valid {
//	        t.Errorf("Validation failed: %v", result.Errors)
//	    }
//	}
type ProducerTestKit struct {
	conn          *grpc.ClientConn
	client        pb.ContractValidatorClient
	schemaID      string
	schemaVersion string
	apiKey        string
}

// NewProducerTestKit creates a new ProducerTestKit instance.
func NewProducerTestKit(config TestConfig) (*ProducerTestKit, error) {
	if config.SchemaID == "" {
		return nil, fmt.Errorf("SchemaID is required")
	}

	address := config.ServerAddress
	if address == "" {
		address = "localhost:9550"
	}

	// Build dial options
	var dialOpts []grpc.DialOption

	if config.TLS != nil && config.TLS.Enabled {
		creds, err := cvt.CreateTLSCredentials(config.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS credentials: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Create gRPC connection
	conn, err := grpc.NewClient(address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CVT server: %w", err)
	}

	client := pb.NewContractValidatorClient(conn)

	return &ProducerTestKit{
		conn:          conn,
		client:        client,
		schemaID:      config.SchemaID,
		schemaVersion: config.SchemaVersion,
		apiKey:        config.APIKey,
	}, nil
}

// contextWithAPIKey adds the API key to the context if configured.
func (t *ProducerTestKit) contextWithAPIKey(ctx context.Context) context.Context {
	if t.apiKey == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "x-api-key", t.apiKey)
}

// serializeBody converts a body value to a JSON string.
func serializeBody(body any) (string, error) {
	if body == nil {
		return "", nil
	}
	if s, ok := body.(string); ok {
		return s, nil
	}
	bytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to serialize body: %w", err)
	}
	return string(bytes), nil
}

// ValidateResponse validates a producer's response against the registered schema.
//
// Returns a TestValidationResult with Valid=true if validation succeeds,
// or Valid=false with error details if validation fails.
func (t *ProducerTestKit) ValidateResponse(ctx context.Context, params ValidateResponseParams) (*TestValidationResult, error) {
	// Serialize response body
	responseBody, err := serializeBody(params.Response.Body)
	if err != nil {
		return nil, err
	}

	// Build gRPC request
	req := &pb.ValidateProducerRequest{
		SchemaId:      t.schemaID,
		SchemaVersion: t.schemaVersion,
		Method:        strings.ToUpper(params.Method),
		Path:          params.Path,
		Response: &pb.ResponseData{
			StatusCode: int32(params.Response.StatusCode),
			Headers:    params.Response.Headers,
			Body:       responseBody,
		},
	}

	// Add optional request context if provided
	if params.Request != nil {
		requestBody, err := serializeBody(params.Request.Body)
		if err != nil {
			return nil, err
		}
		req.Request = &pb.RequestData{
			Method:  params.Request.Method,
			Path:    params.Request.Path,
			Headers: params.Request.Headers,
			Body:    requestBody,
		}
	}

	// Add API key to context
	ctx = t.contextWithAPIKey(ctx)

	// Call server
	result, err := t.client.ValidateProducerResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("validation request failed: %w", err)
	}

	return &TestValidationResult{
		Valid:                   result.Valid,
		Errors:                  result.Errors,
		ValidatedAgainstVersion: result.ValidatedAgainstVersion,
		ValidatedAgainstHash:    result.ValidatedAgainstHash,
	}, nil
}

// ValidateInteraction validates a full interaction (request + response) against the schema.
//
// This is useful when you want to validate both the request and response
// together, ensuring the complete contract is honored.
func (t *ProducerTestKit) ValidateInteraction(ctx context.Context, request TestRequestContext, response TestResponseData) (*TestValidationResult, error) {
	return t.ValidateResponse(ctx, ValidateResponseParams{
		Method:   request.Method,
		Path:     request.Path,
		Response: response,
		Request:  &request,
	})
}

// EndpointTester provides a simplified interface for testing a specific endpoint.
type EndpointTester struct {
	testKit     *ProducerTestKit
	method      string
	pathPattern string
}

// ForEndpoint creates a helper for testing a specific endpoint.
//
// This is useful when testing multiple scenarios for the same endpoint.
//
// Example:
//
//	getUserEndpoint := testKit.ForEndpoint("GET", "/users/{id}")
//
//	// Test valid response
//	result, err := getUserEndpoint.ValidateResponse(ctx, producer.TestResponseData{
//	    StatusCode: 200,
//	    Body:       map[string]any{"id": "123", "name": "John"},
//	}, map[string]string{"id": "123"})
//
//	// Test not found
//	result, err := getUserEndpoint.ValidateResponse(ctx, producer.TestResponseData{
//	    StatusCode: 404,
//	    Body:       map[string]any{"error": "User not found"},
//	}, map[string]string{"id": "999"})
func (t *ProducerTestKit) ForEndpoint(method, pathPattern string) *EndpointTester {
	return &EndpointTester{
		testKit:     t,
		method:      method,
		pathPattern: pathPattern,
	}
}

// ValidateResponse validates a response for this endpoint.
//
// The pathValues map is used to substitute path parameters in the pattern.
// For example, if pathPattern is "/users/{id}" and pathValues is {"id": "123"},
// the actual path "/users/123" will be used for validation.
func (e *EndpointTester) ValidateResponse(ctx context.Context, response TestResponseData, pathValues map[string]string) (*TestValidationResult, error) {
	actualPath := e.pathPattern

	// Substitute path parameters
	for key, value := range pathValues {
		actualPath = strings.ReplaceAll(actualPath, "{"+key+"}", value)
	}

	return e.testKit.ValidateResponse(ctx, ValidateResponseParams{
		Method:   e.method,
		Path:     actualPath,
		Response: response,
	})
}

// Close closes the gRPC connection.
// Should be called when the test kit is no longer needed.
func (t *ProducerTestKit) Close() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}
