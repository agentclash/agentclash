package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	datasetgate "github.com/agentclash/agentclash/backend/internal/datasets/gate"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrDatasetBaselineNotFound = errors.New("dataset baseline not found")

type DatasetBaseline struct {
	ID                       uuid.UUID       `json:"id"`
	DatasetID                uuid.UUID       `json:"dataset_id"`
	DatasetVersionID         uuid.UUID       `json:"dataset_version_id"`
	DatasetVersionInputSetID *uuid.UUID      `json:"dataset_version_input_set_id,omitempty"`
	ChallengePackVersionID   uuid.UUID       `json:"challenge_pack_version_id"`
	ChallengeKey             string          `json:"challenge_key"`
	AgentDeploymentID        *uuid.UUID      `json:"agent_deployment_id,omitempty"`
	RunID                    uuid.UUID       `json:"run_id"`
	PassRate                 *float64        `json:"pass_rate,omitempty"`
	Metrics                  json.RawMessage `json:"metrics"`
	ExampleOutcomes          json.RawMessage `json:"example_outcomes"`
	Label                    *string         `json:"label,omitempty"`
	CreatedBy                uuid.UUID       `json:"created_by"`
	CreatedAt                time.Time       `json:"created_at"`
}

type CreateDatasetBaselineParams struct {
	DatasetID         uuid.UUID
	RunID             uuid.UUID
	AgentDeploymentID *uuid.UUID
	Label             *string
	Actor             uuid.UUID
}

type ListDatasetBaselinesParams struct {
	DatasetID uuid.UUID
	Limit     int32
	Offset    int32
}

type ListDatasetBaselinesResult struct {
	Items  []DatasetBaseline `json:"items"`
	Total  int64             `json:"total"`
	Limit  int32             `json:"limit"`
	Offset int32             `json:"offset"`
}

func (r *Repository) CreateDatasetBaseline(ctx context.Context, params CreateDatasetBaselineParams) (DatasetBaseline, error) {
	evalRun, err := r.queries.GetDatasetEvalRunByRunID(ctx, repositorysqlc.GetDatasetEvalRunByRunIDParams{RunID: params.RunID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetBaseline{}, ErrRunNotFound
		}
		return DatasetBaseline{}, fmt.Errorf("get dataset eval run: %w", err)
	}
	if evalRun.DatasetID != params.DatasetID {
		return DatasetBaseline{}, ErrRunNotFound
	}
	inputSet, err := r.queries.GetDatasetVersionInputSetByID(ctx, repositorysqlc.GetDatasetVersionInputSetByIDParams{ID: evalRun.DatasetVersionInputSetID})
	if err != nil {
		return DatasetBaseline{}, fmt.Errorf("get dataset version input set: %w", err)
	}
	outcomes, err := r.listDatasetEvalOutcomesForRun(ctx, params.RunID, params.AgentDeploymentID)
	if err != nil {
		return DatasetBaseline{}, err
	}
	passRate := datasetgate.RoundPassRate(datasetgate.PassRate(outcomes))
	outcomesJSON, err := json.Marshal(outcomes)
	if err != nil {
		return DatasetBaseline{}, err
	}
	metricsJSON, err := json.Marshal(map[string]any{
		"example_count": len(outcomes),
		"pass_rate":     passRate,
	})
	if err != nil {
		return DatasetBaseline{}, err
	}
	row, err := r.queries.InsertDatasetBaseline(ctx, repositorysqlc.InsertDatasetBaselineParams{
		DatasetID:                params.DatasetID,
		DatasetVersionID:         evalRun.DatasetVersionID,
		DatasetVersionInputSetID: &evalRun.DatasetVersionInputSetID,
		ChallengePackVersionID:   inputSet.ChallengePackVersionID,
		ChallengeKey:             inputSet.ChallengeKey,
		AgentDeploymentID:        params.AgentDeploymentID,
		RunID:                    params.RunID,
		PassRate:                 pgtypeNumericFromFloat(passRate),
		Metrics:                  metricsJSON,
		ExampleOutcomes:          outcomesJSON,
		Label:                    params.Label,
		CreatedByUserID:          params.Actor,
	})
	if err != nil {
		return DatasetBaseline{}, fmt.Errorf("insert dataset baseline: %w", err)
	}
	return mapDatasetBaseline(row), nil
}

func (r *Repository) ListDatasetBaselines(ctx context.Context, params ListDatasetBaselinesParams) (ListDatasetBaselinesResult, error) {
	total, err := r.queries.CountDatasetBaselinesByDatasetID(ctx, repositorysqlc.CountDatasetBaselinesByDatasetIDParams{DatasetID: params.DatasetID})
	if err != nil {
		return ListDatasetBaselinesResult{}, err
	}
	rows, err := r.queries.ListDatasetBaselinesByDatasetID(ctx, repositorysqlc.ListDatasetBaselinesByDatasetIDParams{
		DatasetID:    params.DatasetID,
		ResultLimit:  params.Limit,
		ResultOffset: params.Offset,
	})
	if err != nil {
		return ListDatasetBaselinesResult{}, err
	}
	items := make([]DatasetBaseline, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapDatasetBaseline(row))
	}
	return ListDatasetBaselinesResult{Items: items, Total: total, Limit: params.Limit, Offset: params.Offset}, nil
}

