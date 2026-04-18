// Package pluginclient is CVT's core-side glue between pkg/cvt.Hooks and
// internal/pluginmgr. It translates hook calls into typed plugin RPCs,
// records metrics + audit, and applies the per-plugin on_error policy.
//
// Core imports this package and injects HooksAdapter wherever pkg/cvt.Hooks
// is required. pkg/cvt itself stays free of internal/* imports.
package pluginclient

import (
	"context"
	"errors"
	"time"

	"github.com/sahina/cvt/internal/pluginmgr"
	"github.com/sahina/cvt/pkg/cvt"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HooksAdapter implements pkg/cvt.Hooks by dispatching to whichever plugin
// is bound to each hook in the config. Unbound hooks delegate to cvt.NoopHooks
// (read hooks return nil, events are no-ops).
type HooksAdapter struct {
	mgr  *pluginmgr.Manager
	noop cvt.NoopHooks
}

// NewHooks constructs an adapter. The manager's config controls which
// plugin (if any) handles each hook.
func NewHooks(mgr *pluginmgr.Manager) *HooksAdapter {
	return &HooksAdapter{mgr: mgr}
}

// Compile-time assertion.
var _ cvt.Hooks = (*HooksAdapter)(nil)

// isUnimplemented reports whether err signals "plugin chose not to handle
// this RPC." Plugins opt out of events by returning codes.Unimplemented
// (documented behavior in pkg/cvtplugin/events.go). Treat it as a no-op,
// bypassing the per-plugin on_error policy — Unimplemented is explicit
// intent, not a fault.
func isUnimplemented(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.Unimplemented
}

// FetchSchema invokes the plugin bound to the fetch_schema hook, if any.
// Returns (nil, nil) when no plugin is bound — the caller falls back to
// direct resolution.
func (a *HooksAdapter) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	if a == nil || a.mgr == nil {
		return a.noop.FetchSchema(ctx, req)
	}
	name := a.mgr.Cfg().Hooks.FetchSchema
	if name == "" {
		return a.noop.FetchSchema(ctx, req)
	}
	client := a.mgr.Registry(name)
	if client == nil {
		// Plugin bound in config but not running (startup failure). Policy
		// applies as if the call errored.
		return nil, applyOnError(a.mgr, name, errors.New("plugin not running"))
	}
	resp, err := timedCall(ctx, a.mgr, name, pluginmgr.AuditKindRead, "registry.v1", "FetchSchema", req.GetRequestId(), func(c context.Context) (interface{}, error) {
		return client.FetchSchema(c, req)
	})
	if err != nil {
		if isUnimplemented(err) {
			return a.noop.FetchSchema(ctx, req)
		}
		return nil, applyOnError(a.mgr, name, err)
	}
	typed, ok := resp.(*registrypb.FetchSchemaResponse)
	if !ok || typed == nil {
		// A plugin that returns (nil, nil) from FetchSchema is valid gRPC
		// but callers can't dereference a nil response. Fall back to the
		// noop path (caller resolves schema themselves).
		return a.noop.FetchSchema(ctx, req)
	}
	return typed, nil
}

// RegisterConsumerUsage invokes the plugin bound to the
// register_consumer_usage hook, if any.
func (a *HooksAdapter) RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	if a == nil || a.mgr == nil {
		return a.noop.RegisterConsumerUsage(ctx, req)
	}
	name := a.mgr.Cfg().Hooks.RegisterConsumerUsage
	if name == "" {
		return a.noop.RegisterConsumerUsage(ctx, req)
	}
	client := a.mgr.Registry(name)
	if client == nil {
		return nil, applyOnError(a.mgr, name, errors.New("plugin not running"))
	}
	resp, err := timedCall(ctx, a.mgr, name, pluginmgr.AuditKindWrite, "registry.v1", "RegisterConsumerUsage", req.GetRequestId(), func(c context.Context) (interface{}, error) {
		return client.RegisterConsumerUsage(c, req)
	})
	if err != nil {
		if isUnimplemented(err) {
			return a.noop.RegisterConsumerUsage(ctx, req)
		}
		if wrapped := applyOnError(a.mgr, name, err); wrapped != nil {
			return nil, wrapped
		}
		// fail_open swallowed the error — return a noop-ack response.
		return a.noop.RegisterConsumerUsage(ctx, req)
	}
	typed, ok := resp.(*registrypb.RegisterConsumerUsageResponse)
	if !ok || typed == nil {
		return a.noop.RegisterConsumerUsage(ctx, req)
	}
	return typed, nil
}

