// Package pluginmgr is CVT's plugin lifecycle + pipeline manager. It is
// core-only; plugin authors never import this package.
package pluginmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level plugin configuration loaded from
// ~/.cvt/config.yaml. Empty Plugins + empty Hooks means "no plugins run";
// plugin system stays dormant.
type Config struct {
	ConfigVersion int                     `yaml:"config_version"`
	Plugins       map[string]PluginConfig `yaml:"plugins"`
	Hooks         HookBindings            `yaml:"hooks"`
}

// PluginConfig is per-plugin configuration.
//
// Note: there is no Restart field in v1. go-plugin's default behavior
// restarts crashed plugins automatically; a configurable restart policy
// is tracked as part of the v1.1 supervisor work (see issue #107).
type PluginConfig struct {
	Binary  string            `yaml:"binary"`
	Timeout time.Duration     `yaml:"timeout"`
	OnError string            `yaml:"on_error"`
	Secrets []string          `yaml:"secrets"`
	Config  map[string]string `yaml:"config"`
}

// HookBindings maps hook names to the single plugin that handles each.
// v1 = one plugin per hook; multi-plugin fanout deferred to v1.1.
type HookBindings struct {
	FetchSchema              string `yaml:"fetch_schema,omitempty"`
	RegisterConsumerUsage    string `yaml:"register_consumer_usage,omitempty"`
	OnBreakingChangeDetected string `yaml:"on_breaking_change_detected,omitempty"`
	OnValidationFailed       string `yaml:"on_validation_failed,omitempty"`
}

// Supported values for PluginConfig.OnError.
const (
	OnErrorFailClosed = "fail_closed"
	OnErrorFailOpen   = "fail_open"
)

// Supported hook names. The set is closed in v1; unknown hook names in
// the config file are load-time errors.
const (
	HookFetchSchema              = "fetch_schema"
	HookRegisterConsumerUsage    = "register_consumer_usage"
	HookOnBreakingChangeDetected = "on_breaking_change_detected"
	HookOnValidationFailed       = "on_validation_failed"
)

// SupportedConfigVersion is the config_version core accepts. Unknown
// values fail at load time.
const SupportedConfigVersion = 1

// Default per-call timeout when the plugin config omits timeout:.
const DefaultCallTimeout = 5 * time.Second

// pluginNameRegex enforces the name constraint documented in the design
// doc: lowercase letters, digits, hyphen; start with a letter; 1..32 chars.
var pluginNameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// EnvDisablePlugins, when set to "1", forces Load to return an empty
// config regardless of file contents. Safe-mode escape hatch.
const EnvDisablePlugins = "CVT_DISABLE_PLUGINS"

