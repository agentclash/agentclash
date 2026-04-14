package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

func TestHostedRunIngestionManagerPersistsAndSignalsTerminalEvent(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	externalRunID := "ext-123"
	status := hostedruns.FinalStatusCompleted
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: externalRunID,
		EventType:     hostedruns.EventTypeRunFinished,
		OccurredAt:    time.Now().UTC(),
		FinalStatus:   &status,
		Output:        []byte(`{"answer":"done"}`),
	}
	token, err := hostedruns.NewCallbackTokenSigner("secret").Sign(runID, runAgentID)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	repo := &fakeHostedRunExecutionRepository{
		execution: repository.HostedRunExecution{
			RunID:         runID,
			RunAgentID:    runAgentID,
			ExternalRunID: &externalRunID,
			Status:        "accepted",
		},
	}
	signaler := &fakeHostedRunWorkflowSignaler{}
	manager := NewHostedRunIngestionManager(repo, "secret", signaler, nil, slog.Default())

	if err := manager.IngestEvent(context.Background(), runID, token, event); err != nil {
		t.Fatalf("IngestEvent returned error: %v", err)
	}
	if repo.applyParams == nil || repo.applyParams.Status != "completed" {
		t.Fatalf("apply params = %#v, want completed status", repo.applyParams)
	}
	if repo.recordParams == nil {
		t.Fatalf("expected record params to be captured")
	}
	if repo.recordParams.Event.EventType != runevents.EventTypeSystemRunCompleted {
		t.Fatalf("recorded event type = %q, want %q", repo.recordParams.Event.EventType, runevents.EventTypeSystemRunCompleted)
	}
	if repo.recordParams.Event.Source != runevents.SourceHostedExternal {
		t.Fatalf("recorded event source = %q, want %q", repo.recordParams.Event.Source, runevents.SourceHostedExternal)
	}
	if signaler.signalCount != 1 {
		t.Fatalf("signal count = %d, want 1", signaler.signalCount)
	}
}

func TestHostedReplaySummaryDoesNotInlinePayload(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	finalStatus := hostedruns.FinalStatusCompleted
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: "ext-123",
		EventType:     hostedruns.EventTypeRunFinished,
		OccurredAt:    time.Now().UTC(),
		FinalStatus:   &finalStatus,
		Output:        []byte(`{"answer":"done"}`),
	}

	normalizedEvent, err := runevents.NormalizeHostedEvent(runID, event)
	if err != nil {
		t.Fatalf("NormalizeHostedEvent returned error: %v", err)
	}
	summaryJSON, err := hostedReplaySummary(normalizedEvent, event)
	if err != nil {
		t.Fatalf("hostedReplaySummary returned error: %v", err)
	}

	var summary map[string]any
	if err := json.Unmarshal(summaryJSON, &summary); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if _, ok := summary["payload"]; ok {
		t.Fatalf("summary unexpectedly included payload: %#v", summary)
	}
	if summary["source"] != string(runevents.SourceHostedExternal) {
		t.Fatalf("summary source = %#v, want %q", summary["source"], runevents.SourceHostedExternal)
	}
	if summary["last_event_type"] != string(runevents.EventTypeSystemRunCompleted) {
		t.Fatalf("summary last_event_type = %#v, want %q", summary["last_event_type"], runevents.EventTypeSystemRunCompleted)
	}
	if summary["status"] != "completed" {
		t.Fatalf("summary status = %#v, want completed", summary["status"])
	}
	if summary["raw_event_type"] != hostedruns.EventTypeRunFinished {
		t.Fatalf("summary raw_event_type = %#v, want %q", summary["raw_event_type"], hostedruns.EventTypeRunFinished)
	}
}

func TestHostedRunIngestionManagerNormalizesErrorEventIntoCanonicalVocabulary(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	externalRunID := "ext-err"
	event := hostedruns.Event{
		RunAgentID:    runAgentID,
		ExternalRunID: externalRunID,
		EventType:     hostedruns.EventTypeError,
		OccurredAt:    time.Now().UTC(),
		ErrorMessage:  stringPtr("boom"),
	}
	token, err := hostedruns.NewCallbackTokenSigner("secret").Sign(runID, runAgentID)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	repo := &fakeHostedRunExecutionRepository{
		execution: repository.HostedRunExecution{
			RunID:         runID,
			RunAgentID:    runAgentID,
			ExternalRunID: &externalRunID,
			Status:        "accepted",
		},
	}
	manager := NewHostedRunIngestionManager(repo, "secret", &fakeHostedRunWorkflowSignaler{}, nil, slog.Default())

	if err := manager.IngestEvent(context.Background(), runID, token, event); err != nil {
		t.Fatalf("IngestEvent returned error: %v", err)
	}
	if repo.recordParams == nil {
		t.Fatalf("expected record params to be captured")
	}
	if repo.recordParams.Event.EventType != runevents.EventTypeSystemRunFailed {
		t.Fatalf("recorded event type = %q, want %q", repo.recordParams.Event.EventType, runevents.EventTypeSystemRunFailed)
	}
	if repo.recordParams.Event.Summary.Status != "failed" {
		t.Fatalf("recorded summary status = %q, want failed", repo.recordParams.Event.Summary.Status)
	}
}

