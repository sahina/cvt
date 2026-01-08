package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cvt/cvt-sdk/go/cvt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockValidator is a mock implementation for testing
type mockValidator struct {
	validateFunc func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error)
	callCount    int
	mu           sync.Mutex
	lastRequest  cvt.ValidationRequest
	lastResponse cvt.ValidationResponse
}

func (m *mockValidator) Validate(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
	m.mu.Lock()
	m.callCount++
	m.lastRequest = req
	m.lastResponse = resp
	m.mu.Unlock()

	if m.validateFunc != nil {
		return m.validateFunc(ctx, req, resp)
	}
	return &cvt.ValidationResult{Valid: true}, nil
}

func (m *mockValidator) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// =============================================================================
// Path Filtering Tests
// =============================================================================

func TestPathFiltering(t *testing.T) {
	t.Run("matchesPathFilter with string", func(t *testing.T) {
		assert.True(t, matchesPathFilter("/api/pet/1", "/pet"))
		assert.False(t, matchesPathFilter("/api/user/1", "/pet"))
	})

	t.Run("matchesPathFilter with regex", func(t *testing.T) {
		pattern := regexp.MustCompile(`^/pet`)
		assert.True(t, matchesPathFilter("/pet/1", pattern))
		assert.False(t, matchesPathFilter("/user/1", pattern))
	})

	t.Run("shouldValidatePath with empty filters", func(t *testing.T) {
		assert.True(t, shouldValidatePath("/any/path", nil, nil))
	})

	t.Run("shouldValidatePath with exclude", func(t *testing.T) {
		excludes := []PathFilter{"/health"}
		assert.False(t, shouldValidatePath("/health", nil, excludes))
		assert.True(t, shouldValidatePath("/pet/1", nil, excludes))
	})

	t.Run("shouldValidatePath with include", func(t *testing.T) {
		includes := []PathFilter{regexp.MustCompile(`^/pet`)}
		assert.True(t, shouldValidatePath("/pet/1", includes, nil))
		assert.False(t, shouldValidatePath("/user/1", includes, nil))
	})

	t.Run("shouldValidatePath exclude takes precedence over include", func(t *testing.T) {
		includes := []PathFilter{regexp.MustCompile(`^/pet`)}
		excludes := []PathFilter{"/pet/health"}
		assert.False(t, shouldValidatePath("/pet/health", includes, excludes))
		assert.True(t, shouldValidatePath("/pet/1", includes, excludes))
	})
}

// =============================================================================
// ValidatingRoundTripper Tests
// =============================================================================

func TestValidatingRoundTripper_PanicsWithoutValidator(t *testing.T) {
	assert.Panics(t, func() {
		NewValidatingRoundTripper(RoundTripperConfig{})
	})
}

func TestValidatingRoundTripper_UsesDefaultTransport(t *testing.T) {
	validator := &mockValidator{}
	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator: validator,
	})
	require.NotNil(t, rt)
	assert.NotNil(t, rt.config.Transport)
}

func TestValidatingRoundTripper_CapturesRequestBody(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"received": string(body)})
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}
	reqBody := `{"name":"Fluffy","photoUrls":["http://example.com/photo.jpg"]}`
	req, _ := http.NewRequest("POST", server.URL+"/pet", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Verify interaction was captured
	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)
	assert.Equal(t, "POST", interactions[0].Request.Method)
	assert.NotNil(t, interactions[0].Request.Body)
}

func TestValidatingRoundTripper_CapturesResponseBody(t *testing.T) {
	validator := &mockValidator{}
	expectedBody := map[string]any{"id": float64(1), "name": "Fluffy"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedBody)
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}
	resp, err := client.Get(server.URL + "/pet/1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Verify response body was captured
	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)
	assert.Equal(t, http.StatusOK, interactions[0].Response.StatusCode)
	assert.NotNil(t, interactions[0].Response.Body)
}

