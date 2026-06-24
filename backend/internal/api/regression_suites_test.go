package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/failurereview"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRegressionManagerRejectsCrossWorkspaceSuiteAccess(t *testing.T) {
	requestWorkspaceID := uuid.New()
	actualWorkspaceID := uuid.New()
	suiteID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		suite: repository.RegressionSuite{
			ID:          suiteID,
			WorkspaceID: actualWorkspaceID,
			Status:      domain.RegressionSuiteStatusActive,
		},
	})

	_, err := manager.GetRegressionSuite(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			requestWorkspaceID: {WorkspaceID: requestWorkspaceID, Role: RoleWorkspaceMember},
		},
	}, GetRegressionSuiteInput{
		WorkspaceID: requestWorkspaceID,
		SuiteID:     suiteID,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetRegressionSuite error = %v, want ErrForbidden", err)
	}
}

func TestRegressionManagerRejectsCrossWorkspaceCasePatch(t *testing.T) {
	requestWorkspaceID := uuid.New()
	actualWorkspaceID := uuid.New()
	caseID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		regressionCase: repository.RegressionCase{
			ID:          caseID,
			WorkspaceID: actualWorkspaceID,
			Status:      domain.RegressionCaseStatusActive,
		},
	})

	status := domain.RegressionCaseStatusMuted
	_, err := manager.PatchRegressionCase(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			requestWorkspaceID: {WorkspaceID: requestWorkspaceID, Role: RoleWorkspaceMember},
		},
	}, PatchRegressionCaseInput{
		WorkspaceID: requestWorkspaceID,
		CaseID:      caseID,
		Status:      &status,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("PatchRegressionCase error = %v, want ErrForbidden", err)
	}
}

func TestRegressionManagerRejectsInvisibleEvalPackOnCreate(t *testing.T) {
	workspaceID := uuid.New()
	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		evalPacks: []repository.EvalPackSummary{{ID: uuid.New()}},
	})

	_, err := manager.CreateRegressionSuite(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, CreateRegressionSuiteInput{
		WorkspaceID:           workspaceID,
		SourceEvalPackID: uuid.New(),
		Name:                  "Critical regressions",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
	})
	if !errors.Is(err, ErrEvalPackNotFound) {
		t.Fatalf("CreateRegressionSuite error = %v, want ErrEvalPackNotFound", err)
	}
}

func TestRegressionManagerPromoteFailureDefaultsSeverity(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()
	challengeIdentityID := uuid.New()
	evalPackID := uuid.New()
	item := failurereview.Item{
		RunID:                  runID,
		RunAgentID:             uuid.New(),
		ChallengeIdentityID:    &challengeIdentityID,
		ChallengeKey:           "ticket-a",
		CaseKey:                "case-a",
		ItemKey:                "prompt.txt",
		FailureClass:           failurereview.FailureClassPolicyViolation,
		Promotable:             true,
		PromotionModeAvailable: []failurereview.PromotionMode{failurereview.PromotionModeFullExecutable},
		EvidenceTier:           failurereview.EvidenceTierNativeStructured,
	}

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceEvalPackID: evalPackID,
			Status:                domain.RegressionSuiteStatusActive,
		},
		failureItems: []failurereview.Item{item},
		executionContext: repository.RunAgentExecutionContext{
			EvalPackVersion: repository.EvalPackVersionExecutionContext{
				EvalPackID: evalPackID,
			},
		},
		scorecard: repository.RunAgentScorecard{
			EvaluationSpecID: uuid.New(),
		},
		evaluationSpec: repository.EvaluationSpecRecord{
			Definition: json.RawMessage(`{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"runtime_limits":{"max_duration_ms":60000},"scorecard":{"dimensions":["correctness"]}}`),
		},
		promoteResult: repository.PromoteFailureResult{
			Case: repository.RegressionCase{ID: uuid.New(), WorkspaceID: workspaceID},
		},
	})

	proposedStatus := domain.RegressionCaseStatusProposed
	result, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: challengeIdentityID,
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
			Status:        &proposedStatus,
		},
	})
	if err != nil {
		t.Fatalf("PromoteFailure returned error: %v", err)
	}
	if manager.repo.(*fakeRegressionRepository).promoteInput == nil {
		t.Fatal("expected promote input to be captured")
	}
	if got := manager.repo.(*fakeRegressionRepository).promoteInput.Severity; got != domain.RegressionSeverityBlocking {
		t.Fatalf("default severity = %s, want blocking", got)
	}
	if got := manager.repo.(*fakeRegressionRepository).promoteInput.Status; got != domain.RegressionCaseStatusProposed {
		t.Fatalf("status = %s, want proposed", got)
	}
	var expectedContract map[string]any
	if err := json.Unmarshal(manager.repo.(*fakeRegressionRepository).promoteInput.ExpectedContract, &expectedContract); err != nil {
		t.Fatalf("json.Unmarshal expected contract returned error: %v", err)
	}
	if _, ok := expectedContract["scorecard"]; !ok {
		t.Fatalf("expected contract = %#v, want scorecard subset", expectedContract)
	}
	if _, ok := expectedContract["runtime_limits"]; ok {
		t.Fatalf("expected contract = %#v, did not expect runtime_limits in frozen subset", expectedContract)
	}
	if result.Case.ID == uuid.Nil {
		t.Fatal("expected promoted case to be returned")
	}
}

func TestRegressionManagerPromoteFailureRejectsPackMismatch(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()
	challengeIdentityID := uuid.New()
	item := failurereview.Item{
		RunID:                  runID,
		RunAgentID:             uuid.New(),
		ChallengeIdentityID:    &challengeIdentityID,
		ChallengeKey:           "ticket-a",
		CaseKey:                "case-a",
		ItemKey:                "prompt.txt",
		FailureClass:           failurereview.FailureClassOther,
		Promotable:             true,
		PromotionModeAvailable: []failurereview.PromotionMode{failurereview.PromotionModeFullExecutable},
		EvidenceTier:           failurereview.EvidenceTierNativeStructured,
	}

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceEvalPackID: uuid.New(),
			Status:                domain.RegressionSuiteStatusActive,
		},
		failureItems: []failurereview.Item{item},
		executionContext: repository.RunAgentExecutionContext{
			EvalPackVersion: repository.EvalPackVersionExecutionContext{
				EvalPackID: uuid.New(),
			},
		},
	})

	_, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: challengeIdentityID,
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
		},
	})
	if !errors.Is(err, ErrRegressionSuitePackMismatch) {
		t.Fatalf("PromoteFailure error = %v, want ErrRegressionSuitePackMismatch", err)
	}
}

func TestRegressionManagerPromoteFailureRejectsArchivedSuite(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		suite: repository.RegressionSuite{
			ID:          suiteID,
			WorkspaceID: workspaceID,
			Status:      domain.RegressionSuiteStatusArchived,
		},
	})

	_, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: uuid.New(),
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
		},
	})
	if !errors.Is(err, ErrRegressionSuiteArchived) {
		t.Fatalf("PromoteFailure error = %v, want ErrRegressionSuiteArchived", err)
	}
}