func TestHostedRunIngestionManagerRejectsInvalidToken(t *testing.T) {
	manager := NewHostedRunIngestionManager(&fakeHostedRunExecutionRepository{}, "secret", &fakeHostedRunWorkflowSignaler{}, nil, slog.Default())

	err := manager.IngestEvent(context.Background(), uuid.New(), "bad-token", hostedruns.Event{
		RunAgentID:    uuid.New(),
		ExternalRunID: "ext-123",
		EventType:     hostedruns.EventTypeError,
		OccurredAt:    time.Now().UTC(),
		ErrorMessage:  stringPtr("boom"),
	})
	if !errors.Is(err, hostedruns.ErrInvalidCallbackToken) {
		t.Fatalf("error = %v, want invalid callback token", err)
	}
}

func TestIngestHostedRunEventHandlerRejectsMalformedEvent(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	token, err := hostedruns.NewCallbackTokenSigner("secret").Sign(runID, runAgentID)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/hosted-runs/"+runID.String()+"/events", bytes.NewBufferString(`{
		"run_agent_id":"`+runAgentID.String()+`",
		"external_run_id":"ext-123",
		"event_type":"run_finished",
		"occurred_at":"2026-03-14T10:00:00Z"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	ingestHostedRunEventHandler(slog.New(slog.NewTextHandler(testWriter{t}, nil)), NewHostedRunIngestionManager(
		&fakeHostedRunExecutionRepository{},
		"secret",
		&fakeHostedRunWorkflowSignaler{},
		nil,
		slog.Default(),
	)).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestIngestHostedRunEventHandlerReturnsInternalErrorForRepositoryFailure(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()
	token, err := hostedruns.NewCallbackTokenSigner("secret").Sign(runID, runAgentID)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	eventBody := `{
		"run_agent_id":"` + runAgentID.String() + `",
		"external_run_id":"ext-123",
		"event_type":"error",
		"occurred_at":"2026-03-14T10:00:00Z",
		"error_message":"boom"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/hosted-runs/"+runID.String()+"/events", bytes.NewBufferString(eventBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router := newRouter("dev", nil,
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
		nil,
		0,
		stubRunCreationService{},
		stubRunReadService{},
		stubReplayReadService{},
		NewHostedRunIngestionManager(
			&fakeHostedRunExecutionRepository{
				execution: repository.HostedRunExecution{
					RunID:         runID,
					RunAgentID:    runAgentID,
					ExternalRunID: stringPtr("ext-123"),
					Status:        "accepted",
				},
				applyErr: errors.New("db unavailable"),
			},
			"secret",
			&fakeHostedRunWorkflowSignaler{},
			nil,
			slog.Default(),
		),
		nil,
		stubAgentDeploymentReadService{},
		stubChallengePackReadService{},
		stubAgentBuildService{},
		noopReleaseGateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

type fakeHostedRunExecutionRepository struct {
	execution    repository.HostedRunExecution
	getErr       error
	applyParams  *repository.ApplyHostedRunEventParams
	applyErr     error
	recordParams *repository.RecordHostedRunEventParams
	recordErr    error
}

func (f *fakeHostedRunExecutionRepository) GetHostedRunExecutionByRunAgentID(_ context.Context, _ uuid.UUID) (repository.HostedRunExecution, error) {
	return f.execution, f.getErr
}

func (f *fakeHostedRunExecutionRepository) ApplyHostedRunEvent(_ context.Context, params repository.ApplyHostedRunEventParams) (repository.HostedRunExecution, error) {
	f.applyParams = &params
	return f.execution, f.applyErr
}

func (f *fakeHostedRunExecutionRepository) RecordHostedRunEvent(_ context.Context, params repository.RecordHostedRunEventParams) (repository.RunAgentReplay, error) {
	f.recordParams = &params
	return repository.RunAgentReplay{}, f.recordErr
}

type fakeHostedRunWorkflowSignaler struct {
	signalCount int
}

func (f *fakeHostedRunWorkflowSignaler) SignalRunAgentWorkflow(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ hostedruns.Event) error {
	f.signalCount++
	return nil
}

func stringPtr(value string) *string {
	return &value
}
