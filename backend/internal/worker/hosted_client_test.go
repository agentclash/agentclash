package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/hostedruns"
	"github.com/agentclash/agentclash/backend/internal/repository"
	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/google/uuid"
)

func TestHostedRunClientStartPostsStandardContract(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	challengePackVersionID := uuid.New()

	var captured hostedruns.StartRequest
	client := NewHostedRunClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
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
	})}, "http://agentclash.local", "test-secret")
	result, err := client.Start(context.Background(), workflowpkg.HostedRunStartInput{
		ExecutionContext: hostedExecutionContextFixture(runID, runAgentID, challengePackVersionID, "https://remote.example"),
		TraceLevel:       hostedruns.TraceLevelBlackBox,
		TaskPayload:      []byte(`{"task":"payload"}`),
		DeadlineAt:       time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC),
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
	client := NewHostedRunClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial failed")
	})}, "http://agentclash.local", "test-secret")

	_, err := client.Start(context.Background(), workflowpkg.HostedRunStartInput{
		ExecutionContext: hostedExecutionContextFixture(uuid.New(), uuid.New(), uuid.New(), "https://remote.example"),
		TraceLevel:       hostedruns.TraceLevelBlackBox,
		TaskPayload:      []byte(`{}`),
		DeadlineAt:       time.Now().UTC().Add(time.Minute),
	})
	if err == nil || !strings.Contains(err.Error(), "dial failed") {
		t.Fatalf("error = %v, want transport error", err)
	}
}

func TestHostedRunClientStartRejectsMalformedResponse(t *testing.T) {
	client := NewHostedRunClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"accepted":true}`), nil
	})}, "http://agentclash.local", "test-secret")
	_, err := client.Start(context.Background(), workflowpkg.HostedRunStartInput{
		ExecutionContext: hostedExecutionContextFixture(uuid.New(), uuid.New(), uuid.New(), "https://remote.example"),
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
