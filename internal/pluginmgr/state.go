package pluginmgr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	Version int                        `json:"version"`
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
//
// Note: ReadState does NOT hold a lock. Pure reads are acceptable
// (stale-read tolerance for list/inspect). Mutating callers must use
// withStateLock to serialize read-modify-write cycles.
func ReadState(path string) (*StateFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyState(), nil
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

// WriteState writes state to the given path. Acquires the flock before
// writing. If you're doing a read-modify-write cycle, prefer
// withStateLock so the lock is held across both operations (otherwise
// concurrent mutators race to clobber each other's entries).
func WriteState(path string, state *StateFile) error {
	return withStateLock(path, func(_ *StateFile) (*StateFile, error) {
		return state, nil
	})
}

// withStateLock runs fn under an exclusive file lock on the state file.
// fn receives the current state (or an empty state if the file doesn't
// exist) and returns the new state to persist; returning an error from
// fn aborts without writing. The write is atomic via temp-file + rename.
//
// Callers use this to make Install / Remove transactional across
// concurrent CLI invocations: two simultaneous `cvt plugins install`
// calls no longer race to clobber each other's entries.
func withStateLock(path string, fn func(*StateFile) (*StateFile, error)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
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
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()

	// Read-under-lock: ReadState itself doesn't lock, but we're holding
	// the flock exclusively here, so no concurrent mutator can interleave.
	current, err := ReadState(path)
	if err != nil {
		return err
	}

	next, err := fn(current)
	if err != nil {
		return err
	}
	if next == nil {
		return fmt.Errorf("withStateLock: callback returned nil state")
	}

	return writeStateUnderLock(path, next)
}

// writeStateUnderLock persists state to disk. Caller MUST already hold
// the flock.
func writeStateUnderLock(path string, state *StateFile) error {
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
		_ = tmp.Close()
		removeTmp()
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
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

func emptyState() *StateFile {
	return &StateFile{Version: CurrentStateVersion, Plugins: map[string]InstalledPlugin{}}
}

// Install copies (or verifies) the binary at srcPath into pluginRoot and
// records the entry in state.json under a single flock-guarded
// transaction. Returns the persisted entry.
//
// Behavior:
//   - If srcPath is already inside pluginRoot, the binary stays in place
//     and only its SHA256 is computed.
//   - Otherwise the binary is copied to pluginRoot/<basename>(src). The
//     copy is staged to a temp file and atomically renamed, so a crashed
//     or concurrent install never leaves a half-written binary.
//   - If the destination path is already registered under a DIFFERENT
//     plugin name, Install refuses: the user must pick a distinct
//     binary file or remove the existing plugin first.
//   - name must match the plugin-name regex.
func Install(srcPath, name, pluginRoot, statePath string) (InstalledPlugin, error) {
	if !pluginNameRegex.MatchString(name) {
		return InstalledPlugin{}, fmt.Errorf("invalid plugin name %q", name)
	}

	absSrc, err := filepath.Abs(srcPath)
	if err != nil {
		return InstalledPlugin{}, fmt.Errorf("resolve source: %w", err)
	}
	// Lstat (not Stat) — symlinks inside pluginRoot pointing outside it
	// would otherwise pass the insideRoot check but exec their target at
	// runtime, silently defeating the path policy. Install is the
	// documented trust boundary; reject symlinks here.
	info, err := os.Lstat(absSrc)
	if err != nil {
		return InstalledPlugin{}, fmt.Errorf("stat source: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return InstalledPlugin{}, fmt.Errorf("source is a symlink: %s", absSrc)
	}
	if info.IsDir() {
		return InstalledPlugin{}, fmt.Errorf("source is a directory: %s", absSrc)
	}

	root := filepath.Clean(pluginRoot)
	// 0o700: only the CVT-invoking user should enumerate installed plugins.
	// Plugin binaries inside use 0o700 as well. MkdirAll is a no-op if the
	// directory already exists, so a pre-existing 0o755 pluginRoot stays
	// 0o755 — explicitly Chmod so repeat installs tighten loose perms.
	if err := os.MkdirAll(root, 0o700); err != nil {
		return InstalledPlugin{}, fmt.Errorf("mkdir plugin root: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return InstalledPlugin{}, fmt.Errorf("chmod plugin root: %w", err)
	}

	var destPath string
	rel, relErr := filepath.Rel(root, absSrc)
	insideRoot := relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))

	var entry InstalledPlugin
	txErr := withStateLock(statePath, func(state *StateFile) (*StateFile, error) {
		if insideRoot {
			destPath = absSrc
		} else {
			destPath = filepath.Join(root, filepath.Base(absSrc))
			// Collision guard: if the dest path is already registered under
			// a different name, refuse rather than silently truncating the
			// other plugin's binary.
			for otherName, other := range state.Plugins {
				if otherName == name {
					continue
				}
				if filepath.Clean(other.BinaryPath) == filepath.Clean(destPath) {
					return nil, fmt.Errorf("binary path %s already used by plugin %q; pick a different source filename or use --name, or remove %q first",
						destPath, otherName, otherName)
				}
			}
			if err := atomicCopyFile(absSrc, destPath, 0o700); err != nil {
				return nil, fmt.Errorf("copy binary: %w", err)
			}
		}

		sum, err := sha256File(destPath)
		if err != nil {
			return nil, fmt.Errorf("sha256: %w", err)
		}
		entry = InstalledPlugin{
			BinaryPath:  destPath,
			SHA256:      sum,
			InstalledAt: time.Now().UTC().Truncate(time.Second),
		}
		state.Plugins[name] = entry
		return state, nil
	})
	if txErr != nil {
		return InstalledPlugin{}, txErr
	}
	return entry, nil
}

// Remove deletes the plugin binary and its state entry under a single
// flock-guarded transaction. Ordering:
//  1. Acquire lock.
//  2. Verify entry exists and binary path is under pluginRoot.
//  3. Write new state (without the entry).
//  4. Release lock.
//  5. Delete the binary (outside lock).
//
// If step 3 fails, nothing changes. If step 5 fails, state is correct
// but the orphaned binary remains on disk — operator can delete by hand
// and reinstall works. The reverse ordering (binary-first) would leave
// state pointing at a missing file, blocking reinstall.
func Remove(name, pluginRoot, statePath string) error {
	if !pluginNameRegex.MatchString(name) {
		return fmt.Errorf("invalid plugin name %q", name)
	}

	var binaryPath string
	txErr := withStateLock(statePath, func(state *StateFile) (*StateFile, error) {
		entry, ok := state.Plugins[name]
		if !ok {
			return nil, fmt.Errorf("plugin %q not installed", name)
		}
		// Defensive: state.json sits in the user's home directory and is
		// writable outside this CLI. If someone rewrote the entry to point
		// at a path outside pluginRoot, refuse rather than letting
		// `cvt plugins remove` become a file-deletion primitive driven
		// by unvalidated JSON.
		if _, err := validateBinaryPath(entry.BinaryPath, pluginRoot); err != nil {
			return nil, fmt.Errorf("plugin %q binary path escapes plugin root: %w", name, err)
		}
		binaryPath = entry.BinaryPath
		delete(state.Plugins, name)
		return state, nil
	})
	if txErr != nil {
		return txErr
	}
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove binary (state already updated): %w", err)
	}
	return nil
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
			entry.BinaryPath, shortHash(entry.SHA256), shortHash(sum))
	}
	return nil
}

// shortHash returns the first 12 hex chars of a hash string, safe for
// truncated strings (no panic on short inputs).
func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
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

// atomicCopyFile copies src to dst via a temp file in dst's directory
// followed by atomic rename. A concurrent copy of the same dst (or a
// crash mid-copy) can never leave a partially-written file at dst: the
// destination only becomes visible once it's complete and fsynced.
func atomicCopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	removeTmp := func() { _ = os.Remove(tmpPath) }

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		removeTmp()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		removeTmp()
		return fmt.Errorf("copy: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		removeTmp()
		return fmt.Errorf("fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		removeTmp()
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		removeTmp()
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
