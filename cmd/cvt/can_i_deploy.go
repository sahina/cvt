package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	pb "github.com/sahina/cvt/server/pb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var errUnsafeToDeploy = errors.New("unsafe to deploy: breaking changes would affect registered consumers")

func canIDeployCmd() *cobra.Command {
	var (
		schemaID    string
		newVersion  string
		environment string
		serverAddr  string
		outputJSON  bool
		timeout     int
	)

	cmd := &cobra.Command{
		Use:   "can-i-deploy",
		Short: "Check if a schema version can be safely deployed",
		Long: `Check if a schema version can be safely deployed without breaking registered consumers.

This command queries the CVT server to determine if deploying a new schema version
will cause breaking changes for any consumers registered in the target environment.

The command will:
1. Detect breaking changes between the current and new schema versions
2. Check if any registered consumers use the affected endpoints/fields
3. Report which consumers will be impacted

Examples:
  # Check if version 2.0.0 can be deployed to production
  cvt can-i-deploy --schema my-api --version 2.0.0 --env prod

  # Check against a specific server
  cvt can-i-deploy --schema user-api --version 1.1.0 --env staging --server localhost:9550

  # Output as JSON for CI/CD integration
  cvt can-i-deploy --schema my-api --version 2.0.0 --env prod --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if schemaID == "" {
				return fmt.Errorf("--schema is required")
			}
			if newVersion == "" {
				return fmt.Errorf("--version is required")
			}
			if environment == "" {
				return fmt.Errorf("--env is required")
			}

			// Connect to the server
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

			// Call CanIDeploy
			req := &pb.CanIDeployRequest{
				SchemaId:    schemaID,
				NewVersion:  newVersion,
				Environment: environment,
			}

			resp, err := client.CanIDeploy(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to check deployment safety: %w", err)
			}

			// Output result
			if outputJSON {
				return outputJSONResult(resp)
			}
			return outputHumanResult(resp, schemaID, newVersion, environment)
		},
	}

	cmd.Flags().StringVar(&schemaID, "schema", "", "Schema ID to check (required)")
	cmd.Flags().StringVar(&newVersion, "version", "", "New version to deploy (required)")
	cmd.Flags().StringVar(&environment, "env", "", "Target environment: dev, staging, prod (required)")
	cmd.Flags().StringVar(&serverAddr, "server", "localhost:9550", "CVT server address")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output result as JSON")
	cmd.Flags().IntVar(&timeout, "timeout", 30, "Connection timeout in seconds")

	return cmd
}

func outputJSONResult(resp *pb.CanIDeployResponse) error {
	result := struct {
		SafeToDeploy      bool                   `json:"safe_to_deploy"`
		Summary           string                 `json:"summary"`
		BreakingChanges   []breakingChangeOutput `json:"breaking_changes,omitempty"`
		AffectedConsumers []consumerImpactOutput `json:"affected_consumers,omitempty"`
	}{
		SafeToDeploy: resp.SafeToDeploy,
		Summary:      resp.Summary,
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

	for _, c := range resp.AffectedConsumers {
		if c == nil {
			continue
		}
		impact := consumerImpactOutput{
			ConsumerID:           c.ConsumerId,
			ConsumerVersion:      c.ConsumerVersion,
			CurrentSchemaVersion: c.CurrentSchemaVersion,
			Environment:          c.Environment,
			WillBreak:            c.WillBreak,
		}

		for _, bc := range c.RelevantChanges {
			if bc == nil {
				continue
			}
			impact.RelevantChanges = append(impact.RelevantChanges, breakingChangeOutput{
				Type:        bc.Type.String(),
				Path:        bc.Path,
				Method:      bc.Method,
				Description: bc.Description,
				OldValue:    bc.OldValue,
				NewValue:    bc.NewValue,
			})
		}

		result.AffectedConsumers = append(result.AffectedConsumers, impact)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("failed to encode result: %w", err)
	}

	if !resp.SafeToDeploy {
		return errUnsafeToDeploy
	}
	return nil
}

func outputHumanResult(resp *pb.CanIDeployResponse, schemaID, newVersion, environment string) error {
	fmt.Printf("\nDeployment Safety Check\n")
	fmt.Printf("=======================\n")
	fmt.Printf("Schema:      %s\n", schemaID)
	fmt.Printf("Version:     %s\n", newVersion)
	fmt.Printf("Environment: %s\n\n", environment)

	if resp.SafeToDeploy {
		fmt.Println("\u2705 SAFE TO DEPLOY")
		fmt.Println()
		if resp.Summary != "" {
			fmt.Printf("%s\n", resp.Summary)
		} else {
			fmt.Println("No breaking changes detected that would affect registered consumers.")
		}
		return nil
	}

	// Unsafe to deploy
	fmt.Println("\u274C UNSAFE TO DEPLOY")
	fmt.Println()

	// Show breaking changes
	if len(resp.BreakingChanges) > 0 {
		fmt.Printf("Breaking changes in v%s:\n", newVersion)
		for _, bc := range resp.BreakingChanges {
			if bc == nil {
				continue
			}
			fmt.Printf("  - %s: %s %s\n", bc.Type.String(), bc.Method, bc.Path)
			fmt.Printf("    %s\n", bc.Description)
		}
		fmt.Println()
	}

	// Show affected consumers
	if len(resp.AffectedConsumers) > 0 {
		fmt.Printf("Affected consumers in %s:\n", environment)

		safeCount := 0
		affectedCount := 0

		for i, c := range resp.AffectedConsumers {
			if c == nil {
				continue
			}
			if c.WillBreak {
				affectedCount++
			} else {
				safeCount++
			}

			// Tree-like structure
			isLast := i == len(resp.AffectedConsumers)-1
			prefix := "\u251C\u2500\u2500"
			continuation := "\u2502"
			if isLast {
				prefix = "\u2514\u2500\u2500"
				continuation = " "
			}

			impactStatus := "None"
			if c.WillBreak {
				impactStatus = "BREAKING"
			}

			fmt.Printf("  %s %s v%s\n", prefix, c.ConsumerId, c.ConsumerVersion)
			fmt.Printf("  %s   Schema version: %s\n", continuation, c.CurrentSchemaVersion)
			fmt.Printf("  %s   Impact: %s\n", continuation, impactStatus)

			if c.WillBreak && len(c.RelevantChanges) > 0 {
				fmt.Printf("  %s   Affected by:\n", continuation)
				for _, bc := range c.RelevantChanges {
					if bc == nil {
						continue
					}
					fmt.Printf("  %s     - %s %s\n", continuation, bc.Method, bc.Path)
				}
			}
			fmt.Printf("  %s\n", continuation)
		}

		fmt.Println()
		fmt.Printf("Safe consumers:     %d/%d\n", safeCount, len(resp.AffectedConsumers))
		fmt.Printf("Affected consumers: %d/%d\n", affectedCount, len(resp.AffectedConsumers))
	}

	if resp.Summary != "" {
		fmt.Printf("\nRecommendation: %s\n", resp.Summary)
	}

	return errUnsafeToDeploy
}
