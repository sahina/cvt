package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cvt/cvt/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestSchemaRecord(schemaID, version string) *SchemaRecord {
	return &SchemaRecord{
		SchemaID:       schemaID,
		Version:        version,
		Content:        `{"openapi":"3.0.0"}`,
		ContentHash:    "abc123",
		OpenAPIVersion: "3.0.0",
		EndpointCount:  5,
		RegisteredAt:   time.Now(),
		UpdatedAt:      time.Now(),
		Ownership: &pb.SchemaOwnership{
			Owner: "test-owner",
			Team:  "test-team",
		},
		Environment: "test",
	}
}

func createTestValidationRecord(schemaID string, valid bool) *ValidationRecord {
	return &ValidationRecord{
		SchemaID:       schemaID,
		SchemaVersion:  "1.0.0",
		RequestMethod:  "GET",
		RequestPath:    "/users",
		ResponseStatus: 200,
		Valid:          valid,
		ValidatedAt:    time.Now(),
		Environment:    "test",
	}
}

func createTestConsumerRecord(consumerID, schemaID, environment string) *ConsumerRecord {
	return &ConsumerRecord{
		ConsumerID:      consumerID,
		ConsumerVersion: "1.0.0",
		SchemaID:        schemaID,
		SchemaVersion:   "1.0.0",
		Environment:     environment,
		RegisteredAt:    time.Now(),
		LastValidatedAt: time.Now(),
	}
}

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	require.NotNil(t, store)
	assert.NotNil(t, store.schemas)
	assert.NotNil(t, store.validations)
	assert.NotNil(t, store.comparisons)
	assert.NotNil(t, store.consumers)
}

func TestMemoryStore_Lifecycle(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("Migrate", func(t *testing.T) {
		err := store.Migrate(ctx)
		assert.NoError(t, err)
	})

	t.Run("Ping", func(t *testing.T) {
		err := store.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("Close", func(t *testing.T) {
		err := store.Close()
		assert.NoError(t, err)
	})
}

func TestMemoryStore_SetSchema(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("basic set", func(t *testing.T) {
		record := createTestSchemaRecord("test-schema", "1.0.0")
		err := store.SetSchema(ctx, record)
		require.NoError(t, err)
		assert.NotEmpty(t, record.ID)
		assert.True(t, record.IsLatest)
	})

	t.Run("auto generates ID", func(t *testing.T) {
		record := createTestSchemaRecord("auto-id-schema", "1.0.0")
		record.ID = ""
		err := store.SetSchema(ctx, record)
		require.NoError(t, err)
		assert.NotEmpty(t, record.ID)
	})

	t.Run("preserves provided ID", func(t *testing.T) {
		record := createTestSchemaRecord("preserve-id-schema", "1.0.0")
		record.ID = "custom-id-123"
		err := store.SetSchema(ctx, record)
		require.NoError(t, err)
		assert.Equal(t, "custom-id-123", record.ID)
	})

	t.Run("version management marks previous as not latest", func(t *testing.T) {
		schemaID := "versioned-schema"
		v1 := createTestSchemaRecord(schemaID, "1.0.0")
		v2 := createTestSchemaRecord(schemaID, "2.0.0")

		err := store.SetSchema(ctx, v1)
		require.NoError(t, err)
		assert.True(t, v1.IsLatest)

		err = store.SetSchema(ctx, v2)
		require.NoError(t, err)
		assert.True(t, v2.IsLatest)
		assert.False(t, v1.IsLatest)
	})
}

func TestMemoryStore_GetSchema(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("get latest version", func(t *testing.T) {
		schemaID := "get-test-schema"
		v1 := createTestSchemaRecord(schemaID, "1.0.0")
		v2 := createTestSchemaRecord(schemaID, "2.0.0")

		require.NoError(t, store.SetSchema(ctx, v1))
		require.NoError(t, store.SetSchema(ctx, v2))

		result, err := store.GetSchema(ctx, schemaID)
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", result.Version)
		assert.True(t, result.IsLatest)
	})

	t.Run("not found returns error", func(t *testing.T) {
		_, err := store.GetSchema(ctx, "non-existent")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})
}

func TestMemoryStore_GetSchemaVersion(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	schemaID := "version-test-schema"
	v1 := createTestSchemaRecord(schemaID, "1.0.0")
	v2 := createTestSchemaRecord(schemaID, "2.0.0")
	require.NoError(t, store.SetSchema(ctx, v1))
	require.NoError(t, store.SetSchema(ctx, v2))

	t.Run("get specific version", func(t *testing.T) {
		result, err := store.GetSchemaVersion(ctx, schemaID, "1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", result.Version)
	})

	t.Run("schema not found", func(t *testing.T) {
		_, err := store.GetSchemaVersion(ctx, "non-existent", "1.0.0")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})

	t.Run("version not found", func(t *testing.T) {
		_, err := store.GetSchemaVersion(ctx, schemaID, "99.0.0")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})
}

func TestMemoryStore_DeleteSchema(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("delete existing schema", func(t *testing.T) {
		schemaID := "delete-test-schema"
		record := createTestSchemaRecord(schemaID, "1.0.0")
		require.NoError(t, store.SetSchema(ctx, record))

		err := store.DeleteSchema(ctx, schemaID)
		require.NoError(t, err)

		_, err = store.GetSchema(ctx, schemaID)
		assert.True(t, IsNotFound(err))
	})

	t.Run("delete non-existent schema returns error", func(t *testing.T) {
		err := store.DeleteSchema(ctx, "non-existent")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})

	t.Run("delete removes all versions", func(t *testing.T) {
		schemaID := "multi-version-delete"
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "1.0.0")))
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "2.0.0")))

		err := store.DeleteSchema(ctx, schemaID)
		require.NoError(t, err)

		_, err = store.GetSchemaVersion(ctx, schemaID, "1.0.0")
		assert.True(t, IsNotFound(err))
		_, err = store.GetSchemaVersion(ctx, schemaID, "2.0.0")
		assert.True(t, IsNotFound(err))
	})
}

