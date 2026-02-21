package main

import (
	"testing"
)

func TestServeCmd_CommandMetadata(t *testing.T) {
	cmd := serveCmd()

	if cmd.Use != "serve" {
		t.Errorf("expected Use to be 'serve', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if cmd.Short != "Start the gRPC validation server" {
		t.Errorf("expected Short to be 'Start the gRPC validation server', got %q", cmd.Short)
	}

	if cmd.Long == "" {
		t.Error("expected Long description to be set")
	}

	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestServeCmd_Flags(t *testing.T) {
	cmd := serveCmd()

	tests := []struct {
		name         string
		shorthand    string
		defaultValue string
		flagType     string
	}{
		{"port", "p", "9550", "int"},
		{"metrics-port", "", "9551", "int"},
		{"tls", "", "false", "bool"},
		{"cert", "", "", "string"},
		{"key", "", "", "string"},
		{"api-key-auth", "", "false", "bool"},
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
			if flag.Value.Type() != tt.flagType {
				t.Errorf("expected type %q for --%s, got %q", tt.flagType, tt.name, flag.Value.Type())
			}
		})
	}
}

func TestServeCmd_TLSRequiresCertAndKey(t *testing.T) {
	tests := []struct {
		name     string
		setCert  string
		setKey   string
		errorMsg string
	}{
		{
			name:     "TLS enabled without cert or key",
			setCert:  "",
			setKey:   "",
			errorMsg: "--cert and --key are required",
		},
		{
			name:     "TLS enabled with cert but no key",
			setCert:  "server.crt",
			setKey:   "",
			errorMsg: "--cert and --key are required",
		},
		{
			name:     "TLS enabled with key but no cert",
			setCert:  "",
			setKey:   "server.key",
			errorMsg: "--cert and --key are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear TLS env vars so they don't interfere
			t.Setenv("CVT_TLS_ENABLED", "")
			t.Setenv("CVT_TLS_CERT_FILE", "")
			t.Setenv("CVT_TLS_KEY_FILE", "")
			t.Setenv("CVT_API_KEY_ENABLED", "")

			cmd := serveCmd()

			if err := cmd.Flags().Set("tls", "true"); err != nil {
				t.Fatalf("failed to set tls flag: %v", err)
			}
			if tt.setCert != "" {
				if err := cmd.Flags().Set("cert", tt.setCert); err != nil {
					t.Fatalf("failed to set cert flag: %v", err)
				}
			}
			if tt.setKey != "" {
				if err := cmd.Flags().Set("key", tt.setKey); err != nil {
					t.Fatalf("failed to set key flag: %v", err)
				}
			}

			err := cmd.RunE(cmd, []string{})
			if err == nil {
				t.Error("expected error but got nil")
				return
			}
			if !containsString(err.Error(), tt.errorMsg) {
				t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
