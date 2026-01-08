// Package main provides caching functionality for the Contract Validation Tool.
// This file implements a high-performance cache for storing OpenAPI schemas
// using the Ristretto cache library.
package main

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/getkin/kin-openapi/openapi3"
)

const (
	// MaxSchemas is the maximum number of schemas that can be stored in the cache
	MaxSchemas = 1000

	// SchemaTTL is the time-to-live for cached schemas (24 hours)
	// After this duration, schemas are automatically evicted from the cache
	SchemaTTL = 24 * time.Hour

	// CacheNumCounters is the number of counters for the cache's Count-Min Sketch
	// Set to 10x the max items for better accuracy in frequency estimation
	CacheNumCounters = 10000 // 10x the max items for better accuracy

	// CacheMaxCost is the maximum total cost the cache can hold
	// Each schema has a cost of 1, so this equals the maximum number of schemas
	CacheMaxCost = 1000 // Each schema has cost of 1

	// CacheBufferItems is the number of keys per Get buffer
	// This controls the size of the buffer for handling concurrent Get operations
	CacheBufferItems = 64 // Number of keys per Get buffer
)

// VersionSeparator is used to create versioned cache keys.
const VersionSeparator = "@"

// EndpointUsage describes which endpoints and fields a consumer uses.
type EndpointUsage struct {
	Method     string   // HTTP method (GET, POST, etc.)
	Path       string   // API path (e.g., "/users/{id}")
	UsedFields []string // Fields used in response (e.g., ["email", "name"])
}

// ConsumerEntry represents a consumer's dependency on a schema.
type ConsumerEntry struct {
	ConsumerID      string          // User-facing consumer ID (e.g., "order-service")
	ConsumerVersion string          // Consumer's version (e.g., "2.1.0")
	SchemaID        string          // Schema this consumer depends on
	SchemaVersion   string          // Schema version consumer was tested against
	Environment     string          // Environment (dev, staging, prod)
	RegisteredAt    time.Time       // Initial registration timestamp
	LastValidatedAt time.Time       // Last successful validation timestamp
	UsedEndpoints   []EndpointUsage // Which endpoints the consumer uses
}

// SchemaCache wraps Ristretto cache for storing OpenAPI validators.
// It provides a high-performance, thread-safe cache with automatic eviction
// based on TTL (24 hours) and LRU (Least Recently Used) policies.
//
// The cache uses Ristretto, which implements a TinyLFU admission policy
// for optimal cache hit rates.
//
// Cache Key Format:
//   - "schema_id" -> latest SchemaEntry
//   - "schema_id@version" -> specific version's SchemaEntry
type SchemaCache struct {
	cache *ristretto.Cache

	// schemaVersions tracks all versions of each schema for ListSchemas
	schemaVersions map[string][]string // schemaID -> list of versions
	versionsMu     sync.RWMutex

	// consumers tracks registered consumers by consumerID/environment
	consumers   map[string]*ConsumerEntry // consumerID/environment -> entry
	consumersMu sync.RWMutex
}

// NewSchemaCache creates a new schema cache with the specified configuration.
// The cache is configured with:
// - Maximum capacity: 1000 schemas
// - TTL: 24 hours (schemas expire after this duration)
// - Eviction policy: LRU with TinyLFU admission
// - Thread-safe: Supports concurrent access
//
// Returns:
//   - *SchemaCache: A new cache instance ready for use
//   - error: An error if cache initialization fails
func NewSchemaCache() (*SchemaCache, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: CacheNumCounters,
		MaxCost:     CacheMaxCost,
		BufferItems: CacheBufferItems,
	})
	if err != nil {
		return nil, err
	}

	return &SchemaCache{
		cache:          cache,
		schemaVersions: make(map[string][]string),
		consumers:      make(map[string]*ConsumerEntry),
	}, nil
}

