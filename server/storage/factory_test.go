package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_Memory(t *testing.T) {
	ctx := context.Background()
	cfg := Config{Type: StoreTypeMemory}

	store, err := NewStore(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Verify it's a MemoryStore
	_, ok := store.(*MemoryStore)
	assert.True(t, ok, "expected MemoryStore type")
}

func TestNewStore_UnregisteredType(t *testing.T) {
	ctx := context.Background()
	cfg := Config{Type: StoreType("unknown")}

	store, err := NewStore(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "unsupported storage type: unknown")
	assert.Contains(t, err.Error(), "did you import the driver")
}

func TestRegister(t *testing.T) {
	// Create a test factory
	testType := StoreType("test_driver")
	var factoryCalled bool

	testFactory := func(ctx context.Context, cfg Config) (Store, error) {
		factoryCalled = true
		return NewMemoryStore(), nil
	}

	// Clean up after test
	defer delete(registry, testType)

	// Register the factory
	Register(testType, testFactory)

	// Verify the factory is registered
	_, ok := registry[testType]
	assert.True(t, ok)

	// Verify it can create a store
	ctx := context.Background()
	cfg := Config{Type: testType}

	store, err := NewStore(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, store)
	assert.True(t, factoryCalled)
}

func TestRegister_FactoryReturnsError(t *testing.T) {
	testType := StoreType("error_driver")
	expectedErr := errors.New("factory error")

	testFactory := func(ctx context.Context, cfg Config) (Store, error) {
		return nil, expectedErr
	}

	defer delete(registry, testType)

	Register(testType, testFactory)

	ctx := context.Background()
	cfg := Config{Type: testType}

	store, err := NewStore(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, store)
	assert.Equal(t, expectedErr, err)
}

func TestMustNewStore_Success(t *testing.T) {
	ctx := context.Background()
	cfg := Config{Type: StoreTypeMemory}

	// Should not panic
	assert.NotPanics(t, func() {
		store := MustNewStore(ctx, cfg)
		assert.NotNil(t, store)
	})
}

func TestMustNewStore_Panic(t *testing.T) {
	ctx := context.Background()
	cfg := Config{Type: StoreType("nonexistent_for_panic_test")}

	// Should panic
	assert.Panics(t, func() {
		MustNewStore(ctx, cfg)
	})
}

func TestMustNewStore_PanicMessage(t *testing.T) {
	ctx := context.Background()
	cfg := Config{Type: StoreType("bad_driver")}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}

		panicMsg, ok := r.(string)
		assert.True(t, ok)
		assert.Contains(t, panicMsg, "failed to create store")
		assert.Contains(t, panicMsg, "unsupported storage type")
	}()

	MustNewStore(ctx, cfg)
}

func TestNewStore_WithRegisteredFactory(t *testing.T) {
	// Test that registered factories receive the correct config
	testType := StoreType("config_test_driver")
	var receivedCfg Config

	testFactory := func(ctx context.Context, cfg Config) (Store, error) {
		receivedCfg = cfg
		return NewMemoryStore(), nil
	}

	defer delete(registry, testType)

	Register(testType, testFactory)

	ctx := context.Background()
	cfg := Config{
		Type:           testType,
		DSN:            "test-dsn",
		MaxConnections: 10,
		CacheEnabled:   true,
	}

	store, err := NewStore(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Verify config was passed correctly
	assert.Equal(t, testType, receivedCfg.Type)
	assert.Equal(t, "test-dsn", receivedCfg.DSN)
	assert.Equal(t, 10, receivedCfg.MaxConnections)
	assert.True(t, receivedCfg.CacheEnabled)
}
