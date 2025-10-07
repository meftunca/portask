package network

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TLSConfig holds TLS/SSL configuration
type TLSConfig struct {
	Enabled            bool     `yaml:"enabled" json:"enabled"`
	CertFile           string   `yaml:"cert_file" json:"cert_file"`
	KeyFile            string   `yaml:"key_file" json:"key_file"`
	CAFile             string   `yaml:"ca_file" json:"ca_file"`
	ClientAuth         string   `yaml:"client_auth" json:"client_auth"` // "none", "request", "require", "verify"
	MinVersion         string   `yaml:"min_version" json:"min_version"` // "TLS10", "TLS11", "TLS12", "TLS13"
	MaxVersion         string   `yaml:"max_version" json:"max_version"`
	CipherSuites       []string `yaml:"cipher_suites" json:"cipher_suites"`
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
}

// DefaultTLSConfig returns a secure default TLS configuration
func DefaultTLSConfig() *TLSConfig {
	return &TLSConfig{
		Enabled:            false,
		ClientAuth:         "none",
		MinVersion:         "TLS12",
		MaxVersion:         "TLS13",
		InsecureSkipVerify: false,
	}
}

// ProductionTLSConfig returns a production-ready TLS configuration
func ProductionTLSConfig(certFile, keyFile string) *TLSConfig {
	return &TLSConfig{
		Enabled:    true,
		CertFile:   certFile,
		KeyFile:    keyFile,
		ClientAuth: "none",
		MinVersion: "TLS13",
		MaxVersion: "TLS13",
		CipherSuites: []string{
			"TLS_AES_128_GCM_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_CHACHA20_POLY1305_SHA256",
		},
		InsecureSkipVerify: false,
	}
}

// MutualTLSConfig returns a configuration with mutual TLS authentication
func MutualTLSConfig(certFile, keyFile, caFile string) *TLSConfig {
	return &TLSConfig{
		Enabled:    true,
		CertFile:   certFile,
		KeyFile:    keyFile,
		CAFile:     caFile,
		ClientAuth: "require",
		MinVersion: "TLS13",
		MaxVersion: "TLS13",
		CipherSuites: []string{
			"TLS_AES_128_GCM_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_CHACHA20_POLY1305_SHA256",
		},
		InsecureSkipVerify: false,
	}
}

// BuildTLSConfig creates a tls.Config from TLSConfig
func (tc *TLSConfig) BuildTLSConfig() (*tls.Config, error) {
	if !tc.Enabled {
		return nil, nil
	}

	// Load certificate and key
	if tc.CertFile == "" || tc.KeyFile == "" {
		return nil, fmt.Errorf("cert_file and key_file are required when TLS is enabled")
	}

	cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   parseTLSVersion(tc.MinVersion),
		MaxVersion:   parseTLSVersion(tc.MaxVersion),
		CipherSuites: parseCipherSuites(tc.CipherSuites),
		InsecureSkipVerify: tc.InsecureSkipVerify,
	}

	// Configure client authentication
	if tc.ClientAuth != "" && tc.ClientAuth != "none" {
		config.ClientAuth = parseClientAuthType(tc.ClientAuth)

		// Load CA certificate for client verification
		if tc.CAFile != "" {
			caCert, err := os.ReadFile(tc.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA file: %w", err)
			}

			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}

			config.ClientCAs = caCertPool
		}
	}

	return config, nil
}

// parseTLSVersion converts string to tls version constant
func parseTLSVersion(version string) uint16 {
	switch version {
	case "TLS10":
		return tls.VersionTLS10
	case "TLS11":
		return tls.VersionTLS11
	case "TLS12":
		return tls.VersionTLS12
	case "TLS13":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12 // Safe default
	}
}

// parseClientAuthType converts string to tls.ClientAuthType
func parseClientAuthType(authType string) tls.ClientAuthType {
	switch authType {
	case "none":
		return tls.NoClientCert
	case "request":
		return tls.RequestClientCert
	case "require":
		return tls.RequireAnyClientCert
	case "verify":
		return tls.RequireAndVerifyClientCert
	default:
		return tls.NoClientCert
	}
}