func TestValidatingRoundTripper_ExtractsHeaders(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Response", "response-value")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}
	req, _ := http.NewRequest("GET", server.URL+"/pet/1", nil)
	req.Header.Set("X-Custom-Request", "request-value")
	req.Header.Set("Authorization", "Bearer token123")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)

	// Verify request headers
	assert.Equal(t, "request-value", interactions[0].Request.Headers["X-Custom-Request"])
	assert.Equal(t, "Bearer token123", interactions[0].Request.Headers["Authorization"])

	// Verify response headers
	assert.Equal(t, "response-value", interactions[0].Response.Headers["X-Custom-Response"])
}

func TestValidatingRoundTripper_ParsesJSONBody(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 1, "name": "Fluffy", "tags": ["cute", "fluffy"]}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}
	resp, err := client.Get(server.URL + "/pet/1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)

	// Body should be parsed as map
	body, ok := interactions[0].Response.Body.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), body["id"])
	assert.Equal(t, "Fluffy", body["name"])
}

func TestValidatingRoundTripper_HandlesInvalidJSON(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json content"))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}
	resp, err := client.Get(server.URL + "/pet/1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Should not panic, body will be nil when JSON parsing fails
	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)
	assert.Nil(t, interactions[0].Response.Body)
}

func TestValidatingRoundTripper_IncludesQueryString(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}
	resp, err := client.Get(server.URL + "/pet/findByStatus?status=available&limit=10")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)
	assert.Equal(t, "/pet/findByStatus?status=available&limit=10", interactions[0].Request.Path)
}

func TestValidatingRoundTripper_AutoValidateEnabled(t *testing.T) {
	validator := &mockValidator{
		validateFunc: func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
			return &cvt.ValidationResult{Valid: true}, nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}
	resp, err := client.Get(server.URL + "/pet/1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Validator should be called
	assert.Equal(t, 1, validator.getCallCount())

	// ValidationResult should be attached
	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)
	require.NotNil(t, interactions[0].ValidationResult)
	assert.True(t, interactions[0].ValidationResult.Valid)
}

func TestValidatingRoundTripper_AutoValidateDisabled(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: false,
	})

	client := &http.Client{Transport: rt}
	resp, err := client.Get(server.URL + "/pet/1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Validator should NOT be called when AutoValidate is false
	assert.Equal(t, 0, validator.getCallCount())

	// Interaction should still be captured but without validation result
	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)
	assert.Nil(t, interactions[0].ValidationResult)
}

func TestValidatingRoundTripper_CallsOnValidationFailure(t *testing.T) {
	callbackCalled := false
	var callbackResult *cvt.ValidationResult

	validator := &mockValidator{
		validateFunc: func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
			return &cvt.ValidationResult{
				Valid:  false,
				Errors: []string{"Missing required field: name"},
			}, nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
		OnValidationFailure: func(result *cvt.ValidationResult, req *http.Request, resp *http.Response) error {
			callbackCalled = true
			callbackResult = result
			return nil // Don't fail the request
		},
	})

	client := &http.Client{Transport: rt}
	resp, err := client.Get(server.URL + "/pet/1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.True(t, callbackCalled)
	require.NotNil(t, callbackResult)
	assert.False(t, callbackResult.Valid)
	assert.Contains(t, callbackResult.Errors, "Missing required field: name")
}

func TestValidatingRoundTripper_OnValidationFailureReturnsError(t *testing.T) {
	validator := &mockValidator{
		validateFunc: func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
			return &cvt.ValidationResult{Valid: false, Errors: []string{"error"}}, nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	expectedError := errors.New("validation failed")
	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
		OnValidationFailure: func(result *cvt.ValidationResult, req *http.Request, resp *http.Response) error {
			return expectedError
		},
	})

	client := &http.Client{Transport: rt}
	_, err := client.Get(server.URL + "/pet/1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidatingRoundTripper_GetInteractions(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}

	// Make multiple requests
	for i := 0; i < 3; i++ {
		resp, err := client.Get(server.URL + "/pet/1")
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	interactions := rt.GetInteractions()
	assert.Len(t, interactions, 3)
}

func TestValidatingRoundTripper_ClearInteractions(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}

	resp, err := client.Get(server.URL + "/pet/1")
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Len(t, rt.GetInteractions(), 1)

	rt.ClearInteractions()
	assert.Len(t, rt.GetInteractions(), 0)
}

