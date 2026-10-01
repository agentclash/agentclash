package temporalutil

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadConnectionConfigFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		env         map[string]string
		wantTLS     bool
		wantAPIKey  bool
		wantError   string
	}{
		{name: "development plaintext", environment: "development"},
		{name: "explicit development plaintext", environment: "local", env: map[string]string{"TEMPORAL_TLS_ENABLED": "false"}},
		{name: "existing Cloud configuration", environment: "production", env: map[string]string{"TEMPORAL_API_KEY": "temporal-test-key"}, wantTLS: true, wantAPIKey: true},
		{name: "explicit Cloud TLS", environment: "production", env: map[string]string{"TEMPORAL_API_KEY": "temporal-test-key", "TEMPORAL_TLS_ENABLED": "true"}, wantTLS: true, wantAPIKey: true},
		{name: "TLS independent of API key", environment: "test", env: map[string]string{"TEMPORAL_TLS_ENABLED": "true"}, wantTLS: true},
		{name: "production plaintext rejected", environment: "production", wantError: "TEMPORAL_TLS_ENABLED"},
		{name: "staging plaintext rejected", environment: "staging", wantError: "TEMPORAL_TLS_ENABLED"},
		{name: "unknown environment rejected", environment: "developmnt", wantError: "TEMPORAL_TLS_ENABLED"},
		{name: "empty environment rejected", wantError: "TEMPORAL_TLS_ENABLED"},
		{name: "unauthenticated production TLS rejected", environment: "production", env: map[string]string{"TEMPORAL_TLS_ENABLED": "true"}, wantError: "certificate/key pair"},
		{name: "API key cannot disable TLS", environment: "test", env: map[string]string{"TEMPORAL_API_KEY": "temporal-test-key", "TEMPORAL_TLS_ENABLED": "false"}, wantError: "TEMPORAL_TLS_ENABLED"},
		{name: "CA setting needs TLS", environment: "test", env: map[string]string{"TEMPORAL_TLS_CA_FILE": "unused-test-path"}, wantError: "TEMPORAL_TLS_ENABLED"},
		{name: "server name cannot disable TLS", environment: "test", env: map[string]string{"TEMPORAL_TLS_ENABLED": "false", "TEMPORAL_TLS_SERVER_NAME": "temporal.test"}, wantError: "TEMPORAL_TLS_ENABLED"},
		{name: "certificate requires key", environment: "production", env: map[string]string{"TEMPORAL_TLS_ENABLED": "true", "TEMPORAL_TLS_CERT_FILE": "unused-test-path"}, wantError: "must be set together"},
		{name: "key requires certificate", environment: "production", env: map[string]string{"TEMPORAL_TLS_ENABLED": "true", "TEMPORAL_TLS_KEY_FILE": "unused-test-path"}, wantError: "must be set together"},
		{name: "mixed authentication rejected", environment: "production", env: map[string]string{"TEMPORAL_API_KEY": "temporal-test-key", "TEMPORAL_TLS_CERT_FILE": "unused-cert", "TEMPORAL_TLS_KEY_FILE": "unused-key"}, wantError: "not both"},
		{name: "malformed boolean", environment: "test", env: map[string]string{"TEMPORAL_TLS_ENABLED": "not-a-boolean"}, wantError: "must be a boolean"},
		{name: "empty explicit boolean", environment: "test", env: map[string]string{"TEMPORAL_TLS_ENABLED": ""}, wantError: "must be a boolean"},
		{name: "API key whitespace", environment: "test", env: map[string]string{"TEMPORAL_API_KEY": "synthetic key"}, wantError: "TEMPORAL_API_KEY"},
		{name: "API key newline", environment: "test", env: map[string]string{"TEMPORAL_API_KEY": "synthetic\nkey"}, wantError: "TEMPORAL_API_KEY"},
		{name: "server name with scheme", environment: "test", env: map[string]string{"TEMPORAL_TLS_ENABLED": "true", "TEMPORAL_TLS_SERVER_NAME": "https://temporal.test"}, wantError: "TEMPORAL_TLS_SERVER_NAME"},
		{name: "server name with port", environment: "test", env: map[string]string{"TEMPORAL_TLS_ENABLED": "true", "TEMPORAL_TLS_SERVER_NAME": "temporal.test:7233"}, wantError: "TEMPORAL_TLS_SERVER_NAME"},
		{name: "IP server name", environment: "test", env: map[string]string{"TEMPORAL_TLS_ENABLED": "true", "TEMPORAL_TLS_SERVER_NAME": "::1"}, wantTLS: true},
	}
	for _, environment := range []string{"development", "dev", "local", "test", " DEV "} {
		tests = append(tests, struct {
			name, environment string
			env               map[string]string
			wantTLS           bool
			wantAPIKey        bool
			wantError         string
		}{name: "development alias " + environment, environment: environment})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConnectionEnv(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			config, err := LoadConnectionConfigFromEnv(test.environment)
			if test.wantError != "" {
				if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want configuration error mentioning %s", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !config.configured || (config.tlsConfig != nil) != test.wantTLS || (config.credentials != nil) != test.wantAPIKey {
				t.Fatal("unexpected transport/authentication mode")
			}
			if test.wantTLS {
				if config.tlsConfig.MinVersion != tls.VersionTLS12 || config.tlsConfig.InsecureSkipVerify {
					t.Fatal("TLS must verify certificates and require at least TLS 1.2")
				}
				if config.tlsConfig.RootCAs != nil {
					t.Fatal("unset custom CA must retain system trust")
				}
				if config.tlsConfig.ServerName != test.env["TEMPORAL_TLS_SERVER_NAME"] {
					t.Fatal("server name override was not preserved")
				}
			}
		})
	}
}

