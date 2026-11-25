package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/api"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func TestRepositoryDatasetInputSetOwnershipTrigger(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	version := seedInputSetDatasetVersion(t, repository.New(db), fixture)
	otherOwner := uuid.New()

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	insertSet := func(key string, owner *uuid.UUID) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO challenge_input_sets (challenge_pack_version_id, input_key, name, input_checksum, generated_at)
			VALUES ($1, $2, 'Dataset inputs', 'checksum', now()) RETURNING id
		`, fixture.challengePackVersionID, key).Scan(&id); err != nil {
			t.Fatalf("insert input set: %v", err)
		}
		if owner != nil {
			if _, err := tx.Exec(ctx, `UPDATE challenge_input_sets SET owner_workspace_id = $1 WHERE id = $2`, owner, id); err != nil {
				t.Fatalf("assign input set owner: %v", err)
			}
		}
		return id
	}
	first := insertSet("first-dataset-set", nil)
	second := insertSet("second-dataset-set", nil)
	alreadyOwned := insertSet("owned-dataset-set", &otherOwner)
	linkID := insertDatasetInputSetLink(t, tx, fixture, version, first)
	checkOwner := func(id, want uuid.UUID) {
		t.Helper()
		var got *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT owner_workspace_id FROM challenge_input_sets WHERE id = $1`, id).Scan(&got); err != nil {
			t.Fatalf("read input set owner: %v", err)
		}
		if got == nil || *got != want {
			t.Errorf("input set %s owner = %v, want %s", id, got, want)
		}
	}
	checkOwner(first, fixture.workspaceID)
	for _, id := range []uuid.UUID{second, alreadyOwned} {
		if _, err := tx.Exec(ctx, `UPDATE dataset_version_input_sets SET challenge_input_set_id = $1 WHERE id = $2`, id, linkID); err != nil {
			t.Fatalf("move link: %v", err)
		}
	}
	checkOwner(second, fixture.workspaceID)
	checkOwner(first, fixture.workspaceID)
	checkOwner(alreadyOwned, otherOwner)
}

