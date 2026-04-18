package cvtplugin

import (
	"context"

	"github.com/hashicorp/go-plugin"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"google.golang.org/grpc"
)

// RegistryProvider is the Go interface plugin authors implement to provide
// a schema registry backend. Methods must be safe for concurrent use.
type RegistryProvider interface {
	FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error)
	RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error)
}

// registryGRPCPlugin is the go-plugin adapter for RegistryProvider. It lives
// on both sides: plugin server (GRPCServer) and CVT core client (GRPCClient).
type registryGRPCPlugin struct {
	plugin.Plugin
	Impl RegistryProvider
}

func (p *registryGRPCPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	registrypb.RegisterRegistryProviderServer(s, &registryServer{impl: p.Impl})
	return nil
}

func (p *registryGRPCPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return registrypb.NewRegistryProviderClient(c), nil
}

// registryServer adapts the author's RegistryProvider to the gRPC service.
type registryServer struct {
	registrypb.UnimplementedRegistryProviderServer
	impl RegistryProvider
}

func (s *registryServer) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	return s.impl.FetchSchema(ctx, req)
}

func (s *registryServer) RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	return s.impl.RegisterConsumerUsage(ctx, req)
}