func TestValidatingRoundTripper_HTTPMethods(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
		case "POST":
			w.WriteHeader(http.StatusCreated)
		case "PUT":
			w.WriteHeader(http.StatusOK)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		case "PATCH":
			w.WriteHeader(http.StatusOK)
		}
		if r.Method != "DELETE" {
			_, _ = w.Write([]byte(`{"success": true}`))
		}
	}))
	defer server.Close()

	tests := []struct {
		name           string
		method         string
		body           string
		expectedStatus int
	}{
		{"GET request", "GET", "", http.StatusOK},
		{"POST request", "POST", `{"name":"Fluffy"}`, http.StatusCreated},
		{"PUT request", "PUT", `{"name":"Buddy"}`, http.StatusOK},
		{"DELETE request", "DELETE", "", http.StatusNoContent},
		{"PATCH request", "PATCH", `{"name":"Max"}`, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := NewValidatingRoundTripper(RoundTripperConfig{
				Validator:    validator,
				AutoValidate: true,
			})

			client := &http.Client{Transport: rt}

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			req, _ := http.NewRequest(tt.method, server.URL+"/pet/1", body)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			interactions := rt.GetInteractions()
			require.Len(t, interactions, 1)
			assert.Equal(t, tt.method, interactions[0].Request.Method)
			assert.Equal(t, tt.expectedStatus, interactions[0].Response.StatusCode)
		})
	}
}

func TestValidatingRoundTripper_HandlesErrorResponses(t *testing.T) {
	validator := &mockValidator{}

	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"400 Bad Request", http.StatusBadRequest, `{"error": "Invalid input"}`},
		{"401 Unauthorized", http.StatusUnauthorized, `{"error": "Not authenticated"}`},
		{"403 Forbidden", http.StatusForbidden, `{"error": "Access denied"}`},
		{"404 Not Found", http.StatusNotFound, `{"error": "Pet not found"}`},
		{"500 Internal Server Error", http.StatusInternalServerError, `{"error": "Server error"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			rt := NewValidatingRoundTripper(RoundTripperConfig{
				Validator:    validator,
				AutoValidate: true,
			})

			client := &http.Client{Transport: rt}
			resp, err := client.Get(server.URL + "/pet/1")
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			interactions := rt.GetInteractions()
			require.Len(t, interactions, 1)
			assert.Equal(t, tt.statusCode, interactions[0].Response.StatusCode)
		})
	}
}

func TestValidatingRoundTripper_PathFiltering(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
		IncludePaths: []PathFilter{regexp.MustCompile(`^/pet`)},
		ExcludePaths: []PathFilter{"/health"},
	})

	client := &http.Client{Transport: rt}

	// This should be captured (matches include pattern)
	resp1, _ := client.Get(server.URL + "/pet/1")
	_ = resp1.Body.Close()

	// This should NOT be captured (doesn't match include pattern)
	resp2, _ := client.Get(server.URL + "/user/1")
	_ = resp2.Body.Close()

	// Validator should only be called once (for /pet/1)
	assert.Equal(t, 1, validator.getCallCount())
	assert.Len(t, rt.GetInteractions(), 1)
}

func TestValidatingRoundTripper_ConcurrentRequests(t *testing.T) {
	validator := &mockValidator{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Small delay
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	rt := NewValidatingRoundTripper(RoundTripperConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	client := &http.Client{Transport: rt}

	var wg sync.WaitGroup
	numRequests := 20

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(server.URL + "/pet/1")
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
	}

	wg.Wait()

	// All interactions should be captured
	interactions := rt.GetInteractions()
	assert.Len(t, interactions, numRequests)
}

// =============================================================================
// ValidatingMiddleware Tests
// =============================================================================

func TestValidatingMiddleware_PanicsWithoutValidator(t *testing.T) {
	assert.Panics(t, func() {
		NewValidatingMiddleware(MiddlewareConfig{})
	})
}

func TestValidatingMiddleware_HandlerCaptures(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 1, "name": "Fluffy"}`))
	})

	wrapped := middleware.Handler(handler)

	req := httptest.NewRequest("GET", "/pet/1", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	interactions := middleware.GetInteractions()
	require.Len(t, interactions, 1)
	assert.Equal(t, "GET", interactions[0].Request.Method)
	assert.Equal(t, "/pet/1", interactions[0].Request.Path)
	assert.Equal(t, http.StatusOK, interactions[0].Response.StatusCode)
}

