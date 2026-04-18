package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahina/cvt/internal/pluginmgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withHomeDir rebinds $HOME to a fresh temp directory so `cvt plugins`
// commands write to an isolated ~/.cvt/plugins/ tree. Returns the fake
// home path for assertions.
func withHomeDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// writeFakeBinary drops a fake executable at srcPath. The bytes are
// arbitrary; we only need a file with mode 0755 that hashes consistently.
func writeFakeBinary(t *testing.T, srcPath string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(srcPath), 0o755))
	require.NoError(t, os.WriteFile(srcPath, []byte(content), 0o755))
}

// runCmd executes the full cobra command tree with argv, captures stdout,
// and returns (output, error). Uses pluginsCmd() directly so tests don't
// have to rebuild the whole binary.
func runCmd(t *testing.T, argv ...string) (string, error) {
	t.Helper()
	cmd := pluginsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(argv)
	err := cmd.Execute()
	return buf.String(), err
}

// runCmdPrintToStdout runs the command but captures process stdout (os.Stdout).
// Several pluginsCmd handlers use fmt.Fprint(os.Stdout, ...) directly, which
// bypasses cobra's out buffer. Tests that need that output redirect os.Stdout.
func runCmdPrintToStdout(t *testing.T, argv ...string) string {
	t.Helper()
	origOut := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = origOut }()

	cmd := pluginsCmd()
	cmd.SetArgs(argv)
	errRun := cmd.Execute()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	require.NoError(t, errRun)
	return buf.String()
}

func TestPluginsListEmpty(t *testing.T) {
	_ = withHomeDir(t)
	out := runCmdPrintToStdout(t, "list")
	assert.Contains(t, out, "No plugins installed")
}

func TestPluginsInstallThenList(t *testing.T) {
	home := withHomeDir(t)
	src := filepath.Join(t.TempDir(), "cvt-plugin-registry-rest")
	writeFakeBinary(t, src, "fake-binary")

	installOut := runCmdPrintToStdout(t, "install", src)
	assert.Contains(t, installOut, `Installed "registry-rest"`)
	assert.Contains(t, installOut, "sha256:")

	listOut := runCmdPrintToStdout(t, "list")
	assert.Contains(t, listOut, "registry-rest")
	assert.Contains(t, listOut, filepath.Join(home, ".cvt", "plugins", "cvt-plugin-registry-rest"))
}

func TestPluginsInstallWithExplicitName(t *testing.T) {
	home := withHomeDir(t)
	src := filepath.Join(t.TempDir(), "binary-with-unusual-name")
	writeFakeBinary(t, src, "fake-binary")

	runCmdPrintToStdout(t, "install", src, "--name", "my-reg")

	// state.json contains the explicit name.
	statePath := filepath.Join(home, ".cvt", "plugins", "state.json")
	raw, err := os.ReadFile(statePath)
	require.NoError(t, err)
	var state pluginmgr.StateFile
	require.NoError(t, json.Unmarshal(raw, &state))
	_, ok := state.Plugins["my-reg"]
	assert.True(t, ok, "state.json should contain plugin under explicit name")
}

func TestPluginsInstallInvalidName(t *testing.T) {
	_ = withHomeDir(t)
	src := filepath.Join(t.TempDir(), "Bad_Name")
	writeFakeBinary(t, src, "x")

	out, err := runCmd(t, "install", src)
	// cobra routes RunE errors to stderr; either way the combined buffer
	// contains the failure.
	_ = out
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "invalid plugin name")
}

func TestPluginsRemoveRoundTrip(t *testing.T) {
	home := withHomeDir(t)
	src := filepath.Join(t.TempDir(), "cvt-plugin-slack-events")
	writeFakeBinary(t, src, "x")

	runCmdPrintToStdout(t, "install", src)
	binPath := filepath.Join(home, ".cvt", "plugins", "cvt-plugin-slack-events")
	_, err := os.Stat(binPath)
	require.NoError(t, err, "binary should exist after install")

	out := runCmdPrintToStdout(t, "remove", "slack-events")
	assert.Contains(t, out, `Removed "slack-events"`)

	_, err = os.Stat(binPath)
	assert.True(t, os.IsNotExist(err), "binary should be gone after remove")

	listOut := runCmdPrintToStdout(t, "list")
	assert.Contains(t, listOut, "No plugins installed")
}

func TestPluginsRemoveUnknownPluginErrors(t *testing.T) {
	_ = withHomeDir(t)
	_, err := runCmd(t, "remove", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

func TestPluginsListJSON(t *testing.T) {
	home := withHomeDir(t)
	src := filepath.Join(t.TempDir(), "cvt-plugin-registry-rest")
	writeFakeBinary(t, src, "x")
	runCmdPrintToStdout(t, "install", src)

	out := runCmdPrintToStdout(t, "list", "--json")

	var state pluginmgr.StateFile
	require.NoError(t, json.Unmarshal([]byte(out), &state))
	entry, ok := state.Plugins["registry-rest"]
	require.True(t, ok)
	assert.Equal(t, filepath.Join(home, ".cvt", "plugins", "cvt-plugin-registry-rest"), entry.BinaryPath)
	assert.Len(t, entry.SHA256, 64)
}
