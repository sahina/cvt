package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestCheckHealth_ServerNotRunning(t *testing.T) {
	// Test that checkHealth returns an error when no server is running
	err := checkHealth("localhost:59999") // Use a port that's unlikely to be in use
	if err == nil {
		t.Error("expected error when server is not running, got nil")
	}
}

func TestCheckHealth_InvalidAddress(t *testing.T) {
	// Test with an invalid address
	err := checkHealth("invalid:address:format")
	if err == nil {
		t.Error("expected error with invalid address, got nil")
	}
}

func TestWaitCmd_Flags(t *testing.T) {
	cmd := waitCmd()

	// Verify command metadata
	if cmd.Use != "wait" {
		t.Errorf("expected Use to be 'wait', got %q", cmd.Use)
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
		{"timeout", "t", "60"},
		{"interval", "i", "2"},
		{"quiet", "q", "false"},
		{"json", "j", "false"},
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

func TestOutputWaitJSON_Success(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputWaitJSON("localhost:9550", "ready", 3, nil)

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

	if result["status"] != "ready" {
		t.Errorf("expected status='ready', got %v", result["status"])
	}
	if result["server"] != "localhost:9550" {
		t.Errorf("expected server='localhost:9550', got %v", result["server"])
	}
	if result["attempts"] != float64(3) {
		t.Errorf("expected attempts=3, got %v", result["attempts"])
	}
	if _, hasError := result["error"]; hasError {
		t.Error("expected no error field in success case")
	}
}

func TestOutputWaitJSON_Timeout(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	timeoutErr := errors.New("timeout: server not ready")
	err := outputWaitJSON("localhost:9550", "timeout", 5, timeoutErr)

	_ = w.Close()
	os.Stdout = oldStdout

	// Should return the original error
	if err == nil || err.Error() != "timeout: server not ready" {
		t.Errorf("expected timeout error, got %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("failed to parse JSON output: %v", err)
	}

	if result["status"] != "timeout" {
		t.Errorf("expected status='timeout', got %v", result["status"])
	}
	if result["error"] != "timeout: server not ready" {
		t.Errorf("expected error message, got %v", result["error"])
	}
}
