// Package examples provides common utilities and test data for CVT Go SDK examples.
package examples

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/sahina/cvt/sdks/go/cvt"
)

// Pet represents a pet entity from the Petstore API
type Pet struct {
	ID        int64     `json:"id,omitempty"`
	Name      string    `json:"name"`
	PhotoUrls []string  `json:"photoUrls"`
	Status    string    `json:"status,omitempty"`
	Category  *Category `json:"category,omitempty"`
	Tags      []Tag     `json:"tags,omitempty"`
}

// User represents a user entity from the Petstore API
type User struct {
	ID         int64  `json:"id,omitempty"`
	Username   string `json:"username"`
	FirstName  string `json:"firstName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
	Email      string `json:"email,omitempty"`
	Password   string `json:"password,omitempty"`
	Phone      string `json:"phone,omitempty"`
	UserStatus int    `json:"userStatus,omitempty"`
}

// Order represents a store order entity
type Order struct {
	ID       int64  `json:"id,omitempty"`
	PetID    int64  `json:"petId,omitempty"`
	Quantity int    `json:"quantity,omitempty"`
	ShipDate string `json:"shipDate,omitempty"`
	Status   string `json:"status,omitempty"`
	Complete bool   `json:"complete,omitempty"`
}

// Category represents a pet category
type Category struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Tag represents a pet tag
type Tag struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// GetOpenAPISchemaPath returns the path to the shared OpenAPI schema
func GetOpenAPISchemaPath() string {
	_, filename, _, _ := runtime.Caller(0)
	examplesDir := filepath.Dir(filename)
	return filepath.Join(examplesDir, "..", "..", "shared", "openapi.json")
}

// GetSwaggerSchemaPath returns the path to the shared Swagger 2.0 schema
func GetSwaggerSchemaPath() string {
	_, filename, _, _ := runtime.Caller(0)
	examplesDir := filepath.Dir(filename)
	return filepath.Join(examplesDir, "..", "..", "shared", "swagger.json")
}

// GetOpenAPIV1SchemaPath returns the path to the OpenAPI v1 schema for breaking change testing
func GetOpenAPIV1SchemaPath() string {
	_, filename, _, _ := runtime.Caller(0)
	examplesDir := filepath.Dir(filename)
	return filepath.Join(examplesDir, "..", "..", "shared", "openapi-v1.json")
}

// GetOpenAPIV2BreakingSchemaPath returns the path to the OpenAPI v2 schema with breaking changes
func GetOpenAPIV2BreakingSchemaPath() string {
	_, filename, _, _ := runtime.Caller(0)
	examplesDir := filepath.Dir(filename)
	return filepath.Join(examplesDir, "..", "..", "shared", "openapi-v2-breaking.json")
}

// CreateSamplePet creates a sample pet with optional overrides
func CreateSamplePet() Pet {
	return Pet{
		Name:      "Fluffy",
		PhotoUrls: []string{"http://example.com/photo1.jpg"},
		Status:    "available",
	}
}

// CreateSampleUser creates a sample user with optional overrides
func CreateSampleUser() User {
	return User{
		Username:  "alice",
		FirstName: "Alice",
		LastName:  "Smith",
		Email:     "alice@example.com",
		Password:  "password123",
		Phone:     "123-456-7890",
	}
}

// CreateSampleOrder creates a sample order
func CreateSampleOrder() Order {
	return Order{
		PetID:    1,
		Quantity: 1,
		Status:   "placed",
		Complete: false,
	}
}

// LogValidationResult logs validation results in a consistent format
func LogValidationResult(testName string, result *cvt.ValidationResult) {
	fmt.Printf("\n%s\n", testName)
	if result.Valid {
		fmt.Println("Result: ✅ Valid")
	} else {
		fmt.Println("Result: ❌ Invalid")
		if len(result.Errors) > 0 {
			fmt.Printf("Errors: %v\n", result.Errors)
		}
	}
	fmt.Println()
}

// LogBreakingChanges logs breaking changes in a consistent format
func LogBreakingChanges(result *cvt.CompareResult) {
	if result.Compatible {
		fmt.Println("No breaking changes detected.")
		return
	}

	fmt.Printf("Breaking changes detected: %d\n", len(result.BreakingChanges))
	fmt.Println("--------------------------------------------------")

	for i, change := range result.BreakingChanges {
		fmt.Printf("%d. [%s]\n", i+1, change.Type)
		fmt.Printf("   %s\n", change.Description)
		if change.Path != "" {
			fmt.Printf("   Path: %s %s\n", change.Method, change.Path)
		}
		fmt.Println()
	}
}