// parseCipherSuites converts cipher suite names to constants
func parseCipherSuites(suites []string) []uint16 {
	if len(suites) == 0 {
		return nil // Use default cipher suites
	}

	cipherMap := map[string]uint16{
		// TLS 1.3 cipher suites
		"TLS_AES_128_GCM_SHA256":       tls.TLS_AES_128_GCM_SHA256,
		"TLS_AES_256_GCM_SHA384":       tls.TLS_AES_256_GCM_SHA384,
		"TLS_CHACHA20_POLY1305_SHA256": tls.TLS_CHACHA20_POLY1305_SHA256,

		// TLS 1.2 cipher suites
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	}

	var cipherSuites []uint16
	for _, name := range suites {
		if cipher, ok := cipherMap[name]; ok {
			cipherSuites = append(cipherSuites, cipher)
		}
	}

	return cipherSuites
}

// ValidateTLSConfig validates TLS configuration
func (tc *TLSConfig) Validate() error {
	if !tc.Enabled {
		return nil
	}

	if tc.CertFile == "" {
		return fmt.Errorf("cert_file is required when TLS is enabled")
	}

	if tc.KeyFile == "" {
		return fmt.Errorf("key_file is required when TLS is enabled")
	}

	// Check if files exist
	if _, err := os.Stat(tc.CertFile); os.IsNotExist(err) {
		return fmt.Errorf("certificate file not found: %s", tc.CertFile)
	}

	if _, err := os.Stat(tc.KeyFile); os.IsNotExist(err) {
		return fmt.Errorf("key file not found: %s", tc.KeyFile)
	}

	// Validate client auth configuration
	if tc.ClientAuth != "none" && tc.ClientAuth != "request" && 
	   tc.ClientAuth != "require" && tc.ClientAuth != "verify" {
		return fmt.Errorf("invalid client_auth value: %s", tc.ClientAuth)
	}

	if (tc.ClientAuth == "require" || tc.ClientAuth == "verify") && tc.CAFile == "" {
		return fmt.Errorf("ca_file is required when client_auth is %s", tc.ClientAuth)
	}

	if tc.CAFile != "" {
		if _, err := os.Stat(tc.CAFile); os.IsNotExist(err) {
			return fmt.Errorf("CA file not found: %s", tc.CAFile)
		}
	}

	return nil
}

// GenerateSelfSignedCert generates a self-signed certificate (for testing only)
func GenerateSelfSignedCert(certFile, keyFile string) error {
	// This is a placeholder for self-signed certificate generation
	// In production, use proper certificate management tools
	return fmt.Errorf("self-signed certificate generation not implemented - use openssl or certbot")
}

// TLSConfigExample provides example configurations
func TLSConfigExample() string {
	return `
# TLS Configuration Examples:

## 1. Basic TLS (Server Certificate Only)
tls:
  enabled: true
  cert_file: "/path/to/server.crt"
  key_file: "/path/to/server.key"
  min_version: "TLS12"
  max_version: "TLS13"

## 2. Mutual TLS (Client + Server Authentication)
tls:
  enabled: true
  cert_file: "/path/to/server.crt"
  key_file: "/path/to/server.key"
  ca_file: "/path/to/ca.crt"
  client_auth: "require"
  min_version: "TLS13"

## 3. Production-Grade Configuration
tls:
  enabled: true
  cert_file: "/etc/portask/tls/server.crt"
  key_file: "/etc/portask/tls/server.key"
  min_version: "TLS13"
  max_version: "TLS13"
  cipher_suites:
    - "TLS_AES_128_GCM_SHA256"
    - "TLS_AES_256_GCM_SHA384"
    - "TLS_CHACHA20_POLY1305_SHA256"
  insecure_skip_verify: false

## Generate Self-Signed Certificates (Testing Only):
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes -subj "/CN=localhost"
`
}

