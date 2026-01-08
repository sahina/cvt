package storage

import (
	"context"
	"fmt"
)

// StoreFactory is a function that creates a Store from configuration.
type StoreFactory func(ctx context.Context, cfg Config) (Store, error)

// Registry holds store factories by type.
var registry = make(map[StoreType]StoreFactory)

// Register registers a store factory for a given type.
// This should be called from init() in implementation packages.
func Register(storeType StoreType, factory StoreFactory) {
	registry[storeType] = factory
}

// NewStore creates a new Store based on the provided configuration.
func NewStore(ctx context.Context, cfg Config) (Store, error) {
	// Handle built-in memory store
	if cfg.Type == StoreTypeMemory {
		return NewMemoryStore(), nil
	}

	factory, ok := registry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported storage type: %s (did you import the driver?)", cfg.Type)
	}

	return factory(ctx, cfg)
}

// MustNewStore creates a new Store and panics on error.
func MustNewStore(ctx context.Context, cfg Config) Store {
	store, err := NewStore(ctx, cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create store: %v", err))
	}
	return store
}
