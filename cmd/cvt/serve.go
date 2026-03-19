package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sahina/cvt/server/cvtservice"
	"github.com/sahina/cvt/server/pb"
	"github.com/sahina/cvt/server/storage"
	_ "github.com/sahina/cvt/server/storage/postgres"
	_ "github.com/sahina/cvt/server/storage/sqlite"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
			// Initialize structured logger (development mode if LOG_LEVEL=debug)
			development := os.Getenv("LOG_LEVEL") == "debug"
			if err := cvtservice.InitLogger(development); err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer func() { _ = cvtservice.Sync() }()

			// Resolve ports: CLI flags take precedence over env vars
			if !cmd.Flags().Changed("port") {
				if envPort := os.Getenv("CVT_PORT"); envPort != "" {
					if p, err := strconv.Atoi(envPort); err == nil {
						port = p
					} else {
						cvtservice.Warn("Invalid CVT_PORT value, using default",
							zap.String("value", envPort), zap.Int("default", port))
					}
				}
			}
			if !cmd.Flags().Changed("metrics-port") {
				if envMetrics := os.Getenv("CVT_METRICS_PORT"); envMetrics != "" {
					if p, err := strconv.Atoi(envMetrics); err == nil {
						metricsPort = p
					} else {
						cvtservice.Warn("Invalid CVT_METRICS_PORT value, using default",
							zap.String("value", envMetrics), zap.Int("default", metricsPort))
					}
				}
			}

			cvtservice.Info("Starting CVT Server",
				zap.Int("grpc_port", port),
				zap.Int("metrics_port", metricsPort))

			// Build gRPC server options
			var serverOpts []grpc.ServerOption

			// Configure TLS: CLI flag takes precedence, then check env vars
			if tlsEnabled {
				// TLS enabled via CLI flag — use --cert/--key values
				if tlsCert == "" || tlsKey == "" {
					return fmt.Errorf("TLS enabled but --cert and --key are required")
				}
				tlsConfig := &cvtservice.TLSConfig{
					Enabled:  true,
					CertFile: tlsCert,
					KeyFile:  tlsKey,
				}
				creds, err := cvtservice.LoadTLSCredentials(tlsConfig)
				if err != nil {
					return fmt.Errorf("failed to load TLS credentials: %w", err)
				}
				serverOpts = append(serverOpts, grpc.Creds(creds))
				cvtservice.Info("TLS enabled (CLI flags)",
					zap.String("certFile", tlsCert),
					zap.String("keyFile", tlsKey))
			} else {
				// Check env vars for TLS configuration
				tlsConfig, err := cvtservice.LoadTLSConfigFromEnv()
				if err != nil {
					return fmt.Errorf("failed to load TLS config: %w", err)
				}
				if tlsConfig.Enabled {
					creds, err := cvtservice.LoadTLSCredentials(tlsConfig)
					if err != nil {
						return fmt.Errorf("failed to load TLS credentials: %w", err)
					}
					serverOpts = append(serverOpts, grpc.Creds(creds))
					cvtservice.Info("TLS enabled (env vars)",
						zap.String("certFile", tlsConfig.CertFile),
						zap.String("keyFile", tlsConfig.KeyFile))
				} else {
					cvtservice.Warn("TLS disabled - using insecure connection")
				}
			}

			// Configure API key authentication: CLI flag OR env var
			if !apiKeyAuth {
				// Check env var fallback
				apiKeyAuth = os.Getenv("CVT_API_KEY_ENABLED") == "true"
			}

			if apiKeyAuth {
				authConfig, err := cvtservice.LoadAuthConfigFromEnv()
				if err != nil {
					return fmt.Errorf("failed to load auth config: %w", err)
				}
				// Force enabled since we determined it should be on
				authConfig.Enabled = true

				apiKeyStore, err := cvtservice.LoadAPIKeys(authConfig)
				if err != nil {
					return fmt.Errorf("failed to load API keys: %w", err)
				}
				serverOpts = append(serverOpts,
					grpc.ChainUnaryInterceptor(cvtservice.UnaryAuthInterceptor(apiKeyStore)),
					grpc.ChainStreamInterceptor(cvtservice.StreamAuthInterceptor(apiKeyStore)),
				)
				cvtservice.Info("API key authentication enabled", zap.Int("keyCount", apiKeyStore.Count()))
			} else {
				cvtservice.Warn("API key authentication disabled")
			}

			// Create gRPC server with configured options
			grpcServer := grpc.NewServer(serverOpts...)

			// Load storage configuration and create the validator service
			storageCfg := storage.LoadConfigFromEnv()
			var validatorService *cvtservice.ValidatorService
			var err error

			if storageCfg.IsEnabled() {
				store, storeErr := storage.NewStore(context.Background(), storageCfg)
				if storeErr != nil {
					return fmt.Errorf("failed to create storage: %w", storeErr)
				}
				if migrateErr := store.Migrate(context.Background()); migrateErr != nil {
					_ = store.Close()
					return fmt.Errorf("failed to migrate storage: %w", migrateErr)
				}
				cvtservice.Info("Storage enabled",
					zap.String("type", string(storageCfg.Type)))

				validatorService, err = cvtservice.NewValidatorServiceWithStore(store)
				if err != nil {
					_ = store.Close()
					return fmt.Errorf("failed to create validator service: %w", err)
				}
			} else {
				cvtservice.Info("Storage disabled — using in-memory cache only")
				validatorService, err = cvtservice.NewValidatorService()
				if err != nil {
					return fmt.Errorf("failed to create validator service: %w", err)
				}
			}
			defer validatorService.Close()

			pb.RegisterContractValidatorServer(grpcServer, validatorService)
			cvtservice.Info("Registered ValidatorService")

			// Create and register the health check service
			healthService := cvtservice.NewHealthService()
			healthService.SetAllServingStatus(grpc_health_v1.HealthCheckResponse_SERVING)
			grpc_health_v1.RegisterHealthServer(grpcServer, healthService)
			cvtservice.Info("Registered HealthService")

			// Register reflection service for debugging
			reflection.Register(grpcServer)
			cvtservice.Info("Registered ProtoReflectionService")

			// Start Prometheus metrics HTTP server
			metricsServer := &http.Server{
				Addr:         fmt.Sprintf(":%d", metricsPort),
				Handler:      promhttp.Handler(),
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  60 * time.Second,
			}
			go func() {
				cvtservice.Info("Metrics server listening", zap.String("address", metricsServer.Addr))
				if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					cvtservice.Error("Metrics server failed", zap.Error(err))
				}
			}()

			// Create TCP listener
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return fmt.Errorf("failed to listen: %w", err)
			}

			// Set up graceful shutdown handling
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

			// Start gRPC server in a goroutine with error channel
			errCh := make(chan error, 1)
			go func() {
				cvtservice.Info("Server listening", zap.String("address", listener.Addr().String()))
				if err := grpcServer.Serve(listener); err != nil {
					errCh <- err
				}
			}()

			// Wait for shutdown signal or server error
			select {
			case sig := <-sigCh:
				cvtservice.Info("Received shutdown signal", zap.String("signal", sig.String()))
			case err := <-errCh:
				cvtservice.Error("Server failed", zap.Error(err))
				return fmt.Errorf("gRPC server failed: %w", err)
			}

			// Set health status to NOT_SERVING before stopping
			healthService.SetAllServingStatus(grpc_health_v1.HealthCheckResponse_NOT_SERVING)
			cvtservice.Info("Health status set to NOT_SERVING")

			// Shutdown metrics server gracefully
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := metricsServer.Shutdown(shutdownCtx); err != nil {
				cvtservice.Error("Failed to shutdown metrics server", zap.Error(err))
			} else {
				cvtservice.Info("Metrics server stopped")
			}

			// Perform graceful shutdown
			cvtservice.Info("Shutting down server gracefully...")
			grpcServer.GracefulStop()
			cvtservice.Info("Server stopped")

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
