package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/sahina/cvt/sdks/go/cvt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMockingValidator implements MockingValidator for testing
type mockMockingValidator struct {
	validateFunc         func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error)
	generateResponseFunc func(ctx context.Context, method, path string, opts *cvt.GenerateOptions) (*cvt.GeneratedResponse, error)
	callCount            int
	generateCallCount    int
	mu                   sync.Mutex
}

func (m *mockMockingValidator) Validate(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.validateFunc != nil {
		return m.validateFunc(ctx, req, resp)
	}
	return &cvt.ValidationResult{Valid: true}, nil
}

func (m *mockMockingValidator) GenerateResponse(ctx context.Context, method, path string, opts *cvt.GenerateOptions) (*cvt.GeneratedResponse, error) {
	m.mu.Lock()
	m.generateCallCount++
	m.mu.Unlock()

	if m.generateResponseFunc != nil {
		return m.generateResponseFunc(ctx, method, path, opts)
	}
	return &cvt.GeneratedResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       map[string]any{"id": "123", "name": "Test User"},
	}, nil
}

func (m *mockMockingValidator) getGenerateCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generateCallCount
}

// =============================================================================
// MockingRoundTripper Tests
// =============================================================================

func TestMockingRoundTripper_PanicsWithoutValidator(t *testing.T) {
	assert.Panics(t, func() {
		NewMockingRoundTripper(MockingRoundTripperConfig{})
	})
}

func TestMockingRoundTripper_BasicMocking(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator: validator,
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("GET", "http://mock.api/users/123", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)
	assert.Equal(t, "123", data["id"])
	assert.Equal(t, "Test User", data["name"])
}

func TestMockingRoundTripper_RecordsInteractions(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator: validator,
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("GET", "http://mock.api/users/123", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)

	assert.Equal(t, "GET", interactions[0].Request.Method)
	assert.Equal(t, "http://mock.api/users/123", interactions[0].Request.Path)
	assert.Equal(t, 200, interactions[0].Response.StatusCode)
}

func TestMockingRoundTripper_ClearInteractions(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator: validator,
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("GET", "http://mock.api/users/123", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Len(t, rt.GetInteractions(), 1)

	rt.ClearInteractions()

	assert.Len(t, rt.GetInteractions(), 0)
}

func TestMockingRoundTripper_CachesResponses(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator:      validator,
		CacheResponses: true,
	})

	client := &http.Client{Transport: rt}

	// Make two requests to the same endpoint
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest("GET", "http://mock.api/users/123", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	// GenerateResponse should only be called once due to caching
	assert.Equal(t, 1, validator.getGenerateCallCount())
}

func TestMockingRoundTripper_NoCacheByDefault(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator: validator,
		// CacheResponses: false (default)
	})

	client := &http.Client{Transport: rt}

	// Make two requests to the same endpoint
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest("GET", "http://mock.api/users/123", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	// GenerateResponse should be called twice without caching
	assert.Equal(t, 2, validator.getGenerateCallCount())
}

func TestMockingRoundTripper_ClearCache(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator:      validator,
		CacheResponses: true,
	})

	client := &http.Client{Transport: rt}

	// First request
	req, err := http.NewRequest("GET", "http://mock.api/users/123", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// Clear cache
	rt.ClearCache()

	// Second request after cache clear
	resp, err = client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// GenerateResponse should be called twice (cache was cleared)
	assert.Equal(t, 2, validator.getGenerateCallCount())
}

func TestMockingRoundTripper_ValidatesRequests(t *testing.T) {
	validator := &mockMockingValidator{
		validateFunc: func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
			return &cvt.ValidationResult{
				Valid:  false,
				Errors: []string{"invalid request body"},
			}, nil
		},
	}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator:        validator,
		ValidateRequests: true,
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("POST", "http://mock.api/users", strings.NewReader(`{"invalid": true}`))
	require.NoError(t, err)

	_, err = client.Do(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request validation failed")
}

