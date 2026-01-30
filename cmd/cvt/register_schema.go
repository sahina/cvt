package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	pb "github.com/sahina/cvt/server/pb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func registerSchemaCmd() *cobra.Command {
	var (
		serverAddr         string
		schemaVersion      string
		checkCompatibility bool
		failOnBreaking     bool
		outputJSON         bool
		quiet              bool
		timeout            int
		owner              string
		team               string
	)

	cmd := &cobra.Command{
		Use:   "register-schema <schema-id> <schema-file>",
		Short: "Register an OpenAPI schema with the CVT server",
		Long: `Register an OpenAPI schema with the CVT server for validation and tracking.

The schema file can be in YAML or JSON format. Once registered, the schema
can be used for validation, consumer registration, and deployment safety checks.

Examples:
  # Register a schema
  cvt register-schema my-api ./openapi.yaml

  # Register with a specific version
  cvt register-schema my-api ./openapi.yaml --version 2.0.0

  # Register and check for breaking changes against previous version
  cvt register-schema my-api ./openapi.yaml --check-compatibility

  # Fail if breaking changes detected (for CI/CD)
  cvt register-schema my-api ./openapi.yaml --check-compatibility --fail-on-breaking

  # Output as JSON
  cvt register-schema my-api ./openapi.yaml --json

  # With ownership information
  cvt register-schema my-api ./openapi.yaml --owner "John Doe" --team "Platform"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaID := args[0]
			schemaFile := args[1]

			// Read schema file
			content, err := os.ReadFile(schemaFile)
			if err != nil {
				return fmt.Errorf("failed to read schema file: %w", err)
			}

			// Connect to server
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			defer cancel()

			conn, err := grpc.NewClient(serverAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				return fmt.Errorf("failed to connect to server %s: %w", serverAddr, err)
			}
			defer func() { _ = conn.Close() }()

			client := pb.NewContractValidatorClient(conn)

			// Build request
			req := &pb.RegisterSchemaRequest{
				SchemaId:           schemaID,
				SchemaContent:      string(content),
				SchemaVersion:      schemaVersion,
				CheckCompatibility: checkCompatibility,
			}

			// Add ownership if provided
			if owner != "" || team != "" {
				req.Ownership = &pb.SchemaOwnership{
					Owner: owner,
					Team:  team,
				}
			}

			resp, err := client.RegisterSchema(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to register schema: %w", err)
			}

			// Output result
			if outputJSON {
				return outputRegisterJSON(resp, failOnBreaking)
			}
			return outputRegisterHuman(resp, schemaID, schemaFile, quiet, failOnBreaking)
		},
	}

	cmd.Flags().StringVarP(&serverAddr, "server", "S", "localhost:9550", "CVT server address")
	cmd.Flags().StringVarP(&schemaVersion, "version", "v", "", "Schema version (optional, extracted from spec if not provided)")
	cmd.Flags().BoolVar(&checkCompatibility, "check-compatibility", false, "Check for breaking changes against previous version")
	cmd.Flags().BoolVar(&failOnBreaking, "fail-on-breaking", false, "Exit with error if breaking changes detected")
	cmd.Flags().BoolVarP(&outputJSON, "json", "j", false, "Output result as JSON")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress output except errors")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 30, "Connection timeout in seconds")
	cmd.Flags().StringVar(&owner, "owner", "", "Schema owner name")
	cmd.Flags().StringVar(&team, "team", "", "Owning team name")

	return cmd
}

func outputRegisterJSON(resp *pb.RegisterSchemaResponse, failOnBreaking bool) error {
	result := struct {
		Success         bool                   `json:"success"`
		Message         string                 `json:"message"`
		SchemaID        string                 `json:"schema_id,omitempty"`
		Version         string                 `json:"version,omitempty"`
		BreakingChanges []breakingChangeOutput `json:"breaking_changes,omitempty"`
	}{
		Success: resp.Success,
		Message: resp.Message,
	}

	if resp.Metadata != nil {
		result.SchemaID = resp.Metadata.SchemaId
		result.Version = resp.Metadata.SchemaVersion
	}

	for _, bc := range resp.BreakingChanges {
		if bc == nil {
			continue
		}
		result.BreakingChanges = append(result.BreakingChanges, breakingChangeOutput{
			Type:        bc.Type.String(),
			Path:        bc.Path,
			Method:      bc.Method,
			Description: bc.Description,
			OldValue:    bc.OldValue,
			NewValue:    bc.NewValue,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("failed to encode result: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("registration failed")
	}

	if failOnBreaking && len(resp.BreakingChanges) > 0 {
		return fmt.Errorf("breaking changes detected")
	}

	return nil
}

func outputRegisterHuman(resp *pb.RegisterSchemaResponse, schemaID, schemaFile string, quiet, failOnBreaking bool) error {
	if resp.Success {
		if !quiet {
			fmt.Printf("✓ Schema registered successfully\n")
			fmt.Printf("  Schema ID: %s\n", schemaID)
			fmt.Printf("  File:      %s\n", schemaFile)
			if resp.Metadata != nil && resp.Metadata.SchemaVersion != "" {
				fmt.Printf("  Version:   %s\n", resp.Metadata.SchemaVersion)
			}
			if resp.Message != "" {
				fmt.Printf("  Message:   %s\n", resp.Message)
			}
		}
	} else {
		fmt.Printf("✗ Failed to register schema: %s\n", resp.Message)
		return fmt.Errorf("registration failed")
	}

	if len(resp.BreakingChanges) > 0 {
		if !quiet {
			fmt.Printf("\n⚠ Breaking changes detected:\n")
			for _, bc := range resp.BreakingChanges {
				if bc == nil {
					continue
				}
				fmt.Printf("  - %s: %s %s\n", bc.Type.String(), bc.Method, bc.Path)
				fmt.Printf("    %s\n", bc.Description)
				if bc.OldValue != "" || bc.NewValue != "" {
					fmt.Printf("    Old: %s → New: %s\n", bc.OldValue, bc.NewValue)
				}
			}
		}

		if failOnBreaking {
			return fmt.Errorf("breaking changes detected")
		}
	}

	return nil
}
