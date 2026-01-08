package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cvt/cvt/pkg/cvt"
	"github.com/spf13/cobra"
)

func generateCmd() *cobra.Command {
	var (
		schemaFile  string
		method      string
		path        string
		statusCode  int
		useExamples bool
		contentType string
		outputFile  string
		outputType  string
		listOnly    bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate test fixtures from an OpenAPI schema",
		Long: `Generate test fixtures (request/response pairs) from an OpenAPI schema.

This command helps you create test data for contract testing by extracting
examples or generating values based on the schema definition.

Output Types:
  fixture   - Complete request/response pair (default)
  request   - Request body only
  response  - Response only

Examples:
  # List all available endpoints
  cvt generate --schema ./openapi.json --list

  # Generate a complete fixture for an endpoint
  cvt generate --schema ./openapi.json --method POST --path /users

  # Generate response only with specific status code
  cvt generate --schema ./openapi.json --method GET --path /users/123 --output-type response

  # Generate request body only
  cvt generate --schema ./openapi.json --method POST --path /users --output-type request

  # Use schema examples instead of generated values
  cvt generate --schema ./openapi.json --method GET --path /users/123 --use-examples

  # Save output to file
  cvt generate --schema ./openapi.json --method GET --path /users/123 -o fixture.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load schema
			if schemaFile == "" {
				return fmt.Errorf("--schema is required")
			}

			v := cvt.NewValidator()
			if err := v.RegisterSchemaFromFile("schema", schemaFile); err != nil {
				return fmt.Errorf("failed to load schema: %w", err)
			}

			// List endpoints if requested
			if listOnly {
				endpoints, err := v.ListEndpoints("schema")
				if err != nil {
					return fmt.Errorf("failed to list endpoints: %w", err)
				}
				fmt.Println("Available endpoints:")
				for _, ep := range endpoints {
					fmt.Printf("  %s\n", ep)
				}
				return nil
			}

			// Validate required flags for generation
			if method == "" || path == "" {
				return fmt.Errorf("--method and --path are required (or use --list to see available endpoints)")
			}

			// Build generation options
			opts := cvt.DefaultGenerateOptions()
			opts.StatusCode = statusCode
			opts.UseExamples = useExamples
			if contentType != "" {
				opts.ContentType = contentType
			}

			// Generate output based on type
			var output interface{}
			var err error

			switch strings.ToLower(outputType) {
			case "request":
				output, err = v.GenerateRequestBody("schema", strings.ToUpper(method), path, opts)
				if err != nil {
					return fmt.Errorf("failed to generate request body: %w", err)
				}
			case "response":
				output, err = v.GenerateResponse("schema", strings.ToUpper(method), path, opts)
				if err != nil {
					return fmt.Errorf("failed to generate response: %w", err)
				}
			case "fixture", "":
				output, err = v.GenerateFixture("schema", strings.ToUpper(method), path, opts)
				if err != nil {
					return fmt.Errorf("failed to generate fixture: %w", err)
				}
			default:
				return fmt.Errorf("unknown output type: %s (valid: fixture, request, response)", outputType)
			}

			// Format output as JSON
			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}

			// Write output
			if outputFile != "" {
				if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Printf("✓ Generated %s saved to %s\n", outputType, outputFile)
			} else {
				fmt.Println(string(jsonData))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&schemaFile, "schema", "s", "", "Path to OpenAPI schema file (required)")
	cmd.Flags().StringVarP(&method, "method", "m", "", "HTTP method (GET, POST, PUT, DELETE, etc.)")
	cmd.Flags().StringVarP(&path, "path", "p", "", "API path (e.g., /users, /users/{id})")
	cmd.Flags().IntVar(&statusCode, "status-code", 0, "Response status code (default: first successful status)")
	cmd.Flags().BoolVar(&useExamples, "use-examples", true, "Use schema examples when available")
	cmd.Flags().StringVar(&contentType, "content-type", "application/json", "Content type for request/response")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().StringVarP(&outputType, "output-type", "t", "fixture", "Output type: fixture, request, response")
	cmd.Flags().BoolVarP(&listOnly, "list", "l", false, "List available endpoints and exit")

	return cmd
}
