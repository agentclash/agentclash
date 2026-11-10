package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRepositoryUpdateDatasetGenerationJobStatusKeepsTerminalJobs(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	dataset, err := repo.CreateDataset(ctx, repository.CreateDatasetParams{
		WorkspaceID: fixture.workspaceID,
		Slug:        "generation-guard-" + uuid.NewString()[:8],
		Name:        "Generation guard",
		InputSchema: json.RawMessage(`{}`),
		CreatedBy:   fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateDataset returned error: %v", err)
	}
	newJob := func(t *testing.T) repository.DatasetGenerationJob {
		t.Helper()
		job, err := repo.CreateDatasetGenerationJob(ctx, repository.CreateDatasetGenerationJobParams{
			DatasetID:   dataset.ID,
			WorkspaceID: fixture.workspaceID,
			Strategy:    "self_instruct",
			Config:      json.RawMessage(`{}`),
			TargetCount: 1,
			Actor:       fixture.userID,
			QueuedAt:    time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("CreateDatasetGenerationJob returned error: %v", err)
		}
		return job
	}
	update := func(jobID uuid.UUID, status repository.DatasetGenerationStatus) (repository.DatasetGenerationJob, error) {
		return repo.UpdateDatasetGenerationJobStatus(ctx, repository.UpdateDatasetGenerationJobStatusParams{ID: jobID, Status: status})
	}

	t.Run("non-terminal transitions and same-state writes", func(t *testing.T) {
		job := newJob(t)
		for _, status := range []repository.DatasetGenerationStatus{
			repository.DatasetGenerationStatusRunning,
			repository.DatasetGenerationStatusRunning,
			repository.DatasetGenerationStatusCompleted,
			repository.DatasetGenerationStatusCompleted,
		} {
			updated, err := update(job.ID, status)
			if err != nil {
				t.Fatalf("update to %s returned error: %v", status, err)
			}
			if updated.Status != status {
				t.Fatalf("status = %s, want %s", updated.Status, status)
			}
		}
	})

	for _, terminal := range []repository.DatasetGenerationStatus{
		repository.DatasetGenerationStatusCompleted,
		repository.DatasetGenerationStatusFailed,
	} {
		t.Run("terminal "+string(terminal)+" stays terminal", func(t *testing.T) {
			job := newJob(t)
			if _, err := update(job.ID, terminal); err != nil {
				t.Fatalf("update to %s returned error: %v", terminal, err)
			}
			for _, next := range []repository.DatasetGenerationStatus{
				repository.DatasetGenerationStatusQueued,
				repository.DatasetGenerationStatusRunning,
			} {
				_, err := update(job.ID, next)
				if !errors.Is(err, repository.ErrInvalidTransition) {
					t.Fatalf("update %s -> %s error = %v, want ErrInvalidTransition", terminal, next, err)
				}
			}
			current, err := repo.GetDatasetGenerationJobByID(ctx, job.ID)
			if err != nil {
				t.Fatalf("GetDatasetGenerationJobByID returned error: %v", err)
			}
			if current.Status != terminal {
				t.Fatalf("status = %s, want %s", current.Status, terminal)
			}
		})
	}

	t.Run("missing job", func(t *testing.T) {
		_, err := update(uuid.New(), repository.DatasetGenerationStatusRunning)
		if !errors.Is(err, repository.ErrDatasetGenerationJobNotFound) {
			t.Fatalf("error = %v, want ErrDatasetGenerationJobNotFound", err)
		}
	})
}
