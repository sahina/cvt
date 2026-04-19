package cvtservice

import (
	"context"
	"os"
	"testing"

	"github.com/sahina/cvt/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanIDeploy tests deployment safety checks
func TestCanIDeploy(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register a schema first
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	regResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "can-i-deploy-schema",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	require.True(t, regResp.Success)

	t.Run("safe to deploy with no consumers", func(t *testing.T) {
		req := &pb.CanIDeployRequest{
			SchemaId:    "can-i-deploy-schema",
			NewVersion:  "2.0.0",
			Environment: "prod",
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.SafeToDeploy)
		assert.Contains(t, resp.Summary, "No consumers registered")
	})

	t.Run("missing schema_id", func(t *testing.T) {
		req := &pb.CanIDeployRequest{
			NewVersion:  "2.0.0",
			Environment: "prod",
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.SafeToDeploy)
		assert.Contains(t, resp.Summary, "schema_id is required")
	})

	t.Run("schema not found", func(t *testing.T) {
		req := &pb.CanIDeployRequest{
			SchemaId:    "non-existent-schema",
			NewVersion:  "2.0.0",
			Environment: "prod",
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.SafeToDeploy)
		assert.Contains(t, resp.Summary, "schema not found")
	})

	t.Run("with consumers registered", func(t *testing.T) {
		// Register a consumer
		_, err := service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
			ConsumerId:    "deploy-test-consumer",
			SchemaId:      "can-i-deploy-schema",
			SchemaVersion: "1.0.0",
			Environment:   "prod",
		})
		require.NoError(t, err)

		req := &pb.CanIDeployRequest{
			SchemaId:    "can-i-deploy-schema",
			NewVersion:  "2.0.0",
			Environment: "prod",
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		// With consumers, the response indicates the analysis result
		assert.NotEmpty(t, resp.Summary)
	})

	t.Run("default environment is prod", func(t *testing.T) {
		req := &pb.CanIDeployRequest{
			SchemaId:   "can-i-deploy-schema",
			NewVersion: "2.0.0",
			// No environment - should default to "prod"
		}

		resp, err := service.CanIDeploy(context.Background(), req)
		require.NoError(t, err)
		// Should work with default environment
		assert.NotNil(t, resp)
	})
}

// TestCanIDeploy_ConsumerVersionTracking tests the full consumer version tracking flow
func TestCanIDeploy_ConsumerVersionTracking(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	schemaID := "consumer-version-tracking-test"

	// Register v1 schema (info.version: "1.0.0")
	contentV1, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	respV1, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      schemaID,
		SchemaContent: string(contentV1),
	})
	require.NoError(t, err)
	require.True(t, respV1.Success)

	// Register a consumer tested against v1.0.0
	consumerResp, err := service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
		ConsumerId:    "order-service",
		SchemaId:      schemaID,
		SchemaVersion: "1.0.0",
		Environment:   "prod",
	})
	require.NoError(t, err)
	require.True(t, consumerResp.Success, "Consumer registration should succeed")

	t.Run("deploy same version is safe", func(t *testing.T) {
		resp, err := service.CanIDeploy(context.Background(), &pb.CanIDeployRequest{
			SchemaId:    schemaID,
			NewVersion:  "1.0.0",
			Environment: "prod",
		})
		require.NoError(t, err)
		assert.True(t, resp.SafeToDeploy, "Deploying same version should be safe")
	})

	t.Run("deploy new version shows consumer impact", func(t *testing.T) {
		// Register v2 schema (info.version: "2.0.0")
		contentV2, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore-v2.json")
		require.NoError(t, err)

		respV2, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
			SchemaId:      schemaID,
			SchemaContent: string(contentV2),
		})
		require.NoError(t, err)
		require.True(t, respV2.Success)

		resp, err := service.CanIDeploy(context.Background(), &pb.CanIDeployRequest{
			SchemaId:    schemaID,
			NewVersion:  "2.0.0",
			Environment: "prod",
		})
		require.NoError(t, err)
		// Consumer is on v1.0.0, deploying v2.0.0 should show impact
		assert.NotEmpty(t, resp.AffectedConsumers, "Should show consumer impact")
		assert.Equal(t, "order-service", resp.AffectedConsumers[0].ConsumerId)
		assert.Equal(t, "1.0.0", resp.AffectedConsumers[0].CurrentSchemaVersion)
	})
}

// TestFilterChangesForConsumer tests the filterChangesForConsumer helper function
func TestFilterChangesForConsumer(t *testing.T) {
	// Sample breaking changes
	changes := []*pb.BreakingChange{
		{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users", Method: "GET", Description: "GET /users removed"},
		{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users/{id}", Method: "DELETE", Description: "DELETE /users/{id} removed"},
		{Type: pb.BreakingChangeType_REQUIRED_PARAMETER_ADDED, Path: "/pets", Method: "POST", Description: "Required param added to POST /pets"},
		{Type: pb.BreakingChangeType_RESPONSE_SCHEMA_CHANGED, Path: "/orders", Method: "", Description: "Response schema changed for /orders"},
	}

	t.Run("no endpoints returns all changes (conservative)", func(t *testing.T) {
		endpoints := []EndpointUsage{}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 4, "Should return all changes when no endpoints specified")
	})

	t.Run("filters to matching endpoint", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "GET", Path: "/users"},
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 1)
		assert.Equal(t, "/users", result[0].Path)
		assert.Equal(t, "GET", result[0].Method)
	})

	t.Run("filters multiple endpoints", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/pets"},
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 2)
	})

	t.Run("matches when change has empty method (affects all methods)", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "GET", Path: "/orders"},
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 1, "Should match change with empty method")
		assert.Equal(t, "/orders", result[0].Path)
	})

	t.Run("no match returns empty", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "GET", Path: "/nonexistent"},
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 0)
	})

	t.Run("method mismatch does not match", func(t *testing.T) {
		endpoints := []EndpointUsage{
			{Method: "POST", Path: "/users"}, // Change is for GET /users
		}
		result := filterChangesForConsumer(changes, endpoints)
		assert.Len(t, result, 0, "POST /users should not match GET /users change")
	})
}