func TestRepositoryDatasetInputSetOwnershipBackfill(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	version := seedInputSetDatasetVersion(t, repository.New(db), fixture)
	droppedVersionID := uuid.New()
	keyPrefix := "dataset-" + version.ID.String() + "-"
	description := "Materialized from dataset version " + version.ID.String() + " for challenge first-ticket."

	migration, err := os.ReadFile("../../db/migrations/00101_challenge_input_set_owner.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, down, ok := strings.Cut(string(migration), "-- +goose Down\n")
	if !ok {
		t.Fatal("missing Down migration")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, down); err != nil {
		t.Fatalf("roll back migration: %v", err)
	}

	tests := []struct {
		name, key, description             string
		generated, linked, owned, archived bool
		id                                 uuid.UUID
	}{
		{name: "linked set", key: "linked", linked: true, owned: true},
		{name: "detached set", key: keyPrefix + "detached", description: description, generated: true, owned: true},
		{name: "pack-defined set", key: keyPrefix + "pack", description: description},
		{name: "other generated set", key: keyPrefix + "other", description: "Other inputs", generated: true},
		{name: "orphaned set", key: "dataset-" + droppedVersionID.String() + "-orphan",
			description: "Materialized from dataset version " + droppedVersionID.String() + " for challenge first-ticket.", generated: true, archived: true},
	}
	var orphanID uuid.UUID
	for i := range tests {
		tt := &tests[i]
		if err := tx.QueryRow(ctx, `
			INSERT INTO challenge_input_sets (challenge_pack_version_id, input_key, name, description, input_checksum, generated_at)
			VALUES ($1, $2, $3, $4, 'checksum', CASE WHEN $5 THEN now() END) RETURNING id
		`, fixture.challengePackVersionID, tt.key, tt.name, tt.description, tt.generated).Scan(&tt.id); err != nil {
			t.Fatalf("insert %s: %v", tt.name, err)
		}
		if tt.linked {
			insertDatasetInputSetLink(t, tx, fixture, version, tt.id)
		}
		if tt.archived {
			orphanID = tt.id
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE runs SET challenge_input_set_id = $1 WHERE id = $2`, orphanID, fixture.runID); err != nil {
		t.Fatalf("attach orphaned set to existing run: %v", err)
	}
	if _, err := tx.Exec(ctx, up); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var owner *uuid.UUID
			var archived bool
			if err := tx.QueryRow(ctx, `SELECT owner_workspace_id, archived_at IS NOT NULL FROM challenge_input_sets WHERE id = $1`, tt.id).Scan(&owner, &archived); err != nil {
				t.Fatal(err)
			}
			if (owner != nil) != tt.owned || (owner != nil && *owner != fixture.workspaceID) || archived != tt.archived {
				t.Fatalf("owner = %v, archived = %t, want dataset owner = %t, archived = %t", owner, archived, tt.owned, tt.archived)
			}
		})
	}
	execution, err := sqlc.New(tx).GetRunAgentExecutionContextByID(ctx, sqlc.GetRunAgentExecutionContextByIDParams{ID: fixture.primaryRunAgentID})
	if err != nil {
		t.Fatal(err)
	}
	if execution.ChallengeInputSetID == nil || *execution.ChallengeInputSetID != orphanID {
		t.Fatalf("existing run input set = %v, want %s", execution.ChallengeInputSetID, orphanID)
	}
}

func seedInputSetDatasetVersion(t *testing.T, repo *repository.Repository, fixture testFixture) repository.DatasetVersion {
	t.Helper()
	dataset, err := repo.CreateDataset(t.Context(), repository.CreateDatasetParams{
		WorkspaceID: fixture.workspaceID, Slug: "support-dataset", Name: "Support dataset", CreatedBy: fixture.userID,
	})
	if err != nil {
		t.Fatalf("create dataset: %v", err)
	}
	version, err := repo.CreateDatasetVersion(t.Context(), repository.CreateDatasetVersionParams{DatasetID: dataset.ID, Actor: fixture.userID})
	if err != nil {
		t.Fatalf("create dataset version: %v", err)
	}
	return version
}

func insertDatasetInputSetLink(t *testing.T, tx pgx.Tx, fixture testFixture, version repository.DatasetVersion, inputSetID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := tx.QueryRow(t.Context(), `
		INSERT INTO dataset_version_input_sets (
			dataset_id, dataset_version_id, challenge_pack_version_id, challenge_identity_id,
			challenge_key, challenge_input_set_id, input_key, input_checksum
		)
		SELECT $1, $2, challenge_pack_version_id, $3, 'first-ticket', id, input_key, input_checksum
		FROM challenge_input_sets WHERE id = $4
		RETURNING id
	`, version.DatasetID, version.ID, fixture.firstChallengeIdentityID, inputSetID).Scan(&id); err != nil {
		t.Fatalf("insert dataset input set link: %v", err)
	}
	return id
}

func TestRepositoryDatasetInputSetVisibleOnlyToOwningWorkspace(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	otherWorkspaceID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO workspaces (id, organization_id, name, slug)
		VALUES ($1, $2, 'Other Workspace', 'other-workspace')
	`, otherWorkspaceID, fixture.organizationID); err != nil {
		t.Fatalf("insert other workspace: %v", err)
	}
	dataset, err := repo.CreateDataset(ctx, repository.CreateDatasetParams{
		WorkspaceID: fixture.workspaceID,
		Slug:        "support-dataset",
		Name:        "Support dataset",
		CreatedBy:   fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateDataset: %v", err)
	}
	if _, err := repo.UpsertDatasetExample(ctx, repository.UpsertDatasetExampleParams{
		DatasetID: dataset.ID,
		Input:     json.RawMessage(`{"message":"Check my ticket"}`),
		Status:    domain.DatasetExampleStatusActive,
		Source:    domain.DatasetExampleSourceImport,
		Actor:     fixture.userID,
	}); err != nil {
		t.Fatalf("UpsertDatasetExample: %v", err)
	}
	version, err := repo.CreateDatasetVersion(ctx, repository.CreateDatasetVersionParams{
		DatasetID: dataset.ID,
		Actor:     fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion: %v", err)
	}
	params := repository.MaterializeDatasetVersionInputSetParams{
		DatasetID:              dataset.ID,
		DatasetVersionID:       version.ID,
		ChallengePackVersionID: fixture.challengePackVersionID,
		ChallengeKey:           "first-ticket",
	}
	first, err := repo.MaterializeDatasetVersionInputSet(ctx, params)
	if err != nil {
		t.Fatalf("materialize first input set: %v", err)
	}
	withMapping := params
	withMapping.Mapping = json.RawMessage(`{"input":"message"}`)
	second, err := repo.MaterializeDatasetVersionInputSet(ctx, withMapping)
	if err != nil {
		t.Fatalf("materialize second input set: %v", err)
	}
	if first.ID != second.ID || first.ChallengeInputSetID == second.ChallengeInputSetID {
		t.Fatalf("expected the link to move between input sets, got %+v and %+v", first, second)
	}
	retried, err := repo.MaterializeDatasetVersionInputSet(ctx, withMapping)
	if err != nil || retried.ChallengeInputSetID != second.ChallengeInputSetID {
		t.Fatalf("retry materialization = %s, %v, want %s", retried.ChallengeInputSetID, err, second.ChallengeInputSetID)
	}

	for _, tt := range []struct {
		name        string
		workspaceID uuid.UUID
		want        []uuid.UUID
	}{
		{name: "owner", workspaceID: fixture.workspaceID, want: []uuid.UUID{fixture.challengeInputSetID, first.ChallengeInputSetID, second.ChallengeInputSetID}},
		{name: "other workspace", workspaceID: otherWorkspaceID, want: []uuid.UUID{fixture.challengeInputSetID}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			inputSets, err := repo.ListChallengeInputSetsVisibleToWorkspace(ctx, fixture.challengePackVersionID, tt.workspaceID)
			if err != nil {
				t.Fatalf("ListChallengeInputSetsVisibleToWorkspace: %v", err)
			}
			if len(inputSets) != len(tt.want) {
				t.Errorf("listed input sets = %+v, want IDs %v", inputSets, tt.want)
			} else {
				for i, id := range tt.want {
					if inputSets[i].ID != id {
						t.Errorf("listed input set %d = %s, want %s", i, inputSets[i].ID, id)
					}
				}
			}
			for _, check := range []struct {
				id   uuid.UUID
				want bool
			}{
				{id: fixture.challengeInputSetID, want: true},
				{id: first.ChallengeInputSetID, want: tt.workspaceID == fixture.workspaceID},
				{id: second.ChallengeInputSetID, want: tt.workspaceID == fixture.workspaceID},
			} {
				got, err := repo.ChallengeInputSetVisibleToWorkspace(ctx, check.id, tt.workspaceID)
				if err != nil || got != check.want {
					t.Errorf("ChallengeInputSetVisibleToWorkspace(%s) = %t, %v, want %t", check.id, got, err, check.want)
				}
			}
		})
	}

	t.Run("public snapshot", func(t *testing.T) {
		snapshot, err := repo.GetPublicChallengePackVersionSnapshot(ctx, fixture.challengePackVersionID)
		if err != nil {
			t.Fatalf("GetPublicChallengePackVersionSnapshot: %v", err)
		}
		if len(snapshot.InputSets) != 1 || snapshot.InputSets[0].ID != fixture.challengeInputSetID {
			t.Fatalf("public input sets = %+v, want only %s", snapshot.InputSets, fixture.challengeInputSetID)
		}
	})

	t.Run("default input sets", func(t *testing.T) {
		want := []uuid.UUID{fixture.challengeInputSetID}
		got, err := repo.ListDefaultChallengeInputSetIDsByVersionID(ctx, fixture.challengePackVersionID)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("default input sets = %v, %v, want %v", got, err, want)
		}

		// A dataset input set keeps its owner when its dataset link is removed.
		// It must not become a default input set then.
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `DELETE FROM dataset_version_input_sets WHERE dataset_id = $1`, dataset.ID); err != nil {
			t.Fatalf("remove dataset links: %v", err)
		}
		got, err = sqlc.New(tx).ListDefaultChallengeInputSetIDsByVersionID(ctx, sqlc.ListDefaultChallengeInputSetIDsByVersionIDParams{
			ChallengePackVersionID: fixture.challengePackVersionID,
		})
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("default input sets without dataset links = %v, %v, want %v", got, err, want)
		}
	})

	for _, id := range []uuid.UUID{first.ChallengeInputSetID, second.ChallengeInputSetID} {
		var owner *uuid.UUID
		if err := db.QueryRow(ctx, `SELECT owner_workspace_id FROM challenge_input_sets WHERE id = $1`, id).Scan(&owner); err != nil {
			t.Fatalf("load owner of input set %s: %v", id, err)
		}
		if owner == nil || *owner != fixture.workspaceID {
			t.Errorf("input set %s owner = %v, want %s", id, owner, fixture.workspaceID)
		}
	}

	if _, err := db.Exec(ctx, `UPDATE challenge_input_sets SET input_key = 'retired-input-key' WHERE id = $1`, first.ChallengeInputSetID); err != nil {
		t.Fatalf("move first input set key: %v", err)
	}
	for _, tt := range []struct {
		name  string
		owner *uuid.UUID
	}{
		{name: "pack-defined key collision"},
		{name: "other workspace key collision", owner: &otherWorkspaceID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.Exec(ctx, `
				UPDATE challenge_input_sets SET input_key = $1, owner_workspace_id = $2 WHERE id = $3
			`, first.InputKey, tt.owner, fixture.challengeInputSetID); err != nil {
				t.Fatalf("set up input key collision: %v", err)
			}
			if _, err := repo.MaterializeDatasetVersionInputSet(ctx, params); !errors.Is(err, repository.ErrInvalidDatasetEvalInput) {
				t.Fatalf("materialize colliding input set = %v, want ErrInvalidDatasetEvalInput", err)
			}
			var sameOwner bool
			var name, checksum string
			if err := db.QueryRow(ctx, `
				SELECT owner_workspace_id IS NOT DISTINCT FROM $2::uuid, name, input_checksum
				FROM challenge_input_sets WHERE id = $1
			`, fixture.challengeInputSetID, tt.owner).Scan(&sameOwner, &name, &checksum); err != nil {
				t.Fatalf("load colliding input set: %v", err)
			}
			if !sameOwner || name != "Default Inputs" || checksum != "input-checksum" {
				t.Fatalf("colliding input set changed: same owner %t, name %q, checksum %q", sameOwner, name, checksum)
			}
		})
	}
}
