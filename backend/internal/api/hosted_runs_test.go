package api

import (
	"bytes"
	"context"
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
	manager := NewHostedRunIngestionManager(repo, "secret", signaler)

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

func TestHostedRunIngestionManagerRejectsInvalidToken(t *testing.T) {
	manager := NewHostedRunIngestionManager(&fakeHostedRunExecutionRepository{}, "secret", &fakeHostedRunWorkflowSignaler{})

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
	router := newRouter(
		slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		NewDevelopmentAuthenticator(),
		NewCallerWorkspaceAuthorizer(),
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
		),
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
