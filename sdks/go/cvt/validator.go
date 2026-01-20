// Package cvt provides a Go SDK for the Contract Validator Toolkit (CVT).
//
// The CVT SDK allows you to validate HTTP interactions against OpenAPI schemas
// via a gRPC service.
package cvt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	pb "github.com/sahina/cvt/sdks/go/cvt/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// ValidationRequest represents an HTTP request for validation.
type ValidationRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    any
}

// ValidationResponse represents an HTTP response for validation.
type ValidationResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       any
}

// ValidationResult represents the result of a validation operation.
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// BreakingChange represents a breaking change detected between schema versions.
type BreakingChange struct {
	// Type is the type of breaking change (e.g., ENDPOINT_REMOVED, REQUIRED_FIELD_ADDED)
	Type string
	// Path is the API path affected (e.g., "/pet/{petId}")
	Path string
	// Method is the HTTP method affected (e.g., "DELETE")
	Method string
	// Description is a human-readable description of the breaking change
	Description string
	// OldValue is the previous value (for context)
	OldValue string
	// NewValue is the new value (for context)
	NewValue string
}

// CompareResult represents the result of comparing two schema versions.
type CompareResult struct {
	// Compatible is true if no breaking changes were detected
	Compatible bool
	// BreakingChanges is the list of breaking changes detected
	BreakingChanges []BreakingChange
}

// GenerateOptions configures fixture generation.
type GenerateOptions struct {
	// StatusCode is the HTTP status code to generate (0 = auto-select first successful).
	StatusCode int
	// UseExamples indicates whether to use schema examples when available.
	UseExamples bool
	// ContentType is the content type for generation (default: "application/json").
	ContentType string
}

// GeneratedRequest represents a generated HTTP request fixture.
type GeneratedRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    any
}

// GeneratedResponse represents a generated HTTP response fixture.
type GeneratedResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       any
}

// GeneratedFixture represents a complete generated request/response pair.
type GeneratedFixture struct {
	Request  *GeneratedRequest
	Response *GeneratedResponse
}

// EndpointInfo represents information about an API endpoint.
type EndpointInfo struct {
	Method      string
	Path        string
	OperationID string
	Summary     string
}

// ============================================================================
// Consumer Registry Types
// ============================================================================

// EndpointUsage describes which endpoints and fields a consumer uses.
type EndpointUsage struct {
	// Method is the HTTP method (GET, POST, etc.)
	Method string
	// Path is the API path (e.g., "/users/{id}")
	Path string
	// UsedFields are the fields used in response (e.g., ["email", "name"])
	UsedFields []string
}

// ConsumerInfo represents information about a registered consumer.
type ConsumerInfo struct {
	// ConsumerID is the unique consumer identifier (e.g., "order-service")
	ConsumerID string
	// ConsumerVersion is the consumer's version (e.g., "2.1.0")
	ConsumerVersion string
	// SchemaID is the schema this consumer depends on
	SchemaID string
	// SchemaVersion is the schema version consumer was tested against
	SchemaVersion string
	// Environment is the deployment environment (dev, staging, prod)
	Environment string
	// RegisteredAt is the Unix timestamp of registration
	RegisteredAt int64
	// LastValidatedAt is the Unix timestamp of last successful validation
	LastValidatedAt int64
	// UsedEndpoints are the endpoints the consumer uses
	UsedEndpoints []EndpointUsage
}

// RegisterConsumerOptions configures consumer registration.
type RegisterConsumerOptions struct {
	// ConsumerID is the unique consumer identifier (e.g., "order-service")
	ConsumerID string
	// ConsumerVersion is the consumer's version (e.g., "2.1.0")
	ConsumerVersion string
	// SchemaID is the schema this consumer depends on
	SchemaID string
	// SchemaVersion is the schema version consumer was tested against
	SchemaVersion string
	// Environment is the deployment environment (dev, staging, prod)
	Environment string
	// UsedEndpoints are the endpoints the consumer uses
	UsedEndpoints []EndpointUsage
}

