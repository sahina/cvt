// Package cvtplugin is the Go SDK for authoring CVT plugins.
//
// A CVT plugin is a separate binary that implements one or more of the
// CVT plugin services (RegistryProvider, EventHandler) and communicates
// with `cvt` via hashicorp/go-plugin over a unix socket.
//
// Minimal registry plugin:
//
//	package main
//
//	import (
//		"context"
//		"github.com/sahina/cvt/pkg/cvtplugin"
//		registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
//	)
//
//	type myRegistry struct{}
//
//	func (*myRegistry) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
//		// ... fetch from your registry ...
//		return &registrypb.FetchSchemaResponse{Spec: specBytes}, nil
//	}
//
//	func (*myRegistry) RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
//		// ... idempotent upsert ...
//		return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
//	}
//
//	func main() {
//		cvtplugin.Serve(cvtplugin.PluginInfo{
//			Name: "my-registry", Version: "0.1.0",
//		}, cvtplugin.WithRegistryProvider(&myRegistry{}))
//	}
//
// See docs/plugins/authoring-go.md for the full guide.
package cvtplugin
