package cvtplugin

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// Option configures a plugin Serve invocation.
type Option func(*serveConfig)

type serveConfig struct {
	registry RegistryProvider
	events   EventHandler
	receiver ConfigReceiver
	logger   hclog.Logger
}

// WithRegistryProvider registers a RegistryProvider implementation.
func WithRegistryProvider(p RegistryProvider) Option {
	return func(c *serveConfig) { c.registry = p }
}

// WithEventHandler registers an EventHandler implementation.
func WithEventHandler(h EventHandler) Option {
	return func(c *serveConfig) { c.events = h }
}

// WithConfigReceiver registers an optional handler for SetConfig RPC calls.
// The receiver gets called once per config key after Info succeeds and
// before any extension-point RPC runs. Secret keys declared in
// plugins.<name>.secrets arrive here, not via os.Getenv.
func WithConfigReceiver(r ConfigReceiver) Option {
	return func(c *serveConfig) { c.receiver = r }
}

// WithLogger overrides the default hclog logger. The SDK's default writes
// JSON-structured logs to stderr, which hashicorp/go-plugin forwards to
// the CVT host's structured logger.
func WithLogger(l hclog.Logger) Option {
	return func(c *serveConfig) { c.logger = l }
}

// Serve is the plugin author's entry point. Call from main():
//
//	func main() {
//		cvtplugin.Serve(cvtplugin.PluginInfo{Name: "my-registry", Version: "0.1.0"},
//			cvtplugin.WithRegistryProvider(&myRegistry{}))
//	}
//
// Serve blocks until CVT core disconnects. It never returns; any return
// from plugin.Serve causes the go-plugin framework to exit the process.
func Serve(info PluginInfo, opts ...Option) {
	cfg := &serveConfig{}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.logger == nil {
		cfg.logger = hclog.New(&hclog.LoggerOptions{
			Name:       info.Name,
			Level:      hclog.Info,
			JSONFormat: true,
		})
	}

	services := []string{}
	pluginSet := plugin.PluginSet{}

	if cfg.registry != nil {
		services = append(services, ServiceRegistryV1)
		pluginSet[PluginKeyRegistry] = &registryGRPCPlugin{Impl: cfg.registry}
	}
	if cfg.events != nil {
		services = append(services, ServiceEventsV1)
		pluginSet[PluginKeyEvents] = &eventsGRPCPlugin{Impl: cfg.events}
	}

	hs := &handshakeService{
		info:     info,
		services: services,
		receiver: cfg.receiver,
		healthy:  true,
	}
	pluginSet[PluginKeyHandshake] = &handshakeGRPCPlugin{svc: hs}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         pluginSet,
		GRPCServer:      plugin.DefaultGRPCServer,
		Logger:          cfg.logger,
	})
}

// Plugin set keys. Core dispenses clients by these keys via Client.Dispense.
// Exported so core-side consumers (internal/pluginmgr) can use the same
// identifiers without redefining them.
const (
	PluginKeyHandshake = "handshake"
	PluginKeyRegistry  = "registry"
	PluginKeyEvents    = "events"
)