// ConsumerImpact represents the impact of schema changes on a specific consumer.
type ConsumerImpact struct {
	// ConsumerID is the consumer identifier
	ConsumerID string
	// ConsumerVersion is the consumer version
	ConsumerVersion string
	// CurrentSchemaVersion is the schema version consumer was tested against
	CurrentSchemaVersion string
	// Environment is the deployment environment
	Environment string
	// WillBreak is true if the consumer will be affected
	WillBreak bool
	// RelevantChanges are the breaking changes that affect this consumer
	RelevantChanges []BreakingChange
}

// CanIDeployResult represents the result of a deployment safety check.
type CanIDeployResult struct {
	// SafeToDeploy is true if safe to deploy
	SafeToDeploy bool
	// Summary is a human-readable summary
	Summary string
	// BreakingChanges are all breaking changes in the new version
	BreakingChanges []BreakingChange
	// AffectedConsumers are the consumers affected by the changes
	AffectedConsumers []ConsumerImpact
}

// TLSOptions configures TLS for secure connections.
type TLSOptions struct {
	// Enabled indicates whether to use TLS.
	Enabled bool

	// RootCertPath is the path to the root CA certificate for server verification.
	RootCertPath string

	// CertPath is the path to the client certificate (for mTLS).
	CertPath string

	// KeyPath is the path to the client private key (for mTLS).
	KeyPath string
}

// ValidatorOptions configures the Validator client.
type ValidatorOptions struct {
	// Address is the gRPC server address (default: "localhost:9550").
	Address string

	// TLS configures TLS for secure connections.
	TLS *TLSOptions

	// APIKey is the API key for authentication.
	APIKey string
}

// Validator is a client for validating HTTP interactions against OpenAPI schemas.
type Validator struct {
	conn     *grpc.ClientConn
	client   pb.ContractValidatorClient
	schemaID string
	apiKey   string
}

// NewValidator creates a new Validator instance with an insecure connection.
//
// The address parameter specifies the gRPC server address.
// If empty, defaults to "localhost:9550".
//
// For TLS and API key authentication, use NewValidatorWithOptions instead.
//
// Example:
//
//	validator, err := cvt.NewValidator("")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer validator.Close()
func NewValidator(address string) (*Validator, error) {
	return NewValidatorWithOptions(ValidatorOptions{Address: address})
}

// NewValidatorWithOptions creates a new Validator instance with the specified options.
//
// Example with TLS and API key:
//
//	validator, err := cvt.NewValidatorWithOptions(cvt.ValidatorOptions{
//	    Address: "localhost:9550",
//	    TLS: &cvt.TLSOptions{
//	        Enabled:      true,
//	        RootCertPath: "./certs/ca.crt",
//	    },
//	    APIKey: "cvt-dev-key-12345",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer validator.Close()
func NewValidatorWithOptions(opts ValidatorOptions) (*Validator, error) {
	address := opts.Address
	if address == "" {
		address = "localhost:9550"
	}

	// Build dial options
	var dialOpts []grpc.DialOption

	if opts.TLS != nil && opts.TLS.Enabled {
		creds, err := CreateTLSCredentials(opts.TLS)
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

	// Create validator client
	client := pb.NewContractValidatorClient(conn)

	return &Validator{
		conn:   conn,
		client: client,
		apiKey: opts.APIKey,
	}, nil
}

// CreateTLSCredentials creates TLS transport credentials from the options.
// This is exported for use by subpackages like producer.
func CreateTLSCredentials(opts *TLSOptions) (credentials.TransportCredentials, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load root CA if specified
	if opts.RootCertPath != "" {
		caCert, err := os.ReadFile(opts.RootCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = certPool
	}

	// Load client certificate if specified (for mTLS)
	if opts.CertPath != "" && opts.KeyPath != "" {
		clientCert, err := tls.LoadX509KeyPair(opts.CertPath, opts.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return credentials.NewTLS(tlsConfig), nil
}

// contextWithAPIKey adds the API key to the context if configured.
func (v *Validator) contextWithAPIKey(ctx context.Context) context.Context {
	if v.apiKey == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "x-api-key", v.apiKey)
}

// RegisterSchema registers an OpenAPI schema for validation.
//
// The schemaID is a unique identifier for the schema.
// The schemaPath can be either a local file path or an HTTP/HTTPS URL.
//
// Example:
//
//	err := validator.RegisterSchema(ctx, "my-api", "./openapi.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (v *Validator) RegisterSchema(ctx context.Context, schemaID, schemaPath string) error {
	// Read schema content from file or URL
	var schemaContent string
	var err error

	if len(schemaPath) >= 8 && (schemaPath[:7] == "http://" || schemaPath[:8] == "https://") {
		schemaContent, err = fetchSchemaFromURL(ctx, schemaPath)
	} else {
		schemaContent, err = readSchemaFromFile(schemaPath)
	}

	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}

	// Register schema with server
	req := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: schemaContent,
	}

	// Add API key to context if configured
	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.RegisterSchema(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register schema: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("schema registration failed: %s", resp.Message)
	}

	v.schemaID = schemaID
	return nil
}

