package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, StoreTypeSQLite, cfg.Type)
	assert.Equal(t, "cvt.db", cfg.DSN)
	assert.Equal(t, 5, cfg.MaxIdleConns)
	assert.Equal(t, time.Hour, cfg.ConnMaxLifetime)
	assert.True(t, cfg.CacheEnabled)
	assert.Equal(t, 1000, cfg.CacheMaxSchemas)
	assert.Equal(t, 24*time.Hour, cfg.CacheTTL)
	assert.Equal(t, 90, cfg.ValidationRetentionDays)
}

func TestConfig_IsEnabled(t *testing.T) {
	t.Run("SQLite is enabled", func(t *testing.T) {
		cfg := Config{Type: StoreTypeSQLite}
		assert.True(t, cfg.IsEnabled())
	})

	t.Run("Postgres is enabled", func(t *testing.T) {
		cfg := Config{Type: StoreTypePostgres}
		assert.True(t, cfg.IsEnabled())
	})

	t.Run("Memory is not enabled", func(t *testing.T) {
		cfg := Config{Type: StoreTypeMemory}
		assert.False(t, cfg.IsEnabled())
	})
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Helper to clean up env vars - ensures clean state at start of each test
	cleanupEnv := func() {
		envVars := []string{
			"CVT_STORAGE_ENABLED",
			"CVT_STORAGE_TYPE",
			"CVT_STORAGE_DSN",
			"CVT_POSTGRES_DSN",
			"CVT_POSTGRES_HOST",
			"CVT_POSTGRES_PORT",
			"CVT_POSTGRES_USER",
			"CVT_POSTGRES_PASSWORD",
			"CVT_POSTGRES_DB",
			"CVT_POSTGRES_SSLMODE",
			"CVT_POSTGRES_MAX_CONNS",
			"CVT_STORAGE_CACHE_ENABLED",
			"CVT_STORAGE_CACHE_MAX_SCHEMAS",
			"CVT_VALIDATION_RETENTION_DAYS",
		}
		for _, v := range envVars {
			_ = os.Unsetenv(v)
		}
	}

	t.Run("storage disabled returns memory type", func(t *testing.T) {
		cleanupEnv()

		cfg := LoadConfigFromEnv()
		assert.Equal(t, StoreTypeMemory, cfg.Type)
	})

	t.Run("storage enabled with true", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, StoreTypeSQLite, cfg.Type)
	})

	t.Run("storage enabled with 1", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "1")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, StoreTypeSQLite, cfg.Type)
	})

	t.Run("custom storage type", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_TYPE", "postgres")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, StoreTypePostgres, cfg.Type)
	})

	t.Run("custom DSN for SQLite", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_DSN", "/path/to/custom.db")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, "/path/to/custom.db", cfg.DSN)
	})

	t.Run("PostgreSQL with DSN", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_TYPE", "postgres")
		t.Setenv("CVT_POSTGRES_DSN", "postgresql://user:pass@host:5432/db")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, StoreTypePostgres, cfg.Type)
		assert.Equal(t, "postgresql://user:pass@host:5432/db", cfg.DSN)
	})

	t.Run("PostgreSQL with individual components", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_TYPE", "postgres")
		t.Setenv("CVT_POSTGRES_HOST", "db.example.com")
		t.Setenv("CVT_POSTGRES_PORT", "5433")
		t.Setenv("CVT_POSTGRES_USER", "myuser")
		t.Setenv("CVT_POSTGRES_PASSWORD", "mypassword")
		t.Setenv("CVT_POSTGRES_DB", "mydb")
		t.Setenv("CVT_POSTGRES_SSLMODE", "require")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, StoreTypePostgres, cfg.Type)
		assert.Contains(t, cfg.DSN, "host=db.example.com")
		assert.Contains(t, cfg.DSN, "port=5433")
		assert.Contains(t, cfg.DSN, "user=myuser")
		assert.Contains(t, cfg.DSN, "password=mypassword")
		assert.Contains(t, cfg.DSN, "dbname=mydb")
		assert.Contains(t, cfg.DSN, "sslmode=require")
	})

	t.Run("PostgreSQL max connections", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_TYPE", "postgres")
		t.Setenv("CVT_POSTGRES_MAX_CONNS", "50")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, 50, cfg.MaxConnections)
	})

	t.Run("PostgreSQL default max connections", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_TYPE", "postgres")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, 25, cfg.MaxConnections)
	})

	t.Run("cache disabled with false", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_CACHE_ENABLED", "false")

		cfg := LoadConfigFromEnv()
		assert.False(t, cfg.CacheEnabled)
	})

	t.Run("cache disabled with 0", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_CACHE_ENABLED", "0")

		cfg := LoadConfigFromEnv()
		assert.False(t, cfg.CacheEnabled)
	})

	t.Run("custom cache max schemas", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_STORAGE_CACHE_MAX_SCHEMAS", "500")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, 500, cfg.CacheMaxSchemas)
	})

	t.Run("custom validation retention days", func(t *testing.T) {
		cleanupEnv()

		t.Setenv("CVT_STORAGE_ENABLED", "true")
		t.Setenv("CVT_VALIDATION_RETENTION_DAYS", "30")

		cfg := LoadConfigFromEnv()
		assert.Equal(t, 30, cfg.ValidationRetentionDays)
	})
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		key := "TEST_GET_ENV_OR_DEFAULT"
		t.Setenv(key, "custom_value")

		result := getEnvOrDefault(key, "default_value")
		assert.Equal(t, "custom_value", result)
	})

	t.Run("returns default when env not set", func(t *testing.T) {
		result := getEnvOrDefault("NONEXISTENT_ENV_VAR", "default_value")
		assert.Equal(t, "default_value", result)
	})

	t.Run("returns default when env is empty", func(t *testing.T) {
		key := "TEST_EMPTY_ENV"
		t.Setenv(key, "")

		result := getEnvOrDefault(key, "default_value")
		assert.Equal(t, "default_value", result)
	})
}

