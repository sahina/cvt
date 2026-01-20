// Package main provides the validator service implementation for contract testing.
// This service validates HTTP request/response interactions against OpenAPI specifications.
package cvtservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/sahina/cvt/server/pb"
	"go.uber.org/zap"
)

// ValidatorService implements the ContractValidator gRPC service.
// It provides two main operations:
// 1. RegisterSchema - Registers an OpenAPI schema for later validation
// 2. ValidateInteraction - Validates HTTP request/response pairs against a registered schema
//
// The service maintains a cache of registered schemas for efficient validation.
type ValidatorService struct {
	pb.UnimplementedContractValidatorServer
	cache *SchemaCache
}

// NewValidatorService creates a new instance of ValidatorService with an initialized cache.
// The cache stores registered OpenAPI schemas with a 24-hour TTL and supports up to 1000 schemas.
//
// Returns:
//   - *ValidatorService: A new validator service instance
//   - error: An error if cache initialization fails
func NewValidatorService() (*ValidatorService, error) {
	cache, err := NewSchemaCache()
	if err != nil {
		return nil, fmt.Errorf("failed to create schema cache: %w", err)
	}

	return &ValidatorService{
		cache: cache,
	}, nil
}

// NewValidatorServiceWithCache creates a new ValidatorService with a shared cache.
// This allows multiple services (gRPC and REST) to share the same schema registry.
func NewValidatorServiceWithCache(cache *SchemaCache) *ValidatorService {
	return &ValidatorService{
		cache: cache,
	}
}

// GetCache returns the underlying schema cache for sharing with other services.
func (s *ValidatorService) GetCache() *SchemaCache {
	return s.cache
}

// RegisterSchema registers an OpenAPI schema for validation.
// This method performs the following operations:
// 1. Validates the request (schema ID and content)
// 2. Parses the schema content (supports both OpenAPI v2 and v3)
// 3. Validates the parsed OpenAPI document
// 4. Stores the schema in the cache for later validation
//
// Parameters:
//   - ctx: The request context
//   - req: RegisterSchemaRequest containing schemaId and schemaContent
//
// Returns:
//   - RegisterSchemaResponse: Contains success status and message
//   - error: gRPC error (always nil, errors are returned in response)
//
// The schema is automatically converted from OpenAPI v2 to v3 if needed.
func (s *ValidatorService) RegisterSchema(ctx context.Context, req *pb.RegisterSchemaRequest) (*pb.RegisterSchemaResponse, error) {
	// Record metrics for gRPC request timing
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("RegisterSchema").Observe(time.Since(start).Seconds())
	}()

	// Log the registration request for debugging and audit purposes
	Info("Received RegisterSchema request", zap.String("schemaId", req.SchemaId))

	// Validate inputs (schema ID length, content size, etc.)
	if err := s.validateRegisterSchemaRequest(req); err != nil {
		Warn("Validation error in RegisterSchema",
			zap.String("schemaId", req.SchemaId),
			zap.Error(err))
		schemasRegistered.WithLabelValues("failure").Inc()
		grpcRequestsTotal.WithLabelValues("RegisterSchema", "failure").Inc()
		return &pb.RegisterSchemaResponse{
			Success: false,
			Message: fmt.Sprintf("Validation error: %v", err),
		}, nil
	}

	// Parse and validate the OpenAPI schema
	// Automatically detects and converts OpenAPI v2 (Swagger) to v3
	doc, err := s.parseAndConvertSchema([]byte(req.SchemaContent))
	if err != nil {
		Error("Failed to parse OpenAPI schema",
			zap.String("schemaId", req.SchemaId),
			zap.Error(err))
		schemasRegistered.WithLabelValues("failure").Inc()
		schemaRegistrationErrors.WithLabelValues("parse_error").Inc()
		grpcRequestsTotal.WithLabelValues("RegisterSchema", "failure").Inc()
		return &pb.RegisterSchemaResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to parse schema: %v", err),
		}, nil
	}

	// Validate the OpenAPI document structure and integrity
	// This ensures the schema is well-formed and can be used for validation
	loader := openapi3.NewLoader()
	if err := doc.Validate(loader.Context); err != nil {
		Error("Invalid OpenAPI schema",
			zap.String("schemaId", req.SchemaId),
			zap.Error(err))
		schemasRegistered.WithLabelValues("failure").Inc()
		schemaRegistrationErrors.WithLabelValues("validation_error").Inc()
		grpcRequestsTotal.WithLabelValues("RegisterSchema", "failure").Inc()
		return &pb.RegisterSchemaResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid OpenAPI schema: %v", err),
		}, nil
	}

	// Get version from schema - this is the source of truth
	schemaInfoVersion := ""
	if doc.Info != nil {
		schemaInfoVersion = doc.Info.Version
	}

	// Note: OpenAPI validation already ensures info.version is present,
	// but we check defensively
	if schemaInfoVersion == "" {
		Error("Schema missing info.version",
			zap.String("schemaId", req.SchemaId))
		schemasRegistered.WithLabelValues("failure").Inc()
		schemaRegistrationErrors.WithLabelValues("missing_version").Inc()
		grpcRequestsTotal.WithLabelValues("RegisterSchema", "failure").Inc()
		return &pb.RegisterSchemaResponse{
			Success: false,
			Message: "Schema must have info.version defined",
		}, nil
	}

	// If SchemaVersion provided, validate it matches (optional confirmation)
	if req.SchemaVersion != "" && req.SchemaVersion != schemaInfoVersion {
		Warn("Schema version mismatch",
			zap.String("schemaId", req.SchemaId),
			zap.String("providedVersion", req.SchemaVersion),
			zap.String("schemaInfoVersion", schemaInfoVersion))
		schemasRegistered.WithLabelValues("failure").Inc()
		schemaRegistrationErrors.WithLabelValues("version_mismatch").Inc()
		grpcRequestsTotal.WithLabelValues("RegisterSchema", "failure").Inc()
		return &pb.RegisterSchemaResponse{
			Success: false,
			Message: fmt.Sprintf("Version mismatch: provided version '%s' does not match schema info.version '%s'", req.SchemaVersion, schemaInfoVersion),
		}, nil
	}

	// Always use schema's info.version as the version
	version := schemaInfoVersion

	// Create schema entry with metadata
	entry := NewSchemaEntry(
		req.SchemaId,
		req.SchemaContent,
		doc,
		version,
		req.Ownership,
	)

	// Store the validated schema in cache with 24-hour TTL
	s.cache.Set(req.SchemaId, entry)

	Info("Schema registered successfully",
		zap.String("schemaId", req.SchemaId),
		zap.String("version", version),
		zap.String("hash", entry.Metadata.SchemaHash))

	// Record successful schema registration
	schemasRegistered.WithLabelValues("success").Inc()
	grpcRequestsTotal.WithLabelValues("RegisterSchema", "success").Inc()

	return &pb.RegisterSchemaResponse{
		Success:  true,
		Message:  "Schema registered successfully",
		Metadata: entry.Metadata,
	}, nil
}

