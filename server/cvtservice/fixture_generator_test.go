package cvtservice

import (
	"context"
	"os"
	"testing"

	"github.com/sahina/cvt/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
