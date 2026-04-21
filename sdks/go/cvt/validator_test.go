package cvt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/sahina/cvt/sdks/go/cvt/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// mockClient implements pb.ContractValidatorClient
type mockClient struct {
	registerSchemaFunc      func(ctx context.Context, in *pb.RegisterSchemaRequest, opts ...grpc.CallOption) (*pb.RegisterSchemaResponse, error)
	validateInteractionFunc func(ctx context.Context, in *pb.InteractionRequest, opts ...grpc.CallOption) (*pb.ValidationResult, error)
	getSchemasFunc          func(ctx context.Context, in *pb.GetSchemaRequest, opts ...grpc.CallOption) (*pb.GetSchemaResponse, error)
	listSchemasFunc         func(ctx context.Context, in *pb.ListSchemasRequest, opts ...grpc.CallOption) (*pb.ListSchemasResponse, error)
	compareSchemasFunc      func(ctx context.Context, in *pb.CompareSchemasRequest, opts ...grpc.CallOption) (*pb.CompareSchemasResponse, error)
	generateFixtureFunc     func(ctx context.Context, in *pb.GenerateFixtureRequest, opts ...grpc.CallOption) (*pb.GenerateFixtureResponse, error)
	listEndpointsFunc       func(ctx context.Context, in *pb.ListEndpointsRequest, opts ...grpc.CallOption) (*pb.ListEndpointsResponse, error)
}

func (m *mockClient) RegisterSchema(ctx context.Context, in *pb.RegisterSchemaRequest, opts ...grpc.CallOption) (*pb.RegisterSchemaResponse, error) {
	if m.registerSchemaFunc != nil {
		return m.registerSchemaFunc(ctx, in, opts...)
	}
	return &pb.RegisterSchemaResponse{Success: true}, nil
}

func (m *mockClient) ValidateInteraction(ctx context.Context, in *pb.InteractionRequest, opts ...grpc.CallOption) (*pb.ValidationResult, error) {
	if m.validateInteractionFunc != nil {
		return m.validateInteractionFunc(ctx, in, opts...)
	}
	return &pb.ValidationResult{Valid: true}, nil
}

func (m *mockClient) GetSchema(ctx context.Context, in *pb.GetSchemaRequest, opts ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
	if m.getSchemasFunc != nil {
		return m.getSchemasFunc(ctx, in, opts...)
	}
	return &pb.GetSchemaResponse{Found: true}, nil
}

func (m *mockClient) ListSchemas(ctx context.Context, in *pb.ListSchemasRequest, opts ...grpc.CallOption) (*pb.ListSchemasResponse, error) {
	if m.listSchemasFunc != nil {
		return m.listSchemasFunc(ctx, in, opts...)
	}
	return &pb.ListSchemasResponse{}, nil
}

func (m *mockClient) CompareSchemas(ctx context.Context, in *pb.CompareSchemasRequest, opts ...grpc.CallOption) (*pb.CompareSchemasResponse, error) {
	if m.compareSchemasFunc != nil {
		return m.compareSchemasFunc(ctx, in, opts...)
	}
	return &pb.CompareSchemasResponse{Compatible: true}, nil
}

func (m *mockClient) GenerateFixture(ctx context.Context, in *pb.GenerateFixtureRequest, opts ...grpc.CallOption) (*pb.GenerateFixtureResponse, error) {
	if m.generateFixtureFunc != nil {
		return m.generateFixtureFunc(ctx, in, opts...)
	}
	return &pb.GenerateFixtureResponse{Success: true}, nil
}

func (m *mockClient) ListEndpoints(ctx context.Context, in *pb.ListEndpointsRequest, opts ...grpc.CallOption) (*pb.ListEndpointsResponse, error) {
	if m.listEndpointsFunc != nil {
		return m.listEndpointsFunc(ctx, in, opts...)
	}
	return &pb.ListEndpointsResponse{}, nil
}

func (m *mockClient) ValidateProducerResponse(ctx context.Context, in *pb.ValidateProducerRequest, opts ...grpc.CallOption) (*pb.ValidationResult, error) {
	return &pb.ValidationResult{Valid: true}, nil
}

func (m *mockClient) RegisterConsumer(ctx context.Context, in *pb.RegisterConsumerRequest, opts ...grpc.CallOption) (*pb.RegisterConsumerResponse, error) {
	return &pb.RegisterConsumerResponse{Success: true}, nil
}

func (m *mockClient) ListConsumers(ctx context.Context, in *pb.ListConsumersRequest, opts ...grpc.CallOption) (*pb.ListConsumersResponse, error) {
	return &pb.ListConsumersResponse{}, nil
}

func (m *mockClient) DeregisterConsumer(ctx context.Context, in *pb.DeregisterConsumerRequest, opts ...grpc.CallOption) (*pb.DeregisterConsumerResponse, error) {
	return &pb.DeregisterConsumerResponse{Success: true}, nil
}

func (m *mockClient) CanIDeploy(ctx context.Context, in *pb.CanIDeployRequest, opts ...grpc.CallOption) (*pb.CanIDeployResponse, error) {
	return &pb.CanIDeployResponse{SafeToDeploy: true}, nil
}

// TestNewValidator tests the validator constructor
func TestNewValidator(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "default address",
			address: "",
			wantErr: false,
		},
		{
			name:    "custom address",
			address: "localhost:9550",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator(tt.address)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, validator)
			defer func() { _ = validator.Close() }()
		})
	}
}