func TestRegressionManagerPromoteFailureRejectsCrossWorkspaceSuite(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		suite: repository.RegressionSuite{
			ID:          suiteID,
			WorkspaceID: uuid.New(),
			Status:      domain.RegressionSuiteStatusActive,
		},
	})

	_, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: uuid.New(),
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
		},
	})
	if !errors.Is(err, repository.ErrRegressionSuiteNotFound) {
		t.Fatalf("PromoteFailure error = %v, want ErrRegressionSuiteNotFound", err)
	}
}

func TestRegressionManagerPromoteFailureRejectsNonPromotableItem(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()
	challengeIdentityID := uuid.New()
	evalPackID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceEvalPackID: evalPackID,
			Status:                domain.RegressionSuiteStatusActive,
		},
		failureItems: []failurereview.Item{{
			RunID:                  runID,
			RunAgentID:             uuid.New(),
			ChallengeIdentityID:    &challengeIdentityID,
			Promotable:             false,
			PromotionModeAvailable: []failurereview.PromotionMode{failurereview.PromotionModeFullExecutable},
		}},
	})

	_, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: challengeIdentityID,
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
		},
	})
	if !errors.Is(err, ErrFailurePromotionNotAllowed) {
		t.Fatalf("PromoteFailure error = %v, want ErrFailurePromotionNotAllowed", err)
	}
}

func TestRegressionManagerPromoteFailureRejectsUnavailableMode(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()
	challengeIdentityID := uuid.New()
	evalPackID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceEvalPackID: evalPackID,
			Status:                domain.RegressionSuiteStatusActive,
		},
		failureItems: []failurereview.Item{{
			RunID:                  runID,
			RunAgentID:             uuid.New(),
			ChallengeIdentityID:    &challengeIdentityID,
			Promotable:             true,
			PromotionModeAvailable: []failurereview.PromotionMode{failurereview.PromotionModeOutputOnly},
		}},
	})

	_, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: challengeIdentityID,
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
		},
	})
	if !errors.Is(err, ErrFailurePromotionModeUnavailable) {
		t.Fatalf("PromoteFailure error = %v, want ErrFailurePromotionModeUnavailable", err)
	}
}

func TestRegressionManagerPromoteFailureRejectsCrossWorkspaceRun(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: uuid.New()},
	})

	_, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: uuid.New(),
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
		},
	})
	if !errors.Is(err, repository.ErrRunNotFound) {
		t.Fatalf("PromoteFailure error = %v, want ErrRunNotFound", err)
	}
}

func TestRegressionManagerPromoteFailureRejectsAmbiguousFailureItem(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()
	challengeIdentityID := uuid.New()
	evalPackID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceEvalPackID: evalPackID,
			Status:                domain.RegressionSuiteStatusActive,
		},
		failureItems: []failurereview.Item{
			{RunID: runID, RunAgentID: uuid.New(), ChallengeIdentityID: &challengeIdentityID, Promotable: true},
			{RunID: runID, RunAgentID: uuid.New(), ChallengeIdentityID: &challengeIdentityID, Promotable: true},
		},
	})

	_, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: challengeIdentityID,
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
		},
	})
	if !errors.Is(err, ErrFailureReviewItemAmbiguous) {
		t.Fatalf("PromoteFailure error = %v, want ErrFailureReviewItemAmbiguous", err)
	}
}

func TestRegressionManagerPromoteFailureResolvesDuplicateChallengeIdentityWithRunAgentID(t *testing.T) {
	workspaceID := uuid.New()
	runID := uuid.New()
	suiteID := uuid.New()
	challengeIdentityID := uuid.New()
	evalPackID := uuid.New()
	selectedRunAgentID := uuid.New()
	otherRunAgentID := uuid.New()

	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), &fakeRegressionRepository{
		run: domain.Run{ID: runID, WorkspaceID: workspaceID},
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceEvalPackID: evalPackID,
			Status:                domain.RegressionSuiteStatusActive,
		},
		failureItems: []failurereview.Item{
			{
				RunID:                  runID,
				RunAgentID:             otherRunAgentID,
				ChallengeIdentityID:    &challengeIdentityID,
				Promotable:             true,
				PromotionModeAvailable: []failurereview.PromotionMode{failurereview.PromotionModeFullExecutable},
			},
			{
				RunID:                  runID,
				RunAgentID:             selectedRunAgentID,
				ChallengeIdentityID:    &challengeIdentityID,
				ChallengeKey:           "ticket-a",
				CaseKey:                "case-a",
				ItemKey:                "prompt.txt",
				FailureClass:           failurereview.FailureClassPolicyViolation,
				Promotable:             true,
				PromotionModeAvailable: []failurereview.PromotionMode{failurereview.PromotionModeFullExecutable},
				EvidenceTier:           failurereview.EvidenceTierNativeStructured,
			},
		},
		executionContext: repository.RunAgentExecutionContext{
			EvalPackVersion: repository.EvalPackVersionExecutionContext{
				EvalPackID: evalPackID,
			},
		},
		scorecard: repository.RunAgentScorecard{
			EvaluationSpecID: uuid.New(),
		},
		evaluationSpec: repository.EvaluationSpecRecord{
			Definition: json.RawMessage(`{"name":"spec","version_number":1,"judge_mode":"deterministic","validators":[{"key":"exact","type":"exact_match","target":"final_output","expected_from":"challenge_input"}],"runtime_limits":{"max_duration_ms":60000},"scorecard":{"dimensions":["correctness"]}}`),
		},
		promoteResult: repository.PromoteFailureResult{
			Case:    repository.RegressionCase{ID: uuid.New(), WorkspaceID: workspaceID},
			Created: true,
		},
	})

	result, err := manager.PromoteFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, PromoteFailureInput{
		WorkspaceID:         workspaceID,
		RunID:               runID,
		ChallengeIdentityID: challengeIdentityID,
		RunAgentID:          &selectedRunAgentID,
		Request: domain.PromotionRequest{
			SuiteID:       suiteID,
			PromotionMode: domain.RegressionPromotionModeFullExecutable,
			Title:         "Promoted failure",
		},
	})
	if err != nil {
		t.Fatalf("PromoteFailure returned error: %v", err)
	}
	if !result.Created {
		t.Fatalf("PromoteFailure created = %v, want true", result.Created)
	}
	if manager.repo.(*fakeRegressionRepository).promoteInput == nil {
		t.Fatal("expected promote input to be captured")
	}
	if got := manager.repo.(*fakeRegressionRepository).promoteInput.RunAgentID; got != selectedRunAgentID {
		t.Fatalf("promote run_agent_id = %s, want %s", got, selectedRunAgentID)
	}
}

