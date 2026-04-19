package cvtservice

import (
	"context"
	"fmt"
	"time"

	"github.com/sahina/cvt/server/pb"
	"github.com/sahina/cvt/server/storage"
	"go.uber.org/zap"
)

// RegisterConsumer registers a consumer's dependency on a schema.
// This allows tracking which consumers depend on which schemas, enabling
// deployment safety checks via CanIDeploy.
func (s *ValidatorService) RegisterConsumer(ctx context.Context, req *pb.RegisterConsumerRequest) (*pb.RegisterConsumerResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("RegisterConsumer").Observe(time.Since(start).Seconds())
	}()

	Info("Received RegisterConsumer request",
		zap.String("consumerId", req.ConsumerId),
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", req.Environment))

	// Validate request
	if req.ConsumerId == "" {
		grpcRequestsTotal.WithLabelValues("RegisterConsumer", "failure").Inc()
		return &pb.RegisterConsumerResponse{
			Success: false,
			Message: "consumer_id is required",
		}, nil
	}
	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("RegisterConsumer", "failure").Inc()
		return &pb.RegisterConsumerResponse{
			Success: false,
			Message: "schema_id is required",
		}, nil
	}
	if req.Environment == "" {
		req.Environment = "dev" // Default to dev
	}

	// Verify the schema exists (cache or storage)
	_, found := s.getSchemaEntry(ctx, req.SchemaId, "")
	if !found {
		grpcRequestsTotal.WithLabelValues("RegisterConsumer", "failure").Inc()
		return &pb.RegisterConsumerResponse{
			Success: false,
			Message: fmt.Sprintf("schema not found: %s", req.SchemaId),
		}, nil
	}

	// Check consumer cap per schema (soft cap — not atomic with registration,
	// so may be briefly exceeded under high concurrency; acceptable at 10K limit)
	existingConsumers := s.cache.ListConsumers(req.SchemaId, "")
	if len(existingConsumers) >= MaxConsumersPerSchema {
		grpcRequestsTotal.WithLabelValues("RegisterConsumer", "failure").Inc()
		return &pb.RegisterConsumerResponse{
			Success: false,
			Message: fmt.Sprintf("maximum consumers per schema reached (%d)", MaxConsumersPerSchema),
		}, nil
	}

	// Convert endpoint usage from proto to storage format
	usedEndpoints := make([]EndpointUsage, len(req.UsedEndpoints))
	for i, eu := range req.UsedEndpoints {
		usedEndpoints[i] = EndpointUsage{
			Method:     eu.Method,
			Path:       eu.Path,
			UsedFields: eu.UsedFields,
		}
	}

	now := time.Now()

	// Register in cache
	consumer := &ConsumerEntry{
		ConsumerID:      req.ConsumerId,
		ConsumerVersion: req.ConsumerVersion,
		SchemaID:        req.SchemaId,
		SchemaVersion:   req.SchemaVersion,
		Environment:     req.Environment,
		RegisteredAt:    now,
		LastValidatedAt: now,
		UsedEndpoints:   usedEndpoints,
	}
	s.cache.RegisterConsumer(consumer)

	// Persist to storage if available
	if s.store != nil {
		record := &storage.ConsumerRecord{
			ConsumerID:      req.ConsumerId,
			ConsumerVersion: req.ConsumerVersion,
			SchemaID:        req.SchemaId,
			SchemaVersion:   req.SchemaVersion,
			Environment:     req.Environment,
			RegisteredAt:    now,
			LastValidatedAt: now,
		}
		for _, eu := range usedEndpoints {
			record.UsedEndpoints = append(record.UsedEndpoints, storage.EndpointUsage{
				Method:     eu.Method,
				Path:       eu.Path,
				UsedFields: eu.UsedFields,
			})
		}
		if storeErr := s.store.RegisterConsumer(ctx, record); storeErr != nil {
			Warn("Failed to persist consumer to storage",
				zap.String("consumerId", req.ConsumerId),
				zap.Error(storeErr))
		}
	}

	Info("Consumer registered successfully",
		zap.String("consumerId", req.ConsumerId),
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", req.Environment))

	// Fire register_consumer_usage hook to notify a configured registry
	// plugin (e.g., Central API Registry) that this consumer depends on
	// the given schema + endpoints. Per locked decision, hook fires after
	// cache write succeeds regardless of storage outcome — matches existing
	// RegisterConsumer semantics where storage failures log a warning but
	// don't fail the RPC.
	s.fireRegisterConsumerUsage(ctx, req)

	grpcRequestsTotal.WithLabelValues("RegisterConsumer", "success").Inc()

	// Convert endpoint usage to proto format for response
	protoEndpoints := make([]*pb.EndpointUsage, len(usedEndpoints))
	for i, eu := range usedEndpoints {
		protoEndpoints[i] = &pb.EndpointUsage{
			Method:     eu.Method,
			Path:       eu.Path,
			UsedFields: eu.UsedFields,
		}
	}

	return &pb.RegisterConsumerResponse{
		Success: true,
		Message: "Consumer registered successfully",
		Consumer: &pb.ConsumerInfo{
			ConsumerId:      req.ConsumerId,
			ConsumerVersion: req.ConsumerVersion,
			SchemaId:        req.SchemaId,
			SchemaVersion:   req.SchemaVersion,
			Environment:     req.Environment,
			RegisteredAt:    now.Unix(),
			LastValidatedAt: now.Unix(),
			UsedEndpoints:   protoEndpoints,
		},
	}, nil
}

