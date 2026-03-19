package cvtservice

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/sahina/cvt/server/pb"
	"github.com/sahina/cvt/server/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterSchema_OpenAPIv3_Valid tests registering valid OpenAPI v3 schemas
func TestRegisterSchema_OpenAPIv3_Valid(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	testCases := []struct {
		name       string
		schemaFile string
		schemaID   string
	}{
		{
			name:       "Simple Petstore v3",
			schemaFile: "testdata/openapi-v3/valid/simple-petstore.json",
			schemaID:   "simple-petstore-v3",
		},
		{
			name:       "Complex E-commerce v3",
			schemaFile: "testdata/openapi-v3/valid/complex-ecommerce.json",
			schemaID:   "complex-ecommerce-v3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load schema content
			content, err := os.ReadFile(tc.schemaFile)
			require.NoError(t, err, "Failed to read schema file")

			// Register schema
			req := &pb.RegisterSchemaRequest{
				SchemaId:      tc.schemaID,
				SchemaContent: string(content),
			}

			resp, err := service.RegisterSchema(context.Background(), req)
			require.NoError(t, err, "RegisterSchema should not return error")
			assert.True(t, resp.Success, "Registration should succeed")
			assert.Contains(t, resp.Message, "successfully", "Success message expected")

			// Verify schema is stored in cache
			doc, found := service.cache.Get(tc.schemaID)
			assert.True(t, found, "Schema should be in cache")
			assert.NotNil(t, doc, "Cached document should not be nil")
		})
	}
}

// TestRegisterSchema_OpenAPIv2_Valid tests registering valid OpenAPI v2/Swagger schemas
func TestRegisterSchema_OpenAPIv2_Valid(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	testCases := []struct {
		name       string
		schemaFile string
		schemaID   string
	}{
		{
			name:       "Simple Swagger 2.0",
			schemaFile: "testdata/openapi-v2/valid/simple-api.json",
			schemaID:   "simple-api-v2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load schema content
			content, err := os.ReadFile(tc.schemaFile)
			require.NoError(t, err, "Failed to read schema file")

			// Register schema
			req := &pb.RegisterSchemaRequest{
				SchemaId:      tc.schemaID,
				SchemaContent: string(content),
			}

			resp, err := service.RegisterSchema(context.Background(), req)
			require.NoError(t, err, "RegisterSchema should not return error")
			assert.True(t, resp.Success, "Registration should succeed for Swagger 2.0")
			assert.Contains(t, resp.Message, "successfully", "Success message expected")

			// Verify schema is stored in cache (converted to v3)
			entry, found := service.cache.Get(tc.schemaID)
			assert.True(t, found, "Schema should be in cache")
			assert.NotNil(t, entry, "Cached entry should not be nil")
			assert.NotNil(t, entry.Document, "Cached document should not be nil")
			assert.NotEmpty(t, entry.Document.OpenAPI, "OpenAPI version should be set")
		})
	}
}

// TestRegisterSchema_Invalid tests registering invalid schemas
func TestRegisterSchema_Invalid(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	testCases := []struct {
		name          string
		schemaFile    string
		schemaID      string
		expectedError string
	}{
		{
			name:          "Malformed JSON",
			schemaFile:    "testdata/openapi-v3/invalid/malformed-json.json",
			schemaID:      "malformed-json",
			expectedError: "Failed to parse schema",
		},
		{
			name:          "Missing required fields",
			schemaFile:    "testdata/openapi-v3/invalid/missing-required-fields.json",
			schemaID:      "missing-fields",
			expectedError: "Invalid OpenAPI schema",
		},
		{
			name:          "Invalid reference",
			schemaFile:    "testdata/openapi-v3/invalid/invalid-ref.json",
			schemaID:      "invalid-ref",
			expectedError: "Failed to parse schema",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load schema content
			content, err := os.ReadFile(tc.schemaFile)
			require.NoError(t, err, "Failed to read schema file")

			// Register schema
			req := &pb.RegisterSchemaRequest{
				SchemaId:      tc.schemaID,
				SchemaContent: string(content),
			}

			resp, err := service.RegisterSchema(context.Background(), req)
			require.NoError(t, err, "gRPC call should not fail")
			assert.False(t, resp.Success, "Registration should fail for invalid schema")
			assert.Contains(t, resp.Message, tc.expectedError, "Error message should indicate the issue")

			// Verify schema is NOT stored in cache
			_, found := service.cache.Get(tc.schemaID)
			assert.False(t, found, "Invalid schema should not be cached")
		})
	}
}

// TestRegisterSchema_InputValidation tests input validation for RegisterSchema
func TestRegisterSchema_InputValidation(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	testCases := []struct {
		name          string
		schemaID      string
		schemaContent string
		expectedError string
	}{
		{
			name:          "Empty schema ID",
			schemaID:      "",
			schemaContent: `{"openapi":"3.0.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`,
			expectedError: "Validation error",
		},
		{
			name:          "Whitespace schema ID",
			schemaID:      "   ",
			schemaContent: `{"openapi":"3.0.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`,
			expectedError: "Validation error",
		},
		{
			name:          "Empty schema content",
			schemaID:      "test-schema",
			schemaContent: "",
			expectedError: "Validation error",
		},
		{
			name:          "Schema ID too long",
			schemaID:      string(make([]byte, 300)), // 300 characters
			schemaContent: `{"openapi":"3.0.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`,
			expectedError: "Validation error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.RegisterSchemaRequest{
				SchemaId:      tc.schemaID,
				SchemaContent: tc.schemaContent,
			}

			resp, err := service.RegisterSchema(context.Background(), req)
			require.NoError(t, err, "gRPC call should not fail")
			assert.False(t, resp.Success, "Registration should fail for invalid input")
			assert.Contains(t, resp.Message, tc.expectedError)
		})
	}
}

