package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryAuthInterceptor(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add(APIKeyInfo{Key: "valid-key", Name: "test-service"})
	interceptor := UnaryAuthInterceptor(store)

	t.Run("skips auth for health checks", func(t *testing.T) {
		ctx := context.Background()
		info := &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}
		handlerCalled := false
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			handlerCalled = true
			return "ok", nil
		}

		result, err := interceptor(ctx, nil, info, handler)
		assert.NoError(t, err)
		assert.True(t, handlerCalled)
		assert.Equal(t, "ok", result)
	})

	t.Run("authenticates valid API key", func(t *testing.T) {
		md := metadata.New(map[string]string{APIKeyMetadataKey: "valid-key"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		info := &grpc.UnaryServerInfo{FullMethod: "/cvt.ContractValidator/ValidateInteraction"}
		handlerCalled := false
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			handlerCalled = true
			// Verify key info is in context
			keyInfo := GetAPIKeyInfo(ctx)
			assert.NotNil(t, keyInfo)
			assert.Equal(t, "test-service", keyInfo.Name)
			return "ok", nil
		}

		result, err := interceptor(ctx, nil, info, handler)
		assert.NoError(t, err)
		assert.True(t, handlerCalled)
		assert.Equal(t, "ok", result)
	})

	t.Run("rejects missing metadata", func(t *testing.T) {
		ctx := context.Background()
		info := &grpc.UnaryServerInfo{FullMethod: "/cvt.ContractValidator/ValidateInteraction"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			t.Fatal("handler should not be called")
			return nil, nil
		}

		_, err := interceptor(ctx, nil, info, handler)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "missing metadata")
	})

	t.Run("rejects missing API key", func(t *testing.T) {
		md := metadata.New(map[string]string{})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		info := &grpc.UnaryServerInfo{FullMethod: "/cvt.ContractValidator/ValidateInteraction"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			t.Fatal("handler should not be called")
			return nil, nil
		}

		_, err := interceptor(ctx, nil, info, handler)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "missing API key")
	})

	t.Run("rejects invalid API key", func(t *testing.T) {
		md := metadata.New(map[string]string{APIKeyMetadataKey: "invalid-key"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		info := &grpc.UnaryServerInfo{FullMethod: "/cvt.ContractValidator/ValidateInteraction"}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			t.Fatal("handler should not be called")
			return nil, nil
		}

		_, err := interceptor(ctx, nil, info, handler)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "invalid API key")
	})
}

// mockServerStream implements grpc.ServerStream for testing
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestStreamAuthInterceptor(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add(APIKeyInfo{Key: "valid-key", Name: "test-service"})
	interceptor := StreamAuthInterceptor(store)

	t.Run("skips auth for health checks", func(t *testing.T) {
		ctx := context.Background()
		stream := &mockServerStream{ctx: ctx}
		info := &grpc.StreamServerInfo{FullMethod: "/grpc.health.v1.Health/Watch"}
		handlerCalled := false
		handler := func(srv interface{}, stream grpc.ServerStream) error {
			handlerCalled = true
			return nil
		}

		err := interceptor(nil, stream, info, handler)
		assert.NoError(t, err)
		assert.True(t, handlerCalled)
	})

	t.Run("authenticates valid API key", func(t *testing.T) {
		md := metadata.New(map[string]string{APIKeyMetadataKey: "valid-key"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		stream := &mockServerStream{ctx: ctx}
		info := &grpc.StreamServerInfo{FullMethod: "/cvt.ContractValidator/StreamValidation"}
		handlerCalled := false
		handler := func(srv interface{}, stream grpc.ServerStream) error {
			handlerCalled = true
			return nil
		}

		err := interceptor(nil, stream, info, handler)
		assert.NoError(t, err)
		assert.True(t, handlerCalled)
	})

	t.Run("rejects missing metadata", func(t *testing.T) {
		ctx := context.Background()
		stream := &mockServerStream{ctx: ctx}
		info := &grpc.StreamServerInfo{FullMethod: "/cvt.ContractValidator/StreamValidation"}
		handler := func(srv interface{}, stream grpc.ServerStream) error {
			t.Fatal("handler should not be called")
			return nil
		}

		err := interceptor(nil, stream, info, handler)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("rejects invalid API key", func(t *testing.T) {
		md := metadata.New(map[string]string{APIKeyMetadataKey: "invalid-key"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		stream := &mockServerStream{ctx: ctx}
		info := &grpc.StreamServerInfo{FullMethod: "/cvt.ContractValidator/StreamValidation"}
		handler := func(srv interface{}, stream grpc.ServerStream) error {
			t.Fatal("handler should not be called")
			return nil
		}

		err := interceptor(nil, stream, info, handler)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})
}

func TestValidateAPIKey(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add(APIKeyInfo{Key: "valid-key", Name: "test-service"})

	t.Run("valid key returns info", func(t *testing.T) {
		md := metadata.New(map[string]string{APIKeyMetadataKey: "valid-key"})
		ctx := metadata.NewIncomingContext(context.Background(), md)

		info, err := validateAPIKey(ctx, store)
		assert.NoError(t, err)
		assert.Equal(t, "test-service", info.Name)
		assert.Equal(t, "valid-key", info.Key)
	})

	t.Run("missing metadata returns error", func(t *testing.T) {
		ctx := context.Background()

		_, err := validateAPIKey(ctx, store)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "missing metadata")
	})

	t.Run("empty API key returns error", func(t *testing.T) {
		md := metadata.New(map[string]string{})
		ctx := metadata.NewIncomingContext(context.Background(), md)

		_, err := validateAPIKey(ctx, store)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "missing API key")
	})

	t.Run("invalid key returns error", func(t *testing.T) {
		md := metadata.New(map[string]string{APIKeyMetadataKey: "wrong-key"})
		ctx := metadata.NewIncomingContext(context.Background(), md)

		_, err := validateAPIKey(ctx, store)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "invalid API key")
	})
}

func TestGetAPIKeyInfo(t *testing.T) {
	t.Run("returns nil when no key info in context", func(t *testing.T) {
		ctx := context.Background()
		info := GetAPIKeyInfo(ctx)
		assert.Nil(t, info)
	})

	t.Run("returns key info from context", func(t *testing.T) {
		keyInfo := APIKeyInfo{Key: "test-key", Name: "test-service"}
		ctx := context.WithValue(context.Background(), apiKeyContextKey{}, keyInfo)

		info := GetAPIKeyInfo(ctx)
		require.NotNil(t, info)
		assert.Equal(t, "test-key", info.Key)
		assert.Equal(t, "test-service", info.Name)
	})

	t.Run("returns nil when wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), apiKeyContextKey{}, "wrong-type")
		info := GetAPIKeyInfo(ctx)
		assert.Nil(t, info)
	})
}
