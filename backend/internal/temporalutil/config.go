package temporalutil

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	temporalsdk "go.temporal.io/sdk/client"
)

var ErrInvalidConfig = errors.New("invalid Temporal connection configuration")

// ConnectionConfig holds validated transport settings loaded at startup. Keep
// credentials and parsed key material private to this package and out of logs.
// Its zero value cannot be used to create a client.
type ConnectionConfig struct {
	configured  bool
	tlsConfig   *tls.Config
	credentials temporalsdk.Credentials
}

// LoadConnectionConfigFromEnv preserves API-key TLS for Temporal Cloud and also
// supports explicit TLS/mTLS for self-hosted servers. Certificate files are read
// once; restart clients after rotating their mounted certificates or keys.
func LoadConnectionConfigFromEnv(appEnvironment string) (ConnectionConfig, error) {
	apiKey := os.Getenv("TEMPORAL_API_KEY")
	if strings.IndexFunc(apiKey, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return ConnectionConfig{}, fmt.Errorf("%w: TEMPORAL_API_KEY must not contain whitespace or control characters", ErrInvalidConfig)
	}
	caFile := os.Getenv("TEMPORAL_TLS_CA_FILE")
	serverName := os.Getenv("TEMPORAL_TLS_SERVER_NAME")
	certFile := os.Getenv("TEMPORAL_TLS_CERT_FILE")
	keyFile := os.Getenv("TEMPORAL_TLS_KEY_FILE")

	tlsEnabled := apiKey != ""
	if value, ok := os.LookupEnv("TEMPORAL_TLS_ENABLED"); ok {
		var err error
		tlsEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return ConnectionConfig{}, fmt.Errorf("%w: TEMPORAL_TLS_ENABLED must be a boolean", ErrInvalidConfig)
		}
	}
	if !tlsEnabled && (apiKey != "" || caFile != "" || serverName != "" || certFile != "" || keyFile != "") {
		return ConnectionConfig{}, fmt.Errorf("%w: TEMPORAL_TLS_ENABLED must be true when credentials or TLS settings are provided", ErrInvalidConfig)
	}
	if (certFile == "") != (keyFile == "") {
		return ConnectionConfig{}, fmt.Errorf("%w: TEMPORAL_TLS_CERT_FILE and TEMPORAL_TLS_KEY_FILE must be set together", ErrInvalidConfig)
	}
	if apiKey != "" && certFile != "" {
		return ConnectionConfig{}, fmt.Errorf("%w: use TEMPORAL_API_KEY or the TLS client certificate/key pair, not both", ErrInvalidConfig)
	}
	if !developmentEnvironment(appEnvironment) {
		if !tlsEnabled {
			return ConnectionConfig{}, fmt.Errorf("%w: TEMPORAL_TLS_ENABLED must be true outside development (an API key enables it by default)", ErrInvalidConfig)
		}
		if apiKey == "" && certFile == "" {
			return ConnectionConfig{}, fmt.Errorf("%w: TEMPORAL_API_KEY or a TLS client certificate/key pair is required outside development", ErrInvalidConfig)
		}
	}

	config := ConnectionConfig{configured: true}
	if !tlsEnabled {
		return config, nil
	}
	if serverName != "" && !validServerName(serverName) {
		return ConnectionConfig{}, fmt.Errorf("%w: TEMPORAL_TLS_SERVER_NAME must be a DNS name or IP address without a port", ErrInvalidConfig)
	}
	config.tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: serverName,
	}
	if caFile != "" {
		roots, err := loadRootCAs(caFile)
		if err != nil {
			return ConnectionConfig{}, err
		}
		config.tlsConfig.RootCAs = roots
	}
	if certFile != "" {
		certificate, err := loadClientCertificate(certFile, keyFile)
		if err != nil {
			return ConnectionConfig{}, err
		}
		config.tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	if apiKey != "" {
		config.credentials = temporalsdk.NewAPIKeyStaticCredentials(apiKey)
	}
	return config, nil
}

func loadRootCAs(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read TEMPORAL_TLS_CA_FILE", ErrInvalidConfig)
	}
	pool := x509.NewCertPool()
	count := 0
	for data = bytes.TrimSpace(data); len(data) != 0; data = bytes.TrimSpace(data) {
		// Decode can skip text before a PEM block. Reject such data rather than
		// accepting a partially malformed or incorrectly mounted CA bundle.
		if !bytes.HasPrefix(data, []byte("-----BEGIN CERTIFICATE-----")) {
			return nil, fmt.Errorf("%w: TEMPORAL_TLS_CA_FILE must contain only PEM CA certificates", ErrInvalidConfig)
		}
		endMarker := []byte("-----END CERTIFICATE-----")
		end := bytes.Index(data, endMarker)
		if end < 0 {
			return nil, fmt.Errorf("%w: TEMPORAL_TLS_CA_FILE contains invalid PEM certificate data", ErrInvalidConfig)
		}
		end += len(endMarker)
		encoded := data[:end]
		block, remainder := pem.Decode(encoded)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 || len(bytes.TrimSpace(remainder)) != 0 || bytes.Count(encoded, []byte("-----BEGIN")) != 1 {
			return nil, fmt.Errorf("%w: TEMPORAL_TLS_CA_FILE contains invalid PEM certificate data", ErrInvalidConfig)
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !certificate.IsCA {
			return nil, fmt.Errorf("%w: TEMPORAL_TLS_CA_FILE contains an invalid CA certificate", ErrInvalidConfig)
		}
		pool.AddCert(certificate)
		count++
		data = data[end:]
	}
	if count == 0 {
		return nil, fmt.Errorf("%w: TEMPORAL_TLS_CA_FILE contains no CA certificates", ErrInvalidConfig)
	}
	return pool, nil
}

func loadClientCertificate(certFile, keyFile string) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("%w: cannot read TEMPORAL_TLS_CERT_FILE", ErrInvalidConfig)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("%w: cannot read TEMPORAL_TLS_KEY_FILE", ErrInvalidConfig)
	}
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("%w: TEMPORAL_TLS_CERT_FILE and TEMPORAL_TLS_KEY_FILE must contain a matching PEM certificate/key pair", ErrInvalidConfig)
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("%w: TEMPORAL_TLS_CERT_FILE contains an invalid client certificate", ErrInvalidConfig)
	}
	if leaf.IsCA {
		return tls.Certificate{}, fmt.Errorf("%w: TEMPORAL_TLS_CERT_FILE must be an end-entity client certificate, not a CA", ErrInvalidConfig)
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return tls.Certificate{}, fmt.Errorf("%w: TEMPORAL_TLS_CERT_FILE client certificate is expired or not yet valid", ErrInvalidConfig)
	}
	if !permitsClientAuthentication(leaf) {
		return tls.Certificate{}, fmt.Errorf("%w: TEMPORAL_TLS_CERT_FILE certificate does not permit client authentication", ErrInvalidConfig)
	}
	certificate.Leaf = leaf
	return certificate, nil
}

func permitsClientAuthentication(certificate *x509.Certificate) bool {
	if len(certificate.ExtKeyUsage) == 0 && len(certificate.UnknownExtKeyUsage) == 0 {
		return true // No extended-key-usage restriction.
	}
	for _, usage := range certificate.ExtKeyUsage {
		if usage == x509.ExtKeyUsageAny || usage == x509.ExtKeyUsageClientAuth {
			return true
		}
	}
	return false
}

func developmentEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

func validServerName(value string) bool {
	if net.ParseIP(value) != nil {
		return true
	}
	value = strings.TrimSuffix(value, ".")
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-') {
				return false
			}
		}
	}
	return true
}
