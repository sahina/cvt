package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/sahina/cvt/internal/pluginclient"
	"github.com/sahina/cvt/internal/pluginmgr"
	"github.com/sahina/cvt/pkg/cvt"
	"github.com/spf13/cobra"
)

// pluginHooks holds the active Hooks adapter for the current cvt invocation.
// Set once by the root command's PersistentPreRunE; subcommands read it when
// constructing a *cvt.Validator. Defaults to NoopHooks so any code path that
// runs before bootstrap (or in tests) is safe.
var (
	pluginHooks    cvt.Hooks = cvt.NoopHooks{}
	pluginShutdown           = func() {}
	pluginInitOnce sync.Once
)

// skipPluginBootstrap lists command paths that don't need plugin runtime.
// These either don't validate anything (mock, version, wait) or manage the
// plugin set itself (plugins).
var skipPluginBootstrap = map[string]bool{
	"cvt version":         true,
	"cvt help":            true,
	"cvt wait":            true,
	"cvt mock":            true,
	"cvt plugins":         true,
	"cvt plugins list":    true,
	"cvt plugins install": true,
	"cvt plugins remove":  true,
}

// installPluginBootstrap attaches PersistentPreRunE to the root command so
// every subcommand bootstraps the plugin runtime once. Subcommands listed in
// skipPluginBootstrap are no-ops.
func installPluginBootstrap(rootCmd *cobra.Command) {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if skipPluginBootstrap[cmd.CommandPath()] {
			return nil
		}
		var err error
		pluginInitOnce.Do(func() {
			pluginHooks, pluginShutdown, err = BootstrapForCommand(cmd.Context())
		})
		return err
	}
}

// runPluginShutdown stops any running plugins. Safe to call multiple times.
func runPluginShutdown() {
	if pluginShutdown != nil {
		pluginShutdown()
	}
}

// BootstrapForCommand derives the default config + state + plugin-root paths
// and delegates to bootstrapWithPaths. CLI commands call this; tests call
// bootstrapWithPaths directly to control file locations.
func BootstrapForCommand(ctx context.Context) (cvt.Hooks, func(), error) {
	if os.Getenv("CVT_DISABLE_PLUGINS") == "1" {
		return cvt.NoopHooks{}, func() {}, nil
	}
	cfgPath, cfgErr := pluginmgr.DefaultConfigPath()
	pluginRoot, rootErr := pluginmgr.DefaultPluginRoot()
	statePath, stateErr := pluginmgr.DefaultStatePath()
	if cfgErr != nil || rootErr != nil || stateErr != nil {
		// HOME unset or similar OS oddity. Treat as no-plugins.
		return cvt.NoopHooks{}, func() {}, nil
	}
	return bootstrapWithPaths(ctx, cfgPath, pluginRoot, statePath)
}

// bootstrapWithPaths is the testable core of BootstrapForCommand.
//
// Behavior matrix (graceful by design — most users have no plugins):
//   - config file missing                 -> NoopHooks, silent (handled by pluginmgr.Load)
//   - config exists but empty plugins map -> NoopHooks, silent
//   - config malformed                    -> NoopHooks + stderr warning
//   - manager Start error                 -> propagate (fail_closed plugins)
//   - happy path                          -> real adapter + working shutdown
func bootstrapWithPaths(ctx context.Context, configPath, pluginRoot, statePath string) (cvt.Hooks, func(), error) {
	cfg, err := pluginmgr.Load(configPath, pluginRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load plugin config from %s: %v (continuing without plugins)\n", configPath, err)
		return cvt.NoopHooks{}, func() {}, nil
	}
	if len(cfg.Plugins) == 0 {
		return cvt.NoopHooks{}, func() {}, nil
	}
	state, err := pluginmgr.ReadState(statePath)
	if err != nil {
		// Treat unreadable state as empty; pluginmgr will reject binaries
		// that aren't recorded if it needs them. Log so the operator can
		// trace a confusing "binary not in state" error back to the real
		// cause (corrupt state file, permissions, etc.).
		fmt.Fprintf(os.Stderr, "warning: failed to read plugin state from %s: %v (treating as empty)\n", statePath, err)
		state = &pluginmgr.StateFile{}
	}
	mgr := pluginmgr.New(cfg, state, pluginmgr.Options{})
	if err := mgr.Start(ctx); err != nil {
		return cvt.NoopHooks{}, func() {}, err
	}
	adapter := pluginclient.NewHooks(mgr)
	return adapter, mgr.Stop, nil
}
