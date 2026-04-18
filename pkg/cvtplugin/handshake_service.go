package cvtplugin

import (
	"context"
	"sync"

	"github.com/hashicorp/go-plugin"
	handshakepb "github.com/sahina/cvt/pkg/cvtplugin/pb/handshake/v1"
	"google.golang.org/grpc"
)

// PluginInfo is the static identity plugin authors provide to Serve.
// Values flow into the Info RPC response.
type PluginInfo struct {
	// Name is the plugin's self-reported name. Used in logs/audit as
	// reported_name; core does not trust it for identity.
	Name string

	// Version is the plugin's self-reported version. Logged as
	// reported_version; core uses install-time sha256 as trusted identity.
	Version string
}

// ConfigReceiver is an optional interface a provider can implement to
// receive config values (including secrets) from core via SetConfig.
// Providers that don't need config can skip this interface.
type ConfigReceiver interface {
	SetConfig(ctx context.Context, key, value string) error
}

// handshakeService implements the PluginHandshake gRPC service. It is
// internal to the SDK; plugin authors never construct it directly.
type handshakeService struct {
	handshakepb.UnimplementedPluginHandshakeServer

	info     PluginInfo
	services []string
	receiver ConfigReceiver // may be nil

	mu      sync.RWMutex
	healthy bool
}

func (s *handshakeService) Info(_ context.Context, _ *handshakepb.InfoRequest) (*handshakepb.InfoResponse, error) {
	return &handshakepb.InfoResponse{
		Name:            s.info.Name,
		Version:         s.info.Version,
		Services:        s.services,
		ProtocolVersion: ProtocolVersion,
	}, nil
}

func (s *handshakeService) Health(_ context.Context, _ *handshakepb.HealthRequest) (*handshakepb.HealthResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := handshakepb.HealthResponse_SERVING
	if !s.healthy {
		status = handshakepb.HealthResponse_NOT_SERVING
	}
	return &handshakepb.HealthResponse{Status: status}, nil
}

func (s *handshakeService) SetConfig(ctx context.Context, req *handshakepb.SetConfigRequest) (*handshakepb.SetConfigResponse, error) {
	if s.receiver != nil {
		if err := s.receiver.SetConfig(ctx, req.GetKey(), req.GetValue()); err != nil {
			return nil, err
		}
	}
	return &handshakepb.SetConfigResponse{}, nil
}

// handshakeGRPCPlugin is the go-plugin adapter that registers the
// PluginHandshake service on the plugin's gRPC server and returns a
// typed client on the CVT core side.
type handshakeGRPCPlugin struct {
	plugin.Plugin
	svc *handshakeService
}

func (p *handshakeGRPCPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	handshakepb.RegisterPluginHandshakeServer(s, p.svc)
	return nil
}

func (p *handshakeGRPCPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return handshakepb.NewPluginHandshakeClient(c), nil
}
