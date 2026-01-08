package storage

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// StoreType represents the storage backend type.
type StoreType string

const (
	StoreTypeSQLite   StoreType = "sqlite"
	StoreTypePostgres StoreType = "postgres"
	StoreTypeMemory   StoreType = "memory" // For testing
)

// Config holds storage configuration.
type Config struct {
	Type            StoreType
	DSN             string        // Data source name
	MaxConnections  int           // Max open connections (postgres only)
	MaxIdleConns    int           // Max idle connections
	ConnMaxLifetime time.Duration // Connection max lifetime

	// Cache settings (for hybrid cache+db mode)
	CacheEnabled    bool
	CacheMaxSchemas int
	CacheTTL        time.Duration

	// Retention settings
	ValidationRetentionDays int // How long to keep validation records
}

// DefaultConfig returns a default configuration for SQLite.
func DefaultConfig() Config {
	return Config{
		Type:                    StoreTypeSQLite,
		DSN:                     "cvt.db",
		MaxIdleConns:            5,
		ConnMaxLifetime:         time.Hour,
		CacheEnabled:            true,
		CacheMaxSchemas:         1000,
		CacheTTL:                24 * time.Hour,
		ValidationRetentionDays: 90,
	}
}

// LoadConfigFromEnv loads storage configuration from environment variables.
func LoadConfigFromEnv() Config {
	cfg := DefaultConfig()

	// Check if storage is enabled
	if enabled := os.Getenv("CVT_STORAGE_ENABLED"); enabled != "true" && enabled != "1" {
		// Return config but caller should check if storage is needed
		cfg.Type = StoreTypeMemory
		return cfg
	}

	// Storage type
	if storeType := os.Getenv("CVT_STORAGE_TYPE"); storeType != "" {
		cfg.Type = StoreType(storeType)
	}

	// DSN for SQLite
	if dsn := os.Getenv("CVT_STORAGE_DSN"); dsn != "" {
		cfg.DSN = dsn
	}

	// PostgreSQL specific configuration
	if cfg.Type == StoreTypePostgres {
		if dsn := os.Getenv("CVT_POSTGRES_DSN"); dsn != "" {
			cfg.DSN = dsn
		} else {
			// Build DSN from individual components
			host := getEnvOrDefault("CVT_POSTGRES_HOST", "localhost")
			port := getEnvOrDefault("CVT_POSTGRES_PORT", "5432")
			user := getEnvOrDefault("CVT_POSTGRES_USER", "cvt")
			password := os.Getenv("CVT_POSTGRES_PASSWORD")
			dbname := getEnvOrDefault("CVT_POSTGRES_DB", "cvt")
			sslmode := getEnvOrDefault("CVT_POSTGRES_SSLMODE", "disable")

			cfg.DSN = fmt.Sprintf(
				"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
				host, port, user, password, dbname, sslmode,
			)
		}
		cfg.MaxConnections = getEnvIntOrDefault("CVT_POSTGRES_MAX_CONNS", 25)
	}

	// Cache settings
	if cacheEnabled := os.Getenv("CVT_STORAGE_CACHE_ENABLED"); cacheEnabled == "false" || cacheEnabled == "0" {
		cfg.CacheEnabled = false
	}

	cfg.CacheMaxSchemas = getEnvIntOrDefault("CVT_STORAGE_CACHE_MAX_SCHEMAS", cfg.CacheMaxSchemas)
	cfg.ValidationRetentionDays = getEnvIntOrDefault("CVT_VALIDATION_RETENTION_DAYS", cfg.ValidationRetentionDays)

	return cfg
}

// IsEnabled returns true if persistent storage is enabled.
func (c Config) IsEnabled() bool {
	return c.Type != StoreTypeMemory
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvIntOrDefault returns the environment variable as int or a default.
func getEnvIntOrDefault(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
