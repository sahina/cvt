// Deployment safety checks. CanIDeploy answers whether a new schema version
// can ship without breaking registered consumers in a target environment.
package cvtservice

import (
	"context"
	"fmt"
	"time"

	"github.com/sahina/cvt/server/pb"
	"go.uber.org/zap"
)

// CanIDeploy checks if a schema version can be safely deployed.
// It checks for breaking changes and analyzes impact on registered consumers.
func (s *ValidatorService) CanIDeploy(ctx context.Context, req *pb.CanIDeployRequest) (*pb.CanIDeployResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("CanIDeploy").Observe(time.Since(start).Seconds())
	}()

	Info("Received CanIDeploy request",
		zap.String("schemaId", req.SchemaId),
		zap.String("newVersion", req.NewVersion),
		zap.String("environment", req.Environment))

	// Validate request
	if req.SchemaId == "" {
		grpcRequestsTotal.WithLabelValues("CanIDeploy", "failure").Inc()
		return &pb.CanIDeployResponse{
			SafeToDeploy: false,
			Summary:      "schema_id is required",
		}, nil
	}

	environment := req.Environment
	if environment == "" {
		environment = "prod" // Default to prod for deployment safety
	}

	// Get the new schema version (cache or storage)
	_, found := s.getSchemaEntry(ctx, req.SchemaId, "")
	if !found {
		grpcRequestsTotal.WithLabelValues("CanIDeploy", "failure").Inc()
		return &pb.CanIDeployResponse{
			SafeToDeploy: false,
			Summary:      fmt.Sprintf("schema not found: %s", req.SchemaId),
		}, nil
	}

	// Get the new schema version entry for comparison
	newEntry, newFound := s.getSchemaEntry(ctx, req.SchemaId, req.NewVersion)
	if !newFound || newEntry == nil {
		// Try to get latest if specific version not found
		newEntry, newFound = s.getSchemaEntry(ctx, req.SchemaId, "")
		if !newFound || newEntry == nil {
			grpcRequestsTotal.WithLabelValues("CanIDeploy", "failure").Inc()
			return &pb.CanIDeployResponse{
				SafeToDeploy: false,
				Summary:      fmt.Sprintf("schema version not found: %s@%s", req.SchemaId, req.NewVersion),
			}, nil
		}
	}

	// Get all consumers in the target environment
	consumers := s.cache.ListConsumers(req.SchemaId, environment)

	// If no consumers, it's safe to deploy
	if len(consumers) == 0 {
		grpcRequestsTotal.WithLabelValues("CanIDeploy", "success").Inc()
		return &pb.CanIDeployResponse{
			SafeToDeploy: true,
			Summary:      fmt.Sprintf("No consumers registered for %s in %s environment", req.SchemaId, environment),
		}, nil
	}

	// Use CompatibilityEngine to detect actual breaking changes
	engine := NewCompatibilityEngine()
	var allBreakingChanges []*pb.BreakingChange
	var affectedConsumers []*pb.ConsumerImpact
	allSafe := true

	for _, consumer := range consumers {
		// If consumer is on the same version or not version-pinned, no need to compare
		if consumer.SchemaVersion == "" || consumer.SchemaVersion == req.NewVersion {
			affectedConsumers = append(affectedConsumers, &pb.ConsumerImpact{
				ConsumerId:           consumer.ConsumerID,
				ConsumerVersion:      consumer.ConsumerVersion,
				CurrentSchemaVersion: consumer.SchemaVersion,
				Environment:          consumer.Environment,
				WillBreak:            false,
			})
			continue
		}

		// Get the consumer's current schema version for comparison
		oldEntry, oldFound := s.getSchemaEntry(ctx, req.SchemaId, consumer.SchemaVersion)
		if !oldFound || oldEntry == nil {
			// Can't compare if old version not found, mark as potentially affected
			Info("Cannot find consumer's schema version for comparison",
				zap.String("consumerId", consumer.ConsumerID),
				zap.String("schemaVersion", consumer.SchemaVersion))

			affectedConsumers = append(affectedConsumers, &pb.ConsumerImpact{
				ConsumerId:           consumer.ConsumerID,
				ConsumerVersion:      consumer.ConsumerVersion,
				CurrentSchemaVersion: consumer.SchemaVersion,
				Environment:          consumer.Environment,
				WillBreak:            true, // Conservative: assume breaking if can't compare
			})
			allSafe = false
			continue
		}

		// Compare schemas to detect breaking changes
		changes, _ := engine.CompareSchemas(oldEntry.Document, newEntry.Document)

		// Filter breaking changes to only those affecting this consumer's used endpoints
		relevantChanges := filterChangesForConsumer(changes, consumer.UsedEndpoints)

		willBreak := len(relevantChanges) > 0
		if willBreak {
			allSafe = false
			allBreakingChanges = append(allBreakingChanges, relevantChanges...)
		}

		affectedConsumers = append(affectedConsumers, &pb.ConsumerImpact{
			ConsumerId:           consumer.ConsumerID,
			ConsumerVersion:      consumer.ConsumerVersion,
			CurrentSchemaVersion: consumer.SchemaVersion,
			Environment:          consumer.Environment,
			WillBreak:            willBreak,
			RelevantChanges:      relevantChanges,
		})
	}

	var summary string
	if allSafe {
		summary = fmt.Sprintf("Safe to deploy %s version %s to %s - %d consumer(s) verified",
			req.SchemaId, req.NewVersion, environment, len(consumers))
	} else {
		breakingCount := 0
		for _, c := range affectedConsumers {
			if c.WillBreak {
				breakingCount++
			}
		}
		summary = fmt.Sprintf("Deployment of %s version %s to %s will break %d of %d consumer(s) - review required",
			req.SchemaId, req.NewVersion, environment, breakingCount, len(affectedConsumers))
	}

	grpcRequestsTotal.WithLabelValues("CanIDeploy", "success").Inc()

	return &pb.CanIDeployResponse{
		SafeToDeploy:      allSafe,
		Summary:           summary,
		BreakingChanges:   allBreakingChanges,
		AffectedConsumers: affectedConsumers,
	}, nil
}

// filterChangesForConsumer filters breaking changes to only those affecting the consumer's used endpoints.
func filterChangesForConsumer(changes []*pb.BreakingChange, endpoints []EndpointUsage) []*pb.BreakingChange {
	if len(endpoints) == 0 {
		// If no endpoints specified, all changes are relevant (conservative)
		return changes
	}

	var relevant []*pb.BreakingChange
	for _, change := range changes {
		for _, ep := range endpoints {
			// Match by path (and optionally method)
			pathMatches := change.Path == ep.Path || change.Path == "" // Empty path means all paths affected
			methodMatches := change.Method == "" || change.Method == ep.Method

			if pathMatches && methodMatches {
				relevant = append(relevant, change)
				break
			}
		}
	}
	return relevant
}
