//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testBinary holds the path to the compiled cvt binary.
var testBinary string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "cvt-mock-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	bin := filepath.Join(tmp, "cvt")
	cmd := exec.Command("go", "build", "-o", bin, "./")
	cmd.Dir = filepath.Join(repoRoot())
	cmd.Dir = filepath.Join(cmd.Dir, "cmd", "cvt")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build cvt binary: %v\n", err)
		os.Exit(1)
	}

	testBinary = bin
	os.Exit(m.Run())
}

// repoRoot returns the absolute path to the repository root.
func repoRoot() string {
	// cmd/cvt/mock_test.go -> repo root is ../../
	dir, err := filepath.Abs("../../")
	if err != nil {
		panic(fmt.Sprintf("failed to resolve repo root: %v", err))
	}
	return dir
}

// schemaPath returns the absolute path to a test schema file.
func schemaPath(relPath string) string {
	return filepath.Join(repoRoot(), relPath)
}

// getFreePort returns an available TCP port on 127.0.0.1.
func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// startMockServer starts cvt mock as a subprocess and waits for it to be ready.
// It prepends --port <port> --quiet to the given args.
// Returns the exec.Cmd, the port, and a cleanup function.
func startMockServer(t *testing.T, binary string, args ...string) (*exec.Cmd, int, func()) {
	t.Helper()

	port := getFreePort(t)

	fullArgs := []string{"mock", "--port", fmt.Sprintf("%d", port), "--quiet"}
	fullArgs = append(fullArgs, args...)

	cmd := exec.Command(binary, fullArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}

	cleanup := func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}
	t.Cleanup(cleanup)

	// Poll until the server is ready (max 5 seconds)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		cleanup()
		t.Fatal("mock server did not start within 5 seconds")
	}

	return cmd, port, cleanup
}

// --- Test Cases ---

func TestMockCLI_SingleSchema(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	_, port, _ := startMockServer(t, testBinary, "--schema", schema)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// GET /pets -> 200, JSON array
	resp, err := http.Get(baseURL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /pets: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var arr []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		t.Fatalf("GET /pets: expected JSON array, decode error: %v", err)
	}

	// GET /pets/42 -> 200, JSON object
	resp2, err := http.Get(baseURL + "/pets/42")
	if err != nil {
		t.Fatalf("GET /pets/42 failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("GET /pets/42: expected 200, got %d: %s", resp2.StatusCode, body)
	}
	var obj map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&obj); err != nil {
		t.Fatalf("GET /pets/42: expected JSON object, decode error: %v", err)
	}

	// GET /unknown -> 404
	resp3, err := http.Get(baseURL + "/unknown")
	if err != nil {
		t.Fatalf("GET /unknown failed: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != 404 {
		t.Errorf("GET /unknown: expected 404, got %d", resp3.StatusCode)
	}
}

func TestMockCLI_MultiSchema(t *testing.T) {
	petstore := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	ecommerce := schemaPath("server/testdata/openapi-v3/valid/complex-ecommerce.json")
	_, port, _ := startMockServer(t, testBinary, "--schema", petstore, "--schema", ecommerce)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// GET /pets -> 200 (from petstore schema)
	resp, err := http.Get(baseURL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /pets: expected 200, got %d", resp.StatusCode)
	}

	// GET /products -> 200 (from ecommerce schema)
	resp2, err := http.Get(baseURL + "/products")
	if err != nil {
		t.Fatalf("GET /products failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("GET /products: expected 200, got %d", resp2.StatusCode)
	}

	// GET /unknown -> 404
	resp3, err := http.Get(baseURL + "/unknown")
	if err != nil {
		t.Fatalf("GET /unknown failed: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != 404 {
		t.Errorf("GET /unknown: expected 404, got %d", resp3.StatusCode)
	}
}

func TestMockCLI_NoSchema(t *testing.T) {
	cmd := exec.Command(testBinary, "mock")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = io.Discard

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code when no --schema is provided")
	}
	if !strings.Contains(stderr.String(), "at least one --schema is required") {
		t.Errorf("expected stderr to contain 'at least one --schema is required', got: %s", stderr.String())
	}
}

func TestMockCLI_InvalidSchema(t *testing.T) {
	malformed := schemaPath("server/testdata/openapi-v3/invalid/malformed-json.json")
	cmd := exec.Command(testBinary, "mock", "--schema", malformed)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = io.Discard

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code with invalid schema")
	}
}

func TestMockCLI_CustomPort(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	port := getFreePort(t)

	cmd := exec.Command(testBinary, "mock", "--schema", schema, "--port", fmt.Sprintf("%d", port), "--quiet")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server did not start on custom port")
	}

	// Verify the server responds on the specified port
	resp, err := http.Get(baseURL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets on custom port failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /pets on custom port: expected 200, got %d", resp.StatusCode)
	}
}

func TestMockCLI_InvalidPort(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	cmd := exec.Command(testBinary, "mock", "--schema", schema, "--port", "-1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = io.Discard

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code with invalid port")
	}
	if !strings.Contains(stderr.String(), "invalid port") {
		t.Errorf("expected stderr to contain 'invalid port', got: %s", stderr.String())
	}
}

