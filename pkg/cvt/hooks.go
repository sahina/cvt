package cvt

import (
	"context"

	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
)

// Hooks is the extension-point interface CVT core calls at the four
// plugin hook sites. The interface is injected by callers (cmd/cvt,
// server/cvtservice); the implementation lives in internal/pluginclient.
//
// This indirection keeps pkg/cvt free of internal/* imports (Go enforces
// that contract at compile time) while letting the plugin manager plug
// in seamlessly.
//
// A nil-safe NoopHooks is provided for callers that want to disable
// plugin dispatch entirely (tests, CVT_DISABLE_PLUGINS=1 safe mode).
type Hooks interface {
	FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error)
	RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error)
	OnBreakingChangeDetected(ctx context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error)
	OnValidationFailed(ctx context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error)
}

// NoopHooks is a Hooks implementation that returns zero values for reads
// and no-ops for writes/events. Call sites can use this when no plugin is
// configured for a given hook, avoiding nil checks at every call site.
type NoopHooks struct{}

// FetchSchema returns nil, nil. Callers treat nil response + nil err as
// "no plugin configured for this hook" and fall back to their default
// resolution path (e.g., direct file/URL load).
func (NoopHooks) FetchSchema(_ context.Context, _ *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	return nil, nil
}

// RegisterConsumerUsage returns an acknowledged response without side
// effect. Callers treat this as "no consumer registry configured."
func (NoopHooks) RegisterConsumerUsage(_ context.Context, _ *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
}

// OnBreakingChangeDetected is a no-op.
func (NoopHooks) OnBreakingChangeDetected(_ context.Context, _ *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

// OnValidationFailed is a no-op.
func (NoopHooks) OnValidationFailed(_ context.Context, _ *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	return &eventspb.EventResponse{Acknowledged: true}, nil
}