func TestRegressionManagerCaptureProductionFailureCreatesProposedCase(t *testing.T) {
	workspaceID := uuid.New()
	suiteID := uuid.New()
	evalPackID := uuid.New()
	versionID := uuid.New()
	inputSetID := uuid.New()
	challengeIdentityID := uuid.New()
	observedAt := time.Date(2026, 5, 5, 10, 30, 0, 0, time.UTC)

	repo := &fakeRegressionRepository{
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceEvalPackID: evalPackID,
			Status:                domain.RegressionSuiteStatusActive,
		},
		evalPackVersion: repository.RunnableEvalPackVersion{
			ID:              versionID,
			EvalPackID: evalPackID,
			WorkspaceID:     &workspaceID,
		},
		challengeInputSet: repository.ChallengeInputSet{
			ID:                     inputSetID,
			EvalPackVersionID: versionID,
		},
		challengeIdentityIDs: []uuid.UUID{challengeIdentityID},
		createCaseResult:     repository.RegressionCase{WorkspaceID: workspaceID},
	}
	manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), repo)

	regressionCase, err := manager.CaptureProductionFailure(context.Background(), Caller{
		UserID: uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
		},
	}, CaptureProductionFailureInput{
		WorkspaceID:                  workspaceID,
		SuiteID:                      suiteID,
		SourceEvalPackVersionID: versionID,
		SourceChallengeInputSetID:    &inputSetID,
		SourceChallengeIdentityID:    challengeIdentityID,
		SourceCaseKey:                "prod-incident-123",
		Title:                        "Production tool schema incident",
		FailureSummary:               "Agent emitted an invalid tool argument in production.",
		FailureClass:                 "tool_argument_error",
		PayloadSnapshot:              json.RawMessage(`{"ticket":"example"}`),
		ExpectedContract:             json.RawMessage(`{"validators":[]}`),
		Metadata:                     json.RawMessage(`{"origin":"caller","team":"support"}`),
		IncidentID:                   "INC-123",
		ExternalURL:                  "https://example.test/incidents/INC-123",
		ObservedAt:                   &observedAt,
	})
	if err != nil {
		t.Fatalf("CaptureProductionFailure returned error: %v", err)
	}
	if regressionCase.ID == uuid.Nil {
		t.Fatal("expected created regression case")
	}
	if repo.createCaseInput == nil {
		t.Fatal("expected create case input")
	}
	if repo.createCaseInput.Status != domain.RegressionCaseStatusProposed {
		t.Fatalf("status = %s, want proposed", repo.createCaseInput.Status)
	}
	if repo.createCaseInput.PromotionMode != domain.RegressionPromotionModeOutputOnly {
		t.Fatalf("promotion mode = %s, want output_only", repo.createCaseInput.PromotionMode)
	}
	if repo.createCaseInput.Severity != domain.RegressionSeverityWarning {
		t.Fatalf("severity = %s, want warning", repo.createCaseInput.Severity)
	}
	if repo.createCaseInput.SourceRunID != nil || repo.createCaseInput.SourceRunAgentID != nil {
		t.Fatalf("source run fields = %v/%v, want nil", repo.createCaseInput.SourceRunID, repo.createCaseInput.SourceRunAgentID)
	}
	var metadata map[string]any
	if err := json.Unmarshal(repo.createCaseInput.Metadata, &metadata); err != nil {
		t.Fatalf("metadata unmarshal: %v", err)
	}
	if metadata["origin"] != "production_failure" {
		t.Fatalf("metadata origin = %#v, want production_failure", metadata["origin"])
	}
	if metadata["team"] != "support" || metadata["production_incident_id"] != "INC-123" {
		t.Fatalf("metadata = %#v, want caller and incident fields", metadata)
	}
	if metadata["source_failure_fingerprint"] == "" || metadata["source_failure_cluster_key"] == "" {
		t.Fatalf("metadata = %#v, want generated failure keys", metadata)
	}
}

func TestProductionFailureMetadataDefaultsSourceBeforeFingerprint(t *testing.T) {
	versionID := uuid.New()
	challengeIdentityID := uuid.New()
	baseInput := CaptureProductionFailureInput{
		SourceEvalPackVersionID: versionID,
		SourceChallengeIdentityID:    challengeIdentityID,
		SourceCaseKey:                "prod-incident-123",
		FailureSummary:               "Agent emitted an invalid tool argument in production.",
	}
	defaultMetadata, err := productionFailureMetadata(baseInput, string(failurereview.FailureClassToolArgumentError))
	if err != nil {
		t.Fatalf("productionFailureMetadata default source: %v", err)
	}
	baseInput.Source = "production"
	explicitMetadata, err := productionFailureMetadata(baseInput, string(failurereview.FailureClassToolArgumentError))
	if err != nil {
		t.Fatalf("productionFailureMetadata explicit source: %v", err)
	}

	var defaultValues map[string]any
	if err := json.Unmarshal(defaultMetadata, &defaultValues); err != nil {
		t.Fatalf("default metadata unmarshal: %v", err)
	}
	var explicitValues map[string]any
	if err := json.Unmarshal(explicitMetadata, &explicitValues); err != nil {
		t.Fatalf("explicit metadata unmarshal: %v", err)
	}
	if defaultValues["source"] != "production" {
		t.Fatalf("default source = %#v, want production", defaultValues["source"])
	}
	if defaultValues["source_failure_fingerprint"] != explicitValues["source_failure_fingerprint"] {
		t.Fatalf("fingerprint default source = %#v, explicit production = %#v", defaultValues["source_failure_fingerprint"], explicitValues["source_failure_fingerprint"])
	}
}

