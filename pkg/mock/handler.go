// Package mock provides an HTTP handler that serves mock responses from OpenAPI schemas.
package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/sahina/cvt/pkg/cvt"
)

// HandlerConfig controls mock handler behavior.
type HandlerConfig struct {
	UseExamples      bool
	ValidateRequests bool
	LatencyMs        int
	Quiet            bool
}

// MockHandler serves mock HTTP responses from OpenAPI schemas.
type MockHandler struct {
	validator *cvt.Validator
	schemaIDs []string // ordered for deterministic multi-schema matching
	config    HandlerConfig
}

// NewMockHandler creates a new mock HTTP handler.
func NewMockHandler(v *cvt.Validator, schemaIDs []string, cfg HandlerConfig) *MockHandler {
	return &MockHandler{
		validator: v,
		schemaIDs: schemaIDs,
		config:    cfg,
	}
}

// ServeHTTP handles incoming HTTP requests by matching them against registered
// OpenAPI schemas and returning generated mock responses.
func (h *MockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Apply latency delay if configured
	if h.config.LatencyMs > 0 {
		time.Sleep(time.Duration(h.config.LatencyMs) * time.Millisecond)
	}

	// Try each schema in order for deterministic routing
	for _, schemaID := range h.schemaIDs {
		doc, ok := h.validator.GetSchema(schemaID)
		if !ok {
			continue
		}

		router, err := gorillamux.NewRouter(doc)
		if err != nil {
			continue
		}

		route, _, err := router.FindRoute(r)
		if err != nil {
			continue
		}

		// Route matched — handle it
		h.handleMatchedRoute(w, r, schemaID, doc, route.Operation, start)
		return
	}

	// No schema matched
	h.writeJSON(w, http.StatusNotFound, map[string]interface{}{
		"error":  "no matching route",
		"method": r.Method,
		"path":   r.URL.Path,
	})
	h.logRequest(r, http.StatusNotFound, start)
}

// handleMatchedRoute processes a request that matched a route in a schema.
func (h *MockHandler) handleMatchedRoute(w http.ResponseWriter, r *http.Request, schemaID string, doc *openapi3.T, op *openapi3.Operation, start time.Time) {
	// Validate request if configured
	if h.config.ValidateRequests {
		// Read and restore the body so it can be read again downstream
		var bodyStr string
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil {
				bodyStr = string(bodyBytes)
			}
			r.Body = io.NopCloser(strings.NewReader(bodyStr))
		}

		headers := make(map[string]string)
		for k := range r.Header {
			headers[k] = r.Header.Get(k)
		}

		result, err := h.validator.ValidateRequest(schemaID, r.Method, r.URL.Path, headers, bodyStr)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":   "request validation error",
				"details": err.Error(),
			})
			h.logRequest(r, http.StatusInternalServerError, start)
			return
		}
		if !result.Valid {
			h.writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":      "request validation failed",
				"violations": result.Errors,
			})
			h.logRequest(r, http.StatusBadRequest, start)
			return
		}
	}

	// Determine success status code
	statusCode := selectSuccessStatus(op)

	// Check if operation has JSON response content
	if !hasJSONResponse(op, statusCode) {
		contentType := getFirstContentType(op, statusCode)
		h.writeJSON(w, http.StatusNotAcceptable, map[string]interface{}{
			"error":       "mock server only supports application/json responses",
			"method":      r.Method,
			"path":        r.URL.Path,
			"contentType": contentType,
		})
		h.logRequest(r, http.StatusNotAcceptable, start)
		return
	}

	// Resolve the template path from the operation back to the OpenAPI path pattern
	templatePath := h.findTemplatePath(doc, r.Method, op)

	// Generate the response
	opts := cvt.GenerateOptions{
		UseExamples: h.config.UseExamples,
		ContentType: "application/json",
	}
	resp, err := h.validator.GenerateResponse(schemaID, r.Method, templatePath, opts)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":   "response generation failed",
			"details": err.Error(),
		})
		h.logRequest(r, http.StatusInternalServerError, start)
		return
	}

	// Write the generated response
	w.Header().Set("Content-Type", "application/json")
	for k, v := range resp.Headers {
		if !strings.EqualFold(k, "Content-Type") {
			w.Header().Set(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if resp.Body != nil {
		data, err := json.Marshal(resp.Body)
		if err != nil {
			// Already wrote status, best effort
			h.logRequest(r, resp.StatusCode, start)
			return
		}
		_, _ = w.Write(data)
	}

	h.logRequest(r, resp.StatusCode, start)
}

// findTemplatePath finds the OpenAPI template path (e.g. /users/{id}) for an operation.
func (h *MockHandler) findTemplatePath(doc *openapi3.T, method string, op *openapi3.Operation) string {
	if doc.Paths == nil {
		return ""
	}
	for path, pathItem := range doc.Paths.Map() {
		if pathItem.GetOperation(method) == op {
			return path
		}
	}
	return ""
}

// RecoverMiddleware wraps an http.Handler with panic recovery that returns 500 JSON.
func (h *MockHandler) RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				h.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// writeJSON writes a JSON response with the given status code.
func (h *MockHandler) writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	data, err := json.Marshal(body)
	if err != nil {
		return
	}
	_, _ = w.Write(data)
}

// logRequest logs a request if not in quiet mode.
func (h *MockHandler) logRequest(r *http.Request, status int, start time.Time) {
	if h.config.Quiet {
		return
	}
	elapsed := time.Since(start)
	log.Printf("[mock] %s %s -> %d (%dms)", r.Method, r.URL.Path, status, elapsed.Milliseconds())
}

// selectSuccessStatus returns the first 2XX status code from the operation, preferring
// 200, 201, 202, 204 in that order.
func selectSuccessStatus(op *openapi3.Operation) int {
	if op.Responses == nil {
		return 200
	}

	preferred := []int{200, 201, 202, 204}
	for _, code := range preferred {
		if op.Responses.Status(code) != nil {
			return code
		}
	}

	// Check for any 2XX status
	for code := range op.Responses.Map() {
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

// hasJSONResponse checks if the operation has a JSON response content type for the
// given status code.
func hasJSONResponse(op *openapi3.Operation, statusCode int) bool {
	if op.Responses == nil {
		return false
	}

	resp := op.Responses.Status(statusCode)
	if resp == nil || resp.Value == nil || resp.Value.Content == nil {
		return false
	}

	for contentType := range resp.Value.Content {
		if strings.Contains(contentType, "json") {
			return true
		}
	}
	return false
}

// getFirstContentType returns the first content type defined for a response status code,
// used in error messages when the content type is not supported.
func getFirstContentType(op *openapi3.Operation, statusCode int) string {
	if op.Responses == nil {
		return ""
	}

	resp := op.Responses.Status(statusCode)
	if resp == nil || resp.Value == nil || resp.Value.Content == nil {
		return ""
	}

	for contentType := range resp.Value.Content {
		return contentType
	}
	return ""
}
