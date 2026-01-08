package producer

import (
	"testing"

	cvt "github.com/cvt/cvt-sdk/go/cvt"
)

func TestNewProducerTestKit_RequiresSchemaID(t *testing.T) {
	_, err := NewProducerTestKit(TestConfig{
		SchemaID: "",
	})
	if err == nil {
		t.Error("expected error when SchemaID is empty")
	}
	if err.Error() != "SchemaID is required" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestNewProducerTestKit_DefaultServerAddress(t *testing.T) {
	// This test verifies the config is accepted, even though it will fail to connect
	// without a real server running. We're testing the configuration handling.
	config := TestConfig{
		SchemaID: "test-api",
		// ServerAddress left empty - should default to localhost:50051
	}

	testKit, err := NewProducerTestKit(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = testKit.Close() }()

	if testKit.schemaID != "test-api" {
		t.Errorf("expected schemaID 'test-api', got '%s'", testKit.schemaID)
	}
}

func TestNewProducerTestKit_CustomServerAddress(t *testing.T) {
	config := TestConfig{
		SchemaID:      "test-api",
		ServerAddress: "custom-host:9999",
	}

	testKit, err := NewProducerTestKit(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = testKit.Close() }()
}

func TestNewProducerTestKit_WithSchemaVersion(t *testing.T) {
	config := TestConfig{
		SchemaID:      "test-api",
		SchemaVersion: "1.0.0",
	}

	testKit, err := NewProducerTestKit(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = testKit.Close() }()

	if testKit.schemaVersion != "1.0.0" {
		t.Errorf("expected schemaVersion '1.0.0', got '%s'", testKit.schemaVersion)
	}
}

func TestNewProducerTestKit_WithAPIKey(t *testing.T) {
	config := TestConfig{
		SchemaID: "test-api",
		APIKey:   "test-api-key",
	}

	testKit, err := NewProducerTestKit(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = testKit.Close() }()

	if testKit.apiKey != "test-api-key" {
		t.Errorf("expected apiKey 'test-api-key', got '%s'", testKit.apiKey)
	}
}

func TestNewProducerTestKit_WithTLSEnabled(t *testing.T) {
	config := TestConfig{
		SchemaID: "test-api",
		TLS: &cvt.TLSOptions{
			Enabled: true,
			// Note: This will fail to connect without valid certs,
			// but we're just testing that the TLS config is accepted
		},
	}

	// Creating with TLS enabled should work (connection may fail later without valid certs)
	testKit, err := NewProducerTestKit(config)
	if err != nil {
		t.Fatalf("unexpected error creating test kit with TLS: %v", err)
	}
	defer func() { _ = testKit.Close() }()
}

func TestNewProducerTestKit_WithTLSDisabled(t *testing.T) {
	config := TestConfig{
		SchemaID: "test-api",
		TLS: &cvt.TLSOptions{
			Enabled: false,
		},
	}

	testKit, err := NewProducerTestKit(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = testKit.Close() }()
}

func TestForEndpoint(t *testing.T) {
	config := TestConfig{
		SchemaID: "test-api",
	}

	testKit, err := NewProducerTestKit(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = testKit.Close() }()

	endpointTester := testKit.ForEndpoint("GET", "/users/{id}")

	if endpointTester.method != "GET" {
		t.Errorf("expected method 'GET', got '%s'", endpointTester.method)
	}
	if endpointTester.pathPattern != "/users/{id}" {
		t.Errorf("expected pathPattern '/users/{id}', got '%s'", endpointTester.pathPattern)
	}
	if endpointTester.testKit != testKit {
		t.Error("expected testKit reference to be set")
	}
}

func TestSerializeBody(t *testing.T) {
	tests := []struct {
		name     string
		body     any
		expected string
		wantErr  bool
	}{
		{
			name:     "nil body",
			body:     nil,
			expected: "",
			wantErr:  false,
		},
		{
			name:     "string body",
			body:     `{"key": "value"}`,
			expected: `{"key": "value"}`,
			wantErr:  false,
		},
		{
			name:     "map body",
			body:     map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
			wantErr:  false,
		},
		{
			name:     "struct body",
			body:     struct{ Name string }{"test"},
			expected: `{"Name":"test"}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := serializeBody(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("serializeBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("serializeBody() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTestResponseData(t *testing.T) {
	response := TestResponseData{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       map[string]any{"id": "123", "name": "John"},
	}

	if response.StatusCode != 200 {
		t.Errorf("expected StatusCode 200, got %d", response.StatusCode)
	}
	if response.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type header")
	}
}

func TestTestRequestContext(t *testing.T) {
	request := TestRequestContext{
		Method:  "POST",
		Path:    "/users",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    map[string]any{"name": "John", "email": "john@example.com"},
	}

	if request.Method != "POST" {
		t.Errorf("expected Method 'POST', got '%s'", request.Method)
	}
	if request.Path != "/users" {
		t.Errorf("expected Path '/users', got '%s'", request.Path)
	}
}

func TestValidateResponseParams(t *testing.T) {
	params := ValidateResponseParams{
		Method: "GET",
		Path:   "/users/123",
		Response: TestResponseData{
			StatusCode: 200,
			Body:       map[string]any{"id": "123"},
		},
		Request: &TestRequestContext{
			Method: "GET",
			Path:   "/users/123",
		},
	}

	if params.Method != "GET" {
		t.Errorf("expected Method 'GET', got '%s'", params.Method)
	}
	if params.Request == nil {
		t.Error("expected Request to be set")
	}
}

func TestTestValidationResult(t *testing.T) {
	result := TestValidationResult{
		Valid:                   true,
		Errors:                  []string{},
		ValidatedAgainstVersion: "1.0.0",
		ValidatedAgainstHash:    "abc123",
	}

	if !result.Valid {
		t.Error("expected Valid to be true")
	}
	if len(result.Errors) != 0 {
		t.Error("expected no errors")
	}
	if result.ValidatedAgainstVersion != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", result.ValidatedAgainstVersion)
	}
}
