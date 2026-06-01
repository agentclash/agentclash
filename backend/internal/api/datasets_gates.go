package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	datasetgate "github.com/agentclash/agentclash/backend/internal/datasets/gate"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type DatasetGateRepository interface {
	CreateDatasetBaseline(context.Context, repository.CreateDatasetBaselineParams) (repository.DatasetBaseline, error)
	ListDatasetBaselines(context.Context, repository.ListDatasetBaselinesParams) (repository.ListDatasetBaselinesResult, error)
	GetDatasetBaselineByID(context.Context, uuid.UUID) (repository.DatasetBaseline, error)
	ListDatasetEvalOutcomesForRun(context.Context, uuid.UUID, *uuid.UUID) ([]datasetgate.ExampleOutcome, error)
	GetRunByID(context.Context, uuid.UUID) (domain.Run, error)
	GetDatasetEvalRunByRunID(context.Context, uuid.UUID) (repository.DatasetEvalRun, error)
}

type DatasetGateService interface {
	CreateDatasetBaseline(context.Context, Caller, CreateDatasetBaselineInput) (repository.DatasetBaseline, error)
	ListDatasetBaselines(context.Context, Caller, ListDatasetBaselinesInput) (repository.ListDatasetBaselinesResult, error)
	EvaluateDatasetGate(context.Context, Caller, EvaluateDatasetGateInput) (EvaluateDatasetGateResult, error)
}

type CreateDatasetBaselineInput struct {
	WorkspaceID       uuid.UUID
	DatasetID         uuid.UUID
	RunID             uuid.UUID
	AgentDeploymentID *uuid.UUID
	Label             *string
}

type ListDatasetBaselinesInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	Limit       int32
	Offset      int32
}

type EvaluateDatasetGateInput struct {
	WorkspaceID       uuid.UUID
	DatasetID         uuid.UUID
	BaselineID        uuid.UUID
	RunID             uuid.UUID
	AgentDeploymentID *uuid.UUID
	MinPassRate       *float64
	MaxRegressions    *int
}

type EvaluateDatasetGateResult struct {
	Baseline repository.DatasetBaseline `json:"baseline"`
	CandidateRunID uuid.UUID            `json:"candidate_run_id"`
	Gate       datasetgate.Result       `json:"gate"`
}

func (m *DatasetManager) CreateDatasetBaseline(ctx context.Context, caller Caller, input CreateDatasetBaselineInput) (repository.DatasetBaseline, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.DatasetBaseline{}, err
	}
	gateRepo, ok := m.repo.(DatasetGateRepository)
	if !ok {
		return repository.DatasetBaseline{}, errors.New("dataset gate repository not configured")
	}
	run, err := gateRepo.GetRunByID(ctx, input.RunID)
	if err != nil {
		return repository.DatasetBaseline{}, err
	}
	if run.WorkspaceID != input.WorkspaceID {
		return repository.DatasetBaseline{}, ErrForbidden
	}
	return gateRepo.CreateDatasetBaseline(ctx, repository.CreateDatasetBaselineParams{
		DatasetID:         input.DatasetID,
		RunID:             input.RunID,
		AgentDeploymentID: input.AgentDeploymentID,
		Label:             input.Label,
		Actor:             caller.UserID,
	})
}

func (m *DatasetManager) ListDatasetBaselines(ctx context.Context, caller Caller, input ListDatasetBaselinesInput) (repository.ListDatasetBaselinesResult, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.ListDatasetBaselinesResult{}, err
	}
	gateRepo, ok := m.repo.(DatasetGateRepository)
	if !ok {
		return repository.ListDatasetBaselinesResult{}, errors.New("dataset gate repository not configured")
	}
	return gateRepo.ListDatasetBaselines(ctx, repository.ListDatasetBaselinesParams{
		DatasetID: input.DatasetID,
		Limit:     input.Limit,
		Offset:    input.Offset,
	})
}

