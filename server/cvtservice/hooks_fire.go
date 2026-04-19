package cvtservice

import (
	"context"

	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/sahina/cvt/server/pb"
	"go.uber.org/zap"
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
// caller. We deliberately use a fresh context.Background() rather than
// the RPC's ctx: by the time we fire the hook, the response has already
// been computed, and a client-side cancellation/deadline must NOT cancel
// the plugin notification (mirrors pkg/cvt/hooks_fire.go). The plugin's
// own per-call timeout still applies via the adapter.
func (s *ValidatorService) fireOnBreakingChangeDetected(
	_ context.Context,
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
	_, _ = h.OnBreakingChangeDetected(context.Background(), req)
}

// fireRegisterConsumerUsage dispatches the register_consumer_usage hook.
// Called from RegisterConsumer's success path. Maps server
// pb.RegisterConsumerRequest fields directly to the plugin proto;
// used_fields propagates end-to-end as of plugin proto v1.1.
//
// Errors ignored, same rationale as fireOnBreakingChangeDetected.
// Uses context.Background() for the same client-cancellation reason.
func (s *ValidatorService) fireRegisterConsumerUsage(
	_ context.Context,
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
	_, _ = h.RegisterConsumerUsage(context.Background(), pluginReq)
}

// tryFetchSchemaFromPlugin asks the fetch_schema-bound plugin for a
// schema before falling back to storage. Unlike the other fire helpers,
// this one runs on the critical path — it uses the caller's ctx so the
// plugin's per-call timeout can cancel cleanly, and its return value
// drives control flow.
//
// Returns:
//   - (entry, true, nil)   plugin supplied a usable spec; caller uses it
//   - (nil,   false, nil)  no plugin bound, plugin returned nothing, or
//     plugin returned malformed bytes — caller falls through to storage
//   - (nil,   false, err)  plugin returned an error under fail_closed;
//     caller refuses the resolution (does NOT fall through, honoring
//     the plugin's refusal as authoritative)
//
// Plugin-supplied specs are cached and mirrored into the fixture
// generator but NOT written into s.store: the plugin is a fetch, not a
// registration, and mirroring would diverge two sources of truth.
func (s *ValidatorService) tryFetchSchemaFromPlugin(ctx context.Context, schemaID, version string) (*SchemaEntry, bool, error) {
	h := s.hooksOrNoop()
	resp, err := h.FetchSchema(ctx, &registrypb.FetchSchemaRequest{
		SchemaId: schemaID,
		Version:  version,
	})
	if err != nil {
		// Surface the error so the caller's fail-closed path fires:
		// getSchemaEntry returns (nil,false) which collapses to NOT_FOUND
		// at the RPC boundary (same shape as a storage error during
		// rehydration, and matches the plugin's "I refuse to answer"
		// intent better than masking it as a success).
		Warn("fetch_schema plugin returned error",
			zap.String("schemaId", schemaID),
			zap.String("version", version),
			zap.Error(err))
		return nil, false, err
	}
	if resp == nil || len(resp.Spec) == 0 {
		return nil, false, nil
	}

	doc, parseErr := s.parseAndConvertSchema(resp.Spec)
	if parseErr != nil {
		Warn("Failed to parse schema from fetch_schema plugin",
			zap.String("schemaId", schemaID),
			zap.Error(parseErr))
		return nil, false, nil
	}

	if valErr := doc.Validate(ctx); valErr != nil {
		Warn("fetch_schema plugin spec failed validation",
			zap.String("schemaId", schemaID),
			zap.Error(valErr))
		return nil, false, nil
	}

	// Fall back to the spec's own info.version when the plugin omits
	// ResolvedVersion — prevents empty cache keys and empty metadata.
	resolvedVersion := resp.ResolvedVersion
	if resolvedVersion == "" && doc.Info != nil {
		resolvedVersion = doc.Info.Version
	}

	entry := NewSchemaEntry(schemaID, string(resp.Spec), doc, resolvedVersion, nil)

	router, routerErr := s.buildRouter(doc)
	if routerErr != nil {
		Warn("Failed to build router for plugin-supplied schema",
			zap.String("schemaId", schemaID),
			zap.Error(routerErr))
		return nil, false, nil
	}
	entry.Router = router

	if version == "" {
		s.cache.Set(schemaID, entry)
	} else {
		// Cache under both the requested key and the resolved key when
		// they differ — otherwise alias lookups (e.g. "v1" resolving to
		// "1.2.3") pay the plugin round-trip on every call.
		s.cache.SetVersion(schemaID, version, entry)
		if resolvedVersion != "" && resolvedVersion != version {
			s.cache.SetVersion(schemaID, resolvedVersion, entry)
		}
	}

	if genErr := s.generator.RegisterSchema(schemaID, resp.Spec); genErr != nil {
		Warn("Failed to register plugin-supplied schema in generator",
			zap.String("schemaId", schemaID),
			zap.Error(genErr))
	}

	Info("Loaded schema via fetch_schema plugin",
		zap.String("schemaId", schemaID),
		zap.String("version", resolvedVersion))

	return entry, true, nil
}
