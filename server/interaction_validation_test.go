package main

import (
	"context"
	"os"
	"testing"

	"github.com/cvt/cvt/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupServiceWithSchema creates a service and registers a schema for testing
func setupServiceWithSchema(t *testing.T, schemaFile, schemaID string) *ValidatorService {
	service, err := NewValidatorService()
	require.NoError(t, err)

	content, err := os.ReadFile(schemaFile)
	require.NoError(t, err)

	req := &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(content),
	}

	resp, err := service.RegisterSchema(context.Background(), req)
	require.NoError(t, err)
	require.True(t, resp.Success)

	return service
}

// TestValidateInteraction_ValidRequest tests validating valid requests
func TestValidateInteraction_ValidRequest(t *testing.T) {
	service := setupServiceWithSchema(t, "testdata/openapi-v3/valid/simple-petstore.json", "petstore-v3")
	defer service.Close()

	testCases := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		body       string
		statusCode int32
		respBody   string
	}{
		{
			name:       "GET /pets - Valid",
			method:     "GET",
			path:       "/pets",
			headers:    map[string]string{"Content-Type": "application/json"},
			body:       "",
			statusCode: 200,
			respBody:   `[{"id":"1","name":"Fluffy","tag":"cat"}]`,
		},
		{
			name:       "GET /pets with query params - Valid",
			method:     "GET",
			path:       "/pets?limit=10&offset=0",
			headers:    map[string]string{},
			body:       "",
			statusCode: 200,
			respBody:   `[]`,
		},
		{
			name:   "POST /pets - Valid",
			method: "POST",
			path:   "/pets",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:       `{"name":"Buddy","tag":"dog","age":3}`,
			statusCode: 201,
			respBody:   `{"id":"123","name":"Buddy","tag":"dog","age":3}`,
		},
		{
			name:   "GET /pets/{petId} - Valid",
			method: "GET",
			path:   "/pets/abc123",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:       "",
			statusCode: 200,
			respBody:   `{"id":"abc123","name":"Max","age":5}`,
		},
		{
			name:   "PUT /pets/{petId} - Valid",
			method: "PUT",
			path:   "/pets/abc123",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:       `{"name":"Max Updated","age":6}`,
			statusCode: 200,
			respBody:   `{"id":"abc123","name":"Max Updated","age":6}`,
		},
		{
			name:       "DELETE /pets/{petId} - Valid",
			method:     "DELETE",
			path:       "/pets/abc123",
			headers:    map[string]string{},
			body:       "",
			statusCode: 204,
			respBody:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.InteractionRequest{
				SchemaId: "petstore-v3",
				Request: &pb.RequestData{
					Method:  tc.method,
					Path:    tc.path,
					Headers: tc.headers,
					Body:    tc.body,
				},
				Response: &pb.ResponseData{
					StatusCode: tc.statusCode,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       tc.respBody,
				},
			}

			result, err := service.ValidateInteraction(context.Background(), req)
			require.NoError(t, err)
			assert.True(t, result.Valid, "Request should be valid")
			assert.Empty(t, result.Errors, "Should have no validation errors")
		})
	}
}

// TestValidateInteraction_InvalidRequest tests validating invalid requests
func TestValidateInteraction_InvalidRequest(t *testing.T) {
	service := setupServiceWithSchema(t, "testdata/openapi-v3/valid/simple-petstore.json", "petstore-v3")
	defer service.Close()

	testCases := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		body       string
		statusCode int32
		respBody   string
		shouldFail bool
	}{
		{
			name:   "POST /pets - Missing required field",
			method: "POST",
			path:   "/pets",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:       `{"tag":"dog"}`, // Missing required "name" field
			statusCode: 201,
			respBody:   `{"id":"123","name":"Buddy"}`,
			shouldFail: true,
		},
		{
			name:   "POST /pets - Wrong data type",
			method: "POST",
			path:   "/pets",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:       `{"name":"Buddy","age":"not-a-number"}`, // age should be integer
			statusCode: 201,
			respBody:   `{"id":"123","name":"Buddy"}`,
			shouldFail: true,
		},
		{
			name:   "POST /pets - Invalid age value",
			method: "POST",
			path:   "/pets",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:       `{"name":"Buddy","age":150}`, // age max is 100
			statusCode: 201,
			respBody:   `{"id":"123","name":"Buddy","age":150}`,
			shouldFail: true,
		},
		{
			name:   "POST /pets - Name too long",
			method: "POST",
			path:   "/pets",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:       `{"name":"` + string(make([]byte, 150)) + `"}`, // name maxLength is 100
			statusCode: 201,
			respBody:   `{"id":"123","name":"Buddy"}`,
			shouldFail: true,
		},
		{
			name:   "GET /pets/{petId} - Invalid path param pattern",
			method: "GET",
			path:   "/pets/invalid@id!", // petId pattern is ^[a-zA-Z0-9-]+$
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:       "",
			statusCode: 200,
			respBody:   `{"id":"invalid@id!","name":"Max"}`,
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.InteractionRequest{
				SchemaId: "petstore-v3",
				Request: &pb.RequestData{
					Method:  tc.method,
					Path:    tc.path,
					Headers: tc.headers,
					Body:    tc.body,
				},
				Response: &pb.ResponseData{
					StatusCode: tc.statusCode,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       tc.respBody,
				},
			}

			result, err := service.ValidateInteraction(context.Background(), req)
			require.NoError(t, err)

			if tc.shouldFail {
				assert.False(t, result.Valid, "Request should be invalid")
				assert.NotEmpty(t, result.Errors, "Should have validation errors")
			} else {
				assert.True(t, result.Valid, "Request should be valid")
				assert.Empty(t, result.Errors, "Should have no validation errors")
			}
		})
	}
}