// TestRegisterSchema tests schema registration
func TestRegisterSchema(t *testing.T) {
	ctx := context.Background()

	// Get the shared schema path for local file test
	schemaPath := filepath.Join("..", "..", "shared", "openapi.json")

	// Start a local test server for URL tests
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"openapi": "3.0.0"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	tests := []struct {
		name       string
		schemaID   string
		schemaPath string
		mockResp   *pb.RegisterSchemaResponse
		mockErr    error
		wantErr    bool
	}{
		{
			name:       "register schema from local file",
			schemaID:   "test-schema",
			schemaPath: schemaPath,
			mockResp:   &pb.RegisterSchemaResponse{Success: true},
			wantErr:    false,
		},
		{
			name:       "register schema from URL",
			schemaID:   "url-test-schema",
			schemaPath: ts.URL + "/openapi.json",
			mockResp:   &pb.RegisterSchemaResponse{Success: true},
			wantErr:    false,
		},
		{
			name:       "register schema from 404 URL",
			schemaID:   "404-schema",
			schemaPath: ts.URL + "/notfound",
			wantErr:    true,
		},
		{
			name:       "register schema failure from server",
			schemaID:   "fail-schema",
			schemaPath: schemaPath,
			mockResp:   &pb.RegisterSchemaResponse{Success: false, Message: "registration failed"},
			wantErr:    true,
		},
		{
			name:       "register schema gRPC error",
			schemaID:   "error-schema",
			schemaPath: schemaPath,
			mockErr:    errors.New("gRPC error"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			// Inject mock client
			validator.client = &mockClient{
				registerSchemaFunc: func(ctx context.Context, in *pb.RegisterSchemaRequest, opts ...grpc.CallOption) (*pb.RegisterSchemaResponse, error) {
					assert.Equal(t, tt.schemaID, in.SchemaId)
					assert.NotEmpty(t, in.SchemaContent)
					return tt.mockResp, tt.mockErr
				},
			}

			err = validator.RegisterSchema(ctx, tt.schemaID, tt.schemaPath)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate tests validation functionality
func TestValidate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		request   ValidationRequest
		response  ValidationResponse
		mockValid bool
		mockErrs  []string
		mockErr   error
		wantValid bool
		wantErr   bool
	}{
		{
			name: "valid pet creation",
			request: ValidationRequest{
				Method:  "POST",
				Path:    "/pet",
				Headers: map[string]string{"content-type": "application/json"},
				Body:    map[string]interface{}{"name": "Fluffy"},
			},
			response:  ValidationResponse{StatusCode: 405},
			mockValid: true,
			wantValid: true,
			wantErr:   false,
		},
		{
			name:      "invalid pet creation",
			request:   ValidationRequest{Method: "POST", Path: "/pet"},
			response:  ValidationResponse{StatusCode: 400},
			mockValid: false,
			mockErrs:  []string{"missing field"},
			wantValid: false,
			wantErr:   false,
		},
		{
			name:     "validation gRPC error",
			request:  ValidationRequest{Method: "GET", Path: "/"},
			response: ValidationResponse{StatusCode: 200},
			mockErr:  errors.New("gRPC error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			// Manually set schemaID to simulate registered state
			validator.schemaID = "test-schema"

			// Inject mock client
			validator.client = &mockClient{
				validateInteractionFunc: func(ctx context.Context, in *pb.InteractionRequest, opts ...grpc.CallOption) (*pb.ValidationResult, error) {
					assert.Equal(t, "test-schema", in.SchemaId)
					assert.Equal(t, tt.request.Method, in.Request.Method)
					return &pb.ValidationResult{Valid: tt.mockValid, Errors: tt.mockErrs}, tt.mockErr
				},
			}

			result, err := validator.Validate(ctx, tt.request, tt.response)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.wantValid {
				assert.True(t, result.Valid)
			} else {
				assert.False(t, result.Valid)
				assert.Equal(t, tt.mockErrs, result.Errors)
			}
		})
	}
}

// TestValidateWithoutSchema tests that validation fails without schema registration
func TestValidateWithoutSchema(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	// Do NOT set schemaID

	request := ValidationRequest{Method: "GET", Path: "/"}
	response := ValidationResponse{StatusCode: 200}

	_, err = validator.Validate(ctx, request, response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema not registered")
}

// TestClose tests closing the validator
func TestClose(t *testing.T) {
	validator, err := NewValidator("")
	require.NoError(t, err)

	err = validator.Close()
	assert.NoError(t, err)
}

// TestCloseNilConn tests closing with nil connection
func TestCloseNilConn(t *testing.T) {
	validator := &Validator{conn: nil}
	err := validator.Close()
	assert.NoError(t, err)
}

// TestRegisterSchemaFileNotFound tests registering schema from non-existent file
func TestRegisterSchemaFileNotFound(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	err = validator.RegisterSchema(ctx, "test", "/nonexistent/file.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load schema")
}

// TestRegisterSchemaInvalidURL tests registering schema from invalid URL
func TestRegisterSchemaInvalidURL(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	err = validator.RegisterSchema(ctx, "test", "http://invalid.localhost.test:99999/schema.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load schema")
}

// TestValidateRequestBodyMarshalError tests validation with unmarshalable request body
func TestValidateRequestBodyMarshalError(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.schemaID = "test-schema"
	validator.client = &mockClient{}

	// Create a body that cannot be marshaled (channel)
	request := ValidationRequest{
		Method: "POST",
		Path:   "/test",
		Body:   make(chan int),
	}
	response := ValidationResponse{StatusCode: 200}

	_, err = validator.Validate(ctx, request, response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal request body")
}

// TestValidateResponseBodyMarshalError tests validation with unmarshalable response body
func TestValidateResponseBodyMarshalError(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.schemaID = "test-schema"
	validator.client = &mockClient{}

	request := ValidationRequest{
		Method: "GET",
		Path:   "/test",
	}
	// Create a response body that cannot be marshaled (channel)
	response := ValidationResponse{
		StatusCode: 200,
		Body:       make(chan int),
	}

	_, err = validator.Validate(ctx, request, response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal response body")
}

// TestFetchSchemaFromURLHTTPS tests fetching schema from HTTPS URL
func TestFetchSchemaFromURLHTTPS(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	// Test with https:// prefix detection (will fail to connect but tests the path)
	err = validator.RegisterSchema(ctx, "test", "https://invalid.localhost.test/schema.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load schema")
}

// TestGenerateFixture tests fixture generation
func TestGenerateFixture(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		method   string
		path     string
		opts     *GenerateOptions
		mockResp *pb.GenerateFixtureResponse
		mockErr  error
		wantErr  bool
	}{
		{
			name:   "generate fixture success",
			method: "POST",
			path:   "/users",
			opts:   nil,
			mockResp: &pb.GenerateFixtureResponse{
				Success: true,
				Fixture: &pb.GeneratedFixture{
					Request: &pb.GeneratedRequest{
						Method: "POST",
						Path:   "/users",
						Body:   `{"name":"test"}`,
					},
					Response: &pb.GeneratedResponse{
						StatusCode: 201,
						Body:       `{"id":1,"name":"test"}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "generate fixture with options",
			method: "get",
			path:   "/users/123",
			opts: &GenerateOptions{
				StatusCode:  200,
				UseExamples: true,
				ContentType: "application/json",
			},
			mockResp: &pb.GenerateFixtureResponse{
				Success: true,
				Fixture: &pb.GeneratedFixture{
					Request: &pb.GeneratedRequest{
						Method: "GET",
						Path:   "/users/123",
					},
					Response: &pb.GeneratedResponse{
						StatusCode: 200,
						Body:       `{"id":123}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "generate fixture failure",
			method:   "POST",
			path:     "/invalid",
			mockResp: &pb.GenerateFixtureResponse{Success: false, Message: "path not found"},
			wantErr:  true,
		},
		{
			name:    "generate fixture gRPC error",
			method:  "GET",
			path:    "/test",
			mockErr: errors.New("gRPC error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			validator.schemaID = "test-schema"
			validator.client = &mockClient{
				generateFixtureFunc: func(ctx context.Context, in *pb.GenerateFixtureRequest, opts ...grpc.CallOption) (*pb.GenerateFixtureResponse, error) {
					assert.Equal(t, "test-schema", in.SchemaId)
					assert.Equal(t, pb.OutputType_OUTPUT_FIXTURE, in.OutputType)
					return tt.mockResp, tt.mockErr
				},
			}

			result, err := validator.GenerateFixture(ctx, tt.method, tt.path, tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.NotNil(t, result.Request)
			assert.NotNil(t, result.Response)
		})
	}
}

// TestGenerateFixtureWithoutSchema tests that fixture generation fails without schema registration
func TestGenerateFixtureWithoutSchema(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	_, err = validator.GenerateFixture(ctx, "POST", "/test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema not registered")
}

// TestGenerateResponse tests response generation
func TestGenerateResponse(t *testing.T) {
	ctx := context.Background()

	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.schemaID = "test-schema"
	validator.client = &mockClient{
		generateFixtureFunc: func(ctx context.Context, in *pb.GenerateFixtureRequest, opts ...grpc.CallOption) (*pb.GenerateFixtureResponse, error) {
			assert.Equal(t, pb.OutputType_OUTPUT_RESPONSE, in.OutputType)
			return &pb.GenerateFixtureResponse{
				Success: true,
				Response: &pb.GeneratedResponse{
					StatusCode: 200,
					Body:       `{"status":"ok"}`,
				},
			}, nil
		},
	}

	result, err := validator.GenerateResponse(ctx, "GET", "/health", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 200, result.StatusCode)
	assert.NotNil(t, result.Body)
}

// TestGenerateResponseWithoutSchema tests that response generation fails without schema registration
func TestGenerateResponseWithoutSchema(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	_, err = validator.GenerateResponse(ctx, "GET", "/test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema not registered")
}

// TestGenerateRequestBody tests request body generation
func TestGenerateRequestBody(t *testing.T) {
	ctx := context.Background()

	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.schemaID = "test-schema"
	validator.client = &mockClient{
		generateFixtureFunc: func(ctx context.Context, in *pb.GenerateFixtureRequest, opts ...grpc.CallOption) (*pb.GenerateFixtureResponse, error) {
			assert.Equal(t, pb.OutputType_OUTPUT_REQUEST, in.OutputType)
			return &pb.GenerateFixtureResponse{
				Success:     true,
				RequestBody: `{"name":"test","email":"test@example.com"}`,
			}, nil
		},
	}

	result, err := validator.GenerateRequestBody(ctx, "POST", "/users", nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Result should be a parsed map
	bodyMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test", bodyMap["name"])
	assert.Equal(t, "test@example.com", bodyMap["email"])
}

// TestGenerateRequestBodyEmpty tests request body generation with empty body
func TestGenerateRequestBodyEmpty(t *testing.T) {
	ctx := context.Background()

	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.schemaID = "test-schema"
	validator.client = &mockClient{
		generateFixtureFunc: func(ctx context.Context, in *pb.GenerateFixtureRequest, opts ...grpc.CallOption) (*pb.GenerateFixtureResponse, error) {
			return &pb.GenerateFixtureResponse{
				Success:     true,
				RequestBody: "",
			}, nil
		},
	}

	result, err := validator.GenerateRequestBody(ctx, "GET", "/test", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

// TestGenerateRequestBodyWithoutSchema tests that request body generation fails without schema registration
func TestGenerateRequestBodyWithoutSchema(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	_, err = validator.GenerateRequestBody(ctx, "POST", "/test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema not registered")
}

// TestListEndpoints tests listing endpoints
func TestListEndpoints(t *testing.T) {
	ctx := context.Background()

	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.schemaID = "test-schema"
	validator.client = &mockClient{
		listEndpointsFunc: func(ctx context.Context, in *pb.ListEndpointsRequest, opts ...grpc.CallOption) (*pb.ListEndpointsResponse, error) {
			assert.Equal(t, "test-schema", in.SchemaId)
			return &pb.ListEndpointsResponse{
				Endpoints: []*pb.EndpointInfo{
					{Method: "GET", Path: "/users", OperationId: "getUsers", Summary: "Get all users"},
					{Method: "POST", Path: "/users", OperationId: "createUser", Summary: "Create a user"},
					{Method: "GET", Path: "/users/{id}", OperationId: "getUser", Summary: "Get a user"},
				},
			}, nil
		},
	}

	endpoints, err := validator.ListEndpoints(ctx)
	require.NoError(t, err)
	require.Len(t, endpoints, 3)

	assert.Equal(t, "GET", endpoints[0].Method)
	assert.Equal(t, "/users", endpoints[0].Path)
	assert.Equal(t, "getUsers", endpoints[0].OperationID)
	assert.Equal(t, "Get all users", endpoints[0].Summary)

	assert.Equal(t, "POST", endpoints[1].Method)
	assert.Equal(t, "/users", endpoints[1].Path)
}

// TestListEndpointsWithoutSchema tests that listing endpoints fails without schema registration
func TestListEndpointsWithoutSchema(t *testing.T) {
	ctx := context.Background()
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	_, err = validator.ListEndpoints(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema not registered")
}

// TestListEndpointsGRPCError tests listing endpoints with gRPC error
func TestListEndpointsGRPCError(t *testing.T) {
	ctx := context.Background()

	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.schemaID = "test-schema"
	validator.client = &mockClient{
		listEndpointsFunc: func(ctx context.Context, in *pb.ListEndpointsRequest, opts ...grpc.CallOption) (*pb.ListEndpointsResponse, error) {
			return nil, errors.New("gRPC error")
		},
	}

	_, err = validator.ListEndpoints(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list endpoints")
}

// TestNormalizeGenerateOptions tests option normalization
func TestNormalizeGenerateOptions(t *testing.T) {
	// Test with nil options
	opts := normalizeGenerateOptions(nil)
	assert.Equal(t, 0, opts.StatusCode)
	assert.True(t, opts.UseExamples)
	assert.Equal(t, "application/json", opts.ContentType)

	// Test with partial options
	opts = normalizeGenerateOptions(&GenerateOptions{
		StatusCode:  201,
		UseExamples: false,
	})
	assert.Equal(t, 201, opts.StatusCode)
	assert.False(t, opts.UseExamples)
	assert.Equal(t, "application/json", opts.ContentType)

	// Test with full options
	opts = normalizeGenerateOptions(&GenerateOptions{
		StatusCode:  200,
		UseExamples: true,
		ContentType: "text/plain",
	})
	assert.Equal(t, 200, opts.StatusCode)
	assert.True(t, opts.UseExamples)
	assert.Equal(t, "text/plain", opts.ContentType)
}

// TestRegisterSchemaWithVersion tests schema registration with explicit version
func TestRegisterSchemaWithVersion(t *testing.T) {
	ctx := context.Background()
	schemaPath := filepath.Join("..", "..", "shared", "openapi.json")

	tests := []struct {
		name       string
		schemaID   string
		schemaPath string
		version    string
		mockResp   *pb.RegisterSchemaResponse
		mockErr    error
		wantErr    bool
	}{
		{
			name:       "register with version success",
			schemaID:   "test-schema",
			schemaPath: schemaPath,
			version:    "1.0.0",
			mockResp:   &pb.RegisterSchemaResponse{Success: true},
			wantErr:    false,
		},
		{
			name:       "register with version failure",
			schemaID:   "fail-schema",
			schemaPath: schemaPath,
			version:    "2.0.0",
			mockResp:   &pb.RegisterSchemaResponse{Success: false, Message: "version conflict"},
			wantErr:    true,
		},
		{
			name:       "register with version gRPC error",
			schemaID:   "error-schema",
			schemaPath: schemaPath,
			version:    "1.0.0",
			mockErr:    errors.New("gRPC error"),
			wantErr:    true,
		},
		{
			name:       "register with version file not found",
			schemaID:   "test",
			schemaPath: "/nonexistent/file.json",
			version:    "1.0.0",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			validator.client = &mockClient{
				registerSchemaFunc: func(ctx context.Context, in *pb.RegisterSchemaRequest, opts ...grpc.CallOption) (*pb.RegisterSchemaResponse, error) {
					assert.Equal(t, tt.schemaID, in.SchemaId)
					if tt.mockResp != nil || tt.mockErr != nil {
						assert.Equal(t, tt.version, in.SchemaVersion)
					}
					return tt.mockResp, tt.mockErr
				},
			}

			err = validator.RegisterSchemaWithVersion(ctx, tt.schemaID, tt.schemaPath, tt.version)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCompareSchemas tests schema comparison
func TestCompareSchemas(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		schemaID   string
		oldVersion string
		newVersion string
		mockResp   *pb.CompareSchemasResponse
		mockErr    error
		wantErr    bool
	}{
		{
			name:       "compatible schemas",
			schemaID:   "test-api",
			oldVersion: "1.0.0",
			newVersion: "1.1.0",
			mockResp: &pb.CompareSchemasResponse{
				Compatible: true,
			},
			wantErr: false,
		},
		{
			name:       "incompatible schemas with breaking changes",
			schemaID:   "test-api",
			oldVersion: "1.0.0",
			newVersion: "2.0.0",
			mockResp: &pb.CompareSchemasResponse{
				Compatible: false,
				BreakingChanges: []*pb.BreakingChange{
					{
						Type:        pb.BreakingChangeType_ENDPOINT_REMOVED,
						Path:        "/users/{id}",
						Method:      "DELETE",
						Description: "Endpoint was removed",
					},
				},
			},
			wantErr: false,
		},
		{
			name:       "comparison gRPC error",
			schemaID:   "test-api",
			oldVersion: "1.0.0",
			newVersion: "2.0.0",
			mockErr:    errors.New("gRPC error"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			validator.client = &mockClient{
				compareSchemasFunc: func(ctx context.Context, in *pb.CompareSchemasRequest, opts ...grpc.CallOption) (*pb.CompareSchemasResponse, error) {
					assert.Equal(t, tt.schemaID, in.SchemaId)
					assert.Equal(t, tt.oldVersion, in.OldVersion)
					assert.Equal(t, tt.newVersion, in.NewVersion)
					return tt.mockResp, tt.mockErr
				},
			}

			result, err := validator.CompareSchemas(ctx, tt.schemaID, tt.oldVersion, tt.newVersion)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.mockResp.Compatible, result.Compatible)
			assert.Equal(t, len(tt.mockResp.BreakingChanges), len(result.BreakingChanges))
		})
	}
}

// TestRegisterConsumer tests consumer registration
func TestRegisterConsumer(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		opts     RegisterConsumerOptions
		mockResp *pb.RegisterConsumerResponse
		mockErr  error
		wantErr  bool
	}{
		{
			name: "register consumer success",
			opts: RegisterConsumerOptions{
				ConsumerID:      "order-service",
				ConsumerVersion: "1.0.0",
				SchemaID:        "user-api",
				SchemaVersion:   "1.0.0",
				Environment:     "prod",
				UsedEndpoints: []EndpointUsage{
					{Method: "GET", Path: "/users/{id}", UsedFields: []string{"email", "name"}},
				},
			},
			mockResp: &pb.RegisterConsumerResponse{
				Success: true,
				Consumer: &pb.ConsumerInfo{
					ConsumerId:      "order-service",
					ConsumerVersion: "1.0.0",
					SchemaId:        "user-api",
					SchemaVersion:   "1.0.0",
					Environment:     "prod",
				},
			},
			wantErr: false,
		},
		{
			name: "register consumer failure",
			opts: RegisterConsumerOptions{
				ConsumerID: "fail-service",
			},
			mockResp: &pb.RegisterConsumerResponse{Success: false, Message: "registration failed"},
			wantErr:  true,
		},
		{
			name: "register consumer success but nil consumer",
			opts: RegisterConsumerOptions{
				ConsumerID: "nil-consumer",
			},
			mockResp: &pb.RegisterConsumerResponse{Success: true, Consumer: nil},
			wantErr:  true,
		},
		{
			name: "register consumer gRPC error",
			opts: RegisterConsumerOptions{
				ConsumerID: "error-service",
			},
			mockErr: errors.New("gRPC error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			validator.client = &mockClient{
				registerSchemaFunc: func(ctx context.Context, in *pb.RegisterSchemaRequest, opts ...grpc.CallOption) (*pb.RegisterSchemaResponse, error) {
					return &pb.RegisterSchemaResponse{Success: true}, nil
				},
			}
			// Override RegisterConsumer in mockClient
			mockCl := validator.client.(*mockClient)
			origRegConsumer := mockCl.RegisterConsumer
			_ = origRegConsumer // avoid unused warning
			validator.client = &mockClientWithConsumer{
				mockClient: mockCl,
				registerConsumerFunc: func(ctx context.Context, in *pb.RegisterConsumerRequest, opts ...grpc.CallOption) (*pb.RegisterConsumerResponse, error) {
					assert.Equal(t, tt.opts.ConsumerID, in.ConsumerId)
					return tt.mockResp, tt.mockErr
				},
			}

			result, err := validator.RegisterConsumer(ctx, tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.opts.ConsumerID, result.ConsumerID)
		})
	}
}

// mockClientWithConsumer extends mockClient with custom RegisterConsumer
type mockClientWithConsumer struct {
	*mockClient
	registerConsumerFunc   func(ctx context.Context, in *pb.RegisterConsumerRequest, opts ...grpc.CallOption) (*pb.RegisterConsumerResponse, error)
	listConsumersFunc      func(ctx context.Context, in *pb.ListConsumersRequest, opts ...grpc.CallOption) (*pb.ListConsumersResponse, error)
	deregisterConsumerFunc func(ctx context.Context, in *pb.DeregisterConsumerRequest, opts ...grpc.CallOption) (*pb.DeregisterConsumerResponse, error)
	canIDeployFunc         func(ctx context.Context, in *pb.CanIDeployRequest, opts ...grpc.CallOption) (*pb.CanIDeployResponse, error)
}

func (m *mockClientWithConsumer) RegisterConsumer(ctx context.Context, in *pb.RegisterConsumerRequest, opts ...grpc.CallOption) (*pb.RegisterConsumerResponse, error) {
	if m.registerConsumerFunc != nil {
		return m.registerConsumerFunc(ctx, in, opts...)
	}
	return m.mockClient.RegisterConsumer(ctx, in, opts...)
}

func (m *mockClientWithConsumer) ListConsumers(ctx context.Context, in *pb.ListConsumersRequest, opts ...grpc.CallOption) (*pb.ListConsumersResponse, error) {
	if m.listConsumersFunc != nil {
		return m.listConsumersFunc(ctx, in, opts...)
	}
	return m.mockClient.ListConsumers(ctx, in, opts...)
}

func (m *mockClientWithConsumer) DeregisterConsumer(ctx context.Context, in *pb.DeregisterConsumerRequest, opts ...grpc.CallOption) (*pb.DeregisterConsumerResponse, error) {
	if m.deregisterConsumerFunc != nil {
		return m.deregisterConsumerFunc(ctx, in, opts...)
	}
	return m.mockClient.DeregisterConsumer(ctx, in, opts...)
}

func (m *mockClientWithConsumer) CanIDeploy(ctx context.Context, in *pb.CanIDeployRequest, opts ...grpc.CallOption) (*pb.CanIDeployResponse, error) {
	if m.canIDeployFunc != nil {
		return m.canIDeployFunc(ctx, in, opts...)
	}
	return m.mockClient.CanIDeploy(ctx, in, opts...)
}

// TestListConsumers tests listing consumers
func TestListConsumers(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		schemaID    string
		environment string
		mockResp    *pb.ListConsumersResponse
		mockErr     error
		wantErr     bool
		wantCount   int
	}{
		{
			name:        "list consumers success",
			schemaID:    "user-api",
			environment: "prod",
			mockResp: &pb.ListConsumersResponse{
				Consumers: []*pb.ConsumerInfo{
					{ConsumerId: "order-service", ConsumerVersion: "1.0.0"},
					{ConsumerId: "billing-service", ConsumerVersion: "2.0.0"},
				},
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:        "list consumers empty",
			schemaID:    "unused-api",
			environment: "",
			mockResp:    &pb.ListConsumersResponse{Consumers: nil},
			wantErr:     false,
			wantCount:   0,
		},
		{
			name:        "list consumers gRPC error",
			schemaID:    "error-api",
			environment: "prod",
			mockErr:     errors.New("gRPC error"),
			wantErr:     true,
		},
		{
			name:        "list consumers with nil entries",
			schemaID:    "user-api",
			environment: "prod",
			mockResp: &pb.ListConsumersResponse{
				Consumers: []*pb.ConsumerInfo{
					{ConsumerId: "order-service", ConsumerVersion: "1.0.0"},
					nil, // Should be skipped gracefully
					{ConsumerId: "billing-service", ConsumerVersion: "2.0.0"},
				},
			},
			wantErr:   false,
			wantCount: 2, // nil entry should be filtered out
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			validator.client = &mockClientWithConsumer{
				mockClient: &mockClient{},
				listConsumersFunc: func(ctx context.Context, in *pb.ListConsumersRequest, opts ...grpc.CallOption) (*pb.ListConsumersResponse, error) {
					assert.Equal(t, tt.schemaID, in.SchemaId)
					assert.Equal(t, tt.environment, in.Environment)
					return tt.mockResp, tt.mockErr
				},
			}

			result, err := validator.ListConsumers(ctx, tt.schemaID, tt.environment)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.wantCount)
		})
	}
}

// TestDeregisterConsumer tests consumer deregistration
func TestDeregisterConsumer(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		consumerID  string
		schemaID    string
		environment string
		mockResp    *pb.DeregisterConsumerResponse
		mockErr     error
		wantErr     bool
	}{
		{
			name:        "deregister success",
			consumerID:  "order-service",
			schemaID:    "user-api",
			environment: "prod",
			mockResp:    &pb.DeregisterConsumerResponse{Success: true},
			wantErr:     false,
		},
		{
			name:        "deregister failure",
			consumerID:  "unknown-service",
			schemaID:    "user-api",
			environment: "prod",
			mockResp:    &pb.DeregisterConsumerResponse{Success: false, Message: "consumer not found"},
			wantErr:     true,
		},
		{
			name:        "deregister gRPC error",
			consumerID:  "error-service",
			schemaID:    "user-api",
			environment: "prod",
			mockErr:     errors.New("gRPC error"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			validator.client = &mockClientWithConsumer{
				mockClient: &mockClient{},
				deregisterConsumerFunc: func(ctx context.Context, in *pb.DeregisterConsumerRequest, opts ...grpc.CallOption) (*pb.DeregisterConsumerResponse, error) {
					assert.Equal(t, tt.consumerID, in.ConsumerId)
					assert.Equal(t, tt.schemaID, in.SchemaId)
					assert.Equal(t, tt.environment, in.Environment)
					return tt.mockResp, tt.mockErr
				},
			}

			err = validator.DeregisterConsumer(ctx, tt.consumerID, tt.schemaID, tt.environment)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCanIDeploy tests deployment safety check
func TestCanIDeploy(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		schemaID    string
		newVersion  string
		environment string
		mockResp    *pb.CanIDeployResponse
		mockErr     error
		wantErr     bool
		wantSafe    bool
	}{
		{
			name:        "safe to deploy",
			schemaID:    "user-api",
			newVersion:  "1.1.0",
			environment: "prod",
			mockResp: &pb.CanIDeployResponse{
				SafeToDeploy: true,
				Summary:      "No breaking changes",
			},
			wantErr:  false,
			wantSafe: true,
		},
		{
			name:        "unsafe to deploy with affected consumers",
			schemaID:    "user-api",
			newVersion:  "2.0.0",
			environment: "prod",
			mockResp: &pb.CanIDeployResponse{
				SafeToDeploy: false,
				Summary:      "Breaking changes affect 1 consumer",
				BreakingChanges: []*pb.BreakingChange{
					{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users", Method: "DELETE"},
				},
				AffectedConsumers: []*pb.ConsumerImpact{
					{
						ConsumerId:      "order-service",
						ConsumerVersion: "1.0.0",
						WillBreak:       true,
						RelevantChanges: []*pb.BreakingChange{
							{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users", Method: "DELETE"},
						},
					},
				},
			},
			wantErr:  false,
			wantSafe: false,
		},
		{
			name:        "can I deploy gRPC error",
			schemaID:    "error-api",
			newVersion:  "1.0.0",
			environment: "prod",
			mockErr:     errors.New("gRPC error"),
			wantErr:     true,
		},
		{
			name:        "handles nil elements in repeated fields",
			schemaID:    "user-api",
			newVersion:  "2.0.0",
			environment: "prod",
			mockResp: &pb.CanIDeployResponse{
				SafeToDeploy: false,
				Summary:      "Breaking changes with nil elements",
				BreakingChanges: []*pb.BreakingChange{
					{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users", Method: "DELETE"},
					nil, // Test nil handling
					{Type: pb.BreakingChangeType_TYPE_CHANGED, Path: "/users", Method: "GET"},
				},
				AffectedConsumers: []*pb.ConsumerImpact{
					{
						ConsumerId:      "order-service",
						ConsumerVersion: "1.0.0",
						WillBreak:       true,
						RelevantChanges: []*pb.BreakingChange{
							{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users", Method: "DELETE"},
							nil, // Test nil handling
						},
					},
					nil, // Test nil handling
					{
						ConsumerId:      "billing-service",
						ConsumerVersion: "2.0.0",
						WillBreak:       false,
						RelevantChanges: []*pb.BreakingChange{},
					},
				},
			},
			wantErr:  false,
			wantSafe: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator("")
			require.NoError(t, err)
			defer func() { _ = validator.Close() }()

			validator.client = &mockClientWithConsumer{
				mockClient: &mockClient{},
				canIDeployFunc: func(ctx context.Context, in *pb.CanIDeployRequest, opts ...grpc.CallOption) (*pb.CanIDeployResponse, error) {
					assert.Equal(t, tt.schemaID, in.SchemaId)
					assert.Equal(t, tt.newVersion, in.NewVersion)
					assert.Equal(t, tt.environment, in.Environment)
					return tt.mockResp, tt.mockErr
				},
			}

			result, err := validator.CanIDeploy(ctx, tt.schemaID, tt.newVersion, tt.environment)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSafe, result.SafeToDeploy)
			if !tt.wantSafe && tt.mockResp.AffectedConsumers != nil {
				assert.NotEmpty(t, result.AffectedConsumers)
			}
		})
	}
}

// TestMapConsumerInfo tests the consumer info mapper
func TestMapConsumerInfo(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result := mapConsumerInfo(nil)
		assert.Nil(t, result)
	})

	t.Run("maps all fields correctly", func(t *testing.T) {
		pbInfo := &pb.ConsumerInfo{
			ConsumerId:      "order-service",
			ConsumerVersion: "1.0.0",
			SchemaId:        "user-api",
			SchemaVersion:   "2.0.0",
			Environment:     "prod",
			RegisteredAt:    1234567890,
			LastValidatedAt: 1234567899,
			UsedEndpoints: []*pb.EndpointUsage{
				{Method: "GET", Path: "/users/{id}", UsedFields: []string{"email", "name"}},
				{Method: "POST", Path: "/users", UsedFields: []string{"email"}},
			},
		}

		result := mapConsumerInfo(pbInfo)
		require.NotNil(t, result)
		assert.Equal(t, "order-service", result.ConsumerID)
		assert.Equal(t, "1.0.0", result.ConsumerVersion)
		assert.Equal(t, "user-api", result.SchemaID)
		assert.Equal(t, "2.0.0", result.SchemaVersion)
		assert.Equal(t, "prod", result.Environment)
		assert.Equal(t, int64(1234567890), result.RegisteredAt)
		assert.Equal(t, int64(1234567899), result.LastValidatedAt)
		require.Len(t, result.UsedEndpoints, 2)
		assert.Equal(t, "GET", result.UsedEndpoints[0].Method)
		assert.Equal(t, "/users/{id}", result.UsedEndpoints[0].Path)
		assert.Equal(t, []string{"email", "name"}, result.UsedEndpoints[0].UsedFields)
	})
}

// TestNewValidatorWithOptions tests validator creation with options
func TestNewValidatorWithOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    ValidatorOptions
		wantErr bool
	}{
		{
			name:    "empty options uses defaults",
			opts:    ValidatorOptions{},
			wantErr: false,
		},
		{
			name: "with custom address",
			opts: ValidatorOptions{
				Address: "localhost:9550",
			},
			wantErr: false,
		},
		{
			name: "with API key",
			opts: ValidatorOptions{
				APIKey: "test-api-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidatorWithOptions(tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, validator)
			defer func() { _ = validator.Close() }()
		})
	}
}

// TestContextWithAPIKey tests API key context creation
func TestContextWithAPIKey(t *testing.T) {
	t.Run("without API key returns same context", func(t *testing.T) {
		validator, err := NewValidator("")
		require.NoError(t, err)
		defer func() { _ = validator.Close() }()

		ctx := context.Background()
		newCtx := validator.contextWithAPIKey(ctx)
		// Context should be the same since no API key is set
		assert.Equal(t, ctx, newCtx)
	})

	t.Run("with API key adds metadata", func(t *testing.T) {
		validator, err := NewValidatorWithOptions(ValidatorOptions{
			APIKey: "test-key",
		})
		require.NoError(t, err)
		defer func() { _ = validator.Close() }()

		ctx := context.Background()
		newCtx := validator.contextWithAPIKey(ctx)
		// Context should be different since API key is set
		assert.NotEqual(t, ctx, newCtx)
	})
}

func buildSchemaMetadata() *pb.SchemaMetadata {
	return &pb.SchemaMetadata{
		SchemaId:       "petstore",
		SchemaVersion:  "1.2.0",
		SchemaHash:     "abc123",
		RegisteredAt:   1714000000,
		UpdatedAt:      1714000500,
		OpenapiVersion: "3.0.0",
		EndpointCount:  7,
		Ownership: &pb.SchemaOwnership{
			Owner:        "jane",
			Team:         "platform",
			ContactEmail: "platform@example.com",
			ReadOnly:     false,
		},
	}
}

func TestUseSchema_ResolvesLatest(t *testing.T) {
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	var capturedReq *pb.GetSchemaRequest
	validator.client = &mockClient{
		getSchemasFunc: func(_ context.Context, in *pb.GetSchemaRequest, _ ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
			capturedReq = in
			return &pb.GetSchemaResponse{Found: true, Metadata: buildSchemaMetadata()}, nil
		},
	}

	info, err := validator.UseSchema(context.Background(), "petstore")
	require.NoError(t, err)
	assert.Equal(t, "petstore", capturedReq.SchemaId)
	assert.Equal(t, "", capturedReq.SchemaVersion)
	assert.Equal(t, "petstore", info.SchemaID)
	assert.Equal(t, "1.2.0", info.SchemaVersion)
	assert.Equal(t, int32(7), info.EndpointCount)
	require.NotNil(t, info.Ownership)
	assert.Equal(t, "jane", info.Ownership.Owner)
	assert.Equal(t, "platform", info.Ownership.Team)
}

func TestUseSchema_PinsExplicitVersion(t *testing.T) {
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	var capturedReq *pb.GetSchemaRequest
	validator.client = &mockClient{
		getSchemasFunc: func(_ context.Context, in *pb.GetSchemaRequest, _ ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
			capturedReq = in
			md := buildSchemaMetadata()
			md.SchemaVersion = "1.0.0"
			return &pb.GetSchemaResponse{Found: true, Metadata: md}, nil
		},
	}

	info, err := validator.UseSchema(context.Background(), "petstore", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", capturedReq.SchemaVersion)
	assert.Equal(t, "1.0.0", info.SchemaVersion)
}

func TestUseSchema_ReturnsErrorWhenNotFound(t *testing.T) {
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.client = &mockClient{
		getSchemasFunc: func(_ context.Context, _ *pb.GetSchemaRequest, _ ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
			return &pb.GetSchemaResponse{Found: false}, nil
		},
	}

	_, err = validator.UseSchema(context.Background(), "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'nope'")
	assert.Contains(t, err.Error(), "not registered")
}

func TestUseSchema_ErrorIncludesVersionWhenPinning(t *testing.T) {
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.client = &mockClient{
		getSchemasFunc: func(_ context.Context, _ *pb.GetSchemaRequest, _ ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
			return &pb.GetSchemaResponse{Found: false}, nil
		},
	}

	_, err = validator.UseSchema(context.Background(), "petstore", "9.9.9")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'petstore@9.9.9'")
}

func TestUseSchema_PropagatesRPCError(t *testing.T) {
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	validator.client = &mockClient{
		getSchemasFunc: func(_ context.Context, _ *pb.GetSchemaRequest, _ ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
			return nil, errors.New("connection refused")
		},
	}

	_, err = validator.UseSchema(context.Background(), "petstore")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestValidate_SendsSchemaVersionAfterUseSchema(t *testing.T) {
	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	var capturedInteraction *pb.InteractionRequest
	validator.client = &mockClient{
		getSchemasFunc: func(_ context.Context, _ *pb.GetSchemaRequest, _ ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
			return &pb.GetSchemaResponse{Found: true, Metadata: buildSchemaMetadata()}, nil
		},
		validateInteractionFunc: func(_ context.Context, in *pb.InteractionRequest, _ ...grpc.CallOption) (*pb.ValidationResult, error) {
			capturedInteraction = in
			return &pb.ValidationResult{Valid: true}, nil
		},
	}

	_, err = validator.UseSchema(context.Background(), "petstore")
	require.NoError(t, err)
	_, err = validator.Validate(context.Background(),
		ValidationRequest{Method: "GET", Path: "/pet/1"},
		ValidationResponse{StatusCode: 200},
	)
	require.NoError(t, err)
	require.NotNil(t, capturedInteraction)
	assert.Equal(t, "petstore", capturedInteraction.SchemaId)
	assert.Equal(t, "1.2.0", capturedInteraction.SchemaVersion)
}

func TestValidate_DoesNotSendVersionAfterRegisterSchema(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "openapi.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte(`{"openapi":"3.0.0"}`), 0o600))

	validator, err := NewValidator("")
	require.NoError(t, err)
	defer func() { _ = validator.Close() }()

	var capturedInteraction *pb.InteractionRequest
	validator.client = &mockClient{
		registerSchemaFunc: func(_ context.Context, _ *pb.RegisterSchemaRequest, _ ...grpc.CallOption) (*pb.RegisterSchemaResponse, error) {
			return &pb.RegisterSchemaResponse{Success: true}, nil
		},
		validateInteractionFunc: func(_ context.Context, in *pb.InteractionRequest, _ ...grpc.CallOption) (*pb.ValidationResult, error) {
			capturedInteraction = in
			return &pb.ValidationResult{Valid: true}, nil
		},
	}

	require.NoError(t, validator.RegisterSchema(context.Background(), "test-schema", schemaPath))
	_, err = validator.Validate(context.Background(),
		ValidationRequest{Method: "GET", Path: "/pet/1"},
		ValidationResponse{StatusCode: 200},
	)
	require.NoError(t, err)
	require.NotNil(t, capturedInteraction)
	assert.Equal(t, "test-schema", capturedInteraction.SchemaId)
	assert.Equal(t, "", capturedInteraction.SchemaVersion)
}