func (r *Repository) GetDatasetBaselineByID(ctx context.Context, baselineID uuid.UUID) (DatasetBaseline, error) {
	row, err := r.queries.GetDatasetBaselineByID(ctx, repositorysqlc.GetDatasetBaselineByIDParams{ID: baselineID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetBaseline{}, ErrDatasetBaselineNotFound
		}
		return DatasetBaseline{}, err
	}
	return mapDatasetBaseline(row), nil
}

func (r *Repository) GetDatasetEvalRunByRunID(ctx context.Context, runID uuid.UUID) (DatasetEvalRun, error) {
	row, err := r.queries.GetDatasetEvalRunByRunID(ctx, repositorysqlc.GetDatasetEvalRunByRunIDParams{RunID: runID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetEvalRun{}, ErrRunNotFound
		}
		return DatasetEvalRun{}, err
	}
	return DatasetEvalRun{
		ID:                       row.ID,
		DatasetID:                row.DatasetID,
		DatasetVersionID:         row.DatasetVersionID,
		DatasetVersionInputSetID: row.DatasetVersionInputSetID,
		RunID:                    row.RunID,
		CreatedAt:                pgTimeValue(row.CreatedAt),
	}, nil
}

func (r *Repository) ListDatasetEvalOutcomesForRun(ctx context.Context, runID uuid.UUID, agentDeploymentID *uuid.UUID) ([]datasetgate.ExampleOutcome, error) {
	return r.listDatasetEvalOutcomesForRun(ctx, runID, agentDeploymentID)
}

func (r *Repository) listDatasetEvalOutcomesForRun(ctx context.Context, runID uuid.UUID, agentDeploymentID *uuid.UUID) ([]datasetgate.ExampleOutcome, error) {
	rows, err := r.queries.ListDatasetEvalResultsForRun(ctx, repositorysqlc.ListDatasetEvalResultsForRunParams{
		RunID:             runID,
		AgentDeploymentID: agentDeploymentID,
	})
	if err != nil {
		return nil, fmt.Errorf("list dataset eval results for run: %w", err)
	}
	outcomes := make([]datasetgate.ExampleOutcome, 0, len(rows))
	for _, row := range rows {
		item := datasetgate.ExampleOutcome{DatasetExampleID: row.DatasetExampleID}
		if row.Verdict != nil {
			item.Verdict = row.Verdict
		}
		if row.NormalizedScore.Valid {
			score := numericToFloat64(row.NormalizedScore)
			item.NormalizedScore = &score
		}
		outcomes = append(outcomes, item)
	}
	return outcomes, nil
}

func mapDatasetBaseline(row repositorysqlc.DatasetBaseline) DatasetBaseline {
	baseline := DatasetBaseline{
		ID:                     row.ID,
		DatasetID:              row.DatasetID,
		DatasetVersionID:       row.DatasetVersionID,
		ChallengePackVersionID: row.ChallengePackVersionID,
		ChallengeKey:           row.ChallengeKey,
		RunID:                  row.RunID,
		Metrics:                append(json.RawMessage(nil), row.Metrics...),
		ExampleOutcomes:        append(json.RawMessage(nil), row.ExampleOutcomes...),
		CreatedBy:              row.CreatedByUserID,
	}
	if row.DatasetVersionInputSetID != nil {
		baseline.DatasetVersionInputSetID = row.DatasetVersionInputSetID
	}
	if row.AgentDeploymentID != nil {
		baseline.AgentDeploymentID = row.AgentDeploymentID
	}
	if row.PassRate.Valid {
		value := numericToFloat64(row.PassRate)
		baseline.PassRate = &value
	}
	if row.Label != nil {
		baseline.Label = row.Label
	}
	baseline.CreatedAt = pgTimeValue(row.CreatedAt)
	return baseline
}

func pgTimeValue(value pgtype.Timestamptz) time.Time {
	if value.Valid {
		return value.Time
	}
	return time.Time{}
}

func pgtypeNumericFromFloat(value float64) pgtype.Numeric {
	var numeric pgtype.Numeric
	_ = numeric.Scan(fmt.Sprintf("%f", value))
	return numeric
}

func numericToFloat64(value pgtype.Numeric) float64 {
	f, err := value.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func DecodeDatasetBaselineOutcomes(raw json.RawMessage) ([]datasetgate.ExampleOutcome, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var outcomes []datasetgate.ExampleOutcome
	if err := json.Unmarshal(raw, &outcomes); err != nil {
		return nil, err
	}
	return outcomes, nil
}
