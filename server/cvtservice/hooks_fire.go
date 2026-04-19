package cvtservice

import (
	"context"

	"github.com/sahina/cvt/server/pb"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
)

// fireOnBreakingChangeDetected dispatches the on_breaking_change_detected
// hook. The empty-changes guard lives HERE so call sites stay dumb: they
// always call this helper at the success-return; the helper decides
// whether the event is interesting.
//
// detectedBy is "CompareSchemas" or "RegisterSchema" — used by plugins
// (and audit) to disambiguate the originating RPC.
//
// Errors from the plugin are intentionally ignored. The original RPC
// already returned its result; hook failures are observability concerns
// surfaced via the adapter's audit + metrics path, not propagated to the
// caller.
func (s *ValidatorService) fireOnBreakingChangeDetected(
	ctx context.Context,
	schemaID, oldVer, newVer string,
	changes []*pb.BreakingChange,
	detectedBy string,
) {
	if len(changes) == 0 {
		return
	}
	h := s.hooksOrNoop()
	req := &eventspb.BreakingChangeDetectedRequest{
		SchemaId:   schemaID,
		OldVersion: oldVer,
		NewVersion: newVer,
		Changes:    convertBreakingChanges(changes),
		DetectedBy: detectedBy,
	}
	_, _ = h.OnBreakingChangeDetected(ctx, req)
}

// fireRegisterConsumerUsage dispatches the register_consumer_usage hook.
// Called from RegisterConsumer's success path. Maps server
// pb.RegisterConsumerRequest fields directly to the plugin proto;
// used_fields propagates end-to-end as of plugin proto v1.1.
//
// Errors ignored, same rationale as fireOnBreakingChangeDetected.
func (s *ValidatorService) fireRegisterConsumerUsage(
	ctx context.Context,
	req *pb.RegisterConsumerRequest,
) {
	h := s.hooksOrNoop()
	pluginReq := &registrypb.RegisterConsumerUsageRequest{
		ConsumerId:    req.ConsumerId,
		SchemaId:      req.SchemaId,
		SchemaVersion: req.SchemaVersion,
		Environment:   req.Environment,
		Endpoints:     convertEndpointUsage(req.UsedEndpoints),
	}
	_, _ = h.RegisterConsumerUsage(ctx, pluginReq)
}
