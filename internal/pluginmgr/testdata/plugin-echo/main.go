// Command plugin-echo is a minimal CVT plugin used by the pluginmgr
// integration test. It implements both RegistryProvider and EventHandler
// with deterministic canned responses and records the config values it
// receives so the integration test can assert secret delivery worked.
//
// Build path: this file is compiled by the test harness via `go build -o`;
// the resulting binary lives in the test's temp dir and is passed to the
// Manager.
package main

import (
	"context"
	"sync"

	"github.com/sahina/cvt/pkg/cvtplugin"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
)

type echoPlugin struct {
	mu     sync.Mutex
	config map[string]string
}

func newEcho() *echoPlugin { return &echoPlugin{config: map[string]string{}} }

func (p *echoPlugin) SetConfig(_ context.Context, key, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config[key] = value
	return nil
}

func (p *echoPlugin) FetchSchema(_ context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	// Echo the schema id back in the spec body so the test can assert
	// round-trip.
	return &registrypb.FetchSchemaResponse{
		Spec:            []byte("echoed: " + req.GetSchemaId()),
		ResolvedVersion: "1.0.0",
	}, nil
}

func (p *echoPlugin) RegisterConsumerUsage(_ context.Context, _ *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
}

func (p *echoPlugin) OnBreakingChangeDetected(_ context.Context, _ *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

func (p *echoPlugin) OnValidationFailed(_ context.Context, _ *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

func main() {
	p := newEcho()
	cvtplugin.Serve(
		cvtplugin.PluginInfo{Name: "echo", Version: "0.0.1"},
		cvtplugin.WithRegistryProvider(p),
		cvtplugin.WithEventHandler(p),
		cvtplugin.WithConfigReceiver(p),
	)
}
