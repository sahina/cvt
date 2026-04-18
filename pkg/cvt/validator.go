// Package cvt provides an embedded library for contract validation without gRPC.
// This allows local validation of HTTP interactions against OpenAPI schemas.
package cvt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// Validator provides local validation of HTTP interactions against OpenAPI schemas.
// This is the embedded library version that doesn't require a gRPC server.
type Validator struct {
	schemas map[string]*openapi3.T
	mu      sync.RWMutex
	faker   *gofakeit.Faker
	hooks   Hooks
}

// SetHooks installs a Hooks implementation on the validator. When hooks are
// set, Validate fires OnValidationFailed after a non-valid result. nil
// hooks is equivalent to NoopHooks (no-op). This indirection keeps pkg/cvt
// free of internal/* imports.
//
// Safe to call concurrently with Validate: v.mu serializes the
// assignment against the read in hooksOrNoop.
func (v *Validator) SetHooks(h Hooks) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hooks = h
}

// hooksOrNoop returns the configured hooks or a NoopHooks if none set.
func (v *Validator) hooksOrNoop() Hooks {
	v.mu.RLock()
	h := v.hooks
	v.mu.RUnlock()
	if h == nil {
		return NoopHooks{}
	}
	return h
}

// NewValidator creates a new local validator instance with random faker seed.
func NewValidator() *Validator {
	return &Validator{
		schemas: make(map[string]*openapi3.T),
		faker:   gofakeit.New(0),
	}
}

// NewValidatorWithSeed creates a new local validator instance with a deterministic faker seed.
// Using the same seed produces identical generated values across calls.
func NewValidatorWithSeed(seed uint64) *Validator {
	return &Validator{
		schemas: make(map[string]*openapi3.T),
		faker:   gofakeit.New(seed),
	}
}

// RegisterSchema registers an OpenAPI schema for validation.
// Supports both OpenAPI v2 (Swagger) and v3 formats.
func (v *Validator) RegisterSchema(schemaID string, content []byte) error {
	if schemaID == "" {
		return fmt.Errorf("schema ID is required")
	}
	if len(content) == 0 {
		return fmt.Errorf("schema content is required")
	}

	doc, err := v.parseSchema(content)
	if err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	// Validate the schema
	loader := openapi3.NewLoader()
	if err := doc.Validate(loader.Context); err != nil {
		return fmt.Errorf("invalid OpenAPI schema: %w", err)
	}

	v.mu.Lock()
	v.schemas[schemaID] = doc
	v.mu.Unlock()

	return nil
}

// RegisterSchemaFromFile registers an OpenAPI schema from a file path.
func (v *Validator) RegisterSchemaFromFile(schemaID, filePath string) error {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(filePath)
	if err != nil {
		// Try parsing as Swagger 2.0
		doc, err = v.loadSwaggerFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to load schema from file: %w", err)
		}
	}

	loaderCtx := openapi3.NewLoader()
	if err := doc.Validate(loaderCtx.Context); err != nil {
		return fmt.Errorf("invalid OpenAPI schema: %w", err)
	}

	v.mu.Lock()
	v.schemas[schemaID] = doc
	v.mu.Unlock()

	return nil
}

// RegisterSchemaFromURL fetches and registers an OpenAPI schema from a URL.
func (v *Validator) RegisterSchemaFromURL(schemaID, schemaURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, schemaURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch schema from URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to fetch schema: HTTP %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return v.RegisterSchema(schemaID, content)
}

// RegisterSchemaFromPath loads a schema from a file path or URL.
// It automatically detects URLs (http:// or https://) and fetches them,
// otherwise treats the path as a local file.
func (v *Validator) RegisterSchemaFromPath(schemaID, path string) error {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return v.RegisterSchemaFromURL(schemaID, path)
	}
	return v.RegisterSchemaFromFile(schemaID, path)
}

// Interaction represents an HTTP request/response pair to validate.
type Interaction struct {
	// Request details
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`

	// Response details
	StatusCode      int               `json:"status_code"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    string            `json:"response_body"`
}

// ValidationResult contains the result of validating an interaction.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// Validate validates an HTTP interaction against a registered schema.
func (v *Validator) Validate(schemaID string, interaction *Interaction) (*ValidationResult, error) {
	v.mu.RLock()
	doc, ok := v.schemas[schemaID]
	v.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("schema not found: %s", schemaID)
	}

	result, err := v.validateInteraction(doc, interaction)
	if err == nil && result != nil && !result.Valid {
		v.fireOnValidationFailed(schemaID, interaction, result)
	}
	return result, err
}

