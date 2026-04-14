// Package cvt provides an embedded library for contract validation without gRPC.
// This file contains test fixture generation utilities.
package cvt

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// GeneratedFixture represents a generated test fixture for an API endpoint.
type GeneratedFixture struct {
	Request  GeneratedRequest  `json:"request"`
	Response GeneratedResponse `json:"response"`
}

// GeneratedRequest represents a generated HTTP request.
type GeneratedRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
}

// GeneratedResponse represents a generated HTTP response.
type GeneratedResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       interface{}       `json:"body,omitempty"`
}

// GenerateOptions controls how fixtures are generated.
type GenerateOptions struct {
	// StatusCode specifies which response status code to generate (default: first successful status)
	StatusCode int
	// UseExamples prefers schema examples over generated values when available
	UseExamples bool
	// ContentType specifies the content type to use (default: application/json)
	ContentType string
}

// DefaultGenerateOptions returns the default generation options.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		StatusCode:  0, // Auto-select first successful status
		UseExamples: true,
		ContentType: "application/json",
	}
}

// GetResponseExample extracts an example response from the schema for a given endpoint.
// Returns the example body and status code if found, or an error if not available.
func (v *Validator) GetResponseExample(schemaID, method, path string, statusCode int) (interface{}, error) {
	doc, ok := v.GetSchema(schemaID)
	if !ok {
		return nil, fmt.Errorf("schema not found: %s", schemaID)
	}

	operation, err := v.findOperation(doc, method, path)
	if err != nil {
		return nil, err
	}

	response, err := v.getResponseForStatus(operation, statusCode)
	if err != nil {
		return nil, err
	}

	// Look for example in the response content
	if response.Value != nil && response.Value.Content != nil {
		for contentType, mediaType := range response.Value.Content {
			if !strings.Contains(contentType, "json") {
				continue
			}
			// Check for explicit example
			if mediaType.Example != nil {
				return mediaType.Example, nil
			}
			// Check for examples map
			if len(mediaType.Examples) > 0 {
				for _, ex := range mediaType.Examples {
					if ex.Value != nil && ex.Value.Value != nil {
						return ex.Value.Value, nil
					}
				}
			}
			// Check schema example
			if mediaType.Schema != nil && mediaType.Schema.Value != nil {
				if mediaType.Schema.Value.Example != nil {
					return mediaType.Schema.Value.Example, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no example found for %s %s with status %d", method, path, statusCode)
}

// GenerateResponse generates a response fixture based on the schema definition.
// It generates values based on schema types, preferring examples when UseExamples is true.
func (v *Validator) GenerateResponse(schemaID, method, path string, opts GenerateOptions) (*GeneratedResponse, error) {
	doc, ok := v.GetSchema(schemaID)
	if !ok {
		return nil, fmt.Errorf("schema not found: %s", schemaID)
	}

	operation, err := v.findOperation(doc, method, path)
	if err != nil {
		return nil, err
	}

	// Determine status code
	statusCode := opts.StatusCode
	if statusCode == 0 {
		statusCode = v.selectSuccessStatus(operation)
	}

	response, err := v.getResponseForStatus(operation, statusCode)
	if err != nil {
		return nil, err
	}

	result := &GeneratedResponse{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
	}

	// Generate response headers
	if response.Value != nil {
		for headerName, headerRef := range response.Value.Headers {
			if headerRef.Value != nil && headerRef.Value.Schema != nil {
				val := v.generateValue(doc, headerRef.Value.Schema, opts.UseExamples, 0)
				result.Headers[headerName] = fmt.Sprintf("%v", val)
			}
		}
	}

	// Generate response body
	if response.Value != nil && response.Value.Content != nil {
		contentType := opts.ContentType
		if contentType == "" {
			contentType = "application/json"
		}

		mediaType := response.Value.Content.Get(contentType)
		if mediaType == nil {
			// Try to find any JSON content type
			for ct, mt := range response.Value.Content {
				if strings.Contains(ct, "json") {
					mediaType = mt
					contentType = ct
					break
				}
			}
		}

		if mediaType != nil {
			result.Headers["Content-Type"] = contentType
			if mediaType.Schema != nil {
				result.Body = v.generateValue(doc, mediaType.Schema, opts.UseExamples, 0)
			}
		}
	}

	return result, nil
}

// GenerateRequestBody generates a request body fixture based on the schema definition.
func (v *Validator) GenerateRequestBody(schemaID, method, path string, opts GenerateOptions) (interface{}, error) {
	doc, ok := v.GetSchema(schemaID)
	if !ok {
		return nil, fmt.Errorf("schema not found: %s", schemaID)
	}

	operation, err := v.findOperation(doc, method, path)
	if err != nil {
		return nil, err
	}

	if operation.RequestBody == nil {
		return nil, fmt.Errorf("no request body defined for %s %s", method, path)
	}

	requestBody := operation.RequestBody.Value
	if requestBody == nil {
		return nil, fmt.Errorf("no request body schema for %s %s", method, path)
	}

	contentType := opts.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	mediaType := requestBody.Content.Get(contentType)
	if mediaType == nil {
		// Try to find any JSON content type
		for ct, mt := range requestBody.Content {
			if strings.Contains(ct, "json") {
				mediaType = mt
				break
			}
		}
	}

	if mediaType == nil {
		return nil, fmt.Errorf("no %s content type found for request body", contentType)
	}

	if mediaType.Schema == nil {
		return nil, fmt.Errorf("no schema defined for request body")
	}

	return v.generateValue(doc, mediaType.Schema, opts.UseExamples, 0), nil
}

// GenerateFixture generates a complete request/response fixture for an endpoint.
func (v *Validator) GenerateFixture(schemaID, method, path string, opts GenerateOptions) (*GeneratedFixture, error) {
	doc, ok := v.GetSchema(schemaID)
	if !ok {
		return nil, fmt.Errorf("schema not found: %s", schemaID)
	}

	operation, err := v.findOperation(doc, method, path)
	if err != nil {
		return nil, err
	}

	// Resolve path parameters to generate a concrete path
	resolvedPath := path
	for _, paramRef := range operation.Parameters {
		if paramRef.Value != nil && paramRef.Value.In == "path" {
			paramName := paramRef.Value.Name
			placeholder := "{" + paramName + "}"
			if strings.Contains(resolvedPath, placeholder) {
				var paramValue string
				if paramRef.Value.Schema != nil {
					val := v.generateValue(doc, paramRef.Value.Schema, opts.UseExamples, 0)
					paramValue = fmt.Sprintf("%v", val)
				} else {
					paramValue = "value"
				}
				resolvedPath = strings.Replace(resolvedPath, placeholder, paramValue, 1)
			}
		}
	}

	fixture := &GeneratedFixture{
		Request: GeneratedRequest{
			Method:  method,
			Path:    resolvedPath,
			Headers: make(map[string]string),
		},
	}

	// Generate request headers from parameters
	for _, paramRef := range operation.Parameters {
		if paramRef.Value != nil && paramRef.Value.In == "header" && paramRef.Value.Required {
			if paramRef.Value.Schema != nil {
				val := v.generateValue(doc, paramRef.Value.Schema, opts.UseExamples, 0)
				fixture.Request.Headers[paramRef.Value.Name] = fmt.Sprintf("%v", val)
			}
		}
	}

	// Generate request body if applicable
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		contentType := opts.ContentType
		if contentType == "" {
			contentType = "application/json"
		}

		mediaType := operation.RequestBody.Value.Content.Get(contentType)
		if mediaType == nil {
			for ct, mt := range operation.RequestBody.Value.Content {
				if strings.Contains(ct, "json") {
					mediaType = mt
					contentType = ct
					break
				}
			}
		}

		if mediaType != nil && mediaType.Schema != nil {
			fixture.Request.Headers["Content-Type"] = contentType
			fixture.Request.Body = v.generateValue(doc, mediaType.Schema, opts.UseExamples, 0)
		}
	}

	// Generate response
	response, err := v.GenerateResponse(schemaID, method, path, opts)
	if err != nil {
		return nil, err
	}
	fixture.Response = *response

	return fixture, nil
}

// GenerateFixtureJSON generates a fixture and returns it as a JSON string.
func (v *Validator) GenerateFixtureJSON(schemaID, method, path string, opts GenerateOptions) (string, error) {
	fixture, err := v.GenerateFixture(schemaID, method, path, opts)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal fixture: %w", err)
	}

	return string(data), nil
}

// ListEndpoints returns all available endpoints in a schema.
func (v *Validator) ListEndpoints(schemaID string) ([]string, error) {
	doc, ok := v.GetSchema(schemaID)
	if !ok {
		return nil, fmt.Errorf("schema not found: %s", schemaID)
	}

	var endpoints []string
	for path, pathItem := range doc.Paths.Map() {
		for method := range pathItem.Operations() {
			endpoints = append(endpoints, fmt.Sprintf("%s %s", method, path))
		}
	}
	return endpoints, nil
}

// findOperation finds the operation for a given method and path.
func (v *Validator) findOperation(doc *openapi3.T, method, path string) (*openapi3.Operation, error) {
	// First try exact match
	pathItem := doc.Paths.Find(path)
	if pathItem != nil {
		operation := pathItem.GetOperation(strings.ToUpper(method))
		if operation != nil {
			return operation, nil
		}
	}

	// Try matching with path parameters using router
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	// Create a dummy request to find the route
	req, err := http.NewRequest(strings.ToUpper(method), "http://localhost"+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	route, _, err := router.FindRoute(req)
	if err != nil {
		return nil, fmt.Errorf("route not found: %s %s", method, path)
	}

	return route.Operation, nil
}

// getResponseForStatus gets the response definition for a given status code.
func (v *Validator) getResponseForStatus(operation *openapi3.Operation, statusCode int) (*openapi3.ResponseRef, error) {
	if operation.Responses == nil {
		return nil, fmt.Errorf("no responses defined")
	}

	// Try exact status match
	statusStr := fmt.Sprintf("%d", statusCode)
	if response := operation.Responses.Status(statusCode); response != nil {
		return response, nil
	}

	// Try wildcard status (2XX, 4XX, etc.)
	wildcardStatus := string(statusStr[0]) + "XX"
	for code, response := range operation.Responses.Map() {
		if strings.EqualFold(code, wildcardStatus) {
			return response, nil
		}
	}

	// Try default response
	if def := operation.Responses.Default(); def != nil {
		return def, nil
	}

	return nil, fmt.Errorf("no response found for status %d", statusCode)
}

// selectSuccessStatus selects the first successful status code from responses.
func (v *Validator) selectSuccessStatus(operation *openapi3.Operation) int {
	if operation.Responses == nil {
		return 200
	}

	// Prefer specific success codes in order
	preferredCodes := []int{200, 201, 202, 204}
	for _, code := range preferredCodes {
		if operation.Responses.Status(code) != nil {
			return code
		}
	}

	// Check for any 2XX status
	for code := range operation.Responses.Map() {
		if strings.HasPrefix(code, "2") {
			// Parse the code
			var statusCode int
			_, _ = fmt.Sscanf(code, "%d", &statusCode)
			if statusCode > 0 {
				return statusCode
			}
		}
	}

	// Fall back to first available response code
	for code := range operation.Responses.Map() {
		if code == "default" {
			continue
		}
		var statusCode int
		_, _ = fmt.Sscanf(code, "%d", &statusCode)
		if statusCode > 0 {
			return statusCode
		}
	}

	return 200
}

// generateValue generates a value based on a schema.
func (v *Validator) generateValue(doc *openapi3.T, schemaRef *openapi3.SchemaRef, useExamples bool, depth int) interface{} {
	if schemaRef == nil {
		return nil
	}

	// Prevent infinite recursion
	if depth > 10 {
		return nil
	}

	schema := schemaRef.Value
	if schema == nil {
		return nil
	}

	// Check for example first if useExamples is true
	if useExamples && schema.Example != nil {
		return schema.Example
	}

	// Handle allOf, oneOf, anyOf
	if len(schema.AllOf) > 0 {
		return v.generateAllOf(doc, schema.AllOf, useExamples, depth)
	}
	if len(schema.OneOf) > 0 {
		return v.generateValue(doc, schema.OneOf[0], useExamples, depth+1)
	}
	if len(schema.AnyOf) > 0 {
		return v.generateValue(doc, schema.AnyOf[0], useExamples, depth+1)
	}

	// Handle by type
	types := schema.Type.Slice()
	if len(types) == 0 {
		// Type-less schema (valid in OpenAPI 3.1 with composition)
		if len(schema.Properties) > 0 {
			return v.generateObject(doc, schema, useExamples, depth)
		}
		return nil
	}

	switch types[0] {
	case "object":
		return v.generateObject(doc, schema, useExamples, depth)
	case "array":
		return v.generateArray(doc, schema, useExamples, depth)
	case "string":
		return v.generateString(schema)
	case "integer":
		return v.generateInteger(schema)
	case "number":
		return v.generateNumber(schema)
	case "boolean":
		return v.generateBoolean(schema)
	default:
		if len(schema.Properties) > 0 {
			return v.generateObject(doc, schema, useExamples, depth)
		}
		return nil
	}
}

// generateObject generates an object value.
func (v *Validator) generateObject(doc *openapi3.T, schema *openapi3.Schema, useExamples bool, depth int) map[string]interface{} {
	result := make(map[string]interface{})

	for propName, propRef := range schema.Properties {
		result[propName] = v.generateValue(doc, propRef, useExamples, depth+1)
	}

	// When there are no named properties but additionalProperties has a schema,
	// generate sample entries so the response isn't an empty object.
	if len(result) == 0 && schema.AdditionalProperties.Schema != nil {
		sampleKeys := []string{"key1", "key2", "key3"}
		for _, k := range sampleKeys {
			result[k] = v.generateValue(doc, schema.AdditionalProperties.Schema, useExamples, depth+1)
		}
	}

	return result
}

// generateArray generates an array value.
func (v *Validator) generateArray(doc *openapi3.T, schema *openapi3.Schema, useExamples bool, depth int) []interface{} {
	if schema.Items == nil {
		return []interface{}{}
	}

	// Generate 1-2 items
	count := 1
	if schema.MinItems > 0 && uint64(count) < schema.MinItems {
		count = int(schema.MinItems)
	}

	result := make([]interface{}, count)
	for i := 0; i < count; i++ {
		result[i] = v.generateValue(doc, schema.Items, useExamples, depth+1)
	}

	return result
}

// generateString generates a string value.
func (v *Validator) generateString(schema *openapi3.Schema) string {
	// Check for enum
	if len(schema.Enum) > 0 {
		return fmt.Sprintf("%v", schema.Enum[0])
	}

	// Check for format
	switch schema.Format {
	case "date":
		return "2024-01-15"
	case "date-time":
		return "2024-01-15T10:30:00Z"
	case "email":
		return "user@example.com"
	case "uri", "url":
		return "https://example.com"
	case "uuid":
		return "550e8400-e29b-41d4-a716-446655440000"
	case "hostname":
		return "example.com"
	case "ipv4":
		return "192.168.1.1"
	case "ipv6":
		return "::1"
	case "byte":
		return "c3RyaW5n" // base64 encoded "string"
	case "binary":
		return "binary-data"
	case "password":
		return "********"
	}

	// Check for pattern
	if schema.Pattern != "" {
		return "pattern-value"
	}

	// Default string
	return "string"
}

// generateInteger generates an integer value.
func (v *Validator) generateInteger(schema *openapi3.Schema) int64 {
	// Check for enum
	if len(schema.Enum) > 0 {
		if val, ok := schema.Enum[0].(float64); ok {
			return int64(val)
		}
	}

	// Check for minimum
	if schema.Min != nil {
		return int64(*schema.Min)
	}

	// Default based on format
	switch schema.Format {
	case "int64":
		return 1234567890
	default:
		return 123
	}
}

// generateNumber generates a number value.
func (v *Validator) generateNumber(schema *openapi3.Schema) float64 {
	// Check for enum
	if len(schema.Enum) > 0 {
		if val, ok := schema.Enum[0].(float64); ok {
			return val
		}
	}

	// Check for minimum
	if schema.Min != nil {
		return *schema.Min
	}

	// Default
	return 123.45
}

// generateBoolean generates a boolean value.
func (v *Validator) generateBoolean(schema *openapi3.Schema) bool {
	// Check for enum
	if len(schema.Enum) > 0 {
		if val, ok := schema.Enum[0].(bool); ok {
			return val
		}
	}

	// Random boolean for variety
	return rand.Intn(2) == 1
}

// generateAllOf merges all schemas in allOf.
func (v *Validator) generateAllOf(doc *openapi3.T, allOf openapi3.SchemaRefs, useExamples bool, depth int) map[string]interface{} {
	result := make(map[string]interface{})

	for _, schemaRef := range allOf {
		val := v.generateValue(doc, schemaRef, useExamples, depth+1)
		if obj, ok := val.(map[string]interface{}); ok {
			for k, v := range obj {
				result[k] = v
			}
		}
	}

	return result
}