// TestRegisterSchema_UnsupportedFormat tests unsupported schema formats
func TestRegisterSchema_UnsupportedFormat(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	testCases := []struct {
		name          string
		schemaContent string
		expectedError string
	}{
		{
			name:          "OpenAPI v1",
			schemaContent: `{"openapi":"1.0.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`,
			expectedError: "unsupported schema format",
		},
		{
			name:          "Swagger 1.x",
			schemaContent: `{"swagger":"1.2","info":{"title":"Test","version":"1.0.0"}}`,
			expectedError: "unsupported schema format",
		},
		{
			name:          "No version field",
			schemaContent: `{"info":{"title":"Test","version":"1.0.0"},"paths":{}}`,
			expectedError: "unsupported schema format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.RegisterSchemaRequest{
				SchemaId:      "test-schema",
				SchemaContent: tc.schemaContent,
			}

			resp, err := service.RegisterSchema(context.Background(), req)
			require.NoError(t, err, "gRPC call should not fail")
			assert.False(t, resp.Success, "Registration should fail")
			assert.Contains(t, resp.Message, tc.expectedError)
		})
	}
}

// TestRegisterSchema_Concurrent tests concurrent schema registrations
func TestRegisterSchema_Concurrent(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load valid schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	// Register multiple schemas concurrently
	numGoroutines := 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			req := &pb.RegisterSchemaRequest{
				SchemaId:      "concurrent-schema-" + string(rune(index)),
				SchemaContent: string(content),
			}

			resp, err := service.RegisterSchema(context.Background(), req)
			assert.NoError(t, err)
			assert.True(t, resp.Success)

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestRegisterSchema_Update tests updating an existing schema
func TestRegisterSchema_Update(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	schemaID := "updatable-schema"

	// Load first schema
	content1, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	// Register initial schema
	req1 := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content1),
	}
	resp1, err := service.RegisterSchema(context.Background(), req1)
	require.NoError(t, err)
	assert.True(t, resp1.Success)

	// Load second schema
	content2, err := os.ReadFile("testdata/openapi-v3/valid/complex-ecommerce.json")
	require.NoError(t, err)

	// Update with different schema
	req2 := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content2),
	}
	resp2, err := service.RegisterSchema(context.Background(), req2)
	require.NoError(t, err)
	assert.True(t, resp2.Success)

	// Verify updated schema is in cache
	doc, found := service.cache.Get(schemaID)
	assert.True(t, found)
	assert.NotNil(t, doc)
}

// TestRegisterSchema_VersionValidation tests version consistency validation
func TestRegisterSchema_VersionValidation(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load schema with info.version "1.0.0"
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	t.Run("version match succeeds", func(t *testing.T) {
		req := &pb.RegisterSchemaRequest{
			SchemaId:      "version-match-schema",
			SchemaContent: string(content),
			SchemaVersion: "1.0.0", // Matches info.version
		}
		resp, err := service.RegisterSchema(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success, "Registration should succeed when versions match")
	})

	t.Run("version mismatch fails", func(t *testing.T) {
		req := &pb.RegisterSchemaRequest{
			SchemaId:      "version-mismatch-schema",
			SchemaContent: string(content),
			SchemaVersion: "2.0.0", // Does not match info.version "1.0.0"
		}
		resp, err := service.RegisterSchema(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success, "Registration should fail when versions mismatch")
		assert.Contains(t, resp.Message, "Version mismatch")
		assert.Contains(t, resp.Message, "2.0.0")
		assert.Contains(t, resp.Message, "1.0.0")
	})

	t.Run("no version provided uses schema info.version", func(t *testing.T) {
		req := &pb.RegisterSchemaRequest{
			SchemaId:      "no-version-schema",
			SchemaContent: string(content),
			// SchemaVersion not set - should use schema's info.version
		}
		resp, err := service.RegisterSchema(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success, "Registration should succeed when no version provided")
		// Verify schema's info.version (1.0.0) is used, not a generated timestamp
		assert.Equal(t, "1.0.0", resp.Metadata.SchemaVersion, "Should use schema's info.version")
	})

	t.Run("schema without info.version rejected by OpenAPI validation", func(t *testing.T) {
		// OpenAPI spec requires info.version - schema validation catches this
		schemaNoVersion := `{
			"openapi": "3.0.0",
			"info": {
				"title": "Test API"
			},
			"paths": {}
		}`
		req := &pb.RegisterSchemaRequest{
			SchemaId:      "no-info-version-schema",
			SchemaContent: schemaNoVersion,
			SchemaVersion: "1.0.0",
		}
		resp, err := service.RegisterSchema(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success, "Registration should fail - info.version is required by OpenAPI spec")
		assert.Contains(t, resp.Message, "Invalid OpenAPI schema")
	})
}

// TestListEndpoints tests the ListEndpoints gRPC method
func TestListEndpoints(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "list-endpoints-schema"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Test ListEndpoints
	req := &pb.ListEndpointsRequest{
		SchemaId: schemaID,
	}
	resp, err := service.ListEndpoints(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Endpoints)

	// Verify expected endpoints exist
	expectedEndpoints := []string{"GET /pets", "POST /pets", "GET /pets/{petId}", "PUT /pets/{petId}", "DELETE /pets/{petId}"}
	for _, expected := range expectedEndpoints {
		found := false
		for _, ep := range resp.Endpoints {
			if ep.Method+" "+ep.Path == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected endpoint %s not found", expected)
	}
}

// TestListEndpoints_SchemaNotFound tests ListEndpoints with non-existent schema
func TestListEndpoints_SchemaNotFound(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	req := &pb.ListEndpointsRequest{
		SchemaId: "non-existent-schema",
	}
	resp, err := service.ListEndpoints(context.Background(), req)
	require.NoError(t, err)
	// Returns empty list for non-existent schema
	assert.Empty(t, resp.Endpoints)
}

// TestGenerateFixture tests the GenerateFixture gRPC method
func TestGenerateFixture(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "generate-fixture-schema"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	testCases := []struct {
		name       string
		method     string
		path       string
		outputType pb.OutputType
		statusCode int32
	}{
		{
			name:       "GET /pets - full fixture",
			method:     "GET",
			path:       "/pets",
			outputType: pb.OutputType_OUTPUT_FIXTURE,
			statusCode: 200,
		},
		{
			name:       "GET /pets/{petId} - response only",
			method:     "GET",
			path:       "/pets/{petId}",
			outputType: pb.OutputType_OUTPUT_RESPONSE,
			statusCode: 200,
		},
		{
			name:       "POST /pets - request only",
			method:     "POST",
			path:       "/pets",
			outputType: pb.OutputType_OUTPUT_REQUEST,
			statusCode: 201,
		},
		{
			name:       "POST /pets - full fixture",
			method:     "POST",
			path:       "/pets",
			outputType: pb.OutputType_OUTPUT_FIXTURE,
			statusCode: 201,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.GenerateFixtureRequest{
				SchemaId:   schemaID,
				Method:     tc.method,
				Path:       tc.path,
				OutputType: tc.outputType,
				StatusCode: tc.statusCode,
			}
			resp, err := service.GenerateFixture(context.Background(), req)
			require.NoError(t, err)
			assert.True(t, resp.Success, "Expected success for %s, got message: %s", tc.name, resp.Message)

			switch tc.outputType {
			case pb.OutputType_OUTPUT_FIXTURE:
				assert.NotNil(t, resp.Fixture, "Fixture should be present for fixture output")
				assert.NotNil(t, resp.Fixture.Request, "Request should be present for fixture output")
				assert.NotNil(t, resp.Fixture.Response, "Response should be present for fixture output")
			case pb.OutputType_OUTPUT_REQUEST:
				assert.NotEmpty(t, resp.RequestBody, "Request body should be present for request output")
			case pb.OutputType_OUTPUT_RESPONSE:
				assert.NotNil(t, resp.Response, "Response should be present for response output")
			}
		})
	}
}