// Validate validates an HTTP interaction against the registered schema.
//
// The request and response parameters should be ValidationRequest and ValidationResponse
// structs respectively.
//
// Returns a ValidationResult with Valid=true if validation succeeds,
// or Valid=false with error details if validation fails.
//
// Example:
//
//	result, err := validator.Validate(ctx, ValidationRequest{
//	    Method: "GET",
//	    Path:   "/users",
//	}, ValidationResponse{
//	    StatusCode: 200,
//	    Body:       []User{{ID: 1, Name: "Alice"}},
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if !result.Valid {
//	    log.Printf("Validation failed: %v", result.Errors)
//	}
func (v *Validator) Validate(ctx context.Context, request ValidationRequest, response ValidationResponse) (*ValidationResult, error) {
	if v.schemaID == "" {
		return nil, fmt.Errorf("schema not registered; call RegisterSchema first")
	}

	// Build request data
	requestBody := ""
	if request.Body != nil {
		bodyBytes, err := json.Marshal(request.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		requestBody = string(bodyBytes)
	}

	requestData := &pb.RequestData{
		Method:  request.Method,
		Path:    request.Path,
		Headers: request.Headers,
		Body:    requestBody,
	}

	// Build response data
	responseBody := ""
	if response.Body != nil {
		bodyBytes, err := json.Marshal(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response body: %w", err)
		}
		responseBody = string(bodyBytes)
	}

	responseData := &pb.ResponseData{
		StatusCode: int32(response.StatusCode),
		Headers:    response.Headers,
		Body:       responseBody,
	}

	// Validate interaction
	interactionReq := &pb.InteractionRequest{
		SchemaId: v.schemaID,
		Request:  requestData,
		Response: responseData,
	}

	// Add API key to context if configured
	ctx = v.contextWithAPIKey(ctx)

	result, err := v.client.ValidateInteraction(ctx, interactionReq)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &ValidationResult{
		Valid:  result.Valid,
		Errors: result.Errors,
	}, nil
}

// RegisterSchemaWithVersion registers an OpenAPI schema with version information.
//
// The schemaID is a unique identifier for the schema.
// The schemaPath can be either a local file path or an HTTP/HTTPS URL.
// The version is the semantic version of the schema (e.g., "1.0.0").
//
// Example:
//
//	err := validator.RegisterSchemaWithVersion(ctx, "my-api", "./openapi.yaml", "1.0.0")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (v *Validator) RegisterSchemaWithVersion(ctx context.Context, schemaID, schemaPath, version string) error {
	// Read schema content from file or URL
	var schemaContent string
	var err error

	if len(schemaPath) >= 8 && (schemaPath[:7] == "http://" || schemaPath[:8] == "https://") {
		schemaContent, err = fetchSchemaFromURL(ctx, schemaPath)
	} else {
		schemaContent, err = readSchemaFromFile(schemaPath)
	}

	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}

	// Register schema with server
	req := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: schemaContent,
		SchemaVersion: version,
	}

	// Add API key to context if configured
	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.RegisterSchema(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register schema: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("schema registration failed: %s", resp.Message)
	}

	v.schemaID = schemaID
	return nil
}

