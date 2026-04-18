// Package plugintest provides an in-process test harness for CVT plugin
// authors. It lets you exercise your RegistryProvider or EventHandler
// implementation without forking a subprocess or speaking gRPC.
//
// Use this for unit tests. For end-to-end coverage (handshake, crash
// recovery, real subprocess), use the framework-level integration test
// harness in internal/pluginmgr (core team only).
package plugintest

import (
	"context"

	"github.com/sahina/cvt/pkg/cvtplugin"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
)

// Harness wraps a plugin implementation and exposes the same gRPC-shaped
// API that core would use. Methods call the implementation directly; no
// network, no subprocess, no go-plugin machinery.
type Harness struct {
	Registry cvtplugin.RegistryProvider
	Events   cvtplugin.EventHandler
	Config   map[string]string
}

// New returns a harness with no plugins. Use WithRegistry / WithEvents to
// attach implementations.
func New() *Harness { return &Harness{Config: map[string]string{}} }

// WithRegistry attaches a RegistryProvider implementation.
func (h *Harness) WithRegistry(p cvtplugin.RegistryProvider) *Harness {
	h.Registry = p
	return h
}

// WithEvents attaches an EventHandler implementation.
func (h *Harness) WithEvents(e cvtplugin.EventHandler) *Harness {
	h.Events = e
	return h
}

// SetConfig simulates core delivering a config value. If the plugin
// implements cvtplugin.ConfigReceiver, it receives the call; otherwise the
// harness records the value in h.Config for test assertions.
func (h *Harness) SetConfig(ctx context.Context, key, value string) error {
	if r, ok := h.Registry.(cvtplugin.ConfigReceiver); ok {
		if err := r.SetConfig(ctx, key, value); err != nil {
			return err
		}
	}
	if r, ok := h.Events.(cvtplugin.ConfigReceiver); ok {
		if err := r.SetConfig(ctx, key, value); err != nil {
			return err
		}
	}
	h.Config[key] = value
	return nil
}

// FetchSchema proxies to the attached RegistryProvider.
func (h *Harness) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	return h.Registry.FetchSchema(ctx, req)
}

// RegisterConsumerUsage proxies to the attached RegistryProvider.
func (h *Harness) RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	return h.Registry.RegisterConsumerUsage(ctx, req)
}

// OnBreakingChangeDetected proxies to the attached EventHandler.
func (h *Harness) OnBreakingChangeDetected(ctx context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	return h.Events.OnBreakingChangeDetected(ctx, req)
}

// OnValidationFailed proxies to the attached EventHandler.
func (h *Harness) OnValidationFailed(ctx context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	return h.Events.OnValidationFailed(ctx, req)
}