func TestRegressionManagerCaptureProductionFailureRejectsInvalidSources(t *testing.T) {
	workspaceID := uuid.New()
	suiteID := uuid.New()
	evalPackID := uuid.New()
	versionID := uuid.New()
	inputSetID := uuid.New()
	challengeIdentityID := uuid.New()

	baseRepo := func() *fakeRegressionRepository {
		return &fakeRegressionRepository{
			suite: repository.RegressionSuite{
				ID:                    suiteID,
				WorkspaceID:           workspaceID,
				SourceEvalPackID: evalPackID,
				Status:                domain.RegressionSuiteStatusActive,
			},
			evalPackVersion: repository.RunnableEvalPackVersion{
				ID:              versionID,
				EvalPackID: evalPackID,
				WorkspaceID:     &workspaceID,
			},
			challengeInputSet: repository.ChallengeInputSet{
				ID:                     inputSetID,
				EvalPackVersionID: versionID,
			},
			challengeIdentityIDs: []uuid.UUID{challengeIdentityID},
		}
	}
	baseInput := CaptureProductionFailureInput{
		WorkspaceID:                  workspaceID,
		SuiteID:                      suiteID,
		SourceEvalPackVersionID: versionID,
		SourceChallengeInputSetID:    &inputSetID,
		SourceChallengeIdentityID:    challengeIdentityID,
		SourceCaseKey:                "prod-incident-123",
		Title:                        "Production incident",
		FailureSummary:               "Agent failed in production.",
		PayloadSnapshot:              json.RawMessage(`{"ticket":"example"}`),
	}

	testCases := []struct {
		name    string
		mutate  func(*fakeRegressionRepository, *CaptureProductionFailureInput)
		wantErr error
	}{
		{
			name: "archived suite",
			mutate: func(repo *fakeRegressionRepository, _ *CaptureProductionFailureInput) {
				repo.suite.Status = domain.RegressionSuiteStatusArchived
			},
			wantErr: ErrRegressionSuiteArchived,
		},
		{
			name: "pack mismatch",
			mutate: func(repo *fakeRegressionRepository, _ *CaptureProductionFailureInput) {
				repo.evalPackVersion.EvalPackID = uuid.New()
			},
			wantErr: ErrRegressionSuitePackMismatch,
		},
		{
			name: "input set mismatch",
			mutate: func(repo *fakeRegressionRepository, _ *CaptureProductionFailureInput) {
				repo.challengeInputSet.EvalPackVersionID = uuid.New()
			},
			wantErr: ErrRegressionInputSetMismatch,
		},
		{
			name: "missing identity",
			mutate: func(repo *fakeRegressionRepository, _ *CaptureProductionFailureInput) {
				repo.challengeIdentityIDs = []uuid.UUID{uuid.New()}
			},
			wantErr: ErrRegressionChallengeMismatch,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := baseRepo()
			input := baseInput
			tc.mutate(repo, &input)
			manager := NewRegressionManager(NewCallerWorkspaceAuthorizer(), repo)
			_, err := manager.CaptureProductionFailure(context.Background(), Caller{
				UserID: uuid.New(),
				WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
					workspaceID: {WorkspaceID: workspaceID, Role: RoleWorkspaceMember},
				},
			}, input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CaptureProductionFailure error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestCaptureProductionFailureEndpointCreatesProposedCase(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	suiteID := uuid.New()
	versionID := uuid.New()
	inputSetID := uuid.New()
	challengeIdentityID := uuid.New()
	service := &fakeRegressionService{
		regressionCase: repository.RegressionCase{
			WorkspaceID:   workspaceID,
			Severity:      domain.RegressionSeverityBlocking,
			PromotionMode: domain.RegressionPromotionModeManual,
			EvidenceTier:  "hosted_black_box",
			FailureClass:  "tool_argument_error",
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		},
	}
	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		evalPackReadService:   stubEvalPackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService:          service,
	})

	body := fmt.Sprintf(`{
		"source_eval_pack_version_id":"%s",
		"source_challenge_input_set_id":"%s",
		"source_challenge_identity_id":"%s",
		"source_case_key":"prod-incident-123",
		"title":"Production tool schema incident",
		"failure_summary":"Agent emitted an invalid tool argument in production.",
		"failure_class":"tool_argument_error",
		"evidence_tier":"hosted_black_box",
		"severity":"blocking",
		"promotion_mode":"manual",
		"payload_snapshot":{"ticket":"example"},
		"expected_contract":{"validators":[]},
		"metadata":{"incident_id":"INC-123"}
	}`, versionID, inputSetID, challengeIdentityID)
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String()+"/production-failures", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	if service.captureInput == nil {
		t.Fatal("expected capture input")
	}
	if service.captureInput.SuiteID != suiteID || service.captureInput.SourceEvalPackVersionID != versionID {
		t.Fatalf("capture input = %+v, want suite/version", service.captureInput)
	}
	if service.captureInput.Severity == nil || *service.captureInput.Severity != domain.RegressionSeverityBlocking {
		t.Fatalf("severity = %v, want blocking", service.captureInput.Severity)
	}
	if service.captureInput.PromotionMode == nil || *service.captureInput.PromotionMode != domain.RegressionPromotionModeManual {
		t.Fatalf("promotion mode = %v, want manual", service.captureInput.PromotionMode)
	}
}

func TestCaptureProductionFailureEndpointRejectsInvalidClassAndEvidenceTier(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	suiteID := uuid.New()
	versionID := uuid.New()
	challengeIdentityID := uuid.New()
	service := &fakeRegressionService{}
	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		evalPackReadService:   stubEvalPackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService:          service,
	})

	testCases := []struct {
		name string
		body string
	}{
		{
			name: "invalid failure class",
			body: fmt.Sprintf(`{
				"source_eval_pack_version_id":"%s",
				"source_challenge_identity_id":"%s",
				"source_case_key":"prod-incident-123",
				"title":"Production tool schema incident",
				"failure_summary":"Agent emitted an invalid tool argument in production.",
				"failure_class":"custom_agent_breakage",
				"payload_snapshot":{"ticket":"example"}
			}`, versionID, challengeIdentityID),
		},
		{
			name: "invalid evidence tier",
			body: fmt.Sprintf(`{
				"source_eval_pack_version_id":"%s",
				"source_challenge_identity_id":"%s",
				"source_case_key":"prod-incident-123",
				"title":"Production tool schema incident",
				"failure_summary":"Agent emitted an invalid tool argument in production.",
				"evidence_tier":"custom_replay",
				"payload_snapshot":{"ticket":"example"}
			}`, versionID, challengeIdentityID),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String()+"/production-failures", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(headerUserID, userID.String())
			req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
			}
			if service.captureInput != nil {
				t.Fatal("service should not be called for invalid enum input")
			}
		})
	}
}