// ListConsumers returns all consumers that depend on a schema.
func (s *ValidatorService) ListConsumers(ctx context.Context, req *pb.ListConsumersRequest) (*pb.ListConsumersResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("ListConsumers").Observe(time.Since(start).Seconds())
	}()

	Debug("Received ListConsumers request",
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", req.Environment))

	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("ListConsumers", "failure").Inc()
		return &pb.ListConsumersResponse{}, nil
	}

	consumers := s.cache.ListConsumers(req.SchemaId, req.Environment)

	protoConsumers := make([]*pb.ConsumerInfo, len(consumers))
	for i, c := range consumers {
		protoEndpoints := make([]*pb.EndpointUsage, len(c.UsedEndpoints))
		for j, eu := range c.UsedEndpoints {
			protoEndpoints[j] = &pb.EndpointUsage{
				Method:     eu.Method,
				Path:       eu.Path,
				UsedFields: eu.UsedFields,
			}
		}
		protoConsumers[i] = &pb.ConsumerInfo{
			ConsumerId:      c.ConsumerID,
			ConsumerVersion: c.ConsumerVersion,
			SchemaId:        c.SchemaID,
			SchemaVersion:   c.SchemaVersion,
			Environment:     c.Environment,
			RegisteredAt:    c.RegisteredAt.Unix(),
			LastValidatedAt: c.LastValidatedAt.Unix(),
			UsedEndpoints:   protoEndpoints,
		}
	}

	grpcRequestsTotal.WithLabelValues("ListConsumers", "success").Inc()

	return &pb.ListConsumersResponse{
		Consumers: protoConsumers,
	}, nil
}

// DeregisterConsumer removes a consumer registration.
func (s *ValidatorService) DeregisterConsumer(ctx context.Context, req *pb.DeregisterConsumerRequest) (*pb.DeregisterConsumerResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("DeregisterConsumer").Observe(time.Since(start).Seconds())
	}()

	Info("Received DeregisterConsumer request",
		zap.String("consumerId", req.ConsumerId),
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", req.Environment))

	if req.ConsumerId == "" {
		grpcRequestsTotal.WithLabelValues("DeregisterConsumer", "failure").Inc()
		return &pb.DeregisterConsumerResponse{
			Success: false,
			Message: "consumer_id is required",
		}, nil
	}

	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("DeregisterConsumer", "failure").Inc()
		return &pb.DeregisterConsumerResponse{
			Success: false,
			Message: "schema_id is required",
		}, nil
	}

	environment := req.Environment
	if environment == "" {
		environment = "dev"
	}

	removed := s.cache.DeregisterConsumer(req.ConsumerId, req.SchemaId, environment)
	if !removed {
		grpcRequestsTotal.WithLabelValues("DeregisterConsumer", "failure").Inc()
		return &pb.DeregisterConsumerResponse{
			Success: false,
			Message: fmt.Sprintf("consumer not found: %s/%s/%s", req.ConsumerId, req.SchemaId, environment),
		}, nil
	}

	// Remove from storage if available
	if s.store != nil {
		if storeErr := s.store.DeregisterConsumer(ctx, req.ConsumerId, req.SchemaId, environment); storeErr != nil {
			Warn("Failed to remove consumer from storage",
				zap.String("consumerId", req.ConsumerId),
				zap.Error(storeErr))
		}
	}

	Info("Consumer deregistered successfully",
		zap.String("consumerId", req.ConsumerId),
		zap.String("schemaId", req.SchemaId),
		zap.String("environment", environment))

	grpcRequestsTotal.WithLabelValues("DeregisterConsumer", "success").Inc()

	return &pb.DeregisterConsumerResponse{
		Success: true,
		Message: "Consumer deregistered successfully",
	}, nil
}