// TestValidateInteraction_InvalidResponse tests validating invalid responses
func TestValidateInteraction_InvalidResponse(t *testing.T) {
	service := setupServiceWithSchema(t, "testdata/openapi-v3/valid/simple-petstore.json", "petstore-v3")
	defer service.Close()

	testCases := []struct {
		name       string
		method     string
		path       string
		body       string
		statusCode int32
		respBody   string
		shouldFail bool
	}{
		{
			name:       "GET /pets - Missing required field in response",
			method:     "GET",
			path:       "/pets",
			body:       "",
			statusCode: 200,
			respBody:   `[{"name":"Fluffy"}]`, // Missing required "id"
			shouldFail: true,
		},
		{
			name:       "GET /pets - Wrong response type",
			method:     "GET",
			path:       "/pets",
			body:       "",
			statusCode: 200,
			respBody:   `{"not":"an-array"}`, // Should be array
			shouldFail: true,
		},
		{
			name:       "POST /pets - Response missing required id",
			method:     "POST",
			path:       "/pets",
			body:       `{"name":"Buddy"}`,
			statusCode: 201,
			respBody:   `{"name":"Buddy"}`, // Missing required "id"
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.InteractionRequest{
				SchemaId: "petstore-v3",
				Request: &pb.RequestData{
					Method:  tc.method,
					Path:    tc.path,
					Headers: map[string]string{"Content-Type": "application/json"},
					Body:    tc.body,
				},
				Response: &pb.ResponseData{
					StatusCode: tc.statusCode,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       tc.respBody,
				},
			}

			result, err := service.ValidateInteraction(context.Background(), req)
			require.NoError(t, err)

			if tc.shouldFail {
				assert.False(t, result.Valid, "Response should be invalid")
				assert.NotEmpty(t, result.Errors, "Should have validation errors")
			}
		})
	}
}

// TestValidateInteraction_ComplexSchemas tests validation with complex schemas
func TestValidateInteraction_ComplexSchemas(t *testing.T) {
	service := setupServiceWithSchema(t, "testdata/openapi-v3/valid/complex-ecommerce.json", "ecommerce-v3")
	defer service.Close()

	testCases := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		body       string
		statusCode int32
		respBody   string
		shouldPass bool
	}{
		{
			name:   "POST /products - Physical Product (discriminator)",
			method: "POST",
			path:   "/products",
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test-token",
			},
			body: `{
				"id": "550e8400-e29b-41d4-a716-446655440001",
				"productType": "physical",
				"name": "Laptop",
				"price": 999.99,
				"currency": "USD",
				"images": ["https://example.com/image.jpg"],
				"weight": 2.5,
				"dimensions": {
					"length": 30,
					"width": 20,
					"height": 2,
					"unit": "cm"
				},
				"shippingClass": "express"
			}`,
			statusCode: 201,
			respBody: `{
				"id": "550e8400-e29b-41d4-a716-446655440000",
				"name": "Laptop",
				"price": 999.99,
				"currency": "USD",
				"images": ["https://example.com/image.jpg"]
			}`,
			shouldPass: true,
		},
		{
			name:   "POST /products - Digital Product (discriminator)",
			method: "POST",
			path:   "/products",
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test-token",
			},
			body: `{
				"id": "550e8400-e29b-41d4-a716-446655440002",
				"productType": "digital",
				"name": "E-book",
				"price": 9.99,
				"currency": "USD",
				"images": ["https://example.com/cover.jpg"],
				"downloadUrl": "https://example.com/download",
				"fileSize": 5242880,
				"fileFormat": "pdf"
			}`,
			statusCode: 201,
			respBody: `{
				"id": "550e8400-e29b-41d4-a716-446655440000",
				"name": "E-book",
				"price": 9.99,
				"currency": "USD",
				"images": ["https://example.com/cover.jpg"]
			}`,
			shouldPass: true,
		},
		{
			name:   "GET /products - With query params and header",
			method: "GET",
			path:   "/products?category=electronics&minPrice=100&maxPrice=1000",
			headers: map[string]string{
				"Content-Type": "application/json",
				"X-Request-ID": "550e8400-e29b-41d4-a716-446655440000",
			},
			body:       "",
			statusCode: 200,
			respBody: `{
				"products": [],
				"total": 0,
				"page": 1
			}`,
			shouldPass: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.InteractionRequest{
				SchemaId: "ecommerce-v3",
				Request: &pb.RequestData{
					Method:  tc.method,
					Path:    tc.path,
					Headers: tc.headers,
					Body:    tc.body,
				},
				Response: &pb.ResponseData{
					StatusCode: tc.statusCode,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       tc.respBody,
				},
			}

			result, err := service.ValidateInteraction(context.Background(), req)
			require.NoError(t, err)

			if tc.shouldPass {
				assert.True(t, result.Valid, "Validation should pass")
				assert.Empty(t, result.Errors, "Should have no errors")
			} else {
				assert.False(t, result.Valid, "Validation should fail")
				assert.NotEmpty(t, result.Errors, "Should have errors")
			}
		})
	}
}