func TestMemoryStore_DeleteSchemaVersion(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("delete specific version", func(t *testing.T) {
		schemaID := "version-delete-test"
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "1.0.0")))
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "2.0.0")))

		err := store.DeleteSchemaVersion(ctx, schemaID, "1.0.0")
		require.NoError(t, err)

		_, err = store.GetSchemaVersion(ctx, schemaID, "1.0.0")
		assert.True(t, IsNotFound(err))

		result, err := store.GetSchemaVersion(ctx, schemaID, "2.0.0")
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", result.Version)
	})

	t.Run("schema not found", func(t *testing.T) {
		err := store.DeleteSchemaVersion(ctx, "non-existent", "1.0.0")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})

	t.Run("version not found", func(t *testing.T) {
		schemaID := "version-not-found-test"
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "1.0.0")))

		err := store.DeleteSchemaVersion(ctx, schemaID, "99.0.0")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})
}

func TestMemoryStore_ListSchemaIDs(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("empty store", func(t *testing.T) {
		ids, err := store.ListSchemaIDs(ctx)
		require.NoError(t, err)
		assert.Empty(t, ids)
	})

	t.Run("returns sorted IDs", func(t *testing.T) {
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord("zebra", "1.0.0")))
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord("alpha", "1.0.0")))
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord("beta", "1.0.0")))

		ids, err := store.ListSchemaIDs(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"alpha", "beta", "zebra"}, ids)
	})
}

func TestMemoryStore_ListVersions(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("non-existent schema returns nil", func(t *testing.T) {
		versions, err := store.ListVersions(ctx, "non-existent")
		require.NoError(t, err)
		assert.Nil(t, versions)
	})

	t.Run("returns all versions", func(t *testing.T) {
		schemaID := "list-versions-test"
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "1.0.0")))
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "2.0.0")))
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "3.0.0")))

		versions, err := store.ListVersions(ctx, schemaID)
		require.NoError(t, err)
		assert.Len(t, versions, 3)
		assert.Contains(t, versions, "1.0.0")
		assert.Contains(t, versions, "2.0.0")
		assert.Contains(t, versions, "3.0.0")
	})
}

