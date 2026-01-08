// Package main provides API key authentication for the CVT server.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AuthConfig holds authentication configuration settings.
type AuthConfig struct {
	// Enabled indicates whether API key authentication is enabled.
	Enabled bool

	// Keys is a list of valid API keys (from environment variable).
	Keys []string

	// KeysFile is the path to a JSON file containing API keys.
	KeysFile string
}

// APIKeyInfo contains information about an API key.
type APIKeyInfo struct {
	// Key is the actual API key value.
	Key string `json:"key"`

	// Name is a human-readable name for the key.
	Name string `json:"name"`

	// CreatedAt is when the key was created.
	CreatedAt time.Time `json:"created_at,omitempty"`

	// ExpiresAt is when the key expires (nil means no expiration).
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Scopes defines what the key can access.
	Scopes []string `json:"scopes,omitempty"`
}

// APIKeyStore manages valid API keys.
type APIKeyStore struct {
	keys map[string]APIKeyInfo
	mu   sync.RWMutex
}

// APIKeysFile represents the structure of the API keys JSON file.
type APIKeysFile struct {
	Keys []APIKeyInfo `json:"keys"`
}

// Environment variable names for authentication configuration.
const (
	EnvAPIKeyEnabled  = "CVT_API_KEY_ENABLED"
	EnvAPIKeys        = "CVT_API_KEYS"
	EnvAPIKeysFile    = "CVT_API_KEYS_FILE"
	APIKeyMetadataKey = "x-api-key"
)

// NewAPIKeyStore creates a new empty API key store.
func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{
		keys: make(map[string]APIKeyInfo),
	}
}

// LoadAuthConfigFromEnv loads authentication configuration from environment variables.
func LoadAuthConfigFromEnv() (*AuthConfig, error) {
	config := &AuthConfig{
		Enabled:  os.Getenv(EnvAPIKeyEnabled) == "true",
		KeysFile: os.Getenv(EnvAPIKeysFile),
	}

	// Parse comma-separated API keys from environment
	if keysEnv := os.Getenv(EnvAPIKeys); keysEnv != "" {
		config.Keys = parseAPIKeysFromEnv(keysEnv)
	}

	return config, nil
}

// LoadAPIKeys loads API keys from configuration into a store.
func LoadAPIKeys(config *AuthConfig) (*APIKeyStore, error) {
	store := NewAPIKeyStore()

	// Load keys from environment variable
	for i, key := range config.Keys {
		store.Add(APIKeyInfo{
			Key:       key,
			Name:      fmt.Sprintf("env-key-%d", i+1),
			CreatedAt: time.Now(),
			Scopes:    []string{"*"},
		})
	}

	// Load keys from file if specified
	if config.KeysFile != "" {
		if err := store.LoadFromFile(config.KeysFile); err != nil {
			return nil, fmt.Errorf("failed to load API keys from file: %w", err)
		}
	}

	Info("API keys loaded", zap.Int("count", store.Count()))
	return store, nil
}

// parseAPIKeysFromEnv parses API keys from a comma-separated string.
// Supports format: "key1,key2,key3" or "key1:name1,key2:name2"
func parseAPIKeysFromEnv(keysEnv string) []string {
	var keys []string
	for _, part := range strings.Split(keysEnv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// If format is "key:name", extract just the key
		if idx := strings.Index(part, ":"); idx > 0 {
			part = part[:idx]
		}
		keys = append(keys, part)
	}
	return keys
}

// Add adds an API key to the store.
func (s *APIKeyStore) Add(info APIKeyInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[info.Key] = info
}

// Validate checks if the provided API key is valid.
// Returns the key info and true if valid, or empty info and false if invalid.
func (s *APIKeyStore) Validate(key string) (APIKeyInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.keys[key]
	if !exists {
		return APIKeyInfo{}, false
	}

	// Check expiration
	if info.ExpiresAt != nil && time.Now().After(*info.ExpiresAt) {
		return APIKeyInfo{}, false
	}

	return info, true
}

// Count returns the number of API keys in the store.
func (s *APIKeyStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.keys)
}

// LoadFromFile loads API keys from a JSON file.
func (s *APIKeyStore) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var keysFile APIKeysFile
	if err := json.Unmarshal(data, &keysFile); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	for _, keyInfo := range keysFile.Keys {
		if keyInfo.Key == "" {
			continue
		}
		if keyInfo.Name == "" {
			keyInfo.Name = "unnamed"
		}
		if keyInfo.CreatedAt.IsZero() {
			keyInfo.CreatedAt = time.Now()
		}
		s.Add(keyInfo)
	}

	Info("Loaded API keys from file", zap.String("path", path), zap.Int("count", len(keysFile.Keys)))
	return nil
}