// CompareSchemas compares two schema versions to detect breaking changes.
//
// The schemaID is the identifier for the schema to compare.
// The oldVersion and newVersion parameters specify which versions to compare.
// If either is empty, the server will use sensible defaults (previous and latest).
//
// Example:
//
//	result, err := validator.CompareSchemas(ctx, "my-api", "1.0.0", "2.0.0")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if !result.Compatible {
//	    fmt.Println("Breaking changes detected:")
//	    for _, change := range result.BreakingChanges {
//	        fmt.Printf("- %s: %s\n", change.Type, change.Description)
//	    }
//	}
func (v *Validator) CompareSchemas(ctx context.Context, schemaID, oldVersion, newVersion string) (*CompareResult, error) {
	req := &pb.CompareSchemasRequest{
		SchemaId:   schemaID,
		OldVersion: oldVersion,
		NewVersion: newVersion,
	}

	// Add API key to context if configured
	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.CompareSchemas(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to compare schemas: %w", err)
	}

	// Convert protobuf breaking changes to SDK type
	breakingChanges := make([]BreakingChange, 0, len(resp.BreakingChanges))
	for _, bc := range resp.BreakingChanges {
		breakingChanges = append(breakingChanges, BreakingChange{
			Type:        bc.Type.String(),
			Path:        bc.Path,
			Method:      bc.Method,
			Description: bc.Description,
			OldValue:    bc.OldValue,
			NewValue:    bc.NewValue,
		})
	}

	return &CompareResult{
		Compatible:      resp.Compatible,
		BreakingChanges: breakingChanges,
	}, nil
}

// GenerateFixture generates a complete test fixture (request and response) for an endpoint.
//
// The method parameter should be an HTTP method (GET, POST, etc.).
// The path parameter is the API path (e.g., /users/{id}).
// If opts is nil, sensible defaults are used (UseExamples=true, ContentType="application/json").
//
// Returns a GeneratedFixture with both request and response data.
// Returns an error if no schema has been registered.
//
// Example:
//
//	fixture, err := validator.GenerateFixture(ctx, "POST", "/users", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Request: %+v\n", fixture.Request)
//	fmt.Printf("Response: %+v\n", fixture.Response)
func (v *Validator) GenerateFixture(ctx context.Context, method, path string, opts *GenerateOptions) (*GeneratedFixture, error) {
	if v.schemaID == "" {
		return nil, fmt.Errorf("schema not registered; call RegisterSchema first")
	}

	// Apply defaults
	options := normalizeGenerateOptions(opts)

	req := &pb.GenerateFixtureRequest{
		SchemaId:    v.schemaID,
		Method:      strings.ToUpper(method),
		Path:        path,
		StatusCode:  int32(options.StatusCode),
		UseExamples: options.UseExamples,
		ContentType: options.ContentType,
		OutputType:  pb.OutputType_OUTPUT_FIXTURE,
	}

	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.GenerateFixture(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate fixture: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("fixture generation failed: %s", resp.Message)
	}

	return parseGeneratedFixture(resp.Fixture)
}

// GenerateResponse generates a response fixture for an endpoint.
//
// The method parameter should be an HTTP method (GET, POST, etc.).
// The path parameter is the API path (e.g., /users/{id}).
// If opts is nil, sensible defaults are used.
//
// Returns a GeneratedResponse with status code, headers, and body.
// Returns an error if no schema has been registered.
//
// Example:
//
//	response, err := validator.GenerateResponse(ctx, "GET", "/users/123", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Status: %d, Body: %v\n", response.StatusCode, response.Body)
func (v *Validator) GenerateResponse(ctx context.Context, method, path string, opts *GenerateOptions) (*GeneratedResponse, error) {
	if v.schemaID == "" {
		return nil, fmt.Errorf("schema not registered; call RegisterSchema first")
	}

	options := normalizeGenerateOptions(opts)

	req := &pb.GenerateFixtureRequest{
		SchemaId:    v.schemaID,
		Method:      strings.ToUpper(method),
		Path:        path,
		StatusCode:  int32(options.StatusCode),
		UseExamples: options.UseExamples,
		ContentType: options.ContentType,
		OutputType:  pb.OutputType_OUTPUT_RESPONSE,
	}

	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.GenerateFixture(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("response generation failed: %s", resp.Message)
	}

	return parseGeneratedResponse(resp.Response)
}