// ValidateInteraction validates an HTTP request/response interaction against a registered schema.
// This method performs the following operations:
// 1. Validates the interaction request (schema ID, method, path, status code)
// 2. Retrieves the registered schema from cache
// 3. Converts the request data to an http.Request object
// 4. Finds the matching route in the OpenAPI schema
// 5. Validates the request against the schema (path params, query params, headers, body)
// 6. Validates the response against the schema (status code, headers, body)
//
// Parameters:
//   - ctx: The request context
//   - req: InteractionRequest containing schemaId, request data, and response data
//
// Returns:
//   - ValidationResult: Contains validation status and any error messages
//   - error: gRPC error (always nil, errors are returned in response)
//
// The validation checks that the HTTP interaction matches the contract defined in the OpenAPI schema.
func (s *ValidatorService) ValidateInteraction(ctx context.Context, req *pb.InteractionRequest) (*pb.ValidationResult, error) {
	// Record metrics for validation timing
	start := time.Now()
	method := req.GetRequest().GetMethod()
	schemaID := req.GetSchemaId()

	defer func() {
		validationDuration.WithLabelValues(schemaID, method).Observe(time.Since(start).Seconds())
		grpcRequestDuration.WithLabelValues("ValidateInteraction").Observe(time.Since(start).Seconds())
	}()

	// Log the validation request for debugging purposes
	Debug("Received ValidateInteraction request",
		zap.String("schemaId", schemaID),
		zap.String("method", method),
		zap.String("path", req.Request.Path))

	// Validate inputs (schema ID, HTTP method, path, status code, etc.)
	if err := s.validateInteractionRequest(req); err != nil {
		Warn("Validation error in ValidateInteraction",
			zap.String("schemaId", schemaID),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("input_validation").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateInteraction", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Validation error: %v", err)},
		}, nil
	}

	// Retrieve the previously registered schema from cache
	// Support version-specific retrieval
	var entry *SchemaEntry
	var found bool
	if req.SchemaVersion != "" {
		entry, found = s.cache.GetVersion(schemaID, req.SchemaVersion)
	} else {
		entry, found = s.cache.Get(schemaID)
	}
	if !found || entry == nil {
		Warn("Schema not found in cache", zap.String("schemaId", schemaID))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("schema_not_found").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateInteraction", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Schema not found: %s", schemaID)},
		}, nil
	}
	doc := entry.Document

	// Handle basePath from Swagger 2.0 schemas BEFORE creating HTTP request
	// OpenAPI 3 converts basePath to server URLs with path components
	// Strip the basePath from incoming request path if present
	requestPath := req.Request.Path
	if len(doc.Servers) > 0 {
		for _, server := range doc.Servers {
			// Parse the server URL to extract path component (basePath)
			if serverURL, err := url.Parse(server.URL); err == nil && serverURL.Path != "" && serverURL.Path != "/" {
				basePath := strings.TrimSuffix(serverURL.Path, "/")
				// Strip basePath from the incoming request path
				if strings.HasPrefix(requestPath, basePath+"/") {
					originalPath := requestPath
					requestPath = strings.TrimPrefix(requestPath, basePath)
					Info("Stripped basePath from request path",
						zap.String("basePath", basePath),
						zap.String("originalPath", originalPath),
						zap.String("strippedPath", requestPath))
					break
				}
			}
		}
	}

	// Create request data with potentially modified path
	modifiedReqData := &pb.RequestData{
		Method:  req.Request.Method,
		Path:    requestPath,
		Headers: req.Request.Headers,
		Body:    req.Request.Body,
	}

	// Get the base URL from the first server (if available) to create proper HTTP request
	// This helps the gorillamux router match against server URLs
	var baseURL string
	if len(doc.Servers) > 0 {
		// Use first server URL as base
		if serverURL, err := url.Parse(doc.Servers[0].URL); err == nil {
			baseURL = fmt.Sprintf("%s://%s", serverURL.Scheme, serverURL.Host)
			Info("Using server URL as base for HTTP request", zap.String("baseURL", baseURL))
		}
	}
	if baseURL == "" {
		baseURL = "http://localhost"
	}

	// Create an http.Request object from the request data for validation
	httpReq, err := s.createHTTPRequestWithBase(modifiedReqData, baseURL)
	if err != nil {
		Error("Failed to create HTTP request", zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("http_request_creation").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateInteraction", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to create HTTP request: %v", err)},
		}, nil
	}

	// Save original server URLs and create copies without path components for routing
	// When Swagger 2.0 is converted to OpenAPI 3, basePath becomes part of server URLs
	// But gorillamux router needs server URLs without path components to match routes
	var originalServers openapi3.Servers
	if len(doc.Servers) > 0 {
		serverURLs := make([]string, 0)
		for _, server := range doc.Servers {
			serverURLs = append(serverURLs, server.URL)
		}
		Info("Schema has server URLs (before stripping paths)", zap.Strings("serverURLs", serverURLs))

		// Save original servers to restore later
		originalServers = make(openapi3.Servers, len(doc.Servers))
		copy(originalServers, doc.Servers)

		// Create new server entries without path components
		modifiedServers := make(openapi3.Servers, 0, len(doc.Servers))
		for _, server := range doc.Servers {
			if serverURL, err := url.Parse(server.URL); err == nil {
				if serverURL.Path != "" && serverURL.Path != "/" {
					// Remove path component, keep only scheme + host + port
					originalURL := server.URL
					serverURL.Path = ""
					serverURL.RawPath = ""
					newServer := &openapi3.Server{
						URL:         serverURL.String(),
						Description: server.Description,
						Variables:   server.Variables,
					}
					modifiedServers = append(modifiedServers, newServer)
					Info("Stripped path from server URL",
						zap.String("original", originalURL),
						zap.String("modified", newServer.URL))
				} else {
					modifiedServers = append(modifiedServers, server)
				}
			} else {
				modifiedServers = append(modifiedServers, server)
			}
		}
		doc.Servers = modifiedServers
	} else {
		Info("Schema has no server URLs defined")
	}

	// Create a router to match the request to the appropriate OpenAPI operation
	router, err := gorillamux.NewRouter(doc)

	// Restore original server URLs to avoid affecting subsequent requests
	if originalServers != nil {
		doc.Servers = originalServers
	}
	if err != nil {
		Error("Failed to create router",
			zap.String("schemaId", schemaID),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("router_creation").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateInteraction", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to create router: %v", err)},
		}, nil
	}

	// Log available paths and operations for debugging
	if doc.Paths != nil {
		pathList := make([]string, 0)
		for path, pathItem := range doc.Paths.Map() {
			pathList = append(pathList, path)
			// Check if the requested path matches
			if path == httpReq.URL.Path {
				operations := make([]string, 0)
				if pathItem.Get != nil {
					operations = append(operations, "GET")
				}
				if pathItem.Post != nil {
					operations = append(operations, "POST")
				}
				if pathItem.Put != nil {
					operations = append(operations, "PUT")
				}
				if pathItem.Delete != nil {
					operations = append(operations, "DELETE")
				}
				Info("Found matching path in schema",
					zap.String("path", path),
					zap.Strings("availableOperations", operations))
			}
		}
		Info("Attempting to match request against schema paths",
			zap.String("requestMethod", method),
			zap.String("requestPath", httpReq.URL.Path),
			zap.Strings("availablePaths", pathList))
	}

	// Find the matching route in the OpenAPI schema based on method and path
	route, pathParams, err := router.FindRoute(httpReq)
	if err != nil {
		Error("Route not found",
			zap.String("method", method),
			zap.String("path", req.Request.Path),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "invalid").Inc()
		validationErrors.WithLabelValues("route_not_found").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateInteraction", "success").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Route not found: %v", err)},
		}, nil
	}

	// Validate the request against the OpenAPI schema
	// This checks path parameters, query parameters, headers, and request body
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    httpReq,
		PathParams: pathParams,
		Route:      route,
	}

	if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
		Debug("Request validation failed",
			zap.String("method", method),
			zap.String("path", req.Request.Path),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "invalid").Inc()
		validationErrors.WithLabelValues("request_invalid").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateInteraction", "success").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{err.Error()},
		}, nil
	}

	// Validate the response against the OpenAPI schema
	// This checks the status code, response headers, and response body
	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 int(req.Response.StatusCode),
		Header:                 s.createHTTPHeaders(req.Response.Headers),
		Body:                   io.NopCloser(strings.NewReader(req.Response.Body)),
	}

	if err := openapi3filter.ValidateResponse(ctx, responseValidationInput); err != nil {
		Debug("Response validation failed",
			zap.Int32("statusCode", req.Response.StatusCode),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "invalid").Inc()
		validationErrors.WithLabelValues("response_invalid").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateInteraction", "success").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{err.Error()},
		}, nil
	}

	Info("Interaction validated successfully",
		zap.String("schemaId", schemaID),
		zap.String("method", method),
		zap.String("path", req.Request.Path))

	// Record successful validation
	validationsTotal.WithLabelValues(schemaID, method, "valid").Inc()
	grpcRequestsTotal.WithLabelValues("ValidateInteraction", "success").Inc()

	// Include version and hash in result
	result := &pb.ValidationResult{
		Valid:  true,
		Errors: nil,
	}
	if entry.Metadata != nil {
		result.ValidatedAgainstVersion = entry.Metadata.SchemaVersion
		result.ValidatedAgainstHash = entry.Metadata.SchemaHash
	}

	return result, nil
}

