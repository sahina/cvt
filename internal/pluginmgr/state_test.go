package pluginmgr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStateTest(t *testing.T) (pluginRoot, statePath, srcBinary string) {
	t.Helper()
	tmp := t.TempDir()
	pluginRoot = filepath.Join(tmp, "plugins")
	statePath = filepath.Join(pluginRoot, "state.json")

	// Binary outside pluginRoot — Install should copy it in.
	srcDir := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	srcBinary = filepath.Join(srcDir, "cvt-plugin-registry-rest")
	require.NoError(t, os.WriteFile(srcBinary, []byte("fake-binary-contents"), 0o755))
	return
}

func TestReadStateMissingFileReturnsEmpty(t *testing.T) {
	s, err := ReadState(filepath.Join(t.TempDir(), "nope.json"))
	require.NoError(t, err)
	assert.Equal(t, CurrentStateVersion, s.Version)
	assert.Empty(t, s.Plugins)
}

func TestInstallCopiesFromOutsideRoot(t *testing.T) {
	pluginRoot, statePath, src := setupStateTest(t)

	entry, err := Install(src, "registry", pluginRoot, statePath)
	require.NoError(t, err)

	expectedDest := filepath.Join(pluginRoot, "cvt-plugin-registry-rest")
	assert.Equal(t, expectedDest, entry.BinaryPath)
	assert.Len(t, entry.SHA256, 64, "sha256 hex length")
	assert.False(t, entry.InstalledAt.IsZero())

	// Binary exists at destination.
	stat, err := os.Stat(expectedDest)
	require.NoError(t, err)
	assert.NotZero(t, stat.Mode()&0o100, "binary should be executable by owner")

	// State persisted.
	s, err := ReadState(statePath)
	require.NoError(t, err)
	assert.Equal(t, entry, s.Plugins["registry"])
}

func TestInstallKeepsBinaryAlreadyInRoot(t *testing.T) {
	tmp := t.TempDir()
	pluginRoot := filepath.Join(tmp, "plugins")
	require.NoError(t, os.MkdirAll(pluginRoot, 0o755))
	insideRoot := filepath.Join(pluginRoot, "cvt-plugin-already-here")
	require.NoError(t, os.WriteFile(insideRoot, []byte("x"), 0o755))
	statePath := filepath.Join(pluginRoot, "state.json")

	entry, err := Install(insideRoot, "already", pluginRoot, statePath)
	require.NoError(t, err)
	assert.Equal(t, insideRoot, entry.BinaryPath, "binary already in root should stay put")
}

func TestInstallRejectsInvalidName(t *testing.T) {
	pluginRoot, statePath, src := setupStateTest(t)
	_, err := Install(src, "Bad_Name", pluginRoot, statePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid plugin name")
}

func TestInstallRejectsDirectory(t *testing.T) {
	pluginRoot, statePath, _ := setupStateTest(t)
	dir := t.TempDir()
	_, err := Install(dir, "registry", pluginRoot, statePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestRemoveDeletesBinaryAndStateEntry(t *testing.T) {
	pluginRoot, statePath, src := setupStateTest(t)
	entry, err := Install(src, "registry", pluginRoot, statePath)
	require.NoError(t, err)

	require.NoError(t, Remove("registry", statePath))

	_, err = os.Stat(entry.BinaryPath)
	assert.True(t, os.IsNotExist(err), "binary deleted")

	s, err := ReadState(statePath)
	require.NoError(t, err)
	assert.NotContains(t, s.Plugins, "registry")
}

func TestRemoveUnknownPlugin(t *testing.T) {
	pluginRoot, statePath, _ := setupStateTest(t)
	_ = pluginRoot
	err := Remove("missing", statePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

func TestVerifyInstalledMatches(t *testing.T) {
	pluginRoot, statePath, src := setupStateTest(t)
	entry, err := Install(src, "registry", pluginRoot, statePath)
	require.NoError(t, err)
	assert.NoError(t, VerifyInstalled(entry))
}

func TestVerifyInstalledDetectsTampering(t *testing.T) {
	pluginRoot, statePath, src := setupStateTest(t)
	entry, err := Install(src, "registry", pluginRoot, statePath)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(entry.BinaryPath, []byte("tampered"), 0o700))
	err = VerifyInstalled(entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sha256 mismatch")
}

func TestWriteStateIsAtomic(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	s := &StateFile{Plugins: map[string]InstalledPlugin{"p": {BinaryPath: "/bin/p", SHA256: "abc"}}}
	require.NoError(t, WriteState(path, s))

	// Re-read and confirm contents.
	read, err := ReadState(path)
	require.NoError(t, err)
	assert.Equal(t, "abc", read.Plugins["p"].SHA256)

	// Lock file exists (adjacent artifact of WriteState).
	_, err = os.Stat(path + ".lock")
	assert.NoError(t, err)
}
