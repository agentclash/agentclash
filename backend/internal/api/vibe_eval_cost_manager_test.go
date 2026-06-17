package api

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func estimateCaller(workspaceID uuid.UUID) Caller {
	return Caller{
		UserID:               uuid.New(),
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{workspaceID: {WorkspaceID: workspaceID, Role: "workspace_member"}},
	}
}

func estimateManager(repo *fakeRunCreationRepository) *RunCreationManager {
	return NewRunCreationManager(NewCallerWorkspaceAuthorizer(), repo, &fakeRunWorkflowStarter{}, nil)
}

func managedDeployment(workspaceID uuid.UUID, rate float64) repository.RunnableDeployment {
	return repository.RunnableDeployment{
		ID: uuid.New(), WorkspaceID: workspaceID, ProviderKey: "openai", ProviderModelID: "gpt",
		OutputCostPerMillionTokens: rate, // SourceProviderAccountID nil ⇒ managed
	}
}

func TestEstimateEvalCost_Manager_Success(t *testing.T) {
	ws := uuid.New()
	versionID := uuid.New()
	d := managedDeployment(ws, 3.0)
	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{
			ID: versionID, WorkspaceID: &ws,
			Manifest: []byte(`{"evaluation_spec":{"runtime_limits":{"max_cost_usd":1.5}}}`),
		},
		deployments: []repository.RunnableDeployment{d},
	}
	est, err := estimateManager(repo).EstimateEvalCost(context.Background(), estimateCaller(ws), EstimateEvalCostInput{
		WorkspaceID: ws, ChallengePackVersionID: versionID, AgentDeploymentIDs: []uuid.UUID{d.ID},
	})
	if err != nil {
		t.Fatalf("EstimateEvalCost: %v", err)
	}
	if est.TotalMicros != 1_500_000 || est.Lanes[0].Basis != "max_cost_usd" {
		t.Fatalf("est = %+v, want 1.5M via max_cost_usd", est)
	}
}

func TestEstimateEvalCost_Manager_RejectsDuplicateDeployments(t *testing.T) {
	ws := uuid.New()
	versionID := uuid.New()
	dup := uuid.New()
	repo := &fakeRunCreationRepository{challengePackVersion: repository.RunnableChallengePackVersion{ID: versionID, WorkspaceID: &ws}}
	_, err := estimateManager(repo).EstimateEvalCost(context.Background(), estimateCaller(ws), EstimateEvalCostInput{
		WorkspaceID: ws, ChallengePackVersionID: versionID, AgentDeploymentIDs: []uuid.UUID{dup, dup},
	})
	var verr RunCreationValidationError
	if !errors.As(err, &verr) || verr.Code != "invalid_agent_deployment_ids" {
		t.Fatalf("err = %v, want duplicate rejection", err)
	}
}

func TestEstimateEvalCost_Manager_RejectsPartialDeployments(t *testing.T) {
	ws := uuid.New()
	versionID := uuid.New()
	d := managedDeployment(ws, 3.0)
	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: versionID, WorkspaceID: &ws},
		deployments:          []repository.RunnableDeployment{d}, // only one resolves
	}
	_, err := estimateManager(repo).EstimateEvalCost(context.Background(), estimateCaller(ws), EstimateEvalCostInput{
		WorkspaceID: ws, ChallengePackVersionID: versionID, AgentDeploymentIDs: []uuid.UUID{d.ID, uuid.New()},
	})
	var verr RunCreationValidationError
	if !errors.As(err, &verr) || verr.Code != "invalid_agent_deployment_ids" {
		t.Fatalf("err = %v, want partial (not-visible) rejection", err)
	}
}

func TestEstimateEvalCost_Manager_RejectsVersionNotVisible(t *testing.T) {
	ws := uuid.New()
	otherWS := uuid.New()
	versionID := uuid.New()
	d := managedDeployment(ws, 3.0)
	repo := &fakeRunCreationRepository{
		challengePackVersion: repository.RunnableChallengePackVersion{ID: versionID, WorkspaceID: &otherWS}, // different workspace
		deployments:          []repository.RunnableDeployment{d},
	}
	_, err := estimateManager(repo).EstimateEvalCost(context.Background(), estimateCaller(ws), EstimateEvalCostInput{
		WorkspaceID: ws, ChallengePackVersionID: versionID, AgentDeploymentIDs: []uuid.UUID{d.ID},
	})
	var verr RunCreationValidationError
	if !errors.As(err, &verr) || verr.Code != "invalid_challenge_pack_version_id" {
		t.Fatalf("err = %v, want version-visibility rejection", err)
	}
}
