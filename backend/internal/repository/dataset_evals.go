package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MaterializeDatasetVersionInputSetParams struct {
	DatasetID              uuid.UUID
	DatasetVersionID       uuid.UUID
	ChallengePackVersionID uuid.UUID
	ChallengeKey           string
	Mapping                json.RawMessage
}

type DatasetVersionInputSet struct {
	ID                     uuid.UUID       `json:"id"`
	DatasetID              uuid.UUID       `json:"dataset_id"`
	DatasetVersionID       uuid.UUID       `json:"dataset_version_id"`
	ChallengePackVersionID uuid.UUID       `json:"challenge_pack_version_id"`
	ChallengeIdentityID    uuid.UUID       `json:"challenge_identity_id"`
	ChallengeKey           string          `json:"challenge_key"`
	ChallengeInputSetID    uuid.UUID       `json:"challenge_input_set_id"`
	InputKey               string          `json:"input_key"`
	InputChecksum          string          `json:"input_checksum"`
	Mapping                json.RawMessage `json:"mapping"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

type DatasetEvalRun struct {
	ID                       uuid.UUID `json:"id"`
	DatasetID                uuid.UUID `json:"dataset_id"`
	DatasetVersionID         uuid.UUID `json:"dataset_version_id"`
	DatasetVersionInputSetID uuid.UUID `json:"dataset_version_input_set_id"`
	RunID                    uuid.UUID `json:"run_id"`
	CreatedAt                time.Time `json:"created_at"`
}

type RecordDatasetEvalRunParams struct {
	DatasetID                uuid.UUID
	DatasetVersionID         uuid.UUID
	DatasetVersionInputSetID uuid.UUID
	RunID                    uuid.UUID
}

type DatasetEvalResult struct {
	DatasetExampleID    uuid.UUID  `json:"dataset_example_id"`
	DatasetVersionID    uuid.UUID  `json:"dataset_version_id"`
	ChallengeInputSetID uuid.UUID  `json:"challenge_input_set_id"`
	RunID               *uuid.UUID `json:"run_id,omitempty"`
	RunAgentID          *uuid.UUID `json:"run_agent_id,omitempty"`
	Verdict             *string    `json:"verdict,omitempty"`
	NormalizedScore     *float64   `json:"normalized_score,omitempty"`
	JudgedAt            *time.Time `json:"judged_at,omitempty"`
}

type ListDatasetEvalResultsResult struct {
	Items  []DatasetEvalResult `json:"items"`
	Total  int64               `json:"total"`
	Limit  int32               `json:"limit"`
	Offset int32               `json:"offset"`
}

func (r *Repository) MaterializeDatasetVersionInputSet(ctx context.Context, params MaterializeDatasetVersionInputSetParams) (DatasetVersionInputSet, error) {
	version, err := r.GetDatasetVersionByID(ctx, params.DatasetVersionID)
	if err != nil {
		return DatasetVersionInputSet{}, err
	}
	if version.DatasetID != params.DatasetID {
		return DatasetVersionInputSet{}, ErrDatasetVersionNotFound
	}
	examples, err := r.ListDatasetVersionExamples(ctx, params.DatasetVersionID)
	if err != nil {
		return DatasetVersionInputSet{}, err
	}
	challengeKey := strings.TrimSpace(params.ChallengeKey)
	if challengeKey == "" {
		return DatasetVersionInputSet{}, fmt.Errorf("%w: challenge key is required", ErrInvalidDatasetEvalInput)
	}
	challengeIdentityID, err := r.queries.GetChallengeIdentityForDatasetEval(ctx, repositorysqlc.GetChallengeIdentityForDatasetEvalParams{
		ChallengePackVersionID: params.ChallengePackVersionID,
		ChallengeKey:           challengeKey,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetVersionInputSet{}, ErrChallengePackVersionNotFound
		}
		return DatasetVersionInputSet{}, fmt.Errorf("get dataset eval challenge identity: %w", err)
	}

	materialized, err := buildDatasetMaterializedExamples(examples)
	if err != nil {
		return DatasetVersionInputSet{}, err
	}
	inputChecksum := datasetEvalInputChecksum(version, params, materialized)
	inputKey := datasetEvalInputKey(params.DatasetVersionID, challengeKey, inputChecksum)
	description := fmt.Sprintf("Materialized from dataset version %s for challenge %s.", params.DatasetVersionID, challengeKey)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return DatasetVersionInputSet{}, err
	}
	defer tx.Rollback(ctx)
	q := r.queries.WithTx(tx)

	inputSet, err := q.CreateDatasetEvalChallengeInputSet(ctx, repositorysqlc.CreateDatasetEvalChallengeInputSetParams{
		ChallengePackVersionID: params.ChallengePackVersionID,
		InputKey:               inputKey,
		Name:                   "Dataset " + version.VersionNumberLabel(),
		Description:            &description,
		InputChecksum:          inputChecksum,
	})
	if err != nil {
		return DatasetVersionInputSet{}, fmt.Errorf("create dataset eval challenge input set: %w", err)
	}
	versionInputSet, err := q.CreateDatasetVersionInputSet(ctx, repositorysqlc.CreateDatasetVersionInputSetParams{
		DatasetID:              params.DatasetID,
		DatasetVersionID:       params.DatasetVersionID,
		ChallengePackVersionID: params.ChallengePackVersionID,
		ChallengeIdentityID:    challengeIdentityID,
		ChallengeKey:           challengeKey,
		ChallengeInputSetID:    inputSet.ID,
		InputKey:               inputKey,
		InputChecksum:          inputChecksum,
		Mapping:                datasetDefaultJSONObject(params.Mapping),
	})
	if err != nil {
		return DatasetVersionInputSet{}, fmt.Errorf("create dataset version input set link: %w", err)
	}
	for _, item := range materialized {
		row, err := q.UpsertDatasetChallengeInputItem(ctx, repositorysqlc.UpsertDatasetChallengeInputItemParams{
			ChallengeInputSetID:    inputSet.ID,
			ChallengePackVersionID: params.ChallengePackVersionID,
			ChallengeIdentityID:    challengeIdentityID,
			ItemKey:                item.ItemKey,
			Payload:                item.Payload,
		})
		if err != nil {
			return DatasetVersionInputSet{}, fmt.Errorf("upsert dataset challenge input item: %w", err)
		}
		if _, err := q.UpsertDatasetInputItemLink(ctx, repositorysqlc.UpsertDatasetInputItemLinkParams{
			DatasetVersionInputSetID: versionInputSet.ID,
			DatasetExampleID:         item.DatasetExampleID,
			ChallengeInputItemID:     row.ID,
			ItemKey:                  item.ItemKey,
		}); err != nil {
			return DatasetVersionInputSet{}, fmt.Errorf("upsert dataset input item link: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetVersionInputSet{}, err
	}
	return mapDatasetVersionInputSet(versionInputSet)
}

func (v DatasetVersion) VersionNumberLabel() string {
	return fmt.Sprintf("v%d", v.VersionNumber)
}

type datasetMaterializedExample struct {
	DatasetExampleID uuid.UUID
	ItemKey          string
	Payload          json.RawMessage
}

func buildDatasetMaterializedExamples(examples []DatasetExample) ([]datasetMaterializedExample, error) {
	sorted := append([]DatasetExample(nil), examples...)
	sort.Slice(sorted, func(i, j int) bool {
		return datasetExampleStableKey(sorted[i]) < datasetExampleStableKey(sorted[j])
	})
	out := make([]datasetMaterializedExample, 0, len(sorted))
	for _, example := range sorted {
		itemKey := datasetExampleStableKey(example)
		payload, err := datasetExampleCaseDocument(example, itemKey)
		if err != nil {
			return nil, err
		}
		out = append(out, datasetMaterializedExample{
			DatasetExampleID: example.ID,
			ItemKey:          itemKey,
			Payload:          payload,
		})
	}
	return out, nil
}

func datasetExampleStableKey(example DatasetExample) string {
	if example.ExternalID != nil && strings.TrimSpace(*example.ExternalID) != "" {
		return strings.TrimSpace(*example.ExternalID)
	}
	return example.ID.String()
}

func datasetExampleCaseDocument(example DatasetExample, itemKey string) (json.RawMessage, error) {
	input := decodeRawForCaseDocument(example.Input)
	expected := decodeRawForCaseDocument(example.Expected)
	metadata := decodeRawForCaseDocument(example.Metadata)
	payload := map[string]any{"input": input, "expected": expected, "metadata": metadata}
	doc := challengepack.StoredCaseDocument{
		SchemaVersion: 1,
		CaseKey:       itemKey,
		Payload:       payload,
		Inputs: []challengepack.CaseInput{{
			Key:   "input",
			Kind:  "json",
			Value: input,
		}},
	}
	if len(strings.TrimSpace(string(example.Expected))) > 0 && string(example.Expected) != "null" {
		doc.Expectations = []challengepack.CaseExpectation{{
			Key:    "expected",
			Kind:   "json",
			Value:  expected,
			Source: "dataset",
		}}
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal case document for %q: %w", itemKey, err)
	}
	return encoded, nil
}

func decodeRawForCaseDocument(raw json.RawMessage) any {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}

func datasetEvalInputChecksum(version DatasetVersion, params MaterializeDatasetVersionInputSetParams, items []datasetMaterializedExample) string {
	hash := sha256.New()
	hash.Write([]byte(version.ManifestChecksum))
	hash.Write([]byte(params.ChallengePackVersionID.String()))
	hash.Write([]byte(params.ChallengeKey))
	hash.Write(datasetDefaultJSONObject(params.Mapping))
	for _, item := range items {
		hash.Write([]byte(item.ItemKey))
		hash.Write(item.Payload)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func datasetEvalInputKey(versionID uuid.UUID, challengeKey, checksum string) string {
	short := checksum
	if len(short) > 12 {
		short = short[:12]
	}
	return "dataset-" + versionID.String() + "-" + strings.ToLower(challengeKey) + "-" + short
}

func (r *Repository) RecordDatasetEvalRun(ctx context.Context, params RecordDatasetEvalRunParams) (DatasetEvalRun, error) {
	row, err := r.queries.RecordDatasetEvalRun(ctx, repositorysqlc.RecordDatasetEvalRunParams{
		DatasetID:                params.DatasetID,
		DatasetVersionID:         params.DatasetVersionID,
		DatasetVersionInputSetID: params.DatasetVersionInputSetID,
		RunID:                    params.RunID,
	})
	if err != nil {
		return DatasetEvalRun{}, fmt.Errorf("record dataset eval run: %w", err)
	}
	return mapDatasetEvalRun(row)
}

func (r *Repository) ListDatasetEvalResults(ctx context.Context, datasetID uuid.UUID, versionID *uuid.UUID, limit, offset int32) (ListDatasetEvalResultsResult, error) {
	rows, err := r.queries.ListDatasetEvalResults(ctx, repositorysqlc.ListDatasetEvalResultsParams{
		DatasetID: datasetID, DatasetVersionID: versionID, ResultLimit: limit, ResultOffset: offset,
	})
	if err != nil {
		return ListDatasetEvalResultsResult{}, fmt.Errorf("list dataset eval results: %w", err)
	}
	total, err := r.queries.CountDatasetEvalResults(ctx, repositorysqlc.CountDatasetEvalResultsParams{DatasetID: datasetID, DatasetVersionID: versionID})
	if err != nil {
		return ListDatasetEvalResultsResult{}, fmt.Errorf("count dataset eval results: %w", err)
	}
	out := make([]DatasetEvalResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, DatasetEvalResult{
			DatasetExampleID:    row.DatasetExampleID,
			DatasetVersionID:    row.DatasetVersionID,
			ChallengeInputSetID: row.ChallengeInputSetID,
			RunID:               row.RunID,
			RunAgentID:          row.RunAgentID,
			Verdict:             row.Verdict,
			NormalizedScore:     numericPtr(row.NormalizedScore),
			JudgedAt:            optionalTime(row.JudgedAt),
		})
	}
	return ListDatasetEvalResultsResult{Items: out, Total: total, Limit: limit, Offset: offset}, nil
}

func mapDatasetVersionInputSet(row repositorysqlc.DatasetVersionInputSet) (DatasetVersionInputSet, error) {
	createdAt, err := requiredTime("dataset_version_input_sets.created_at", row.CreatedAt)
	if err != nil {
		return DatasetVersionInputSet{}, err
	}
	updatedAt, err := requiredTime("dataset_version_input_sets.updated_at", row.UpdatedAt)
	if err != nil {
		return DatasetVersionInputSet{}, err
	}
	return DatasetVersionInputSet{
		ID: row.ID, DatasetID: row.DatasetID, DatasetVersionID: row.DatasetVersionID,
		ChallengePackVersionID: row.ChallengePackVersionID, ChallengeIdentityID: row.ChallengeIdentityID,
		ChallengeKey: row.ChallengeKey, ChallengeInputSetID: row.ChallengeInputSetID, InputKey: row.InputKey,
		InputChecksum: row.InputChecksum, Mapping: cloneBytes(row.Mapping), CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func mapDatasetEvalRun(row repositorysqlc.DatasetEvalRun) (DatasetEvalRun, error) {
	createdAt, err := requiredTime("dataset_eval_runs.created_at", row.CreatedAt)
	if err != nil {
		return DatasetEvalRun{}, err
	}
	return DatasetEvalRun{
		ID: row.ID, DatasetID: row.DatasetID, DatasetVersionID: row.DatasetVersionID,
		DatasetVersionInputSetID: row.DatasetVersionInputSetID, RunID: row.RunID, CreatedAt: createdAt,
	}, nil
}