// validateRegisterSchemaRequest validates the RegisterSchemaRequest.
// It checks that the request is not nil and validates the schema ID and content
// according to the validation rules defined in validation_utils.go.
func (s *ValidatorService) validateRegisterSchemaRequest(req *pb.RegisterSchemaRequest) error {
	if req == nil {
		return fmt.Errorf("RegisterSchemaRequest cannot be null")
	}
	if err := ValidateSchemaID(req.SchemaId); err != nil {
		return err
	}
	if err := ValidateSchemaContent(req.SchemaContent); err != nil {
		return err
	}
	return nil
}

// validateInteractionRequest validates the InteractionRequest.
// It checks that the request is not nil and validates all request components:
// - Schema ID must be valid (not empty, max 255 chars)
// - Request data must be present with valid HTTP method and path
// - Response data must be present with valid status code
func (s *ValidatorService) validateInteractionRequest(req *pb.InteractionRequest) error {
	if req == nil {
		return fmt.Errorf("InteractionRequest cannot be null")
	}
	if err := ValidateSchemaID(req.SchemaId); err != nil {
		return err
	}
	if req.Request == nil {
		return fmt.Errorf("RequestData cannot be null")
	}
	if err := ValidateHTTPMethod(req.Request.Method); err != nil {
		return err
	}
	if err := ValidateHTTPPath(req.Request.Path); err != nil {
		return err
	}
	if req.Response == nil {
		return fmt.Errorf("ResponseData cannot be null")
	}
	if err := ValidateStatusCode(req.Response.StatusCode); err != nil {
		return err
	}
	return nil
}

// createHTTPRequestWithBase creates an *http.Request from RequestData with a specified base URL.
// This converts the protobuf RequestData into a standard Go http.Request
// that can be validated against the OpenAPI schema.
func (s *ValidatorService) createHTTPRequestWithBase(reqData *pb.RequestData, baseURL string) (*http.Request, error) {
	var body io.Reader
	if reqData.Body != "" {
		body = strings.NewReader(reqData.Body)
	}

	// Create URL for validation using the provided base URL
	url := fmt.Sprintf("%s%s", baseURL, reqData.Path)

	httpReq, err := http.NewRequest(reqData.Method, url, body)
	if err != nil {
		return nil, err
	}

	Info("Created HTTP request for validation",
		zap.String("method", reqData.Method),
		zap.String("path", reqData.Path),
		zap.String("baseURL", baseURL),
		zap.String("url", httpReq.URL.String()),
		zap.String("urlPath", httpReq.URL.Path))

	// Add headers from the request data
	for key, value := range reqData.Headers {
		httpReq.Header.Set(key, value)
	}

	return httpReq, nil
}

// createHTTPHeaders creates http.Header from a map[string]string.
// This is used to convert response headers from the protobuf format
// to the standard Go http.Header format for validation.
func (s *ValidatorService) createHTTPHeaders(headers map[string]string) http.Header {
	httpHeaders := make(http.Header)
	for key, value := range headers {
		httpHeaders.Set(key, value)
	}
	return httpHeaders
}

// Close cleans up resources used by the ValidatorService.
// This should be called when the service is being shut down to
// properly close the schema cache and release any resources.
func (s *ValidatorService) Close() {
	if s.cache != nil {
		s.cache.Close()
	}
}

// parseAndConvertSchema parses OpenAPI v2 or v3 schema and converts v2 to v3 if needed.
// This method:
// 1. Detects the schema version by checking for "swagger" or "openapi" fields
// 2. If it's OpenAPI v2 (Swagger 2.0), parses and converts it to OpenAPI v3
// 3. If it's OpenAPI v3, parses it directly
// 4. Returns an error if the schema format is not supported
//
// Parameters:
//   - data: The raw schema content as bytes (JSON format)
//
// Returns:
//   - *openapi3.T: The parsed OpenAPI v3 document
//   - error: An error if parsing or conversion fails
//
// Supported formats: OpenAPI v2 (Swagger 2.0) and OpenAPI v3.x
func (s *ValidatorService) parseAndConvertSchema(data []byte) (*openapi3.T, error) {
	// Try to detect the schema version by checking for "swagger" or "openapi" field
	var rawSchema map[string]any
	if err := json.Unmarshal(data, &rawSchema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	// Check if it's OpenAPI v2 (Swagger)
	if swagger, ok := rawSchema["swagger"].(string); ok && strings.HasPrefix(swagger, "2.") {
		Info("Detected OpenAPI v2 (Swagger) schema, converting to OpenAPI v3")

		// Parse as OpenAPI v2
		var v2Doc openapi2.T
		if err := json.Unmarshal(data, &v2Doc); err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI v2 schema: %w", err)
		}

		// Convert to OpenAPI v3 for unified validation
		v3Doc, err := openapi2conv.ToV3(&v2Doc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert OpenAPI v2 to v3: %w", err)
		}

		Info("Successfully converted OpenAPI v2 to v3")
		return v3Doc, nil
	}

	// Check if it's OpenAPI v3
	if openapi, ok := rawSchema["openapi"].(string); ok && strings.HasPrefix(openapi, "3.") {
		Info("Detected OpenAPI v3 schema")

		// Parse as OpenAPI v3
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromData(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI v3 schema: %w", err)
		}

		return doc, nil
	}

	return nil, fmt.Errorf("unsupported schema format: must be OpenAPI v2 (Swagger 2.0) or OpenAPI v3.x")
}

// GetSchema retrieves metadata and content for a registered schema.
func (s *ValidatorService) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	// Record metrics
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("GetSchema").Observe(time.Since(start).Seconds())
	}()

	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("GetSchema", "failure").Inc()
		return &pb.GetSchemaResponse{Found: false}, nil
	}

	var entry *SchemaEntry
	var found bool

	if req.SchemaVersion != "" {
		entry, found = s.cache.GetVersion(req.SchemaId, req.SchemaVersion)
	} else {
		entry, found = s.cache.Get(req.SchemaId)
	}

	if !found || entry == nil {
		grpcRequestsTotal.WithLabelValues("GetSchema", "success").Inc()
		return &pb.GetSchemaResponse{Found: false}, nil
	}

	grpcRequestsTotal.WithLabelValues("GetSchema", "success").Inc()
	return &pb.GetSchemaResponse{
		Found:         true,
		Metadata:      entry.Metadata,
		SchemaContent: entry.Content,
	}, nil
}

