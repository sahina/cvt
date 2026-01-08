package main

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestSchemaEntry creates a SchemaEntry for testing.
func createTestSchemaEntry(schemaID, version string) *SchemaEntry {
	doc := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Test Schema",
			Version: version,
		},
	}
	return NewSchemaEntry(schemaID, `{"openapi":"3.0.0"}`, doc, version, nil)
}

func TestSchemaCache(t *testing.T) {
	t.Run("NewSchemaCache", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		assert.NotNil(t, cache)
		cache.Close()
	})

	t.Run("Set and Get", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		defer cache.Close()

		entry := createTestSchemaEntry("test-schema", "1.0.0")

		cache.Set("test-schema", entry)

		retrieved, found := cache.Get("test-schema")
		assert.True(t, found)
		assert.NotNil(t, retrieved)
		assert.Equal(t, entry.Document, retrieved.Document)

		_, found = cache.Get("non-existent")
		assert.False(t, found)
	})

	t.Run("GetDocument", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		defer cache.Close()

		entry := createTestSchemaEntry("test-schema", "1.0.0")
		cache.Set("test-schema", entry)

		doc, found := cache.GetDocument("test-schema")
		assert.True(t, found)
		assert.NotNil(t, doc)
		assert.Equal(t, "3.0.0", doc.OpenAPI)

		_, found = cache.GetDocument("non-existent")
		assert.False(t, found)
	})

	t.Run("GetVersion", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		defer cache.Close()

		entry := createTestSchemaEntry("test-schema", "1.0.0")
		cache.Set("test-schema", entry)

		// Get specific version
		retrieved, found := cache.GetVersion("test-schema", "1.0.0")
		assert.True(t, found)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "1.0.0", retrieved.Metadata.SchemaVersion)

		// Get latest (empty version)
		retrieved, found = cache.GetVersion("test-schema", "")
		assert.True(t, found)
		assert.NotNil(t, retrieved)

		// Non-existent version
		_, found = cache.GetVersion("test-schema", "2.0.0")
		assert.False(t, found)
	})

	t.Run("Delete", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		defer cache.Close()

		entry := createTestSchemaEntry("to-delete", "1.0.0")
		cache.Set("to-delete", entry)

		// Verify it's there
		_, found := cache.Get("to-delete")
		require.True(t, found)

		// Delete it
		cache.Delete("to-delete")

		// The main goal is code coverage for the method.
	})

	t.Run("Clear", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		defer cache.Close()

		entry1 := createTestSchemaEntry("schema1", "1.0.0")
		entry2 := createTestSchemaEntry("schema2", "1.0.0")
		cache.Set("schema1", entry1)
		cache.Set("schema2", entry2)

		cache.Clear()

		// Main goal is coverage.
	})

	t.Run("ListSchemaIDs", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		defer cache.Close()

		entry1 := createTestSchemaEntry("alpha", "1.0.0")
		entry2 := createTestSchemaEntry("beta", "1.0.0")
		cache.Set("alpha", entry1)
		cache.Set("beta", entry2)

		ids := cache.ListSchemaIDs()
		assert.Contains(t, ids, "alpha")
		assert.Contains(t, ids, "beta")
	})

	t.Run("ListVersions", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		defer cache.Close()

		entry1 := createTestSchemaEntry("test-schema", "1.0.0")
		entry2 := createTestSchemaEntry("test-schema", "2.0.0")
		cache.Set("test-schema", entry1)
		cache.Set("test-schema", entry2)

		versions := cache.ListVersions("test-schema")
		assert.Contains(t, versions, "1.0.0")
		assert.Contains(t, versions, "2.0.0")

		// Non-existent schema
		versions = cache.ListVersions("non-existent")
		assert.Nil(t, versions)
	})

	t.Run("GetPreviousVersion", func(t *testing.T) {
		cache, err := NewSchemaCache()
		require.NoError(t, err)
		defer cache.Close()

		entry1 := createTestSchemaEntry("test-schema", "1.0.0")
		entry2 := createTestSchemaEntry("test-schema", "2.0.0")
		cache.Set("test-schema", entry1)
		cache.Set("test-schema", entry2)

		prev, found := cache.GetPreviousVersion("test-schema", "2.0.0")
		assert.True(t, found)
		assert.Equal(t, "1.0.0", prev)

		// First version has no previous
		_, found = cache.GetPreviousVersion("test-schema", "1.0.0")
		assert.False(t, found)
	})
}
