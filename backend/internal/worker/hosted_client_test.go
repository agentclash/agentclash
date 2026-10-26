package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/runtime/connectors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/agentclash/agentclash/runtime/hostedruns"
	"github.com/google/uuid"
)

func TestHostedRunClientStartPostsStandardContract(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	challengePackVersionID := uuid.New()

	var captured hostedruns.StartRequest
	client := localHostedClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/agentclash/runs" {
			t.Fatalf("path = %s, want /agentclash/runs", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return jsonResponse(`{"accepted":true,"external_run_id":"ext-123"}`), nil
	}))
	result, err := client.Start(context.Background(), workflowpkg.HostedRunStartInput{
		ExecutionContext: hostedExecutionContextFixture(runID, runAgentID, challengePackVersionID, "https://example.com"),
		TraceLevel:       hostedruns.TraceLevelBlackBox,
		TaskPayload:      []byte(`{"task":"payload"}`),
		DeadlineAt:       time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !result.Accepted || result.ExternalRunID != "ext-123" {
		t.Fatalf("result = %+v, want accepted ext-123", result)
	}
	if captured.RunID != runID || captured.RunAgentID != runAgentID {
		t.Fatalf("captured run ids = %s/%s, want %s/%s", captured.RunID, captured.RunAgentID, runID, runAgentID)
	}
	if captured.ChallengePackVersionID != challengePackVersionID {
		t.Fatalf("challenge_pack_version_id = %s, want %s", captured.ChallengePackVersionID, challengePackVersionID)
	}
	if captured.TraceLevel != hostedruns.TraceLevelBlackBox {
		t.Fatalf("trace_level = %q, want black_box", captured.TraceLevel)
	}
	if captured.CallbackURL != fmt.Sprintf("http://agentclash.local/v1/integrations/hosted-runs/%s/events", runID) {
		t.Fatalf("callback_url = %q", captured.CallbackURL)
	}
	if captured.CallbackToken == "" {
		t.Fatalf("callback_token should be populated")
	}
}

func TestHostedRunClientStartReturnsTransportError(t *testing.T) {
	client := localHostedClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial failed")
	}))

	_, err := client.Start(context.Background(), workflowpkg.HostedRunStartInput{
		ExecutionContext: hostedExecutionContextFixture(uuid.New(), uuid.New(), uuid.New(), "https://example.com"),
		TraceLevel:       hostedruns.TraceLevelBlackBox,
		TaskPayload:      []byte(`{}`),
		DeadlineAt:       time.Now().UTC().Add(time.Minute),
	})
	if err == nil || !strings.Contains(err.Error(), "outcome unknown") {
		t.Fatalf("error = %v, want transport error", err)
	}
}

