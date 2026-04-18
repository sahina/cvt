package pluginmgr

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTempConfig creates a temp dir acting as the plugin root, writes a
// fake plugin binary inside it, and writes the given yaml next to it.
// Returns (configPath, pluginRoot).
func writeTempConfig(t *testing.T, yaml string) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	pluginRoot := filepath.Join(tmp, "plugins")
	require.NoError(t, os.MkdirAll(pluginRoot, 0o755))
	// Fake binary so tests referencing /tmp/.../plugins/foo resolve.
	for _, name := range []string{"cvt-plugin-registry-rest", "cvt-plugin-slack-events", "cvt-plugin-other"} {
		p := filepath.Join(pluginRoot, name)
		require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755))
	}
	cfgPath := filepath.Join(tmp, "config.yaml")
	// Replace {PLUGINROOT} placeholder with actual path so tests read cleanly.
	rendered := ""
	for _, ch := range yaml {
		rendered += string(ch)
	}
	expanded := replaceAll(rendered, "{PLUGINROOT}", pluginRoot)
	require.NoError(t, os.WriteFile(cfgPath, []byte(expanded), 0o644))
	return cfgPath, pluginRoot
}

func replaceAll(s, from, to string) string {
	out := ""
	for {
		i := indexOf(s, from)
		if i < 0 {
			out += s
			return out
		}
		out += s[:i] + to
		s = s[i+len(from):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"), t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, cfg.Plugins)
	assert.Empty(t, cfg.Hooks.FetchSchema)
}

func TestLoadSafeModeEnvReturnsEmpty(t *testing.T) {
	t.Setenv(EnvDisablePlugins, "1")
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  registry:
    binary: {PLUGINROOT}/cvt-plugin-registry-rest
    on_error: fail_closed
    config:
      base_url: https://example.com
hooks:
  fetch_schema: registry
`)
	cfg, err := Load(cfgPath, root)
	require.NoError(t, err)
	assert.Empty(t, cfg.Plugins, "CVT_DISABLE_PLUGINS=1 must force empty plugin set")
}

func TestLoadValidConfig(t *testing.T) {
	t.Setenv("CVT_REGISTRY_TOKEN", "s3cret")
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  registry:
    binary: {PLUGINROOT}/cvt-plugin-registry-rest
    timeout: 7s
    on_error: fail_closed
    secrets: [token]
    config:
      base_url: https://registry.example.com
      token: ${CVT_REGISTRY_TOKEN}
  slack:
    binary: {PLUGINROOT}/cvt-plugin-slack-events
    timeout: 2s
    on_error: fail_open
    secrets: [webhook_url]
    config:
      webhook_url: ${SLACK_WEBHOOK:-https://default.example.com/hook}
hooks:
  fetch_schema: registry
  register_consumer_usage: registry
  on_breaking_change_detected: slack
  on_validation_failed: slack
`)
	cfg, err := Load(cfgPath, root)
	require.NoError(t, err)
	require.Len(t, cfg.Plugins, 2)

	reg := cfg.Plugins["registry"]
	assert.Equal(t, 7*time.Second, reg.Timeout)
	assert.Equal(t, "fail_closed", reg.OnError)
	assert.Equal(t, []string{"token"}, reg.Secrets)
	assert.Equal(t, "s3cret", reg.Config["token"], "env interpolation")
	assert.Equal(t, "https://registry.example.com", reg.Config["base_url"])

	slack := cfg.Plugins["slack"]
	assert.Equal(t, "fail_open", slack.OnError)
	assert.Equal(t, "https://default.example.com/hook", slack.Config["webhook_url"], "${VAR:-default} fallback")

	assert.Equal(t, "registry", cfg.Hooks.FetchSchema)
	assert.Equal(t, "slack", cfg.Hooks.OnValidationFailed)
}

func TestLoadDefaultsTimeoutAndOnError(t *testing.T) {
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  registry:
    binary: {PLUGINROOT}/cvt-plugin-registry-rest
`)
	cfg, err := Load(cfgPath, root)
	require.NoError(t, err)
	p := cfg.Plugins["registry"]
	assert.Equal(t, DefaultCallTimeout, p.Timeout)
	assert.Equal(t, OnErrorFailClosed, p.OnError)
}

func TestLoadInvalidPluginName(t *testing.T) {
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  Bad_Name:
    binary: {PLUGINROOT}/cvt-plugin-registry-rest
`)
	_, err := Load(cfgPath, root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestLoadBinaryOutsidePluginRoot(t *testing.T) {
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  registry:
    binary: /tmp/elsewhere/cvt-plugin-registry-rest
`)
	_, err := Load(cfgPath, root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be under")
}

func TestLoadUnsupportedConfigVersion(t *testing.T) {
	cfgPath, root := writeTempConfig(t, `
config_version: 99
plugins: {}
`)
	_, err := Load(cfgPath, root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported config_version")
}

func TestLoadSecretDeclaredButMissing(t *testing.T) {
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  registry:
    binary: {PLUGINROOT}/cvt-plugin-registry-rest
    secrets: [token]
    config:
      base_url: https://example.com
`)
	_, err := Load(cfgPath, root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret")
	assert.Contains(t, err.Error(), "token")
}

func TestLoadUnsetEnvVarNoDefaultFails(t *testing.T) {
	os.Unsetenv("DEFINITELY_NOT_SET_CVTTEST")
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  registry:
    binary: {PLUGINROOT}/cvt-plugin-registry-rest
    secrets: [token]
    config:
      token: ${DEFINITELY_NOT_SET_CVTTEST}
`)
	_, err := Load(cfgPath, root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unset")
}

func TestLoadHookReferencesUndeclaredPlugin(t *testing.T) {
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  registry:
    binary: {PLUGINROOT}/cvt-plugin-registry-rest
hooks:
  fetch_schema: ghost
`)
	_, err := Load(cfgPath, root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undeclared plugin")
	assert.Contains(t, err.Error(), "ghost")
}

func TestLoadInvalidOnError(t *testing.T) {
	cfgPath, root := writeTempConfig(t, `
config_version: 1
plugins:
  registry:
    binary: {PLUGINROOT}/cvt-plugin-registry-rest
    on_error: maybe
`)
	_, err := Load(cfgPath, root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on_error")
}
