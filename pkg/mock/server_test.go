package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func getFreePort() int {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func TestServer_Integration_BasicMocking(t *testing.T) {
	// Write testSchema to a temp file
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "api.json")
	os.WriteFile(schemaPath, []byte(testSchema), 0644)

	port := getFreePort()
	srv := NewServer(ServerConfig{
		Host:        "127.0.0.1",
		Port:        port,
		SchemaFiles: []string{schemaPath},
		UseExamples: true,
		Quiet:       true,
	})

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get(baseURL + "/")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server did not start in time")
	}

	// Test: GET /users returns 200 with JSON
	resp, err := http.Get(baseURL + "/users")
	if err != nil {
		t.Fatalf("GET /users failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /users: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Test: GET /users/42 returns 200
	resp2, err := http.Get(baseURL + "/users/42")
	if err != nil {
		t.Fatalf("GET /users/42 failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("GET /users/42: expected 200, got %d", resp2.StatusCode)
	}

	// Test: GET /unknown returns 404
	resp3, err := http.Get(baseURL + "/unknown")
	if err != nil {
		t.Fatalf("GET /unknown failed: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != 404 {
		t.Errorf("GET /unknown: expected 404, got %d", resp3.StatusCode)
	}

	// Test: GET / returns index page (HTML)
	resp4, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("GET /: expected 200, got %d", resp4.StatusCode)
	}
	if ct := resp4.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("GET /: expected text/html, got %s", ct)
	}

	// Test: CORS headers present
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS headers on response")
	}

	// Shutdown
	srv.httpSrv.Close()
}

func TestServer_SchemaIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"openapi.json", "openapi"},
		{"/path/to/api.yaml", "api"},
		{"spec.yml", "spec"},
		{"https://example.com/openapi.json", "openapi"},
		{"", "schema"},
	}

	for _, tt := range tests {
		got := schemaIDFromPath(tt.path)
		if got != tt.want {
			t.Errorf("schemaIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