// ListSchemas returns a list of all registered schemas.
func (s *ValidatorService) ListSchemas(ctx context.Context, req *pb.ListSchemasRequest) (*pb.ListSchemasResponse, error) {
	// Record metrics
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("ListSchemas").Observe(time.Since(start).Seconds())
	}()

	// Get all schema IDs
	schemaIDs := s.cache.ListSchemaIDs()

	// Apply pagination
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 100 // Default page size
	}

	// Collect schema metadata
	var schemas []*pb.SchemaMetadata
	for _, schemaID := range schemaIDs {
		entry, found := s.cache.Get(schemaID)
		if !found || entry == nil || entry.Metadata == nil {
			continue
		}

		// Apply filters
		if req.Owner != "" && (entry.Metadata.Ownership == nil || entry.Metadata.Ownership.Owner != req.Owner) {
			continue
		}
		if req.Team != "" && (entry.Metadata.Ownership == nil || entry.Metadata.Ownership.Team != req.Team) {
			continue
		}

		schemas = append(schemas, entry.Metadata)
	}

	totalCount := len(schemas)

	// Apply pagination
	if len(schemas) > pageSize {
		schemas = schemas[:pageSize]
	}

	grpcRequestsTotal.WithLabelValues("ListSchemas", "success").Inc()
	return &pb.ListSchemasResponse{
		Schemas:    schemas,
		TotalCount: int32(totalCount),
	}, nil
}

// CompareSchemas compares two versions of a schema for breaking changes.
// Uses the CompatibilityEngine to detect breaking changes between schema versions.
func (s *ValidatorService) CompareSchemas(ctx context.Context, req *pb.CompareSchemasRequest) (*pb.CompareSchemasResponse, error) {
	// Record metrics
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("CompareSchemas").Observe(time.Since(start).Seconds())
	}()

	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("CompareSchemas", "failure").Inc()
		return &pb.CompareSchemasResponse{
			Compatible: true, // No comparison possible without schema ID
		}, nil
	}

	Info("Comparing schemas",
		zap.String("schemaId", req.SchemaId),
		zap.String("oldVersion", req.OldVersion),
		zap.String("newVersion", req.NewVersion))

	// Get old version
	oldVersion := req.OldVersion
	if oldVersion == "" {
		// Get previous version from latest
		entry, found := s.cache.Get(req.SchemaId)
		if found && entry != nil && entry.Metadata != nil {
			oldVersion, _ = s.cache.GetPreviousVersion(req.SchemaId, entry.Metadata.SchemaVersion)
		}
	}

	// Get new version
	newVersion := req.NewVersion
	if newVersion == "" {
		entry, found := s.cache.Get(req.SchemaId)
		if found && entry != nil && entry.Metadata != nil {
			newVersion = entry.Metadata.SchemaVersion
		}
	}

	// Get both schema entries
	var oldEntry, newEntry *SchemaEntry
	var oldFound, newFound bool

	if oldVersion != "" {
		oldEntry, oldFound = s.cache.GetVersion(req.SchemaId, oldVersion)
	}
	if newVersion != "" {
		newEntry, newFound = s.cache.GetVersion(req.SchemaId, newVersion)
	}

	if !oldFound || !newFound || oldEntry == nil || newEntry == nil {
		Info("Schema versions not found for comparison",
			zap.String("schemaId", req.SchemaId),
			zap.Bool("oldFound", oldFound),
			zap.Bool("newFound", newFound))
		grpcRequestsTotal.WithLabelValues("CompareSchemas", "success").Inc()
		return &pb.CompareSchemasResponse{
			Compatible: true, // Cannot compare if versions not found
		}, nil
	}

	// Quick check: if hashes are identical, schemas are compatible
	if oldEntry.Metadata.SchemaHash == newEntry.Metadata.SchemaHash {
		Info("Schemas have identical hashes, skipping detailed comparison",
			zap.String("schemaId", req.SchemaId),
			zap.String("hash", oldEntry.Metadata.SchemaHash))
		grpcRequestsTotal.WithLabelValues("CompareSchemas", "success").Inc()
		return &pb.CompareSchemasResponse{
			Compatible:      true,
			BreakingChanges: nil,
			OldSchema:       oldEntry.Metadata,
			NewSchema:       newEntry.Metadata,
		}, nil
	}

	// Use compatibility engine to detect breaking changes
	engine := NewCompatibilityEngine()
	breakingChanges, compatible := engine.CompareSchemas(oldEntry.Document, newEntry.Document)

	Info("Schema comparison complete",
		zap.String("schemaId", req.SchemaId),
		zap.String("oldVersion", oldVersion),
		zap.String("newVersion", newVersion),
		zap.Bool("compatible", compatible),
		zap.Int("breakingChanges", len(breakingChanges)))

	// Record metrics for breaking changes
	for _, change := range breakingChanges {
		breakingChangesDetected.WithLabelValues(change.Type.String()).Inc()
	}

	grpcRequestsTotal.WithLabelValues("CompareSchemas", "success").Inc()
	return &pb.CompareSchemasResponse{
		Compatible:      compatible,
		BreakingChanges: breakingChanges,
		OldSchema:       oldEntry.Metadata,
		NewSchema:       newEntry.Metadata,
	}, nil
}

// GenerateFixture generates test fixtures from an OpenAPI schema.
// This method creates request/response pairs based on the schema definition,
// useful for testing APIs without making actual HTTP calls.
func (s *ValidatorService) GenerateFixture(ctx context.Context, req *pb.GenerateFixtureRequest) (*pb.GenerateFixtureResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("GenerateFixture").Observe(time.Since(start).Seconds())
	}()

	Info("Received GenerateFixture request",
		zap.String("schemaId", req.SchemaId),
		zap.String("method", req.Method),
		zap.String("path", req.Path))

	// Validate inputs
	if err := ValidateSchemaID(req.SchemaId); err != nil {
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
		return &pb.GenerateFixtureResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid schema ID: %v", err),
		}, nil
	}

	if req.Method == "" || req.Path == "" {
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
		return &pb.GenerateFixtureResponse{
			Success: false,
			Message: "Method and path are required",
		}, nil
	}

	// Get schema from cache
	entry, found := s.cache.Get(req.SchemaId)
	if !found || entry == nil {
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
		return &pb.GenerateFixtureResponse{
			Success: false,
			Message: fmt.Sprintf("Schema not found: %s", req.SchemaId),
		}, nil
	}

	doc := entry.Document
	method := strings.ToUpper(req.Method)
	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	// Find the operation
	operation, err := s.findOperation(doc, method, req.Path)
	if err != nil {
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
		return &pb.GenerateFixtureResponse{
			Success: false,
			Message: fmt.Sprintf("Operation not found: %v", err),
		}, nil
	}

	// Generate based on output type
	switch req.OutputType {
	case pb.OutputType_OUTPUT_REQUEST:
		body, err := s.generateRequestBody(doc, operation, req.UseExamples, contentType)
		if err != nil {
			grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
			return &pb.GenerateFixtureResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate request body: %v", err),
			}, nil
		}
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "success").Inc()
		return &pb.GenerateFixtureResponse{
			Success:     true,
			Message:     "Request body generated successfully",
			RequestBody: body,
		}, nil

	case pb.OutputType_OUTPUT_RESPONSE:
		resp, err := s.generateResponse(doc, operation, int(req.StatusCode), req.UseExamples, contentType)
		if err != nil {
			grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
			return &pb.GenerateFixtureResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate response: %v", err),
			}, nil
		}
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "success").Inc()
		return &pb.GenerateFixtureResponse{
			Success:  true,
			Message:  "Response generated successfully",
			Response: resp,
		}, nil

	default: // OUTPUT_FIXTURE
		// Generate request
		reqBody, _ := s.generateRequestBody(doc, operation, req.UseExamples, contentType)

		// Generate response
		resp, err := s.generateResponse(doc, operation, int(req.StatusCode), req.UseExamples, contentType)
		if err != nil {
			grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
			return &pb.GenerateFixtureResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate response: %v", err),
			}, nil
		}

		// Resolve path parameters
		resolvedPath := req.Path
		for _, paramRef := range operation.Parameters {
			if paramRef.Value != nil && paramRef.Value.In == "path" {
				paramName := paramRef.Value.Name
				placeholder := "{" + paramName + "}"
				if strings.Contains(resolvedPath, placeholder) {
					var paramValue string
					if paramRef.Value.Schema != nil {
						val := s.generateValue(doc, paramRef.Value.Schema, req.UseExamples, 0)
						paramValue = fmt.Sprintf("%v", val)
					} else {
						paramValue = "value"
					}
					resolvedPath = strings.Replace(resolvedPath, placeholder, paramValue, 1)
				}
			}
		}

		fixture := &pb.GeneratedFixture{
			Request: &pb.GeneratedRequest{
				Method:  method,
				Path:    resolvedPath,
				Headers: make(map[string]string),
			},
			Response: resp,
		}

		if reqBody != "" {
			fixture.Request.Body = reqBody
			fixture.Request.Headers["Content-Type"] = contentType
		}

		grpcRequestsTotal.WithLabelValues("GenerateFixture", "success").Inc()
		return &pb.GenerateFixtureResponse{
			Success: true,
			Message: "Fixture generated successfully",
			Fixture: fixture,
		}, nil
	}
}

