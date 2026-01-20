package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sahina/cvt/sdks/go/cvt"
)

// MockingRoundTripperConfig configures the MockingRoundTripper.
type MockingRoundTripperConfig struct {
	// Validator is the CVT validator instance (required).
	// Must implement MockingValidator interface (supports GenerateResponse).
	Validator MockingValidator

	// ValidateRequests enables request validation before generating response.
	// When true, requests are validated against the schema.
	// Default: false.
	ValidateRequests bool

	// GenerateOptions configures response generation.
	// If nil, sensible defaults are used (UseExamples=true, ContentType="application/json").
	GenerateOptions *cvt.GenerateOptions

	// IncludePaths filters requests to only mock matching paths.
	// If empty, all paths are mocked.
	IncludePaths []PathFilter

	// ExcludePaths filters requests to exclude matching paths from mocking.
	// Excluded paths will return an error.
	ExcludePaths []PathFilter

	// CacheResponses enables caching of generated responses.
	// When true, responses are cached per method+path combination.
	// Default: false.
	CacheResponses bool

	// OnRequestValidationFailure is called when request validation fails.
	// If it returns an error, that error is returned from RoundTrip.
	// If nil, validation failures cause RoundTrip to return an error.
	OnRequestValidationFailure func(result *cvt.ValidationResult, req *http.Request) error
}

// MockingRoundTripper is an http.RoundTripper that returns schema-generated
// mock responses without calling real API endpoints.
//
// This is useful for testing consumers against OpenAPI schemas without
// requiring the producer API to be running.
//
// Example:
//
//	validator, _ := cvt.NewValidator("")
//	validator.RegisterSchema(ctx, "api", "./openapi.json")
//
//	rt := adapters.NewMockingRoundTripper(adapters.MockingRoundTripperConfig{
//	    Validator:        validator,
//	    ValidateRequests: true,
//	    CacheResponses:   true,
//	})
//
//	client := &http.Client{Transport: rt}
//	resp, err := client.Get("http://mock.api/users/123")
//	// resp contains schema-generated mock data, no real API call made
type MockingRoundTripper struct {
	config       MockingRoundTripperConfig
	interactions []CapturedInteraction
	cache        map[string]cachedResponse
	mu           sync.Mutex
}

// cachedResponse stores a cached generated response.
type cachedResponse struct {
	response *cvt.GeneratedResponse
	body     []byte
}

// NewMockingRoundTripper creates a new MockingRoundTripper.
func NewMockingRoundTripper(config MockingRoundTripperConfig) *MockingRoundTripper {
	if config.Validator == nil {
		panic("cvt: Validator is required")
	}

	return &MockingRoundTripper{
		config:       config,
		interactions: make([]CapturedInteraction, 0),
		cache:        make(map[string]cachedResponse),
	}
}

// RoundTrip implements http.RoundTripper.
// Instead of making a real HTTP request, it generates a mock response
// from the registered OpenAPI schema.
func (m *MockingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Build path with query string
	path := req.URL.Path
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}

	// Check path filters
	if !shouldValidatePath(path, m.config.IncludePaths, m.config.ExcludePaths) {
		return nil, fmt.Errorf("cvt: path %q is excluded from mocking", path)
	}

	// Capture request body
	var reqBody []byte
	if req.Body != nil {
		var err error
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	// Build validation request
	validationReq := m.extractRequest(req, reqBody)

	// Generate or retrieve cached response first
	generated, respBody, err := m.getOrGenerateResponse(ctx, req.Method, path)
	if err != nil {
		return nil, fmt.Errorf("failed to generate mock response: %w", err)
	}

	// Build validation response
	validationResp := cvt.ValidationResponse{
		StatusCode: generated.StatusCode,
		Headers:    generated.Headers,
		Body:       generated.Body,
	}

	// Validate the full interaction if enabled
	if m.config.ValidateRequests {
		result, err := m.config.Validator.Validate(ctx, validationReq, validationResp)
		if err != nil {
			return nil, fmt.Errorf("validation error: %w", err)
		}
		if !result.Valid {
			if m.config.OnRequestValidationFailure != nil {
				if failErr := m.config.OnRequestValidationFailure(result, req); failErr != nil {
					return nil, failErr
				}
			} else {
				return nil, fmt.Errorf("request validation failed: %v", result.Errors)
			}
		}
	}

	// Build HTTP response
	resp := m.buildHTTPResponse(req, generated, respBody)

	// Record interaction
	interaction := CapturedInteraction{
		Request:          validationReq,
		Response:         validationResp,
		ValidationResult: &cvt.ValidationResult{Valid: true},
		Timestamp:        time.Now(),
	}
	m.addInteraction(interaction)

	return resp, nil
}

// getOrGenerateResponse returns a cached response or generates a new one.
func (m *MockingRoundTripper) getOrGenerateResponse(ctx context.Context, method, path string) (*cvt.GeneratedResponse, []byte, error) {
	cacheKey := method + ":" + path

	// Check cache if enabled
	if m.config.CacheResponses {
		m.mu.Lock()
		if cached, ok := m.cache[cacheKey]; ok {
			m.mu.Unlock()
			return cached.response, cached.body, nil
		}
		m.mu.Unlock()
	}

	// Generate new response
	generated, err := m.config.Validator.GenerateResponse(ctx, method, path, m.config.GenerateOptions)
	if err != nil {
		return nil, nil, err
	}

	// Serialize body
	var respBody []byte
	if generated.Body != nil {
		respBody, err = json.Marshal(generated.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal response body: %w", err)
		}
	}

	// Cache if enabled
	if m.config.CacheResponses {
		m.mu.Lock()
		m.cache[cacheKey] = cachedResponse{
			response: generated,
			body:     respBody,
		}
		m.mu.Unlock()
	}

	return generated, respBody, nil
}

