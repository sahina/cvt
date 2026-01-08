package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
)

// Producer handles server-side validation of HTTP requests and responses.
type Producer struct {
	config Config
	mu     sync.RWMutex
}

// NewProducer creates a new Producer with the given configuration.
func NewProducer(config Config) (*Producer, error) {
	// Apply defaults
	if config.Mode == "" {
		config.Mode = ModeStrict
	}
	if !config.ValidateRequest && !config.ValidateResponse {
		config.ValidateRequest = true
		config.ValidateResponse = true
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Producer{
		config: config,
	}, nil
}

// ValidateRequest validates an incoming HTTP request against the schema.
// Returns the validation result. For Strict mode, check result.Valid to decide
// whether to reject the request.
func (p *Producer) ValidateRequest(ctx context.Context, r *http.Request, body []byte) *ValidationResult {
	result := &ValidationResult{
		Type:  "request",
		Valid: true,
	}

	if !p.config.ValidateRequest {
		return result
	}

	// Build interaction for request-only validation
	interaction := &Interaction{
		Method:  r.Method,
		Path:    p.buildPath(r),
		Headers: p.extractHeaders(r.Header),
		Body:    string(body),
		// Use a minimal valid response to pass through - we're only validating request
		StatusCode:   200,
		ResponseBody: "{}",
	}

	validationResult, err := p.validateInteraction(ctx, interaction)
	if err != nil {
		result.Valid = false
		result.Errors = []string{err.Error()}
		return result
	}

	// Check if the result contains request-specific errors
	result.Valid = validationResult.Valid
	result.Errors = validationResult.Errors

	return result
}

// ValidateResponse validates an outgoing HTTP response against the schema.
func (p *Producer) ValidateResponse(ctx context.Context, r *http.Request, reqBody []byte, statusCode int, respHeaders http.Header, respBody []byte) *ValidationResult {
	result := &ValidationResult{
		Type:  "response",
		Valid: true,
	}

	if !p.config.ValidateResponse {
		return result
	}

	// Build full interaction for response validation
	interaction := &Interaction{
		Method:          r.Method,
		Path:            p.buildPath(r),
		Headers:         p.extractHeaders(r.Header),
		Body:            string(reqBody),
		StatusCode:      statusCode,
		ResponseHeaders: p.extractHeaders(respHeaders),
		ResponseBody:    string(respBody),
	}

	validationResult, err := p.validateInteraction(ctx, interaction)
	if err != nil {
		result.Valid = false
		result.Errors = []string{err.Error()}
		return result
	}

	result.Valid = validationResult.Valid
	result.Errors = validationResult.Errors

	return result
}

// validateInteraction performs the actual validation using configured backend.
func (p *Producer) validateInteraction(ctx context.Context, interaction *Interaction) (*ValidationResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.config.Validator.Validate(ctx, p.config.SchemaID, interaction)
}

// buildPath constructs the full path including query string.
func (p *Producer) buildPath(r *http.Request) string {
	path := r.URL.Path
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	return path
}

// extractHeaders converts http.Header to a simple map.
func (p *Producer) extractHeaders(h http.Header) map[string]string {
	headers := make(map[string]string)
	for k, v := range h {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return headers
}

// HandleRequestValidationFailure processes a request validation failure based on mode.
// Returns true if the request should continue, false if it was rejected.
func (p *Producer) HandleRequestValidationFailure(w http.ResponseWriter, r *http.Request, result *ValidationResult) bool {
	// Call custom handler if configured
	if p.config.OnRequestFailure != nil {
		return p.config.OnRequestFailure(w, r, result)
	}

	switch p.config.Mode {
	case ModeStrict:
		// Reject with 400 Bad Request
		p.writeErrorResponse(w, result)
		return false

	case ModeWarn:
		// Log and continue
		p.logValidationFailure("request", r, result)
		return true

	case ModeShadow:
		// Metrics only, continue
		recordValidationMetrics("request", result)
		return true
	}

	return true
}

// HandleResponseValidationFailure processes a response validation failure based on mode.
func (p *Producer) HandleResponseValidationFailure(r *http.Request, result *ValidationResult) {
	// Call custom handler if configured
	if p.config.OnResponseFailure != nil {
		p.config.OnResponseFailure(r, result)
		return
	}

	switch p.config.Mode {
	case ModeStrict, ModeWarn:
		// Log the failure (can't modify response - already sent)
		p.logValidationFailure("response", r, result)

	case ModeShadow:
		// Metrics only
		recordValidationMetrics("response", result)
	}
}

// writeErrorResponse writes a standardized error response for validation failures.
func (p *Producer) writeErrorResponse(w http.ResponseWriter, result *ValidationResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	response := map[string]any{
		"error":   "Request validation failed",
		"details": result.Errors,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// logValidationFailure logs a validation failure.
func (p *Producer) logValidationFailure(validationType string, r *http.Request, result *ValidationResult) {
	log.Printf("[CVT] %s validation failed for %s %s: %v",
		validationType, r.Method, r.URL.Path, result.Errors)
}

// ResponseCapture wraps http.ResponseWriter to capture the response.
type ResponseCapture struct {
	http.ResponseWriter
	StatusCode int
	Body       bytes.Buffer
	written    bool
}

// NewResponseCapture creates a new ResponseCapture wrapping the given ResponseWriter.
func NewResponseCapture(w http.ResponseWriter) *ResponseCapture {
	return &ResponseCapture{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code and writes it to the underlying writer.
func (rc *ResponseCapture) WriteHeader(code int) {
	if !rc.written {
		rc.StatusCode = code
		rc.ResponseWriter.WriteHeader(code)
		rc.written = true
	}
}

// Write captures the body and writes it to the underlying writer.
func (rc *ResponseCapture) Write(b []byte) (int, error) {
	if !rc.written {
		rc.WriteHeader(http.StatusOK)
	}
	rc.Body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// CaptureRequestBody reads and restores the request body.
func CaptureRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// Restore the body for the handler
	r.Body = io.NopCloser(bytes.NewReader(body))

	return body, nil
}
