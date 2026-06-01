package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type DatasetRegressionSyncRepository interface {
	GetDatasetRegressionSuiteLink(context.Context, uuid.UUID) (repository.DatasetRegressionSuiteLink, error)
	SyncDatasetRegressionSuite(context.Context, repository.SyncDatasetRegressionSuiteParams) (repository.SyncDatasetRegressionSuiteResult, error)
}

type SyncDatasetRegressionSuiteInput struct {
	WorkspaceID            uuid.UUID
	DatasetID              uuid.UUID
	VersionID              uuid.UUID
	ChallengePackVersionID uuid.UUID
	ChallengeKey           string
	RegressionSuiteID      *uuid.UUID
	SuiteName              *string
}

func (m *DatasetManager) GetDatasetRegressionSuiteLink(ctx context.Context, caller Caller, input GetDatasetInput) (repository.DatasetRegressionSuiteLink, error) {
	if _, err := m.GetDataset(ctx, caller, input); err != nil {
		return repository.DatasetRegressionSuiteLink{}, err
	}
	syncRepo, ok := m.repo.(DatasetRegressionSyncRepository)
	if !ok {
		return repository.DatasetRegressionSuiteLink{}, errors.New("dataset regression sync repository not configured")
	}
	return syncRepo.GetDatasetRegressionSuiteLink(ctx, input.DatasetID)
}

func (m *DatasetManager) SyncDatasetRegressionSuite(ctx context.Context, caller Caller, input SyncDatasetRegressionSuiteInput) (repository.SyncDatasetRegressionSuiteResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageRegressions); err != nil {
		return repository.SyncDatasetRegressionSuiteResult{}, err
	}
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.SyncDatasetRegressionSuiteResult{}, err
	}
	syncRepo, ok := m.repo.(DatasetRegressionSyncRepository)
	if !ok {
		return repository.SyncDatasetRegressionSuiteResult{}, errors.New("dataset regression sync repository not configured")
	}
	return syncRepo.SyncDatasetRegressionSuite(ctx, repository.SyncDatasetRegressionSuiteParams{
		DatasetID:              input.DatasetID,
		WorkspaceID:            input.WorkspaceID,
		VersionID:              input.VersionID,
		ChallengePackVersionID: input.ChallengePackVersionID,
		ChallengeKey:           input.ChallengeKey,
		RegressionSuiteID:      input.RegressionSuiteID,
		SuiteName:              input.SuiteName,
		Actor:                  caller.UserID,
	})
}

func getDatasetRegressionSuiteLinkHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		link, err := service.GetDatasetRegressionSuiteLink(r.Context(), caller, GetDatasetInput{
			WorkspaceID: workspaceID,
			DatasetID:   datasetID,
		})
		if err != nil {
			handleDatasetRegressionSyncError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, link)
	}
}

func syncDatasetRegressionSuiteHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		var body struct {
			VersionID              uuid.UUID  `json:"version_id"`
			ChallengePackVersionID uuid.UUID  `json:"challenge_pack_version_id"`
			ChallengeKey           string     `json:"challenge_key"`
			RegressionSuiteID      *uuid.UUID `json:"regression_suite_id,omitempty"`
			SuiteName              *string    `json:"suite_name,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if body.VersionID == uuid.Nil || body.ChallengePackVersionID == uuid.Nil || body.ChallengeKey == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "version_id, challenge_pack_version_id, and challenge_key are required")
			return
		}
		result, err := service.SyncDatasetRegressionSuite(r.Context(), caller, SyncDatasetRegressionSuiteInput{
			WorkspaceID:            workspaceID,
			DatasetID:              datasetID,
			VersionID:              body.VersionID,
			ChallengePackVersionID: body.ChallengePackVersionID,
			ChallengeKey:           body.ChallengeKey,
			RegressionSuiteID:      body.RegressionSuiteID,
			SuiteName:              body.SuiteName,
		})
		if err != nil {
			handleDatasetRegressionSyncError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleDatasetRegressionSyncError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, repository.ErrDatasetRegressionSuiteLinkNotFound):
		writeError(w, http.StatusNotFound, "dataset_regression_suite_link_not_found", "dataset is not linked to a regression suite")
	case errors.Is(err, repository.ErrRegressionSuitePackMismatch):
		writeError(w, http.StatusBadRequest, "regression_suite_pack_mismatch", "regression suite must belong to the same challenge pack")
	case errors.Is(err, repository.ErrRegressionSuiteNameConflict):
		writeError(w, http.StatusConflict, "regression_suite_name_conflict", "an active regression suite with this name already exists in the workspace")
	default:
		handleDatasetGateError(w, logger, err)
	}
}
