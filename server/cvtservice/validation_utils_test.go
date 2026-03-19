package cvtservice

import (
	"strings"
	"testing"
)

// TestValidateSchemaID tests schema ID validation
func TestValidateSchemaID(t *testing.T) {
	tests := []struct {
		name      string
		schemaID  string
		wantError bool
	}{
		{
			name:      "valid schema ID",
			schemaID:  "my-api-v1",
			wantError: false,
		},
		{
			name:      "empty schema ID",
			schemaID:  "",
			wantError: true,
		},
		{
			name:      "whitespace only schema ID",
			schemaID:  "   ",
			wantError: true,
		},
		{
			name:      "schema ID at max length (255 chars)",
			schemaID:  strings.Repeat("a", 255),
			wantError: false,
		},
		{
			name:      "schema ID exceeds max length (256 chars)",
			schemaID:  strings.Repeat("a", 256),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSchemaID(tt.schemaID)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateSchemaID() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidateSchemaContent tests schema content validation
func TestValidateSchemaContent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name:      "valid schema content",
			content:   `{"openapi": "3.0.0"}`,
			wantError: false,
		},
		{
			name:      "empty schema content",
			content:   "",
			wantError: true,
		},
		{
			name:      "whitespace only schema content",
			content:   "   ",
			wantError: true,
		},
		{
			name:      "schema content at max size (10MB)",
			content:   strings.Repeat("a", MaxSchemaContentBytes),
			wantError: false,
		},
		{
			name:      "schema content exceeds max size",
			content:   strings.Repeat("a", MaxSchemaContentBytes+1),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSchemaContent(tt.content)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateSchemaContent() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidateHTTPMethod tests HTTP method validation
func TestValidateHTTPMethod(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		wantError bool
	}{
		{
			name:      "valid GET method",
			method:    "GET",
			wantError: false,
		},
		{
			name:      "valid POST method",
			method:    "POST",
			wantError: false,
		},
		{
			name:      "valid lowercase get method",
			method:    "get",
			wantError: false,
		},
		{
			name:      "valid mixed case method",
			method:    "PaTcH",
			wantError: false,
		},
		{
			name:      "invalid method",
			method:    "INVALID",
			wantError: true,
		},
		{
			name:      "empty method",
			method:    "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHTTPMethod(tt.method)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateHTTPMethod() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidateHTTPPath tests HTTP path validation
func TestValidateHTTPPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{
			name:      "valid root path",
			path:      "/",
			wantError: false,
		},
		{
			name:      "valid path with segments",
			path:      "/users/123",
			wantError: false,
		},
		{
			name:      "path without leading slash",
			path:      "users/123",
			wantError: true,
		},
		{
			name:      "empty path",
			path:      "",
			wantError: true,
		},
		{
			name:      "whitespace only path",
			path:      "   ",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHTTPPath(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateHTTPPath() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidateStatusCode tests HTTP status code validation
func TestValidateStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int32
		wantError  bool
	}{
		{
			name:       "valid 200 OK",
			statusCode: 200,
			wantError:  false,
		},
		{
			name:       "valid 404 Not Found",
			statusCode: 404,
			wantError:  false,
		},
		{
			name:       "valid min status code (100)",
			statusCode: MinStatusCode,
			wantError:  false,
		},
		{
			name:       "valid max status code (599)",
			statusCode: MaxStatusCode,
			wantError:  false,
		},
		{
			name:       "invalid status code below range (99)",
			statusCode: 99,
			wantError:  true,
		},
		{
			name:       "invalid status code above range (600)",
			statusCode: 600,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCode(tt.statusCode)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateStatusCode() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateRequestBody(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError bool
	}{
		{
			name:      "valid JSON body",
			body:      `{"key": "value"}`,
			wantError: false,
		},
		{
			name:      "empty body (allowed)",
			body:      "",
			wantError: false,
		},
		{
			name:      "body exceeds max size",
			body:      strings.Repeat("x", MaxRequestBodyBytes+1),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequestBody(tt.body)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateRequestBody() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateResponseBody(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError bool
	}{
		{
			name:      "valid JSON body",
			body:      `{"key": "value"}`,
			wantError: false,
		},
		{
			name:      "empty body (allowed)",
			body:      "",
			wantError: false,
		},
		{
			name:      "body exceeds max size",
			body:      strings.Repeat("x", MaxResponseBodyBytes+1),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResponseBody(tt.body)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateResponseBody() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
