package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

func turnEvent(seq int64, eventType runevents.Type, payload map[string]any) repository.RunEvent {
	raw, _ := json.Marshal(payload)
	return repository.RunEvent{
		RunID:          uuid.New(),
		RunAgentID:     uuid.New(),
		SequenceNumber: seq,
		EventType:      eventType,
		Source:         runevents.SourceMultiTurnEngine,
		Payload:        raw,
	}
}

func TestReplayReadManager_GetRunAgentTranscript_ReconstructsTurns(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()

	events := []repository.RunEvent{
		turnEvent(1, runevents.EventTypeTurnUserMessage, map[string]any{
			"turn_index": 0, "actor": "llm", "phase_id": "interview",
			"content": "Where do I start?",
		}),
		turnEvent(2, runevents.EventTypeTurnAssistantMessage, map[string]any{
			"turn_index": 0, "phase_id": "interview",
			"content": "What languages do you know?",
		}),
		turnEvent(3, runevents.EventTypeTurnCompleted, map[string]any{
			"turn_index": 0, "mismatch": true,
		}),
		turnEvent(4, runevents.EventTypeTurnUserMessage, map[string]any{
			"turn_index": 1, "actor": "llm", "phase_id": "interview",
			"content": "No Python.",
		}),
		turnEvent(5, runevents.EventTypeTurnAssistantMessage, map[string]any{
			"turn_index": 1, "phase_id": "interview",
			"content": "We'll start with Python then.",
		}),
		turnEvent(6, runevents.EventTypeConversationCompleted, map[string]any{
			"turn_count": 2,
		}),
	}

	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          runAgentID,
			RunID:       uuid.New(),
			WorkspaceID: workspaceID,
			Status:      domain.RunAgentStatusCompleted,
		},
		runEvents: events,
	})

	result, err := manager.GetRunAgentTranscript(
		context.Background(),
		callerForWorkspace(workspaceID),
		runAgentID,
	)
	if err != nil {
		t.Fatalf("GetRunAgentTranscript: %v", err)
	}
	if result.State != ReplayStateReady {
		t.Fatalf("state = %q, want ready", result.State)
	}
	if len(result.Turns) != 2 {
		t.Fatalf("turn count = %d, want 2", len(result.Turns))
	}

	first := result.Turns[0]
	if first.UserMessage != "Where do I start?" || first.AssistantMessage != "What languages do you know?" {
		t.Fatalf("turn 0 content mismatch: user=%q assistant=%q", first.UserMessage, first.AssistantMessage)
	}
	if !first.Mismatch {
		t.Fatal("turn 0 should be flagged mismatch (from turn.completed payload)")
	}
	if first.Actor != "llm" || first.PhaseID != "interview" {
		t.Fatalf("turn 0 metadata mismatch: actor=%q phase=%q", first.Actor, first.PhaseID)
	}

	second := result.Turns[1]
	if second.UserMessage != "No Python." || second.AssistantMessage != "We'll start with Python then." {
		t.Fatalf("turn 1 content mismatch: user=%q assistant=%q", second.UserMessage, second.AssistantMessage)
	}
}

func TestReplayReadManager_GetRunAgentTranscript_SingleTurnHasNoTurns(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()

	// A single-turn (native) run emits no turn.* events — only model/tool/run
	// events. The transcript should resolve to ready with zero turns, not error.
	events := []repository.RunEvent{
		turnEvent(1, runevents.EventTypeModelCallCompleted, map[string]any{"output_text": "hi"}),
		turnEvent(2, runevents.EventTypeSystemRunCompleted, map[string]any{}),
	}

	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          runAgentID,
			RunID:       uuid.New(),
			WorkspaceID: workspaceID,
			Status:      domain.RunAgentStatusCompleted,
		},
		runEvents: events,
	})

	result, err := manager.GetRunAgentTranscript(
		context.Background(),
		callerForWorkspace(workspaceID),
		runAgentID,
	)
	if err != nil {
		t.Fatalf("GetRunAgentTranscript: %v", err)
	}
	if result.State != ReplayStateReady {
		t.Fatalf("state = %q, want ready", result.State)
	}
	if len(result.Turns) != 0 {
		t.Fatalf("turn count = %d, want 0 for single-turn run", len(result.Turns))
	}
}

func TestReplayReadManager_GetRunAgentTranscript_FailedRunIsErroredButKeepsPartialTurns(t *testing.T) {
	workspaceID := uuid.New()
	runAgentID := uuid.New()

	// A run that recorded one turn then failed: the transcript must surface
	// `errored` (so callers don't mistake it for a clean finish) while still
	// returning the partial conversation in the body.
	events := []repository.RunEvent{
		turnEvent(1, runevents.EventTypeTurnUserMessage, map[string]any{
			"turn_index": 0, "actor": "llm", "content": "Hi",
		}),
		turnEvent(2, runevents.EventTypeTurnAssistantMessage, map[string]any{
			"turn_index": 0, "content": "Hello — what's your goal?",
		}),
	}

	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          runAgentID,
			RunID:       uuid.New(),
			WorkspaceID: workspaceID,
			Status:      domain.RunAgentStatusFailed,
		},
		runEvents: events,
	})

	result, err := manager.GetRunAgentTranscript(
		context.Background(),
		callerForWorkspace(workspaceID),
		runAgentID,
	)
	if err != nil {
		t.Fatalf("GetRunAgentTranscript: %v", err)
	}
	if result.State != ReplayStateErrored {
		t.Fatalf("state = %q, want errored for a failed run", result.State)
	}
	if len(result.Turns) != 1 {
		t.Fatalf("turn count = %d, want 1 partial turn retained", len(result.Turns))
	}
}

func TestReplayReadManager_GetRunAgentTranscript_DeniesOtherWorkspace(t *testing.T) {
	manager := NewReplayReadManager(NewCallerWorkspaceAuthorizer(), &fakeReplayReadRepository{
		runAgent: domain.RunAgent{
			ID:          uuid.New(),
			RunID:       uuid.New(),
			WorkspaceID: uuid.New(),
			Status:      domain.RunAgentStatusCompleted,
		},
	})

	_, err := manager.GetRunAgentTranscript(
		context.Background(),
		callerForWorkspace(uuid.New()), // different workspace
		uuid.New(),
	)
	if err == nil {
		t.Fatal("expected authorization error for caller from a different workspace")
	}
}

func callerForWorkspace(workspaceID uuid.UUID) Caller {
	return Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"},
		},
	}
}
