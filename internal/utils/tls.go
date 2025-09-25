package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"

	interfaces "github.com/llm-d-incubation/workload-variant-autoscaler/internal/interfaces"
	"github.com/llm-d-incubation/workload-variant-autoscaler/internal/logger"
)

// CreateTLSConfig creates a TLS configuration from PrometheusConfig.
// TLS is always enabled for HTTPS-only support. The configuration supports:
// - Server certificate validation via CA certificate
// - Mutual TLS authentication via client certificates
// - Insecure certificate verification (development/testing only)
func CreateTLSConfig(promConfig *interfaces.PrometheusConfig) (*tls.Config, error) {
	if promConfig == nil {
		return nil, nil
	}

	config := &tls.Config{
		InsecureSkipVerify: promConfig.InsecureSkipVerify,
		ServerName:         promConfig.ServerName,
		MinVersion:         tls.VersionTLS12, // Enforce minimum TLS version - https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/security_and_compliance/tls-security-profiles#:~:text=requires%20a%20minimum-,TLS%20version%20of%201.2,-.
	}

	// Load CA certificate if provided
	if promConfig.CACertPath != "" {
		caCert, err := os.ReadFile(promConfig.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate from %s: %w", promConfig.CACertPath, err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", promConfig.CACertPath)
		}
		config.RootCAs = caCertPool
		logger.Log.Info("CA certificate loaded successfully", "path", promConfig.CACertPath)
	}

	// Load client certificate and key if provided
	if promConfig.ClientCertPath != "" && promConfig.ClientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(promConfig.ClientCertPath, promConfig.ClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate from %s and key from %s: %w",
				promConfig.ClientCertPath, promConfig.ClientKeyPath, err)
		}
		config.Certificates = []tls.Certificate{cert}
		logger.Log.Info("Client certificate loaded successfully",
			"cert_path", promConfig.ClientCertPath, "key_path", promConfig.ClientKeyPath)
	}

	return config, nil
}

// ValidateTLSConfig validates TLS configuration.
// Ensures HTTPS is used and certificate files exist when verification is enabled.
// Note: This function assumes promConfig is not nil - nil checks should be performed at a higher level.
func ValidateTLSConfig(promConfig *interfaces.PrometheusConfig) error {
	// Validate that the URL uses HTTPS (TLS is always required)
	u, err := url.Parse(promConfig.BaseURL)
	if err != nil || u.Scheme != "https" {
		return fmt.Errorf("HTTPS is required - URL must use https:// scheme: %s", promConfig.BaseURL)
	}

	// If InsecureSkipVerify is true, we don't need to validate certificate files
	// since we're intentionally skipping certificate verification
	if promConfig.InsecureSkipVerify {
		logger.Log.Warn("TLS certificate verification is disabled - this is not recommended for production")
		return nil
	}

	// Check if certificate files exist (only when not skipping verification)
	if promConfig.CACertPath != "" {
		if _, err := os.Stat(promConfig.CACertPath); os.IsNotExist(err) {
			return fmt.Errorf("CA certificate file not found: %s", promConfig.CACertPath)
		}
	}

	if promConfig.ClientCertPath != "" {
		if _, err := os.Stat(promConfig.ClientCertPath); os.IsNotExist(err) {
			return fmt.Errorf("client certificate file not found: %s", promConfig.ClientCertPath)
		}
	}

	if promConfig.ClientKeyPath != "" {
		if _, err := os.Stat(promConfig.ClientKeyPath); os.IsNotExist(err) {
			return fmt.Errorf("client key file not found: %s", promConfig.ClientKeyPath)
		}
	}

	return nil
}

// ParsePrometheusConfigFromEnv parses Prometheus configuration from environment variables.
// Supports both direct values and file paths for flexible deployment scenarios.
func ParsePrometheusConfigFromEnv() *interfaces.PrometheusConfig {
	config := &interfaces.PrometheusConfig{
		BaseURL: os.Getenv("PROMETHEUS_BASE_URL"),
	}

	// TLS is always enabled for HTTPS-only support
	config.InsecureSkipVerify = os.Getenv("PROMETHEUS_TLS_INSECURE_SKIP_VERIFY") == "true"
	config.CACertPath = os.Getenv("PROMETHEUS_CA_CERT_PATH")
	config.ClientCertPath = os.Getenv("PROMETHEUS_CLIENT_CERT_PATH")
	config.ClientKeyPath = os.Getenv("PROMETHEUS_CLIENT_KEY_PATH")
	config.ServerName = os.Getenv("PROMETHEUS_SERVER_NAME")

	// Support both direct bearer token and token path
	config.BearerToken = os.Getenv("PROMETHEUS_BEARER_TOKEN")
	config.TokenPath = os.Getenv("PROMETHEUS_TOKEN_PATH")

	return config
}