// GenerateRequestBody generates a request body fixture for an endpoint.
//
// The method parameter should be an HTTP method (GET, POST, etc.).
// The path parameter is the API path (e.g., /users).
// If opts is nil, sensible defaults are used.
//
// Returns the generated request body as a parsed JSON value.
// Returns an error if no schema has been registered.
//
// Example:
//
//	body, err := validator.GenerateRequestBody(ctx, "POST", "/users", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Request body: %v\n", body)
func (v *Validator) GenerateRequestBody(ctx context.Context, method, path string, opts *GenerateOptions) (any, error) {
	if v.schemaID == "" {
		return nil, fmt.Errorf("schema not registered; call RegisterSchema first")
	}

	options := normalizeGenerateOptions(opts)

	req := &pb.GenerateFixtureRequest{
		SchemaId:    v.schemaID,
		Method:      strings.ToUpper(method),
		Path:        path,
		StatusCode:  int32(options.StatusCode),
		UseExamples: options.UseExamples,
		ContentType: options.ContentType,
		OutputType:  pb.OutputType_OUTPUT_REQUEST,
	}

	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.GenerateFixture(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate request body: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("request body generation failed: %s", resp.Message)
	}

	// Parse JSON body
	if resp.RequestBody == "" {
		return nil, nil
	}

	var body any
	if err := json.Unmarshal([]byte(resp.RequestBody), &body); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	return body, nil
}

// ListEndpoints returns a list of all endpoints in the registered schema.
//
// Returns an error if no schema has been registered.
//
// Example:
//
//	endpoints, err := validator.ListEndpoints(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, ep := range endpoints {
//	    fmt.Printf("%s %s - %s\n", ep.Method, ep.Path, ep.Summary)
//	}
func (v *Validator) ListEndpoints(ctx context.Context) ([]EndpointInfo, error) {
	if v.schemaID == "" {
		return nil, fmt.Errorf("schema not registered; call RegisterSchema first")
	}

	req := &pb.ListEndpointsRequest{
		SchemaId: v.schemaID,
	}

	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.ListEndpoints(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list endpoints: %w", err)
	}

	endpoints := make([]EndpointInfo, 0, len(resp.Endpoints))
	for _, ep := range resp.Endpoints {
		endpoints = append(endpoints, EndpointInfo{
			Method:      ep.Method,
			Path:        ep.Path,
			OperationID: ep.OperationId,
			Summary:     ep.Summary,
		})
	}

	return endpoints, nil
}

// ============================================================================
// Consumer Registry Methods
// ============================================================================

// RegisterConsumer registers a consumer's dependency on a schema.
// This tracks which consumers use which schemas for deployment safety analysis.
//
// Example:
//
//	consumer, err := validator.RegisterConsumer(ctx, cvt.RegisterConsumerOptions{
//	    ConsumerID:      "order-service",
//	    ConsumerVersion: "2.1.0",
//	    SchemaID:        "user-api",
//	    SchemaVersion:   "1.0.0",
//	    Environment:     "prod",
//	    UsedEndpoints: []cvt.EndpointUsage{
//	        {Method: "GET", Path: "/users/{id}", UsedFields: []string{"email", "name"}},
//	    },
//	})
func (v *Validator) RegisterConsumer(ctx context.Context, opts RegisterConsumerOptions) (*ConsumerInfo, error) {
	// Convert used endpoints to proto format
	usedEndpoints := make([]*pb.EndpointUsage, 0, len(opts.UsedEndpoints))
	for _, ep := range opts.UsedEndpoints {
		usedEndpoints = append(usedEndpoints, &pb.EndpointUsage{
			Method:     ep.Method,
			Path:       ep.Path,
			UsedFields: ep.UsedFields,
		})
	}

	req := &pb.RegisterConsumerRequest{
		ConsumerId:      opts.ConsumerID,
		ConsumerVersion: opts.ConsumerVersion,
		SchemaId:        opts.SchemaID,
		SchemaVersion:   opts.SchemaVersion,
		Environment:     opts.Environment,
		UsedEndpoints:   usedEndpoints,
	}

	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.RegisterConsumer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to register consumer: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("consumer registration failed: %s", resp.Message)
	}

	if resp.Consumer == nil {
		return nil, fmt.Errorf("unexpected server response: consumer info is nil")
	}

	return mapConsumerInfo(resp.Consumer), nil
}

