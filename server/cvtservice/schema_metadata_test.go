package cvtservice

import (
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/sahina/cvt/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestDocument(version string) *openapi3.T {
	return &openapi3.T{
		OpenAPI: version,
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
	}
}

func createTestDocumentWithPaths() *openapi3.T {
	doc := createTestDocument("3.0.0")
	doc.Paths = &openapi3.Paths{}

	// Add a path with multiple methods
	doc.Paths.Set("/users", &openapi3.PathItem{
		Get:    &openapi3.Operation{OperationID: "getUsers"},
		Post:   &openapi3.Operation{OperationID: "createUser"},
		Delete: &openapi3.Operation{OperationID: "deleteUsers"},
	})

	// Add another path
	doc.Paths.Set("/users/{id}", &openapi3.PathItem{
		Get:   &openapi3.Operation{OperationID: "getUser"},
		Put:   &openapi3.Operation{OperationID: "updateUser"},
		Patch: &openapi3.Operation{OperationID: "patchUser"},
	})

	return doc
}

func TestNewSchemaEntry(t *testing.T) {
	t.Run("creates entry with all fields", func(t *testing.T) {
		doc := createTestDocumentWithPaths()
		content := `{"openapi":"3.0.0"}`
		ownership := &pb.SchemaOwnership{
			Owner:        "test-owner",
			Team:         "test-team",
			ContactEmail: "test@example.com",
		}

		entry := NewSchemaEntry("test-schema", content, doc, "1.0.0", ownership)

		require.NotNil(t, entry)
		assert.Equal(t, doc, entry.Document)
		assert.Equal(t, content, entry.Content)
		assert.NotNil(t, entry.Metadata)

		// Check metadata fields
		assert.Equal(t, "test-schema", entry.Metadata.SchemaId)
		assert.Equal(t, "1.0.0", entry.Metadata.SchemaVersion)
		assert.NotEmpty(t, entry.Metadata.SchemaHash)
		assert.Greater(t, entry.Metadata.RegisteredAt, int64(0))
		assert.Equal(t, entry.Metadata.RegisteredAt, entry.Metadata.UpdatedAt)
		assert.Equal(t, ownership, entry.Metadata.Ownership)
		assert.Equal(t, "3.0.0", entry.Metadata.OpenapiVersion)
		assert.Equal(t, int32(6), entry.Metadata.EndpointCount) // 6 methods across 2 paths
	})

	t.Run("handles empty OpenAPI version", func(t *testing.T) {
		doc := &openapi3.T{
			OpenAPI: "", // Empty version
			Info: &openapi3.Info{
				Title:   "Test API",
				Version: "1.0.0",
			},
		}

		entry := NewSchemaEntry("test-schema", "{}", doc, "1.0.0", nil)

		assert.Equal(t, "3.0.0", entry.Metadata.OpenapiVersion)
	})

	t.Run("handles nil ownership", func(t *testing.T) {
		doc := createTestDocument("3.0.0")

		entry := NewSchemaEntry("test-schema", "{}", doc, "1.0.0", nil)

		assert.Nil(t, entry.Metadata.Ownership)
	})

	t.Run("computes hash correctly", func(t *testing.T) {
		doc := createTestDocument("3.0.0")
		content := `{"openapi":"3.0.0"}`

		entry := NewSchemaEntry("test-schema", content, doc, "1.0.0", nil)

		// Hash should be a hex string of SHA256 (64 characters)
		assert.Len(t, entry.Metadata.SchemaHash, 64)
	})
}

func TestSchemaEntry_UpdateMetadata(t *testing.T) {
	t.Run("updates all fields", func(t *testing.T) {
		doc := createTestDocument("3.0.0")
		entry := NewSchemaEntry("test-schema", "{}", doc, "1.0.0", nil)
		originalUpdatedAt := entry.Metadata.UpdatedAt

		// Wait a moment to ensure timestamp changes
		time.Sleep(10 * time.Millisecond)

		newDoc := createTestDocumentWithPaths()
		newDoc.OpenAPI = "3.1.0"
		newContent := `{"openapi":"3.1.0"}`
		newOwnership := &pb.SchemaOwnership{Owner: "new-owner"}

		entry.UpdateMetadata(newContent, newDoc, "2.0.0", newOwnership)

		assert.Equal(t, newDoc, entry.Document)
		assert.Equal(t, newContent, entry.Content)
		assert.Equal(t, "2.0.0", entry.Metadata.SchemaVersion)
		assert.Equal(t, "3.1.0", entry.Metadata.OpenapiVersion)
		assert.Equal(t, newOwnership, entry.Metadata.Ownership)
		assert.Equal(t, int32(6), entry.Metadata.EndpointCount)
		assert.GreaterOrEqual(t, entry.Metadata.UpdatedAt, originalUpdatedAt)
	})

	t.Run("keeps version when empty string provided", func(t *testing.T) {
		doc := createTestDocument("3.0.0")
		entry := NewSchemaEntry("test-schema", "{}", doc, "1.0.0", nil)

		entry.UpdateMetadata("{}", doc, "", nil)

		assert.Equal(t, "1.0.0", entry.Metadata.SchemaVersion)
	})

	t.Run("keeps OpenAPI version when doc version is empty", func(t *testing.T) {
		doc := createTestDocument("3.0.0")
		entry := NewSchemaEntry("test-schema", "{}", doc, "1.0.0", nil)

		newDoc := &openapi3.T{OpenAPI: ""}
		entry.UpdateMetadata("{}", newDoc, "", nil)

		assert.Equal(t, "3.0.0", entry.Metadata.OpenapiVersion)
	})

	t.Run("keeps ownership when nil provided", func(t *testing.T) {
		originalOwnership := &pb.SchemaOwnership{Owner: "original"}
		doc := createTestDocument("3.0.0")
		entry := NewSchemaEntry("test-schema", "{}", doc, "1.0.0", originalOwnership)

		entry.UpdateMetadata("{}", doc, "", nil)

		assert.Equal(t, originalOwnership, entry.Metadata.Ownership)
	})

	t.Run("recomputes hash on update", func(t *testing.T) {
		doc := createTestDocument("3.0.0")
		entry := NewSchemaEntry("test-schema", "original content", doc, "1.0.0", nil)
		originalHash := entry.Metadata.SchemaHash

		entry.UpdateMetadata("new content", doc, "", nil)

		assert.NotEqual(t, originalHash, entry.Metadata.SchemaHash)
	})
}

func TestComputeSchemaHash(t *testing.T) {
	t.Run("returns consistent hash for same content", func(t *testing.T) {
		content := `{"openapi":"3.0.0"}`

		hash1 := computeSchemaHash(content)
		hash2 := computeSchemaHash(content)

		assert.Equal(t, hash1, hash2)
	})

	t.Run("returns different hash for different content", func(t *testing.T) {
		hash1 := computeSchemaHash("content1")
		hash2 := computeSchemaHash("content2")

		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("normalizes whitespace", func(t *testing.T) {
		hash1 := computeSchemaHash("  content  ")
		hash2 := computeSchemaHash("content")

		assert.Equal(t, hash1, hash2)
	})

	t.Run("returns hex-encoded SHA256", func(t *testing.T) {
		hash := computeSchemaHash("test")

		// SHA256 produces 32 bytes = 64 hex characters
		assert.Len(t, hash, 64)
		// Should only contain hex characters
		for _, c := range hash {
			assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'))
		}
	})

	t.Run("handles empty content", func(t *testing.T) {
		hash := computeSchemaHash("")

		assert.Len(t, hash, 64)
	})
}

func TestCountEndpoints(t *testing.T) {
	t.Run("returns 0 for nil document", func(t *testing.T) {
		count := countEndpoints(nil)
		assert.Equal(t, 0, count)
	})

	t.Run("returns 0 for document with nil paths", func(t *testing.T) {
		doc := &openapi3.T{}
		count := countEndpoints(doc)
		assert.Equal(t, 0, count)
	})

	t.Run("returns 0 for empty paths", func(t *testing.T) {
		doc := &openapi3.T{
			Paths: &openapi3.Paths{},
		}
		count := countEndpoints(doc)
		assert.Equal(t, 0, count)
	})

	t.Run("counts GET method", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Get: &openapi3.Operation{},
		})
		count := countEndpoints(doc)
		assert.Equal(t, 1, count)
	})

	t.Run("counts POST method", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Post: &openapi3.Operation{},
		})
		count := countEndpoints(doc)
		assert.Equal(t, 1, count)
	})

	t.Run("counts PUT method", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Put: &openapi3.Operation{},
		})
		count := countEndpoints(doc)
		assert.Equal(t, 1, count)
	})

	t.Run("counts DELETE method", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Delete: &openapi3.Operation{},
		})
		count := countEndpoints(doc)
		assert.Equal(t, 1, count)
	})

	t.Run("counts PATCH method", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Patch: &openapi3.Operation{},
		})
		count := countEndpoints(doc)
		assert.Equal(t, 1, count)
	})

	t.Run("counts HEAD method", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Head: &openapi3.Operation{},
		})
		count := countEndpoints(doc)
		assert.Equal(t, 1, count)
	})

	t.Run("counts OPTIONS method", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Options: &openapi3.Operation{},
		})
		count := countEndpoints(doc)
		assert.Equal(t, 1, count)
	})

	t.Run("counts all methods", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/test", &openapi3.PathItem{
			Get:     &openapi3.Operation{},
			Post:    &openapi3.Operation{},
			Put:     &openapi3.Operation{},
			Delete:  &openapi3.Operation{},
			Patch:   &openapi3.Operation{},
			Head:    &openapi3.Operation{},
			Options: &openapi3.Operation{},
		})
		count := countEndpoints(doc)
		assert.Equal(t, 7, count)
	})

	t.Run("counts multiple paths", func(t *testing.T) {
		doc := createTestDocumentWithPaths()
		count := countEndpoints(doc)
		assert.Equal(t, 6, count) // 3 + 3 methods
	})

	t.Run("skips nil path items", func(t *testing.T) {
		doc := &openapi3.T{Paths: &openapi3.Paths{}}
		doc.Paths.Set("/valid", &openapi3.PathItem{
			Get: &openapi3.Operation{},
		})
		// Can't set nil directly, but the code handles nil PathItem
		count := countEndpoints(doc)
		assert.Equal(t, 1, count)
	})
}

