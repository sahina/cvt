// Package main demonstrates basic usage of the CVT Go SDK.
//
// This example shows how to register a schema and validate API interactions.
//
// Schema Registration Options:
//  1. Local file: validator.RegisterSchema(ctx, "id", "./openapi.yaml")
//  2. URL: validator.RegisterSchema(ctx, "id", "https://api.example.com/openapi.json")
//
// The SDK automatically detects the source type and handles accordingly.
//
// Swagger 2.0 Support:
// The SDK fully supports both OpenAPI 3.x and Swagger 2.0 specifications.
// Swagger 2.0 basePath is handled automatically - if your schema has
// basePath="/api/v2" with paths like "/users", you can validate requests
// to "/api/v2/users" without any special configuration.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sahina/cvt/sdks/go/cvt"
	"github.com/sahina/cvt/sdks/go/examples"
)

func main() {
	fmt.Println("🚀 ContractValidator Basic Usage Example")

	// Create context
	ctx := context.Background()

	// Initialize the validator
	validator, err := cvt.NewValidator("")
	if err != nil {
		log.Fatalf("Failed to create validator: %v", err)
	}
	defer func() { _ = validator.Close() }()

	// Register the OpenAPI schema from local file
	schemaPath := examples.GetOpenAPISchemaPath()
	fmt.Printf("📋 Registering schema from local file: %s\n", schemaPath)

	err = validator.RegisterSchema(ctx, "sample-schema", schemaPath)
	if err != nil {
		log.Fatalf("Failed to register schema: %v", err)
	}
	fmt.Println("✅ Schema registered successfully")

	fmt.Println("📝 Note: You can also register schemas from URLs:")
	fmt.Println("   validator.RegisterSchema(ctx, \"id\", \"https://api.example.com/openapi.json\")")

	// Example 1: Valid pet creation
	fmt.Println("🔍 Example 1: Validating successful pet creation")
	petRequest := cvt.ValidationRequest{
		Method: "POST",
		Path:   "/pet",
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: map[string]interface{}{
			"name":      "Fluffy",
			"photoUrls": []string{"http://example.com/photo1.jpg"},
			"status":    "available",
		},
	}

	petResponse := cvt.ValidationResponse{
		StatusCode: 405, // Petstore API response for successful POST
	}

	validResult, err := validator.Validate(ctx, petRequest, petResponse)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	examples.LogValidationResult("Pet Creation", validResult)

	// Example 2: Valid user creation
	fmt.Println("🔍 Example 2: Validating user creation")
	userRequest := cvt.ValidationRequest{
		Method: "POST",
		Path:   "/user",
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: map[string]interface{}{
			"username":  "alice",
			"firstName": "Alice",
			"lastName":  "Smith",
			"email":     "alice@example.com",
			"password":  "password123",
			"phone":     "123-456-7890",
		},
	}

	userResponse := cvt.ValidationResponse{
		StatusCode: 200, // Default response for user creation
	}

	userResult, err := validator.Validate(ctx, userRequest, userResponse)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	examples.LogValidationResult("User Creation", userResult)

	// Example 3: Invalid pet creation (missing required fields)
	fmt.Println("🔍 Example 3: Validating invalid pet creation (missing required fields)")
	invalidRequest := cvt.ValidationRequest{
		Method: "POST",
		Path:   "/pet",
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: map[string]interface{}{
			"status": "available",
			// Missing required 'name' and 'photoUrls' fields
		},
	}

	invalidResponse := cvt.ValidationResponse{
		StatusCode: 405,
	}

	invalidResult, err := validator.Validate(ctx, invalidRequest, invalidResponse)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	examples.LogValidationResult("Invalid Pet Creation", invalidResult)

	// Example 4: GET pet by ID
	fmt.Println("🔍 Example 4: Validating GET pet by ID")
	getRequest := cvt.ValidationRequest{
		Method: "GET",
		Path:   "/pet/123",
		Headers: map[string]string{
			"api_key": "special-key",
		},
	}

	getResponse := cvt.ValidationResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: map[string]interface{}{
			"id":        123,
			"name":      "Fluffy",
			"photoUrls": []string{"http://example.com/photo1.jpg"},
			"status":    "available",
		},
	}

	getResult, err := validator.Validate(ctx, getRequest, getResponse)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	examples.LogValidationResult("GET Pet by ID", getResult)

	fmt.Println("🎉 All examples completed!")
}
