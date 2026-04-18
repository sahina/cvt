package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/sahina/cvt/internal/pluginmgr"
	"github.com/spf13/cobra"
)

// pluginsCmd is the parent for `cvt plugins {list, install, remove}`.
func pluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage CVT plugins",
		Long: `Install, list, and remove CVT plugins. Plugins live in ~/.cvt/plugins/
and are declared in ~/.cvt/config.yaml. See docs/plugins/ for the full guide.`,
	}
	cmd.AddCommand(pluginsListCmd())
	cmd.AddCommand(pluginsInstallCmd())
	cmd.AddCommand(pluginsRemoveCmd())
	return cmd
}

func pluginsListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			statePath, err := pluginmgr.DefaultStatePath()
			if err != nil {
				return err
			}
			state, err := pluginmgr.ReadState(statePath)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(state)
			}
			if len(state.Plugins) == 0 {
				_, err := fmt.Fprintln(os.Stdout, "No plugins installed. Try `cvt plugins install <path>`.")
				return err
			}
			names := make([]string, 0, len(state.Plugins))
			for n := range state.Plugins {
				names = append(names, n)
			}
			sort.Strings(names)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "NAME\tSHA256\tINSTALLED\tBINARY"); err != nil {
				return err
			}
			for _, n := range names {
				p := state.Plugins[n]
				sha := p.SHA256
				if len(sha) > 12 {
					sha = sha[:12]
				}
				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", n, sha, p.InstalledAt.Format("2006-01-02 15:04:05 MST"), p.BinaryPath); err != nil {
					return err
				}
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print state.json contents as JSON")
	return cmd
}

func pluginsInstallCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "install <path-to-binary>",
		Short: "Install a plugin binary",
		Long: `Install a plugin binary into ~/.cvt/plugins/ and record its SHA256 in
~/.cvt/plugins/state.json.

If --name is omitted, the plugin name is derived from the binary filename
by stripping a "cvt-plugin-" prefix if present (e.g., "cvt-plugin-registry-rest"
becomes "registry-rest"; bare filenames are used as-is).

After install, add an entry to ~/.cvt/config.yaml under plugins: and restart
cvt serve (if running) for the plugin to load.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			root, err := pluginmgr.DefaultPluginRoot()
			if err != nil {
				return err
			}
			statePath, err := pluginmgr.DefaultStatePath()
			if err != nil {
				return err
			}
			if name == "" {
				name = derivePluginName(src)
			}
			entry, err := pluginmgr.Install(src, name, root, statePath)
			if err != nil {
				return err
			}
			sha := entry.SHA256
			if len(sha) > 12 {
				sha = sha[:12]
			}
			if _, err := fmt.Fprintf(os.Stdout, "Installed %q\n  binary: %s\n  sha256: %s\n", name, entry.BinaryPath, sha); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(os.Stdout, "\nAdd this to ~/.cvt/config.yaml:"); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(os.Stdout, "  plugins:\n    %s:\n      binary: %s\n      on_error: fail_closed\n\n", name, entry.BinaryPath); err != nil {
				return err
			}
			_, err = fmt.Fprintln(os.Stdout, "Then restart `cvt serve` if running.")
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Plugin name (defaults to binary basename with cvt-plugin- stripped)")
	return cmd
}

func pluginsRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			statePath, err := pluginmgr.DefaultStatePath()
			if err != nil {
				return err
			}
			if err := pluginmgr.Remove(name, statePath); err != nil {
				return err
			}
			_, err = fmt.Fprintf(os.Stdout, "Removed %q. Also remove its entry from ~/.cvt/config.yaml, then restart `cvt serve` if running.\n", name)
			return err
		},
	}
	return cmd
}

// derivePluginName produces a plugin name from the binary filename by
// stripping a leading "cvt-plugin-" prefix. If the result is empty or
// doesn't match the plugin-name regex, the full basename is returned
// and pluginmgr.Install will surface the validation error.
func derivePluginName(path string) string {
	base := filepath.Base(path)
	const prefix = "cvt-plugin-"
	if len(base) > len(prefix) && base[:len(prefix)] == prefix {
		return base[len(prefix):]
	}
	return base
}