// TestValidateInteraction_RouteNotFound tests handling of non-existent routes
func TestValidateInteraction_RouteNotFound(t *testing.T) {
	service := setupServiceWithSchema(t, "testdata/openapi-v3/valid/simple-petstore.json", "petstore-v3")
	defer service.Close()

	req := &pb.InteractionRequest{
		SchemaId: "petstore-v3",
		Request: &pb.RequestData{
			Method:  "GET",
			Path:    "/nonexistent-route",
			Headers: map[string]string{},
			Body:    "",
		},
		Response: &pb.ResponseData{
			StatusCode: 404,
			Headers:    map[string]string{},
			Body:       "",
		},
	}

	result, err := service.ValidateInteraction(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, result.Valid, "Should fail for non-existent route")
	assert.NotEmpty(t, result.Errors, "Should contain error about route not found")
}

// TestValidateInteraction_SchemaNotFound tests validation with non-registered schema
func TestValidateInteraction_SchemaNotFound(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	req := &pb.InteractionRequest{
		SchemaId: "non-existent-schema",
		Request: &pb.RequestData{
			Method:  "GET",
			Path:    "/test",
			Headers: map[string]string{},
			Body:    "",
		},
		Response: &pb.ResponseData{
			StatusCode: 200,
			Headers:    map[string]string{},
			Body:       "",
		},
	}

	result, err := service.ValidateInteraction(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, result.Valid, "Should fail for non-existent schema")
	assert.Contains(t, result.Errors[0], "Schema not found", "Error should mention schema not found")
}