// OnBreakingChangeDetected invokes the plugin bound to
// on_breaking_change_detected, if any.
func (a *HooksAdapter) OnBreakingChangeDetected(ctx context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	if a == nil || a.mgr == nil {
		return a.noop.OnBreakingChangeDetected(ctx, req)
	}
	name := a.mgr.Cfg().Hooks.OnBreakingChangeDetected
	if name == "" {
		return a.noop.OnBreakingChangeDetected(ctx, req)
	}
	client := a.mgr.Events(name)
	if client == nil {
		if err := applyOnError(a.mgr, name, errors.New("plugin not running")); err != nil {
			return nil, err
		}
		return a.noop.OnBreakingChangeDetected(ctx, req)
	}
	resp, err := timedCall(ctx, a.mgr, name, pluginmgr.AuditKindWrite, "events.v1", "OnBreakingChangeDetected", req.GetRequestId(), func(c context.Context) (interface{}, error) {
		return client.OnBreakingChangeDetected(c, req)
	})
	if err != nil {
		if isUnimplemented(err) {
			return a.noop.OnBreakingChangeDetected(ctx, req)
		}
		if wrapped := applyOnError(a.mgr, name, err); wrapped != nil {
			return nil, wrapped
		}
		return a.noop.OnBreakingChangeDetected(ctx, req)
	}
	typed, ok := resp.(*eventspb.EventResponse)
	if !ok || typed == nil {
		return a.noop.OnBreakingChangeDetected(ctx, req)
	}
	return typed, nil
}

// OnValidationFailed invokes the plugin bound to on_validation_failed.
func (a *HooksAdapter) OnValidationFailed(ctx context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	if a == nil || a.mgr == nil {
		return a.noop.OnValidationFailed(ctx, req)
	}
	name := a.mgr.Cfg().Hooks.OnValidationFailed
	if name == "" {
		return a.noop.OnValidationFailed(ctx, req)
	}
	client := a.mgr.Events(name)
	if client == nil {
		if err := applyOnError(a.mgr, name, errors.New("plugin not running")); err != nil {
			return nil, err
		}
		return a.noop.OnValidationFailed(ctx, req)
	}
	resp, err := timedCall(ctx, a.mgr, name, pluginmgr.AuditKindWrite, "events.v1", "OnValidationFailed", req.GetRequestId(), func(c context.Context) (interface{}, error) {
		return client.OnValidationFailed(c, req)
	})
	if err != nil {
		if isUnimplemented(err) {
			return a.noop.OnValidationFailed(ctx, req)
		}
		if wrapped := applyOnError(a.mgr, name, err); wrapped != nil {
			return nil, wrapped
		}
		return a.noop.OnValidationFailed(ctx, req)
	}
	typed, ok := resp.(*eventspb.EventResponse)
	if !ok || typed == nil {
		return a.noop.OnValidationFailed(ctx, req)
	}
	return typed, nil
}

// timedCall wraps an RPC with per-plugin timeout, metrics, and audit.
func timedCall(
	ctx context.Context,
	mgr *pluginmgr.Manager,
	plugin string,
	kind pluginmgr.AuditKind,
	service, method, requestID string,
	fn func(context.Context) (interface{}, error),
) (interface{}, error) {
	cfg, ok := mgr.Cfg().Plugins[plugin]
	if !ok {
		return nil, errors.New("plugin not declared in config")
	}
	info, _ := mgr.Handle(plugin)

	cctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	start := time.Now()
	resp, err := fn(cctx)
	duration := time.Since(start)

	code := codes.OK.String()
	outcome := pluginmgr.OutcomeOK
	if err != nil {
		outcome = pluginmgr.OutcomeError
		st, _ := status.FromError(err)
		code = st.Code().String()
		if st.Code() == codes.DeadlineExceeded {
			outcome = pluginmgr.OutcomeTimeout
		}
	}

	mgr.Metrics().CallDuration.WithLabelValues(plugin, service, method).Observe(duration.Seconds())
	if err != nil {
		mgr.Metrics().CallErrors.WithLabelValues(plugin, service, method, code).Inc()
	}

	errorCode := ""
	if err != nil {
		errorCode = code
	}
	mgr.Audit().Record(pluginmgr.AuditRecord{
		Kind:            kind,
		Plugin:          plugin,
		ReportedVersion: info.ReportedVersion,
		SHA256:          info.SHA256,
		PID:             info.PID,
		RequestID:       requestID,
		Service:         service,
		Method:          method,
		DurationMS:      duration.Milliseconds(),
		Outcome:         outcome,
		ErrorCode:       errorCode,
		Timestamp:       time.Now().UTC(),
	})

	return resp, err
}

// applyOnError returns err unchanged if the plugin's on_error policy is
// fail_closed (or the plugin isn't declared), and nil if fail_open. The
// same rule applies to reads and writes — callers decide what to return
// on the fail_open path (reads fall back to noop resolution, writes
// return an acknowledged no-op response).
func applyOnError(mgr *pluginmgr.Manager, name string, err error) error {
	cfg, ok := mgr.Cfg().Plugins[name]
	if !ok || cfg.OnError == pluginmgr.OnErrorFailClosed {
		return err
	}
	return nil
}