// ListConsumers returns all consumers that depend on a schema.
//
// The schemaID parameter is required.
// The environment parameter is optional; if empty, consumers from all environments are returned.
//
// Example:
//
//	consumers, err := validator.ListConsumers(ctx, "user-api", "prod")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, c := range consumers {
//	    fmt.Printf("%s v%s\n", c.ConsumerID, c.ConsumerVersion)
//	}
func (v *Validator) ListConsumers(ctx context.Context, schemaID, environment string) ([]ConsumerInfo, error) {
	req := &pb.ListConsumersRequest{
		SchemaId:    schemaID,
		Environment: environment,
	}

	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.ListConsumers(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list consumers: %w", err)
	}

	consumers := make([]ConsumerInfo, 0, len(resp.Consumers))
	for _, c := range resp.Consumers {
		if mapped := mapConsumerInfo(c); mapped != nil {
			consumers = append(consumers, *mapped)
		}
	}

	return consumers, nil
}

// DeregisterConsumer removes a consumer registration for a specific schema.
//
// Example:
//
//	err := validator.DeregisterConsumer(ctx, "order-service", "user-api", "prod")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (v *Validator) DeregisterConsumer(ctx context.Context, consumerID, schemaID, environment string) error {
	req := &pb.DeregisterConsumerRequest{
		ConsumerId:  consumerID,
		SchemaId:    schemaID,
		Environment: environment,
	}

	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.DeregisterConsumer(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to deregister consumer: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("consumer deregistration failed: %s", resp.Message)
	}

	return nil
}

// CanIDeploy checks if a schema version can be safely deployed without breaking consumers.
//
// Example:
//
//	result, err := validator.CanIDeploy(ctx, "user-api", "2.0.0", "prod")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if !result.SafeToDeploy {
//	    fmt.Println("Unsafe to deploy:")
//	    for _, c := range result.AffectedConsumers {
//	        fmt.Printf("- %s will break\n", c.ConsumerID)
//	    }
//	}
func (v *Validator) CanIDeploy(ctx context.Context, schemaID, newVersion, environment string) (*CanIDeployResult, error) {
	req := &pb.CanIDeployRequest{
		SchemaId:    schemaID,
		NewVersion:  newVersion,
		Environment: environment,
	}

	ctx = v.contextWithAPIKey(ctx)

	resp, err := v.client.CanIDeploy(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to check deployment safety: %w", err)
	}

	// Convert breaking changes
	breakingChanges := make([]BreakingChange, 0, len(resp.BreakingChanges))
	for _, bc := range resp.BreakingChanges {
		if bc == nil {
			continue
		}
		breakingChanges = append(breakingChanges, BreakingChange{
			Type:        bc.Type.String(),
			Path:        bc.Path,
			Method:      bc.Method,
			Description: bc.Description,
			OldValue:    bc.OldValue,
			NewValue:    bc.NewValue,
		})
	}

	// Convert affected consumers
	affectedConsumers := make([]ConsumerImpact, 0, len(resp.AffectedConsumers))
	for _, c := range resp.AffectedConsumers {
		if c == nil {
			continue
		}
		relevantChanges := make([]BreakingChange, 0, len(c.RelevantChanges))
		for _, bc := range c.RelevantChanges {
			if bc == nil {
				continue
			}
			relevantChanges = append(relevantChanges, BreakingChange{
				Type:        bc.Type.String(),
				Path:        bc.Path,
				Method:      bc.Method,
				Description: bc.Description,
				OldValue:    bc.OldValue,
				NewValue:    bc.NewValue,
			})
		}

		affectedConsumers = append(affectedConsumers, ConsumerImpact{
			ConsumerID:           c.ConsumerId,
			ConsumerVersion:      c.ConsumerVersion,
			CurrentSchemaVersion: c.CurrentSchemaVersion,
			Environment:          c.Environment,
			WillBreak:            c.WillBreak,
			RelevantChanges:      relevantChanges,
		})
	}

	return &CanIDeployResult{
		SafeToDeploy:      resp.SafeToDeploy,
		Summary:           resp.Summary,
		BreakingChanges:   breakingChanges,
		AffectedConsumers: affectedConsumers,
	}, nil
}

