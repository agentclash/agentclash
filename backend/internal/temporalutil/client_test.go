package temporalutil

import (
	"crypto/tls"
	"errors"
	"testing"

	temporalsdk "go.temporal.io/sdk/client"
)

func TestClientOptions(t *testing.T) {
	clearConnectionEnv(t)
	t.Setenv("TEMPORAL_API_KEY", "temporal-test-key")
	connection, err := LoadConnectionConfigFromEnv("production")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEMPORAL_API_KEY", "")
	t.Setenv("TEMPORAL_TLS_ENABLED", "false")
	opts, err := clientOptions("temporal.test:7233", "test-namespace", connection, WithMetricsHandler(temporalsdk.MetricsNopHandler), nil, WithMetricsHandler(nil))
	if err != nil {
		t.Fatal(err)
	}
	if opts.HostPort != "temporal.test:7233" || opts.Namespace != "test-namespace" || opts.MetricsHandler != temporalsdk.MetricsNopHandler {
		t.Fatal("endpoint, namespace, or metrics handler was lost")
	}
	if opts.ConnectionOptions.TLS == nil || opts.ConnectionOptions.TLSDisabled || opts.Credentials == nil {
		t.Fatal("client creation must use the validated snapshot, not reread the environment")
	}
	opts.ConnectionOptions.TLS.ServerName = "changed.test"
	second, err := clientOptions("[::1]:7233", "test-namespace", connection)
	if err != nil {
		t.Fatal(err)
	}
	if second.ConnectionOptions.TLS.ServerName != "" || connection.tlsConfig.ServerName != "" {
		t.Fatal("TLS configuration must not be mutated across client instances")
	}
	if second.ConnectionOptions.TLS.MinVersion != tls.VersionTLS12 || second.ConnectionOptions.TLS.InsecureSkipVerify {
		t.Fatal("client options weakened certificate verification")
	}
	for _, address := range []string{"", "localhost", ":7233", "localhost:0", "localhost:65536", "localhost:invalid", "https://localhost:7233", "localhost :7233"} {
		t.Run("address "+address, func(t *testing.T) {
			if _, err := NewClient(address, "test-namespace", connection); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("invalid address must fail before dial: %v", err)
			}
		})
	}
	for _, namespace := range []string{"", " ", "test namespace", "test\nnamespace"} {
		t.Run("namespace "+namespace, func(t *testing.T) {
			if _, err := NewClient("localhost:7233", namespace, connection); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("invalid namespace must fail before dial: %v", err)
			}
		})
	}
	if _, err := NewClient("localhost:7233", "test-namespace", ConnectionConfig{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("uninitialized connection must fail before dial: %v", err)
	}
}
