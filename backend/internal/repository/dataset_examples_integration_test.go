package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
)

func TestRepositoryUpsertDatasetExampleKeepsArchivedOrMutedOnDefaultStatus(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	dataset, err := repo.CreateDataset(ctx, repository.CreateDatasetParams{
		WorkspaceID: fixture.workspaceID,
		Slug:        "example-status-" + uuid.NewString()[:8],
		Name:        "Example status",
		InputSchema: json.RawMessage(`{}`),
		CreatedBy:   fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateDataset returned error: %v", err)
	}

	upsert := func(t *testing.T, externalID string, status domain.DatasetExampleStatus, explicit bool, input string) repository.DatasetExample {
		t.Helper()
		example, err := repo.UpsertDatasetExample(ctx, repository.UpsertDatasetExampleParams{
			DatasetID:      dataset.ID,
			ExternalID:     &externalID,
			Input:          json.RawMessage(input),
			Metadata:       json.RawMessage(`{}`),
			Status:         status,
			StatusExplicit: explicit,
			Source:         domain.DatasetExampleSourceImport,
			Actor:          fixture.userID,
		})
		if err != nil {
			t.Fatalf("UpsertDatasetExample(%s) returned error: %v", externalID, err)
		}
		return example
	}
	setStatus := func(t *testing.T, id uuid.UUID, status domain.DatasetExampleStatus) {
		t.Helper()
		if _, err := repo.PatchDatasetExample(ctx, repository.PatchDatasetExampleParams{ID: id, Status: &status, Actor: fixture.userID}); err != nil {
			t.Fatalf("PatchDatasetExample returned error: %v", err)
		}
	}

	for _, kept := range []domain.DatasetExampleStatus{domain.DatasetExampleStatusArchived, domain.DatasetExampleStatusMuted} {
		externalID := "kept-" + string(kept)
		created := upsert(t, externalID, domain.DatasetExampleStatusActive, false, `{"q":"v1"}`)
		if created.Status != domain.DatasetExampleStatusActive {
			t.Fatalf("new example status = %q, want active", created.Status)
		}
		setStatus(t, created.ID, kept)

		// A re-import or promotion only carries the default status.
		reimported := upsert(t, externalID, domain.DatasetExampleStatusActive, false, `{"q":"v2"}`)
		if reimported.Status != kept {
			t.Fatalf("re-imported %s example status = %q, want %s", kept, reimported.Status, kept)
		}
		if string(reimported.Input) != `{"q": "v2"}` && string(reimported.Input) != `{"q":"v2"}` {
			t.Fatalf("re-imported input = %s, want the new input", reimported.Input)
		}

		// A status the caller asked for still applies.
		revived := upsert(t, externalID, domain.DatasetExampleStatusActive, true, `{"q":"v3"}`)
		if revived.Status != domain.DatasetExampleStatusActive {
			t.Fatalf("explicit active upsert status = %q, want active", revived.Status)
		}
	}

	// An explicit status can also archive or mute an active example.
	active := upsert(t, "explicit-mute", domain.DatasetExampleStatusActive, false, `{"q":"v1"}`)
	muted := upsert(t, "explicit-mute", domain.DatasetExampleStatusMuted, true, `{"q":"v2"}`)
	if active.Status != domain.DatasetExampleStatusActive || muted.Status != domain.DatasetExampleStatusMuted {
		t.Fatalf("explicit mute statuses = %q then %q, want active then muted", active.Status, muted.Status)
	}
}