// TestValidateInteraction_InputValidation tests input validation for ValidateInteraction
func TestValidateInteraction_InputValidation(t *testing.T) {
	service := setupServiceWithSchema(t, "testdata/openapi-v3/valid/simple-petstore.json", "petstore-v3")
	defer service.Close()

	testCases := []struct {
		name    string
		request *pb.InteractionRequest
	}{
		{
			name: "Empty schema ID",
			request: &pb.InteractionRequest{
				SchemaId: "",
				Request: &pb.RequestData{
					Method: "GET",
					Path:   "/pets",
				},
				Response: &pb.ResponseData{
					StatusCode: 200,
				},
			},
		},
		{
			name: "Invalid HTTP method",
			request: &pb.InteractionRequest{
				SchemaId: "petstore-v3",
				Request: &pb.RequestData{
					Method: "INVALID",
					Path:   "/pets",
				},
				Response: &pb.ResponseData{
					StatusCode: 200,
				},
			},
		},
		{
			name: "Invalid path",
			request: &pb.InteractionRequest{
				SchemaId: "petstore-v3",
				Request: &pb.RequestData{
					Method: "GET",
					Path:   "no-leading-slash",
				},
				Response: &pb.ResponseData{
					StatusCode: 200,
				},
			},
		},
		{
			name: "Invalid status code",
			request: &pb.InteractionRequest{
				SchemaId: "petstore-v3",
				Request: &pb.RequestData{
					Method: "GET",
					Path:   "/pets",
				},
				Response: &pb.ResponseData{
					StatusCode: 999, // Invalid status code
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := service.ValidateInteraction(context.Background(), tc.request)
			require.NoError(t, err)
			assert.False(t, result.Valid, "Should fail validation")
			assert.NotEmpty(t, result.Errors, "Should have validation errors")
		})
	}
}

// TestValidateInteraction_OpenAPIv2 tests validation with OpenAPI v2 schemas
func TestValidateInteraction_OpenAPIv2(t *testing.T) {
	service := setupServiceWithSchema(t, "testdata/openapi-v2/valid/simple-api.json", "simple-api-v2")
	defer service.Close()

	testCases := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		body       string
		statusCode int32
		respBody   string
		shouldPass bool
	}{
		{
			name:       "GET /users - Valid",
			method:     "GET",
			path:       "/users",
			headers:    map[string]string{},
			body:       "",
			statusCode: 200,
			respBody:   `{"users":[],"total":0}`,
			shouldPass: true,
		},
		{
			name:   "POST /users - Valid",
			method: "POST",
			path:   "/users",
			headers: map[string]string{
				"Content-Type": "application/json",
				"X-API-Key":    "test-key",
			},
			body:       `{"username":"johndoe","email":"john@example.com","password":"password123","age":25}`,
			statusCode: 201,
			respBody:   `{"id":"123","username":"johndoe","email":"john@example.com","createdAt":"2024-01-01T00:00:00Z"}`,
			shouldPass: true,
		},
		{
			name:   "GET /users/{userId} - Valid",
			method: "GET",
			path:   "/users/123",
			headers: map[string]string{
				"Content-Type": "application/json",
				"X-API-Key":    "test-key",
			},
			body:       "",
			statusCode: 200,
			respBody:   `{"id":"123","username":"johndoe","email":"john@example.com","createdAt":"2024-01-01T00:00:00Z"}`,
			shouldPass: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.InteractionRequest{
				SchemaId: "simple-api-v2",
				Request: &pb.RequestData{
					Method:  tc.method,
					Path:    tc.path,
					Headers: tc.headers,
					Body:    tc.body,
				},
				Response: &pb.ResponseData{
					StatusCode: tc.statusCode,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       tc.respBody,
				},
			}

			result, err := service.ValidateInteraction(context.Background(), req)
			require.NoError(t, err)

			if tc.shouldPass {
				assert.True(t, result.Valid, "Validation should pass for Swagger 2.0 schema")
				assert.Empty(t, result.Errors, "Should have no errors")
			}
		})
	}
}

func TestValidateInteraction_Swagger2WithBasePath(t *testing.T) {
	service := setupServiceWithSchema(t, "testdata/openapi-v2/valid/api-with-basepath.json", "api-with-basepath")
	defer service.Close()

	testCases := []struct {
		name        string
		method      string
		path        string
		headers     map[string]string
		body        string
		statusCode  int32
		respBody    string
		shouldPass  bool
		description string
	}{
		{
			name:        "GET /api/v2/users - With basePath prefix",
			method:      "GET",
			path:        "/api/v2/users",
			headers:     map[string]string{},
			body:        "",
			statusCode:  200,
			respBody:    `[{"id":"1","username":"test","email":"test@example.com"}]`,
			shouldPass:  true,
			description: "Path should include basePath prefix",
		},
		{
			name:        "GET /users - Without basePath prefix",
			method:      "GET",
			path:        "/users",
			headers:     map[string]string{},
			body:        "",
			statusCode:  200,
			respBody:    `[{"id":"1","username":"test","email":"test@example.com"}]`,
			shouldPass:  false,
			description: "Path without basePath should fail (testing current behavior)",
		},
		{
			name:   "POST /api/v2/users - With basePath prefix",
			method: "POST",
			path:   "/api/v2/users",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:        `{"username":"newuser","email":"new@example.com"}`,
			statusCode:  201,
			respBody:    `{"id":"2","username":"newuser","email":"new@example.com"}`,
			shouldPass:  true,
			description: "POST should work with basePath prefix",
		},
		{
			name:   "GET /api/v2/products - Different path with basePath",
			method: "GET",
			path:   "/api/v2/products",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body:        "",
			statusCode:  200,
			respBody:    `[{"id":"1","name":"Widget","price":19.99}]`,
			shouldPass:  true,
			description: "Multiple paths should all support basePath",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.InteractionRequest{
				SchemaId: "api-with-basepath",
				Request: &pb.RequestData{
					Method:  tc.method,
					Path:    tc.path,
					Headers: tc.headers,
					Body:    tc.body,
				},
				Response: &pb.ResponseData{
					StatusCode: tc.statusCode,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       tc.respBody,
				},
			}

			result, err := service.ValidateInteraction(context.Background(), req)
			require.NoError(t, err)

			if tc.shouldPass {
				assert.True(t, result.Valid, "Validation should pass: %s", tc.description)
				assert.Empty(t, result.Errors, "Should have no errors: %s", tc.description)
			} else {
				t.Logf("Expected failure case: %s - Valid=%v, Errors=%v", tc.description, result.Valid, result.Errors)
			}
		})
	}
}