// buildHTTPResponse converts a GeneratedResponse to an http.Response.
func (m *MockingRoundTripper) buildHTTPResponse(req *http.Request, generated *cvt.GeneratedResponse, body []byte) *http.Response {
	resp := &http.Response{
		Status:        http.StatusText(generated.StatusCode),
		StatusCode:    generated.StatusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}

	// Copy headers from generated response
	for k, v := range generated.Headers {
		resp.Header.Set(k, v)
	}

	// Ensure Content-Type is set
	if resp.Header.Get("Content-Type") == "" && len(body) > 0 {
		resp.Header.Set("Content-Type", "application/json")
	}

	return resp
}

// extractRequest converts an http.Request to a ValidationRequest.
func (m *MockingRoundTripper) extractRequest(req *http.Request, body []byte) cvt.ValidationRequest {
	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var bodyData any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &bodyData)
	}

	// Store full URL for request extraction (used for captured interactions)
	fullURL := req.URL.String()

	return cvt.ValidationRequest{
		Method:  req.Method,
		Path:    fullURL,
		Headers: headers,
		Body:    bodyData,
	}
}

// addInteraction records an interaction.
func (m *MockingRoundTripper) addInteraction(interaction CapturedInteraction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interactions = append(m.interactions, interaction)
}

// GetInteractions returns all captured interactions.
func (m *MockingRoundTripper) GetInteractions() []CapturedInteraction {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]CapturedInteraction, len(m.interactions))
	copy(result, m.interactions)
	return result
}

// ClearInteractions clears all captured interactions.
func (m *MockingRoundTripper) ClearInteractions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interactions = m.interactions[:0]
}

// ClearCache clears the response cache.
func (m *MockingRoundTripper) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]cachedResponse)
}

// =============================================================================
// Simplified API with Functional Options
// =============================================================================

// MockOption configures mock client behavior.
type MockOption func(*MockingRoundTripperConfig)

// WithCache enables response caching per method+path.
func WithCache() MockOption {
	return func(c *MockingRoundTripperConfig) {
		c.CacheResponses = true
	}
}

// WithRequestValidation enables request validation against the schema.
func WithRequestValidation() MockOption {
	return func(c *MockingRoundTripperConfig) {
		c.ValidateRequests = true
	}
}

// WithGenerateOptions sets custom options for response generation.
func WithGenerateOptions(opts *cvt.GenerateOptions) MockOption {
	return func(c *MockingRoundTripperConfig) {
		c.GenerateOptions = opts
	}
}

// WithIncludePaths filters requests to only mock matching paths.
func WithIncludePaths(paths ...PathFilter) MockOption {
	return func(c *MockingRoundTripperConfig) {
		c.IncludePaths = paths
	}
}

// WithExcludePaths filters requests to exclude matching paths from mocking.
func WithExcludePaths(paths ...PathFilter) MockOption {
	return func(c *MockingRoundTripperConfig) {
		c.ExcludePaths = paths
	}
}

// NewMockClient creates an http.Client that returns schema-generated mock responses.
// This is the simplest way to create a mock client for testing.
//
// Example:
//
//	client := adapters.NewMockClient(validator)
//	resp, _ := client.Get("http://any.host/users/123")
//
// With options:
//
//	client := adapters.NewMockClient(validator, adapters.WithCache())
func NewMockClient(validator MockingValidator, opts ...MockOption) *http.Client {
	return NewMock(validator, opts...).Client()
}

// Mock wraps a MockingRoundTripper and provides access to both the http.Client
// and interaction recording methods.
//
// Use this when you need to inspect captured interactions or clear cache.
//
// Example:
//
//	mock := adapters.NewMock(validator, adapters.WithCache())
//	client := mock.Client()
//	resp, _ := client.Get("http://any.host/users/123")
//	interactions := mock.GetInteractions()
type Mock struct {
	transport *MockingRoundTripper
}

// NewMock creates a new Mock wrapper with the given validator and options.
func NewMock(validator MockingValidator, opts ...MockOption) *Mock {
	config := MockingRoundTripperConfig{
		Validator: validator,
	}
	for _, opt := range opts {
		opt(&config)
	}
	return &Mock{
		transport: NewMockingRoundTripper(config),
	}
}

// Client returns an http.Client configured to use the mock transport.
func (m *Mock) Client() *http.Client {
	return &http.Client{Transport: m.transport}
}

// GetInteractions returns all captured interactions.
func (m *Mock) GetInteractions() []CapturedInteraction {
	return m.transport.GetInteractions()
}

// ClearInteractions clears all captured interactions.
func (m *Mock) ClearInteractions() {
	m.transport.ClearInteractions()
}

// ClearCache clears the response cache.
func (m *Mock) ClearCache() {
	m.transport.ClearCache()
}