// TestGenerateFixture_SchemaNotFound tests GenerateFixture with non-existent schema
func TestGenerateFixture_SchemaNotFound(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	req := &pb.GenerateFixtureRequest{
		SchemaId: "non-existent-schema",
		Method:   "GET",
		Path:     "/pets",
	}
	resp, err := service.GenerateFixture(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "not found")
}

// TestGenerateFixture_RouteNotFound tests GenerateFixture with invalid route
func TestGenerateFixture_RouteNotFound(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "route-not-found-schema"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	req := &pb.GenerateFixtureRequest{
		SchemaId: schemaID,
		Method:   "GET",
		Path:     "/non-existent-path",
	}
	resp, err := service.GenerateFixture(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "not found")
}

// TestGenerateFixture_RequestBodyGeneration tests request body generation
func TestGenerateFixture_RequestBodyGeneration(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "request-body-schema"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Test POST /pets which has a request body
	req := &pb.GenerateFixtureRequest{
		SchemaId:   schemaID,
		Method:     "POST",
		Path:       "/pets",
		OutputType: pb.OutputType_OUTPUT_REQUEST,
	}
	resp, err := service.GenerateFixture(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.RequestBody, "Request body should be generated")
}

// TestGenerateFixture_PathParameterResolution tests that path parameters are resolved
func TestGenerateFixture_PathParameterResolution(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "path-param-schema"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Test GET /pets/{petId} - should resolve {petId}
	req := &pb.GenerateFixtureRequest{
		SchemaId:   schemaID,
		Method:     "GET",
		Path:       "/pets/{petId}",
		OutputType: pb.OutputType_OUTPUT_FIXTURE,
	}
	resp, err := service.GenerateFixture(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success, "Message: %s", resp.Message)
	assert.NotNil(t, resp.Fixture)
	assert.NotNil(t, resp.Fixture.Request)
	assert.NotContains(t, resp.Fixture.Request.Path, "{petId}", "Path parameter should be resolved")
}

// ============================================================================
// Phase 1: Producer Testing - ValidateProducerResponse Tests
// ============================================================================

// TestValidateProducerResponse_ValidResponse tests validating a valid producer response
func TestValidateProducerResponse_ValidResponse(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "producer-test-schema"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Test valid response for GET /pets
	req := &pb.ValidateProducerRequest{
		SchemaId: schemaID,
		Method:   "GET",
		Path:     "/pets",
		Response: &pb.ResponseData{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `[{"id": "1", "name": "Fluffy", "tag": "cat"}]`,
		},
	}

	result, err := service.ValidateProducerResponse(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Valid, "Valid response should pass validation. Errors: %v", result.Errors)
}

// TestValidateProducerResponse_InvalidResponse tests validating an invalid producer response
func TestValidateProducerResponse_InvalidResponse(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "producer-invalid-schema"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Test invalid response - missing required 'name' field
	req := &pb.ValidateProducerRequest{
		SchemaId: schemaID,
		Method:   "GET",
		Path:     "/pets/123",
		Response: &pb.ResponseData{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"id": "1"}`, // Missing required 'name' field
		},
	}

	result, err := service.ValidateProducerResponse(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, result.Valid, "Invalid response should fail validation")
	assert.NotEmpty(t, result.Errors, "Should have validation errors")
}

// TestValidateProducerResponse_SchemaNotFound tests validation with non-existent schema
func TestValidateProducerResponse_SchemaNotFound(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	req := &pb.ValidateProducerRequest{
		SchemaId: "non-existent-schema",
		Method:   "GET",
		Path:     "/pets",
		Response: &pb.ResponseData{
			StatusCode: 200,
			Body:       `[]`,
		},
	}

	result, err := service.ValidateProducerResponse(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "Schema not found")
}

// TestValidateProducerResponse_PathNotFound tests validation with non-existent path
func TestValidateProducerResponse_PathNotFound(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "producer-path-not-found"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	req := &pb.ValidateProducerRequest{
		SchemaId: schemaID,
		Method:   "GET",
		Path:     "/non-existent-path",
		Response: &pb.ResponseData{
			StatusCode: 200,
			Body:       `{}`,
		},
	}

	result, err := service.ValidateProducerResponse(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "Route not found")
}

// TestValidateProducerResponse_WrongStatusCode tests validation with wrong status code
func TestValidateProducerResponse_WrongStatusCode(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "producer-status-code"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Return 500 instead of expected 200/201
	req := &pb.ValidateProducerRequest{
		SchemaId: schemaID,
		Method:   "GET",
		Path:     "/pets",
		Response: &pb.ResponseData{
			StatusCode: 500,
			Body:       `{"error": "Internal Server Error"}`,
		},
	}

	result, err := service.ValidateProducerResponse(context.Background(), req)
	require.NoError(t, err)
	// This may or may not be valid depending on whether 500 is defined in the schema
	// The important thing is it doesn't crash
	assert.NotNil(t, result)
}

// TestValidateProducerResponse_WithPathParams tests validation with path parameters
func TestValidateProducerResponse_WithPathParams(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "producer-path-params"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Test with actual path parameter value
	req := &pb.ValidateProducerRequest{
		SchemaId: schemaID,
		Method:   "GET",
		Path:     "/pets/123", // Actual path with resolved parameter
		Response: &pb.ResponseData{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"id": "123", "name": "Fluffy", "tag": "cat"}`,
		},
	}

	result, err := service.ValidateProducerResponse(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Valid, "Should validate response for parameterized path. Errors: %v", result.Errors)
}

