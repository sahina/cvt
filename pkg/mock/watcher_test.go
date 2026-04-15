package mock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_FileChange(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "test.json")
	if err := os.WriteFile(schemaPath, []byte(`{"openapi":"3.0.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`), 0644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	reloaded := make(chan struct{}, 1)
	w, err := NewWatcher([]string{schemaPath}, func() {
		select {
		case reloaded <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(schemaPath, []byte(`{"openapi":"3.0.0","info":{"title":"Updated","version":"2.0.0"},"paths":{}}`), 0644); err != nil {
		t.Fatalf("write updated schema: %v", err)
	}

	select {
	case <-reloaded:
		// success
	case <-time.After(3 * time.Second):
		t.Error("expected reload callback within 3 seconds")
	}
}

func TestWatcher_IgnoresUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "watched.json")
	otherPath := filepath.Join(dir, "other.json")

	if err := os.WriteFile(schemaPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	reloaded := make(chan struct{}, 1)
	w, err := NewWatcher([]string{schemaPath}, func() {
		select {
		case reloaded <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(otherPath, []byte(`{"other": true}`), 0644); err != nil {
		t.Fatalf("write other file: %v", err)
	}

	select {
	case <-reloaded:
		t.Error("should not reload for unrelated file change")
	case <-time.After(1 * time.Second):
		// success
	}
}