// ListEndpoints returns all endpoints available in a registered schema.
func (s *ValidatorService) ListEndpoints(ctx context.Context, req *pb.ListEndpointsRequest) (*pb.ListEndpointsResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("ListEndpoints").Observe(time.Since(start).Seconds())
	}()

	if err := ValidateSchemaID(req.SchemaId); err != nil {
		grpcRequestsTotal.WithLabelValues("ListEndpoints", "failure").Inc()
		return &pb.ListEndpointsResponse{Endpoints: nil}, nil
	}

	entry, found := s.cache.Get(req.SchemaId)
	if !found || entry == nil {
		grpcRequestsTotal.WithLabelValues("ListEndpoints", "success").Inc()
		return &pb.ListEndpointsResponse{Endpoints: nil}, nil
	}

	doc := entry.Document
	var endpoints []*pb.EndpointInfo

	if doc.Paths != nil {
		for path, pathItem := range doc.Paths.Map() {
			for method, op := range pathItem.Operations() {
				endpoint := &pb.EndpointInfo{
					Method: method,
					Path:   path,
				}
				if op.OperationID != "" {
					endpoint.OperationId = op.OperationID
				}
				if op.Summary != "" {
					endpoint.Summary = op.Summary
				}
				endpoints = append(endpoints, endpoint)
			}
		}
	}

	grpcRequestsTotal.WithLabelValues("ListEndpoints", "success").Inc()
	return &pb.ListEndpointsResponse{Endpoints: endpoints}, nil
}