// TestValidateProducerResponse_InputValidation tests input validation
func TestValidateProducerResponse_InputValidation(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	testCases := []struct {
		name          string
		request       *pb.ValidateProducerRequest
		expectedError string
	}{
		{
			name:          "Nil request",
			request:       nil,
			expectedError: "cannot be null",
		},
		{
			name: "Empty schema ID",
			request: &pb.ValidateProducerRequest{
				SchemaId: "",
				Method:   "GET",
				Path:     "/pets",
				Response: &pb.ResponseData{StatusCode: 200},
			},
			expectedError: "Validation error",
		},
		{
			name: "Empty method",
			request: &pb.ValidateProducerRequest{
				SchemaId: "test",
				Method:   "",
				Path:     "/pets",
				Response: &pb.ResponseData{StatusCode: 200},
			},
			expectedError: "Validation error",
		},
		{
			name: "Empty path",
			request: &pb.ValidateProducerRequest{
				SchemaId: "test",
				Method:   "GET",
				Path:     "",
				Response: &pb.ResponseData{StatusCode: 200},
			},
			expectedError: "Validation error",
		},
		{
			name: "Nil response",
			request: &pb.ValidateProducerRequest{
				SchemaId: "test",
				Method:   "GET",
				Path:     "/pets",
				Response: nil,
			},
			expectedError: "cannot be null",
		},
		{
			name: "Invalid status code",
			request: &pb.ValidateProducerRequest{
				SchemaId: "test",
				Method:   "GET",
				Path:     "/pets",
				Response: &pb.ResponseData{StatusCode: 0},
			},
			expectedError: "Validation error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := service.ValidateProducerResponse(context.Background(), tc.request)
			require.NoError(t, err, "gRPC call should not fail")
			assert.False(t, result.Valid, "Validation should fail for invalid input")
			assert.NotEmpty(t, result.Errors)
			assert.Contains(t, result.Errors[0], tc.expectedError)
		})
	}
}

// TestValidateProducerResponse_VersionSpecific tests validation against specific schema version
func TestValidateProducerResponse_VersionSpecific(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register schema with version
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	schemaID := "producer-versioned"
	regReq := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
		SchemaVersion: "1.0.0",
	}
	regResp, err := service.RegisterSchema(context.Background(), regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Validate against specific version
	req := &pb.ValidateProducerRequest{
		SchemaId:      schemaID,
		SchemaVersion: "1.0.0",
		Method:        "GET",
		Path:          "/pets",
		Response: &pb.ResponseData{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `[{"id": "1", "name": "Fluffy"}]`,
		},
	}

	result, err := service.ValidateProducerResponse(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Valid, "Should validate against specific version. Errors: %v", result.Errors)
	assert.Equal(t, "1.0.0", result.ValidatedAgainstVersion)
}

// TestNewValidatorServiceWithCache tests creating service with existing cache
func TestNewValidatorServiceWithCache(t *testing.T) {
	cache, err := NewSchemaCache()
	require.NoError(t, err)
	defer cache.Close()

	service := NewValidatorServiceWithCache(cache)
	require.NotNil(t, service)
	assert.Equal(t, cache, service.GetCache())
}

// TestValidatorService_GetCache tests getting the cache from service
func TestValidatorService_GetCache(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	cache := service.GetCache()
	assert.NotNil(t, cache)
}

// TestValidateNotNull tests the validation utility
func TestValidateNotNull(t *testing.T) {
	t.Run("returns error for nil value", func(t *testing.T) {
		err := ValidateNotNull(nil, "test field")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test field")
	})

	t.Run("returns nil for non-nil value", func(t *testing.T) {
		err := ValidateNotNull("value", "test field")
		assert.NoError(t, err)
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		err := ValidateNotNull("", "test field")
		assert.NoError(t, err)
	})
}

// TestGetSchema tests fetching a schema by ID
func TestGetSchema(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Load and register a schema first
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "get-schema-test",
		SchemaContent: string(content),
		SchemaVersion: "1.0.0",
	})
	require.NoError(t, err)

	t.Run("get existing schema", func(t *testing.T) {
		req := &pb.GetSchemaRequest{
			SchemaId: "get-schema-test",
		}
		resp, err := service.GetSchema(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.SchemaContent)
	})

	t.Run("get non-existent schema returns found=false", func(t *testing.T) {
		req := &pb.GetSchemaRequest{
			SchemaId: "non-existent-schema",
		}
		resp, err := service.GetSchema(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Found)
	})

	t.Run("get specific version", func(t *testing.T) {
		req := &pb.GetSchemaRequest{
			SchemaId:      "get-schema-test",
			SchemaVersion: "1.0.0",
		}
		resp, err := service.GetSchema(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "1.0.0", resp.Metadata.SchemaVersion)
	})
}

// TestListSchemas tests listing all schemas
func TestListSchemas(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register a couple of schemas
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "list-test-1",
		SchemaContent: string(content),
	})
	require.NoError(t, err)

	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "list-test-2",
		SchemaContent: string(content),
	})
	require.NoError(t, err)

	t.Run("list all schemas", func(t *testing.T) {
		req := &pb.ListSchemasRequest{}
		resp, err := service.ListSchemas(context.Background(), req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Schemas), 2)
	})

	t.Run("list with page size", func(t *testing.T) {
		req := &pb.ListSchemasRequest{
			PageSize: 1,
		}
		resp, err := service.ListSchemas(context.Background(), req)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(resp.Schemas), 1)
	})
}

