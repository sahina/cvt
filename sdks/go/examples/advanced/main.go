// Package main demonstrates advanced usage of the CVT Go SDK.
//
// This example shows advanced validation scenarios:
//   - Nested objects (Category and Tags)
//   - Array responses
//   - Using helper functions
//   - Invalid request scenarios with detailed error messages
//
// Note on Swagger 2.0:
// The SDK fully supports Swagger 2.0 specifications, including basePath handling.
// The server automatically converts Swagger 2.0 to OpenAPI 3.0 internally during
// schema registration and handles basePath prefixes transparently.
//
// To use Swagger 2.0:
//
//	validator.RegisterSchema(ctx, "id", "https://petstore.swagger.io/v2/swagger.json")
//
// BasePath Handling:
// If your Swagger 2.0 schema has basePath="/api/v2" and paths like "/users",
// you can validate requests to "/api/v2/users" - the SDK automatically strips
// the basePath prefix before validation.
//
// Key differences between Swagger 2.0 and OpenAPI 3.0:
//   - Schema Definitions: "definitions" vs "components/schemas"
//   - Security: "securityDefinitions" vs "components/securitySchemes"
//   - Request/Response: "consumes"/"produces" vs "content" with media types
//   - Base Path: "basePath" vs "servers" array with URL paths
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cvt/cvt-sdk/go/cvt"
	"github.com/cvt/cvt-sdk/go/examples"
)

func main() {
	fmt.Println("🚀 ContractValidator - Advanced Usage Example")

	// Create context
	ctx := context.Background()

	// Initialize the validator
	validator, err := cvt.NewValidator("")
	if err != nil {
		log.Fatalf("Failed to create validator: %v", err)
	}
	defer func() { _ = validator.Close() }()

	// Register the OpenAPI schema
	schemaPath := examples.GetOpenAPISchemaPath()
	fmt.Printf("📋 Registering OpenAPI schema: %s\n", schemaPath)

	err = validator.RegisterSchema(ctx, "advanced-schema", schemaPath)
	if err != nil {
		log.Fatalf("Failed to register schema: %v", err)
	}
	fmt.Println("✅ Schema registered successfully")

	fmt.Println(string(make([]byte, 70))) // Separator line
	fmt.Println("ADVANCED VALIDATION SCENARIOS")
	fmt.Println(string(make([]byte, 70)))
	fmt.Println()

	// Example 1: Create pet with nested objects
	fmt.Println("🔍 Example 1: Create pet with nested objects")
	fmt.Println("   Demonstrates: Nested object validation + helper functions")

	petWithNested := cvt.ValidationRequest{
		Method: "POST",
		Path:   "/pet",
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: map[string]interface{}{
			"name":      "Max",
			"photoUrls": []string{"http://example.com/photo1.jpg"},
			"category": map[string]interface{}{
				"id":   1,
				"name": "Dogs",
			},
			"tags": []map[string]interface{}{
				{"id": 1, "name": "friendly"},
				{"id": 2, "name": "trained"},
			},
		},
	}

	petNestedResponse := cvt.ValidationResponse{
		StatusCode: 405,
		Headers: map[string]string{
			"content-type": "application/json",
		},
	}

	nestedResult, err := validator.Validate(ctx, petWithNested, petNestedResponse)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	examples.LogValidationResult("Create Pet with Nested Objects", nestedResult)

	// Example 2: Create multiple users demonstrating code reuse
	fmt.Println("🔍 Example 2: Create multiple users with helper functions")
	fmt.Println("   Demonstrates: Code reuse with helper functions")

	users := []string{"alice", "bob", "charlie"}

	for _, username := range users {
		user := examples.CreateSampleUser()
		user.Username = username

		userRequest := cvt.ValidationRequest{
			Method: "POST",
			Path:   "/user",
			Headers: map[string]string{
				"content-type": "application/json",
			},
			Body: user,
		}

		userResponse := cvt.ValidationResponse{
			StatusCode: 200,
		}

		result, err := validator.Validate(ctx, userRequest, userResponse)
		if err != nil {
			log.Fatalf("Validation failed: %v", err)
		}

		status := "✅"
		if !result.Valid {
			status = "❌"
		}
		fmt.Printf("   User %s: %s\n", username, status)
	}
	fmt.Println()

	// Example 3: GET request with path parameters
	fmt.Println("🔍 Example 3: GET request with path parameter")
	fmt.Println("   Demonstrates: Path parameter validation")

	pet := examples.CreateSamplePet()
	pet.ID = 123

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
		Body: pet,
	}

	getResult, err := validator.Validate(ctx, getRequest, getResponse)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	examples.LogValidationResult("GET Pet by ID", getResult)

	fmt.Println(string(make([]byte, 70)))
	fmt.Println("ERROR SCENARIOS - Demonstrating Validation Failures")
	fmt.Println(string(make([]byte, 70)))
	fmt.Println()

	// Example 4: Invalid pet - missing required fields
	fmt.Println("🔍 Example 4: Invalid pet (missing required fields)")
	fmt.Println("   Demonstrates: Required field validation with detailed errors")

	invalidPetRequest := cvt.ValidationRequest{
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

	invalidPetResponse := cvt.ValidationResponse{
		StatusCode: 405,
	}

	invalidPetResult, err := validator.Validate(ctx, invalidPetRequest, invalidPetResponse)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	examples.LogValidationResult("Invalid Pet (Missing Fields)", invalidPetResult)

	// Example 5: Invalid pet - wrong enum value
	fmt.Println("🔍 Example 5: Invalid pet (wrong enum value)")
	fmt.Println("   Demonstrates: Enum validation")

	invalidEnumRequest := cvt.ValidationRequest{
		Method: "POST",
		Path:   "/pet",
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: map[string]interface{}{
			"name":      "Fluffy",
			"photoUrls": []string{"http://example.com/photo.jpg"},
			"status":    "invalid-status", // Invalid enum value (should be available/pending/sold)
		},
	}

	invalidEnumResponse := cvt.ValidationResponse{
		StatusCode: 405,
	}

	invalidEnumResult, err := validator.Validate(ctx, invalidEnumRequest, invalidEnumResponse)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	examples.LogValidationResult("Invalid Pet (Wrong Enum)", invalidEnumResult)

	fmt.Println(string(make([]byte, 70)))
	fmt.Println("🎉 All advanced examples completed!")
	fmt.Println(string(make([]byte, 70)))
	fmt.Println()
}
