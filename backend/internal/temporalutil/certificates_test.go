package temporalutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test material is generated for each test and never stored in the repository.
type testCertificate struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
	certPEM     []byte
	keyPEM      []byte
}

func newTestCertificate(t *testing.T, issuer *testCertificate, usage x509.ExtKeyUsage, modify ...func(*x509.Certificate)) *testCertificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "temporal.test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature,
	}
	if issuer == nil {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{usage}
		if usage == x509.ExtKeyUsageServerAuth {
			template.DNSNames = []string{"temporal.test"}
			template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		}
	}
	for _, change := range modify {
		change(template)
	}
	parent, signer := template, key
	if issuer != nil {
		parent, signer = issuer.certificate, issuer.key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return &testCertificate{
		certificate: certificate,
		key:         key,
		certPEM:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM:      pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	}
}

func (c *testCertificate) tlsCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	pair, err := tls.X509KeyPair(c.certPEM, c.keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func testPEMFile(t *testing.T, name string, contents []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func setClientCertificateEnv(t *testing.T, certificate *testCertificate) {
	t.Helper()
	t.Setenv("TEMPORAL_TLS_CERT_FILE", testPEMFile(t, "client.crt", certificate.certPEM))
	t.Setenv("TEMPORAL_TLS_KEY_FILE", testPEMFile(t, "client.key", certificate.keyPEM))
}

func clearConnectionEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"TEMPORAL_API_KEY", "TEMPORAL_TLS_ENABLED", "TEMPORAL_TLS_CA_FILE", "TEMPORAL_TLS_SERVER_NAME", "TEMPORAL_TLS_CERT_FILE", "TEMPORAL_TLS_KEY_FILE"} {
		t.Setenv(key, "") // Register cleanup restoring the original environment.
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}
