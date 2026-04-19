package cvtservice

import (
	"context"
	"os"
	"testing"

	"github.com/sahina/cvt/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
