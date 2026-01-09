package cvtservice

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealthService(t *testing.T) {
	t.Run("NewHealthService", func(t *testing.T) {
		h := NewHealthService()
		assert.NotNil(t, h)
		assert.NotNil(t, h.status)
	})

	t.Run("SetServingStatus", func(t *testing.T) {
		h := NewHealthService()
		service := "test.service"

		// Initially not set
		resp, err := h.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: service})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)

		// Set to NOT_SERVING
		h.SetServingStatus(service, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		resp, err = h.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: service})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp.Status)
	})

	t.Run("SetAllServingStatus", func(t *testing.T) {
		h := NewHealthService()

		h.SetAllServingStatus(grpc_health_v1.HealthCheckResponse_NOT_SERVING)

		// Check overall status
		resp, err := h.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: ""})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp.Status)

		// Check CVT service status
		resp, err = h.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: "cvt.ContractValidator"})
		require.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_NOT_SERVING, resp.Status)
	})

	t.Run("Watch", func(t *testing.T) {
		h := NewHealthService()
		h.SetServingStatus("test.service", grpc_health_v1.HealthCheckResponse_SERVING)

		// Create a mock stream
		mockStream := &mockHealthWatchServer{
			ctx:    context.Background(),
			sendCh: make(chan *grpc_health_v1.HealthCheckResponse, 1),
		}

		// Run Watch in a goroutine
		go func() {
			_ = h.Watch(&grpc_health_v1.HealthCheckRequest{Service: "test.service"}, mockStream)
		}()

		// Wait for initial status
		select {
		case resp := <-mockStream.sendCh:
			assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for Watch response")
		}
	})
}

// Mock grpc_health_v1.Health_WatchServer
type mockHealthWatchServer struct {
	grpc.ServerStream
	ctx    context.Context
	sendCh chan *grpc_health_v1.HealthCheckResponse
}

func (m *mockHealthWatchServer) Send(resp *grpc_health_v1.HealthCheckResponse) error {
	m.sendCh <- resp
	return nil
}

func (m *mockHealthWatchServer) Context() context.Context {
	return m.ctx
}