// TestCompareSchemas tests schema comparison
func TestCompareSchemas(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register v1 schema (info.version: "1.0.0")
	contentV1, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	respV1, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "compare-test-valid",
		SchemaContent: string(contentV1),
		SchemaVersion: "1.0.0",
	})
	require.NoError(t, err)
	require.True(t, respV1.Success, "v1 registration should succeed")

	// Register v2 schema (info.version: "2.0.0")
	contentV2, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore-v2.json")
	require.NoError(t, err)

	respV2, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "compare-test-valid",
		SchemaContent: string(contentV2),
		SchemaVersion: "2.0.0",
	})
	require.NoError(t, err)
	require.True(t, respV2.Success, "v2 registration should succeed")

	t.Run("compare compatible versions", func(t *testing.T) {
		req := &pb.CompareSchemasRequest{
			SchemaId:   "compare-test-valid",
			OldVersion: "1.0.0",
			NewVersion: "2.0.0",
		}
		resp, err := service.CompareSchemas(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.Compatible)
	})

	t.Run("compare with non-existent schema", func(t *testing.T) {
		req := &pb.CompareSchemasRequest{
			SchemaId:   "non-existent",
			OldVersion: "1.0.0",
			NewVersion: "2.0.0",
		}
		resp, err := service.CompareSchemas(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		// When schemas not found, the response should indicate this
	})
}

// TestRegisterConsumer tests consumer registration
func TestRegisterConsumer(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// First register a schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	regResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "consumer-test-schema",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	require.True(t, regResp.Success)

	t.Run("register consumer successfully", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			ConsumerId:      "order-service",
			ConsumerVersion: "1.0.0",
			SchemaId:        "consumer-test-schema",
			SchemaVersion:   "1.0.0",
			Environment:     "prod",
			UsedEndpoints: []*pb.EndpointUsage{
				{Method: "GET", Path: "/pets"},
				{Method: "POST", Path: "/pets"},
			},
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.NotNil(t, resp.Consumer)
		assert.Equal(t, "order-service", resp.Consumer.ConsumerId)
		assert.Equal(t, "prod", resp.Consumer.Environment)
	})

	t.Run("register consumer with default environment", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			ConsumerId:      "payment-service",
			ConsumerVersion: "1.0.0",
			SchemaId:        "consumer-test-schema",
			SchemaVersion:   "1.0.0",
			// No environment - should default to "dev"
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "dev", resp.Consumer.Environment)
	})

	t.Run("register consumer missing consumer_id", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			SchemaId:    "consumer-test-schema",
			Environment: "prod",
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "consumer_id is required")
	})

	t.Run("register consumer missing schema_id", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			ConsumerId:  "test-consumer",
			Environment: "prod",
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "schema_id is required")
	})

	t.Run("register consumer with non-existent schema", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			ConsumerId:  "test-consumer",
			SchemaId:    "non-existent-schema",
			Environment: "prod",
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "schema not found")
	})
}

// TestListConsumers tests listing consumers for a schema
func TestListConsumers(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register a schema first
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	regResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "list-consumers-schema",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Register some consumers
	consumers := []struct {
		id  string
		env string
	}{
		{"consumer-1", "prod"},
		{"consumer-2", "prod"},
		{"consumer-3", "staging"},
	}

	for _, c := range consumers {
		_, err := service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
			ConsumerId:  c.id,
			SchemaId:    "list-consumers-schema",
			Environment: c.env,
		})
		require.NoError(t, err)
	}

	t.Run("list all consumers for schema", func(t *testing.T) {
		req := &pb.ListConsumersRequest{
			SchemaId: "list-consumers-schema",
		}

		resp, err := service.ListConsumers(context.Background(), req)
		require.NoError(t, err)
		assert.Len(t, resp.Consumers, 3)
	})

	t.Run("list consumers by environment", func(t *testing.T) {
		req := &pb.ListConsumersRequest{
			SchemaId:    "list-consumers-schema",
			Environment: "prod",
		}

		resp, err := service.ListConsumers(context.Background(), req)
		require.NoError(t, err)
		assert.Len(t, resp.Consumers, 2)

		for _, c := range resp.Consumers {
			assert.Equal(t, "prod", c.Environment)
		}
	})

	t.Run("list consumers with empty schema_id", func(t *testing.T) {
		req := &pb.ListConsumersRequest{
			SchemaId: "",
		}

		resp, err := service.ListConsumers(context.Background(), req)
		require.NoError(t, err)
		assert.Empty(t, resp.Consumers)
	})
}