func TestValidatingMiddleware_HandlerFuncCaptures(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}

	wrapped := middleware.HandlerFunc(handlerFunc)

	req := httptest.NewRequest("POST", "/pet", strings.NewReader(`{"name":"Buddy"}`))
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	interactions := middleware.GetInteractions()
	require.Len(t, interactions, 1)
	assert.Equal(t, "POST", interactions[0].Request.Method)
}

func TestValidatingMiddleware_RequestBodyCapture(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	var capturedBody []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler should still be able to read the body
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	wrapped := middleware.Handler(handler)

	reqBody := `{"name":"Fluffy","photoUrls":["http://example.com/photo.jpg"]}`
	req := httptest.NewRequest("POST", "/pet", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Handler should receive the body
	assert.Equal(t, reqBody, string(capturedBody))

	// Interaction should capture the body
	interactions := middleware.GetInteractions()
	require.Len(t, interactions, 1)
	assert.NotNil(t, interactions[0].Request.Body)
}

func TestValidatingMiddleware_ResponseBodyCapture(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	responseBody := map[string]any{"id": float64(1), "name": "Fluffy"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(responseBody)
	})

	wrapped := middleware.Handler(handler)

	req := httptest.NewRequest("GET", "/pet/1", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	interactions := middleware.GetInteractions()
	require.Len(t, interactions, 1)

	body, ok := interactions[0].Response.Body.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Fluffy", body["name"])
}

func TestValidatingMiddleware_AutoValidation(t *testing.T) {
	validator := &mockValidator{
		validateFunc: func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
			return &cvt.ValidationResult{Valid: true}, nil
		},
	}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	wrapped := middleware.Handler(handler)

	req := httptest.NewRequest("GET", "/pet/1", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, 1, validator.getCallCount())

	interactions := middleware.GetInteractions()
	require.Len(t, interactions, 1)
	require.NotNil(t, interactions[0].ValidationResult)
	assert.True(t, interactions[0].ValidationResult.Valid)
}

func TestValidatingMiddleware_OnValidationFailure(t *testing.T) {
	callbackCalled := false

	validator := &mockValidator{
		validateFunc: func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
			return &cvt.ValidationResult{
				Valid:  false,
				Errors: []string{"Invalid response"},
			}, nil
		},
	}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
		OnValidationFailure: func(w http.ResponseWriter, r *http.Request, result *cvt.ValidationResult) bool {
			callbackCalled = true
			return true
		},
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	wrapped := middleware.Handler(handler)

	req := httptest.NewRequest("GET", "/pet/1", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.True(t, callbackCalled)
}

func TestValidatingMiddleware_PathFiltering(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
		IncludePaths: []PathFilter{regexp.MustCompile(`^/pet`)},
		ExcludePaths: []PathFilter{"/health"},
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	wrapped := middleware.Handler(handler)

	// Should be captured
	req1 := httptest.NewRequest("GET", "/pet/1", nil)
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	// Should NOT be captured (doesn't match include)
	req2 := httptest.NewRequest("GET", "/user/1", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	// Should NOT be captured (excluded)
	req3 := httptest.NewRequest("GET", "/health", nil)
	rec3 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec3, req3)

	// Only /pet/1 should be captured
	assert.Len(t, middleware.GetInteractions(), 1)
	assert.Equal(t, 1, validator.getCallCount())
}

