// Package cvtservice provides TLS configuration for the CVT server.
package cvtservice

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
)

// TLSConfig holds TLS configuration settings.
type TLSConfig struct {
	// Enabled indicates whether TLS is enabled.
	Enabled bool

	// CertFile is the path to the server certificate file.
	CertFile string

	// KeyFile is the path to the server private key file.
	KeyFile string

	// CAFile is the path to the CA certificate for client verification (mTLS).
	// If empty, client certificates are not required.
	CAFile string

	// ClientAuth specifies the client authentication policy.
	// Valid values: "none", "request", "require"
	ClientAuth string
}

// Environment variable names for TLS configuration.
const (
	EnvTLSEnabled    = "CVT_TLS_ENABLED"
	EnvTLSCertFile   = "CVT_TLS_CERT_FILE"
	EnvTLSKeyFile    = "CVT_TLS_KEY_FILE"
	EnvTLSCAFile     = "CVT_TLS_CA_FILE"
	EnvTLSClientAuth = "CVT_TLS_CLIENT_AUTH"
)

// Default paths for TLS certificates.
const (
	DefaultCertFile = "/certs/server.crt"
	DefaultKeyFile  = "/certs/server.key"
)

// LoadTLSConfigFromEnv loads TLS configuration from environment variables.
func LoadTLSConfigFromEnv() (*TLSConfig, error) {
	config := &TLSConfig{
		Enabled:    os.Getenv(EnvTLSEnabled) == "true",
		CertFile:   getEnvOrDefault(EnvTLSCertFile, DefaultCertFile),
		KeyFile:    getEnvOrDefault(EnvTLSKeyFile, DefaultKeyFile),
		CAFile:     os.Getenv(EnvTLSCAFile),
		ClientAuth: getEnvOrDefault(EnvTLSClientAuth, "none"),
	}

	// Validate client auth setting
	switch config.ClientAuth {
	case "none", "request", "require":
		// Valid
	default:
		return nil, fmt.Errorf("invalid CVT_TLS_CLIENT_AUTH value: %s (must be none, request, or require)", config.ClientAuth)
	}

	return config, nil
}

// LoadTLSCredentials loads TLS credentials for the gRPC server.
func LoadTLSCredentials(config *TLSConfig) (credentials.TransportCredentials, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("TLS is not enabled")
	}

	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS12,
	}

	// Configure client authentication if CA file is provided
	if config.CAFile != "" {
		caCert, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		tlsConfig.ClientCAs = caCertPool

		// Set client auth type based on configuration
		switch config.ClientAuth {
		case "request":
			tlsConfig.ClientAuth = tls.RequestClientCert
			Info("TLS client auth: request")
		case "require":
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			Info("TLS client auth: require and verify")
		default:
			tlsConfig.ClientAuth = tls.NoClientCert
		}
	}

	Info("TLS credentials loaded",
		zap.String("certFile", config.CertFile),
		zap.String("keyFile", config.KeyFile),
		zap.String("caFile", config.CAFile),
		zap.String("clientAuth", config.ClientAuth))

	return credentials.NewTLS(tlsConfig), nil
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
