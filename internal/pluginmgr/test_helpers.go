package pluginmgr

import (
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"go.uber.org/zap"
)

// NewForTest constructs a Manager without forking any plugins. Tests
// inject fake clients via InjectClientsForTest and exercise the
// manager-facing API (Registry, Events, Cfg, Metrics, Audit, Handle)
// without standing up real subprocesses.
//
// Production code must use New + Start; this helper is for unit tests
// that target pluginclient / hooks policy paths.
func NewForTest(cfg *Config, opts Options) *Manager {
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return New(cfg, emptyState(), opts)
}

// InjectClientsForTest installs the given typed clients under plugin name
// `name` so Registry(name) / Events(name) / Handle(name) return them.
// Pass nil for a client the test doesn't exercise. Safe to call multiple
// times across different names.
func (m *Manager) InjectClientsForTest(
	name string,
	reg registrypb.RegistryProviderClient,
	ev eventspb.EventHandlerClient,
	info HandleInfo,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.handles == nil {
		m.handles = map[string]*pluginHandle{}
	}
	m.handles[name] = &pluginHandle{
		name:            name,
		registryClient:  reg,
		eventsClient:    ev,
		reportedVersion: info.ReportedVersion,
		pid:             info.PID,
		services:        info.Services,
		installed:       InstalledPlugin{SHA256: info.SHA256},
	}
}