func TestValidatingMiddleware_ConcurrentRequests(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	wrapped := middleware.Handler(handler)

	var wg sync.WaitGroup
	numRequests := 20

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/pet/1", nil)
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)
		}()
	}

	wg.Wait()

	interactions := middleware.GetInteractions()
	assert.Len(t, interactions, numRequests)
}

func TestValidatingMiddleware_GetInteractions(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	wrapped := middleware.Handler(handler)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/pet/1", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	interactions := middleware.GetInteractions()
	assert.Len(t, interactions, 5)
}

func TestValidatingMiddleware_ClearInteractions(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	wrapped := middleware.Handler(handler)

	req := httptest.NewRequest("GET", "/pet/1", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Len(t, middleware.GetInteractions(), 1)

	middleware.ClearInteractions()
	assert.Len(t, middleware.GetInteractions(), 0)
}

func TestValidatingMiddleware_QueryStringCapture(t *testing.T) {
	validator := &mockValidator{}

	middleware := NewValidatingMiddleware(MiddlewareConfig{
		Validator:    validator,
		AutoValidate: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	wrapped := middleware.Handler(handler)

	req := httptest.NewRequest("GET", "/pet/findByStatus?status=available&limit=10", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	interactions := middleware.GetInteractions()
	require.Len(t, interactions, 1)
	assert.Equal(t, "/pet/findByStatus?status=available&limit=10", interactions[0].Request.Path)
}

// =============================================================================
// CapturedInteraction Tests
// =============================================================================

func TestCapturedInteraction(t *testing.T) {
	interaction := CapturedInteraction{
		Request: cvt.ValidationRequest{
			Method:  "GET",
			Path:    "/pet/1",
			Headers: map[string]string{"Accept": "application/json"},
		},
		Response: cvt.ValidationResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		},
		Timestamp: time.Now(),
	}

	assert.Equal(t, "GET", interaction.Request.Method)
	assert.Equal(t, "/pet/1", interaction.Request.Path)
	assert.Equal(t, "application/json", interaction.Request.Headers["Accept"])
	assert.Equal(t, 200, interaction.Response.StatusCode)
	assert.Equal(t, "application/json", interaction.Response.Headers["Content-Type"])
	assert.False(t, interaction.Timestamp.IsZero())
}

// =============================================================================
// ResponseWriter Tests
// =============================================================================

func TestResponseWriter_CapturesStatusCode(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	rw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, rw.statusCode)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestResponseWriter_CapturesBody(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	body := `{"id": 1, "name": "Fluffy"}`
	n, err := rw.Write([]byte(body))

	require.NoError(t, err)
	assert.Equal(t, len(body), n)
	assert.Equal(t, body, rw.body.String())
	assert.Equal(t, body, rec.Body.String())
}

func TestResponseWriter_MultipleWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
		body:           bytes.Buffer{},
	}

	_, _ = rw.Write([]byte(`{"id": 1`))
	_, _ = rw.Write([]byte(`, "name": "Fluffy"}`))

	expected := `{"id": 1, "name": "Fluffy"}`
	assert.Equal(t, expected, rw.body.String())
}

// =============================================================================
// Extract Functions Tests
// =============================================================================

func TestExtractRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/pet?name=fluffy&status=available", strings.NewReader(`{"id": 1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")

	body := []byte(`{"id": 1}`)

	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/pet", req.URL.Path)
	assert.Equal(t, "name=fluffy&status=available", req.URL.RawQuery)
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, "Bearer token123", headers["Authorization"])
	assert.NotEmpty(t, body)
}

func TestExtractResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.Header().Set("X-Request-Id", "abc123")
	rec.WriteHeader(http.StatusCreated)
	_, _ = rec.Write([]byte(`{"id": 1}`))

	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusCreated,
	}
	rw.body.Write([]byte(`{"id": 1}`))

	assert.Equal(t, http.StatusCreated, rw.statusCode)
	assert.Equal(t, `{"id": 1}`, rw.body.String())
}
