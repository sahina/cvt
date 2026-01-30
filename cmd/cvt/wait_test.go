package main

import (
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
