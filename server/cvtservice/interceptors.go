// Package cvtservice provides gRPC interceptors for the CVT server.
package cvtservice

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Context key for storing API key info.
type apiKeyContextKey struct{}

// Prometheus metrics for authentication.
var (
	authSuccesses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_auth_successes_total",
			Help: "Total number of successful authentications by API key name",
		},
		[]string{"key_name"},
	)

	authFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_auth_failures_total",
			Help: "Total number of authentication failures by reason",
		},
		[]string{"reason"},
	)
)

// UnaryAuthInterceptor creates a unary interceptor for API key validation.
func UnaryAuthInterceptor(store *APIKeyStore) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth for health checks
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}

		// Extract and validate API key
		keyInfo, err := validateAPIKey(ctx, store)
		if err != nil {
			return nil, err
		}

		// Add key info to context
		ctx = context.WithValue(ctx, apiKeyContextKey{}, keyInfo)

		// Record success metric
		authSuccesses.WithLabelValues(keyInfo.Name).Inc()

		Debug("API key authenticated",
			zap.String("method", info.FullMethod),
			zap.String("keyName", keyInfo.Name))

		return handler(ctx, req)
	}
}

// StreamAuthInterceptor creates a stream interceptor for API key validation.
func StreamAuthInterceptor(store *APIKeyStore) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Skip auth for health checks
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(srv, ss)
		}

		// Extract and validate API key
		keyInfo, err := validateAPIKey(ss.Context(), store)
		if err != nil {
			return err
		}

		// Record success metric
		authSuccesses.WithLabelValues(keyInfo.Name).Inc()

		Debug("API key authenticated (stream)",
			zap.String("method", info.FullMethod),
			zap.String("keyName", keyInfo.Name))

		return handler(srv, ss)
	}
}

// validateAPIKey extracts and validates the API key from the request context.
func validateAPIKey(ctx context.Context, store *APIKeyStore) (APIKeyInfo, error) {
	// Extract metadata from context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		authFailures.WithLabelValues("missing_metadata").Inc()
		return APIKeyInfo{}, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get API key from metadata
	apiKeys := md.Get(APIKeyMetadataKey)
	if len(apiKeys) == 0 {
		authFailures.WithLabelValues("missing_api_key").Inc()
		return APIKeyInfo{}, status.Error(codes.Unauthenticated, "missing API key")
	}

	// Validate the API key
	keyInfo, valid := store.Validate(apiKeys[0])
	if !valid {
		authFailures.WithLabelValues("invalid_api_key").Inc()
		return APIKeyInfo{}, status.Error(codes.Unauthenticated, "invalid API key")
	}

	return keyInfo, nil
}

// GetAPIKeyInfo retrieves the API key info from the context.
// Returns nil if no API key info is present.
func GetAPIKeyInfo(ctx context.Context) *APIKeyInfo {
	info, ok := ctx.Value(apiKeyContextKey{}).(APIKeyInfo)
	if !ok {
		return nil
	}
	return &info
}