// ValidateWithSchema validates an interaction against a schema provided directly.
func (v *Validator) ValidateWithSchema(schemaContent []byte, interaction *Interaction) (*ValidationResult, error) {
	doc, err := v.parseSchema(schemaContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return v.validateInteraction(doc, interaction)
}

// prepareAndFindRoute handles basePath stripping, HTTP request construction,
// doc.Servers cloning (thread safety), router creation, and route finding.
//
// Returns:
//   - httpReq, requestValidationInput, nil, nil on success (route found)
//   - httpReq, nil, routeNotFoundResult, nil when route not found (ValidationResult with Valid=false)
//   - nil, nil, nil, err on hard error (failed to create request/router)
func (v *Validator) prepareAndFindRoute(doc *openapi3.T, method, path string, headers map[string]string, body io.Reader) (*http.Request, *openapi3filter.RequestValidationInput, *ValidationResult, error) {
	// Handle basePath from Swagger 2.0 schemas
	requestPath := path
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

	// Derive base URL from server definitions
	baseURL := "http://localhost"
	if len(doc.Servers) > 0 {
		if serverURL, err := url.Parse(doc.Servers[0].URL); err == nil &&
			serverURL.Scheme != "" && serverURL.Host != "" {
			baseURL = fmt.Sprintf("%s://%s", serverURL.Scheme, serverURL.Host)
		}
	}

	httpReq, err := http.NewRequest(method, baseURL+requestPath, body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for k, val := range headers {
		httpReq.Header.Set(k, val)
	}

	// Create a shallow clone of the doc with stripped server URLs for routing.
	// This avoids mutating doc.Servers in place, which is not thread-safe.
	routingDoc := *doc
	if len(doc.Servers) > 0 {
		modifiedServers := make(openapi3.Servers, 0, len(doc.Servers))
		for _, server := range doc.Servers {
			if serverURL, err := url.Parse(server.URL); err == nil {
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
		}
		routingDoc.Servers = modifiedServers
	}

	// Create router from the clone (original doc is never mutated)
	router, err := gorillamux.NewRouter(&routingDoc)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create router: %w", err)
	}

	// Find route
	route, pathParams, err := router.FindRoute(httpReq)
	if err != nil {
		return httpReq, nil, &ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("route not found: %v", err)},
		}, nil
	}

	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    httpReq,
		PathParams: pathParams,
		Route:      route,
	}

	return httpReq, requestValidationInput, nil, nil
}

func (v *Validator) validateInteraction(doc *openapi3.T, interaction *Interaction) (*ValidationResult, error) {
	// Prepare body reader
	var body io.Reader
	if interaction.Body != "" {
		body = strings.NewReader(interaction.Body)
	}

	_, requestValidationInput, routeResult, err := v.prepareAndFindRoute(doc, interaction.Method, interaction.Path, interaction.Headers, body)
	if err != nil {
		return nil, err
	}
	if routeResult != nil {
		return routeResult, nil
	}

	// Validate request
	ctx := context.Background()
	if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
		return &ValidationResult{Valid: false, Errors: []string{err.Error()}}, nil
	}

	// Validate response
	responseHeaders := make(http.Header)
	for k, val := range interaction.ResponseHeaders {
		responseHeaders.Set(k, val)
	}

	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 interaction.StatusCode,
		Header:                 responseHeaders,
		Body:                   io.NopCloser(strings.NewReader(interaction.ResponseBody)),
	}

	if err := openapi3filter.ValidateResponse(ctx, responseValidationInput); err != nil {
		return &ValidationResult{Valid: false, Errors: []string{err.Error()}}, nil
	}

	return &ValidationResult{Valid: true}, nil
}

// ValidateRequest validates only the HTTP request (no response) against a registered schema.
// This is useful for mock servers that need to validate incoming requests without a response.
func (v *Validator) ValidateRequest(schemaID, method, path string, headers map[string]string, body string) (*ValidationResult, error) {
	v.mu.RLock()
	doc, ok := v.schemas[schemaID]
	v.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("schema not found: %s", schemaID)
	}

	// Prepare body reader
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	_, requestValidationInput, routeResult, err := v.prepareAndFindRoute(doc, method, path, headers, bodyReader)
	if err != nil {
		return nil, err
	}
	if routeResult != nil {
		return routeResult, nil
	}

	// Validate request only (no response validation)
	ctx := context.Background()
	if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
		return &ValidationResult{Valid: false, Errors: []string{err.Error()}}, nil
	}

	return &ValidationResult{Valid: true}, nil
}

// parseSchema parses OpenAPI v2 or v3 schema content.
func (v *Validator) parseSchema(data []byte) (*openapi3.T, error) {
	var rawSchema map[string]interface{}
	if err := json.Unmarshal(data, &rawSchema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	// Check if it's OpenAPI v2 (Swagger)
	if swagger, ok := rawSchema["swagger"].(string); ok && strings.HasPrefix(swagger, "2.") {
		var v2Doc openapi2.T
		if err := json.Unmarshal(data, &v2Doc); err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI v2 schema: %w", err)
		}

		v3Doc, err := openapi2conv.ToV3(&v2Doc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert OpenAPI v2 to v3: %w", err)
		}
		return v3Doc, nil
	}

	// Check if it's OpenAPI v3
	if openapi, ok := rawSchema["openapi"].(string); ok && strings.HasPrefix(openapi, "3.") {
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromData(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI v3 schema: %w", err)
		}
		return doc, nil
	}

	return nil, fmt.Errorf("unsupported schema format: must be OpenAPI v2 (Swagger 2.0) or OpenAPI v3.x")
}

// loadSwaggerFile loads a Swagger 2.0 file and converts it to OpenAPI 3.
func (v *Validator) loadSwaggerFile(filePath string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	// Try loading directly first
	doc, err := loader.LoadFromFile(filePath)
	if err == nil {
		return doc, nil
	}

	// If that fails, try loading as raw JSON
	return nil, fmt.Errorf("failed to load schema file: %w", err)
}

// ListSchemas returns all registered schema IDs.
func (v *Validator) ListSchemas() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	ids := make([]string, 0, len(v.schemas))
	for id := range v.schemas {
		ids = append(ids, id)
	}
	return ids
}

// RemoveSchema removes a schema from the validator.
func (v *Validator) RemoveSchema(schemaID string) {
	v.mu.Lock()
	delete(v.schemas, schemaID)
	v.mu.Unlock()
}

// GetSchema returns the parsed OpenAPI document for a schema.
func (v *Validator) GetSchema(schemaID string) (*openapi3.T, bool) {
	v.mu.RLock()
	doc, ok := v.schemas[schemaID]
	v.mu.RUnlock()
	return doc, ok
}