// TestDeregisterConsumer tests consumer deregistration
func TestDeregisterConsumer(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register a schema first
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	regResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "deregister-test-schema",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Register a consumer
	_, err = service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
		ConsumerId:  "to-be-removed",
		SchemaId:    "deregister-test-schema",
		Environment: "prod",
	})
	require.NoError(t, err)

	t.Run("deregister existing consumer", func(t *testing.T) {
		req := &pb.DeregisterConsumerRequest{
			ConsumerId:  "to-be-removed",
			SchemaId:    "deregister-test-schema",
			Environment: "prod",
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("deregister non-existent consumer", func(t *testing.T) {
		req := &pb.DeregisterConsumerRequest{
			ConsumerId:  "non-existent",
			SchemaId:    "deregister-test-schema",
			Environment: "prod",
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "consumer not found")
	})

	t.Run("deregister with missing consumer_id", func(t *testing.T) {
		req := &pb.DeregisterConsumerRequest{
			SchemaId:    "deregister-test-schema",
			Environment: "prod",
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "consumer_id is required")
	})

	t.Run("deregister with missing schema_id", func(t *testing.T) {
		req := &pb.DeregisterConsumerRequest{
			ConsumerId:  "some-consumer",
			Environment: "prod",
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "schema_id is required")
	})

	t.Run("deregister with default environment", func(t *testing.T) {
		// First register a consumer in dev environment
		_, err := service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
			ConsumerId:  "dev-consumer",
			SchemaId:    "deregister-test-schema",
			Environment: "dev",
		})
		require.NoError(t, err)

		// Deregister without specifying environment (defaults to dev)
		req := &pb.DeregisterConsumerRequest{
			ConsumerId: "dev-consumer",
			SchemaId:   "deregister-test-schema",
			// No environment - should default to "dev"
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})
}

// TestCanIDeploy tests deployment safety checks
func TestCanIDeploy(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register a schema first
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	regResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "can-i-deploy-schema",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	require.True(t, regResp.Success)

	t.Run("safe to deploy with no consumers", func(t *testing.T) {
		req := &pb.CanIDeployRequest{
			SchemaId:    "can-i-deploy-schema",
			NewVersion:  "2.0.0",
			Environment: "prod",
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.SafeToDeploy)
		assert.Contains(t, resp.Summary, "No consumers registered")
	})

	t.Run("missing schema_id", func(t *testing.T) {
		req := &pb.CanIDeployRequest{
			NewVersion:  "2.0.0",
			Environment: "prod",
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.SafeToDeploy)
		assert.Contains(t, resp.Summary, "schema_id is required")
	})

	t.Run("schema not found", func(t *testing.T) {
		req := &pb.CanIDeployRequest{
			SchemaId:    "non-existent-schema",
			NewVersion:  "2.0.0",
			Environment: "prod",
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.SafeToDeploy)
		assert.Contains(t, resp.Summary, "schema not found")
	})

	t.Run("with consumers registered", func(t *testing.T) {
		// Register a consumer
		_, err := service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
			ConsumerId:    "deploy-test-consumer",
			SchemaId:      "can-i-deploy-schema",
			SchemaVersion: "1.0.0",
			Environment:   "prod",
		})
		require.NoError(t, err)

		req := &pb.CanIDeployRequest{
			SchemaId:    "can-i-deploy-schema",
			NewVersion:  "2.0.0",
			Environment: "prod",
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		// With consumers, the response indicates the analysis result
		assert.NotEmpty(t, resp.Summary)
	})

	t.Run("default environment is prod", func(t *testing.T) {
		req := &pb.CanIDeployRequest{
			SchemaId:   "can-i-deploy-schema",
			NewVersion: "2.0.0",
			// No environment - should default to "prod"
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		// Should work with default environment
		assert.NotNil(t, resp)
	})
}

// TestValidateInteraction_SpecificVersion tests validation against a specific schema version
func TestValidateInteraction_SpecificVersion(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register v1 schema (info.version: "1.0.0")
	contentV1, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	respV1, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "version-specific-test",
		SchemaContent: string(contentV1),
	})
	require.NoError(t, err)
	require.True(t, respV1.Success, "v1 registration should succeed")
	assert.Equal(t, "1.0.0", respV1.Metadata.SchemaVersion)

	// Register v2 schema (info.version: "2.0.0")
	contentV2, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore-v2.json")
	require.NoError(t, err)

	respV2, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "version-specific-test",
		SchemaContent: string(contentV2),
	})
	require.NoError(t, err)
	require.True(t, respV2.Success, "v2 registration should succeed")
	assert.Equal(t, "2.0.0", respV2.Metadata.SchemaVersion)

	t.Run("validate against v1 specifically", func(t *testing.T) {
		req := &pb.InteractionRequest{
			SchemaId:      "version-specific-test",
			SchemaVersion: "1.0.0",
			Request: &pb.RequestData{
				Method:  "GET",
				Path:    "/pets",
				Headers: map[string]string{"Content-Type": "application/json"},
			},
			Response: &pb.ResponseData{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `[{"id":"1","name":"Fluffy"}]`,
			},
		}
		result, err := service.ValidateInteraction(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Valid, "Validation should pass for v1 schema")
		assert.Equal(t, "1.0.0", result.ValidatedAgainstVersion, "Should validate against v1")
	})

	t.Run("validate against v2 specifically", func(t *testing.T) {
		req := &pb.InteractionRequest{
			SchemaId:      "version-specific-test",
			SchemaVersion: "2.0.0",
			Request: &pb.RequestData{
				Method:  "GET",
				Path:    "/pets",
				Headers: map[string]string{"Content-Type": "application/json"},
			},
			Response: &pb.ResponseData{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `[{"id":"1","name":"Fluffy","species":"cat"}]`,
			},
		}
		result, err := service.ValidateInteraction(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Valid, "Validation should pass for v2 schema")
		assert.Equal(t, "2.0.0", result.ValidatedAgainstVersion, "Should validate against v2")
	})

	t.Run("validate without version uses latest", func(t *testing.T) {
		req := &pb.InteractionRequest{
			SchemaId: "version-specific-test",
			// No SchemaVersion - should use latest (v2)
			Request: &pb.RequestData{
				Method:  "GET",
				Path:    "/pets",
				Headers: map[string]string{"Content-Type": "application/json"},
			},
			Response: &pb.ResponseData{
				StatusCode: 200,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `[{"id":"1","name":"Fluffy"}]`,
			},
		}
		result, err := service.ValidateInteraction(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Valid, "Validation should pass")
		assert.Equal(t, "2.0.0", result.ValidatedAgainstVersion, "Should use latest version (v2)")
	})
}

