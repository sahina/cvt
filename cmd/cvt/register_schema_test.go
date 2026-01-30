package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	pb "github.com/sahina/cvt/server/pb"
)

func TestRegisterSchemaCmd_Flags(t *testing.T) {
	cmd := registerSchemaCmd()

	// Verify command metadata
	if cmd.Use != "register-schema <schema-id> <schema-file>" {
		t.Errorf("expected Use to be 'register-schema <schema-id> <schema-file>', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	// Verify flags exist
	tests := []struct {
		name         string
		shorthand    string
		defaultValue string
	}{
		{"server", "S", "localhost:9550"},
		{"version", "v", ""},
		{"check-compatibility", "", "false"},
		{"fail-on-breaking", "", "false"},
		{"json", "j", "false"},
		{"quiet", "q", "false"},
		{"timeout", "t", "30"},
		{"owner", "", ""},
		{"team", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			if flag == nil {
				t.Errorf("flag --%s not found", tt.name)
				return
			}
			if flag.Shorthand != tt.shorthand {
				t.Errorf("expected shorthand %q for --%s, got %q", tt.shorthand, tt.name, flag.Shorthand)
			}
			if flag.DefValue != tt.defaultValue {
				t.Errorf("expected default %q for --%s, got %q", tt.defaultValue, tt.name, flag.DefValue)
			}
		})
	}
}

func TestRegisterSchemaCmd_RequiresArgs(t *testing.T) {
	cmd := registerSchemaCmd()

	// Command should require exactly 2 arguments
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("expected error with no arguments")
	}

	err = cmd.Args(cmd, []string{"only-one"})
	if err == nil {
		t.Error("expected error with only one argument")
	}

	err = cmd.Args(cmd, []string{"schema-id", "schema-file"})
	if err != nil {
		t.Errorf("expected no error with two arguments, got %v", err)
	}

	err = cmd.Args(cmd, []string{"one", "two", "three"})
	if err == nil {
		t.Error("expected error with three arguments")
	}
}

func TestOutputRegisterJSON_Success(t *testing.T) {
	resp := &pb.RegisterSchemaResponse{
		Success: true,
		Message: "Schema registered",
		Metadata: &pb.SchemaMetadata{
			SchemaId:      "test-api",
			SchemaVersion: "1.0.0",
		},
		BreakingChanges: nil,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRegisterJSON(resp, false)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("failed to parse JSON output: %v", err)
	}

	if result["success"] != true {
		t.Errorf("expected success=true, got %v", result["success"])
	}
	if result["schema_id"] != "test-api" {
		t.Errorf("expected schema_id='test-api', got %v", result["schema_id"])
	}
	if result["version"] != "1.0.0" {
		t.Errorf("expected version='1.0.0', got %v", result["version"])
	}
}

func TestOutputRegisterJSON_Failure(t *testing.T) {
	resp := &pb.RegisterSchemaResponse{
		Success: false,
		Message: "Invalid schema",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRegisterJSON(resp, false)

	_ = w.Close()
	os.Stdout = oldStdout

	// Read the output (even though we expect an error)
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if err == nil {
		t.Error("expected error for failed registration")
	}
	if err.Error() != "registration failed" {
		t.Errorf("expected 'registration failed' error, got %v", err)
	}
}

func TestOutputRegisterJSON_WithBreakingChanges(t *testing.T) {
	resp := &pb.RegisterSchemaResponse{
		Success: true,
		Message: "Schema registered with warnings",
		Metadata: &pb.SchemaMetadata{
			SchemaId:      "test-api",
			SchemaVersion: "2.0.0",
		},
		BreakingChanges: []*pb.BreakingChange{
			{
				Type:        pb.BreakingChangeType_ENDPOINT_REMOVED,
				Path:        "/users/{id}",
				Method:      "DELETE",
				Description: "Endpoint was removed",
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRegisterJSON(resp, false)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("failed to parse JSON output: %v", err)
	}

	breakingChanges, ok := result["breaking_changes"].([]interface{})
	if !ok {
		t.Error("expected breaking_changes to be an array")
		return
	}
	if len(breakingChanges) != 1 {
		t.Errorf("expected 1 breaking change, got %d", len(breakingChanges))
	}
}

func TestOutputRegisterJSON_FailOnBreaking(t *testing.T) {
	resp := &pb.RegisterSchemaResponse{
		Success: true,
		Message: "Schema registered",
		BreakingChanges: []*pb.BreakingChange{
			{
				Type:        pb.BreakingChangeType_ENDPOINT_REMOVED,
				Path:        "/users",
				Method:      "GET",
				Description: "Endpoint removed",
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRegisterJSON(resp, true) // failOnBreaking = true

	_ = w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error when failOnBreaking is true and breaking changes exist")
	}
	if err.Error() != "breaking changes detected" {
		t.Errorf("expected 'breaking changes detected' error, got %v", err)
	}
}

func TestOutputRegisterHuman_Success(t *testing.T) {
	resp := &pb.RegisterSchemaResponse{
		Success: true,
		Message: "Schema registered",
		Metadata: &pb.SchemaMetadata{
			SchemaId:      "test-api",
			SchemaVersion: "1.0.0",
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRegisterHuman(resp, "test-api", "openapi.yaml", false, false)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that output contains expected strings
	if !bytes.Contains([]byte(output), []byte("Schema registered successfully")) {
		t.Error("expected output to contain 'Schema registered successfully'")
	}
	if !bytes.Contains([]byte(output), []byte("test-api")) {
		t.Error("expected output to contain schema ID")
	}
}

func TestOutputRegisterHuman_Quiet(t *testing.T) {
	resp := &pb.RegisterSchemaResponse{
		Success: true,
		Message: "Schema registered",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRegisterHuman(resp, "test-api", "openapi.yaml", true, false) // quiet = true

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// In quiet mode, output should be empty for success
	if output != "" {
		t.Errorf("expected empty output in quiet mode, got %q", output)
	}
}

func TestOutputRegisterHuman_Failure(t *testing.T) {
	resp := &pb.RegisterSchemaResponse{
		Success: false,
		Message: "Invalid schema format",
	}

	// Capture stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := outputRegisterHuman(resp, "test-api", "openapi.yaml", false, false)

	_ = w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Error("expected error for failed registration")
	}
}

func TestFetchSchemaFromURL_Success(t *testing.T) {
	expectedContent := `{"openapi": "3.0.0", "info": {"title": "Test API", "version": "1.0.0"}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedContent))
	}))
	defer server.Close()

	content, err := fetchSchemaFromURL(server.URL)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(content))
	}
}

func TestFetchSchemaFromURL_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := fetchSchemaFromURL(server.URL)
	if err == nil {
		t.Error("expected error for 404 response")
	}

	expectedMsg := "failed to fetch schema from URL: server returned 404 Not Found"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestFetchSchemaFromURL_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := fetchSchemaFromURL(server.URL)
	if err == nil {
		t.Error("expected error for 500 response")
	}

	expectedMsg := "failed to fetch schema from URL: server returned 500 Internal Server Error"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestFetchSchemaFromURL_InvalidURL(t *testing.T) {
	_, err := fetchSchemaFromURL("http://localhost:99999/nonexistent")
	if err == nil {
		t.Error("expected error for invalid URL")
	}

	// Error should mention "failed to fetch schema from URL"
	if !bytes.Contains([]byte(err.Error()), []byte("failed to fetch schema from URL")) {
		t.Errorf("expected error to contain 'failed to fetch schema from URL', got %q", err.Error())
	}
}
