package main

import (
	"fmt"

	"github.com/sahina/cvt/pkg/mock"
	"github.com/spf13/cobra"
)

func mockCmd() *cobra.Command {
	var (
		schemas          []string
		port             int
		host             string
		watch            bool
		validateRequests bool
		noExamples       bool
		latency          int
		quiet            bool
		seed             uint64
		seedSet          bool
	)

	cmd := &cobra.Command{
		Use:   "mock",
		Short: "Start a mock HTTP server from OpenAPI schemas",
		Long: `Start an HTTP server that generates mock responses from OpenAPI schemas.

The mock server matches incoming HTTP requests to schema operations and
returns generated responses. Supports multiple schemas, request validation,
hot-reload, and CORS headers for frontend development.

Examples:
  # Basic mock server
  cvt mock --schema ./openapi.json

  # Custom port and multiple schemas
  cvt mock --schema users.json --schema orders.json --port 3000

  # With request validation and hot-reload
  cvt mock --schema ./openapi.json --validate-requests --watch

  # Mock from a URL
  cvt mock --schema https://petstore3.swagger.io/api/v3/openapi.json

  # Simulate network latency
  cvt mock --schema ./openapi.json --latency 200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(schemas) == 0 {
				return fmt.Errorf("at least one --schema is required")
			}

			if port < 0 || port > 65535 {
				return fmt.Errorf("invalid port: %d", port)
			}

			if latency < 0 {
				return fmt.Errorf("--latency must be >= 0")
			}

			cfg := mock.ServerConfig{
				Host:             host,
				Port:             port,
				SchemaFiles:      schemas,
				Watch:            watch,
				ValidateRequests: validateRequests,
				UseExamples:      !noExamples,
				LatencyMs:        latency,
				Quiet:            quiet,
			}
			if seedSet {
				cfg.Seed = &seed
			}

			srv := mock.NewServer(cfg)

			return srv.Start()
		},
	}

	cmd.Flags().StringArrayVar(&schemas, "schema", nil, "OpenAPI schema file path or URL (can be specified multiple times)")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "HTTP server port")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Bind address (default: localhost only)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch schema files for changes and hot-reload")
	cmd.Flags().BoolVar(&validateRequests, "validate-requests", false, "Validate incoming requests against the schema")
	cmd.Flags().BoolVar(&noExamples, "no-examples", false, "Use generated values instead of schema examples")
	cmd.Flags().IntVar(&latency, "latency", 0, "Artificial response delay in milliseconds")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress request logging")
	cmd.Flags().Uint64Var(&seed, "seed", 0, "Random seed for deterministic responses")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		seedSet = cmd.Flags().Changed("seed")
		return nil
	}

	return cmd
}