// findOperation finds the OpenAPI operation for a given method and path.
func (s *ValidatorService) findOperation(doc *openapi3.T, method, path string) (*openapi3.Operation, error) {
	// First try exact match
	pathItem := doc.Paths.Find(path)
	if pathItem != nil {
		operation := pathItem.GetOperation(method)
		if operation != nil {
			return operation, nil
		}
	}

	// Try matching with path parameters using router
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	req, err := http.NewRequest(method, "http://localhost"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	route, _, err := router.FindRoute(req)
	if err != nil {
		return nil, fmt.Errorf("route not found: %s %s", method, path)
	}

	return route.Operation, nil
}

// generateRequestBody generates a request body from the schema.
func (s *ValidatorService) generateRequestBody(doc *openapi3.T, operation *openapi3.Operation, useExamples bool, contentType string) (string, error) {
	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return "", nil // No request body defined
	}

	mediaType := operation.RequestBody.Value.Content.Get(contentType)
	if mediaType == nil {
		// Try to find any JSON content type
		for ct, mt := range operation.RequestBody.Value.Content {
			if strings.Contains(ct, "json") {
				mediaType = mt
				break
			}
		}
	}

	if mediaType == nil || mediaType.Schema == nil {
		return "", nil
	}

	value := s.generateValue(doc, mediaType.Schema, useExamples, 0)
	jsonData, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// generateResponse generates a response from the schema.
func (s *ValidatorService) generateResponse(doc *openapi3.T, operation *openapi3.Operation, statusCode int, useExamples bool, contentType string) (*pb.GeneratedResponse, error) {
	if operation.Responses == nil {
		return nil, fmt.Errorf("no responses defined")
	}

	// Determine status code
	if statusCode == 0 {
		statusCode = s.selectSuccessStatus(operation)
	}

	response := operation.Responses.Status(statusCode)
	if response == nil {
		// Try default
		response = operation.Responses.Default()
	}
	if response == nil {
		return nil, fmt.Errorf("no response found for status %d", statusCode)
	}

	result := &pb.GeneratedResponse{
		StatusCode: int32(statusCode),
		Headers:    make(map[string]string),
	}

	if response.Value != nil && response.Value.Content != nil {
		mediaType := response.Value.Content.Get(contentType)
		if mediaType == nil {
			for ct, mt := range response.Value.Content {
				if strings.Contains(ct, "json") {
					mediaType = mt
					contentType = ct
					break
				}
			}
		}

		if mediaType != nil {
			result.Headers["Content-Type"] = contentType
			if mediaType.Schema != nil {
				value := s.generateValue(doc, mediaType.Schema, useExamples, 0)
				jsonData, err := json.MarshalIndent(value, "", "  ")
				if err != nil {
					return nil, err
				}
				result.Body = string(jsonData)
			}
		}
	}

	return result, nil
}

// selectSuccessStatus selects the first successful status code from responses.
func (s *ValidatorService) selectSuccessStatus(operation *openapi3.Operation) int {
	if operation.Responses == nil {
		return 200
	}

	// Prefer specific success codes in order
	preferredCodes := []int{200, 201, 202, 204}
	for _, code := range preferredCodes {
		if operation.Responses.Status(code) != nil {
			return code
		}
	}

	// Check for any 2XX status
	for code := range operation.Responses.Map() {
		if strings.HasPrefix(code, "2") {
			var statusCode int
			_, _ = fmt.Sscanf(code, "%d", &statusCode)
			if statusCode > 0 {
				return statusCode
			}
		}
	}

	return 200
}

// generateValue generates a value based on a schema.
func (s *ValidatorService) generateValue(doc *openapi3.T, schemaRef *openapi3.SchemaRef, useExamples bool, depth int) any {
	if schemaRef == nil || depth > 10 {
		return nil
	}

	schema := schemaRef.Value
	if schema == nil {
		return nil
	}

	// Check for example first if useExamples is true
	if useExamples && schema.Example != nil {
		return schema.Example
	}

	// Handle allOf, oneOf, anyOf
	if len(schema.AllOf) > 0 {
		return s.generateAllOf(doc, schema.AllOf, useExamples, depth)
	}
	if len(schema.OneOf) > 0 {
		return s.generateValue(doc, schema.OneOf[0], useExamples, depth+1)
	}
	if len(schema.AnyOf) > 0 {
		return s.generateValue(doc, schema.AnyOf[0], useExamples, depth+1)
	}

	// Handle by type
	types := schema.Type.Slice()
	if len(types) == 0 {
		// Check if properties exist (implicit object)
		if len(schema.Properties) > 0 {
			return s.generateObject(doc, schema, useExamples, depth)
		}
		return nil
	}

	switch types[0] {
	case "object":
		return s.generateObject(doc, schema, useExamples, depth)
	case "array":
		return s.generateArray(doc, schema, useExamples, depth)
	case "string":
		return s.generateString(schema)
	case "integer":
		return s.generateInteger(schema)
	case "number":
		return s.generateNumber(schema)
	case "boolean":
		return s.generateBoolean()
	default:
		return nil
	}
}

func (s *ValidatorService) generateObject(doc *openapi3.T, schema *openapi3.Schema, useExamples bool, depth int) map[string]any {
	result := make(map[string]any)
	for propName, propRef := range schema.Properties {
		result[propName] = s.generateValue(doc, propRef, useExamples, depth+1)
	}
	return result
}

func (s *ValidatorService) generateArray(doc *openapi3.T, schema *openapi3.Schema, useExamples bool, depth int) []any {
	if schema.Items == nil {
		return []any{}
	}
	count := 1
	if schema.MinItems > 0 && uint64(count) < schema.MinItems {
		count = int(schema.MinItems)
	}
	result := make([]any, count)
	for i := 0; i < count; i++ {
		result[i] = s.generateValue(doc, schema.Items, useExamples, depth+1)
	}
	return result
}

func (s *ValidatorService) generateString(schema *openapi3.Schema) string {
	if len(schema.Enum) > 0 {
		return fmt.Sprintf("%v", schema.Enum[0])
	}
	switch schema.Format {
	case "date":
		return "2024-01-15"
	case "date-time":
		return "2024-01-15T10:30:00Z"
	case "email":
		return "user@example.com"
	case "uri", "url":
		return "https://example.com"
	case "uuid":
		return "550e8400-e29b-41d4-a716-446655440000"
	}
	return "string"
}

func (s *ValidatorService) generateInteger(schema *openapi3.Schema) int64 {
	if len(schema.Enum) > 0 {
		if val, ok := schema.Enum[0].(float64); ok {
			return int64(val)
		}
	}
	if schema.Min != nil {
		return int64(*schema.Min)
	}
	return 123
}

func (s *ValidatorService) generateNumber(schema *openapi3.Schema) float64 {
	if len(schema.Enum) > 0 {
		if val, ok := schema.Enum[0].(float64); ok {
			return val
		}
	}
	if schema.Min != nil {
		return *schema.Min
	}
	return 123.45
}

func (s *ValidatorService) generateBoolean() bool {
	return true
}

func (s *ValidatorService) generateAllOf(doc *openapi3.T, allOf openapi3.SchemaRefs, useExamples bool, depth int) map[string]any {
	result := make(map[string]any)
	for _, schemaRef := range allOf {
		val := s.generateValue(doc, schemaRef, useExamples, depth+1)
		if obj, ok := val.(map[string]any); ok {
			maps.Copy(result, obj)
		}
	}
	return result
}

// ============================================================================
// Phase 1: Producer Testing
// ============================================================================

// ValidateProducerResponse validates a producer's HTTP response against their OpenAPI schema.
// This is used by producers to write contract tests that verify their handlers return
// spec-compliant responses - without needing actual consumers to call their API.
//
// Unlike ValidateInteraction which validates both request and response (for consumer testing),
// this method focuses on response validation only. Producers use this in their test suites
// to ensure their implementation matches their contract.
//
// Parameters:
//   - ctx: The request context
//   - req: ValidateProducerRequest containing schemaId, method, path, and response data
//
// Returns:
//   - ValidationResult: Contains validation status and any error messages
//   - error: gRPC error (always nil, errors are returned in response)
func (s *ValidatorService) ValidateProducerResponse(ctx context.Context, req *pb.ValidateProducerRequest) (*pb.ValidationResult, error) {
	// Handle nil request early to avoid panic
	if req == nil {
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{"ValidateProducerRequest cannot be null"},
		}, nil
	}

	// Record metrics for validation timing
	start := time.Now()
	schemaID := req.GetSchemaId()
	method := req.GetMethod()

	defer func() {
		validationDuration.WithLabelValues(schemaID, method).Observe(time.Since(start).Seconds())
		grpcRequestDuration.WithLabelValues("ValidateProducerResponse").Observe(time.Since(start).Seconds())
	}()

	Debug("Received ValidateProducerResponse request",
		zap.String("schemaId", schemaID),
		zap.String("method", method),
		zap.String("path", req.Path))

	// Validate inputs
	if err := s.validateProducerRequest(req); err != nil {
		Warn("Validation error in ValidateProducerResponse",
			zap.String("schemaId", schemaID),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("input_validation").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Validation error: %v", err)},
		}, nil
	}

	// Retrieve the schema from cache (support version-specific retrieval)
	var entry *SchemaEntry
	var found bool
	if req.SchemaVersion != "" {
		entry, found = s.cache.GetVersion(schemaID, req.SchemaVersion)
	} else {
		entry, found = s.cache.Get(schemaID)
	}
	if !found || entry == nil {
		Warn("Schema not found in cache", zap.String("schemaId", schemaID))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("schema_not_found").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Schema not found: %s", schemaID)},
		}, nil
	}
	doc := entry.Document

	// Handle basePath from Swagger 2.0 schemas
	requestPath := req.Path
	if len(doc.Servers) > 0 {
		for _, server := range doc.Servers {
			if serverURL, err := url.Parse(server.URL); err == nil && serverURL.Path != "" && serverURL.Path != "/" {
				basePath := strings.TrimSuffix(serverURL.Path, "/")
				if strings.HasPrefix(requestPath, basePath+"/") {
					requestPath = strings.TrimPrefix(requestPath, basePath)
					break
				}
			}
		}
	}

	// Create a minimal request for route matching
	var baseURL string
	if len(doc.Servers) > 0 {
		if serverURL, err := url.Parse(doc.Servers[0].URL); err == nil {
			baseURL = fmt.Sprintf("%s://%s", serverURL.Scheme, serverURL.Host)
		}
	}
	if baseURL == "" {
		baseURL = "http://localhost"
	}

	// Create http.Request for route matching (even though we're only validating response)
	httpReq, err := http.NewRequest(strings.ToUpper(method), fmt.Sprintf("%s%s", baseURL, requestPath), nil)
	if err != nil {
		Error("Failed to create HTTP request for route matching", zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("http_request_creation").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to create HTTP request: %v", err)},
		}, nil
	}

	// Add request headers if provided (for context)
	if req.Request != nil && req.Request.Headers != nil {
		for key, value := range req.Request.Headers {
			httpReq.Header.Set(key, value)
		}
	}

	// Save and modify server URLs for routing (same as ValidateInteraction)
	var originalServers openapi3.Servers
	if len(doc.Servers) > 0 {
		originalServers = make(openapi3.Servers, len(doc.Servers))
		copy(originalServers, doc.Servers)

		modifiedServers := make(openapi3.Servers, 0, len(doc.Servers))
		for _, server := range doc.Servers {
			if serverURL, err := url.Parse(server.URL); err == nil {
				if serverURL.Path != "" && serverURL.Path != "/" {
					serverURL.Path = ""
					serverURL.RawPath = ""
					newServer := &openapi3.Server{
						URL:         serverURL.String(),
						Description: server.Description,
						Variables:   server.Variables,
					}
					modifiedServers = append(modifiedServers, newServer)
				} else {
					modifiedServers = append(modifiedServers, server)
				}
			} else {
				modifiedServers = append(modifiedServers, server)
			}
		}
		doc.Servers = modifiedServers
	}

	// Create router to find the matching operation
	router, err := gorillamux.NewRouter(doc)

	// Restore original server URLs
	if originalServers != nil {
		doc.Servers = originalServers
	}
	if err != nil {
		Error("Failed to create router",
			zap.String("schemaId", schemaID),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("router_creation").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to create router: %v", err)},
		}, nil
	}

	// Find the matching route
	route, pathParams, err := router.FindRoute(httpReq)
	if err != nil {
		Error("Route not found",
			zap.String("method", method),
			zap.String("path", req.Path),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "invalid").Inc()
		validationErrors.WithLabelValues("route_not_found").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "success").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Route not found: %s %s - %v", method, req.Path, err)},
		}, nil
	}

	// Create request validation input (needed for response validation context)
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    httpReq,
		PathParams: pathParams,
		Route:      route,
	}

	// Validate the response against the OpenAPI schema
	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 int(req.Response.StatusCode),
		Header:                 s.createHTTPHeaders(req.Response.Headers),
		Body:                   io.NopCloser(strings.NewReader(req.Response.Body)),
	}

	if err := openapi3filter.ValidateResponse(ctx, responseValidationInput); err != nil {
		Debug("Response validation failed",
			zap.Int32("statusCode", req.Response.StatusCode),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "invalid").Inc()
		validationErrors.WithLabelValues("response_invalid").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "success").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{err.Error()},
		}, nil
	}

	Info("Producer response validated successfully",
		zap.String("schemaId", schemaID),
		zap.String("method", method),
		zap.String("path", req.Path),
		zap.Int32("statusCode", req.Response.StatusCode))

	// Record successful validation
	validationsTotal.WithLabelValues(schemaID, method, "valid").Inc()
	grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "success").Inc()

	// Include version and hash in result
	result := &pb.ValidationResult{
		Valid:  true,
		Errors: nil,
	}
	if entry.Metadata != nil {
		result.ValidatedAgainstVersion = entry.Metadata.SchemaVersion
		result.ValidatedAgainstHash = entry.Metadata.SchemaHash
	}

	return result, nil
}

