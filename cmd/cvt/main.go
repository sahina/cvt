// Package main provides the CVT command-line interface.
// This CLI allows for local contract validation without requiring a gRPC server.
package main

import (
	"errors"
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

// errExit lets a subcommand request a specific non-zero exit code without
// triggering cobra's "Error: ..." print AND without bypassing
// runPluginShutdown via os.Exit. RunE returns errExit{code: 1} from the
// validation-failure / breaking-change paths; main inspects it after
// runPluginShutdown and exits with the encoded code.
type errExit struct{ code int }

func (e errExit) Error() string { return fmt.Sprintf("exit %d", e.code) }

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

	installPluginBootstrap(rootCmd)

	err := rootCmd.Execute()
	runPluginShutdown()
	if err != nil {
		// Subcommands signaling a specific exit code go through errExit
		// — they've already printed user-facing output, so we just exit
		// with the encoded code (no "Error: ..." prefix).
		var ee errExit
		if errors.As(err, &ee) {
			os.Exit(ee.code)
		}
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
