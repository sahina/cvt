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
}

// NewValidator creates a new local validator instance.
func NewValidator() *Validator {
	return &Validator{
		schemas: make(map[string]*openapi3.T),
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

	return v.validateInteraction(doc, interaction)
}

// ValidateWithSchema validates an interaction against a schema provided directly.
func (v *Validator) ValidateWithSchema(schemaContent []byte, interaction *Interaction) (*ValidationResult, error) {
	doc, err := v.parseSchema(schemaContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return v.validateInteraction(doc, interaction)
}

func (v *Validator) validateInteraction(doc *openapi3.T, interaction *Interaction) (*ValidationResult, error) {
	var result ValidationResult
	result.Valid = true

	// Handle basePath from Swagger 2.0 schemas
	requestPath := interaction.Path
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

	// Create HTTP request
	var body io.Reader
	if interaction.Body != "" {
		body = strings.NewReader(interaction.Body)
	}

	baseURL := "http://localhost"
	if len(doc.Servers) > 0 {
		if serverURL, err := url.Parse(doc.Servers[0].URL); err == nil {
			baseURL = fmt.Sprintf("%s://%s", serverURL.Scheme, serverURL.Host)
		}
	}

	httpReq, err := http.NewRequest(interaction.Method, baseURL+requestPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for k, val := range interaction.Headers {
		httpReq.Header.Set(k, val)
	}

	// Temporarily strip basePath from server URLs for routing
	originalServers := doc.Servers
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
		doc.Servers = modifiedServers
	}

	// Create router
	router, err := gorillamux.NewRouter(doc)

	// Restore original servers
	doc.Servers = originalServers

	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	// Find route
	route, pathParams, err := router.FindRoute(httpReq)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("route not found: %v", err))
		return &result, nil
	}

	// Validate request
	ctx := context.Background()
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    httpReq,
		PathParams: pathParams,
		Route:      route,
	}

	if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
		return &result, nil
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
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
		return &result, nil
	}

	return &result, nil
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
