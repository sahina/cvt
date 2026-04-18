package pluginmgr

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/sahina/cvt/pkg/cvtplugin"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	handshakepb "github.com/sahina/cvt/pkg/cvtplugin/pb/handshake/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Manager owns the lifecycle of every configured plugin subprocess. It is
// a singleton per cvt process: Start forks all configured plugins in
// parallel, each plugin stays alive for the life of the manager, and
// Stop tears them all down.
//
// Manager is NOT responsible for invoking plugin RPCs — that's what the
// typed clients from Registry(name) / Events(name) are for. The manager
// just owns their lifecycle and supplies ready-to-use client handles.
type Manager struct {
	cfg     *Config
	state   *StateFile
	logger  *zap.Logger
	metrics *Metrics
	audit   AuditSink

	mu       sync.RWMutex
	handles  map[string]*pluginHandle
}

// pluginHandle is the per-plugin runtime bundle. Holds the go-plugin
// client, the dispensed typed sub-clients, and the identity fields we
// carry into audit records.
type pluginHandle struct {
	name       string
	cfg        PluginConfig
	installed  InstalledPlugin
	client     *plugin.Client

	handshakeClient handshakepb.PluginHandshakeClient
	registryClient  registrypb.RegistryProviderClient
	eventsClient    eventspb.EventHandlerClient

	reportedVersion string
	pid             int
	services        []string
}

// Options bundle optional Manager dependencies. All fields are optional;
// defaults are safe for production.
type Options struct {
	// Logger is the core Zap logger. If nil, a no-op logger is used; tests
	// that care about log output should pass zaptest.NewLogger(t).
	Logger *zap.Logger

	// Metrics is the plugin-metrics struct. If nil, NewMetrics(nil) is
	// called — registering against prometheus.DefaultRegisterer.
	Metrics *Metrics

	// Audit is the plugin-call audit sink. If nil, a ZapAuditSink using
	// Logger is used; if Logger is also nil, audit is dropped.
	Audit AuditSink
}

// New constructs a Manager. The returned manager has no plugins running
// until Start is called.
func New(cfg *Config, state *StateFile, opts Options) *Manager {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	metrics := opts.Metrics
	if metrics == nil {
		metrics = NewMetrics(nil)
	}
	audit := opts.Audit
	if audit == nil {
		audit = ZapAuditSink{L: logger}
	}
	return &Manager{
		cfg:     cfg,
		state:   state,
		logger:  logger,
		metrics: metrics,
		audit:   audit,
		handles: map[string]*pluginHandle{},
	}
}

// Start forks every configured plugin in parallel, waits for each to
// complete its handshake (5s deadline per plugin), delivers configured
// secrets via SetConfig, and dispenses the typed sub-clients. Returns
// the first plugin-start error; on error, every already-started plugin
// is torn down before returning.
//
// An empty plugin set is a valid no-op (returns nil), supporting the
// "safe mode" and "no plugins configured" paths.
func (m *Manager) Start(ctx context.Context) error {
	if len(m.cfg.Plugins) == 0 {
		return nil
	}

	type result struct {
		name string
		h    *pluginHandle
		err  error
	}
	results := make(chan result, len(m.cfg.Plugins))
	var wg sync.WaitGroup
	for name, pcfg := range m.cfg.Plugins {
		wg.Add(1)
		go func(name string, pcfg PluginConfig) {
			defer wg.Done()
			h, err := m.startOne(ctx, name, pcfg)
			results <- result{name: name, h: h, err: err}
		}(name, pcfg)
	}
	wg.Wait()
	close(results)

	started := map[string]*pluginHandle{}
	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("plugin %q: %w", r.name, r.err)
			}
			continue
		}
		started[r.name] = r.h
	}
	if firstErr != nil {
		// Tear down whatever did start so we don't leak subprocesses.
		for _, h := range started {
			h.client.Kill()
		}
		return firstErr
	}

	m.mu.Lock()
	m.handles = started
	m.mu.Unlock()
	return nil
}