func TestMockingRoundTripper_RequestValidationCallback(t *testing.T) {
	validator := &mockMockingValidator{
		validateFunc: func(ctx context.Context, req cvt.ValidationRequest, resp cvt.ValidationResponse) (*cvt.ValidationResult, error) {
			return &cvt.ValidationResult{
				Valid:  false,
				Errors: []string{"invalid request"},
			}, nil
		},
	}

	customErr := errors.New("custom validation error")
	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator:        validator,
		ValidateRequests: true,
		OnRequestValidationFailure: func(result *cvt.ValidationResult, req *http.Request) error {
			return customErr
		},
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("GET", "http://mock.api/users/123", nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	assert.ErrorIs(t, err, customErr)
}

func TestMockingRoundTripper_ExcludePaths(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator:    validator,
		ExcludePaths: []PathFilter{"/health"},
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("GET", "http://mock.api/health", nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "excluded from mocking")
}

func TestMockingRoundTripper_IncludePaths(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator:    validator,
		IncludePaths: []PathFilter{"/api/"},
	})

	client := &http.Client{Transport: rt}

	// Included path should work
	req, err := http.NewRequest("GET", "http://mock.api/api/users/123", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	// Non-included path should fail
	req, err = http.NewRequest("GET", "http://mock.api/other/path", nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	assert.Error(t, err)
}

func TestMockingRoundTripper_GenerateResponseError(t *testing.T) {
	validator := &mockMockingValidator{
		generateResponseFunc: func(ctx context.Context, method, path string, opts *cvt.GenerateOptions) (*cvt.GeneratedResponse, error) {
			return nil, errors.New("schema not found")
		},
	}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator: validator,
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("GET", "http://mock.api/users/123", nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate mock response")
}

func TestMockingRoundTripper_CustomStatusCode(t *testing.T) {
	validator := &mockMockingValidator{
		generateResponseFunc: func(ctx context.Context, method, path string, opts *cvt.GenerateOptions) (*cvt.GeneratedResponse, error) {
			return &cvt.GeneratedResponse{
				StatusCode: 201,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       map[string]any{"id": "new-123"},
			}, nil
		},
	}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator: validator,
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("POST", "http://mock.api/users", strings.NewReader(`{"name": "New User"}`))
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 201, resp.StatusCode)
}

func TestMockingRoundTripper_WithQueryString(t *testing.T) {
	validator := &mockMockingValidator{
		generateResponseFunc: func(ctx context.Context, method, path string, opts *cvt.GenerateOptions) (*cvt.GeneratedResponse, error) {
			// Verify path includes query string
			assert.Contains(t, path, "?")
			assert.Contains(t, path, "status=active")
			return &cvt.GeneratedResponse{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       []map[string]any{{"id": "1"}, {"id": "2"}},
			}, nil
		},
	}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator: validator,
	})

	client := &http.Client{Transport: rt}

	req, err := http.NewRequest("GET", "http://mock.api/users?status=active&limit=10", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}

