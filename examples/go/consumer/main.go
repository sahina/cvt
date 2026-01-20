// Package main demonstrates CVT consumer testing capabilities.
//
// This example shows the full consumer testing workflow:
//   - Register a producer's OpenAPI schema
//   - Validate API interactions against the schema
//   - Register as a consumer (two approaches):
//     a) AUTO: From captured test interactions (recommended)
//     b) MANUAL: Specify endpoints explicitly
//   - List registered consumers
//   - Check deployment safety with CanIDeploy
//   - Deregister consumer (cleanup)
//
// Prerequisites:
//   - CVT server running on localhost:9550
//   - Run: make run-server (from project root)
//
// Usage:
//
//	cd examples/go/consumer
//	go run main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sahina/cvt/sdks/go/cvt"
	"github.com/sahina/cvt/sdks/go/cvt/adapters"
)

const schemaID = "user-api"

// getSchemaPath returns the absolute path to a schema file in the examples/schemas directory.
func getSchemaPath(filename string) string {
	_, currentFile, _, _ := runtime.Caller(0)
	// currentFile is examples/go/consumer/main.go
	// go up 3 levels to get to examples/, then into schemas/
	examplesDir := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
	return filepath.Join(examplesDir, "schemas", filename)
}

func main() {
	fmt.Println("=== CVT Consumer Testing Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates the full consumer testing workflow:")
	fmt.Println("  1. Register producer's OpenAPI schema")
	fmt.Println("  2. Demonstrate version mismatch enforcement")
	fmt.Println("  3. Validate API interactions (two approaches):")
	fmt.Println("     a) Manual: Build request/response structs")
	fmt.Println("     b) MockingRoundTripper: Auto-generate from schema")
	fmt.Println("  4. Register as a consumer (two approaches):")
	fmt.Println("     a) AUTO: From captured test interactions (RECOMMENDED)")
	fmt.Println("     b) MANUAL: Specify endpoints explicitly")
	fmt.Println("  5. List registered consumers")
	fmt.Println("  6. Check deployment safety (CanIDeploy)")
	fmt.Println("  7. Cleanup (deregister consumer)")
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))

	ctx := context.Background()

	// Initialize the validator (connects to CVT server)
	// Use CVT_SERVER_ADDRESS env var, or default to localhost:9550 (local server)
	serverAddr := os.Getenv("CVT_SERVER_ADDRESS")
	if serverAddr == "" {
		serverAddr = "localhost:9550" // Default for local server (make run-server)
	}
	fmt.Printf("Connecting to CVT server at %s...\n", serverAddr)

	validator, err := cvt.NewValidator(serverAddr)
	if err != nil {
		fmt.Printf("Failed to create validator: %v\n", err)
		fmt.Println("\nMake sure CVT server is running:")
		fmt.Println("  make run-server  (local, port 9550)")
		fmt.Println("  make up          (Docker, port 9550)")
		os.Exit(1)
	}
	defer func() { _ = validator.Close() }()

	// ========================================
	// Step 1: Register Schema v1.0.0
	// ========================================
	fmt.Println()
	fmt.Println("Step 1: Registering producer's OpenAPI schema (v1.0.0)...")

	schemaV1Path := getSchemaPath("user-api-v1.json")
	fmt.Printf("        Schema path: %s\n", schemaV1Path)

	if err := validator.RegisterSchemaWithVersion(ctx, schemaID, schemaV1Path, "1.0.0"); err != nil {
		fmt.Printf("Failed to register schema v1.0.0: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("        Schema v1.0.0 registered successfully.")

	// ========================================
	// Step 2: Demonstrate Version Mismatch Enforcement
	// ========================================
	fmt.Println()
	fmt.Println("Step 2: Demonstrating version mismatch enforcement...")
	fmt.Println("        Attempting to register schema with mismatched version...")
	fmt.Println("        (Schema has info.version='1.0.0', but we'll provide '9.9.9')")

	// This should fail because the provided version doesn't match schema's info.version
	err = validator.RegisterSchemaWithVersion(ctx, "mismatch-test", schemaV1Path, "9.9.9")
	if err != nil {
		fmt.Println("        Expected error received:")
		fmt.Printf("        %v\n", err)
		fmt.Println()
		fmt.Println("        This demonstrates that CVT enforces version consistency:")
		fmt.Println("        The provided version MUST match the schema's info.version field.")
	} else {
		fmt.Println("        WARNING: Expected version mismatch error but registration succeeded!")
	}

	// ========================================
	// Step 3a: Validate API Interactions (Manual Approach)
	// ========================================
	fmt.Println()
	fmt.Println("Step 3a: Validating API interactions (MANUAL approach)...")
	fmt.Println("         This approach requires you to construct request/response structs.")

	// Valid interaction: GET /users/{id}
	validRequest := cvt.ValidationRequest{
		Method: "GET",
		Path:   "/users/123",
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}
	validResponse := cvt.ValidationResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]any{
			"id":    "123",
			"name":  "John Doe",
			"email": "john@example.com",
			"role":  "user",
		},
	}

	result, err := validator.Validate(ctx, validRequest, validResponse)
	if err != nil {
		fmt.Printf("        Validation error: %v\n", err)
		os.Exit(1)
	}

	if result.Valid {
		fmt.Println("        Valid interaction: GET /users/123")
	} else {
		fmt.Printf("        Validation failed: %v\n", result.Errors)
	}

	// Invalid interaction: missing required fields
	fmt.Println()
	fmt.Println("         Testing INVALID response (missing required fields)...")
	invalidResponse := cvt.ValidationResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]any{
			"id": "123",
			// Missing "name" and "email" which are required
		},
	}
	result, err = validator.Validate(ctx, validRequest, invalidResponse)
	if err != nil {
		fmt.Printf("        Validation error: %v\n", err)
	} else if !result.Valid {
		fmt.Println("        Correctly detected invalid response:")
		for _, e := range result.Errors {
			fmt.Printf("          - %s\n", e)
		}
	} else {
		fmt.Println("        WARNING: Expected validation to fail but it passed!")
	}

	// POST request with body validation
	fmt.Println()
	fmt.Println("         Testing POST /users with request body...")
	postRequest := cvt.ValidationRequest{
		Method: "POST",
		Path:   "/users",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		Body: map[string]any{
			"name":  "Jane Doe",
			"email": "jane@example.com",
		},
	}
	postResponse := cvt.ValidationResponse{
		StatusCode: 201,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]any{
			"id":    "456",
			"name":  "Jane Doe",
			"email": "jane@example.com",
			"role":  "user",
		},
	}
	result, err = validator.Validate(ctx, postRequest, postResponse)
	if err != nil {
		fmt.Printf("        Validation error: %v\n", err)
	} else if result.Valid {
		fmt.Println("        Valid interaction: POST /users (201 Created)")
	} else {
		fmt.Printf("        Validation failed: %v\n", result.Errors)
	}

	// 404 error response validation
	fmt.Println()
	fmt.Println("         Testing 404 Not Found response...")
	notFoundRequest := cvt.ValidationRequest{
		Method: "GET",
		Path:   "/users/nonexistent",
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}
	notFoundResponse := cvt.ValidationResponse{
		StatusCode: 404,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
	result, err = validator.Validate(ctx, notFoundRequest, notFoundResponse)
	if err != nil {
		fmt.Printf("        Validation error: %v\n", err)
	} else if result.Valid {
		fmt.Println("        Valid interaction: GET /users/nonexistent (404 Not Found)")
	} else {
		fmt.Printf("        Validation failed: %v\n", result.Errors)
	}

	// ========================================
	// Step 3b: Validate API Interactions (MockingRoundTripper)
	// ========================================
	fmt.Println()
	fmt.Println("Step 3b: Validating API interactions (MOCK CLIENT approach)...")
	fmt.Println("         This approach auto-generates mock responses from the schema.")
	fmt.Println("         No real API endpoint needed!")

	// Simplest way: one-liner to get a mock http.Client
	// mockClient := adapters.NewMockClient(validator)

	// If you need to inspect interactions, use NewMock wrapper:
	mock := adapters.NewMock(validator, adapters.WithCache())
	mockClient := mock.Client()

	// Create a request - response is auto-generated from schema!
	req, _ := http.NewRequest("GET", "http://mock.user-api/users/456", nil)
	req.Header.Set("Accept", "application/json")
	req = req.WithContext(ctx)

	// Get respsponse from mock client
	resp, err := mockClient.Do(req)
	if err != nil {
		fmt.Printf("        Mock request error: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read and display the auto-generated response
	body, _ := io.ReadAll(resp.Body)
	var mockData map[string]any
	_ = json.Unmarshal(body, &mockData)

	fmt.Printf("        Mock response status: %d\n", resp.StatusCode)
	fmt.Printf("        Mock response body: %v\n", mockData)

	// Check recorded interactions (available via Mock wrapper)
	interactions := mock.GetInteractions()
	fmt.Printf("        Recorded interactions: %d\n", len(interactions))
	if len(interactions) > 0 {
		fmt.Printf("        Last request: %s %s\n",
			interactions[0].Request.Method,
			interactions[0].Request.Path)
	}

	// ========================================
	// Step 4a: Register as a Consumer (AUTO - from captured interactions)
	// ========================================
	fmt.Println()
	fmt.Println("Step 4a: Registering as a consumer (AUTO from test interactions)...")
	fmt.Println("         This is the RECOMMENDED approach!")
	fmt.Println("         Endpoints and fields are extracted automatically from mock interactions.")

	// Use the interactions captured from Step 3b
	autoConsumerInfo, err := validator.RegisterConsumerFromInteractions(ctx, interactions, cvt.AutoRegisterConfig{
		ConsumerID:      "order-service-auto",
		ConsumerVersion: "2.1.0",
		Environment:     "dev",
		SchemaVersion:   "1.0.0",
		// SchemaID is auto-extracted from URL: http://mock.user-api/... -> "user-api"
	})
	if err != nil {
		fmt.Printf("         Failed to auto-register consumer: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("         Consumer registered: %s v%s\n", autoConsumerInfo.ConsumerID, autoConsumerInfo.ConsumerVersion)
	fmt.Printf("         Uses schema: %s v%s\n", autoConsumerInfo.SchemaID, autoConsumerInfo.SchemaVersion)
	fmt.Printf("         Environment: %s\n", autoConsumerInfo.Environment)
	fmt.Printf("         Auto-detected endpoints: %d\n", len(autoConsumerInfo.UsedEndpoints))
	for _, ep := range autoConsumerInfo.UsedEndpoints {
		fmt.Printf("           - %s %s (fields: %v)\n", ep.Method, ep.Path, ep.UsedFields)
	}

	// ========================================
	// Step 4b: Register as a Consumer (MANUAL - explicit endpoints)
	// ========================================
	fmt.Println()
	fmt.Println("Step 4b: Registering as a consumer (MANUAL with explicit endpoints)...")
	fmt.Println("         Use this approach when you need fine-grained control.")

	consumerInfo, err := validator.RegisterConsumer(ctx, cvt.RegisterConsumerOptions{
		ConsumerID:      "order-service",
		ConsumerVersion: "2.1.0",
		SchemaID:        schemaID,
		SchemaVersion:   "1.0.0",
		Environment:     "dev",
		UsedEndpoints: []cvt.EndpointUsage{
			{
				Method:     "GET",
				Path:       "/users/{id}",
				UsedFields: []string{"id", "name", "email"},
			},
			{
				Method:     "DELETE",
				Path:       "/users/{id}",
				UsedFields: []string{},
			},
		},
	})
	if err != nil {
		fmt.Printf("         Failed to register consumer: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("         Consumer registered: %s v%s\n", consumerInfo.ConsumerID, consumerInfo.ConsumerVersion)
	fmt.Printf("         Uses schema: %s v%s\n", consumerInfo.SchemaID, consumerInfo.SchemaVersion)
	fmt.Printf("         Environment: %s\n", consumerInfo.Environment)
	fmt.Printf("         Tracked endpoints: %d\n", len(consumerInfo.UsedEndpoints))

	// ========================================
	// Step 5: List Registered Consumers
	// ========================================
	fmt.Println()
	fmt.Println("Step 5: Listing registered consumers for this schema...")

	consumers, err := validator.ListConsumers(ctx, schemaID, "dev")
	if err != nil {
		fmt.Printf("        Failed to list consumers: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("        Found %d consumer(s) in dev environment:\n", len(consumers))
	for _, c := range consumers {
		fmt.Printf("          - %s v%s (uses schema v%s)\n", c.ConsumerID, c.ConsumerVersion, c.SchemaVersion)
	}

	// ========================================
	// Step 6: Register Schema v2.0.0 with Breaking Changes
	// ========================================
	fmt.Println()
	fmt.Println("Step 6: Registering schema v2.0.0 (with breaking changes)...")

	schemaV2Path := getSchemaPath("user-api-v2.json")
	if err := validator.RegisterSchemaWithVersion(ctx, schemaID, schemaV2Path, "2.0.0"); err != nil {
		fmt.Printf("        Failed to register schema v2.0.0: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("        Schema v2.0.0 registered successfully.")
	fmt.Println()
	fmt.Println("        Breaking changes in v2.0.0:")
	fmt.Println("          - DELETE /users/{id}: Endpoint removed")
	fmt.Println("          - User.email: Field removed from schema")

	// ========================================
	// Step 7: Check Deployment Safety (CanIDeploy)
	// ========================================
	fmt.Println()
	fmt.Println("Step 7: Checking deployment safety (can-i-deploy v2.0.0 to dev)...")

	deployResult, err := validator.CanIDeploy(ctx, schemaID, "2.0.0", "dev")
	if err != nil {
		fmt.Printf("        Failed to check deployment safety: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 60))
	if deployResult.SafeToDeploy {
		fmt.Println("RESULT: SAFE TO DEPLOY")
		fmt.Println("No breaking changes affect registered consumers.")
	} else {
		fmt.Println("RESULT: UNSAFE TO DEPLOY")
		fmt.Printf("\nSummary: %s\n", deployResult.Summary)

		if len(deployResult.BreakingChanges) > 0 {
			fmt.Printf("\nBreaking changes detected: %d\n", len(deployResult.BreakingChanges))
			for i, change := range deployResult.BreakingChanges {
				fmt.Printf("  %d. [%s] %s\n", i+1, change.Type, change.Description)
				if change.Path != "" {
					fmt.Printf("     Path: %s %s\n", change.Method, change.Path)
				}
			}
		}

		if len(deployResult.AffectedConsumers) > 0 {
			fmt.Printf("\nAffected consumers: %d\n", len(deployResult.AffectedConsumers))
			for _, consumer := range deployResult.AffectedConsumers {
				impact := "NONE"
				if consumer.WillBreak {
					impact = "BREAKING"
				}
				fmt.Printf("  - %s v%s (impact: %s)\n",
					consumer.ConsumerID,
					consumer.ConsumerVersion,
					impact)
				if len(consumer.RelevantChanges) > 0 {
					fmt.Println("    Relevant breaking changes:")
					for _, change := range consumer.RelevantChanges {
						fmt.Printf("      - [%s] %s\n", change.Type, change.Description)
					}
				}
			}
		}
	}
	fmt.Println(strings.Repeat("-", 60))

	// ========================================
	// Step 8: Cleanup - Deregister Consumers
	// ========================================
	fmt.Println()
	fmt.Println("Step 8: Cleaning up (deregistering consumers)...")

	// Deregister the auto-registered consumer
	if err := validator.DeregisterConsumer(ctx, "order-service-auto", schemaID, "dev"); err != nil {
		fmt.Printf("        Failed to deregister order-service-auto: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("        Deregistered: order-service-auto")

	// Deregister the manually-registered consumer
	if err := validator.DeregisterConsumer(ctx, "order-service", schemaID, "dev"); err != nil {
		fmt.Printf("        Failed to deregister order-service: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("        Deregistered: order-service")

	// Verify cleanup
	consumers, err = validator.ListConsumers(ctx, schemaID, "dev")
	if err != nil {
		fmt.Printf("        Failed to list consumers: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("        Remaining consumers in dev: %d\n", len(consumers))

	// ========================================
	// Summary
	// ========================================
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Consumer Testing Example Complete!")
	fmt.Println()
	fmt.Println("What we demonstrated:")
	fmt.Println("  1. Schema registration (v1.0.0 and v2.0.0)")
	fmt.Println("  2. Version mismatch enforcement (provided version must match info.version)")
	fmt.Println("  3. API interaction validation (two approaches):")
	fmt.Println("     a) Manual: Build ValidationRequest/ValidationResponse structs")
	fmt.Println("     b) MockingRoundTripper: Use http.Client with auto-generated responses")
	fmt.Println("  4. Consumer registration (two approaches):")
	fmt.Println("     a) AUTO: RegisterConsumerFromInteractions - extracts from test interactions")
	fmt.Println("     b) MANUAL: RegisterConsumer - specify endpoints explicitly")
	fmt.Println("  5. Consumer listing")
	fmt.Println("  6. Deployment safety checking (CanIDeploy)")
	fmt.Println("  7. Consumer deregistration")
	fmt.Println()
	fmt.Println("Key takeaways:")
	fmt.Println("  - MockingRoundTripper enables testing without a real API endpoint")
	fmt.Println("  - Auto-registration eliminates manual endpoint specification")
	fmt.Println("  - CanIDeploy prevents unsafe deployments by identifying breaking changes")
	fmt.Println(strings.Repeat("=", 60))
}
