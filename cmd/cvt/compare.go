package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sahina/cvt/pkg/cvt"
	"github.com/spf13/cobra"
)

func compareCmd() *cobra.Command {
	var (
		oldSchemaFile string
		newSchemaFile string
		outputJSON    bool
	)

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare two OpenAPI schemas for breaking changes",
		Long: `Compare two versions of an OpenAPI schema and detect breaking changes.

Breaking changes include:
- Removed endpoints
- Added required parameters
- Added required fields in request body
- Changed field types incompatibly
- Removed enum values
- Changed response schema incompatibly

Examples:
  # Compare two schema files
  cvt compare --old ./v1/openapi.json --new ./v2/openapi.json

  # Output as JSON
  cvt compare --old ./v1.json --new ./v2.json --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if oldSchemaFile == "" || newSchemaFile == "" {
				return fmt.Errorf("both --old and --new are required")
			}

			// Load schemas
			v := cvt.NewValidator()
			v.SetHooks(pluginHooks)

			if err := v.RegisterSchemaFromPath("old", oldSchemaFile); err != nil {
				return fmt.Errorf("failed to load old schema: %w", err)
			}

			if err := v.RegisterSchemaFromPath("new", newSchemaFile); err != nil {
				return fmt.Errorf("failed to load new schema: %w", err)
			}

			oldDoc, _ := v.GetSchema("old")
			newDoc, _ := v.GetSchema("new")

			// Compare schemas
			engine := cvt.NewCompatibilityEngine()
			changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

			// Build result
			result := struct {
				Compatible      bool                 `json:"compatible"`
				BreakingChanges []cvt.BreakingChange `json:"breaking_changes,omitempty"`
			}{
				Compatible: compatible,
			}

			for _, c := range changes {
				result.BreakingChanges = append(result.BreakingChanges, cvt.BreakingChange{
					Type:        c.Type,
					Path:        c.Path,
					Method:      c.Method,
					Description: c.Description,
					OldValue:    c.OldValue,
					NewValue:    c.NewValue,
				})
			}

			// Output result
			if outputJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(result); err != nil {
					return fmt.Errorf("failed to encode result: %w", err)
				}
			} else {
				if compatible {
					fmt.Println("✓ Schemas are compatible - no breaking changes detected")
				} else {
					fmt.Printf("✗ Found %d breaking change(s):\n\n", len(changes))
					for _, c := range changes {
						fmt.Printf("  [%s] %s\n", c.Type, c.Description)
						if c.OldValue != "" || c.NewValue != "" {
							if c.OldValue != "" {
								fmt.Printf("    Old: %s\n", c.OldValue)
							}
							if c.NewValue != "" {
								fmt.Printf("    New: %s\n", c.NewValue)
							}
						}
						fmt.Println()
					}
					// Return through cobra so main runs runPluginShutdown
					// before exiting; os.Exit here would orphan plugin
					// subprocesses on every failed CI breaking-change check.
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
					return errExit{code: 1}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&oldSchemaFile, "old", "", "Path to old OpenAPI schema file (required)")
	cmd.Flags().StringVar(&newSchemaFile, "new", "", "Path to new OpenAPI schema file (required)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output result as JSON")

	return cmd
}
