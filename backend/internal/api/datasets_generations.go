package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	datasetgeneration "github.com/agentclash/agentclash/backend/internal/datasets/generation"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DatasetGenerationRepository interface {
	CreateDatasetGenerationJob(context.Context, repository.CreateDatasetGenerationJobParams) (repository.DatasetGenerationJob, error)
	GetDatasetGenerationJobByID(context.Context, uuid.UUID) (repository.DatasetGenerationJob, error)
	GetProviderAccountByID(context.Context, uuid.UUID) (repository.ProviderAccountRow, error)
	ListDatasetExamplesByDatasetID(context.Context, repository.ListDatasetExamplesParams) ([]repository.DatasetExample, error)
}

type DatasetGenerationWorkflowStarter interface {
	StartSyntheticDatasetGenerationWorkflow(ctx context.Context, jobID uuid.UUID) error
}

type DatasetGenerationWorkflowStartError struct {
	Job   repository.DatasetGenerationJob
	Cause error
}

func (e DatasetGenerationWorkflowStartError) Error() string {
	return "failed to start dataset generation workflow: " + e.Cause.Error()
}

func (e DatasetGenerationWorkflowStartError) Unwrap() error { return e.Cause }

type StartDatasetGenerationInput struct {
	WorkspaceID             uuid.UUID
	DatasetID               uuid.UUID
	Strategy                string
	TargetCount             int32
	ProviderAccountID       uuid.UUID
	Model                   string
	SeedsTag                string
	CreateVersion           bool
	VersionLabel            string
	JudgeProviderAccountID  *uuid.UUID
	JudgeModel              string
	MaxRoundsPerExample     int
	AcceptanceMode          string
	MinGap                  *float64
	MaxWeakScore            *float64
	MinStrongScore          *float64
	SolverMode              string
	WeakProviderAccountID   *uuid.UUID
	WeakModel               string
	StrongProviderAccountID *uuid.UUID
	StrongModel             string
	WeakRollouts            int
	StrongRollouts          int
	WeakDeploymentID        *uuid.UUID
	StrongDeploymentID      *uuid.UUID
	ChallengePackVersionID  *uuid.UUID
	ChallengeKey            string
	FieldMapping            json.RawMessage
}

type GetDatasetGenerationJobInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	JobID       uuid.UUID
}

func (m *DatasetManager) WithGenerationWorkflowStarter(starter DatasetGenerationWorkflowStarter) *DatasetManager {
	m.generationWorkflowStarter = starter
	return m
}