// Set stores a schema entry in the cache with the given ID.
// The schema is stored with a cost of 1 and a TTL of 24 hours.
// This also updates the "latest" pointer to this entry.
//
// This method blocks until the value has been processed through
// the cache's internal buffers, ensuring the value is immediately
// available for subsequent Get calls.
//
// Parameters:
//   - schemaID: The unique identifier for the schema
//   - entry: The schema entry to cache (includes document and metadata)
func (sc *SchemaCache) Set(schemaID string, entry *SchemaEntry) {
	// Store as latest (without version suffix)
	sc.cache.SetWithTTL(schemaID, entry, 1, SchemaTTL)

	// Also store with version if specified
	if entry.Metadata != nil && entry.Metadata.SchemaVersion != "" {
		versionedKey := makeVersionedKey(schemaID, entry.Metadata.SchemaVersion)
		sc.cache.SetWithTTL(versionedKey, entry, 1, SchemaTTL)

		// Track version
		sc.trackVersion(schemaID, entry.Metadata.SchemaVersion)
	}

	// Wait for value to pass through buffers to ensure immediate availability
	sc.cache.Wait()
}

// SetLegacy stores a schema document in the cache (for backward compatibility).
// This creates a minimal SchemaEntry without metadata.
//
// Deprecated: Use Set with a proper SchemaEntry instead.
func (sc *SchemaCache) SetLegacy(schemaID string, schema *openapi3.T) {
	entry := &SchemaEntry{
		Document: schema,
	}
	sc.Set(schemaID, entry)
}

// Get retrieves the latest schema entry from the cache by ID.
// This method is thread-safe and can be called concurrently.
//
// Parameters:
//   - schemaID: The unique identifier for the schema
//
// Returns:
//   - *SchemaEntry: The cached schema entry (nil if not found)
//   - bool: True if the schema was found and is valid, false otherwise
func (sc *SchemaCache) Get(schemaID string) (*SchemaEntry, bool) {
	value, found := sc.cache.Get(schemaID)
	if !found {
		cacheMisses.Inc()
		return nil, false
	}

	entry, ok := value.(*SchemaEntry)
	if ok {
		cacheHits.Inc()
	} else {
		cacheMisses.Inc()
	}
	return entry, ok
}

// GetVersion retrieves a specific version of a schema from the cache.
//
// Parameters:
//   - schemaID: The unique identifier for the schema
//   - version: The specific version to retrieve (empty string = latest)
//
// Returns:
//   - *SchemaEntry: The cached schema entry (nil if not found)
//   - bool: True if the schema was found and is valid, false otherwise
func (sc *SchemaCache) GetVersion(schemaID, version string) (*SchemaEntry, bool) {
	if version == "" {
		return sc.Get(schemaID)
	}

	versionedKey := makeVersionedKey(schemaID, version)
	value, found := sc.cache.Get(versionedKey)
	if !found {
		cacheMisses.Inc()
		return nil, false
	}

	entry, ok := value.(*SchemaEntry)
	if ok {
		cacheHits.Inc()
	} else {
		cacheMisses.Inc()
	}
	return entry, ok
}

// GetDocument retrieves the OpenAPI document from the cache by ID.
// This is a convenience method that extracts just the document.
//
// Parameters:
//   - schemaID: The unique identifier for the schema
//
// Returns:
//   - *openapi3.T: The cached schema document
//   - bool: True if the schema was found and is valid, false otherwise
func (sc *SchemaCache) GetDocument(schemaID string) (*openapi3.T, bool) {
	entry, ok := sc.Get(schemaID)
	if !ok || entry == nil {
		return nil, false
	}
	return entry.Document, true
}

// Delete removes a schema from the cache by ID.
// This removes both the latest version and all tracked versions.
//
// Parameters:
//   - schemaID: The unique identifier for the schema to remove
func (sc *SchemaCache) Delete(schemaID string) {
	// Delete latest
	sc.cache.Del(schemaID)

	// Delete all versions
	sc.versionsMu.Lock()
	versions := sc.schemaVersions[schemaID]
	delete(sc.schemaVersions, schemaID)
	sc.versionsMu.Unlock()

	for _, version := range versions {
		sc.cache.Del(makeVersionedKey(schemaID, version))
	}
}

// Clear removes all schemas from the cache.
// This completely empties the cache, removing all stored schemas
// regardless of their TTL or last access time.
func (sc *SchemaCache) Clear() {
	sc.cache.Clear()
	sc.versionsMu.Lock()
	sc.schemaVersions = make(map[string][]string)
	sc.versionsMu.Unlock()
}