func TestMockingRoundTripper_CapturesRequestBody(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator: validator,
	})

	client := &http.Client{Transport: rt}

	reqBody := `{"name": "Test User", "email": "test@example.com"}`
	req, err := http.NewRequest("POST", "http://mock.api/users", strings.NewReader(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	interactions := rt.GetInteractions()
	require.Len(t, interactions, 1)

	body := interactions[0].Request.Body.(map[string]any)
	assert.Equal(t, "Test User", body["name"])
	assert.Equal(t, "test@example.com", body["email"])
}

func TestMockingRoundTripper_ConcurrentRequests(t *testing.T) {
	validator := &mockMockingValidator{}

	rt := NewMockingRoundTripper(MockingRoundTripperConfig{
		Validator:      validator,
		CacheResponses: true,
	})

	client := &http.Client{Transport: rt}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", "http://mock.api/users/123", nil)
			resp, err := client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	// Should have recorded 10 interactions
	interactions := rt.GetInteractions()
	assert.Len(t, interactions, 10)
}

// =============================================================================
// Simplified API Tests (NewMockClient, NewMock, Options)
// =============================================================================

func TestNewMockClient_Basic(t *testing.T) {
	validator := &mockMockingValidator{}

	client := NewMockClient(validator)
	require.NotNil(t, client)

	req, _ := http.NewRequest("GET", "http://mock.api/users/123", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 200, resp.StatusCode)
}

func TestNewMockClient_WithCache(t *testing.T) {
	validator := &mockMockingValidator{}

	client := NewMockClient(validator, WithCache())
	require.NotNil(t, client)

	// Make two requests
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", "http://mock.api/users/123", nil)
		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	// Should only generate once due to caching
	assert.Equal(t, 1, validator.getGenerateCallCount())
}

func TestNewMockClient_WithMultipleOptions(t *testing.T) {
	validator := &mockMockingValidator{}

	client := NewMockClient(validator, WithCache(), WithExcludePaths("/health"))
	require.NotNil(t, client)

	// Normal path works
	req, _ := http.NewRequest("GET", "http://mock.api/users/123", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// Excluded path fails
	req, _ = http.NewRequest("GET", "http://mock.api/health", nil)
	_, err = client.Do(req)
	assert.Error(t, err)
}

func TestNewMock_Basic(t *testing.T) {
	validator := &mockMockingValidator{}

	mock := NewMock(validator)
	require.NotNil(t, mock)

	client := mock.Client()
	require.NotNil(t, client)

	req, _ := http.NewRequest("GET", "http://mock.api/users/123", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// Check interactions
	interactions := mock.GetInteractions()
	assert.Len(t, interactions, 1)
	assert.Equal(t, "GET", interactions[0].Request.Method)
}

func TestNewMock_ClearInteractions(t *testing.T) {
	validator := &mockMockingValidator{}

	mock := NewMock(validator)
	client := mock.Client()

	req, _ := http.NewRequest("GET", "http://mock.api/users/123", nil)
	resp, _ := client.Do(req)
	_ = resp.Body.Close()

	assert.Len(t, mock.GetInteractions(), 1)

	mock.ClearInteractions()
	assert.Len(t, mock.GetInteractions(), 0)
}

func TestNewMock_ClearCache(t *testing.T) {
	validator := &mockMockingValidator{}

	mock := NewMock(validator, WithCache())
	client := mock.Client()

	// First request
	req, _ := http.NewRequest("GET", "http://mock.api/users/123", nil)
	resp, _ := client.Do(req)
	_ = resp.Body.Close()

	// Clear cache
	mock.ClearCache()

	// Second request after cache clear
	resp, _ = client.Do(req)
	_ = resp.Body.Close()

	// Should have called generate twice
	assert.Equal(t, 2, validator.getGenerateCallCount())
}

func TestWithIncludePaths(t *testing.T) {
	validator := &mockMockingValidator{}

	client := NewMockClient(validator, WithIncludePaths("/api/"))

	// Included path works
	req, _ := http.NewRequest("GET", "http://mock.api/api/users/123", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// Non-included path fails
	req, _ = http.NewRequest("GET", "http://mock.api/other/path", nil)
	_, err = client.Do(req)
	assert.Error(t, err)
}

func TestWithGenerateOptions(t *testing.T) {
	validator := &mockMockingValidator{
		generateResponseFunc: func(ctx context.Context, method, path string, opts *cvt.GenerateOptions) (*cvt.GeneratedResponse, error) {
			// Verify options are passed through
			assert.NotNil(t, opts)
			assert.Equal(t, 201, opts.StatusCode)
			return &cvt.GeneratedResponse{
				StatusCode: 201,
				Body:       map[string]any{"created": true},
			}, nil
		},
	}

	client := NewMockClient(validator, WithGenerateOptions(&cvt.GenerateOptions{
		StatusCode: 201,
	}))

	req, _ := http.NewRequest("POST", "http://mock.api/users", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, 201, resp.StatusCode)
}
