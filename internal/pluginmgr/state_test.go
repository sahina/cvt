package pluginmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

	require.NoError(t, Remove("registry", pluginRoot, statePath))

	_, err = os.Stat(entry.BinaryPath)
	assert.True(t, os.IsNotExist(err), "binary deleted")

	s, err := ReadState(statePath)
	require.NoError(t, err)
	assert.NotContains(t, s.Plugins, "registry")
}

func TestRemoveUnknownPlugin(t *testing.T) {
	pluginRoot, statePath, _ := setupStateTest(t)
	err := Remove("missing", pluginRoot, statePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

// Regression for the path-validation guard: if state.json is tampered
// (rewritten to point at an arbitrary path), Remove must refuse rather
// than deleting the targeted file.
func TestRemoveRejectsBinaryPathOutsideRoot(t *testing.T) {
	pluginRoot, statePath, src := setupStateTest(t)
	_, err := Install(src, "registry", pluginRoot, statePath)
	require.NoError(t, err)

	// Tamper: rewrite the recorded binary path to a file outside pluginRoot.
	state, err := ReadState(statePath)
	require.NoError(t, err)
	victim := filepath.Join(t.TempDir(), "victim.txt")
	require.NoError(t, os.WriteFile(victim, []byte("do not delete me"), 0o644))
	entry := state.Plugins["registry"]
	entry.BinaryPath = victim
	state.Plugins["registry"] = entry
	require.NoError(t, WriteState(statePath, state))

	err = Remove("registry", pluginRoot, statePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes plugin root")

	// Victim file untouched.
	_, err = os.Stat(victim)
	assert.NoError(t, err, "victim file must still exist after refused Remove")
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

// TestInstallConcurrentWritersAllPersist pins the flock behavior:
// multiple concurrent Install calls against the same state.json must
// all land in the final state (no silently-dropped entries from racing
// read-modify-write cycles).
func TestInstallConcurrentWritersAllPersist(t *testing.T) {
	tmp := t.TempDir()
	pluginRoot := filepath.Join(tmp, "plugins")
	require.NoError(t, os.MkdirAll(pluginRoot, 0o755))
	statePath := filepath.Join(pluginRoot, "state.json")

	// Build N distinct source binaries (distinct basenames + contents so
	// sha256 differs).
	const N = 10
	srcPaths := make([]string, N)
	names := make([]string, N)
	srcDir := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	for i := 0; i < N; i++ {
		name := fmt.Sprintf("plug-%d", i)
		names[i] = name
		p := filepath.Join(srcDir, fmt.Sprintf("cvt-plugin-plug-%d", i))
		require.NoError(t, os.WriteFile(p, []byte(name), 0o755))
		srcPaths[i] = p
	}

	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := Install(srcPaths[idx], names[idx], pluginRoot, statePath)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}

	// Every install must have landed.
	s, err := ReadState(statePath)
	require.NoError(t, err)
	assert.Len(t, s.Plugins, N, "all concurrent installs should persist")
	for _, name := range names {
		_, ok := s.Plugins[name]
		assert.True(t, ok, "plugin %q missing from state", name)
	}
}

// TestInstallRejectsSameBasenameCollision: two different plugin names
// with source binaries sharing a basename would both resolve to
// pluginRoot/cvt-plugin-foo; the second install must refuse rather than
// silently truncating the first plugin's binary.
func TestInstallRejectsSameBasenameCollision(t *testing.T) {
	tmp := t.TempDir()
	pluginRoot := filepath.Join(tmp, "plugins")
	require.NoError(t, os.MkdirAll(pluginRoot, 0o755))
	statePath := filepath.Join(pluginRoot, "state.json")

	// Build two sources with the same basename in distinct directories.
	dir1 := filepath.Join(tmp, "build-a")
	dir2 := filepath.Join(tmp, "build-b")
	require.NoError(t, os.MkdirAll(dir1, 0o755))
	require.NoError(t, os.MkdirAll(dir2, 0o755))
	src1 := filepath.Join(dir1, "cvt-plugin-shared")
	src2 := filepath.Join(dir2, "cvt-plugin-shared")
	require.NoError(t, os.WriteFile(src1, []byte("A"), 0o755))
	require.NoError(t, os.WriteFile(src2, []byte("B"), 0o755))

	_, err := Install(src1, "alpha", pluginRoot, statePath)
	require.NoError(t, err)

	_, err = Install(src2, "bravo", pluginRoot, statePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used by plugin")
	assert.Contains(t, err.Error(), `"alpha"`)

	// Alpha still intact.
	s, err := ReadState(statePath)
	require.NoError(t, err)
	require.Contains(t, s.Plugins, "alpha")
	assert.NotContains(t, s.Plugins, "bravo", "bravo should not be registered when install refused")
}

// TestInstallAtomicCopyNoPartialWrite: a previous, shorter binary
// shouldn't become visible as truncated if the copy is interrupted.
// atomicCopyFile uses temp+rename, so this asserts that the
// installed-binary size matches the source size exactly.
func TestInstallAtomicCopyFullLength(t *testing.T) {
	tmp := t.TempDir()
	pluginRoot := filepath.Join(tmp, "plugins")
	require.NoError(t, os.MkdirAll(pluginRoot, 0o755))
	statePath := filepath.Join(pluginRoot, "state.json")

	srcDir := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	src := filepath.Join(srcDir, "cvt-plugin-big")
	payload := make([]byte, 1<<20) // 1 MiB of a repeating byte
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	require.NoError(t, os.WriteFile(src, payload, 0o755))

	_, err := Install(src, "big", pluginRoot, statePath)
	require.NoError(t, err)

	dst := filepath.Join(pluginRoot, "cvt-plugin-big")
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, payload, got, "destination binary must match source byte-for-byte")
}