// validateProducerRequest validates the ValidateProducerRequest.
func (s *ValidatorService) validateProducerRequest(req *pb.ValidateProducerRequest) error {
	if req == nil {
		return fmt.Errorf("ValidateProducerRequest cannot be null")
	}
	if err := ValidateSchemaID(req.SchemaId); err != nil {
		return err
	}
	if err := ValidateHTTPMethod(req.Method); err != nil {
		return err
	}
	if err := ValidateHTTPPath(req.Path); err != nil {
		return err
	}
	if req.Response == nil {
		return fmt.Errorf("ResponseData cannot be null")
	}
	if err := ValidateStatusCode(req.Response.StatusCode); err != nil {
		return err
	}
	return nil
}

// ============================================================================
// Consumer Registry RPCs
// ============================================================================

// RegisterConsumer registers a consumer's dependency on a schema.
// This allows tracking which consumers depend on which schemas, enabling
// deployment safety checks via CanIDeploy.
func (s *ValidatorService) RegisterConsumer(ctx context.Context, req *pb.RegisterConsumerRequest) (*pb.RegisterConsumerResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("RegisterConsumer").Observe(time.Since(start).Seconds())
	}()

	Info("Received RegisterConsumer request",
		zap.String("consumerId", req.ConsumerId),
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", req.Environment))

	// Validate request
	if req.ConsumerId == "" {
		grpcRequestsTotal.WithLabelValues("RegisterConsumer", "failure").Inc()
		return &pb.RegisterConsumerResponse{
			Success: false,
			Message: "consumer_id is required",
		}, nil
	}
	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("RegisterConsumer", "failure").Inc()
		return &pb.RegisterConsumerResponse{
			Success: false,
			Message: "schema_id is required",
		}, nil
	}
	if req.Environment == "" {
		req.Environment = "dev" // Default to dev
	}

	// Verify the schema exists
	_, found := s.cache.Get(req.SchemaId)
	if !found {
		grpcRequestsTotal.WithLabelValues("RegisterConsumer", "failure").Inc()
		return &pb.RegisterConsumerResponse{
			Success: false,
			Message: fmt.Sprintf("schema not found: %s", req.SchemaId),
		}, nil
	}

	// Convert endpoint usage from proto to storage format
	usedEndpoints := make([]EndpointUsage, len(req.UsedEndpoints))
	for i, eu := range req.UsedEndpoints {
		usedEndpoints[i] = EndpointUsage{
			Method:     eu.Method,
			Path:       eu.Path,
			UsedFields: eu.UsedFields,
		}
	}

	now := time.Now()

	// Register in cache
	consumer := &ConsumerEntry{
		ConsumerID:      req.ConsumerId,
		ConsumerVersion: req.ConsumerVersion,
		SchemaID:        req.SchemaId,
		SchemaVersion:   req.SchemaVersion,
		Environment:     req.Environment,
		RegisteredAt:    now,
		LastValidatedAt: now,
		UsedEndpoints:   usedEndpoints,
	}
	s.cache.RegisterConsumer(consumer)

	Info("Consumer registered successfully",
		zap.String("consumerId", req.ConsumerId),
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", req.Environment))

	grpcRequestsTotal.WithLabelValues("RegisterConsumer", "success").Inc()

	// Convert endpoint usage to proto format for response
	protoEndpoints := make([]*pb.EndpointUsage, len(usedEndpoints))
	for i, eu := range usedEndpoints {
		protoEndpoints[i] = &pb.EndpointUsage{
			Method:     eu.Method,
			Path:       eu.Path,
			UsedFields: eu.UsedFields,
		}
	}

	return &pb.RegisterConsumerResponse{
		Success: true,
		Message: "Consumer registered successfully",
		Consumer: &pb.ConsumerInfo{
			ConsumerId:      req.ConsumerId,
			ConsumerVersion: req.ConsumerVersion,
			SchemaId:        req.SchemaId,
			SchemaVersion:   req.SchemaVersion,
			Environment:     req.Environment,
			RegisteredAt:    now.Unix(),
			LastValidatedAt: now.Unix(),
			UsedEndpoints:   protoEndpoints,
		},
	}, nil
}

// ListConsumers returns all consumers that depend on a schema.
func (s *ValidatorService) ListConsumers(ctx context.Context, req *pb.ListConsumersRequest) (*pb.ListConsumersResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("ListConsumers").Observe(time.Since(start).Seconds())
	}()

	Debug("Received ListConsumers request",
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", req.Environment))

	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("ListConsumers", "failure").Inc()
		return &pb.ListConsumersResponse{}, nil
	}

	consumers := s.cache.ListConsumers(req.SchemaId, req.Environment)

	protoConsumers := make([]*pb.ConsumerInfo, len(consumers))
	for i, c := range consumers {
		protoEndpoints := make([]*pb.EndpointUsage, len(c.UsedEndpoints))
		for j, eu := range c.UsedEndpoints {
			protoEndpoints[j] = &pb.EndpointUsage{
				Method:     eu.Method,
				Path:       eu.Path,
				UsedFields: eu.UsedFields,
			}
		}
		protoConsumers[i] = &pb.ConsumerInfo{
			ConsumerId:      c.ConsumerID,
			ConsumerVersion: c.ConsumerVersion,
			SchemaId:        c.SchemaID,
			SchemaVersion:   c.SchemaVersion,
			Environment:     c.Environment,
			RegisteredAt:    c.RegisteredAt.Unix(),
			LastValidatedAt: c.LastValidatedAt.Unix(),
			UsedEndpoints:   protoEndpoints,
		}
	}

	grpcRequestsTotal.WithLabelValues("ListConsumers", "success").Inc()

	return &pb.ListConsumersResponse{
		Consumers: protoConsumers,
	}, nil
}