// Close closes the cache and releases resources.
// This should be called when the cache is no longer needed,
// typically during server shutdown. After calling Close,
// the cache should not be used.
func (sc *SchemaCache) Close() {
	sc.cache.Close()
}

// ListSchemaIDs returns all known schema IDs.
func (sc *SchemaCache) ListSchemaIDs() []string {
	sc.versionsMu.RLock()
	defer sc.versionsMu.RUnlock()

	ids := make([]string, 0, len(sc.schemaVersions))
	for id := range sc.schemaVersions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ListVersions returns all versions of a specific schema.
func (sc *SchemaCache) ListVersions(schemaID string) []string {
	sc.versionsMu.RLock()
	defer sc.versionsMu.RUnlock()

	versions, ok := sc.schemaVersions[schemaID]
	if !ok {
		return nil
	}

	// Return a copy to avoid race conditions
	result := make([]string, len(versions))
	copy(result, versions)
	return result
}

// GetPreviousVersion returns the version before the specified version.
func (sc *SchemaCache) GetPreviousVersion(schemaID, currentVersion string) (string, bool) {
	versions := sc.ListVersions(schemaID)
	if len(versions) < 2 {
		return "", false
	}

	// Find the current version index
	for i, v := range versions {
		if v == currentVersion && i > 0 {
			return versions[i-1], true
		}
	}
	return "", false
}

// trackVersion adds a version to the tracking list for a schema.
func (sc *SchemaCache) trackVersion(schemaID, version string) {
	sc.versionsMu.Lock()
	defer sc.versionsMu.Unlock()

	versions := sc.schemaVersions[schemaID]

	// Check if version already exists
	for _, v := range versions {
		if v == version {
			return
		}
	}

	// Add new version and sort
	versions = append(versions, version)
	sort.Strings(versions)
	sc.schemaVersions[schemaID] = versions
}

// makeVersionedKey creates a cache key for a specific version.
func makeVersionedKey(schemaID, version string) string {
	return fmt.Sprintf("%s%s%s", schemaID, VersionSeparator, version)
}

// ============================================================================
// Consumer Registry Methods
// ============================================================================

// makeConsumerKey creates a unique key for a consumer registration.
func makeConsumerKey(consumerID, schemaID, environment string) string {
	return fmt.Sprintf("%s/%s/%s", consumerID, schemaID, environment)
}

// RegisterConsumer registers a consumer's dependency on a schema.
// If the consumer already exists, it updates the registration.
func (sc *SchemaCache) RegisterConsumer(entry *ConsumerEntry) {
	sc.consumersMu.Lock()
	defer sc.consumersMu.Unlock()

	key := makeConsumerKey(entry.ConsumerID, entry.SchemaID, entry.Environment)

	// Preserve original registration time if exists
	if existing, ok := sc.consumers[key]; ok {
		entry.RegisteredAt = existing.RegisteredAt
	}

	sc.consumers[key] = entry
}

// ListConsumers returns all consumers that depend on a schema.
// If environment is empty, returns consumers from all environments.
func (sc *SchemaCache) ListConsumers(schemaID, environment string) []*ConsumerEntry {
	sc.consumersMu.RLock()
	defer sc.consumersMu.RUnlock()

	var result []*ConsumerEntry
	for _, consumer := range sc.consumers {
		if consumer.SchemaID != schemaID {
			continue
		}
		if environment != "" && consumer.Environment != environment {
			continue
		}
		result = append(result, consumer)
	}
	return result
}

// DeregisterConsumer removes a consumer registration.
// Returns true if the consumer was found and removed.
func (sc *SchemaCache) DeregisterConsumer(consumerID, schemaID, environment string) bool {
	sc.consumersMu.Lock()
	defer sc.consumersMu.Unlock()

	key := makeConsumerKey(consumerID, schemaID, environment)
	if _, ok := sc.consumers[key]; !ok {
		return false
	}

	delete(sc.consumers, key)
	return true
}

// GetConsumer retrieves a specific consumer registration.
func (sc *SchemaCache) GetConsumer(consumerID, schemaID, environment string) (*ConsumerEntry, bool) {
	sc.consumersMu.RLock()
	defer sc.consumersMu.RUnlock()

	key := makeConsumerKey(consumerID, schemaID, environment)
	entry, ok := sc.consumers[key]
	return entry, ok
}
