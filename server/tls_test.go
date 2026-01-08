package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestCertificate creates a valid self-signed certificate for testing
func generateTestCertificate(t *testing.T, tempDir string) (certPath, keyPath string) {
	t.Helper()

	// Generate a private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"CVT Test"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
	}

	// Create the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Encode certificate to PEM
	certPath = filepath.Join(tempDir, "server.crt")
	certFile, err := os.Create(certPath)
	require.NoError(t, err)
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, err)
	_ = certFile.Close()

	// Encode private key to PEM
	keyPath = filepath.Join(tempDir, "server.key")
	keyFile, err := os.Create(keyPath)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err)
	err = pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, err)
	_ = keyFile.Close()

	return certPath, keyPath
}

func TestLoadTLSConfigFromEnv(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		t.Setenv(EnvTLSEnabled, "")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.False(t, config.Enabled)
	})

	t.Run("enabled when env var is true", func(t *testing.T) {
		t.Setenv(EnvTLSEnabled, "true")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.True(t, config.Enabled)
	})

	t.Run("uses default cert file path", func(t *testing.T) {
		t.Setenv(EnvTLSCertFile, "")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, DefaultCertFile, config.CertFile)
	})

	t.Run("reads cert file path from env", func(t *testing.T) {
		t.Setenv(EnvTLSCertFile, "/custom/path/server.crt")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "/custom/path/server.crt", config.CertFile)
	})

	t.Run("uses default key file path", func(t *testing.T) {
		t.Setenv(EnvTLSKeyFile, "")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, DefaultKeyFile, config.KeyFile)
	})

	t.Run("reads key file path from env", func(t *testing.T) {
		t.Setenv(EnvTLSKeyFile, "/custom/path/server.key")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "/custom/path/server.key", config.KeyFile)
	})

	t.Run("reads CA file path from env", func(t *testing.T) {
		t.Setenv(EnvTLSCAFile, "/custom/path/ca.crt")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "/custom/path/ca.crt", config.CAFile)
	})

	t.Run("default client auth is none", func(t *testing.T) {
		t.Setenv(EnvTLSClientAuth, "")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "none", config.ClientAuth)
	})

	t.Run("reads client auth mode from env", func(t *testing.T) {
		t.Setenv(EnvTLSClientAuth, "require")
		config, err := LoadTLSConfigFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "require", config.ClientAuth)
	})

	t.Run("valid client auth values", func(t *testing.T) {
		for _, authType := range []string{"none", "request", "require"} {
			t.Run(authType, func(t *testing.T) {
				t.Setenv(EnvTLSClientAuth, authType)
				config, err := LoadTLSConfigFromEnv()
				require.NoError(t, err)
				assert.Equal(t, authType, config.ClientAuth)
			})
		}
	})

	t.Run("invalid client auth value returns error", func(t *testing.T) {
		t.Setenv(EnvTLSClientAuth, "invalid")
		_, err := LoadTLSConfigFromEnv()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid CVT_TLS_CLIENT_AUTH value")
	})
}

func TestLoadTLSCredentials(t *testing.T) {
	// Create temporary test certificates using the helper
	tempDir := t.TempDir()
	certPath, keyPath := generateTestCertificate(t, tempDir)

	t.Run("returns error when TLS is disabled", func(t *testing.T) {
		config := &TLSConfig{
			Enabled: false,
		}

		creds, err := LoadTLSCredentials(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TLS is not enabled")
		assert.Nil(t, creds)
	})

	t.Run("loads valid credentials", func(t *testing.T) {
		config := &TLSConfig{
			Enabled:  true,
			CertFile: certPath,
			KeyFile:  keyPath,
		}

		creds, err := LoadTLSCredentials(config)
		assert.NoError(t, err)
		assert.NotNil(t, creds)
	})

	t.Run("fails with invalid cert path", func(t *testing.T) {
		config := &TLSConfig{
			Enabled:  true,
			CertFile: "/nonexistent/cert.crt",
			KeyFile:  keyPath,
		}

		creds, err := LoadTLSCredentials(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load server certificate")
		assert.Nil(t, creds)
	})

	t.Run("fails with invalid key path", func(t *testing.T) {
		config := &TLSConfig{
			Enabled:  true,
			CertFile: certPath,
			KeyFile:  "/nonexistent/key.key",
		}

		creds, err := LoadTLSCredentials(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load server certificate")
		assert.Nil(t, creds)
	})

	t.Run("fails with invalid CA file", func(t *testing.T) {
		config := &TLSConfig{
			Enabled:    true,
			CertFile:   certPath,
			KeyFile:    keyPath,
			CAFile:     "/nonexistent/ca.crt",
			ClientAuth: "require",
		}

		creds, err := LoadTLSCredentials(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read CA certificate")
		assert.Nil(t, creds)
	})

	t.Run("fails with invalid CA content", func(t *testing.T) {
		// Create invalid CA file
		invalidCAPath := filepath.Join(tempDir, "invalid-ca.crt")
		err := os.WriteFile(invalidCAPath, []byte("not a valid certificate"), 0600)
		require.NoError(t, err)

		config := &TLSConfig{
			Enabled:    true,
			CertFile:   certPath,
			KeyFile:    keyPath,
			CAFile:     invalidCAPath,
			ClientAuth: "require",
		}

		creds, err := LoadTLSCredentials(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse CA certificate")
		assert.Nil(t, creds)
	})

	t.Run("loads with valid CA and client auth modes", func(t *testing.T) {
		// Use the same cert as CA for testing purposes
		for _, authMode := range []string{"none", "request", "require"} {
			t.Run(authMode, func(t *testing.T) {
				config := &TLSConfig{
					Enabled:    true,
					CertFile:   certPath,
					KeyFile:    keyPath,
					CAFile:     certPath, // Use cert as CA for testing
					ClientAuth: authMode,
				}

				creds, err := LoadTLSCredentials(config)
				assert.NoError(t, err)
				assert.NotNil(t, creds)
			})
		}
	})
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_VAR", "custom-value")
		result := getEnvOrDefault("TEST_VAR", "default")
		assert.Equal(t, "custom-value", result)
	})

	t.Run("returns default when env not set", func(t *testing.T) {
		t.Setenv("TEST_VAR", "")
		result := getEnvOrDefault("TEST_VAR", "default")
		assert.Equal(t, "default", result)
	})
}