// DeregisterConsumer removes a consumer registration.
func (s *ValidatorService) DeregisterConsumer(ctx context.Context, req *pb.DeregisterConsumerRequest) (*pb.DeregisterConsumerResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("DeregisterConsumer").Observe(time.Since(start).Seconds())
	}()

	Info("Received DeregisterConsumer request",
		zap.String("consumerId", req.ConsumerId),
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", req.Environment))

	if req.ConsumerId == "" {
		grpcRequestsTotal.WithLabelValues("DeregisterConsumer", "failure").Inc()
		return &pb.DeregisterConsumerResponse{
			Success: false,
			Message: "consumer_id is required",
		}, nil
	}

	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("DeregisterConsumer", "failure").Inc()
		return &pb.DeregisterConsumerResponse{
			Success: false,
			Message: "schema_id is required",
		}, nil
	}

	environment := req.Environment
	if environment == "" {
		environment = "dev"
	}

	removed := s.cache.DeregisterConsumer(req.ConsumerId, req.SchemaId, environment)
	if !removed {
		grpcRequestsTotal.WithLabelValues("DeregisterConsumer", "failure").Inc()
		return &pb.DeregisterConsumerResponse{
			Success: false,
			Message: fmt.Sprintf("consumer not found: %s/%s/%s", req.ConsumerId, req.SchemaId, environment),
		}, nil
	}

	Info("Consumer deregistered successfully",
		zap.String("consumerId", req.ConsumerId),
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", environment))

	grpcRequestsTotal.WithLabelValues("DeregisterConsumer", "success").Inc()

	return &pb.DeregisterConsumerResponse{
		Success: true,
		Message: "Consumer deregistered successfully",
	}, nil
}

// CanIDeploy checks if a schema version can be safely deployed.
// It checks for breaking changes and analyzes impact on registered consumers.
func (s *ValidatorService) CanIDeploy(ctx context.Context, req *pb.CanIDeployRequest) (*pb.CanIDeployResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("CanIDeploy").Observe(time.Since(start).Seconds())
	}()

	Info("Received CanIDeploy request",
		zap.String("schemaId", req.SchemaId),
		zap.String("newVersion", req.NewVersion),
		zap.String("environment", req.Environment))

	// Validate request
	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("CanIDeploy", "failure").Inc()
		return &pb.CanIDeployResponse{
			SafeToDeploy: false,
			Summary:      "schema_id is required",
		}, nil
	}

	environment := req.Environment
	if environment == "" {
		environment = "prod" // Default to prod for deployment safety
	}

	// Get the new schema version
	_, found := s.cache.Get(req.SchemaId)
	if !found {
		grpcRequestsTotal.WithLabelValues("CanIDeploy", "failure").Inc()
		return &pb.CanIDeployResponse{
			SafeToDeploy: false,
			Summary:      fmt.Sprintf("schema not found: %s", req.SchemaId),
		}, nil
	}

	// Get the new schema version entry for comparison
	newEntry, newFound := s.cache.GetVersion(req.SchemaId, req.NewVersion)
	if !newFound || newEntry == nil {
		// Try to get latest if specific version not found
		newEntry, newFound = s.cache.Get(req.SchemaId)
		if !newFound || newEntry == nil {
			grpcRequestsTotal.WithLabelValues("CanIDeploy", "failure").Inc()
			return &pb.CanIDeployResponse{
				SafeToDeploy: false,
				Summary:      fmt.Sprintf("schema version not found: %s@%s", req.SchemaId, req.NewVersion),
			}, nil
		}
	}

	// Get all consumers in the target environment
	consumers := s.cache.ListConsumers(req.SchemaId, environment)

	// If no consumers, it's safe to deploy
	if len(consumers) == 0 {
		grpcRequestsTotal.WithLabelValues("CanIDeploy", "success").Inc()
		return &pb.CanIDeployResponse{
			SafeToDeploy: true,
			Summary:      fmt.Sprintf("No consumers registered for %s in %s environment", req.SchemaId, environment),
		}, nil
	}

	// Use CompatibilityEngine to detect actual breaking changes
	engine := NewCompatibilityEngine()
	var allBreakingChanges []*pb.BreakingChange
	var affectedConsumers []*pb.ConsumerImpact
	allSafe := true

	for _, consumer := range consumers {
		// If consumer is on the same version or not version-pinned, no need to compare
		if consumer.SchemaVersion == "" || consumer.SchemaVersion == req.NewVersion {
			affectedConsumers = append(affectedConsumers, &pb.ConsumerImpact{
				ConsumerId:           consumer.ConsumerID,
				ConsumerVersion:      consumer.ConsumerVersion,
				CurrentSchemaVersion: consumer.SchemaVersion,
				Environment:          consumer.Environment,
				WillBreak:            false,
			})
			continue
		}

		// Get the consumer's current schema version for comparison
		oldEntry, oldFound := s.cache.GetVersion(req.SchemaId, consumer.SchemaVersion)
		if !oldFound || oldEntry == nil {
			// Can't compare if old version not found, mark as potentially affected
			Info("Cannot find consumer's schema version for comparison",
				zap.String("consumerId", consumer.ConsumerID),
				zap.String("schemaVersion", consumer.SchemaVersion))

			affectedConsumers = append(affectedConsumers, &pb.ConsumerImpact{
				ConsumerId:           consumer.ConsumerID,
				ConsumerVersion:      consumer.ConsumerVersion,
				CurrentSchemaVersion: consumer.SchemaVersion,
				Environment:          consumer.Environment,
				WillBreak:            true, // Conservative: assume breaking if can't compare
			})
			allSafe = false
			continue
		}

		// Compare schemas to detect breaking changes
		changes, _ := engine.CompareSchemas(oldEntry.Document, newEntry.Document)

		// Filter breaking changes to only those affecting this consumer's used endpoints
		relevantChanges := filterChangesForConsumer(changes, consumer.UsedEndpoints)

		willBreak := len(relevantChanges) > 0
		if willBreak {
			allSafe = false
			allBreakingChanges = append(allBreakingChanges, relevantChanges...)
		}

		affectedConsumers = append(affectedConsumers, &pb.ConsumerImpact{
			ConsumerId:           consumer.ConsumerID,
			ConsumerVersion:      consumer.ConsumerVersion,
			CurrentSchemaVersion: consumer.SchemaVersion,
			Environment:          consumer.Environment,
			WillBreak:            willBreak,
			RelevantChanges:      relevantChanges,
		})
	}

	var summary string
	if allSafe {
		summary = fmt.Sprintf("Safe to deploy %s version %s to %s - %d consumer(s) verified",
			req.SchemaId, req.NewVersion, environment, len(consumers))
	} else {
		breakingCount := 0
		for _, c := range affectedConsumers {
			if c.WillBreak {
				breakingCount++
			}
		}
		summary = fmt.Sprintf("Deployment of %s version %s to %s will break %d of %d consumer(s) - review required",
			req.SchemaId, req.NewVersion, environment, breakingCount, len(affectedConsumers))
	}

	grpcRequestsTotal.WithLabelValues("CanIDeploy", "success").Inc()

	return &pb.CanIDeployResponse{
		SafeToDeploy:      allSafe,
		Summary:           summary,
		BreakingChanges:   allBreakingChanges,
		AffectedConsumers: affectedConsumers,
	}, nil
}

// filterChangesForConsumer filters breaking changes to only those affecting the consumer's used endpoints.
func filterChangesForConsumer(changes []*pb.BreakingChange, endpoints []EndpointUsage) []*pb.BreakingChange {
	if len(endpoints) == 0 {
		// If no endpoints specified, all changes are relevant (conservative)
		return changes
	}

	var relevant []*pb.BreakingChange
	for _, change := range changes {
		for _, ep := range endpoints {
			// Match by path (and optionally method)
			pathMatches := change.Path == ep.Path || change.Path == "" // Empty path means all paths affected
			methodMatches := change.Method == "" || change.Method == ep.Method

			if pathMatches && methodMatches {
				relevant = append(relevant, change)
				break
			}
		}
	}
	return relevant
}
