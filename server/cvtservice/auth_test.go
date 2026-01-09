package cvtservice

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIKeyStore(t *testing.T) {
	t.Run("creates empty store", func(t *testing.T) {
		store := NewAPIKeyStore()
		assert.NotNil(t, store)
		assert.Equal(t, 0, store.Count())
	})
}

func TestAPIKeyStore_Add(t *testing.T) {
	store := NewAPIKeyStore()

	t.Run("add valid key", func(t *testing.T) {
		store.Add(APIKeyInfo{
			Key:  "test-key-123",
			Name: "test-service",
		})
		assert.Equal(t, 1, store.Count())
	})

	t.Run("add multiple keys", func(t *testing.T) {
		store.Add(APIKeyInfo{
			Key:  "test-key-456",
			Name: "another-service",
		})
		assert.Equal(t, 2, store.Count())
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		store.Add(APIKeyInfo{
			Key:  "test-key-123",
			Name: "updated-service",
		})
		assert.Equal(t, 2, store.Count())
		info, valid := store.Validate("test-key-123")
		require.True(t, valid)
		assert.Equal(t, "updated-service", info.Name)
	})
}

func TestAPIKeyStore_Validate(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add(APIKeyInfo{
		Key:  "valid-key",
		Name: "test-service",
	})

	t.Run("valid key returns true", func(t *testing.T) {
		info, valid := store.Validate("valid-key")
		assert.True(t, valid)
		assert.Equal(t, "test-service", info.Name)
	})

	t.Run("invalid key returns false", func(t *testing.T) {
		_, valid := store.Validate("invalid-key")
		assert.False(t, valid)
	})

	t.Run("empty key returns false", func(t *testing.T) {
		_, valid := store.Validate("")
		assert.False(t, valid)
	})

	t.Run("expired key returns false", func(t *testing.T) {
		pastTime := time.Now().Add(-1 * time.Hour)
		store.Add(APIKeyInfo{
			Key:       "expired-key",
			Name:      "expired-service",
			ExpiresAt: &pastTime,
		})
		_, valid := store.Validate("expired-key")
		assert.False(t, valid)
	})

	t.Run("non-expired key returns true", func(t *testing.T) {
		futureTime := time.Now().Add(1 * time.Hour)
		store.Add(APIKeyInfo{
			Key:       "future-key",
			Name:      "future-service",
			ExpiresAt: &futureTime,
		})
		info, valid := store.Validate("future-key")
		assert.True(t, valid)
		assert.Equal(t, "future-service", info.Name)
	})
}

func TestAPIKeyStore_Count(t *testing.T) {
	store := NewAPIKeyStore()
	assert.Equal(t, 0, store.Count())

	store.Add(APIKeyInfo{Key: "key1", Name: "service1"})
	assert.Equal(t, 1, store.Count())

	store.Add(APIKeyInfo{Key: "key2", Name: "service2"})
	assert.Equal(t, 2, store.Count())
}

func TestAPIKeyStore_LoadFromFile(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("loads valid keys file", func(t *testing.T) {
		keysFile := APIKeysFile{
			Keys: []APIKeyInfo{
				{Key: "file-key-1", Name: "service-1"},
				{Key: "file-key-2", Name: "service-2"},
			},
		}
		data, err := json.Marshal(keysFile)
		require.NoError(t, err)

		filePath := filepath.Join(tempDir, "keys.json")
		err = os.WriteFile(filePath, data, 0600)
		require.NoError(t, err)

		store := NewAPIKeyStore()
		err = store.LoadFromFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, 2, store.Count())

		_, valid := store.Validate("file-key-1")
		assert.True(t, valid)
	})

	t.Run("fails with non-existent file", func(t *testing.T) {
		store := NewAPIKeyStore()
		err := store.LoadFromFile("/nonexistent/keys.json")
		assert.Error(t, err)
	})

	t.Run("fails with invalid JSON", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "invalid.json")
		err := os.WriteFile(filePath, []byte("not valid json"), 0600)
		require.NoError(t, err)

		store := NewAPIKeyStore()
		err = store.LoadFromFile(filePath)
		assert.Error(t, err)
	})

	t.Run("skips keys with empty key field", func(t *testing.T) {
		keysFile := APIKeysFile{
			Keys: []APIKeyInfo{
				{Key: "", Name: "empty-key"},
				{Key: "valid-key", Name: "valid"},
			},
		}
		data, err := json.Marshal(keysFile)
		require.NoError(t, err)

		filePath := filepath.Join(tempDir, "partial.json")
		err = os.WriteFile(filePath, data, 0600)
		require.NoError(t, err)

		store := NewAPIKeyStore()
		err = store.LoadFromFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, 1, store.Count())
	})
}

func TestLoadAuthConfigFromEnv(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		t.Setenv(EnvAPIKeyEnabled, "")
		config, err := LoadAuthConfigFromEnv()
		require.NoError(t, err)
		assert.False(t, config.Enabled)
	})

	t.Run("enabled when env var is true", func(t *testing.T) {
		t.Setenv(EnvAPIKeyEnabled, "true")
		config, err := LoadAuthConfigFromEnv()
		require.NoError(t, err)
		assert.True(t, config.Enabled)
	})

	t.Run("parses comma-separated keys", func(t *testing.T) {
		t.Setenv(EnvAPIKeys, "key1,key2,key3")
		config, err := LoadAuthConfigFromEnv()
		require.NoError(t, err)
		assert.Len(t, config.Keys, 3)
		assert.Contains(t, config.Keys, "key1")
		assert.Contains(t, config.Keys, "key2")
		assert.Contains(t, config.Keys, "key3")
	})

	t.Run("reads keys file path", func(t *testing.T) {
		t.Setenv(EnvAPIKeysFile, "/path/to/keys.json")
		config, err := LoadAuthConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "/path/to/keys.json", config.KeysFile)
	})
}

func TestLoadAPIKeys(t *testing.T) {
	t.Run("loads keys from config", func(t *testing.T) {
		config := &AuthConfig{
			Enabled: true,
			Keys:    []string{"env-key-1", "env-key-2"},
		}

		store, err := LoadAPIKeys(config)
		require.NoError(t, err)
		assert.Equal(t, 2, store.Count())

		_, valid := store.Validate("env-key-1")
		assert.True(t, valid)
	})

	t.Run("fails with invalid keys file", func(t *testing.T) {
		config := &AuthConfig{
			Enabled:  true,
			KeysFile: "/nonexistent/keys.json",
		}

		_, err := LoadAPIKeys(config)
		assert.Error(t, err)
	})
}

func TestParseAPIKeysFromEnv(t *testing.T) {
	t.Run("parses simple comma-separated keys", func(t *testing.T) {
		keys := parseAPIKeysFromEnv("key1,key2,key3")
		assert.Equal(t, []string{"key1", "key2", "key3"}, keys)
	})

	t.Run("handles key:name format", func(t *testing.T) {
		keys := parseAPIKeysFromEnv("key1:name1,key2:name2")
		assert.Equal(t, []string{"key1", "key2"}, keys)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		keys := parseAPIKeysFromEnv(" key1 , key2 , key3 ")
		assert.Equal(t, []string{"key1", "key2", "key3"}, keys)
	})

	t.Run("skips empty parts", func(t *testing.T) {
		keys := parseAPIKeysFromEnv("key1,,key2,")
		assert.Equal(t, []string{"key1", "key2"}, keys)
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		keys := parseAPIKeysFromEnv("")
		assert.Nil(t, keys)
	})
}
