package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/api"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
)

func TestRunCreationWithDatasetInputSet(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	if _, err := db.Exec(ctx, "UPDATE workspaces SET public_packs = true WHERE id = $1", fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	dataset, err := repo.CreateDataset(ctx, repository.CreateDatasetParams{
		WorkspaceID: fixture.workspaceID, Slug: "eval-dataset", Name: "Eval dataset", CreatedBy: fixture.userID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertDatasetExample(ctx, repository.UpsertDatasetExampleParams{
		DatasetID: dataset.ID, Input: []byte(`{"question":"Example question"}`),
		Status: domain.DatasetExampleStatusActive, Source: domain.DatasetExampleSourceManual, Actor: fixture.userID,
	}); err != nil {
		t.Fatal(err)
	}
	version, err := repo.CreateDatasetVersion(ctx, repository.CreateDatasetVersionParams{DatasetID: dataset.ID, Actor: fixture.userID})
	if err != nil {
		t.Fatal(err)
	}
	materialized, err := repo.MaterializeDatasetVersionInputSet(ctx, repository.MaterializeDatasetVersionInputSetParams{
		DatasetID: dataset.ID, DatasetVersionID: version.ID,
		ChallengePackVersionID: fixture.challengePackVersionID, ChallengeKey: "first-ticket",
	})
	if err != nil {
		t.Fatal(err)
	}

	manager := api.NewRunCreationManager(api.NewCallerWorkspaceAuthorizer(), repo, datasetEvalWorkflowStarter{}, nil)
	caller := api.Caller{
		UserID: fixture.userID,
		WorkspaceMemberships: map[uuid.UUID]api.WorkspaceMembership{
			fixture.workspaceID: {WorkspaceID: fixture.workspaceID, Role: "workspace_admin"},
		},
	}
	input := api.CreateRunInput{
		WorkspaceID:            fixture.workspaceID,
		ChallengePackVersionID: fixture.challengePackVersionID,
		AgentDeploymentIDs:     []uuid.UUID{fixture.agentDeploymentID},
	}
	for _, tc := range []struct {
		name       string
		explicitID *uuid.UUID
		wantID     uuid.UUID
	}{
		{name: "auto-selects normal input set", wantID: fixture.challengeInputSetID},
		{name: "accepts explicit dataset input set", explicitID: &materialized.ChallengeInputSetID, wantID: materialized.ChallengeInputSetID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInput := input
			runInput.ChallengeInputSetID = tc.explicitID
			result, err := manager.CreateRun(ctx, caller, runInput)
			if err != nil {
				t.Fatal(err)
			}
			got := result.Run.ChallengeInputSetID
			if got == nil || *got != tc.wantID {
				t.Fatalf("run input set = %v, want %s", got, tc.wantID)
			}
		})
	}

	t.Run("rejects two normal input sets", func(t *testing.T) {
		if _, err := db.Exec(ctx, `
			INSERT INTO challenge_input_sets (challenge_pack_version_id, input_key, name, input_checksum)
			VALUES ($1, 'alternate', 'Alternate inputs', 'alternate-checksum')
		`, fixture.challengePackVersionID); err != nil {
			t.Fatal(err)
		}
		_, err := manager.CreateRun(ctx, caller, input)
		var validationErr api.RunCreationValidationError
		if !errors.As(err, &validationErr) || validationErr.Code != "missing_challenge_input_set_id" {
			t.Fatalf("CreateRun error = %v, want missing_challenge_input_set_id", err)
		}
	})
}

type datasetEvalWorkflowStarter struct{}

func (datasetEvalWorkflowStarter) StartRunWorkflow(context.Context, uuid.UUID) error { return nil }