func (m *DatasetManager) EvaluateDatasetGate(ctx context.Context, caller Caller, input EvaluateDatasetGateInput) (EvaluateDatasetGateResult, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return EvaluateDatasetGateResult{}, err
	}
	gateRepo, ok := m.repo.(DatasetGateRepository)
	if !ok {
		return EvaluateDatasetGateResult{}, errors.New("dataset gate repository not configured")
	}
	baseline, err := gateRepo.GetDatasetBaselineByID(ctx, input.BaselineID)
	if err != nil {
		return EvaluateDatasetGateResult{}, err
	}
	if baseline.DatasetID != input.DatasetID {
		return EvaluateDatasetGateResult{}, repository.ErrDatasetBaselineNotFound
	}
	run, err := gateRepo.GetRunByID(ctx, input.RunID)
	if err != nil {
		return EvaluateDatasetGateResult{}, err
	}
	if run.WorkspaceID != input.WorkspaceID {
		return EvaluateDatasetGateResult{}, ErrForbidden
	}
	evalRun, err := gateRepo.GetDatasetEvalRunByRunID(ctx, input.RunID)
	if err != nil {
		return EvaluateDatasetGateResult{}, err
	}
	if evalRun.DatasetID != input.DatasetID {
		return EvaluateDatasetGateResult{}, ErrForbidden
	}
	deploymentID := input.AgentDeploymentID
	if deploymentID == nil {
		deploymentID = baseline.AgentDeploymentID
	}
	baselineOutcomes, err := repository.DecodeDatasetBaselineOutcomes(baseline.ExampleOutcomes)
	if err != nil {
		return EvaluateDatasetGateResult{}, err
	}
	candidateOutcomes, err := gateRepo.ListDatasetEvalOutcomesForRun(ctx, input.RunID, deploymentID)
	if err != nil {
		return EvaluateDatasetGateResult{}, err
	}
	gateResult := datasetgate.Evaluate(baselineOutcomes, candidateOutcomes, datasetgate.Thresholds{
		MinPassRate:    input.MinPassRate,
		MaxRegressions: input.MaxRegressions,
	})
	return EvaluateDatasetGateResult{
		Baseline:       baseline,
		CandidateRunID: input.RunID,
		Gate:           gateResult,
	}, nil
}

func createDatasetBaselineHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		var body struct {
			RunID             uuid.UUID  `json:"run_id"`
			AgentDeploymentID *uuid.UUID `json:"agent_deployment_id,omitempty"`
			Label             *string    `json:"label,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if body.RunID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "invalid_run_id", "run_id is required")
			return
		}
		result, err := service.CreateDatasetBaseline(r.Context(), caller, CreateDatasetBaselineInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, RunID: body.RunID,
			AgentDeploymentID: body.AgentDeploymentID, Label: body.Label,
		})
		if err != nil {
			handleDatasetGateError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func listDatasetBaselinesHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		limit, offset, ok := paginationFromRequest(w, r)
		if !ok {
			return
		}
		result, err := service.ListDatasetBaselines(r.Context(), caller, ListDatasetBaselinesInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, Limit: limit, Offset: offset,
		})
		if err != nil {
			handleDatasetGateError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func evaluateDatasetGateHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		var body struct {
			BaselineID        uuid.UUID  `json:"baseline_id"`
			RunID             uuid.UUID  `json:"run_id"`
			AgentDeploymentID *uuid.UUID `json:"agent_deployment_id,omitempty"`
			MinPassRate       *float64   `json:"min_pass_rate,omitempty"`
			MaxRegressions    *int       `json:"max_regressions,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if body.BaselineID == uuid.Nil || body.RunID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "baseline_id and run_id are required")
			return
		}
		result, err := service.EvaluateDatasetGate(r.Context(), caller, EvaluateDatasetGateInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, BaselineID: body.BaselineID, RunID: body.RunID,
			AgentDeploymentID: body.AgentDeploymentID, MinPassRate: body.MinPassRate, MaxRegressions: body.MaxRegressions,
		})
		if err != nil {
			handleDatasetGateError(w, logger, err)
			return
		}
		status := http.StatusOK
		if !result.Gate.Pass {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, result)
	}
}

func handleDatasetGateError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, repository.ErrDatasetBaselineNotFound):
		writeError(w, http.StatusNotFound, "dataset_baseline_not_found", "dataset baseline not found")
	case errors.Is(err, repository.ErrRunNotFound):
		writeError(w, http.StatusNotFound, "run_not_found", "run not found")
	default:
		handleDatasetError(w, logger, err)
	}
}