func TestGetEnvIntOrDefault(t *testing.T) {
	t.Run("returns int value when set", func(t *testing.T) {
		key := "TEST_GET_ENV_INT"
		t.Setenv(key, "42")

		result := getEnvIntOrDefault(key, 10)
		assert.Equal(t, 42, result)
	})

	t.Run("returns default when env not set", func(t *testing.T) {
		result := getEnvIntOrDefault("NONEXISTENT_INT_ENV", 10)
		assert.Equal(t, 10, result)
	})

	t.Run("returns default when env is not a valid int", func(t *testing.T) {
		key := "TEST_INVALID_INT"
		t.Setenv(key, "not_a_number")

		result := getEnvIntOrDefault(key, 10)
		assert.Equal(t, 10, result)
	})

	t.Run("returns default when env is empty", func(t *testing.T) {
		key := "TEST_EMPTY_INT"
		t.Setenv(key, "")

		result := getEnvIntOrDefault(key, 10)
		assert.Equal(t, 10, result)
	})

	t.Run("handles negative numbers", func(t *testing.T) {
		key := "TEST_NEGATIVE_INT"
		t.Setenv(key, "-5")

		result := getEnvIntOrDefault(key, 10)
		assert.Equal(t, -5, result)
	})

	t.Run("handles zero", func(t *testing.T) {
		key := "TEST_ZERO_INT"
		t.Setenv(key, "0")

		result := getEnvIntOrDefault(key, 10)
		assert.Equal(t, 0, result)
	})
}

func TestStoreType(t *testing.T) {
	t.Run("store type constants", func(t *testing.T) {
		assert.Equal(t, StoreType("sqlite"), StoreTypeSQLite)
		assert.Equal(t, StoreType("postgres"), StoreTypePostgres)
		assert.Equal(t, StoreType("memory"), StoreTypeMemory)
	})
}