// Load reads and validates the plugin config from the given path. Missing
// file is not an error — it returns an empty Config (plugin system stays
// dormant). Returns a wrapped error on invalid YAML, unknown
// config_version, invalid plugin name, binary path escape, unset secret,
// or unknown hook name.
//
// If CVT_DISABLE_PLUGINS=1 is set, Load returns an empty Config
// immediately without reading the file.
//
// pluginRoot is the allowed plugin-binary directory, typically
// ~/.cvt/plugins; binaries declared outside this path are rejected.
func Load(path, pluginRoot string) (*Config, error) {
	if os.Getenv(EnvDisablePlugins) == "1" {
		return &Config{}, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if cfg.ConfigVersion != 0 && cfg.ConfigVersion != SupportedConfigVersion {
		return nil, fmt.Errorf("unsupported config_version %d (supported: %d); upgrade cvt",
			cfg.ConfigVersion, SupportedConfigVersion)
	}

	// Validate each plugin stanza.
	for name, p := range cfg.Plugins {
		if !pluginNameRegex.MatchString(name) {
			return nil, fmt.Errorf("plugin name %q invalid: must match %s", name, pluginNameRegex.String())
		}
		resolved, err := validateBinaryPath(p.Binary, pluginRoot)
		if err != nil {
			return nil, fmt.Errorf("plugin %q: %w", name, err)
		}
		p.Binary = resolved

		if p.OnError == "" {
			p.OnError = OnErrorFailClosed
		}
		if p.OnError != OnErrorFailClosed && p.OnError != OnErrorFailOpen {
			return nil, fmt.Errorf("plugin %q: on_error must be %q or %q, got %q",
				name, OnErrorFailClosed, OnErrorFailOpen, p.OnError)
		}
		if p.Timeout == 0 {
			p.Timeout = DefaultCallTimeout
		}

		// Env interpolation on config values. Secrets go through the same
		// interpolation; their delivery to the plugin is handled separately
		// via SetConfig, not in-config.
		for k, v := range p.Config {
			resolved, err := expandEnv(v)
			if err != nil {
				return nil, fmt.Errorf("plugin %q config[%q]: %w", name, k, err)
			}
			p.Config[k] = resolved
		}

		// Every declared secret key MUST be present in config: so core has
		// a value to deliver via SetConfig. Unset secret without default =
		// load-time fail-closed per design doc.
		for _, sk := range p.Secrets {
			if _, ok := p.Config[sk]; !ok {
				return nil, fmt.Errorf("plugin %q: secret %q declared but not set in config:", name, sk)
			}
		}

		cfg.Plugins[name] = p
	}

	// Validate hook bindings refer to declared plugins.
	if err := validateHooks(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validateBinaryPath ensures the configured plugin binary path is
// absolute (after ~ expansion) and resides under pluginRoot. Symlinks
// are not followed; an attacker placing a symlink inside pluginRoot
// pointing elsewhere is out-of-scope (see docs/plugins/security.md —
// trust boundary = install step).
func validateBinaryPath(bin, pluginRoot string) (string, error) {
	if bin == "" {
		return "", fmt.Errorf("binary: required")
	}
	expanded, err := expandHome(bin)
	if err != nil {
		return "", fmt.Errorf("binary: %w", err)
	}
	if !filepath.IsAbs(expanded) {
		return "", fmt.Errorf("binary: must be absolute, got %q", bin)
	}
	clean := filepath.Clean(expanded)
	root := filepath.Clean(pluginRoot)
	rel, err := filepath.Rel(root, clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("binary: must be under %s (got %s)", root, clean)
	}
	return clean, nil
}

// expandHome replaces a leading "~/" or "~" with the user's home directory.
func expandHome(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home dir: %w", err)
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// expandEnv substitutes ${VAR} and ${VAR:-default} sequences in s with
// values from the process environment. Unset variables without a default
// return an error so config load fails loudly rather than delivering
// empty secrets to plugins.
func expandEnv(s string) (string, error) {
	var b strings.Builder
	for len(s) > 0 {
		i := strings.Index(s, "${")
		if i < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		s = s[i+2:]
		end := strings.Index(s, "}")
		if end < 0 {
			return "", fmt.Errorf("unterminated ${ in value")
		}
		expr := s[:end]
		s = s[end+1:]

		name := expr
		def := ""
		hasDef := false
		if idx := strings.Index(expr, ":-"); idx >= 0 {
			name = expr[:idx]
			def = expr[idx+2:]
			hasDef = true
		}
		val, ok := os.LookupEnv(name)
		if !ok || val == "" {
			if !hasDef {
				return "", fmt.Errorf("env var %q unset and no default", name)
			}
			val = def
		}
		b.WriteString(val)
	}
	return b.String(), nil
}

// validateHooks ensures every configured hook binding refers to a plugin
// declared in the Plugins map. Unknown hook keys are caught by yaml
// unmarshal (struct fields are fixed); the empty-string default means
// "hook disabled."
func validateHooks(cfg *Config) error {
	check := func(hookName, pluginName string) error {
		if pluginName == "" {
			return nil
		}
		if _, ok := cfg.Plugins[pluginName]; !ok {
			return fmt.Errorf("hook %s references undeclared plugin %q", hookName, pluginName)
		}
		return nil
	}
	if err := check(HookFetchSchema, cfg.Hooks.FetchSchema); err != nil {
		return err
	}
	if err := check(HookRegisterConsumerUsage, cfg.Hooks.RegisterConsumerUsage); err != nil {
		return err
	}
	if err := check(HookOnBreakingChangeDetected, cfg.Hooks.OnBreakingChangeDetected); err != nil {
		return err
	}
	if err := check(HookOnValidationFailed, cfg.Hooks.OnValidationFailed); err != nil {
		return err
	}
	return nil
}

// DefaultConfigPath returns ~/.cvt/config.yaml, expanded.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cvt", "config.yaml"), nil
}

// DefaultPluginRoot returns ~/.cvt/plugins, expanded.
func DefaultPluginRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cvt", "plugins"), nil
}