func TestRegressionSuiteEndpointsRoundTrip(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	sourceEvalPackID := uuid.New()
	suiteID := uuid.New()
	caseID := uuid.New()
	createdAt := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(5 * time.Minute)
	promotionCreatedAt := updatedAt.Add(2 * time.Minute)

	service := &fakeRegressionService{
		suite: repository.RegressionSuite{
			ID:                    suiteID,
			WorkspaceID:           workspaceID,
			SourceEvalPackID: sourceEvalPackID,
			Name:                  "Critical regressions",
			Description:           "Seed suite",
			Status:                domain.RegressionSuiteStatusActive,
			SourceMode:            "derived_only",
			DefaultGateSeverity:   domain.RegressionSeverityWarning,
			CaseCount:             1,
			CreatedByUserID:       userID,
			CreatedAt:             createdAt,
			UpdatedAt:             updatedAt,
		},
		regressionCase: repository.RegressionCase{
			ID:                           caseID,
			SuiteID:                      suiteID,
			WorkspaceID:                  workspaceID,
			Title:                        "Case one",
			Description:                  "First regression case",
			Status:                       domain.RegressionCaseStatusActive,
			Severity:                     domain.RegressionSeverityBlocking,
			PromotionMode:                domain.RegressionPromotionModeFullExecutable,
			SourceEvalPackVersionID: uuid.New(),
			SourceChallengeIdentityID:    uuid.New(),
			SourceCaseKey:                "case-1",
			EvidenceTier:                 "replay",
			FailureClass:                 "behavioral_regression",
			FailureSummary:               "Regressed",
			PayloadSnapshot:              json.RawMessage(`{"payload":"snapshot"}`),
			ExpectedContract:             json.RawMessage(`{"contract":"expected"}`),
			Metadata:                     json.RawMessage(`{"origin":"test","source_challenge_key":"ticket-1","source_failure_fingerprint":"frf-test","source_failure_cluster_key":"frc-test"}`),
			ValidationStats: &repository.RegressionCaseValidationStats{
				RunCount:         5,
				FailureCount:     3,
				PassCount:        2,
				ReproductionRate: 0.6,
				LastOutcome:      "pass",
				LastValidatedAt:  &promotionCreatedAt,
			},
			LatestPromotion: &repository.RegressionPromotion{
				ID:                        uuid.New(),
				WorkspaceRegressionCaseID: caseID,
				SourceRunID:               uuid.New(),
				SourceRunAgentID:          uuid.New(),
				SourceEventRefs:           json.RawMessage(`[{"sequence_number":7}]`),
				PromotedByUserID:          userID,
				PromotionReason:           "Captured from failure review",
				PromotionSnapshot:         json.RawMessage(`{"request":{"title":"Case one"}}`),
				CreatedAt:                 promotionCreatedAt,
			},
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		evalPackReadService:   stubEvalPackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService:          service,
	})

	postReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+workspaceID.String()+"/regression-suites", bytes.NewBufferString(`{
		"source_eval_pack_id":"`+sourceEvalPackID.String()+`",
		"name":"Critical regressions",
		"description":"Seed suite",
		"default_gate_severity":"warning"
	}`))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set(headerUserID, userID.String())
	postReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	postRec := httptest.NewRecorder()
	router.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201", postRec.Code)
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String(), nil)
	getReq.Header.Set(headerUserID, userID.String())
	getReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET suite status = %d, want 200", getRec.Code)
	}

	patchRec := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String(), bytes.NewBufferString(`{
		"description":"Updated suite",
		"status":"archived",
		"default_gate_severity":"blocking"
	}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set(headerUserID, userID.String())
	patchReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH suite status = %d, want 200", patchRec.Code)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/regression-suites?limit=10&offset=0", nil)
	listReq.Header.Set(headerUserID, userID.String())
	listReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("LIST suites status = %d, want 200", listRec.Code)
	}

	var listResponse listRegressionSuitesResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("json.Unmarshal list response returned error: %v", err)
	}
	if len(listResponse.Items) != 1 || listResponse.Items[0].Status != domain.RegressionSuiteStatusArchived {
		t.Fatalf("list suites = %+v, want archived item", listResponse.Items)
	}
	if listResponse.Items[0].CaseCount != 1 {
		t.Fatalf("suite case_count = %d, want 1", listResponse.Items[0].CaseCount)
	}

	casesRec := httptest.NewRecorder()
	casesReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String()+"/cases", nil)
	casesReq.Header.Set(headerUserID, userID.String())
	casesReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(casesRec, casesReq)
	if casesRec.Code != http.StatusOK {
		t.Fatalf("LIST cases status = %d, want 200", casesRec.Code)
	}
	var casesResponse listRegressionCasesResponse
	if err := json.Unmarshal(casesRec.Body.Bytes(), &casesResponse); err != nil {
		t.Fatalf("json.Unmarshal cases response returned error: %v", err)
	}
	if len(casesResponse.Items) != 1 {
		t.Fatalf("cases response items = %d, want 1", len(casesResponse.Items))
	}
	if casesResponse.Items[0].SourceChallengeKey == nil || *casesResponse.Items[0].SourceChallengeKey != "ticket-1" {
		t.Fatalf("source_challenge_key = %v, want ticket-1", casesResponse.Items[0].SourceChallengeKey)
	}
	if casesResponse.Items[0].SourceFailureFingerprint == nil || *casesResponse.Items[0].SourceFailureFingerprint != "frf-test" {
		t.Fatalf("source_failure_fingerprint = %v, want frf-test", casesResponse.Items[0].SourceFailureFingerprint)
	}
	if casesResponse.Items[0].SourceFailureClusterKey == nil || *casesResponse.Items[0].SourceFailureClusterKey != "frc-test" {
		t.Fatalf("source_failure_cluster_key = %v, want frc-test", casesResponse.Items[0].SourceFailureClusterKey)
	}
	validation := casesResponse.Items[0].Validation
	if validation.Status != regressionValidationStatusReproducing {
		t.Fatalf("validation status = %q, want %q", validation.Status, regressionValidationStatusReproducing)
	}
	if validation.MaintenanceStatus != regressionMaintenanceStatusKeepActive {
		t.Fatalf("maintenance status = %q, want %q", validation.MaintenanceStatus, regressionMaintenanceStatusKeepActive)
	}
	if validation.MaintenanceAction == "" {
		t.Fatal("maintenance action is empty")
	}
	if validation.RunCount != 5 || validation.FailureCount != 3 || validation.PassCount != 2 {
		t.Fatalf("validation counts = %+v, want 5/3/2", validation)
	}
	if validation.ReproductionRate == nil || math.Abs(*validation.ReproductionRate-0.6) > 1e-9 {
		t.Fatalf("validation reproduction_rate = %v, want 0.6", validation.ReproductionRate)
	}
	if validation.LastOutcome == nil || *validation.LastOutcome != "pass" {
		t.Fatalf("validation last_outcome = %v, want pass", validation.LastOutcome)
	}
	if validation.LastValidatedAt == nil || !validation.LastValidatedAt.Equal(promotionCreatedAt) {
		t.Fatalf("validation last_validated_at = %v, want %s", validation.LastValidatedAt, promotionCreatedAt)
	}

	workspaceCasesRec := httptest.NewRecorder()
	workspaceCasesReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+workspaceID.String()+"/regression-cases?status=active&limit=10&offset=0", nil)
	workspaceCasesReq.Header.Set(headerUserID, userID.String())
	workspaceCasesReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(workspaceCasesRec, workspaceCasesReq)
	if workspaceCasesRec.Code != http.StatusOK {
		t.Fatalf("LIST workspace cases status = %d, want 200", workspaceCasesRec.Code)
	}
	var workspaceCasesResponse listWorkspaceRegressionCasesResponse
	if err := json.Unmarshal(workspaceCasesRec.Body.Bytes(), &workspaceCasesResponse); err != nil {
		t.Fatalf("json.Unmarshal workspace cases response returned error: %v", err)
	}
	if workspaceCasesResponse.Total != 1 || len(workspaceCasesResponse.Items) != 1 {
		t.Fatalf("workspace cases response = %+v, want one item and total 1", workspaceCasesResponse)
	}
	if service.listCasesInput == nil || service.listCasesInput.Status == nil || *service.listCasesInput.Status != domain.RegressionCaseStatusActive {
		t.Fatalf("workspace case status filter = %+v, want active", service.listCasesInput)
	}
	if workspaceCasesResponse.Items[0].ID != caseID {
		t.Fatalf("workspace case id = %s, want %s", workspaceCasesResponse.Items[0].ID, caseID)
	}

	casePatchRec := httptest.NewRecorder()
	casePatchReq := httptest.NewRequest(http.MethodPatch, "/v1/workspaces/"+workspaceID.String()+"/regression-cases/"+caseID.String(), bytes.NewBufferString(`{
		"title":"Muted case",
		"status":"muted",
		"severity":"warning"
	}`))
	casePatchReq.Header.Set("Content-Type", "application/json")
	casePatchReq.Header.Set(headerUserID, userID.String())
	casePatchReq.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	router.ServeHTTP(casePatchRec, casePatchReq)
	if casePatchRec.Code != http.StatusOK {
		t.Fatalf("PATCH case status = %d, want 200", casePatchRec.Code)
	}

	var patchedCase regressionCaseResponse
	if err := json.Unmarshal(casePatchRec.Body.Bytes(), &patchedCase); err != nil {
		t.Fatalf("json.Unmarshal patched case returned error: %v", err)
	}
	if patchedCase.Status != domain.RegressionCaseStatusMuted {
		t.Fatalf("patched case status = %s, want muted", patchedCase.Status)
	}
	if patchedCase.LatestPromotion == nil {
		t.Fatal("patched case latest_promotion = nil, want populated promotion metadata")
	}
	if patchedCase.LatestPromotion.PromotionReason != "Captured from failure review" {
		t.Fatalf("latest promotion reason = %q, want captured promotion reason", patchedCase.LatestPromotion.PromotionReason)
	}
	if service.patchSuiteInput == nil || service.patchCaseInput == nil {
		t.Fatalf("expected patch inputs to be captured")
	}
}

func TestRegressionFailureProvenanceFromMetadata(t *testing.T) {
	tests := []struct {
		name        string
		metadata    json.RawMessage
		challenge   *string
		fingerprint *string
		cluster     *string
	}{
		{
			name:     "empty metadata",
			metadata: nil,
		},
		{
			name:     "null metadata",
			metadata: json.RawMessage(`null`),
		},
		{
			name:     "malformed metadata",
			metadata: json.RawMessage(`{"source_failure_cluster_key":`),
		},
		{
			name:     "non string values are ignored",
			metadata: json.RawMessage(`{"source_challenge_key":123,"source_failure_fingerprint":true,"source_failure_cluster_key":{"key":"frc"}}`),
		},
		{
			name:        "strings are trimmed and blanks are ignored",
			metadata:    json.RawMessage(`{"source_challenge_key":" ticket-1 ","source_failure_fingerprint":"   ","source_failure_cluster_key":"\tfrc-test\n"}`),
			challenge:   stringPtr("ticket-1"),
			fingerprint: nil,
			cluster:     stringPtr("frc-test"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := regressionFailureProvenanceFromMetadata(tt.metadata)
			assertOptionalString(t, "source_challenge_key", got.SourceChallengeKey, tt.challenge)
			assertOptionalString(t, "source_failure_fingerprint", got.SourceFailureFingerprint, tt.fingerprint)
			assertOptionalString(t, "source_failure_cluster_key", got.SourceFailureClusterKey, tt.cluster)
		})
	}
}

func TestBuildRegressionCaseValidationResponse(t *testing.T) {
	validatedAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name              string
		stats             *repository.RegressionCaseValidationStats
		status            string
		maintenanceStatus string
		rate              *float64
	}{
		{
			name:              "not validated",
			stats:             nil,
			status:            regressionValidationStatusNotValidated,
			maintenanceStatus: regressionMaintenanceStatusNeedsSignal,
		},
		{
			name: "collecting signal",
			stats: &repository.RegressionCaseValidationStats{
				RunCount:         3,
				FailureCount:     2,
				PassCount:        1,
				ReproductionRate: 2.0 / 3.0,
				LastOutcome:      "fail",
				LastValidatedAt:  &validatedAt,
			},
			status:            regressionValidationStatusCollectingSignal,
			maintenanceStatus: regressionMaintenanceStatusNeedsSignal,
			rate:              float64Ptr(2.0 / 3.0),
		},
		{
			name: "reproducing",
			stats: &repository.RegressionCaseValidationStats{
				RunCount:         5,
				FailureCount:     3,
				PassCount:        2,
				ReproductionRate: 0.6,
				LastOutcome:      "pass",
				LastValidatedAt:  &validatedAt,
			},
			status:            regressionValidationStatusReproducing,
			maintenanceStatus: regressionMaintenanceStatusKeepActive,
			rate:              float64Ptr(0.6),
		},
		{
			name: "passing",
			stats: &repository.RegressionCaseValidationStats{
				RunCount:         5,
				FailureCount:     0,
				PassCount:        5,
				ReproductionRate: 0,
				LastOutcome:      "pass",
				LastValidatedAt:  &validatedAt,
			},
			status:            regressionValidationStatusPassing,
			maintenanceStatus: regressionMaintenanceStatusPruneCandidate,
			rate:              float64Ptr(0),
		},
		{
			name: "flaky",
			stats: &repository.RegressionCaseValidationStats{
				RunCount:         5,
				FailureCount:     2,
				PassCount:        3,
				ReproductionRate: 0.4,
				LastOutcome:      "fail",
				LastValidatedAt:  &validatedAt,
			},
			status:            regressionValidationStatusFlaky,
			maintenanceStatus: regressionMaintenanceStatusReviewFlaky,
			rate:              float64Ptr(0.4),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRegressionCaseValidationResponse(tt.stats)
			if got.Status != tt.status {
				t.Fatalf("status = %q, want %q", got.Status, tt.status)
			}
			if got.MaintenanceStatus != tt.maintenanceStatus {
				t.Fatalf("maintenance status = %q, want %q", got.MaintenanceStatus, tt.maintenanceStatus)
			}
			if got.MaintenanceAction == "" {
				t.Fatal("maintenance action is empty")
			}
			if got.RequiredRuns != regressionValidationRequiredRuns {
				t.Fatalf("required_runs = %d, want %d", got.RequiredRuns, regressionValidationRequiredRuns)
			}
			if got.ReproductionThreshold != regressionValidationReproductionThreshold {
				t.Fatalf("reproduction_threshold = %f, want %f", got.ReproductionThreshold, regressionValidationReproductionThreshold)
			}
			if tt.rate == nil {
				if got.ReproductionRate != nil {
					t.Fatalf("reproduction_rate = %v, want nil", got.ReproductionRate)
				}
				return
			}
			if got.ReproductionRate == nil || math.Abs(*got.ReproductionRate-*tt.rate) > 1e-9 {
				t.Fatalf("reproduction_rate = %v, want %f", got.ReproductionRate, *tt.rate)
			}
		})
	}
}

func assertOptionalString(t *testing.T, name string, got *string, want *string) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil || *got != *want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func TestRegressionSuiteEndpointsRejectMalformedPagination(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		evalPackReadService:   stubEvalPackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService:          &fakeRegressionService{},
	})

	for _, path := range []string{
		"/v1/workspaces/" + workspaceID.String() + "/regression-suites?limit=abc&offset=-1",
		"/v1/workspaces/" + workspaceID.String() + "/regression-cases?limit=abc&offset=-1",
		"/v1/workspaces/" + workspaceID.String() + "/regression-cases?status=all",
		"/v1/workspaces/" + workspaceID.String() + "/regression-cases?status=surprise",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(headerUserID, userID.String())
		req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", path, rec.Code)
		}
	}
}

func TestRegressionSuitePatchReturnsConflictOnTransitionConflict(t *testing.T) {
	workspaceID := uuid.New()
	userID := uuid.New()
	suiteID := uuid.New()
	router := buildRouter(routerOptions{
		authMode:                   "dev",
		logger:                     testLogger(t),
		authenticator:              NewDevelopmentAuthenticator(),
		authorizer:                 NewCallerWorkspaceAuthorizer(),
		runCreationService:         stubRunCreationService{},
		runReadService:             stubRunReadService{},
		replayReadService:          stubReplayReadService{},
		hostedRunIngestionService:  stubHostedRunIngestionService{},
		compareReadService:         stubCompareReadService{},
		agentDeploymentReadService: stubAgentDeploymentReadService{},
		evalPackReadService:   stubEvalPackReadService{},
		agentBuildService:          stubAgentBuildService{},
		releaseGateService:         noopReleaseGateService{},
		regressionService: &fakeRegressionService{
			patchSuiteErr: repository.TransitionConflictError{
				Entity:   "regression_suite",
				ID:       suiteID,
				Expected: "active",
			},
		},
	})

	req := httptest.NewRequest(http.MethodPatch, "/v1/workspaces/"+workspaceID.String()+"/regression-suites/"+suiteID.String(), bytes.NewBufferString(`{"status":"archived"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerUserID, userID.String())
	req.Header.Set(headerWorkspaceMemberships, workspaceID.String()+":workspace_member")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("transition conflict status = %d, want 409", rec.Code)
	}
}

