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

	"github.com/sahina/cvt/pkg/cvt"
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

func TestFilterFilePaths(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "mix of files and URLs",
			input: []string{"./openapi.json", "https://example.com/api.json", "/abs/path.yaml", "http://localhost/spec.json"},
			want:  []string{"./openapi.json", "/abs/path.yaml"},
		},
		{
			name:  "all URLs",
			input: []string{"https://a.com/x.json", "http://b.com/y.json"},
			want:  nil,
		},
		{
			name:  "all files",
			input: []string{"a.json", "b.yaml"},
			want:  []string{"a.json", "b.yaml"},
		},
		{
			name:  "empty input",
			input: []string{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterFilePaths(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("filterFilePaths(%v) = %v, want nil", tt.input, got)
				}
			} else {
				if len(got) != len(tt.want) {
					t.Fatalf("filterFilePaths(%v) = %v, want %v", tt.input, got, tt.want)
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("filterFilePaths(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"http://example.com/api.json", true},
		{"https://example.com/api.json", true},
		{"./openapi.json", false},
		{"/abs/path.yaml", false},
		{"openapi.json", false},
		{"ftp://example.com/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isURL(tt.input)
			if got != tt.want {
				t.Errorf("isURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectOverlaps(t *testing.T) {
	// Create two schemas that both define GET /users
	schema1 := `{
	  "openapi": "3.0.0",
	  "info": {"title": "API One", "version": "1.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers1",
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {"type": "array", "items": {"type": "object", "properties": {"id": {"type": "integer"}}}}
	              }
	            }
	          }
	        }
	      }
	    }
	  }
	}`
	schema2 := `{
	  "openapi": "3.0.0",
	  "info": {"title": "API Two", "version": "1.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers2",
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {"type": "array", "items": {"type": "object", "properties": {"id": {"type": "integer"}}}}
	              }
	            }
	          }
	        }
	      }
	    }
	  }
	}`

	v := cvt.NewValidator()
	if err := v.RegisterSchema("api-one", []byte(schema1)); err != nil {
		t.Fatalf("register schema1: %v", err)
	}
	if err := v.RegisterSchema("api-two", []byte(schema2)); err != nil {
		t.Fatalf("register schema2: %v", err)
	}

	// Test with quiet=false to exercise the overlap warning branch
	srv := NewServer(ServerConfig{Quiet: false})
	// Should not panic
	srv.detectOverlaps(v, []string{"api-one", "api-two"})

	// Test with quiet=true (early return)
	srvQuiet := NewServer(ServerConfig{Quiet: true})
	srvQuiet.detectOverlaps(v, []string{"api-one", "api-two"})

	// Test with single schema (early return on len < 2)
	srv.detectOverlaps(v, []string{"api-one"})
}

func TestPrintBanner_NotQuiet(t *testing.T) {
	v := cvt.NewValidator()
	if err := v.RegisterSchema("test-api", []byte(testSchema)); err != nil {
		t.Fatalf("register schema: %v", err)
	}

	srv := NewServer(ServerConfig{Quiet: false})
	// Should not panic when quiet=false
	srv.printBanner(v, []string{"test-api"})
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