func TestMemoryStore_ListSchemas(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Set up test data
	s1 := createTestSchemaRecord("schema-1", "1.0.0")
	s1.Ownership = &pb.SchemaOwnership{Owner: "owner-a", Team: "team-x"}
	s1.Environment = "dev"

	s2 := createTestSchemaRecord("schema-2", "1.0.0")
	s2.Ownership = &pb.SchemaOwnership{Owner: "owner-b", Team: "team-x"}
	s2.Environment = "prod"

	s3 := createTestSchemaRecord("schema-3", "1.0.0")
	s3.Ownership = &pb.SchemaOwnership{Owner: "owner-a", Team: "team-y"}
	s3.Environment = "prod"

	require.NoError(t, store.SetSchema(ctx, s1))
	require.NoError(t, store.SetSchema(ctx, s2))
	require.NoError(t, store.SetSchema(ctx, s3))

	t.Run("no filter returns all latest", func(t *testing.T) {
		schemas, _, total, err := store.ListSchemas(ctx, ListSchemasFilter{})
		require.NoError(t, err)
		assert.Len(t, schemas, 3)
		assert.Equal(t, int32(3), total)
	})

	t.Run("filter by owner", func(t *testing.T) {
		schemas, _, _, err := store.ListSchemas(ctx, ListSchemasFilter{Owner: "owner-a"})
		require.NoError(t, err)
		assert.Len(t, schemas, 2)
	})

	t.Run("filter by team", func(t *testing.T) {
		schemas, _, _, err := store.ListSchemas(ctx, ListSchemasFilter{Team: "team-x"})
		require.NoError(t, err)
		assert.Len(t, schemas, 2)
	})

	t.Run("filter by environment", func(t *testing.T) {
		schemas, _, _, err := store.ListSchemas(ctx, ListSchemasFilter{Environment: "prod"})
		require.NoError(t, err)
		assert.Len(t, schemas, 2)
	})

	t.Run("combined filters", func(t *testing.T) {
		schemas, _, _, err := store.ListSchemas(ctx, ListSchemasFilter{
			Owner:       "owner-a",
			Environment: "prod",
		})
		require.NoError(t, err)
		assert.Len(t, schemas, 1)
		assert.Equal(t, "schema-3", schemas[0].SchemaID)
	})

	t.Run("only returns latest versions", func(t *testing.T) {
		schemaID := "multi-version-list"
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "1.0.0")))
		require.NoError(t, store.SetSchema(ctx, createTestSchemaRecord(schemaID, "2.0.0")))

		schemas, _, _, err := store.ListSchemas(ctx, ListSchemasFilter{})
		require.NoError(t, err)

		count := 0
		for _, s := range schemas {
			if s.SchemaID == schemaID {
				count++
				assert.True(t, s.IsLatest)
			}
		}
		assert.Equal(t, 1, count)
	})
}

func TestMemoryStore_GetPreviousVersion(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	schemaID := "previous-version-test"
	v1 := createTestSchemaRecord(schemaID, "1.0.0")
	v1.RegisteredAt = time.Now().Add(-2 * time.Hour)

	v2 := createTestSchemaRecord(schemaID, "2.0.0")
	v2.RegisteredAt = time.Now().Add(-1 * time.Hour)

	v3 := createTestSchemaRecord(schemaID, "3.0.0")
	v3.RegisteredAt = time.Now()

	require.NoError(t, store.SetSchema(ctx, v1))
	require.NoError(t, store.SetSchema(ctx, v2))
	require.NoError(t, store.SetSchema(ctx, v3))

	t.Run("get previous version", func(t *testing.T) {
		prev, err := store.GetPreviousVersion(ctx, schemaID, "3.0.0")
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", prev)

		prev, err = store.GetPreviousVersion(ctx, schemaID, "2.0.0")
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", prev)
	})

	t.Run("no previous version", func(t *testing.T) {
		_, err := store.GetPreviousVersion(ctx, schemaID, "1.0.0")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})

	t.Run("schema not found", func(t *testing.T) {
		_, err := store.GetPreviousVersion(ctx, "non-existent", "1.0.0")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})

	t.Run("version not found", func(t *testing.T) {
		_, err := store.GetPreviousVersion(ctx, schemaID, "99.0.0")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})
}