func TestLoadConnectionConfigFromEnvCertificateFiles(t *testing.T) {
	ca := newTestCertificate(t, nil, 0)
	client := newTestCertificate(t, ca, x509.ExtKeyUsageClientAuth)
	otherCA := newTestCertificate(t, nil, 0)
	otherClient := newTestCertificate(t, otherCA, x509.ExtKeyUsageClientAuth)
	tests := []struct {
		name      string
		configure func(*testing.T)
		wantError string
	}{
		{name: "valid production mTLS"},
		{name: "CA rotation bundle", configure: func(t *testing.T) {
			t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "ca-bundle.pem", append(append([]byte{}, ca.certPEM...), otherCA.certPEM...)))
		}},
		{name: "missing CA", configure: func(t *testing.T) { t.Setenv("TEMPORAL_TLS_CA_FILE", t.TempDir()+"/missing.pem") }, wantError: "cannot read TEMPORAL_TLS_CA_FILE"},
		{name: "unreadable CA directory", configure: func(t *testing.T) { t.Setenv("TEMPORAL_TLS_CA_FILE", t.TempDir()) }, wantError: "cannot read TEMPORAL_TLS_CA_FILE"},
		{name: "empty CA", configure: func(t *testing.T) { t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "empty.pem", nil)) }, wantError: "contains no CA certificates"},
		{name: "malformed CA", configure: func(t *testing.T) {
			t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "bad.pem", []byte("synthetic-private-material")))
		}, wantError: "only PEM CA certificates"},
		{name: "partially malformed CA", configure: func(t *testing.T) {
			t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "partial.pem", append(append([]byte{}, ca.certPEM...), []byte("synthetic-private-material")...)))
		}, wantError: "only PEM CA certificates"},
		{name: "malformed PEM before valid CA", configure: func(t *testing.T) {
			bad := []byte("-----BEGIN CERTIFICATE-----\ninvalid-base64\n-----END CERTIFICATE-----\n")
			t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "prefixed.pem", append(bad, ca.certPEM...)))
		}, wantError: "invalid PEM certificate data"},
		{name: "unterminated PEM before valid CA", configure: func(t *testing.T) {
			bad := []byte("-----BEGIN CERTIFICATE-----\ninvalid-base64\n")
			t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "unterminated.pem", append(bad, ca.certPEM...)))
		}, wantError: "invalid PEM certificate data"},
		{name: "leaf is not a CA", configure: func(t *testing.T) { t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "leaf.pem", client.certPEM)) }, wantError: "invalid CA certificate"},
		{name: "missing client certificate", configure: func(t *testing.T) { t.Setenv("TEMPORAL_TLS_CERT_FILE", t.TempDir()+"/missing.crt") }, wantError: "cannot read TEMPORAL_TLS_CERT_FILE"},
		{name: "missing client key", configure: func(t *testing.T) { t.Setenv("TEMPORAL_TLS_KEY_FILE", t.TempDir()+"/missing.key") }, wantError: "cannot read TEMPORAL_TLS_KEY_FILE"},
		{name: "malformed client key", configure: func(t *testing.T) {
			t.Setenv("TEMPORAL_TLS_KEY_FILE", testPEMFile(t, "bad.key", []byte("synthetic-private-material")))
		}, wantError: "matching PEM certificate/key pair"},
		{name: "mismatched key", configure: func(t *testing.T) { t.Setenv("TEMPORAL_TLS_KEY_FILE", testPEMFile(t, "other.key", otherClient.keyPEM)) }, wantError: "matching PEM certificate/key pair"},
		{name: "expired client", configure: func(t *testing.T) {
			setClientCertificateEnv(t, newTestCertificate(t, ca, x509.ExtKeyUsageClientAuth, func(c *x509.Certificate) { c.NotAfter = time.Now().Add(-time.Minute) }))
		}, wantError: "expired or not yet valid"},
		{name: "future client", configure: func(t *testing.T) {
			setClientCertificateEnv(t, newTestCertificate(t, ca, x509.ExtKeyUsageClientAuth, func(c *x509.Certificate) { c.NotBefore = time.Now().Add(time.Minute) }))
		}, wantError: "expired or not yet valid"},
		{name: "server-only certificate", configure: func(t *testing.T) { setClientCertificateEnv(t, newTestCertificate(t, ca, x509.ExtKeyUsageServerAuth)) }, wantError: "does not permit client authentication"},
		{name: "CA is not an application identity", configure: func(t *testing.T) { setClientCertificateEnv(t, ca) }, wantError: "end-entity client certificate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConnectionEnv(t)
			t.Setenv("TEMPORAL_TLS_ENABLED", "true")
			t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "ca.pem", ca.certPEM))
			setClientCertificateEnv(t, client)
			if test.configure != nil {
				test.configure(t)
			}
			config, err := LoadConnectionConfigFromEnv("production")
			if test.wantError != "" {
				if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want configuration error mentioning %s", err, test.wantError)
				}
				for _, sensitive := range []string{os.Getenv("TEMPORAL_TLS_CA_FILE"), os.Getenv("TEMPORAL_TLS_CERT_FILE"), os.Getenv("TEMPORAL_TLS_KEY_FILE"), "synthetic-private-material"} {
					if sensitive != "" && strings.Contains(err.Error(), sensitive) {
						t.Fatal("configuration error exposed a file path or certificate/key data")
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if config.tlsConfig == nil || config.tlsConfig.RootCAs == nil || len(config.tlsConfig.Certificates) != 1 || config.credentials != nil {
				t.Fatal("expected custom trust, one client certificate, and no API key")
			}
			if test.name == "CA rotation bundle" && len(config.tlsConfig.RootCAs.Subjects()) != 2 {
				t.Fatal("rotation bundle must retain both CA certificates")
			}
		})
	}
}
