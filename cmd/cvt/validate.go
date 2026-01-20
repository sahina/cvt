package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sahina/cvt/pkg/cvt"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	var (
		schemaFile      string
		requestFile     string
		responseFile    string
		interactionFile string
		method          string
		path            string
		requestBody     string
		statusCode      int
		responseBody    string
		outputJSON      bool
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate an HTTP interaction against an OpenAPI schema",
		Long: `Validate an HTTP request/response pair against an OpenAPI schema.

You can provide the interaction data in multiple ways:
1. Using a single interaction JSON file (--interaction)
2. Using separate request and response files (--request, --response)
3. Using command-line flags for simple cases (--method, --path, etc.)

Examples:
  # Validate using an interaction file
  cvt validate --schema ./openapi.json --interaction ./interaction.json

  # Validate using separate request/response files
  cvt validate --schema ./openapi.json --request req.json --response resp.json

  # Validate using flags
  cvt validate --schema ./openapi.json --method GET --path /pets --status-code 200 --response-body '[]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load schema
			if schemaFile == "" {
				return fmt.Errorf("--schema is required")
			}

			v := cvt.NewValidator()
			if err := v.RegisterSchemaFromFile("schema", schemaFile); err != nil {
				return fmt.Errorf("failed to load schema: %w", err)
			}

			// Build interaction
			var interaction cvt.Interaction

			if interactionFile != "" {
				// Load from interaction file
				data, err := os.ReadFile(interactionFile)
				if err != nil {
					return fmt.Errorf("failed to read interaction file: %w", err)
				}

				// Try to parse as fixture format first (nested request/response)
				var fixtureFormat struct {
					Request struct {
						Method  string            `json:"method"`
						Path    string            `json:"path"`
						Headers map[string]string `json:"headers"`
						Body    interface{}       `json:"body"`
					} `json:"request"`
					Response struct {
						StatusCode int               `json:"statusCode"`
						Headers    map[string]string `json:"headers"`
						Body       interface{}       `json:"body"`
					} `json:"response"`
				}

				if err := json.Unmarshal(data, &fixtureFormat); err == nil && fixtureFormat.Request.Method != "" {
					// Successfully parsed as fixture format
					interaction.Method = fixtureFormat.Request.Method
					interaction.Path = fixtureFormat.Request.Path
					interaction.Headers = fixtureFormat.Request.Headers
					if fixtureFormat.Request.Body != nil {
						bodyBytes, _ := json.Marshal(fixtureFormat.Request.Body)
						interaction.Body = string(bodyBytes)
					}
					interaction.StatusCode = fixtureFormat.Response.StatusCode
					interaction.ResponseHeaders = fixtureFormat.Response.Headers
					if fixtureFormat.Response.Body != nil {
						bodyBytes, _ := json.Marshal(fixtureFormat.Response.Body)
						interaction.ResponseBody = string(bodyBytes)
					}
				} else {
					// Try parsing as flat Interaction format
					if err := json.Unmarshal(data, &interaction); err != nil {
						return fmt.Errorf("failed to parse interaction file: %w", err)
					}
				}
			} else if requestFile != "" && responseFile != "" {
				// Load from separate files
				reqData, err := os.ReadFile(requestFile)
				if err != nil {
					return fmt.Errorf("failed to read request file: %w", err)
				}

				var reqPart struct {
					Method  string            `json:"method"`
					Path    string            `json:"path"`
					Headers map[string]string `json:"headers"`
					Body    string            `json:"body"`
				}
				if err := json.Unmarshal(reqData, &reqPart); err != nil {
					return fmt.Errorf("failed to parse request file: %w", err)
				}

				respData, err := os.ReadFile(responseFile)
				if err != nil {
					return fmt.Errorf("failed to read response file: %w", err)
				}

				var respPart struct {
					StatusCode int               `json:"status_code"`
					Headers    map[string]string `json:"headers"`
					Body       string            `json:"body"`
				}
				if err := json.Unmarshal(respData, &respPart); err != nil {
					return fmt.Errorf("failed to parse response file: %w", err)
				}

				interaction = cvt.Interaction{
					Method:          reqPart.Method,
					Path:            reqPart.Path,
					Headers:         reqPart.Headers,
					Body:            reqPart.Body,
					StatusCode:      respPart.StatusCode,
					ResponseHeaders: respPart.Headers,
					ResponseBody:    respPart.Body,
				}
			} else if method != "" && path != "" {
				// Build from flags
				interaction = cvt.Interaction{
					Method:       method,
					Path:         path,
					Body:         requestBody,
					StatusCode:   statusCode,
					ResponseBody: responseBody,
				}
			} else {
				return fmt.Errorf("must provide --interaction, or --request and --response, or --method and --path")
			}

			// Validate
			result, err := v.Validate("schema", &interaction)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			// Output result
			if outputJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(result); err != nil {
					return fmt.Errorf("failed to encode result: %w", err)
				}
			} else {
				if result.Valid {
					fmt.Println("✓ Validation passed")
				} else {
					fmt.Println("✗ Validation failed")
					for _, e := range result.Errors {
						fmt.Printf("  - %s\n", e)
					}
					os.Exit(1)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&schemaFile, "schema", "s", "", "Path to OpenAPI schema file (required)")
	cmd.Flags().StringVarP(&interactionFile, "interaction", "i", "", "Path to interaction JSON file")
	cmd.Flags().StringVar(&requestFile, "request", "", "Path to request JSON file")
	cmd.Flags().StringVar(&responseFile, "response", "", "Path to response JSON file")
	cmd.Flags().StringVarP(&method, "method", "m", "", "HTTP method (GET, POST, etc.)")
	cmd.Flags().StringVarP(&path, "path", "p", "", "Request path (e.g., /pets)")
	cmd.Flags().StringVar(&requestBody, "request-body", "", "Request body as JSON string")
	cmd.Flags().IntVar(&statusCode, "status-code", 200, "Response status code")
	cmd.Flags().StringVar(&responseBody, "response-body", "", "Response body as JSON string")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output result as JSON")

	return cmd
}