// Stop gracefully shuts down every running plugin. Safe to call multiple
// times; subsequent calls are no-ops.
func (m *Manager) Stop() {
	m.mu.Lock()
	handles := m.handles
	m.handles = map[string]*pluginHandle{}
	m.mu.Unlock()

	for name, h := range handles {
		m.logger.Info("plugin_stopping", zap.String("plugin", name))
		h.client.Kill()
		m.metrics.Up.WithLabelValues(name, h.reportedVersion).Set(0)
	}
}

// Registry returns the typed RegistryProvider client for the named plugin,
// or nil if the plugin is not running or doesn't implement registry.v1.
func (m *Manager) Registry(name string) registrypb.RegistryProviderClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.handles[name]
	if h == nil {
		return nil
	}
	return h.registryClient
}

// Events returns the typed EventHandler client for the named plugin, or
// nil if the plugin is not running or doesn't implement events.v1.
func (m *Manager) Events(name string) eventspb.EventHandlerClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.handles[name]
	if h == nil {
		return nil
	}
	return h.eventsClient
}

// Handle returns the runtime identity bundle for the named plugin (used
// by audit wiring). Returns false if the plugin isn't running.
func (m *Manager) Handle(name string) (HandleInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.handles[name]
	if h == nil {
		return HandleInfo{}, false
	}
	return HandleInfo{
		Name:            h.name,
		ReportedVersion: h.reportedVersion,
		SHA256:          h.installed.SHA256,
		PID:             h.pid,
		Services:        h.services,
	}, true
}

// HandleInfo is the public read-only identity view of a running plugin.
type HandleInfo struct {
	Name            string
	ReportedVersion string
	SHA256          string
	PID             int
	Services        []string
}

// Metrics exposes the metrics struct so callers (pluginclient) can
// observe per-call.
func (m *Manager) Metrics() *Metrics { return m.metrics }

// Audit exposes the audit sink so callers can emit records.
func (m *Manager) Audit() AuditSink { return m.audit }

// Cfg exposes the loaded config (used by pluginclient to look up per-hook
// plugin name and per-plugin on_error policy).
func (m *Manager) Cfg() *Config { return m.cfg }

