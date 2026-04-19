// Package cvtservice provides the validator service implementation for contract testing.
// This service validates HTTP request/response interactions against OpenAPI specifications.
package cvtservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/sahina/cvt/pkg/cvt"
	"github.com/sahina/cvt/server/pb"
	"github.com/sahina/cvt/server/storage"
	"go.uber.org/zap"
)

// hooksHolder wraps the cvt.Hooks interface so it can live behind an
// atomic.Pointer (atomic.Value would panic on type changes between
// SetHooks calls because cvt.Hooks is an interface with multiple
// concrete implementations).
type hooksHolder struct{ h cvt.Hooks }

// schemaCompatLocks gives CheckCompatibility a per-schema-ID critical
// section so two concurrent RegisterSchema calls for the same schema_id
// can't race on the lookup-prior → compare → cache.Set sequence. Worst-
// case race without the lock: caller A reads prior=v1, caller B reads
// prior=v1, both write v2 + v3, both fire breaking-change hooks against
// the wrong baseline.
type schemaCompatLocks struct{ m sync.Map } // map[string]*sync.Mutex

func (l *schemaCompatLocks) lock(schemaID string) func() {
	v, _ := l.m.LoadOrStore(schemaID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// ValidatorService implements the ContractValidator gRPC service.
// It provides two main operations:
// 1. RegisterSchema - Registers an OpenAPI schema for later validation
// 2. ValidateInteraction - Validates HTTP request/response pairs against a registered schema
//
// The service maintains a cache of registered schemas for efficient validation.
type ValidatorService struct {
	pb.UnimplementedContractValidatorServer
	cache       *SchemaCache
	store       storage.Store  // Optional persistent storage (nil = cache-only)
	generator   *cvt.Validator // Embedded generator for fixture generation (delegates to pkg/cvt)
	hooks       atomic.Pointer[hooksHolder]
	compatLocks schemaCompatLocks
}

// SetHooks installs a plugin Hooks adapter on the service. Safe to call
// concurrently (atomic.Pointer); the most recent value wins. cmd/cvt/serve.go
// calls this once before Serve; future hot-reload paths can call it
// repeatedly without a data race. Tests typically leave hooks unset;
// hooksOrNoop returns NoopHooks{} in that case so call sites stay simple.
func (s *ValidatorService) SetHooks(h cvt.Hooks) {
	s.hooks.Store(&hooksHolder{h: h})
}

// hooksOrNoop returns the configured hooks adapter or a NoopHooks{} when
// none has been set. Mirrors *Validator.hooksOrNoop in pkg/cvt.
func (s *ValidatorService) hooksOrNoop() cvt.Hooks {
	if p := s.hooks.Load(); p != nil && p.h != nil {
		return p.h
	}
	return cvt.NoopHooks{}
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
		cache:     cache,
		generator: cvt.NewValidator(),
	}, nil
}

// NewValidatorServiceWithStore creates a new ValidatorService with persistent storage.
// The service uses a read-through cache: reads from cache first, falls back to storage.
// Writes go to both cache and storage (write-through).
func NewValidatorServiceWithStore(store storage.Store) (*ValidatorService, error) {
	cache, err := NewSchemaCache()
	if err != nil {
		return nil, fmt.Errorf("failed to create schema cache: %w", err)
	}
	return &ValidatorService{cache: cache, store: store, generator: cvt.NewValidator()}, nil
}

// NewValidatorServiceWithCache creates a new ValidatorService with a shared cache.
// This allows multiple services (gRPC and REST) to share the same schema registry.
func NewValidatorServiceWithCache(cache *SchemaCache) *ValidatorService {
	return &ValidatorService{
		cache:     cache,
		generator: cvt.NewValidator(),
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

	// --check-compatibility: look up the prior version (if any) and compute
	// breaking changes against the new schema BEFORE we commit. The
	// comparison must happen against the actual prior, not against the new
	// version we're about to write. Per decision 1C (eng review), a storage
	// error during prior-version lookup is fail-closed: we refuse the
	// registration so the caller knows the safety check did not run.
	//
	// Per-schema-ID lock prevents the lookup → cache.Set sequence from
	// racing across concurrent registrations of the same schema_id. Two
	// callers racing without this lock can both see prior=v1 and produce
	// hook events against the wrong baseline.
	var (
		breakingChanges []*pb.BreakingChange
		priorVersion    string
	)
	if req.CheckCompatibility {
		unlock := s.compatLocks.lock(req.SchemaId)
		defer unlock()
		prior, found, lookupErr := s.lookupPriorSchemaForCompat(ctx, req.SchemaId)
		if lookupErr != nil {
			Error("CheckCompatibility failed: storage error during prior-version lookup",
				zap.String("schemaId", req.SchemaId),
				zap.Error(lookupErr))
			schemasRegistered.WithLabelValues("failure").Inc()
			schemaRegistrationErrors.WithLabelValues("compat_check_storage_error").Inc()
			grpcRequestsTotal.WithLabelValues("RegisterSchema", "failure").Inc()
			return &pb.RegisterSchemaResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to check compatibility: storage error during prior-version lookup: %v", lookupErr),
			}, nil
		}
		if found && prior != nil && prior.Document != nil {
			if prior.Metadata != nil {
				priorVersion = prior.Metadata.SchemaVersion
			}
			engine := NewCompatibilityEngine()
			bc, _ := engine.CompareSchemas(prior.Document, doc)
			breakingChanges = bc
		}
	}

	// Create schema entry with metadata
	entry := NewSchemaEntry(
		req.SchemaId,
		req.SchemaContent,
		doc,
		version,
		req.Ownership,
	)

	// Build router for the schema (cached for per-request reuse)
	router, routerErr := s.buildRouter(doc)
	if routerErr != nil {
		Error("Failed to build router for schema",
			zap.String("schemaId", req.SchemaId),
			zap.Error(routerErr))
		schemasRegistered.WithLabelValues("failure").Inc()
		grpcRequestsTotal.WithLabelValues("RegisterSchema", "failure").Inc()
		return &pb.RegisterSchemaResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to build router: %v", routerErr),
		}, nil
	}
	entry.Router = router

	// Store the validated schema in cache with 24-hour TTL
	s.cache.Set(req.SchemaId, entry)

	// Mirror schema into embedded generator for fixture generation
	if genErr := s.generator.RegisterSchema(req.SchemaId, []byte(req.SchemaContent)); genErr != nil {
		Warn("Failed to register schema in generator", zap.String("schemaId", req.SchemaId), zap.Error(genErr))
	}

	// Persist to storage if available (write-through)
	var storageWarning string
	if s.store != nil {
		record := &storage.SchemaRecord{
			SchemaID:       req.SchemaId,
			Version:        version,
			Content:        req.SchemaContent,
			ContentHash:    entry.Metadata.SchemaHash,
			OpenAPIVersion: entry.Metadata.OpenapiVersion,
			EndpointCount:  entry.Metadata.EndpointCount,
			RegisteredAt:   time.Now(),
			UpdatedAt:      time.Now(),
			Ownership:      req.Ownership,
		}
		if storeErr := s.store.SetSchema(ctx, record); storeErr != nil {
			Warn("Failed to persist schema to storage (cache-only mode)",
				zap.String("schemaId", req.SchemaId),
				zap.Error(storeErr))
			storageWarning = " (WARNING: storage write failed, schema is cached only and will be lost on restart)"
		}
	}

	successMsg := "Schema registered successfully" + storageWarning

	Info("Schema registered successfully",
		zap.String("schemaId", req.SchemaId),
		zap.String("version", version),
		zap.String("hash", entry.Metadata.SchemaHash))

	// Record successful schema registration
	schemasRegistered.WithLabelValues("success").Inc()
	grpcRequestsTotal.WithLabelValues("RegisterSchema", "success").Inc()

	// Fire on_breaking_change_detected hook only when we actually ran the
	// compat check AND it surfaced changes. Helper guards on empty.
	s.fireOnBreakingChangeDetected(ctx, req.SchemaId, priorVersion, version, breakingChanges, "RegisterSchema")

	return &pb.RegisterSchemaResponse{
		Success:         true,
		Message:         successMsg,
		Metadata:        entry.Metadata,
		BreakingChanges: breakingChanges,
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

	// Retrieve schema (cache first, then storage)
	entry, found := s.getSchemaEntry(ctx, schemaID, req.SchemaVersion)
	if !found || entry == nil {
		Warn("Schema not found", zap.String("schemaId", schemaID))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("schema_not_found").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateInteraction", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Schema not found: %s", schemaID)},
		}, nil
	}
	doc := entry.Document

	// Handle basePath from Swagger 2.0 schemas and resolve base URL
	requestPath := stripBasePath(doc, req.Request.Path)
	baseURL := resolveBaseURL(doc)

	// Create request data with potentially modified path
	modifiedReqData := &pb.RequestData{
		Method:  req.Request.Method,
		Path:    requestPath,
		Headers: req.Request.Headers,
		Body:    req.Request.Body,
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

	// Use the pre-built router from the schema entry (built at registration time).
	// Use a local variable to avoid mutating the shared cache entry (data race).
	router := entry.Router
	if router == nil {
		// Fallback: build router on the fly if not cached
		var routerErr error
		router, routerErr = s.buildRouter(doc)
		if routerErr != nil {
			Error("Failed to create router",
				zap.String("schemaId", schemaID),
				zap.Error(routerErr))
			validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
			validationErrors.WithLabelValues("router_creation").Inc()
			grpcRequestsTotal.WithLabelValues("ValidateInteraction", "failure").Inc()
			return &pb.ValidationResult{
				Valid:  false,
				Errors: []string{fmt.Sprintf("Failed to create router: %v", routerErr)},
			}, nil
		}
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

	// Asynchronously record validation to storage
	if s.store != nil {
		go func() {
			record := &storage.ValidationRecord{
				SchemaID:       schemaID,
				SchemaVersion:  entry.Metadata.SchemaVersion,
				SchemaHash:     entry.Metadata.SchemaHash,
				RequestMethod:  method,
				RequestPath:    req.Request.Path,
				ResponseStatus: req.Response.StatusCode,
				Valid:          result.Valid,
				Errors:         result.Errors,
				DurationMs:     time.Since(start).Milliseconds(),
				ValidatedAt:    time.Now(),
			}
			if recErr := s.store.RecordValidation(context.Background(), record); recErr != nil {
				Warn("Failed to record validation", zap.Error(recErr))
			}
		}()
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
	if err := ValidateRequestBody(req.Request.Body); err != nil {
		return err
	}
	if req.Response == nil {
		return fmt.Errorf("ResponseData cannot be null")
	}
	if err := ValidateStatusCode(req.Response.StatusCode); err != nil {
		return err
	}
	if err := ValidateResponseBody(req.Response.Body); err != nil {
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

	Debug("Created HTTP request for validation",
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

// getSchemaEntry retrieves a schema entry, trying cache first then storage.
func (s *ValidatorService) getSchemaEntry(ctx context.Context, schemaID, version string) (*SchemaEntry, bool) {
	// Try cache first
	var entry *SchemaEntry
	var found bool
	if version != "" {
		entry, found = s.cache.GetVersion(schemaID, version)
	} else {
		entry, found = s.cache.Get(schemaID)
	}
	if found && entry != nil {
		return entry, true
	}

	// Cache miss — try storage
	if s.store == nil {
		return nil, false
	}

	var record *storage.SchemaRecord
	var err error
	if version != "" {
		record, err = s.store.GetSchemaVersion(ctx, schemaID, version)
	} else {
		record, err = s.store.GetSchema(ctx, schemaID)
	}
	if err != nil {
		Warn("Failed to read schema from storage",
			zap.String("schemaId", schemaID),
			zap.Error(err))
		return nil, false
	}
	if record == nil {
		return nil, false
	}

	// Rehydrate: parse the schema content and populate cache
	doc, parseErr := s.parseAndConvertSchema([]byte(record.Content))
	if parseErr != nil {
		Warn("Failed to rehydrate schema from storage",
			zap.String("schemaId", schemaID),
			zap.Error(parseErr))
		return nil, false
	}

	// Validate the rehydrated schema
	loader := openapi3.NewLoader()
	if valErr := doc.Validate(loader.Context); valErr != nil {
		Warn("Rehydrated schema failed validation",
			zap.String("schemaId", schemaID),
			zap.Error(valErr))
		return nil, false
	}

	entry = NewSchemaEntry(schemaID, record.Content, doc, record.Version, record.Ownership)

	// Build router
	router, routerErr := s.buildRouter(doc)
	if routerErr != nil {
		Warn("Failed to build router for rehydrated schema",
			zap.String("schemaId", schemaID),
			zap.Error(routerErr))
		return nil, false
	}
	entry.Router = router

	// Repopulate cache — only write to the bare (unversioned) key when the
	// lookup was unversioned, to avoid promoting an old version into the
	// "latest" cache slot.
	if version == "" {
		s.cache.Set(schemaID, entry)
	} else {
		// Store only under the versioned key
		s.cache.SetVersion(schemaID, record.Version, entry)
	}

	// Mirror into embedded generator for fixture generation
	if genErr := s.generator.RegisterSchema(schemaID, []byte(record.Content)); genErr != nil {
		Warn("Failed to register rehydrated schema in generator", zap.String("schemaId", schemaID), zap.Error(genErr))
	}

	Info("Rehydrated schema from storage into cache",
		zap.String("schemaId", schemaID),
		zap.String("version", record.Version))

	return entry, true
}

// lookupPriorSchemaForCompat returns the latest registered schema entry
// for use as the "prior" side of a compatibility check. Differs from
// getSchemaEntry by surfacing storage errors instead of swallowing them
// — required by decision 1C (fail-closed on storage error during
// --check-compatibility).
//
// Returns:
//   - (entry, true, nil)  prior version exists in cache or storage
//   - (nil, false, nil)   no prior version (first registration of schemaID)
//   - (nil, false, err)   storage error during lookup; caller refuses register
func (s *ValidatorService) lookupPriorSchemaForCompat(ctx context.Context, schemaID string) (*SchemaEntry, bool, error) {
	if entry, ok := s.cache.Get(schemaID); ok && entry != nil {
		return entry, true, nil
	}
	if s.store == nil {
		return nil, false, nil
	}
	record, err := s.store.GetSchema(ctx, schemaID)
	if err != nil {
		return nil, false, err
	}
	if record == nil {
		return nil, false, nil
	}
	doc, parseErr := s.parseAndConvertSchema([]byte(record.Content))
	if parseErr != nil {
		return nil, false, fmt.Errorf("rehydrate prior schema: %w", parseErr)
	}
	entry := NewSchemaEntry(schemaID, record.Content, doc, record.Version, record.Ownership)
	return entry, true, nil
}

// buildRouter creates a gorillamux router from a document, handling Swagger v2 basePath.
// It creates a copy of the servers slice to avoid mutating the original document.
func (s *ValidatorService) buildRouter(doc *openapi3.T) (routers.Router, error) {
	routingDoc := doc
	if len(doc.Servers) > 0 {
		// Shallow-copy the document and replace Servers to avoid mutating the original
		docCopy := *doc
		routingDoc = &docCopy
		modifiedServers := make(openapi3.Servers, 0, len(doc.Servers))
		for _, server := range doc.Servers {
			if serverURL, parseErr := url.Parse(server.URL); parseErr == nil {
				if serverURL.Path != "" && serverURL.Path != "/" {
					serverURL.Path = ""
					serverURL.RawPath = ""
					modifiedServers = append(modifiedServers, &openapi3.Server{
						URL:         serverURL.String(),
						Description: server.Description,
						Variables:   server.Variables,
					})
				} else {
					modifiedServers = append(modifiedServers, server)
				}
			} else {
				modifiedServers = append(modifiedServers, server)
			}
		}
		routingDoc.Servers = modifiedServers
	}
	return gorillamux.NewRouter(routingDoc)
}

// stripBasePath removes the server basePath prefix from the request path if present.
// This handles Swagger 2.0 schemas that are converted to OpenAPI 3 with server URLs
// containing path components (e.g., /api/v2).
func stripBasePath(doc *openapi3.T, requestPath string) string {
	if len(doc.Servers) > 0 {
		for _, server := range doc.Servers {
			if serverURL, err := url.Parse(server.URL); err == nil && serverURL.Path != "" && serverURL.Path != "/" {
				basePath := strings.TrimSuffix(serverURL.Path, "/")
				if strings.HasPrefix(requestPath, basePath+"/") {
					stripped := strings.TrimPrefix(requestPath, basePath)
					Debug("Stripped basePath from request path",
						zap.String("basePath", basePath),
						zap.String("originalPath", requestPath),
						zap.String("strippedPath", stripped))
					return stripped
				}
			}
		}
	}
	return requestPath
}

// resolveBaseURL extracts the base URL (scheme + host) from the first server entry.
// Returns "http://localhost" if no server entries are found or parsing fails.
func resolveBaseURL(doc *openapi3.T) string {
	if len(doc.Servers) > 0 {
		if serverURL, err := url.Parse(doc.Servers[0].URL); err == nil &&
			serverURL.Scheme != "" && serverURL.Host != "" {
			baseURL := fmt.Sprintf("%s://%s", serverURL.Scheme, serverURL.Host)
			Debug("Using server URL as base for HTTP request", zap.String("baseURL", baseURL))
			return baseURL
		}
	}
	return "http://localhost"
}

// Close cleans up resources used by the ValidatorService.
// This should be called when the service is being shut down to
// properly close the schema cache and release any resources.
func (s *ValidatorService) Close() {
	if s.cache != nil {
		s.cache.Close()
	}
	if s.store != nil {
		_ = s.store.Close()
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

	entry, found := s.getSchemaEntry(ctx, req.SchemaId, req.SchemaVersion)
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
		entry, found := s.getSchemaEntry(ctx, schemaID, "")
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
		entry, found := s.getSchemaEntry(ctx, req.SchemaId, "")
		if found && entry != nil && entry.Metadata != nil {
			oldVersion, _ = s.cache.GetPreviousVersion(req.SchemaId, entry.Metadata.SchemaVersion)
		}
	}

	// Get new version
	newVersion := req.NewVersion
	if newVersion == "" {
		entry, found := s.getSchemaEntry(ctx, req.SchemaId, "")
		if found && entry != nil && entry.Metadata != nil {
			newVersion = entry.Metadata.SchemaVersion
		}
	}

	// Get both schema entries
	var oldEntry, newEntry *SchemaEntry
	var oldFound, newFound bool

	if oldVersion != "" {
		oldEntry, oldFound = s.getSchemaEntry(ctx, req.SchemaId, oldVersion)
	}
	if newVersion != "" {
		newEntry, newFound = s.getSchemaEntry(ctx, req.SchemaId, newVersion)
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

	// Fire on_breaking_change_detected hook (helper guards on empty changes).
	s.fireOnBreakingChangeDetected(ctx, req.SchemaId, oldVersion, newVersion, breakingChanges, "CompareSchemas")

	grpcRequestsTotal.WithLabelValues("CompareSchemas", "success").Inc()
	return &pb.CompareSchemasResponse{
		Compatible:      compatible,
		BreakingChanges: breakingChanges,
		OldSchema:       oldEntry.Metadata,
		NewSchema:       newEntry.Metadata,
	}, nil
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

	entry, found := s.getSchemaEntry(ctx, req.SchemaId, "")
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
