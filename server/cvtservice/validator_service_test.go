package cvtservice

import (
	"context"
	"errors"
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

// ============================================================================
// Phase 1: Producer Testing - ValidateProducerResponse Tests
// ============================================================================

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

// ============================================================================
// CheckCompatibility + hook fire tests (PR 2 — issue #107)
// ============================================================================

const compatTestV1Schema = `{
  "openapi": "3.0.0",
  "info": {"title": "Compat Test", "version": "1.0.0"},
  "paths": {
    "/items": {
      "get": {"responses": {"200": {"description": "ok"}}}
    }
  }
}`

// v2 removes /items endpoint => ENDPOINT_REMOVED breaking change.
const compatTestV2Schema = `{
  "openapi": "3.0.0",
  "info": {"title": "Compat Test", "version": "2.0.0"},
  "paths": {
    "/items/v2": {
      "get": {"responses": {"200": {"description": "ok"}}}
    }
  }
}`

// v2 same shape as v1 plus a new endpoint => no breaking change.
const compatTestV2NonBreakingSchema = `{
  "openapi": "3.0.0",
  "info": {"title": "Compat Test", "version": "2.0.0"},
  "paths": {
    "/items": {
      "get": {"responses": {"200": {"description": "ok"}}}
    },
    "/items/new": {
      "get": {"responses": {"200": {"description": "ok"}}}
    }
  }
}`

func TestRegisterSchema_CheckCompatibility_NoPrior_RegistersClean(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	rec := &recordingHooks{}
	service.SetHooks(rec)

	resp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:           "compat-no-prior",
		SchemaContent:      compatTestV1Schema,
		CheckCompatibility: true,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success, "first registration with --check-compatibility should succeed cleanly")
	assert.Empty(t, resp.BreakingChanges)
	assert.Empty(t, rec.breakingChangeCalls, "no prior version means no comparison and no fire")
}

func TestRegisterSchema_CheckCompatibility_NoBreaking_NoFire(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()
	rec := &recordingHooks{}
	service.SetHooks(rec)

	// v1
	setupResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "compat-no-breaking",
		SchemaContent: compatTestV1Schema,
	})
	require.NoError(t, err)
	require.True(t, setupResp.Success, "v1 setup must succeed: %s", setupResp.Message)
	rec.breakingChangeCalls = nil // reset

	// v2 — additive only
	resp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:           "compat-no-breaking",
		SchemaContent:      compatTestV2NonBreakingSchema,
		CheckCompatibility: true,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Empty(t, resp.BreakingChanges, "additive change => no breaking changes")
	assert.Empty(t, rec.breakingChangeCalls, "no breaking changes => hook must not fire")
}

func TestRegisterSchema_CheckCompatibility_BreakingDetected_FiresHook(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()
	rec := &recordingHooks{}
	service.SetHooks(rec)

	setupResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "compat-breaking",
		SchemaContent: compatTestV1Schema,
	})
	require.NoError(t, err)
	require.True(t, setupResp.Success, "v1 setup must succeed: %s", setupResp.Message)
	rec.breakingChangeCalls = nil

	resp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:           "compat-breaking",
		SchemaContent:      compatTestV2Schema, // /items removed
		CheckCompatibility: true,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success, "breaking changes are reported, not refused")
	require.NotEmpty(t, resp.BreakingChanges, "expected ENDPOINT_REMOVED in response")

	require.Len(t, rec.breakingChangeCalls, 1, "hook should fire exactly once")
	got := rec.breakingChangeCalls[0]
	assert.Equal(t, "compat-breaking", got.SchemaId)
	assert.Equal(t, "1.0.0", got.OldVersion)
	assert.Equal(t, "2.0.0", got.NewVersion)
	assert.Equal(t, "RegisterSchema", got.DetectedBy)
	assert.NotEmpty(t, got.Changes)
}

// errorStore wraps a real Store but returns an error from GetSchema.
// Used to drive decision 1C: storage error during prior-version lookup
// must fail-close the registration. All other Store calls forward to the
// embedded MemoryStore so service.Close() and friends behave normally.
type errorStore struct {
	storage.Store
	getSchemaErr error
}

func newErrorStore(err error) *errorStore {
	return &errorStore{Store: storage.NewMemoryStore(), getSchemaErr: err}
}

func (e *errorStore) GetSchema(_ context.Context, _ string) (*storage.SchemaRecord, error) {
	return nil, e.getSchemaErr
}

func TestRegisterSchema_CheckCompatibility_StorageError_FailsClosed(t *testing.T) {
	// Build a service with a store that errors on GetSchema. The cache
	// is empty for this schema_id so getSchemaEntry path is bypassed in
	// favor of the store; the store error must surface as fail-closed.
	store := newErrorStore(errors.New("simulated postgres timeout"))
	service, err := NewValidatorServiceWithStore(store)
	require.NoError(t, err)
	defer service.Close()

	rec := &recordingHooks{}
	service.SetHooks(rec)

	resp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:           "compat-storage-err",
		SchemaContent:      compatTestV1Schema,
		CheckCompatibility: true,
	})
	require.NoError(t, err, "gRPC call returns response, not error (codebase convention)")
	assert.False(t, resp.Success, "fail-closed: must refuse the registration on storage error (decision 1C)")
	assert.Contains(t, resp.Message, "compatibility")
	assert.Contains(t, resp.Message, "storage error")
	assert.Empty(t, rec.breakingChangeCalls, "no fire when registration was refused")
}

func TestCompareSchemas_BreakingDetected_FiresHook(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()
	rec := &recordingHooks{}
	service.SetHooks(rec)

	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "compare-fire",
		SchemaContent: compatTestV1Schema,
		SchemaVersion: "1.0.0",
	})
	require.NoError(t, err)
	_, err = service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "compare-fire",
		SchemaContent: compatTestV2Schema,
		SchemaVersion: "2.0.0",
	})
	require.NoError(t, err)
	rec.breakingChangeCalls = nil

	resp, err := service.CompareSchemas(context.Background(), &pb.CompareSchemasRequest{
		SchemaId:   "compare-fire",
		OldVersion: "1.0.0",
		NewVersion: "2.0.0",
	})
	require.NoError(t, err)
	assert.False(t, resp.Compatible)
	require.NotEmpty(t, resp.BreakingChanges)
	require.Len(t, rec.breakingChangeCalls, 1)
	got := rec.breakingChangeCalls[0]
	assert.Equal(t, "compare-fire", got.SchemaId)
	assert.Equal(t, "CompareSchemas", got.DetectedBy)
}