// mapConsumerInfo converts a protobuf ConsumerInfo to an SDK ConsumerInfo.
func mapConsumerInfo(c *pb.ConsumerInfo) *ConsumerInfo {
	if c == nil {
		return nil
	}

	usedEndpoints := make([]EndpointUsage, 0, len(c.UsedEndpoints))
	for _, ep := range c.UsedEndpoints {
		if ep == nil {
			continue
		}
		usedEndpoints = append(usedEndpoints, EndpointUsage{
			Method:     ep.Method,
			Path:       ep.Path,
			UsedFields: ep.UsedFields,
		})
	}

	return &ConsumerInfo{
		ConsumerID:      c.ConsumerId,
		ConsumerVersion: c.ConsumerVersion,
		SchemaID:        c.SchemaId,
		SchemaVersion:   c.SchemaVersion,
		Environment:     c.Environment,
		RegisteredAt:    c.RegisteredAt,
		LastValidatedAt: c.LastValidatedAt,
		UsedEndpoints:   usedEndpoints,
	}
}

// normalizeGenerateOptions applies defaults to GenerateOptions.
func normalizeGenerateOptions(opts *GenerateOptions) *GenerateOptions {
	if opts == nil {
		return &GenerateOptions{
			StatusCode:  0,
			UseExamples: true,
			ContentType: "application/json",
		}
	}

	result := *opts
	if result.ContentType == "" {
		result.ContentType = "application/json"
	}
	return &result
}

// parseGeneratedFixture converts a protobuf fixture to an SDK fixture.
func parseGeneratedFixture(pbFixture *pb.GeneratedFixture) (*GeneratedFixture, error) {
	if pbFixture == nil {
		return nil, fmt.Errorf("no fixture in response")
	}

	request, err := parseGeneratedRequest(pbFixture.Request)
	if err != nil {
		return nil, err
	}

	response, err := parseGeneratedResponse(pbFixture.Response)
	if err != nil {
		return nil, err
	}

	return &GeneratedFixture{
		Request:  request,
		Response: response,
	}, nil
}

// parseGeneratedRequest converts a protobuf request to an SDK request.
func parseGeneratedRequest(pbReq *pb.GeneratedRequest) (*GeneratedRequest, error) {
	if pbReq == nil {
		return nil, nil
	}

	var body any
	if pbReq.Body != "" {
		if err := json.Unmarshal([]byte(pbReq.Body), &body); err != nil {
			return nil, fmt.Errorf("failed to parse request body: %w", err)
		}
	}

	return &GeneratedRequest{
		Method:  pbReq.Method,
		Path:    pbReq.Path,
		Headers: pbReq.Headers,
		Body:    body,
	}, nil
}

// parseGeneratedResponse converts a protobuf response to an SDK response.
func parseGeneratedResponse(pbResp *pb.GeneratedResponse) (*GeneratedResponse, error) {
	if pbResp == nil {
		return nil, nil
	}

	var body any
	if pbResp.Body != "" {
		if err := json.Unmarshal([]byte(pbResp.Body), &body); err != nil {
			return nil, fmt.Errorf("failed to parse response body: %w", err)
		}
	}

	return &GeneratedResponse{
		StatusCode: int(pbResp.StatusCode),
		Headers:    pbResp.Headers,
		Body:       body,
	}, nil
}

// Close closes the gRPC connection.
// Should be called when the validator is no longer needed.
func (v *Validator) Close() error {
	if v.conn != nil {
		return v.conn.Close()
	}
	return nil
}

// readSchemaFromFile reads a schema from a local file.
func readSchemaFromFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

// fetchSchemaFromURL fetches a schema from an HTTP/HTTPS URL.
func fetchSchemaFromURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to fetch schema: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}
