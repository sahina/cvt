// Package cvtservice provides validation utilities for the Contract Validation Tool.
// This file contains validation functions for schema IDs, HTTP methods, paths,
// status codes, and other request/response components.
package cvtservice

import (
	"fmt"
	"strings"
)

const (
	// MaxSchemaIDLength is the maximum allowed length for a schema ID (255 characters)
	MaxSchemaIDLength = 255

	// MaxSchemaContentBytes is the maximum allowed size for schema content (10MB)
	MaxSchemaContentBytes = 10 * 1024 * 1024 // 10MB

	// MinStatusCode is the minimum valid HTTP status code (100)
	MinStatusCode = 100

	// MaxStatusCode is the maximum valid HTTP status code (599)
	MaxStatusCode = 599

	// MaxRequestBodyBytes is the maximum allowed size for request body in validation (5MB)
	MaxRequestBodyBytes = 5 * 1024 * 1024

	// MaxResponseBodyBytes is the maximum allowed size for response body in validation (5MB)
	MaxResponseBodyBytes = 5 * 1024 * 1024

	// MaxConsumersPerSchema is the maximum number of consumers that can be registered per schema
	MaxConsumersPerSchema = 10000
)

// validHTTPMethods defines the set of supported HTTP methods.
// Only these methods are allowed for validation requests.
var validHTTPMethods = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"DELETE":  true,
	"PATCH":   true,
	"HEAD":    true,
	"OPTIONS": true,
}

// ValidateNotNull checks if the value is not nil.
// This is a generic validation function that can be used for any interface type.
//
// Parameters:
//   - value: The value to check for nil
//   - name: The name of the field (used in error messages)
//
// Returns:
//   - error: An error if the value is nil, nil otherwise
func ValidateNotNull(value interface{}, name string) error {
	if value == nil {
		return fmt.Errorf("%s cannot be null", name)
	}
	return nil
}

// ValidateSchemaID validates the schema ID according to the following rules:
// - Must not be null or empty (after trimming whitespace)
// - Must not exceed 255 characters
//
// Parameters:
//   - schemaID: The schema ID to validate
//
// Returns:
//   - error: An error if validation fails, nil if valid
func ValidateSchemaID(schemaID string) error {
	if strings.TrimSpace(schemaID) == "" {
		return fmt.Errorf("schema ID cannot be null or empty")
	}
	if len(schemaID) > MaxSchemaIDLength {
		return fmt.Errorf("schema ID cannot exceed %d characters (got %d)", MaxSchemaIDLength, len(schemaID))
	}
	return nil
}

// ValidateSchemaContent validates the schema content according to the following rules:
// - Must not be null or empty (after trimming whitespace)
// - Must not exceed 10MB (10,485,760 bytes)
//
// This prevents excessively large schemas from consuming too much memory.
//
// Parameters:
//   - schemaContent: The schema content to validate (JSON format)
//
// Returns:
//   - error: An error if validation fails, nil if valid
func ValidateSchemaContent(schemaContent string) error {
	if strings.TrimSpace(schemaContent) == "" {
		return fmt.Errorf("schema content cannot be null or empty")
	}
	contentSize := len([]byte(schemaContent))
	if contentSize > MaxSchemaContentBytes {
		return fmt.Errorf("schema content cannot exceed %d bytes (got %d)", MaxSchemaContentBytes, contentSize)
	}
	return nil
}

// ValidateRequestBody validates the request body size.
func ValidateRequestBody(body string) error {
	if len(body) > MaxRequestBodyBytes {
		return fmt.Errorf("request body cannot exceed %d bytes (got %d)", MaxRequestBodyBytes, len(body))
	}
	return nil
}

// ValidateResponseBody validates the response body size.
func ValidateResponseBody(body string) error {
	if len(body) > MaxResponseBodyBytes {
		return fmt.Errorf("response body cannot exceed %d bytes (got %d)", MaxResponseBodyBytes, len(body))
	}
	return nil
}

// ValidateHTTPMethod validates the HTTP method according to the following rules:
// - Must not be null or empty
// - Must be one of: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
// - Case-insensitive (converted to uppercase for comparison)
//
// Parameters:
//   - method: The HTTP method to validate
//
// Returns:
//   - error: An error if validation fails, nil if valid
func ValidateHTTPMethod(method string) error {
	if strings.TrimSpace(method) == "" {
		return fmt.Errorf("HTTP method cannot be null or empty")
	}
	upperMethod := strings.ToUpper(method)
	if !validHTTPMethods[upperMethod] {
		return fmt.Errorf("invalid HTTP method: %s (valid methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)", method)
	}
	return nil
}

// ValidateHTTPPath validates the HTTP path according to the following rules:
// - Must not be null or empty
// - Must start with '/' (absolute path required)
//
// Parameters:
//   - path: The HTTP path to validate
//
// Returns:
//   - error: An error if validation fails, nil if valid
func ValidateHTTPPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("HTTP path cannot be null or empty")
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("HTTP path must start with '/' (got: %s)", path)
	}
	return nil
}

// ValidateStatusCode validates the HTTP status code according to the following rules:
// - Must be in the range 100-599 (inclusive)
//
// This covers all standard HTTP status code ranges:
// - 1xx: Informational responses
// - 2xx: Successful responses
// - 3xx: Redirection messages
// - 4xx: Client error responses
// - 5xx: Server error responses
//
// Parameters:
//   - statusCode: The HTTP status code to validate
//
// Returns:
//   - error: An error if validation fails, nil if valid
func ValidateStatusCode(statusCode int32) error {
	if statusCode < MinStatusCode || statusCode > MaxStatusCode {
		return fmt.Errorf("status code must be between %d and %d (got: %d)", MinStatusCode, MaxStatusCode, statusCode)
	}
	return nil
}
