package mock

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceInterval = 500 * time.Millisecond

// Watcher watches schema files for changes and triggers a reload callback.
type Watcher struct {
	watcher *fsnotify.Watcher
	done    chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	timer   *time.Timer
}

// NewWatcher creates a file watcher that calls onReload when any watched file changes.
func NewWatcher(files []string, onReload func()) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch parent directories (handles vim/emacs atomic writes via RENAME/CREATE)
	watchedDirs := make(map[string]bool)
	fileSet := make(map[string]bool)

	for _, f := range files {
		absPath, err := filepath.Abs(f)
		if err != nil {
			absPath = f
		}
		fileSet[absPath] = true

		dir := filepath.Dir(absPath)
		if !watchedDirs[dir] {
			if err := fw.Add(dir); err != nil {
				_ = fw.Close()
				return nil, err
			}
			watchedDirs[dir] = true
		}
	}

	w := &Watcher{
		watcher: fw,
		done:    make(chan struct{}),
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		for {
			select {
			case event, ok := <-fw.Events:
				if !ok {
					return
				}

				absPath, err := filepath.Abs(event.Name)
				if err != nil {
					absPath = event.Name
				}

				if !fileSet[absPath] {
					continue
				}

				// React to Write, Create, and Rename (covers vim/emacs atomic writes)
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}

				w.mu.Lock()
				if w.timer != nil {
					w.timer.Stop()
				}
				w.timer = time.AfterFunc(debounceInterval, func() {
					select {
					case <-w.done:
						return
					default:
						onReload()
					}
				})
				w.mu.Unlock()

			case _, ok := <-fw.Errors:
				if !ok {
					return
				}

			case <-w.done:
				return
			}
		}
	}()

	return w, nil
}

// Close stops the watcher.
func (w *Watcher) Close() {
	close(w.done)
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.mu.Unlock()
	_ = w.watcher.Close()
	w.wg.Wait()
}