type fakeRegressionRepository struct {
	evalPacks          []repository.EvalPackSummary
	evalPacksErr       error
	publicPacksDisabled     bool
	suite                   repository.RegressionSuite
	suiteErr                error
	regressionCase          repository.RegressionCase
	regressionCaseErr       error
	listSuites              []repository.RegressionSuite
	listSuitesErr           error
	listCases               []repository.RegressionCase
	listCasesErr            error
	countSuites             int64
	countSuitesErr          error
	patchSuiteResult        repository.RegressionSuite
	patchSuiteErr           error
	createCaseResult        repository.RegressionCase
	createCaseErr           error
	createCaseInput         *repository.CreateRegressionCaseParams
	patchCaseResult         repository.RegressionCase
	patchCaseErr            error
	evalPackVersion    repository.RunnableEvalPackVersion
	evalPackVersionErr error
	challengeInputSet       repository.ChallengeInputSet
	challengeInputSetErr    error
	challengeIdentityIDs    []uuid.UUID
	challengeIdentityIDsErr error
	run                     domain.Run
	runErr                  error
	failureItems            []failurereview.Item
	failureItemsErr         error
	executionContext        repository.RunAgentExecutionContext
	executionContextErr     error
	scorecard               repository.RunAgentScorecard
	scorecardErr            error
	evaluationSpec          repository.EvaluationSpecRecord
	evaluationSpecErr       error
	promoteResult           repository.PromoteFailureResult
	promoteErr              error
	promoteInput            *repository.PromoteFailureParams
}

