// Package main provides the CVT command-line interface.
// This CLI allows for local contract validation without requiring a gRPC server.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information (set at build time)
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cvt",
		Short: "Contract Validation Tool - Validate HTTP interactions against OpenAPI specs",
		Long: `CVT (Contract Validation Tool) validates HTTP request/response interactions
against OpenAPI specifications. It can be used locally without a server,
or you can run a gRPC server for remote validation.

Examples:
  # Validate a single interaction locally
  cvt validate --schema ./openapi.json --request req.json --response resp.json

  # Compare two schema versions for breaking changes
  cvt compare --old ./v1/openapi.json --new ./v2/openapi.json

  # Check if a schema version can be safely deployed
  cvt can-i-deploy --schema my-api --version 2.0.0 --env prod

  # Start the gRPC validation server
  cvt serve --port 9550`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildDate),
	}

	// Add subcommands
	rootCmd.AddCommand(validateCmd())
	rootCmd.AddCommand(compareCmd())
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(generateCmd())
	rootCmd.AddCommand(mockCmd())
	rootCmd.AddCommand(canIDeployCmd())
	rootCmd.AddCommand(waitCmd())
	rootCmd.AddCommand(registerSchemaCmd())
	rootCmd.AddCommand(pluginsCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("CVT version %s\n", version)
			fmt.Printf("  Commit:  %s\n", commit)
			fmt.Printf("  Built:   %s\n", buildDate)
		},
	}
}
