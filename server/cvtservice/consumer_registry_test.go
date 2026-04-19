package cvtservice

import (
	"context"
	"os"
	"testing"

	"github.com/sahina/cvt/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterConsumer tests consumer registration
func TestRegisterConsumer(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// First register a schema
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	regResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "consumer-test-schema",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	require.True(t, regResp.Success)

	t.Run("register consumer successfully", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			ConsumerId:      "order-service",
			ConsumerVersion: "1.0.0",
			SchemaId:        "consumer-test-schema",
			SchemaVersion:   "1.0.0",
			Environment:     "prod",
			UsedEndpoints: []*pb.EndpointUsage{
				{Method: "GET", Path: "/pets"},
				{Method: "POST", Path: "/pets"},
			},
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.NotNil(t, resp.Consumer)
		assert.Equal(t, "order-service", resp.Consumer.ConsumerId)
		assert.Equal(t, "prod", resp.Consumer.Environment)
	})

	t.Run("register consumer with default environment", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			ConsumerId:      "payment-service",
			ConsumerVersion: "1.0.0",
			SchemaId:        "consumer-test-schema",
			SchemaVersion:   "1.0.0",
			// No environment - should default to "dev"
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "dev", resp.Consumer.Environment)
	})

	t.Run("register consumer missing consumer_id", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			SchemaId:    "consumer-test-schema",
			Environment: "prod",
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "consumer_id is required")
	})

	t.Run("register consumer missing schema_id", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			ConsumerId:  "test-consumer",
			Environment: "prod",
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "schema_id is required")
	})

	t.Run("register consumer with non-existent schema", func(t *testing.T) {
		req := &pb.RegisterConsumerRequest{
			ConsumerId:  "test-consumer",
			SchemaId:    "non-existent-schema",
			Environment: "prod",
		}

		resp, err := service.RegisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "schema not found")
	})
}

// TestListConsumers tests listing consumers for a schema
func TestListConsumers(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register a schema first
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	regResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "list-consumers-schema",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Register some consumers
	consumers := []struct {
		id  string
		env string
	}{
		{"consumer-1", "prod"},
		{"consumer-2", "prod"},
		{"consumer-3", "staging"},
	}

	for _, c := range consumers {
		_, err := service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
			ConsumerId:  c.id,
			SchemaId:    "list-consumers-schema",
			Environment: c.env,
		})
		require.NoError(t, err)
	}

	t.Run("list all consumers for schema", func(t *testing.T) {
		req := &pb.ListConsumersRequest{
			SchemaId: "list-consumers-schema",
		}

		resp, err := service.ListConsumers(context.Background(), req)
		require.NoError(t, err)
		assert.Len(t, resp.Consumers, 3)
	})

	t.Run("list consumers by environment", func(t *testing.T) {
		req := &pb.ListConsumersRequest{
			SchemaId:    "list-consumers-schema",
			Environment: "prod",
		}

		resp, err := service.ListConsumers(context.Background(), req)
		require.NoError(t, err)
		assert.Len(t, resp.Consumers, 2)

		for _, c := range resp.Consumers {
			assert.Equal(t, "prod", c.Environment)
		}
	})

	t.Run("list consumers with empty schema_id", func(t *testing.T) {
		req := &pb.ListConsumersRequest{
			SchemaId: "",
		}

		resp, err := service.ListConsumers(context.Background(), req)
		require.NoError(t, err)
		assert.Empty(t, resp.Consumers)
	})
}

// TestDeregisterConsumer tests consumer deregistration
func TestDeregisterConsumer(t *testing.T) {
	service, err := NewValidatorService()
	require.NoError(t, err)
	defer service.Close()

	// Register a schema first
	content, err := os.ReadFile("testdata/openapi-v3/valid/simple-petstore.json")
	require.NoError(t, err)

	regResp, err := service.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "deregister-test-schema",
		SchemaContent: string(content),
	})
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Register a consumer
	_, err = service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
		ConsumerId:  "to-be-removed",
		SchemaId:    "deregister-test-schema",
		Environment: "prod",
	})
	require.NoError(t, err)

	t.Run("deregister existing consumer", func(t *testing.T) {
		req := &pb.DeregisterConsumerRequest{
			ConsumerId:  "to-be-removed",
			SchemaId:    "deregister-test-schema",
			Environment: "prod",
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("deregister non-existent consumer", func(t *testing.T) {
		req := &pb.DeregisterConsumerRequest{
			ConsumerId:  "non-existent",
			SchemaId:    "deregister-test-schema",
			Environment: "prod",
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "consumer not found")
	})

	t.Run("deregister with missing consumer_id", func(t *testing.T) {
		req := &pb.DeregisterConsumerRequest{
			SchemaId:    "deregister-test-schema",
			Environment: "prod",
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "consumer_id is required")
	})

	t.Run("deregister with missing schema_id", func(t *testing.T) {
		req := &pb.DeregisterConsumerRequest{
			ConsumerId:  "some-consumer",
			Environment: "prod",
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Message, "schema_id is required")
	})

	t.Run("deregister with default environment", func(t *testing.T) {
		// First register a consumer in dev environment
		_, err := service.RegisterConsumer(context.Background(), &pb.RegisterConsumerRequest{
			ConsumerId:  "dev-consumer",
			SchemaId:    "deregister-test-schema",
			Environment: "dev",
		})
		require.NoError(t, err)

		// Deregister without specifying environment (defaults to dev)
		req := &pb.DeregisterConsumerRequest{
			ConsumerId: "dev-consumer",
			SchemaId:   "deregister-test-schema",
			// No environment - should default to "dev"
		}

		resp, err := service.DeregisterConsumer(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})
}
