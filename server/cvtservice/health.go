// Package cvtservice provides health check functionality for the Contract Validation Tool.
// This implements the standard gRPC Health Checking Protocol (GRPC Health v1).
package cvtservice

import (
	"context"
	"sync"

	"google.golang.org/grpc/health/grpc_health_v1"
)

// HealthService implements the gRPC Health Check Protocol.
// It provides both unary Check and streaming Watch methods for health monitoring.
//
// The service maintains the serving status for multiple services, including:
// - The overall server (empty string key)
// - Individual gRPC services (e.g., "cvt.ContractValidator")
//
// This allows load balancers and monitoring systems to query the server's health status.
type HealthService struct {
	grpc_health_v1.UnimplementedHealthServer
	mu     sync.RWMutex
	status map[string]grpc_health_v1.HealthCheckResponse_ServingStatus
}

// NewHealthService creates a new health service.
// The service is initialized with an empty status map.
// Services must call SetServingStatus or SetAllServingStatus to set their status.
//
// Returns:
//   - *HealthService: A new health service instance
func NewHealthService() *HealthService {
	return &HealthService{
		status: make(map[string]grpc_health_v1.HealthCheckResponse_ServingStatus),
	}
}

// Check performs a unary health check.
// This is the standard health check method that returns the current serving status.
//
// Parameters:
//   - ctx: The request context
//   - req: HealthCheckRequest containing the optional service name
//
// Returns:
//   - HealthCheckResponse: Contains the serving status (SERVING, NOT_SERVING, etc.)
//   - error: Always nil (errors are not returned in this implementation)
//
// If the requested service is not found in the status map, it defaults to SERVING.
func (h *HealthService) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	service := req.GetService()
	status, ok := h.status[service]
	if !ok {
		// If service not found, default to SERVING
		status = grpc_health_v1.HealthCheckResponse_SERVING
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: status,
	}, nil
}

// Watch performs a streaming health check.
// This method sends the current health status and keeps the stream open
// until the client closes the connection or the context is cancelled.
//
// Parameters:
//   - req: HealthCheckRequest containing the optional service name
//   - stream: The bidirectional stream for sending health updates
//
// Returns:
//   - error: An error if sending the status fails or the context is cancelled
//
// Note: This is a simplified implementation that only sends the initial status.
// A production implementation would send updates when the status changes.
func (h *HealthService) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	service := req.GetService()

	h.mu.RLock()
	status, ok := h.status[service]
	if !ok {
		status = grpc_health_v1.HealthCheckResponse_SERVING
	}
	h.mu.RUnlock()

	// Send initial status to the client
	if err := stream.Send(&grpc_health_v1.HealthCheckResponse{
		Status: status,
	}); err != nil {
		return err
	}

	// Keep connection open until the client closes it or context is cancelled
	// In a more advanced implementation, this would send updates when status changes
	<-stream.Context().Done()
	return stream.Context().Err()
}

// SetServingStatus sets the serving status for a specific service.
// This method is thread-safe and can be called concurrently.
//
// Parameters:
//   - service: The service name (use empty string for the overall server)
//   - status: The new serving status (SERVING, NOT_SERVING, etc.)
func (h *HealthService) SetServingStatus(service string, status grpc_health_v1.HealthCheckResponse_ServingStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status[service] = status
}

// SetAllServingStatus sets the serving status for all services.
// This sets the status for:
// - The overall server (empty string key)
// - The ContractValidator service ("cvt.ContractValidator")
//
// This method is typically called during server startup (set to SERVING)
// and shutdown (set to NOT_SERVING).
//
// Parameters:
//   - status: The new serving status for all services
func (h *HealthService) SetAllServingStatus(status grpc_health_v1.HealthCheckResponse_ServingStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Set status for the overall server (empty string)
	h.status[""] = status

	// Set status for the ContractValidator service
	h.status["cvt.ContractValidator"] = status
}