func (m *DatasetManager) StartDatasetGeneration(ctx context.Context, caller Caller, input StartDatasetGenerationInput) (repository.DatasetGenerationJob, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageDatasets); err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	if m.generationWorkflowStarter == nil {
		return repository.DatasetGenerationJob{}, errors.New("dataset generation workflow starter is not configured")
	}
	genRepo, ok := m.repo.(DatasetGenerationRepository)
	if !ok {
		return repository.DatasetGenerationJob{}, errors.New("dataset generation repository not configured")
	}

	strategy, err := datasetgeneration.ParseStrategy(input.Strategy)
	if err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	if input.TargetCount <= 0 || input.TargetCount > 100 {
		return repository.DatasetGenerationJob{}, errors.New("target_count must be between 1 and 100")
	}
	providerAccount, err := genRepo.GetProviderAccountByID(ctx, input.ProviderAccountID)
	if err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	if providerAccount.WorkspaceID == nil || *providerAccount.WorkspaceID != input.WorkspaceID {
		return repository.DatasetGenerationJob{}, ErrForbidden
	}
	if strings.TrimSpace(input.Model) == "" {
		return repository.DatasetGenerationJob{}, errors.New("model is required")
	}
	validateProviderAccount := func(accountID uuid.UUID) error {
		account, accountErr := genRepo.GetProviderAccountByID(ctx, accountID)
		if accountErr != nil {
			return accountErr
		}
		if account.WorkspaceID == nil || *account.WorkspaceID != input.WorkspaceID {
			return ErrForbidden
		}
		return nil
	}
	if strategy == datasetgeneration.StrategyAgenticSelfInstruct {
		if input.JudgeProviderAccountID == nil || *input.JudgeProviderAccountID == uuid.Nil {
			return repository.DatasetGenerationJob{}, errors.New("judge_provider_account_id is required for agentic_self_instruct")
		}
		if err := validateProviderAccount(*input.JudgeProviderAccountID); err != nil {
			return repository.DatasetGenerationJob{}, err
		}
		if strings.TrimSpace(input.JudgeModel) == "" {
			return repository.DatasetGenerationJob{}, errors.New("judge_model is required for agentic_self_instruct")
		}
		if input.MaxRoundsPerExample < 0 || input.MaxRoundsPerExample > 15 {
			return repository.DatasetGenerationJob{}, errors.New("max_rounds_per_example must be between 0 and 15")
		}
		if datasetgeneration.NormalizeAgenticSolverMode(input.SolverMode) == datasetgeneration.SolverModeDirectProvider {
			if input.WeakProviderAccountID != nil && *input.WeakProviderAccountID != uuid.Nil {
				if err := validateProviderAccount(*input.WeakProviderAccountID); err != nil {
					return repository.DatasetGenerationJob{}, err
				}
			}
			if input.StrongProviderAccountID != nil && *input.StrongProviderAccountID != uuid.Nil {
				if err := validateProviderAccount(*input.StrongProviderAccountID); err != nil {
					return repository.DatasetGenerationJob{}, err
				}
			}
		}
	}

	active := domain.DatasetExampleStatusActive
	examples, err := genRepo.ListDatasetExamplesByDatasetID(ctx, repository.ListDatasetExamplesParams{
		DatasetID: input.DatasetID,
		Status:    &active,
		Limit:     10_000,
		Offset:    0,
	})
	if err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	seedCount := 0
	for _, example := range examples {
		if input.SeedsTag != "" {
			if !datasetgeneration.ContainsTag(example.Tags, input.SeedsTag) {
				continue
			}
		}
		seedCount++
	}
	if seedCount == 0 {
		return repository.DatasetGenerationJob{}, errors.New("dataset must have at least one active seed example")
	}

	rawConfig := datasetgeneration.JobConfig{
		ProviderAccountID:       input.ProviderAccountID,
		Model:                   strings.TrimSpace(input.Model),
		SeedsTag:                strings.TrimSpace(input.SeedsTag),
		CreateVersion:           input.CreateVersion,
		VersionLabel:            strings.TrimSpace(input.VersionLabel),
		JudgeProviderAccountID:  input.JudgeProviderAccountID,
		JudgeModel:              strings.TrimSpace(input.JudgeModel),
		MaxRoundsPerExample:     input.MaxRoundsPerExample,
		AcceptanceMode:          strings.TrimSpace(input.AcceptanceMode),
		MinGap:                  input.MinGap,
		MaxWeakScore:            input.MaxWeakScore,
		MinStrongScore:          input.MinStrongScore,
		SolverMode:              strings.TrimSpace(input.SolverMode),
		WeakProviderAccountID:   input.WeakProviderAccountID,
		WeakModel:               strings.TrimSpace(input.WeakModel),
		StrongProviderAccountID: input.StrongProviderAccountID,
		StrongModel:             strings.TrimSpace(input.StrongModel),
		WeakRollouts:            input.WeakRollouts,
		StrongRollouts:          input.StrongRollouts,
		WeakDeploymentID:        input.WeakDeploymentID,
		StrongDeploymentID:      input.StrongDeploymentID,
		ChallengePackVersionID:  input.ChallengePackVersionID,
		ChallengeKey:            strings.TrimSpace(input.ChallengeKey),
		FieldMapping:            append(json.RawMessage(nil), input.FieldMapping...),
	}
	config, err := json.Marshal(rawConfig)
	if err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	decodedConfig, err := datasetgeneration.DecodeJobConfigForStrategy(config, strategy)
	if err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	config, err = json.Marshal(decodedConfig)
	if err != nil {
		return repository.DatasetGenerationJob{}, err
	}

	job, err := genRepo.CreateDatasetGenerationJob(ctx, repository.CreateDatasetGenerationJobParams{
		DatasetID:   input.DatasetID,
		WorkspaceID: input.WorkspaceID,
		Strategy:    strategy,
		Config:      config,
		TargetCount: input.TargetCount,
		Actor:       caller.UserID,
		QueuedAt:    time.Now().UTC(),
	})
	if err != nil {
		return repository.DatasetGenerationJob{}, err
	}

	if err := m.generationWorkflowStarter.StartSyntheticDatasetGenerationWorkflow(ctx, job.ID); err != nil {
		return repository.DatasetGenerationJob{}, DatasetGenerationWorkflowStartError{Job: job, Cause: err}
	}
	return job, nil
}

func (m *DatasetManager) GetDatasetGenerationJob(ctx context.Context, caller Caller, input GetDatasetGenerationJobInput) (repository.DatasetGenerationJob, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	genRepo, ok := m.repo.(DatasetGenerationRepository)
	if !ok {
		return repository.DatasetGenerationJob{}, errors.New("dataset generation repository not configured")
	}
	job, err := genRepo.GetDatasetGenerationJobByID(ctx, input.JobID)
	if err != nil {
		return repository.DatasetGenerationJob{}, err
	}
	if job.DatasetID != input.DatasetID || job.WorkspaceID != input.WorkspaceID {
		return repository.DatasetGenerationJob{}, repository.ErrDatasetGenerationJobNotFound
	}
	return job, nil
}

func startDatasetGenerationHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		var body struct {
			Strategy                string          `json:"strategy"`
			TargetCount             int32           `json:"target_count"`
			ProviderAccountID       uuid.UUID       `json:"provider_account_id"`
			Model                   string          `json:"model"`
			SeedsTag                string          `json:"seeds_tag,omitempty"`
			CreateVersion           bool            `json:"create_version,omitempty"`
			VersionLabel            string          `json:"version_label,omitempty"`
			JudgeProviderAccountID  *uuid.UUID      `json:"judge_provider_account_id,omitempty"`
			JudgeModel              string          `json:"judge_model,omitempty"`
			MaxRoundsPerExample     int             `json:"max_rounds_per_example,omitempty"`
			AcceptanceMode          string          `json:"acceptance_mode,omitempty"`
			MinGap                  *float64        `json:"min_gap,omitempty"`
			MaxWeakScore            *float64        `json:"max_weak_score,omitempty"`
			MinStrongScore          *float64        `json:"min_strong_score,omitempty"`
			SolverMode              string          `json:"solver_mode,omitempty"`
			WeakProviderAccountID   *uuid.UUID      `json:"weak_provider_account_id,omitempty"`
			WeakModel               string          `json:"weak_model,omitempty"`
			StrongProviderAccountID *uuid.UUID      `json:"strong_provider_account_id,omitempty"`
			StrongModel             string          `json:"strong_model,omitempty"`
			WeakRollouts            int             `json:"weak_rollouts,omitempty"`
			StrongRollouts          int             `json:"strong_rollouts,omitempty"`
			WeakDeploymentID        *uuid.UUID      `json:"weak_deployment_id,omitempty"`
			StrongDeploymentID      *uuid.UUID      `json:"strong_deployment_id,omitempty"`
			ChallengePackVersionID  *uuid.UUID      `json:"challenge_pack_version_id,omitempty"`
			ChallengeKey            string          `json:"challenge_key,omitempty"`
			FieldMapping            json.RawMessage `json:"field_mapping,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if body.Strategy == "" || body.TargetCount <= 0 || body.ProviderAccountID == uuid.Nil || strings.TrimSpace(body.Model) == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "strategy, target_count, provider_account_id, and model are required")
			return
		}
		job, err := service.StartDatasetGeneration(r.Context(), caller, StartDatasetGenerationInput{
			WorkspaceID:             workspaceID,
			DatasetID:               datasetID,
			Strategy:                body.Strategy,
			TargetCount:             body.TargetCount,
			ProviderAccountID:       body.ProviderAccountID,
			SeedsTag:                body.SeedsTag,
			CreateVersion:           body.CreateVersion,
			VersionLabel:            body.VersionLabel,
			Model:                   body.Model,
			JudgeProviderAccountID:  body.JudgeProviderAccountID,
			JudgeModel:              body.JudgeModel,
			MaxRoundsPerExample:     body.MaxRoundsPerExample,
			AcceptanceMode:          body.AcceptanceMode,
			MinGap:                  body.MinGap,
			MaxWeakScore:            body.MaxWeakScore,
			MinStrongScore:          body.MinStrongScore,
			SolverMode:              body.SolverMode,
			WeakProviderAccountID:   body.WeakProviderAccountID,
			WeakModel:               body.WeakModel,
			StrongProviderAccountID: body.StrongProviderAccountID,
			StrongModel:             body.StrongModel,
			WeakRollouts:            body.WeakRollouts,
			StrongRollouts:          body.StrongRollouts,
			WeakDeploymentID:        body.WeakDeploymentID,
			StrongDeploymentID:      body.StrongDeploymentID,
			ChallengePackVersionID:  body.ChallengePackVersionID,
			ChallengeKey:            body.ChallengeKey,
			FieldMapping:            body.FieldMapping,
		})
		if err != nil {
			handleDatasetGenerationError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusAccepted, job)
	}
}

func getDatasetGenerationJobHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_job_id", "jobID must be a UUID")
			return
		}
		job, err := service.GetDatasetGenerationJob(r.Context(), caller, GetDatasetGenerationJobInput{
			WorkspaceID: workspaceID,
			DatasetID:   datasetID,
			JobID:       jobID,
		})
		if err != nil {
			handleDatasetGenerationError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, job)
	}
}

func handleDatasetGenerationError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, datasetgeneration.ErrUnsupportedStrategy):
		writeError(w, http.StatusBadRequest, "unsupported_generation_strategy", err.Error())
	case errors.Is(err, repository.ErrDatasetGenerationJobNotFound):
		writeError(w, http.StatusNotFound, "dataset_generation_job_not_found", "dataset generation job not found")
	case errors.Is(err, repository.ErrProviderAccountNotFound):
		writeError(w, http.StatusNotFound, "provider_account_not_found", "provider account not found")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
	case err != nil && isDatasetGenerationValidationError(err):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	default:
		handleDatasetError(w, logger, err)
	}
}

func isDatasetGenerationValidationError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	if message == "target_count must be between 1 and 100" || message == "dataset must have at least one active seed example" || message == "model is required" {
		return true
	}
	return strings.Contains(message, " is required") ||
		strings.Contains(message, " must be ") ||
		strings.Contains(message, "must be between") ||
		strings.Contains(message, "must be valid JSON")
}