// startOne forks a single plugin and completes its handshake + config
// delivery. Returns a fully-dispensed pluginHandle on success; on error,
// the plugin subprocess is killed before return.
func (m *Manager) startOne(ctx context.Context, name string, pcfg PluginConfig) (*pluginHandle, error) {
	installed, ok := m.state.Plugins[name]
	if !ok {
		return nil, fmt.Errorf("no install record in state.json; run `cvt plugins install`")
	}
	if installed.BinaryPath != pcfg.Binary {
		return nil, fmt.Errorf("state.json binary %q differs from config binary %q; reinstall", installed.BinaryPath, pcfg.Binary)
	}
	if err := VerifyInstalled(installed); err != nil {
		return nil, fmt.Errorf("install verification failed: %w", err)
	}

	pluginSet := plugin.PluginSet{
		cvtplugin.PluginKeyHandshake: &coreHandshakePlugin{},
		cvtplugin.PluginKeyRegistry:  &coreRegistryPlugin{},
		cvtplugin.PluginKeyEvents:    &coreEventsPlugin{},
	}

	pluginLogger := cvtplugin.NewHCLogFromZap(m.logger, name)
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  cvtplugin.Handshake,
		Plugins:          pluginSet,
		Cmd:              exec.Command(pcfg.Binary),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           pluginLogger,
		StartTimeout:     5 * time.Second,
	})

	// Ensure we kill the subprocess on any error from here on.
	success := false
	defer func() {
		if !success {
			client.Kill()
		}
	}()

	rpcClient, err := client.Client()
	if err != nil {
		return nil, fmt.Errorf("connect to plugin: %w", err)
	}

	dispense := func(key string) (interface{}, error) {
		raw, err := rpcClient.Dispense(key)
		if err != nil {
			return nil, fmt.Errorf("dispense %s: %w", key, err)
		}
		return raw, nil
	}

	rawH, err := dispense(cvtplugin.PluginKeyHandshake)
	if err != nil {
		return nil, err
	}
	hClient, ok := rawH.(handshakepb.PluginHandshakeClient)
	if !ok {
		return nil, fmt.Errorf("handshake client type assertion failed: %T", rawH)
	}

	infoCtx, cancel := context.WithTimeout(ctx, pcfg.Timeout)
	defer cancel()
	info, err := hClient.Info(infoCtx, &handshakepb.InfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("handshake Info: %w", err)
	}
	if info.GetProtocolVersion() != cvtplugin.ProtocolVersion {
		return nil, fmt.Errorf("protocol version mismatch: plugin reports %d, core supports %d",
			info.GetProtocolVersion(), cvtplugin.ProtocolVersion)
	}

	// Deliver every configured config value (including secrets) via
	// SetConfig. We send ALL config entries, not just secrets; plugins
	// that want to pull values from os.Environ directly may ignore the
	// delivery, but sending everything keeps the plugin SDK contract
	// uniform.
	for k, v := range pcfg.Config {
		cctx, ccancel := context.WithTimeout(ctx, pcfg.Timeout)
		_, err := hClient.SetConfig(cctx, &handshakepb.SetConfigRequest{Key: k, Value: v})
		ccancel()
		if err != nil {
			return nil, fmt.Errorf("SetConfig %q: %w", k, err)
		}
	}

	h := &pluginHandle{
		name:            name,
		cfg:             pcfg,
		installed:       installed,
		client:          client,
		handshakeClient: hClient,
		reportedVersion: info.GetName() + "/" + info.GetVersion(),
		pid:             pidOf(client),
		services:        info.GetServices(),
	}

	// Dispense typed sub-clients only for services the plugin declared.
	for _, svc := range info.GetServices() {
		switch svc {
		case cvtplugin.ServiceRegistryV1:
			raw, err := dispense(cvtplugin.PluginKeyRegistry)
			if err != nil {
				return nil, err
			}
			cl, ok := raw.(registrypb.RegistryProviderClient)
			if !ok {
				return nil, fmt.Errorf("registry client type assertion failed: %T", raw)
			}
			h.registryClient = cl
		case cvtplugin.ServiceEventsV1:
			raw, err := dispense(cvtplugin.PluginKeyEvents)
			if err != nil {
				return nil, err
			}
			cl, ok := raw.(eventspb.EventHandlerClient)
			if !ok {
				return nil, fmt.Errorf("events client type assertion failed: %T", raw)
			}
			h.eventsClient = cl
		}
	}

	m.logger.Info("plugin_started",
		zap.String("plugin", name),
		zap.String("reported_version", h.reportedVersion),
		zap.String("sha256", short(installed.SHA256)),
		zap.Int("pid", h.pid),
		zap.Strings("services", info.GetServices()),
	)
	m.metrics.Up.WithLabelValues(name, h.reportedVersion).Set(1)

	success = true
	return h, nil
}

// pidOf returns the best-effort pid for logs/audit. go-plugin exposes
// the pid on ReattachConfig; we fall back to 0 if unavailable.
func pidOf(c *plugin.Client) int {
	ra := c.ReattachConfig()
	if ra == nil {
		return 0
	}
	return ra.Pid
}

// coreHandshakePlugin is the core-side adapter that dispenses a typed
// PluginHandshakeClient. It mirrors the plugin-side adapter in
// pkg/cvtplugin/handshake_service.go but returns only a client.
type coreHandshakePlugin struct{ plugin.Plugin }

func (coreHandshakePlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("handshake server not implemented on client side")
}
func (coreHandshakePlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return handshakepb.NewPluginHandshakeClient(c), nil
}

type coreRegistryPlugin struct{ plugin.Plugin }

func (coreRegistryPlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("registry server not implemented on client side")
}
func (coreRegistryPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return registrypb.NewRegistryProviderClient(c), nil
}

type coreEventsPlugin struct{ plugin.Plugin }

func (coreEventsPlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("events server not implemented on client side")
}
func (coreEventsPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return eventspb.NewEventHandlerClient(c), nil
}
