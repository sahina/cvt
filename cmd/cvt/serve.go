package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/cvt/cvt/server/cvtservice"
	"github.com/cvt/cvt/server/pb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func serveCmd() *cobra.Command {
	var (
		port        int
		metricsPort int
		tlsEnabled  bool
		tlsCert     string
		tlsKey      string
		apiKeyAuth  bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the gRPC validation server",
		Long: `Start the CVT gRPC server for remote contract validation.

The server exposes the ContractValidator gRPC service which provides:
- RegisterSchema: Register OpenAPI schemas for validation
- ValidateInteraction: Validate HTTP request/response pairs

Examples:
  # Start with default settings (port 9550)
  cvt serve

  # Start on a custom port
  cvt serve --port 8080

  # Start with TLS enabled
  cvt serve --tls --cert server.crt --key server.key

  # Start with API key authentication
  cvt serve --api-key-auth`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Starting CVT gRPC server on port %d...\n", port)

			// Create gRPC server options
			var serverOpts []grpc.ServerOption

			// Configure TLS if enabled
			if tlsEnabled {
				if tlsCert == "" || tlsKey == "" {
					return fmt.Errorf("TLS enabled but --cert and --key are required")
				}

				creds, err := credentials.NewServerTLSFromFile(tlsCert, tlsKey)
				if err != nil {
					return fmt.Errorf("failed to load TLS credentials: %w", err)
				}
				serverOpts = append(serverOpts, grpc.Creds(creds))
				fmt.Println("TLS enabled")
			}

			grpcServer := grpc.NewServer(serverOpts...)

			// Create and register the validator service
			validatorService, err := cvtservice.NewValidatorService()
			if err != nil {
				return fmt.Errorf("failed to create validator service: %w", err)
			}
			defer validatorService.Close()

			pb.RegisterContractValidatorServer(grpcServer, validatorService)
			fmt.Println("Registered ContractValidator service")

			// Register health service
			healthService := cvtservice.NewHealthService()
			healthService.SetAllServingStatus(grpc_health_v1.HealthCheckResponse_SERVING)
			grpc_health_v1.RegisterHealthServer(grpcServer, healthService)

			// Enable reflection for debugging
			reflection.Register(grpcServer)

			// Start metrics server
			go func() {
				mux := http.NewServeMux()
				mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("OK"))
				})

				metricsAddr := fmt.Sprintf(":%d", metricsPort)
				fmt.Printf("Metrics server listening on %s\n", metricsAddr)
				if err := http.ListenAndServe(metricsAddr, mux); err != nil {
					fmt.Fprintf(os.Stderr, "Metrics server error: %v\n", err)
				}
			}()

			// Start gRPC server
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return fmt.Errorf("failed to listen: %w", err)
			}

			// Handle graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigCh
				fmt.Println("\nShutting down server...")
				grpcServer.GracefulStop()
			}()

			fmt.Printf("gRPC server listening on :%d\n", port)

			if err := grpcServer.Serve(listener); err != nil {
				return fmt.Errorf("server error: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 9550, "gRPC server port")
	cmd.Flags().IntVar(&metricsPort, "metrics-port", 9551, "Metrics server port")
	cmd.Flags().BoolVar(&tlsEnabled, "tls", false, "Enable TLS")
	cmd.Flags().StringVar(&tlsCert, "cert", "", "TLS certificate file")
	cmd.Flags().StringVar(&tlsKey, "key", "", "TLS private key file")
	cmd.Flags().BoolVar(&apiKeyAuth, "api-key-auth", false, "Enable API key authentication")

	return cmd
}