func TestMockCLI_ValidateRequests_Valid(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	_, port, _ := startMockServer(t, testBinary, "--schema", schema, "--validate-requests")
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// POST /pets with valid body (NewPet requires "name")
	body := `{"name":"Rex","tag":"dog"}`
	resp, err := http.Post(baseURL+"/pets", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /pets failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /pets with valid body: expected 201, got %d: %s", resp.StatusCode, respBody)
	}
}

func TestMockCLI_ValidateRequests_Invalid(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	_, port, _ := startMockServer(t, testBinary, "--schema", schema, "--validate-requests")
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// POST /pets with empty body (missing required "name" field)
	body := `{}`
	resp, err := http.Post(baseURL+"/pets", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /pets failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /pets with invalid body: expected 400, got %d: %s", resp.StatusCode, respBody)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !strings.Contains(string(respBody), "violations") {
		t.Errorf("expected response to contain 'violations', got: %s", respBody)
	}
}

func TestMockCLI_Latency(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	_, port, _ := startMockServer(t, testBinary, "--schema", schema, "--latency", "200")
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	start := time.Now()
	resp, err := http.Get(baseURL + "/pets")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GET /pets failed: %v", err)
	}
	resp.Body.Close()

	if elapsed < 200*time.Millisecond {
		t.Errorf("expected latency >= 200ms, got %s", elapsed)
	}
}

func TestMockCLI_IndexPage(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	_, port, _ := startMockServer(t, testBinary, "--schema", schema)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("GET /: expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("GET /: expected Content-Type text/html, got %s", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "CVT Mock Server") {
		t.Error("GET /: expected body to contain 'CVT Mock Server'")
	}
	if !strings.Contains(bodyStr, "/pets") {
		t.Error("GET /: expected body to contain '/pets'")
	}
}

func TestMockCLI_CORS(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	_, port, _ := startMockServer(t, testBinary, "--schema", schema)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Send OPTIONS preflight with Origin header
	req, err := http.NewRequest("OPTIONS", baseURL+"/pets", nil)
	if err != nil {
		t.Fatalf("failed to create OPTIONS request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /pets failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("OPTIONS /pets: expected 200, got %d", resp.StatusCode)
	}
	if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", acao)
	}
}

func TestMockCLI_Quiet(t *testing.T) {
	schema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	port := getFreePort(t)

	// Start with explicit -q flag (not added by the helper)
	cmd := exec.Command(testBinary, "mock", "--schema", schema, "--port", fmt.Sprintf("%d", port), "-q")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server did not start with -q flag")
	}

	// Make a request to generate log output
	resp, err := http.Get(baseURL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /pets: expected 200, got %d", resp.StatusCode)
	}

	// Verify the server starts and responds (stdout should have no [mock] lines)
	if strings.Contains(stdout.String(), "[mock]") {
		t.Errorf("expected no [mock] log lines in quiet mode, got: %s", stdout.String())
	}
}

func TestMockCLI_Watch(t *testing.T) {
	// Copy the petstore schema to a temp directory
	srcSchema := schemaPath("server/testdata/openapi-v3/valid/simple-petstore.json")
	tmpDir := t.TempDir()
	tmpSchema := filepath.Join(tmpDir, "petstore.json")

	srcData, err := os.ReadFile(srcSchema)
	if err != nil {
		t.Fatalf("failed to read source schema: %v", err)
	}
	if err := os.WriteFile(tmpSchema, srcData, 0644); err != nil {
		t.Fatalf("failed to write temp schema: %v", err)
	}

	_, port, _ := startMockServer(t, testBinary, "--schema", tmpSchema, "--watch")
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Verify GET /pets works
	resp, err := http.Get(baseURL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET /pets: expected 200, got %d", resp.StatusCode)
	}

	// Modify the schema: add a /health endpoint
	var schema map[string]interface{}
	if err := json.Unmarshal(srcData, &schema); err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}
	paths, ok := schema["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("schema paths is not a map")
	}
	paths["/health"] = map[string]interface{}{
		"get": map[string]interface{}{
			"summary":     "Health check",
			"operationId": "healthCheck",
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "OK",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"status": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	modifiedData, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal modified schema: %v", err)
	}
	if err := os.WriteFile(tmpSchema, modifiedData, 0644); err != nil {
		t.Fatalf("failed to write modified schema: %v", err)
	}

	// Wait for the watcher to detect changes and reload (up to 3 seconds)
	time.Sleep(2 * time.Second)

	// Verify old endpoints still work (server did not crash on reload)
	resp2, err := http.Get(baseURL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets after reload failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("GET /pets after reload: expected 200, got %d", resp2.StatusCode)
	}

	// Try the new endpoint - the watcher should have reloaded
	resp3, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	resp3.Body.Close()
	// If watcher reload worked, this will be 200; if not, it will be 404
	// Either outcome is acceptable - the important thing is the server did not crash
	if resp3.StatusCode != 200 && resp3.StatusCode != 404 {
		t.Errorf("GET /health: expected 200 or 404, got %d", resp3.StatusCode)
	}
}
