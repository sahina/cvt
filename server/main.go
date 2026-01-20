// Package main provides the entry point for the Contract Validation Tool (CVT) server.
// The server implements a gRPC service that validates HTTP request/response interactions
// against OpenAPI specifications.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sahina/cvt/server/cvtservice"
	"github.com/sahina/cvt/server/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	// DefaultPort is the default port the server listens on if CVT_PORT is not set
	DefaultPort = "9550"

	// DefaultMetricsPort is the default port for Prometheus metrics if CVT_METRICS_PORT is not set
	DefaultMetricsPort = "9551"
)

// main is the entry point for the CVT server application.
// It performs the following operations:
// 1. Initializes the logger based on LOG_LEVEL environment variable
// 2. Configures the server port from CVT_PORT environment variable
// 3. Creates a TCP listener for gRPC connections
// 4. Initializes and registers the validator service
// 5. Initializes and registers the health check service
// 6. Registers the gRPC reflection service for debugging
// 7. Starts the server and handles graceful shutdown on SIGTERM/SIGINT
func main() {
	// Initialize logger (development mode if LOG_LEVEL=debug)
	// Development mode provides colorized output and more verbose logging
	development := os.Getenv("LOG_LEVEL") == "debug"
	if err := cvtservice.InitLogger(development); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = cvtservice.Sync() }()

	// Get port from environment or use default
	// The CVT_PORT environment variable allows configuring the server port
	port := os.Getenv("CVT_PORT")
	if port == "" {
		port = DefaultPort
	}

	// Get metrics port from environment or use default
	metricsPort := os.Getenv("CVT_METRICS_PORT")
	if metricsPort == "" {
		metricsPort = DefaultMetricsPort
	}

	cvtservice.Info("Starting CVT Server",
		zap.String("grpc_port", port),
		zap.String("metrics_port", metricsPort))

	// Start Prometheus metrics HTTP server
	// This runs concurrently with the gRPC server
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", metricsPort),
		Handler: promhttp.Handler(),
	}

	go func() {
		cvtservice.Info("Metrics server listening", zap.String("address", metricsServer.Addr))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			cvtservice.Error("Metrics server failed", zap.Error(err))
		}
	}()

	// Create TCP listener on the specified port
	// This listener will accept incoming gRPC connections
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		cvtservice.Fatal("Failed to listen", zap.String("port", port), zap.Error(err))
	}

	// Build gRPC server options
	var serverOpts []grpc.ServerOption

	// Load TLS configuration
	tlsConfig, err := cvtservice.LoadTLSConfigFromEnv()
	if err != nil {
		cvtservice.Fatal("Failed to load TLS config", zap.Error(err))
	}

	if tlsConfig.Enabled {
		creds, err := cvtservice.LoadTLSCredentials(tlsConfig)
		if err != nil {
			cvtservice.Fatal("Failed to load TLS credentials", zap.Error(err))
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
		cvtservice.Info("TLS enabled",
			zap.String("certFile", tlsConfig.CertFile),
			zap.String("keyFile", tlsConfig.KeyFile))
	} else {
		cvtservice.Warn("TLS disabled - using insecure connection")
	}

	// Load authentication configuration
	authConfig, err := cvtservice.LoadAuthConfigFromEnv()
	if err != nil {
		cvtservice.Fatal("Failed to load auth config", zap.Error(err))
	}

	if authConfig.Enabled {
		apiKeyStore, err := cvtservice.LoadAPIKeys(authConfig)
		if err != nil {
			cvtservice.Fatal("Failed to load API keys", zap.Error(err))
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

	// Create and register the validator service
	// This service handles OpenAPI schema registration and HTTP interaction validation
	validatorService, err := cvtservice.NewValidatorService()
	if err != nil {
		cvtservice.Fatal("Failed to create validator service", zap.Error(err))
	}
	defer validatorService.Close()

	pb.RegisterContractValidatorServer(grpcServer, validatorService)
	cvtservice.Info("Registered ValidatorService")

	// Create and register the health check service
	// This implements the standard gRPC health checking protocol
	healthService := cvtservice.NewHealthService()
	healthService.SetAllServingStatus(grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthService)
	cvtservice.Info("Registered HealthService")

	// Register reflection service for debugging
	// This allows tools like grpcurl to introspect the service
	reflection.Register(grpcServer)
	cvtservice.Info("Registered ProtoReflectionService")

	// Set up graceful shutdown handling
	// The server will shut down cleanly when receiving SIGINT or SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine to allow concurrent shutdown signal handling
	// The server will block on grpcServer.Serve() until shutdown
	go func() {
		cvtservice.Info("Server listening", zap.String("address", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil {
			cvtservice.Fatal("Failed to serve", zap.Error(err))
		}
	}()

	// Wait for shutdown signal (blocks until signal is received)
	sig := <-sigChan
	cvtservice.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Set health status to NOT_SERVING to inform clients the server is shutting down
	// This allows load balancers and clients to stop sending new requests
	healthService.SetAllServingStatus(grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	cvtservice.Info("Health status set to NOT_SERVING")

	// Shutdown metrics server
	if err := metricsServer.Close(); err != nil {
		cvtservice.Error("Failed to shutdown metrics server", zap.Error(err))
	} else {
		cvtservice.Info("Metrics server stopped")
	}

	// Perform graceful shutdown
	// GracefulStop waits for all active RPCs to complete before stopping
	cvtservice.Info("Shutting down server gracefully...")
	grpcServer.GracefulStop()
	cvtservice.Info("Server stopped")
}