func (f *fakeRegressionRepository) ListVisibleEvalPacks(_ context.Context, _ uuid.UUID) ([]repository.EvalPackSummary, error) {
	return f.evalPacks, f.evalPacksErr
}

func (f *fakeRegressionRepository) WorkspacePublicPacksEnabled(context.Context, uuid.UUID) (bool, error) {
	return !f.publicPacksDisabled, nil
}

func (f *fakeRegressionRepository) CreateRegressionSuite(_ context.Context, params repository.CreateRegressionSuiteParams) (repository.RegressionSuite, error) {
	return repository.RegressionSuite{
		ID:                    uuid.New(),
		WorkspaceID:           params.WorkspaceID,
		SourceEvalPackID: params.SourceEvalPackID,
		Name:                  params.Name,
		Description:           params.Description,
		Status:                params.Status,
		SourceMode:            params.SourceMode,
		DefaultGateSeverity:   params.DefaultGateSeverity,
		CreatedByUserID:       params.CreatedByUserID,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	}, nil
}

func (f *fakeRegressionRepository) GetRegressionSuiteByID(_ context.Context, _ uuid.UUID) (repository.RegressionSuite, error) {
	if f.suiteErr != nil {
		return repository.RegressionSuite{}, f.suiteErr
	}
	return f.suite, nil
}

func (f *fakeRegressionRepository) ListRegressionSuitesByWorkspaceID(_ context.Context, _ uuid.UUID, _, _ int32) ([]repository.RegressionSuite, error) {
	return f.listSuites, f.listSuitesErr
}

func (f *fakeRegressionRepository) CountRegressionSuitesByWorkspaceID(_ context.Context, _ uuid.UUID) (int64, error) {
	return f.countSuites, f.countSuitesErr
}

func (f *fakeRegressionRepository) PatchRegressionSuite(_ context.Context, _ repository.PatchRegressionSuiteParams) (repository.RegressionSuite, error) {
	return f.patchSuiteResult, f.patchSuiteErr
}

func (f *fakeRegressionRepository) CreateRegressionCase(_ context.Context, params repository.CreateRegressionCaseParams) (repository.RegressionCase, error) {
	if f.createCaseErr != nil {
		return repository.RegressionCase{}, f.createCaseErr
	}
	f.createCaseInput = &params
	result := f.createCaseResult
	if result.ID == uuid.Nil {
		result.ID = uuid.New()
	}
	result.SuiteID = params.SuiteID
	result.Title = params.Title
	result.Status = params.Status
	result.Severity = params.Severity
	result.PromotionMode = params.PromotionMode
	result.SourceEvalPackVersionID = params.SourceEvalPackVersionID
	result.SourceChallengeInputSetID = cloneUUIDPtr(params.SourceChallengeInputSetID)
	result.SourceChallengeIdentityID = params.SourceChallengeIdentityID
	result.SourceCaseKey = params.SourceCaseKey
	result.SourceItemKey = cloneStringPtr(params.SourceItemKey)
	result.EvidenceTier = params.EvidenceTier
	result.FailureClass = params.FailureClass
	result.FailureSummary = params.FailureSummary
	result.PayloadSnapshot = params.PayloadSnapshot
	result.ExpectedContract = params.ExpectedContract
	result.ValidatorOverrides = params.ValidatorOverrides
	result.Metadata = params.Metadata
	return result, nil
}

func (f *fakeRegressionRepository) GetRegressionCaseByID(_ context.Context, _ uuid.UUID) (repository.RegressionCase, error) {
	if f.regressionCaseErr != nil {
		return repository.RegressionCase{}, f.regressionCaseErr
	}
	return f.regressionCase, nil
}

func (f *fakeRegressionRepository) ListRegressionCasesBySuiteID(_ context.Context, _ uuid.UUID) ([]repository.RegressionCase, error) {
	return f.listCases, f.listCasesErr
}

func (f *fakeRegressionRepository) ListRegressionCasesByWorkspaceID(_ context.Context, _ repository.ListRegressionCasesByWorkspaceIDParams) ([]repository.RegressionCase, error) {
	return f.listCases, f.listCasesErr
}

func (f *fakeRegressionRepository) CountRegressionCasesByWorkspaceID(_ context.Context, _ uuid.UUID, _ *domain.RegressionCaseStatus) (int64, error) {
	return int64(len(f.listCases)), f.listCasesErr
}

func (f *fakeRegressionRepository) PatchRegressionCase(_ context.Context, _ repository.PatchRegressionCaseParams) (repository.RegressionCase, error) {
	return f.patchCaseResult, f.patchCaseErr
}

func (f *fakeRegressionRepository) GetRunnableEvalPackVersionByID(_ context.Context, _ uuid.UUID) (repository.RunnableEvalPackVersion, error) {
	if f.evalPackVersionErr != nil {
		return repository.RunnableEvalPackVersion{}, f.evalPackVersionErr
	}
	return f.evalPackVersion, nil
}

func (f *fakeRegressionRepository) GetChallengeInputSetByID(_ context.Context, _ uuid.UUID) (repository.ChallengeInputSet, error) {
	if f.challengeInputSetErr != nil {
		return repository.ChallengeInputSet{}, f.challengeInputSetErr
	}
	return f.challengeInputSet, nil
}

func (f *fakeRegressionRepository) ListChallengeIdentityIDsByPackVersionID(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return f.challengeIdentityIDs, f.challengeIdentityIDsErr
}

func (f *fakeRegressionRepository) GetRunByID(_ context.Context, _ uuid.UUID) (domain.Run, error) {
	if f.runErr != nil {
		return domain.Run{}, f.runErr
	}
	return f.run, nil
}

func (f *fakeRegressionRepository) ListRunFailureReviewItems(_ context.Context, _ uuid.UUID, _ *uuid.UUID) ([]failurereview.Item, error) {
	return f.failureItems, f.failureItemsErr
}