func TestHostedRunClientStartRejectsMalformedResponse(t *testing.T) {
	client := localHostedClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"accepted":true}`), nil
	}))
	_, err := client.Start(context.Background(), workflowpkg.HostedRunStartInput{
		ExecutionContext: hostedExecutionContextFixture(uuid.New(), uuid.New(), uuid.New(), "https://example.com"),
		TraceLevel:       hostedruns.TraceLevelBlackBox,
		TaskPayload:      []byte(`{}`),
		DeadlineAt:       time.Now().UTC().Add(time.Minute),
	})
	if err == nil || err.Error() != "hosted deployment response is missing external_run_id" {
		t.Fatalf("error = %v, want missing external_run_id", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func hostedExecutionContextFixture(runID uuid.UUID, runAgentID uuid.UUID, challengePackVersionID uuid.UUID, endpointURL string) repository.RunAgentExecutionContext {
	return repository.RunAgentExecutionContext{
		Run: domain.Run{
			ID:                     runID,
			ChallengePackVersionID: challengePackVersionID,
		},
		RunAgent: domain.RunAgent{
			ID: runAgentID,
		},
		Deployment: repository.AgentDeploymentExecutionContext{
			DeploymentType: "hosted_external",
			EndpointURL:    &endpointURL,
		},
	}
}

func TestHostedConnectorRejectsPrivateTargetBeforeSending(t *testing.T) {
	calls := 0
	client := NewHostedRunClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(`{"accepted":true,"external_run_id":"leaked"}`), nil
	})}, "https://callback.example.com", "test-secret").WithConnectorRepository(hostedCredentials{endpoint: "http://127.0.0.1:8080"})
	_, err := client.Start(context.Background(), workflowpkg.HostedRunStartInput{ExecutionContext: hostedExecutionContextFixture(uuid.New(), uuid.New(), uuid.New(), "http://127.0.0.1:8080"), TraceLevel: hostedruns.TraceLevelBlackBox, TaskPayload: []byte(`{}`), DeadlineAt: time.Now().Add(time.Minute)})
	if err == nil || calls != 0 {
		t.Fatalf("private target reached transport: calls=%d err=%v", calls, err)
	}
}

type hostedResolver struct{}

func (hostedResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
}

type hostedDialer func(context.Context, string, string) (net.Conn, error)

func (f hostedDialer) DialContext(ctx context.Context, n, a string) (net.Conn, error) {
	return f(ctx, n, a)
}

type hostedCredentials struct{ endpoint string }

func (f hostedCredentials) ResolveApprovedConnector(_ context.Context, snapshot, workspace uuid.UUID) (connectors.Binding, string, error) {
	return connectors.Binding{SnapshotID: snapshot, WorkspaceID: workspace, Config: connectors.Config{Adapter: "hosted", EndpointURL: f.endpoint, Auth: "bearer", SecretKey: "APP_TOKEN", TimeoutSeconds: 30}}, "fixture-app-secret", nil
}
func localHostedClient(t *testing.T, handle roundTripFunc) HostedRunClient {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-app-secret" {
			t.Error("missing approved workspace credential")
		}
		response, err := handle(r)
		if err != nil {
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		defer response.Body.Close()
		for k, v := range response.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
	}))
	t.Cleanup(server.Close)
	client := NewHostedRunClient(server.Client(), "http://agentclash.local", "test-secret").WithConnectorRepository(hostedCredentials{endpoint: "https://example.com"})
	client.transport.Resolver = hostedResolver{}
	client.transport.Dialer = hostedDialer(func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "93.184.216.34:443" {
			t.Fatalf("unpinned dial %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	return client
}

type revocableCredentials struct{ revoked bool }

func (f *revocableCredentials) ResolveApprovedConnector(ctx context.Context, snapshot, workspace uuid.UUID) (connectors.Binding, string, error) {
	if f.revoked {
		return connectors.Binding{}, "", repository.ErrConnectorUnapproved
	}
	return (hostedCredentials{endpoint: "https://example.com"}).ResolveApprovedConnector(ctx, snapshot, workspace)
}

type revokingResolver struct{ credentials *revocableCredentials }

func (f revokingResolver) LookupNetIP(ctx context.Context, n, h string) ([]netip.Addr, error) {
	f.credentials.revoked = true
	return (hostedResolver{}).LookupNetIP(ctx, n, h)
}
func TestHostedConnectorRevalidatesApprovalAfterDNSBeforeCredentialSend(t *testing.T) {
	calls := 0
	c := localHostedClient(t, func(*http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(`{"accepted":true,"external_run_id":"should-not-send"}`), nil
	})
	credentials := &revocableCredentials{}
	c.credentials = credentials
	c.transport.Resolver = revokingResolver{credentials: credentials}
	_, err := c.Start(context.Background(), workflowpkg.HostedRunStartInput{ExecutionContext: hostedExecutionContextFixture(uuid.New(), uuid.New(), uuid.New(), "https://example.com"), TraceLevel: hostedruns.TraceLevelBlackBox, TaskPayload: []byte(`{}`), DeadlineAt: time.Now().Add(time.Minute)})
	if err == nil || calls != 0 {
		t.Fatalf("revoked approval sent credential: calls=%d err=%v", calls, err)
	}
}