func TestGenerateDefaultVersion(t *testing.T) {
	t.Run("returns non-empty string", func(t *testing.T) {
		version := GenerateDefaultVersion()
		assert.NotEmpty(t, version)
	})

	t.Run("follows expected format", func(t *testing.T) {
		version := GenerateDefaultVersion()

		// Format: YYYY.MM.DD-HHMMSS
		assert.Contains(t, version, ".")
		assert.Contains(t, version, "-")

		parts := strings.Split(version, "-")
		assert.Len(t, parts, 2)

		dateParts := strings.Split(parts[0], ".")
		assert.Len(t, dateParts, 3)

		// Year should be 4 digits
		assert.Len(t, dateParts[0], 4)
		// Month should be 2 digits
		assert.Len(t, dateParts[1], 2)
		// Day should be 2 digits
		assert.Len(t, dateParts[2], 2)
		// Time should be 6 digits (HHMMSS)
		assert.Len(t, parts[1], 6)
	})

	t.Run("generates different versions at different times", func(t *testing.T) {
		version1 := GenerateDefaultVersion()
		time.Sleep(1100 * time.Millisecond) // Wait over 1 second
		version2 := GenerateDefaultVersion()

		assert.NotEqual(t, version1, version2)
	})

	t.Run("can be parsed back to time", func(t *testing.T) {
		now := time.Now()
		version := GenerateDefaultVersion()

		// Parse in the same timezone as generation (local time)
		parsed, err := time.ParseInLocation("2006.01.02-150405", version, time.Local)
		require.NoError(t, err)

		// Should be within a few seconds of generation time
		assert.WithinDuration(t, now, parsed, 5*time.Second)
	})
}
