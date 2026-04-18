package pluginmgr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// StateFile is the on-disk install-time metadata store at
// ~/.cvt/plugins/state.json. It tracks which plugins have been installed,
// their binary paths, and install-time SHA256 hashes.
//
// Runtime state (pid, up/down, restart count, circuit state) is NEVER
// stored here. That data lives in-process and is exposed via the
// Prometheus /metrics endpoint. Keeping state.json install-only avoids
// disk contention between the running daemon and the CLI, and eliminates
// stale-read risk.
type StateFile struct {
	Version int                    `json:"version"`
	Plugins map[string]InstalledPlugin `json:"plugins"`
}

// InstalledPlugin is per-plugin install-time metadata.
type InstalledPlugin struct {
	BinaryPath  string    `json:"binary_path"`
	SHA256      string    `json:"sha256"`
	InstalledAt time.Time `json:"installed_at"`
}

// CurrentStateVersion is the schema version of StateFile. Bumped if the
// shape changes.
const CurrentStateVersion = 1

// DefaultStatePath returns ~/.cvt/plugins/state.json.
func DefaultStatePath() (string, error) {
	root, err := DefaultPluginRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "state.json"), nil
}

// ReadState loads the state file. Missing file returns an empty StateFile
// (not an error) since fresh installs have no state yet.
func ReadState(path string) (*StateFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &StateFile{Version: CurrentStateVersion, Plugins: map[string]InstalledPlugin{}}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s StateFile
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.Plugins == nil {
		s.Plugins = map[string]InstalledPlugin{}
	}
	if s.Version != 0 && s.Version != CurrentStateVersion {
		return nil, fmt.Errorf("unsupported state.json version %d (supported: %d)", s.Version, CurrentStateVersion)
	}
	s.Version = CurrentStateVersion
	return &s, nil
}

// WriteState writes state to the given path atomically, guarded by a
// file lock so concurrent CLI invocations don't corrupt each other. The
// write is temp-file + rename, which is atomic on POSIX filesystems.
func WriteState(path string, state *StateFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open lock: %w", err)
	}
	defer lock.Close()

	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	tmp, err := os.CreateTemp(filepath.Dir(path), "state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	removeTmp := func() { _ = os.Remove(tmpPath) }

	state.Version = CurrentStateVersion
	if state.Plugins == nil {
		state.Plugins = map[string]InstalledPlugin{}
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(state); err != nil {
		tmp.Close()
		removeTmp()
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		removeTmp()
		return fmt.Errorf("fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		removeTmp()
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		removeTmp()
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// Install copies (or verifies) the binary at srcPath into pluginRoot,
// records the entry in state.json, and returns the installed entry.
//
// If srcPath is already inside pluginRoot, the binary stays where it is
// and only the SHA256 is computed and recorded. Otherwise the binary is
// copied into pluginRoot with the same basename and permission 0o700.
//
// name must match the plugin-name regex; this is the key used for the
// state entry.
func Install(srcPath, name, pluginRoot, statePath string) (InstalledPlugin, error) {
	if !pluginNameRegex.MatchString(name) {
		return InstalledPlugin{}, fmt.Errorf("invalid plugin name %q", name)
	}

	absSrc, err := filepath.Abs(srcPath)
	if err != nil {
		return InstalledPlugin{}, fmt.Errorf("resolve source: %w", err)
	}
	info, err := os.Stat(absSrc)
	if err != nil {
		return InstalledPlugin{}, fmt.Errorf("stat source: %w", err)
	}
	if info.IsDir() {
		return InstalledPlugin{}, fmt.Errorf("source is a directory: %s", absSrc)
	}

	root := filepath.Clean(pluginRoot)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return InstalledPlugin{}, fmt.Errorf("mkdir plugin root: %w", err)
	}

	var destPath string
	rel, relErr := filepath.Rel(root, absSrc)
	insideRoot := relErr == nil && rel != ".." && !hasDotDotPrefix(rel)

	if insideRoot {
		destPath = absSrc
	} else {
		destPath = filepath.Join(root, filepath.Base(absSrc))
		if err := copyFileTo(absSrc, destPath, 0o700); err != nil {
			return InstalledPlugin{}, fmt.Errorf("copy binary: %w", err)
		}
	}

	sum, err := sha256File(destPath)
	if err != nil {
		return InstalledPlugin{}, fmt.Errorf("sha256: %w", err)
	}

	state, err := ReadState(statePath)
	if err != nil {
		return InstalledPlugin{}, err
	}
	entry := InstalledPlugin{
		BinaryPath:  destPath,
		SHA256:      sum,
		InstalledAt: time.Now().UTC().Truncate(time.Second),
	}
	state.Plugins[name] = entry
	if err := WriteState(statePath, state); err != nil {
		return InstalledPlugin{}, err
	}
	return entry, nil
}

// Remove deletes the plugin binary and its state entry. Returns an error
// if the plugin is not installed.
func Remove(name, statePath string) error {
	if !pluginNameRegex.MatchString(name) {
		return fmt.Errorf("invalid plugin name %q", name)
	}
	state, err := ReadState(statePath)
	if err != nil {
		return err
	}
	entry, ok := state.Plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not installed", name)
	}
	if err := os.Remove(entry.BinaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove binary: %w", err)
	}
	delete(state.Plugins, name)
	return WriteState(statePath, state)
}

// VerifyInstalled re-hashes the binary on disk and returns an error if it
// no longer matches the stored sha256, or the file is missing.
func VerifyInstalled(entry InstalledPlugin) error {
	sum, err := sha256File(entry.BinaryPath)
	if err != nil {
		return fmt.Errorf("hash %s: %w", entry.BinaryPath, err)
	}
	if sum != entry.SHA256 {
		return fmt.Errorf("sha256 mismatch for %s: expected %s, got %s",
			entry.BinaryPath, entry.SHA256[:12], sum[:12])
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFileTo(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func hasDotDotPrefix(rel string) bool {
	return len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}
