package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func waitCmd() *cobra.Command {
	var (
		serverAddr string
		timeout    int
		interval   int
		quiet      bool
	)

	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for CVT server to be ready",
		Long: `Wait for the CVT gRPC server to be ready to accept connections.

This command polls the server's health check endpoint until it responds
or the timeout is reached. Useful in CI/CD pipelines to ensure the server
is ready before running other commands.

Examples:
  # Wait with default settings (60s timeout, 2s interval)
  cvt wait

  # Wait for a specific server with custom timeout
  cvt wait --server localhost:9550 --timeout 120

  # Wait with a shorter polling interval
  cvt wait --interval 1

  # Quiet mode for CI/CD
  cvt wait -q`,
		RunE: func(cmd *cobra.Command, args []string) error {
			timeoutDuration := time.Duration(timeout) * time.Second
			intervalDuration := time.Duration(interval) * time.Second

			if !quiet {
				fmt.Printf("Waiting for CVT server at %s (timeout: %ds)...\n", serverAddr, timeout)
			}

			deadline := time.Now().Add(timeoutDuration)
			attempt := 0

			for time.Now().Before(deadline) {
				attempt++

				if err := checkHealth(serverAddr); err == nil {
					if !quiet {
						fmt.Printf("✓ CVT server is ready (after %d attempts)\n", attempt)
					}
					return nil
				}

				if !quiet {
					remaining := time.Until(deadline).Truncate(time.Second)
					fmt.Printf("  Attempt %d: server not ready, retrying in %ds (%s remaining)...\n",
						attempt, interval, remaining)
				}

				time.Sleep(intervalDuration)
			}

			return fmt.Errorf("timeout: CVT server at %s did not become ready within %ds", serverAddr, timeout)
		},
	}

	cmd.Flags().StringVarP(&serverAddr, "server", "S", "localhost:9550", "CVT server address")
	cmd.Flags().IntVarP(&timeout, "timeout", "t", 60, "Maximum time to wait in seconds")
	cmd.Flags().IntVarP(&interval, "interval", "i", 2, "Polling interval in seconds")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress output except errors")

	return cmd
}

func checkHealth(serverAddr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("server status: %s", resp.Status.String())
	}

	return nil
}
