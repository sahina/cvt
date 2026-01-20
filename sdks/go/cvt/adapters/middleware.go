package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sahina/cvt/sdks/go/cvt"
)

// MiddlewareConfig configures the server-side validation middleware.
type MiddlewareConfig struct {
	// Validator is the CVT validator instance (required).
	// Accepts any type implementing the Validator interface.
	Validator Validator

	// AutoValidate enables automatic validation (default: true).
	AutoValidate bool

	// OnValidationFailure is called when validation fails.
	// Return true to continue handling, false to stop.
	OnValidationFailure func(w http.ResponseWriter, r *http.Request, result *cvt.ValidationResult) bool

	// IncludePaths filters requests to only validate matching paths.
	IncludePaths []PathFilter

	// ExcludePaths filters requests to exclude matching paths.
	ExcludePaths []PathFilter
}

// responseWriter wraps http.ResponseWriter to capture response data.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// ValidatingMiddleware is HTTP middleware that validates traffic against CVT.
//
// Example:
//
//	validator, _ := cvt.NewValidator("")
//	validator.RegisterSchema(ctx, "api", "./openapi.json")
//
//	middleware := adapters.NewValidatingMiddleware(adapters.MiddlewareConfig{
//	    Validator:    validator,
//	    AutoValidate: true,
//	})
//
//	handler := middleware.Handler(myHandler)
//	http.ListenAndServe(":8080", handler)
type ValidatingMiddleware struct {
	config       MiddlewareConfig
	interactions []CapturedInteraction
	mu           sync.Mutex
}

// NewValidatingMiddleware creates a new ValidatingMiddleware.
func NewValidatingMiddleware(config MiddlewareConfig) *ValidatingMiddleware {
	if config.Validator == nil {
		panic("cvt: Validator is required")
	}

	return &ValidatingMiddleware{
		config:       config,
		interactions: make([]CapturedInteraction, 0),
	}
}

// Handler returns an http.Handler that wraps the provided handler with validation.
func (m *ValidatingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}

		if !shouldValidatePath(path, m.config.IncludePaths, m.config.ExcludePaths) {
			next.ServeHTTP(w, r)
			return
		}

		// Capture request body
		var reqBody []byte
		if r.Body != nil {
			reqBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// Wrap response writer to capture response
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Build validation objects
		validationReq := m.extractRequest(r, reqBody)
		validationResp := m.extractResponse(wrapped)

		interaction := CapturedInteraction{
			Request:   validationReq,
			Response:  validationResp,
			Timestamp: time.Now(),
		}

		// Validate
		if m.config.AutoValidate {
			ctx := r.Context()
			result, err := m.config.Validator.Validate(ctx, validationReq, validationResp)
			if err == nil {
				interaction.ValidationResult = result
				if !result.Valid && m.config.OnValidationFailure != nil {
					m.config.OnValidationFailure(w, r, result)
				}
			}
		}

		m.addInteraction(interaction)
	})
}

// HandlerFunc returns an http.Handler that wraps the provided handler function.
func (m *ValidatingMiddleware) HandlerFunc(next http.HandlerFunc) http.Handler {
	return m.Handler(next)
}

func (m *ValidatingMiddleware) addInteraction(interaction CapturedInteraction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interactions = append(m.interactions, interaction)
}

func (m *ValidatingMiddleware) extractRequest(r *http.Request, body []byte) cvt.ValidationRequest {
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var bodyData any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &bodyData)
	}

	path := r.URL.Path
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}

	return cvt.ValidationRequest{
		Method:  r.Method,
		Path:    path,
		Headers: headers,
		Body:    bodyData,
	}
}

func (m *ValidatingMiddleware) extractResponse(rw *responseWriter) cvt.ValidationResponse {
	headers := make(map[string]string)
	for k, v := range rw.Header() {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var bodyData any
	if rw.body.Len() > 0 {
		_ = json.Unmarshal(rw.body.Bytes(), &bodyData)
	}

	return cvt.ValidationResponse{
		StatusCode: rw.statusCode,
		Headers:    headers,
		Body:       bodyData,
	}
}

// GetInteractions returns all captured interactions.
func (m *ValidatingMiddleware) GetInteractions() []CapturedInteraction {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]CapturedInteraction, len(m.interactions))
	copy(result, m.interactions)
	return result
}

// ClearInteractions clears all captured interactions.
func (m *ValidatingMiddleware) ClearInteractions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interactions = m.interactions[:0]
}

// ValidateInteraction manually validates a captured interaction.
func (m *ValidatingMiddleware) ValidateInteraction(ctx context.Context, interaction CapturedInteraction) (*cvt.ValidationResult, error) {
	return m.config.Validator.Validate(ctx, interaction.Request, interaction.Response)
}