func (f *fakeRegressionRepository) GetRunAgentExecutionContextByID(_ context.Context, _ uuid.UUID) (repository.RunAgentExecutionContext, error) {
	if f.executionContextErr != nil {
		return repository.RunAgentExecutionContext{}, f.executionContextErr
	}
	return f.executionContext, nil
}

func (f *fakeRegressionRepository) GetRunAgentScorecardByRunAgentID(_ context.Context, _ uuid.UUID) (repository.RunAgentScorecard, error) {
	if f.scorecardErr != nil {
		return repository.RunAgentScorecard{}, f.scorecardErr
	}
	return f.scorecard, nil
}

func (f *fakeRegressionRepository) GetEvaluationSpecByID(_ context.Context, _ uuid.UUID) (repository.EvaluationSpecRecord, error) {
	if f.evaluationSpecErr != nil {
		return repository.EvaluationSpecRecord{}, f.evaluationSpecErr
	}
	return f.evaluationSpec, nil
}

func (f *fakeRegressionRepository) PromoteFailure(_ context.Context, params repository.PromoteFailureParams) (repository.PromoteFailureResult, error) {
	if f.promoteErr != nil {
		return repository.PromoteFailureResult{}, f.promoteErr
	}
	f.promoteInput = &params
	return f.promoteResult, nil
}

type fakeRegressionService struct {
	suite           repository.RegressionSuite
	regressionCase  repository.RegressionCase
	promoteResult   PromoteFailureResult
	createSuiteErr  error
	listSuitesErr   error
	getSuiteErr     error
	patchSuiteErr   error
	listCasesErr    error
	patchCaseErr    error
	promoteErr      error
	captureErr      error
	patchSuiteInput *PatchRegressionSuiteInput
	listCasesInput  *ListWorkspaceRegressionCasesInput
	patchCaseInput  *PatchRegressionCaseInput
	promoteInput    *PromoteFailureInput
	captureInput    *CaptureProductionFailureInput
}

func (f *fakeRegressionService) CreateRegressionSuite(_ context.Context, _ Caller, input CreateRegressionSuiteInput) (repository.RegressionSuite, error) {
	if f.createSuiteErr != nil {
		return repository.RegressionSuite{}, f.createSuiteErr
	}
	f.suite.WorkspaceID = input.WorkspaceID
	f.suite.SourceEvalPackID = input.SourceEvalPackID
	f.suite.Name = input.Name
	f.suite.Description = input.Description
	f.suite.DefaultGateSeverity = input.DefaultGateSeverity
	return f.suite, nil
}

func (f *fakeRegressionService) ListRegressionSuites(_ context.Context, _ Caller, input ListRegressionSuitesInput) (ListRegressionSuitesResult, error) {
	if f.listSuitesErr != nil {
		return ListRegressionSuitesResult{}, f.listSuitesErr
	}
	return ListRegressionSuitesResult{
		Items:  []repository.RegressionSuite{f.suite},
		Total:  1,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
}

func (f *fakeRegressionService) GetRegressionSuite(_ context.Context, _ Caller, _ GetRegressionSuiteInput) (repository.RegressionSuite, error) {
	if f.getSuiteErr != nil {
		return repository.RegressionSuite{}, f.getSuiteErr
	}
	return f.suite, nil
}

func (f *fakeRegressionService) PatchRegressionSuite(_ context.Context, _ Caller, input PatchRegressionSuiteInput) (repository.RegressionSuite, error) {
	if f.patchSuiteErr != nil {
		return repository.RegressionSuite{}, f.patchSuiteErr
	}
	f.patchSuiteInput = &input
	if input.Description != nil {
		f.suite.Description = *input.Description
	}
	if input.Status != nil {
		f.suite.Status = *input.Status
	}
	if input.DefaultGateSeverity != nil {
		f.suite.DefaultGateSeverity = *input.DefaultGateSeverity
	}
	return f.suite, nil
}

func (f *fakeRegressionService) ListRegressionCases(_ context.Context, _ Caller, _ ListRegressionCasesInput) ([]repository.RegressionCase, error) {
	if f.listCasesErr != nil {
		return nil, f.listCasesErr
	}
	return []repository.RegressionCase{f.regressionCase}, nil
}

func (f *fakeRegressionService) ListWorkspaceRegressionCases(_ context.Context, _ Caller, input ListWorkspaceRegressionCasesInput) (ListWorkspaceRegressionCasesResult, error) {
	if f.listCasesErr != nil {
		return ListWorkspaceRegressionCasesResult{}, f.listCasesErr
	}
	f.listCasesInput = &input
	return ListWorkspaceRegressionCasesResult{
		Items:  []repository.RegressionCase{f.regressionCase},
		Total:  1,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
}

func (f *fakeRegressionService) PatchRegressionCase(_ context.Context, _ Caller, input PatchRegressionCaseInput) (repository.RegressionCase, error) {
	if f.patchCaseErr != nil {
		return repository.RegressionCase{}, f.patchCaseErr
	}
	f.patchCaseInput = &input
	if input.Title != nil {
		f.regressionCase.Title = *input.Title
	}
	if input.Status != nil {
		f.regressionCase.Status = *input.Status
	}
	if input.Severity != nil {
		f.regressionCase.Severity = *input.Severity
	}
	return f.regressionCase, nil
}

func (f *fakeRegressionService) PromoteFailure(_ context.Context, _ Caller, input PromoteFailureInput) (PromoteFailureResult, error) {
	if f.promoteErr != nil {
		return PromoteFailureResult{}, f.promoteErr
	}
	f.promoteInput = &input
	if f.promoteResult.Case.ID == uuid.Nil {
		f.promoteResult.Case = f.regressionCase
	}
	return f.promoteResult, nil
}

func (f *fakeRegressionService) CaptureProductionFailure(_ context.Context, _ Caller, input CaptureProductionFailureInput) (repository.RegressionCase, error) {
	if f.captureErr != nil {
		return repository.RegressionCase{}, f.captureErr
	}
	f.captureInput = &input
	if f.regressionCase.ID == uuid.Nil {
		f.regressionCase.ID = uuid.New()
	}
	f.regressionCase.WorkspaceID = input.WorkspaceID
	f.regressionCase.SuiteID = input.SuiteID
	f.regressionCase.Title = input.Title
	f.regressionCase.Status = domain.RegressionCaseStatusProposed
	if input.Severity != nil {
		f.regressionCase.Severity = *input.Severity
	}
	if input.PromotionMode != nil {
		f.regressionCase.PromotionMode = *input.PromotionMode
	}
	f.regressionCase.SourceEvalPackVersionID = input.SourceEvalPackVersionID
	f.regressionCase.SourceChallengeInputSetID = cloneUUIDPtr(input.SourceChallengeInputSetID)
	f.regressionCase.SourceChallengeIdentityID = input.SourceChallengeIdentityID
	f.regressionCase.SourceCaseKey = input.SourceCaseKey
	f.regressionCase.SourceItemKey = cloneStringPtr(input.SourceItemKey)
	f.regressionCase.EvidenceTier = input.EvidenceTier
	f.regressionCase.FailureClass = input.FailureClass
	f.regressionCase.FailureSummary = input.FailureSummary
	f.regressionCase.PayloadSnapshot = input.PayloadSnapshot
	f.regressionCase.ExpectedContract = input.ExpectedContract
	f.regressionCase.Metadata = input.Metadata
	return f.regressionCase, nil
}
