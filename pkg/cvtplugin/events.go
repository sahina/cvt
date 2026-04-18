package cvtplugin

import (
	"context"

	"github.com/hashicorp/go-plugin"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	"google.golang.org/grpc"
)

// EventHandler is the Go interface plugin authors implement to receive CVT
// events (breaking-change detection, validation failure). Methods must be
// safe for concurrent use and should dedup/rate-limit on the plugin side
// since core fires every event.
//
// Plugins that don't care about a particular event can return
// status.Error(codes.Unimplemented, "not handled"); core treats that as a
// no-op, not an error. Alternatively, embed UnimplementedEventHandler to
// opt out of specific events at compile time.
type EventHandler interface {
	OnBreakingChangeDetected(ctx context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error)
	OnValidationFailed(ctx context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error)
}

// UnimplementedEventHandler can be embedded in a partial EventHandler
// implementation to opt out of events the plugin doesn't handle. Embedded
// methods return Unimplemented, which core treats as a no-op.
type UnimplementedEventHandler = eventspb.UnimplementedEventHandlerServer

// eventsGRPCPlugin is the go-plugin adapter for EventHandler.
type eventsGRPCPlugin struct {
	plugin.Plugin
	Impl EventHandler
}

func (p *eventsGRPCPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	eventspb.RegisterEventHandlerServer(s, &eventsServer{impl: p.Impl})
	return nil
}

func (p *eventsGRPCPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return eventspb.NewEventHandlerClient(c), nil
}

type eventsServer struct {
	eventspb.UnimplementedEventHandlerServer
	impl EventHandler
}

func (s *eventsServer) OnBreakingChangeDetected(ctx context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	return s.impl.OnBreakingChangeDetected(ctx, req)
}

func (s *eventsServer) OnValidationFailed(ctx context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	return s.impl.OnValidationFailed(ctx, req)
}