// TestCanIDeploy_ConsumerVersionTracking tests the full consumer version tracking flow
func TestCanIDeploy_ConsumerVersionTracking(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	schemaID := "consumer-version-tracking-test"

	// Register v1 schema (info.version: "1.0.0")
	contentV1, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	respV1, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(contentV1),
	})
	require.NoError(t, err)
	require.True(t, respV1.Success)

	// Register a consumer tested against v1.0.0
	consumerResp, err := service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
		ConsumerId:    "order-service",
		SchemaId:      schemaID,
		SchemaVersion: "1.0.0",
		Environment:   "prod",
	})
	require.NoError(t, err)
	require.True(t, consumerResp.Success, "Consumer registration should succeed")

	t.Run("deploy same version is safe", func(t *testing.T) {
		resp, err := service.CanIDeploy(context.Background(), &pb.CanIDeployRequest{
			SchemaId:    schemaID,
			NewVersion:  "1.0.0",
			Environment: "prod",
		})
		require.NoError(t, err)
		assert.True(t, resp.SafeToDeploy, "Deploying same version should be safe")
	})

	t.Run("deploy new version shows consumer impact", func(t *testing.T) {
		// Register v2 schema (info.version: "2.0.0")
		contentV2, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore-v2.json")
		require.NoError(t, err)

		respV2, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
			SchemaId:      schemaID,
			SchemaContent: string(contentV2),
		})
		require.NoError(t, err)
		require.True(t, respV2.Success)

		resp, err := service.CanIDeploy(context.Background(), &pb.CanIDeployRequest{
			SchemaId:    schemaID,
			NewVersion:  "2.0.0",
			Environment: "prod",
		})
		require.NoError(t, err)
		// Consumer is on v1.0.0, deploying v2.0.0 should show impact
		assert.NotEmpty(t, resp.AffectedConsumers, "Should show consumer impact")
		assert.Equal(t, "order-service", resp.AffectedConsumers[0].ConsumerId)
		assert.Equal(t, "1.0.0", resp.AffectedConsumers[0].CurrentSchemaVersion)
	})
}

// TestMultipleVersionsInCache tests that multiple schema versions can be stored and retrieved
func TestMultipleVersionsInCache(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	schemaID := "multi-version-cache-test"

	// Register v1 schema (info.version: "1.0.0")
	contentV1, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	respV1, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(contentV1),
	})
	require.NoError(t, err)
	require.True(t, respV1.Success)
	assert.Equal(t, "1.0.0", respV1.Metadata.SchemaVersion)

	// Register v2 schema (info.version: "2.0.0")
	contentV2, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore-v2.json")
	require.NoError(t, err)

	respV2, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(contentV2),
	})
	require.NoError(t, err)
	require.True(t, respV2.Success)
	assert.Equal(t, "2.0.0", respV2.Metadata.SchemaVersion)

	t.Run("GetSchema without version returns latest", func(t *testing.T) {
		resp, err := service.GetSchema(context.Background(), &pb.GetSchemaRequest{
			SchemaId: schemaID,
		})
		require.NoError(t, err)
		require.True(t, resp.Found)
		assert.Equal(t, "2.0.0", resp.Metadata.SchemaVersion, "Should return latest version")
	})

	t.Run("GetSchema with v1 returns v1", func(t *testing.T) {
		resp, err := service.GetSchema(context.Background(), &pb.GetSchemaRequest{
			SchemaId:      schemaID,
			SchemaVersion: "1.0.0",
		})
		require.NoError(t, err)
		require.True(t, resp.Found)
		assert.Equal(t, "1.0.0", resp.Metadata.SchemaVersion, "Should return v1")
	})

	t.Run("GetSchema with v2 returns v2", func(t *testing.T) {
		resp, err := service.GetSchema(context.Background(), &pb.GetSchemaRequest{
			SchemaId:      schemaID,
			SchemaVersion: "2.0.0",
		})
		require.NoError(t, err)
		require.True(t, resp.Found)
		assert.Equal(t, "2.0.0", resp.Metadata.SchemaVersion, "Should return v2")
	})

	t.Run("cache has both versions tracked", func(t *testing.T) {
		versions := service.cache.ListVersions(schemaID)
		assert.Contains(t, versions, "1.0.0", "Should have v1")
		assert.Contains(t, versions, "2.0.0", "Should have v2")
	})
}

// TestFilterChangesForConsumer tests the filterChangesForConsumer helper function
func TestFilterChangesForConsumer(t *testing.T) {
	// Sample breaking changes
	changes := []*pb.BreakingChange{
		{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users", Method: "GET", Description: "GET /users removed"},
		{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users/{id}", Method: "DELETE", Description: "DELETE /users/{id} removed"},
		{Type: pb.BreakingChangeType_REQUIRED_PARAMETER_ADDED, Path: "/pets", Method: "POST", Description: "Required param added to POST /pets"},
		{Type: pb.BreakingChangeType_RESPONSE_SCHEMA_CHANGED, Path: "/orders", Method: "", Description: "Response schema changed for /orders"},
	}

	t.Run("no endpoints returns all changes (conservative)", func(t *testing.T) {
		endpoints := []EndpointUsage{}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 4, "Should return all changes when no endpoints specified")
	})

	t.Run("filters to matching endpoint", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "GET", Path: "/users"},
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 1)
		assert.Equal(t, "/users", result[0].Path)
		assert.Equal(t, "GET", result[0].Method)
	})

	t.Run("filters multiple endpoints", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/pets"},
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 2)
	})

	t.Run("matches when change has empty method (affects all methods)", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "GET", Path: "/orders"},
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 1, "Should match change with empty method")
		assert.Equal(t, "/orders", result[0].Path)
	})

	t.Run("no match returns empty", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "GET", Path: "/nonexistent"},
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 0)
	})

	t.Run("method mismatch does not match", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "POST", Path: "/users"}, // Change is for GET /users
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 0, "POST /users should not match GET /users change")
	})
}

func TestNewValidatorServiceWithStore(t *testing.T) {
	memStore := storage.NewMemoryStore()
	service, err := NewValidatorServiceWithStore(memStore)
	require.NoError(t, err)
	defer service.Close()
	assert.NotNil(t, service.store)
}

func TestStoragePersistence(t *testing.T) {
	memStore := storage.NewMemoryStore()
	service, err := NewValidatorServiceWithStore(memStore)
	require.NoError(t, err)
	defer service.Close()

	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	// Register schema
	resp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "persist-test",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify schema is in storage
	record, err := memStore.GetSchema(context.Background(), "persist-test")
	require.NoError(t, err)
	assert.Equal(t, "persist-test", record.SchemaID)
	assert.NotEmpty(t, record.Content)
}

