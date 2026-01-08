package cvt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFieldsFromBody(t *testing.T) {
	tests := []struct {
		name     string
		body     any
		prefix   string
		expected []string
	}{
		{
			name:     "nil body",
			body:     nil,
			prefix:   "",
			expected: nil,
		},
		{
			name: "flat object",
			body: map[string]any{
				"id":    "123",
				"name":  "John",
				"email": "john@example.com",
			},
			prefix:   "",
			expected: []string{"id", "name", "email"},
		},
		{
			name: "nested object",
			body: map[string]any{
				"id": "123",
				"address": map[string]any{
					"city": "NYC",
					"zip":  "10001",
				},
			},
			prefix:   "",
			expected: []string{"id", "address", "address.city", "address.zip"},
		},
		{
			name: "deeply nested",
			body: map[string]any{
				"user": map[string]any{
					"profile": map[string]any{
						"name": "John",
					},
				},
			},
			prefix:   "",
			expected: []string{"user", "user.profile", "user.profile.name"},
		},
		{
			name: "array with objects",
			body: []any{
				map[string]any{
					"id":   "1",
					"name": "Item 1",
				},
			},
			prefix:   "",
			expected: []string{"id", "name"},
		},
		{
			name: "with prefix",
			body: map[string]any{
				"city": "NYC",
			},
			prefix:   "address",
			expected: []string{"address.city"},
		},
		{
			name:     "primitive value",
			body:     "string value",
			prefix:   "",
			expected: nil,
		},
		{
			name:     "empty map",
			body:     map[string]any{},
			prefix:   "",
			expected: nil,
		},
		{
			name:     "empty array",
			body:     []any{},
			prefix:   "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFieldsFromBody(tt.body, tt.prefix)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestExtractSchemaIDFromURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expected    string
		expectError bool
	}{
		{
			name:     "mock prefix",
			url:      "http://mock.user-api/users/123",
			expected: "user-api",
		},
		{
			name:     "mock prefix with subdomain",
			url:      "http://mock.my-service/api/v1/items",
			expected: "my-service",
		},
		{
			name:     "no mock prefix",
			url:      "http://api.example.com/users",
			expected: "api.example.com",
		},
		{
			name:     "https",
			url:      "https://mock.secure-api/data",
			expected: "secure-api",
		},
		{
			name:     "with port",
			url:      "http://mock.test-api:8080/endpoint",
			expected: "test-api",
		},
		{
			name:        "invalid URL",
			url:         "not a url",
			expectError: true,
		},
		{
			name:        "empty hostname",
			url:         "http:///path",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractSchemaIDFromURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestNormalizePathForEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "/users/123",
			expected: "/users/123",
		},
		{
			name:     "path with query string",
			input:    "/users?page=1&limit=10",
			expected: "/users",
		},
		{
			name:     "full URL",
			input:    "http://mock.user-api/users/123",
			expected: "/users/123",
		},
		{
			name:     "full URL with query",
			input:    "http://mock.user-api/users?active=true",
			expected: "/users",
		},
		{
			name:     "https URL",
			input:    "https://mock.api/data/items",
			expected: "/data/items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePathForEndpoint(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeStringSlices(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "no overlap",
			a:        []string{"a", "b"},
			b:        []string{"c", "d"},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "with overlap",
			a:        []string{"a", "b", "c"},
			b:        []string{"b", "c", "d"},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "empty first",
			a:        []string{},
			b:        []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "empty second",
			a:        []string{"a", "b"},
			b:        []string{},
			expected: []string{"a", "b"},
		},
		{
			name:     "both empty",
			a:        []string{},
			b:        []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeStringSlices(tt.a, tt.b)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestExtractSchemaIDFromInteractions(t *testing.T) {
	tests := []struct {
		name         string
		interactions []CapturedInteraction
		expected     string
		expectError  bool
		errorMsg     string
	}{
		{
			name: "single schema",
			interactions: []CapturedInteraction{
				{
					Request:   ValidationRequest{Method: "GET", Path: "http://mock.user-api/users/123"},
					Response:  ValidationResponse{StatusCode: 200},
					Timestamp: time.Now(),
				},
			},
			expected: "user-api",
		},
		{
			name: "multiple interactions same schema",
			interactions: []CapturedInteraction{
				{
					Request:   ValidationRequest{Method: "GET", Path: "http://mock.user-api/users/123"},
					Response:  ValidationResponse{StatusCode: 200},
					Timestamp: time.Now(),
				},
				{
					Request:   ValidationRequest{Method: "POST", Path: "http://mock.user-api/users"},
					Response:  ValidationResponse{StatusCode: 201},
					Timestamp: time.Now(),
				},
			},
			expected: "user-api",
		},
		{
			name: "multiple different schemas",
			interactions: []CapturedInteraction{
				{
					Request:   ValidationRequest{Method: "GET", Path: "http://mock.user-api/users/123"},
					Response:  ValidationResponse{StatusCode: 200},
					Timestamp: time.Now(),
				},
				{
					Request:   ValidationRequest{Method: "GET", Path: "http://mock.order-api/orders/456"},
					Response:  ValidationResponse{StatusCode: 200},
					Timestamp: time.Now(),
				},
			},
			expectError: true,
			errorMsg:    "multiple schemas detected",
		},
		{
			name: "no URLs in paths",
			interactions: []CapturedInteraction{
				{
					Request:   ValidationRequest{Method: "GET", Path: "/users/123"},
					Response:  ValidationResponse{StatusCode: 200},
					Timestamp: time.Now(),
				},
			},
			expectError: true,
			errorMsg:    "could not extract schemaID",
		},
		{
			name:         "empty interactions",
			interactions: []CapturedInteraction{},
			expectError:  true,
			errorMsg:     "no interactions to register",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "empty interactions" {
				// This is tested in BuildConsumerFromInteractions
				return
			}
			result, err := extractSchemaIDFromInteractions(tt.interactions)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMergeInteractionsToEndpoints(t *testing.T) {
	interactions := []CapturedInteraction{
		{
			Request: ValidationRequest{
				Method: "GET",
				Path:   "http://mock.user-api/users/123",
			},
			Response: ValidationResponse{
				StatusCode: 200,
				Body: map[string]any{
					"id":   "123",
					"name": "John",
				},
			},
			Timestamp: time.Now(),
		},
		{
			Request: ValidationRequest{
				Method: "GET",
				Path:   "http://mock.user-api/users/456",
			},
			Response: ValidationResponse{
				StatusCode: 200,
				Body: map[string]any{
					"id":    "456",
					"name":  "Jane",
					"email": "jane@example.com",
				},
			},
			Timestamp: time.Now(),
		},
		{
			Request: ValidationRequest{
				Method: "POST",
				Path:   "http://mock.user-api/users",
			},
			Response: ValidationResponse{
				StatusCode: 201,
				Body: map[string]any{
					"id": "789",
				},
			},
			Timestamp: time.Now(),
		},
	}

	endpoints := mergeInteractionsToEndpoints(interactions)

	// Should have 2 unique endpoints (GET /users/{id} merged, POST /users)
	// Note: Since we're using actual path values, they'll be distinct paths
	// In real usage, the paths should be templates like /users/{id}
	assert.Len(t, endpoints, 3) // GET /users/123, GET /users/456, POST /users

	// Check that POST /users has only "id" field
	var postEndpoint *EndpointUsage
	for i := range endpoints {
		if endpoints[i].Method == "POST" && endpoints[i].Path == "/users" {
			postEndpoint = &endpoints[i]
			break
		}
	}
	require.NotNil(t, postEndpoint)
	assert.ElementsMatch(t, []string{"id"}, postEndpoint.UsedFields)
}

func TestBuildConsumerFromInteractions_Validation(t *testing.T) {
	ctx := context.Background()
	v := &Validator{}

	interactions := []CapturedInteraction{
		{
			Request:   ValidationRequest{Method: "GET", Path: "http://mock.user-api/users/123"},
			Response:  ValidationResponse{StatusCode: 200, Body: map[string]any{"id": "123"}},
			Timestamp: time.Now(),
		},
	}

	tests := []struct {
		name     string
		config   AutoRegisterConfig
		errorMsg string
	}{
		{
			name: "missing ConsumerID",
			config: AutoRegisterConfig{
				ConsumerVersion: "1.0.0",
				Environment:     "dev",
				SchemaVersion:   "1.0.0",
			},
			errorMsg: "consumerID is required",
		},
		{
			name: "missing ConsumerVersion",
			config: AutoRegisterConfig{
				ConsumerID:    "test-service",
				Environment:   "dev",
				SchemaVersion: "1.0.0",
			},
			errorMsg: "consumerVersion is required",
		},
		{
			name: "missing Environment",
			config: AutoRegisterConfig{
				ConsumerID:      "test-service",
				ConsumerVersion: "1.0.0",
				SchemaVersion:   "1.0.0",
			},
			errorMsg: "environment is required",
		},
		{
			name: "missing SchemaVersion",
			config: AutoRegisterConfig{
				ConsumerID:      "test-service",
				ConsumerVersion: "1.0.0",
				Environment:     "dev",
			},
			errorMsg: "schemaVersion is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.BuildConsumerFromInteractions(ctx, interactions, tt.config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestBuildConsumerFromInteractions_EmptyInteractions(t *testing.T) {
	ctx := context.Background()
	v := &Validator{}

	config := AutoRegisterConfig{
		ConsumerID:      "test-service",
		ConsumerVersion: "1.0.0",
		Environment:     "dev",
		SchemaVersion:   "1.0.0",
	}

	_, err := v.BuildConsumerFromInteractions(ctx, []CapturedInteraction{}, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no interactions to register")
}

func TestBuildConsumerFromInteractions_Success(t *testing.T) {
	ctx := context.Background()
	v := &Validator{}

	interactions := []CapturedInteraction{
		{
			Request: ValidationRequest{
				Method: "GET",
				Path:   "http://mock.user-api/users/123",
			},
			Response: ValidationResponse{
				StatusCode: 200,
				Body: map[string]any{
					"id":    "123",
					"name":  "John",
					"email": "john@example.com",
				},
			},
			Timestamp: time.Now(),
		},
		{
			Request: ValidationRequest{
				Method: "POST",
				Path:   "http://mock.user-api/users",
			},
			Response: ValidationResponse{
				StatusCode: 201,
				Body: map[string]any{
					"id": "456",
				},
			},
			Timestamp: time.Now(),
		},
	}

	config := AutoRegisterConfig{
		ConsumerID:      "order-service",
		ConsumerVersion: "2.1.0",
		Environment:     "dev",
		SchemaVersion:   "1.0.0",
	}

	opts, err := v.BuildConsumerFromInteractions(ctx, interactions, config)
	require.NoError(t, err)

	assert.Equal(t, "order-service", opts.ConsumerID)
	assert.Equal(t, "2.1.0", opts.ConsumerVersion)
	assert.Equal(t, "user-api", opts.SchemaID) // Auto-extracted
	assert.Equal(t, "1.0.0", opts.SchemaVersion)
	assert.Equal(t, "dev", opts.Environment)
	assert.Len(t, opts.UsedEndpoints, 2)

	// Verify endpoints
	endpointMap := make(map[string]EndpointUsage)
	for _, ep := range opts.UsedEndpoints {
		endpointMap[ep.Method+":"+ep.Path] = ep
	}

	getEndpoint, ok := endpointMap["GET:/users/123"]
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"id", "name", "email"}, getEndpoint.UsedFields)

	postEndpoint, ok := endpointMap["POST:/users"]
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"id"}, postEndpoint.UsedFields)
}

func TestBuildConsumerFromInteractions_ExplicitSchemaID(t *testing.T) {
	ctx := context.Background()
	v := &Validator{}

	interactions := []CapturedInteraction{
		{
			Request:   ValidationRequest{Method: "GET", Path: "/users/123"}, // No URL, just path
			Response:  ValidationResponse{StatusCode: 200, Body: map[string]any{"id": "123"}},
			Timestamp: time.Now(),
		},
	}

	config := AutoRegisterConfig{
		ConsumerID:      "order-service",
		ConsumerVersion: "1.0.0",
		Environment:     "dev",
		SchemaVersion:   "1.0.0",
		SchemaID:        "my-custom-schema", // Explicit override
	}

	opts, err := v.BuildConsumerFromInteractions(ctx, interactions, config)
	require.NoError(t, err)
	assert.Equal(t, "my-custom-schema", opts.SchemaID)
}

func TestBuildConsumerFromInteractions_NestedResponseFields(t *testing.T) {
	ctx := context.Background()
	v := &Validator{}

	interactions := []CapturedInteraction{
		{
			Request: ValidationRequest{
				Method: "GET",
				Path:   "http://mock.user-api/users/123",
			},
			Response: ValidationResponse{
				StatusCode: 200,
				Body: map[string]any{
					"id": "123",
					"address": map[string]any{
						"city": "NYC",
						"zip":  "10001",
					},
					"tags": []any{"premium", "active"},
				},
			},
			Timestamp: time.Now(),
		},
	}

	config := AutoRegisterConfig{
		ConsumerID:      "order-service",
		ConsumerVersion: "1.0.0",
		Environment:     "dev",
		SchemaVersion:   "1.0.0",
	}

	opts, err := v.BuildConsumerFromInteractions(ctx, interactions, config)
	require.NoError(t, err)

	assert.Len(t, opts.UsedEndpoints, 1)
	fields := opts.UsedEndpoints[0].UsedFields

	// Should include nested fields with dot notation
	assert.Contains(t, fields, "id")
	assert.Contains(t, fields, "address")
	assert.Contains(t, fields, "address.city")
	assert.Contains(t, fields, "address.zip")
	// Arrays of primitives don't add fields (just the container if it was an object array)
}
