// Package main demonstrates breaking change detection using the CVT SDK.
//
// This example shows how to use the CVT SDK to detect breaking changes
// between two versions of an OpenAPI schema. This is useful for:
// - CI/CD pipelines to prevent breaking API changes from being deployed
// - API governance to ensure backward compatibility
// - Schema evolution tracking
//
// Prerequisites:
// - CVT server running on localhost:50052
// - Run: make up (from project root)
//
// Usage:
// - go run examples/breaking/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cvt/cvt-sdk/go/cvt"
)

const schemaID = "petstore-api"

// getSchemaPath returns the absolute path to a schema file in the shared directory.
func getSchemaPath(filename string) string {
	_, currentFile, _, _ := runtime.Caller(0)
	examplesDir := filepath.Dir(filepath.Dir(currentFile))
	return filepath.Join(examplesDir, "..", "..", "shared", filename)
}

// formatBreakingChange formats a breaking change for display.
func formatBreakingChange(change cvt.BreakingChange, index int) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("%d. %s", index+1, change.Type))
	lines = append(lines, fmt.Sprintf("   %s", change.Description))

	if change.Path != "" {
		lines = append(lines, fmt.Sprintf("   Path: %s %s", change.Method, change.Path))
	}

	if change.OldValue != "" && change.NewValue != "" {
		lines = append(lines, fmt.Sprintf("   Changed: \"%s\" -> \"%s\"", change.OldValue, change.NewValue))
	}

	return strings.Join(lines, "\n")
}

// logCompareResult logs the comparison result in a formatted way.
func logCompareResult(result *cvt.CompareResult) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))

	if result.Compatible {
		fmt.Println("RESULT: COMPATIBLE")
		fmt.Println("No breaking changes detected between schema versions.")
	} else {
		fmt.Println("RESULT: INCOMPATIBLE")
		fmt.Printf("\nBreaking changes detected: %d\n", len(result.BreakingChanges))
		fmt.Println(strings.Repeat("-", 60))

		for i, change := range result.BreakingChanges {
			fmt.Println(formatBreakingChange(change, i))
			fmt.Println()
		}
	}

	fmt.Println(strings.Repeat("=", 60))
}

func main() {
	fmt.Println("=== CVT Breaking Change Detection Example ===")
	fmt.Println()

	ctx := context.Background()

	validator, err := cvt.NewValidator("")
	if err != nil {
		fmt.Printf("Failed to create validator: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = validator.Close()
		fmt.Println("\nValidator closed.")
	}()

	schemaV1Path := getSchemaPath("openapi-v1.json")
	schemaV2Path := getSchemaPath("openapi-v2-breaking.json")

	// Step 1: Register schema v1.0.0
	fmt.Println("Step 1: Registering schema v1.0.0...")
	if err := validator.RegisterSchemaWithVersion(ctx, schemaID, schemaV1Path, "1.0.0"); err != nil {
		fmt.Printf("Failed to register schema v1.0.0: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("        Schema v1.0.0 registered successfully.")
	fmt.Println()

	// Step 2: Register schema v2.0.0 (with breaking changes)
	fmt.Println("Step 2: Registering schema v2.0.0...")
	if err := validator.RegisterSchemaWithVersion(ctx, schemaID, schemaV2Path, "2.0.0"); err != nil {
		fmt.Printf("Failed to register schema v2.0.0: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("        Schema v2.0.0 registered successfully.")
	fmt.Println()

	// Step 3: Compare the two versions
	fmt.Println("Step 3: Comparing schema versions 1.0.0 and 2.0.0...")
	result, err := validator.CompareSchemas(ctx, schemaID, "1.0.0", "2.0.0")
	if err != nil {
		fmt.Printf("Failed to compare schemas: %v\n", err)
		os.Exit(1)
	}

	// Display the results
	logCompareResult(result)

	// Step 4: Demonstrate CI/CD integration pattern
	fmt.Println()
	fmt.Println("--- CI/CD Integration Example ---")
	if !result.Compatible {
		fmt.Println("In a CI/CD pipeline, you would fail the build here:")
		fmt.Println("  os.Exit(1) // Fail build due to breaking changes")
		fmt.Println()
		fmt.Println("Or create a report for review:")
		for _, change := range result.BreakingChanges {
			fmt.Printf("  - [%s] %s\n", change.Type, change.Description)
		}
	} else {
		fmt.Println("Schema changes are backward compatible. Safe to deploy!")
	}
}
