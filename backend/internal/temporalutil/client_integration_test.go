package temporalutil

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"testing"
	"time"

	workflowservice "go.temporal.io/api/workflowservice/v1"
	temporalsdk "go.temporal.io/sdk/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type observedConnection struct {
	authorization     string
	clientCertificate []byte
	serverName        string
	tlsVersion        uint16
}

type testTemporalService struct {
	workflowservice.UnimplementedWorkflowServiceServer
	connections chan observedConnection
}

func (s *testTemporalService) GetSystemInfo(ctx context.Context, _ *workflowservice.GetSystemInfoRequest) (*workflowservice.GetSystemInfoResponse, error) {
	observation := observedConnection{}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("authorization"); len(values) != 0 {
			observation.authorization = values[0]
		}
	}
	if remote, ok := peer.FromContext(ctx); ok {
		if info, ok := remote.AuthInfo.(credentials.TLSInfo); ok {
			observation.tlsVersion = info.State.Version
			observation.serverName = info.State.ServerName
			if len(info.State.PeerCertificates) != 0 {
				observation.clientCertificate = info.State.PeerCertificates[0].Raw
			}
		}
	}
	select {
	case s.connections <- observation:
	default:
	}
	return &workflowservice.GetSystemInfoResponse{ServerVersion: "test", Capabilities: &workflowservice.GetSystemInfoResponse_Capabilities{}}, nil
}

func startTestTemporalServer(t *testing.T, transport *tls.Config) (string, <-chan observedConnection) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var options []grpc.ServerOption
	if transport != nil {
		options = append(options, grpc.Creds(credentials.NewTLS(transport)))
	}
	server := grpc.NewServer(options...)
	service := &testTemporalService{connections: make(chan observedConnection, 1)}
	workflowservice.RegisterWorkflowServiceServer(server, service)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
		<-stopped
	})
	return listener.Addr().String(), service.connections
}

func TestNewClientTransportIntegration(t *testing.T) {
	ca := newTestCertificate(t, nil, 0)
	serverCertificate := newTestCertificate(t, ca, x509.ExtKeyUsageServerAuth)
	clientCertificate := newTestCertificate(t, ca, x509.ExtKeyUsageClientAuth)
	otherCA := newTestCertificate(t, nil, 0)
	otherClient := newTestCertificate(t, otherCA, x509.ExtKeyUsageClientAuth)
	tests := []struct {
		name            string
		plaintext       bool
		serverPlaintext bool
		apiKey          bool
		addressName     bool
		wrongName       bool
		wrongCA         bool
		wrongClient     bool
		missingClient   bool
		removeFiles     bool
		tls12Only       bool
		wantError       bool
	}{
		{name: "development plaintext", plaintext: true},
		{name: "Cloud bearer authentication", apiKey: true},
		{name: "self-hosted mTLS"},
		{name: "TLS 1.2 mTLS", tls12Only: true},
		{name: "address-derived certificate identity", addressName: true},
		{name: "uses startup certificate snapshot", removeFiles: true},
		{name: "wrong server name", wrongName: true, wantError: true},
		{name: "untrusted server CA", wrongCA: true, wantError: true},
		{name: "untrusted client certificate", wrongClient: true, wantError: true},
		{name: "server requires client certificate", missingClient: true, wantError: true},
		{name: "TLS client never falls back to plaintext", serverPlaintext: true, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConnectionEnv(t)
			environment := "production"
			var serverTLS *tls.Config
			if test.plaintext {
				environment = "development"
			} else {
				serverTLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCertificate.tlsCertificate(t)}}
				if test.tls12Only {
					serverTLS.MaxVersion = tls.VersionTLS12
				}
				trust := ca
				if test.wrongCA {
					trust = otherCA
				}
				t.Setenv("TEMPORAL_TLS_CA_FILE", testPEMFile(t, "ca.pem", trust.certPEM))
				if !test.addressName {
					name := "temporal.test"
					if test.wrongName {
						name = "wrong.test"
					}
					t.Setenv("TEMPORAL_TLS_SERVER_NAME", name)
				}
				if test.apiKey {
					// Omitting TLS_ENABLED exercises the existing Cloud auto-TLS path.
					t.Setenv("TEMPORAL_API_KEY", "temporal-test-key")
				} else {
					t.Setenv("TEMPORAL_TLS_ENABLED", "true")
					serverTLS.ClientAuth = tls.RequireAndVerifyClientCert
					serverTLS.ClientCAs = x509.NewCertPool()
					serverTLS.ClientCAs.AddCert(ca.certificate)
					certificate := clientCertificate
					if test.wrongClient {
						certificate = otherClient
					}
					if test.missingClient {
						// Production rejects this before dialing. Development permits
						// TLS-only, allowing this test to verify the server's rejection.
						environment = "test"
					} else {
						setClientCertificateEnv(t, certificate)
					}
				}
			}
			connection, err := LoadConnectionConfigFromEnv(environment)
			if err != nil {
				t.Fatal(err)
			}
			if test.removeFiles {
				for _, key := range []string{"TEMPORAL_TLS_CA_FILE", "TEMPORAL_TLS_CERT_FILE", "TEMPORAL_TLS_KEY_FILE"} {
					if err := os.Remove(os.Getenv(key)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if test.serverPlaintext {
				serverTLS = nil
			}
			address, observations := startTestTemporalServer(t, serverTLS)
			timeout := 3 * time.Second
			if test.wantError {
				timeout = 500 * time.Millisecond
			}
			client, err := NewClient(address, "test-namespace", connection, func(options *temporalsdk.Options) {
				options.ConnectionOptions.GetSystemInfoTimeout = timeout
			})
			if client != nil {
				defer client.Close()
			}
			if test.wantError {
				if err == nil {
					t.Fatal("invalid TLS authentication unexpectedly connected")
				}
				select {
				case <-observations:
					t.Fatal("a rejected TLS connection reached the Temporal service")
				default:
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case observed := <-observations:
				if test.plaintext {
					if observed.tlsVersion != 0 || observed.authorization != "" {
						t.Fatal("unexpected security settings on local development connection")
					}
					return
				}
				if observed.tlsVersion < tls.VersionTLS12 {
					t.Fatal("TLS connection used a version below 1.2")
				}
				if !test.addressName && observed.serverName != "temporal.test" {
					t.Fatal("configured server name was not used for TLS")
				}
				if test.apiKey {
					if observed.authorization != "Bearer temporal-test-key" || len(observed.clientCertificate) != 0 {
						t.Fatal("expected API-key bearer authentication without a client certificate")
					}
				} else if observed.authorization != "" || !bytes.Equal(observed.clientCertificate, clientCertificate.certificate.Raw) {
					t.Fatal("expected the configured mTLS client certificate without an API key")
				}
			case <-time.After(time.Second):
				t.Fatal("Temporal SDK dial did not reach the test service")
			}
		})
	}
}