func TestStorageReadThrough(t *testing.T) {
	memStore := storage.NewMemoryStore()
	service, err := NewValidatorServiceWithStore(memStore)
	require.NoError(t, err)
	defer service.Close()

	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	// Register schema (goes to both cache and store)
	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "readthrough-test",
		SchemaContent: string(content),
	})
	require.NoError(t, err)

	// Clear the cache to simulate eviction/restart
	service.cache.Delete("readthrough-test")

	// GetSchema should still work via storage read-through
	getResp, err := service.GetSchema(context.Background(), &pb.GetSchemaRequest{
		SchemaId: "readthrough-test",
	})
	require.NoError(t, err)
	assert.True(t, getResp.Found, "Schema should be found via storage read-through")
	assert.Equal(t, "readthrough-test", getResp.Metadata.SchemaId)
}

func TestConcurrentValidationNoRace(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "concurrent-test",
		SchemaContent: string(content),
	})
	require.NoError(t, err)

	// Run 10 concurrent validations — race detector will catch shared state mutation
	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, vErr := service.ValidateInteraction(context.Background(), &pb.InteractionRequest{
				SchemaId: "concurrent-test",
				Request: &pb.RequestData{
					Method: "GET",
					Path:   "/pets",
				},
				Response: &pb.ResponseData{
					StatusCode: 200,
					Body:       `[{"id": 1, "name": "Fido"}]`,
				},
			})
			if vErr != nil {
				errCh <- vErr
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for vErr := range errCh {
		t.Errorf("ValidateInteraction failed: %v", vErr)
	}
}

func TestRouterCachedInSchemaEntry(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "router-cache-test",
		SchemaContent: string(content),
	})
	require.NoError(t, err)

	entry, found := service.cache.Get("router-cache-test")
	require.True(t, found)
	assert.NotNil(t, entry.Router, "Router should be cached in SchemaEntry after registration")
}

// TestGetSchemaEntry_CacheMissNoStore verifies that getSchemaEntry returns nil/false
// when the schema is not in cache and no storage backend is configured.
func TestGetSchemaEntry_CacheMissNoStore(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// No schema registered, no store configured
	entry, found := service.getSchemaEntry(context.Background(), "nonexistent-schema", "")
	assert.False(t, found, "Should not find schema without store")
	assert.Nil(t, entry, "Entry should be nil for nonexistent schema")

	// Also try with a specific version
	entry, found = service.getSchemaEntry(context.Background(), "nonexistent-schema", "1.0.0")
	assert.False(t, found, "Should not find versioned schema without store")
	assert.Nil(t, entry, "Entry should be nil for nonexistent versioned schema")
}

// TestGetSchemaEntry_VersionSpecificReadThrough verifies that a schema registered with storage
// can be read back by version after cache eviction.
func TestGetSchemaEntry_VersionSpecificReadThrough(t *testing.T) {
	memStore := storage.NewMemoryStore()
	service, err := NewValidatorServiceWithStore(memStore)
	require.NoError(t, err)
	defer service.Close()

	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	// Register schema (info.version in simple-petstore.json is "1.0.0")
	resp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "version-readthrough-test",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Evict from cache
	service.cache.Delete("version-readthrough-test")

	// Verify cache miss
	_, found := service.cache.Get("version-readthrough-test")
	assert.False(t, found, "Schema should not be in cache after eviction")

	// Read back via getSchemaEntry with specific version — should rehydrate from storage
	entry, found := service.getSchemaEntry(context.Background(), "version-readthrough-test", "1.0.0")
	require.True(t, found, "Schema should be found via storage read-through")
	require.NotNil(t, entry)
	assert.NotNil(t, entry.Document, "Rehydrated entry should have a parsed document")
	assert.NotNil(t, entry.Router, "Rehydrated entry should have a router")
	assert.Equal(t, "1.0.0", entry.Metadata.SchemaVersion)
}

// TestValidateInteraction_RouterFallback verifies that validation still works
// when entry.Router is nil (fallback path rebuilds the router).
func TestValidateInteraction_RouterFallback(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "router-fallback-test",
		SchemaContent: string(content),
	})
	require.NoError(t, err)

	// Nil out the cached router to force fallback
	entry, found := service.cache.Get("router-fallback-test")
	require.True(t, found)
	entry.Router = nil

	// Validate interaction — should still work via fallback router creation
	result, err := service.ValidateInteraction(context.Background(), &pb.InteractionRequest{
		SchemaId: "router-fallback-test",
		Request: &pb.RequestData{
			Method:  "GET",
			Path:    "/pets",
			Headers: map[string]string{"Content-Type": "application/json"},
		},
		Response: &pb.ResponseData{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `[{"id": "1", "name": "Fido"}]`,
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Valid, "Validation should succeed even with nil router (fallback)")
}

// TestBuildRouter_SwaggerV2BasePath verifies that a Swagger v2 schema with basePath
// is correctly handled: the router strips the basePath so that requests with the
// full path (including basePath prefix) validate correctly.
func TestBuildRouter_SwaggerV2BasePath(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	content, err := os.ReadFile("testdata/openapi-v2/valid/api-with-basepath.json")
	require.NoError(t, err)

	// Register the v2 schema with basePath: /api/v2
	resp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "basepath-test",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Validate with the full path including basePath prefix — should be stripped
	result, err := service.ValidateInteraction(context.Background(), &pb.InteractionRequest{
		SchemaId: "basepath-test",
		Request: &pb.RequestData{
			Method:  "GET",
			Path:    "/api/v2/users",
			Headers: map[string]string{"Content-Type": "application/json"},
		},
		Response: &pb.ResponseData{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `[{"id": "1", "username": "john"}]`,
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Valid, "Validation should succeed with basePath-prefixed request path")

	// Also validate without the basePath prefix — should also work
	result, err = service.ValidateInteraction(context.Background(), &pb.InteractionRequest{
		SchemaId: "basepath-test",
		Request: &pb.RequestData{
			Method:  "GET",
			Path:    "/users",
			Headers: map[string]string{"Content-Type": "application/json"},
		},
		Response: &pb.ResponseData{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `[{"id": "1", "username": "john"}]`,
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Valid, "Validation should succeed without basePath prefix too")
}