func TestMemoryStore_RecordValidation(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("record validation", func(t *testing.T) {
		record := createTestValidationRecord("test-schema", true)
		err := store.RecordValidation(ctx, record)
		require.NoError(t, err)
		assert.NotEmpty(t, record.ID)
	})

	t.Run("auto generates ID", func(t *testing.T) {
		record := createTestValidationRecord("test-schema", true)
		record.ID = ""
		err := store.RecordValidation(ctx, record)
		require.NoError(t, err)
		assert.NotEmpty(t, record.ID)
	})

	t.Run("preserves provided ID", func(t *testing.T) {
		record := createTestValidationRecord("test-schema", true)
		record.ID = "custom-validation-id"
		err := store.RecordValidation(ctx, record)
		require.NoError(t, err)
		assert.Equal(t, "custom-validation-id", record.ID)
	})
}

func TestMemoryStore_ListValidations(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Set up test data
	now := time.Now()

	v1 := &ValidationRecord{
		SchemaID:      "schema-a",
		RequestMethod: "GET",
		Valid:         true,
		ValidatedAt:   now.Add(-2 * time.Hour),
		Environment:   "dev",
	}
	v2 := &ValidationRecord{
		SchemaID:      "schema-a",
		RequestMethod: "POST",
		Valid:         false,
		ValidatedAt:   now.Add(-1 * time.Hour),
		Environment:   "prod",
	}
	v3 := &ValidationRecord{
		SchemaID:      "schema-b",
		RequestMethod: "GET",
		Valid:         true,
		ValidatedAt:   now,
		Environment:   "prod",
	}

	require.NoError(t, store.RecordValidation(ctx, v1))
	require.NoError(t, store.RecordValidation(ctx, v2))
	require.NoError(t, store.RecordValidation(ctx, v3))

	t.Run("no filter returns all", func(t *testing.T) {
		results, _, err := store.ListValidations(ctx, ListValidationsFilter{})
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("filter by schema ID", func(t *testing.T) {
		results, _, err := store.ListValidations(ctx, ListValidationsFilter{SchemaID: "schema-a"})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by method", func(t *testing.T) {
		results, _, err := store.ListValidations(ctx, ListValidationsFilter{Method: "GET"})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by environment", func(t *testing.T) {
		results, _, err := store.ListValidations(ctx, ListValidationsFilter{Environment: "prod"})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by valid", func(t *testing.T) {
		valid := true
		results, _, err := store.ListValidations(ctx, ListValidationsFilter{Valid: &valid})
		require.NoError(t, err)
		assert.Len(t, results, 2)

		invalid := false
		results, _, err = store.ListValidations(ctx, ListValidationsFilter{Valid: &invalid})
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})

	t.Run("filter by time range", func(t *testing.T) {
		results, _, err := store.ListValidations(ctx, ListValidationsFilter{
			StartTime: now.Add(-90 * time.Minute),
			EndTime:   now.Add(-30 * time.Minute),
		})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "schema-a", results[0].SchemaID)
		assert.Equal(t, "POST", results[0].RequestMethod)
	})

	t.Run("respects page size", func(t *testing.T) {
		results, _, err := store.ListValidations(ctx, ListValidationsFilter{PageSize: 2})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("default page size is 100", func(t *testing.T) {
		// Add more validations to exceed default
		for i := 0; i < 50; i++ {
			require.NoError(t, store.RecordValidation(ctx, createTestValidationRecord("bulk-schema", true)))
		}

		results, _, err := store.ListValidations(ctx, ListValidationsFilter{})
		require.NoError(t, err)
		assert.LessOrEqual(t, len(results), 100)
	})
}

func TestMemoryStore_GetValidationAnalytics(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	now := time.Now()

	// Add test validations
	validations := []*ValidationRecord{
		{SchemaID: "schema-a", RequestMethod: "GET", Valid: true, ValidatedAt: now},
		{SchemaID: "schema-a", RequestMethod: "POST", Valid: true, ValidatedAt: now},
		{SchemaID: "schema-a", RequestMethod: "GET", Valid: false, ValidatedAt: now},
		{SchemaID: "schema-b", RequestMethod: "GET", Valid: true, ValidatedAt: now},
	}

	for _, v := range validations {
		require.NoError(t, store.RecordValidation(ctx, v))
	}

	t.Run("basic analytics", func(t *testing.T) {
		analytics, err := store.GetValidationAnalytics(ctx, ListValidationsFilter{})
		require.NoError(t, err)

		assert.Equal(t, int64(4), analytics.TotalValidations)
		assert.Equal(t, int64(3), analytics.PassCount)
		assert.Equal(t, int64(1), analytics.FailCount)
		assert.Equal(t, 75.0, analytics.PassRate)
	})

	t.Run("by method counts", func(t *testing.T) {
		analytics, err := store.GetValidationAnalytics(ctx, ListValidationsFilter{})
		require.NoError(t, err)

		assert.Equal(t, int64(3), analytics.ByMethod["GET"])
		assert.Equal(t, int64(1), analytics.ByMethod["POST"])
	})

	t.Run("by schema counts", func(t *testing.T) {
		analytics, err := store.GetValidationAnalytics(ctx, ListValidationsFilter{})
		require.NoError(t, err)

		assert.Equal(t, int64(3), analytics.BySchema["schema-a"])
		assert.Equal(t, int64(1), analytics.BySchema["schema-b"])
	})

	t.Run("filter by schema", func(t *testing.T) {
		analytics, err := store.GetValidationAnalytics(ctx, ListValidationsFilter{SchemaID: "schema-a"})
		require.NoError(t, err)

		assert.Equal(t, int64(3), analytics.TotalValidations)
	})

	t.Run("filter by time range", func(t *testing.T) {
		oldValidation := &ValidationRecord{
			SchemaID:      "old-schema",
			RequestMethod: "GET",
			Valid:         true,
			ValidatedAt:   now.Add(-24 * time.Hour),
		}
		require.NoError(t, store.RecordValidation(ctx, oldValidation))

		analytics, err := store.GetValidationAnalytics(ctx, ListValidationsFilter{
			StartTime: now.Add(-1 * time.Hour),
		})
		require.NoError(t, err)
		assert.Equal(t, int64(4), analytics.TotalValidations)
	})

	t.Run("empty results", func(t *testing.T) {
		analytics, err := store.GetValidationAnalytics(ctx, ListValidationsFilter{SchemaID: "non-existent"})
		require.NoError(t, err)
		assert.Equal(t, int64(0), analytics.TotalValidations)
		assert.Equal(t, float64(0), analytics.PassRate)
	})
}

func TestMemoryStore_RecordComparison(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("record comparison", func(t *testing.T) {
		record := &ComparisonRecord{
			SchemaID:   "test-schema",
			OldVersion: "1.0.0",
			NewVersion: "2.0.0",
			Compatible: true,
			ComparedAt: time.Now(),
		}
		err := store.RecordComparison(ctx, record)
		require.NoError(t, err)
		assert.NotEmpty(t, record.ID)
	})

	t.Run("auto generates ID", func(t *testing.T) {
		record := &ComparisonRecord{
			SchemaID:   "auto-id-schema",
			OldVersion: "1.0.0",
			NewVersion: "2.0.0",
		}
		err := store.RecordComparison(ctx, record)
		require.NoError(t, err)
		assert.NotEmpty(t, record.ID)
	})

	t.Run("upsert behavior", func(t *testing.T) {
		record1 := &ComparisonRecord{
			SchemaID:   "upsert-schema",
			OldVersion: "1.0.0",
			NewVersion: "2.0.0",
			Compatible: true,
		}
		record2 := &ComparisonRecord{
			SchemaID:   "upsert-schema",
			OldVersion: "1.0.0",
			NewVersion: "2.0.0",
			Compatible: false,
		}

		require.NoError(t, store.RecordComparison(ctx, record1))
		require.NoError(t, store.RecordComparison(ctx, record2))

		result, err := store.GetComparison(ctx, "upsert-schema", "1.0.0", "2.0.0")
		require.NoError(t, err)
		assert.False(t, result.Compatible)
	})
}

func TestMemoryStore_GetComparison(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	record := &ComparisonRecord{
		SchemaID:   "get-comparison-schema",
		OldVersion: "1.0.0",
		NewVersion: "2.0.0",
		Compatible: true,
	}
	require.NoError(t, store.RecordComparison(ctx, record))

	t.Run("get existing comparison", func(t *testing.T) {
		result, err := store.GetComparison(ctx, "get-comparison-schema", "1.0.0", "2.0.0")
		require.NoError(t, err)
		assert.True(t, result.Compatible)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetComparison(ctx, "non-existent", "1.0.0", "2.0.0")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})
}

func TestMemoryStore_RegisterConsumer(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("register new consumer", func(t *testing.T) {
		record := createTestConsumerRecord("consumer-1", "schema-1", "prod")
		err := store.RegisterConsumer(ctx, record)
		require.NoError(t, err)
		assert.NotEmpty(t, record.ID)
	})

	t.Run("auto generates ID", func(t *testing.T) {
		record := createTestConsumerRecord("consumer-2", "schema-1", "prod")
		record.ID = ""
		err := store.RegisterConsumer(ctx, record)
		require.NoError(t, err)
		assert.NotEmpty(t, record.ID)
	})

	t.Run("upsert preserves registration time", func(t *testing.T) {
		originalTime := time.Now().Add(-1 * time.Hour)
		record1 := createTestConsumerRecord("consumer-upsert", "schema-1", "prod")
		record1.RegisteredAt = originalTime

		require.NoError(t, store.RegisterConsumer(ctx, record1))

		record2 := createTestConsumerRecord("consumer-upsert", "schema-1", "prod")
		record2.RegisteredAt = time.Now()

		require.NoError(t, store.RegisterConsumer(ctx, record2))

		result, err := store.GetConsumer(ctx, "consumer-upsert", "schema-1", "prod")
		require.NoError(t, err)
		assert.Equal(t, originalTime.Unix(), result.RegisteredAt.Unix())
	})
}

func TestMemoryStore_GetConsumer(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	record := createTestConsumerRecord("get-consumer", "schema-1", "prod")
	require.NoError(t, store.RegisterConsumer(ctx, record))

	t.Run("get existing consumer", func(t *testing.T) {
		result, err := store.GetConsumer(ctx, "get-consumer", "schema-1", "prod")
		require.NoError(t, err)
		assert.Equal(t, "get-consumer", result.ConsumerID)
		assert.Equal(t, "prod", result.Environment)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetConsumer(ctx, "non-existent", "schema-1", "prod")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})

	t.Run("different environment", func(t *testing.T) {
		_, err := store.GetConsumer(ctx, "get-consumer", "schema-1", "dev")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})
}

func TestMemoryStore_ListConsumers(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Set up test data
	c1 := createTestConsumerRecord("consumer-1", "schema-a", "prod")
	c2 := createTestConsumerRecord("consumer-2", "schema-a", "dev")
	c3 := createTestConsumerRecord("consumer-3", "schema-b", "prod")

	require.NoError(t, store.RegisterConsumer(ctx, c1))
	require.NoError(t, store.RegisterConsumer(ctx, c2))
	require.NoError(t, store.RegisterConsumer(ctx, c3))

	t.Run("no filter returns all", func(t *testing.T) {
		results, err := store.ListConsumers(ctx, ListConsumersFilter{})
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("filter by schema ID", func(t *testing.T) {
		results, err := store.ListConsumers(ctx, ListConsumersFilter{SchemaID: "schema-a"})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by environment", func(t *testing.T) {
		results, err := store.ListConsumers(ctx, ListConsumersFilter{Environment: "prod"})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by consumer ID", func(t *testing.T) {
		results, err := store.ListConsumers(ctx, ListConsumersFilter{ConsumerID: "consumer-1"})
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})

	t.Run("combined filters", func(t *testing.T) {
		results, err := store.ListConsumers(ctx, ListConsumersFilter{
			SchemaID:    "schema-a",
			Environment: "prod",
		})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "consumer-1", results[0].ConsumerID)
	})
}

func TestMemoryStore_DeregisterConsumer(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("deregister existing consumer", func(t *testing.T) {
		record := createTestConsumerRecord("to-deregister", "schema-1", "prod")
		require.NoError(t, store.RegisterConsumer(ctx, record))

		err := store.DeregisterConsumer(ctx, "to-deregister", "schema-1", "prod")
		require.NoError(t, err)

		_, err = store.GetConsumer(ctx, "to-deregister", "schema-1", "prod")
		assert.True(t, IsNotFound(err))
	})

	t.Run("not found", func(t *testing.T) {
		err := store.DeregisterConsumer(ctx, "non-existent", "schema-1", "prod")
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})
}

func TestMemoryStore_UpdateConsumerValidation(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("update validation time", func(t *testing.T) {
		record := createTestConsumerRecord("update-validation", "schema-1", "prod")
		record.LastValidatedAt = time.Now().Add(-1 * time.Hour)
		require.NoError(t, store.RegisterConsumer(ctx, record))

		newTime := time.Now()
		err := store.UpdateConsumerValidation(ctx, "update-validation", "schema-1", "prod", newTime)
		require.NoError(t, err)

		result, err := store.GetConsumer(ctx, "update-validation", "schema-1", "prod")
		require.NoError(t, err)
		assert.Equal(t, newTime.Unix(), result.LastValidatedAt.Unix())
	})

	t.Run("not found", func(t *testing.T) {
		err := store.UpdateConsumerValidation(ctx, "non-existent", "schema-1", "prod", time.Now())
		assert.Error(t, err)
		assert.True(t, IsNotFound(err))
	})
}

func TestErrNotFound(t *testing.T) {
	t.Run("error message", func(t *testing.T) {
		err := &ErrNotFound{Resource: "schema", ID: "test-123"}
		assert.Equal(t, "schema not found: test-123", err.Error())
	})

	t.Run("IsNotFound returns true for ErrNotFound", func(t *testing.T) {
		err := &ErrNotFound{Resource: "schema", ID: "test"}
		assert.True(t, IsNotFound(err))
	})

	t.Run("IsNotFound returns false for other errors", func(t *testing.T) {
		err := assert.AnError
		assert.False(t, IsNotFound(err))
	})

	t.Run("IsNotFound returns false for nil", func(t *testing.T) {
		assert.False(t, IsNotFound(nil))
	})
}

func TestMemoryStore_Concurrency(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t.Run("concurrent schema writes", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				record := createTestSchemaRecord("concurrent-schema", "1.0.0")
				_ = store.SetSchema(ctx, record)
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		schemaID := "concurrent-rw-schema"
		record := createTestSchemaRecord(schemaID, "1.0.0")
		require.NoError(t, store.SetSchema(ctx, record))

		var wg sync.WaitGroup
		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(2)

			go func() {
				defer wg.Done()
				_, _ = store.GetSchema(ctx, schemaID)
			}()

			go func(idx int) {
				defer wg.Done()
				record := createTestSchemaRecord(schemaID, "2.0.0")
				_ = store.SetSchema(ctx, record)
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent validation recording", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				record := createTestValidationRecord("concurrent-schema", true)
				_ = store.RecordValidation(ctx, record)
			}()
		}

		wg.Wait()

		// Verify all validations were recorded
		results, _, err := store.ListValidations(ctx, ListValidationsFilter{SchemaID: "concurrent-schema"})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), numGoroutines)
	})
}

func TestConsumerKey(t *testing.T) {
	key := consumerKey("my-consumer", "schema-1", "prod")
	assert.Equal(t, "my-consumer/schema-1/prod", key)
}
