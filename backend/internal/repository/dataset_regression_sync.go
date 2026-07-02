package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrDatasetRegressionSuiteLinkNotFound = errors.New("dataset regression suite link not found")

type DatasetRegressionSuiteLink struct {
	DatasetID         uuid.UUID  `json:"dataset_id"`
	RegressionSuiteID uuid.UUID  `json:"regression_suite_id"`
	SyncedVersionID   *uuid.UUID `json:"synced_version_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type SyncDatasetRegressionSuiteParams struct {
	DatasetID              uuid.UUID
	WorkspaceID            uuid.UUID
	VersionID              uuid.UUID
	ChallengePackVersionID uuid.UUID
	ChallengeKey           string
	RegressionSuiteID      *uuid.UUID
	SuiteName              *string
	Actor                  uuid.UUID
}

type SyncDatasetRegressionSuiteResult struct {
	Link          DatasetRegressionSuiteLink `json:"link"`
	Suite         RegressionSuite            `json:"suite"`
	CreatedCases  int                        `json:"created_cases"`
	SkippedCases  int                        `json:"skipped_cases"`
	TotalExamples int                        `json:"total_examples"`
}

func (r *Repository) GetDatasetRegressionSuiteLink(ctx context.Context, datasetID uuid.UUID) (DatasetRegressionSuiteLink, error) {
	row, err := r.queries.GetDatasetRegressionSuiteLinkByDatasetID(ctx, repositorysqlc.GetDatasetRegressionSuiteLinkByDatasetIDParams{DatasetID: datasetID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetRegressionSuiteLink{}, ErrDatasetRegressionSuiteLinkNotFound
		}
		return DatasetRegressionSuiteLink{}, err
	}
	return mapDatasetRegressionSuiteLink(row), nil
}

func (r *Repository) SyncDatasetRegressionSuite(ctx context.Context, params SyncDatasetRegressionSuiteParams) (SyncDatasetRegressionSuiteResult, error) {
	dataset, err := r.GetDatasetByID(ctx, params.DatasetID)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}
	if dataset.WorkspaceID != params.WorkspaceID {
		return SyncDatasetRegressionSuiteResult{}, ErrDatasetNotFound
	}
	version, err := r.GetDatasetVersionByID(ctx, params.VersionID)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}
	if version.DatasetID != params.DatasetID {
		return SyncDatasetRegressionSuiteResult{}, ErrDatasetVersionNotFound
	}
	packVersion, err := r.GetRunnableChallengePackVersionByID(ctx, params.ChallengePackVersionID)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}
	if packVersion.WorkspaceID != nil && *packVersion.WorkspaceID != params.WorkspaceID {
		return SyncDatasetRegressionSuiteResult{}, ErrChallengePackVersionNotFound
	}
	if packVersion.WorkspaceID == nil {
		publicPacks, accessErr := r.WorkspacePublicPacksEnabled(ctx, params.WorkspaceID)
		if accessErr != nil {
			return SyncDatasetRegressionSuiteResult{}, accessErr
		}
		if !publicPacks {
			return SyncDatasetRegressionSuiteResult{}, ErrChallengePackVersionNotFound
		}
	}

	versionInputSet, err := r.MaterializeDatasetVersionInputSet(ctx, MaterializeDatasetVersionInputSetParams{
		DatasetID:              params.DatasetID,
		DatasetVersionID:       params.VersionID,
		ChallengePackVersionID: params.ChallengePackVersionID,
		ChallengeKey:           params.ChallengeKey,
	})
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}

	examples, err := r.ListDatasetVersionExamples(ctx, params.VersionID)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}
	materialized, err := buildDatasetMaterializedExamples(examples)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, fmt.Errorf("begin dataset regression sync transaction: %w", err)
	}
	defer rollback(ctx, tx)

	queries := r.queries.WithTx(tx)
	suite, err := r.resolveDatasetRegressionSuiteWithQueries(ctx, queries, params, dataset, packVersion.ChallengePackID)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}
	if suite.SourceChallengePackID != packVersion.ChallengePackID {
		return SyncDatasetRegressionSuiteResult{}, ErrRegressionSuitePackMismatch
	}

	existingCases, err := r.listRegressionCasesBySuiteIDWithQueries(ctx, queries, suite.ID)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}
	existingByExample := regressionCasesByDatasetExampleID(existingCases)

	inputSetID := versionInputSet.ChallengeInputSetID
	createdCases := 0
	skippedCases := 0
	for _, item := range materialized {
		if _, ok := existingByExample[item.DatasetExampleID.String()]; ok {
			skippedCases++
			continue
		}
		example := exampleByID(examples, item.DatasetExampleID)
		if example == nil {
			continue
		}
		title := datasetRegressionCaseTitle(*example, item.ItemKey)
		metadata, err := datasetRegressionCaseMetadata(*example, params.VersionID)
		if err != nil {
			return SyncDatasetRegressionSuiteResult{}, err
		}
		if _, err := r.createRegressionCaseWithQueries(ctx, queries, CreateRegressionCaseParams{
			SuiteID:                      suite.ID,
			Title:                        title,
			Description:                  fmt.Sprintf("Promoted from dataset version %s.", version.VersionNumberLabel()),
			Status:                       domain.RegressionCaseStatusActive,
			Severity:                     domain.RegressionSeverityInfo,
			PromotionMode:                domain.RegressionPromotionModeManual,
			SourceRunID:                  example.SourceRunID,
			SourceChallengePackVersionID: params.ChallengePackVersionID,
			SourceChallengeInputSetID:    &inputSetID,
			SourceChallengeIdentityID:    versionInputSet.ChallengeIdentityID,
			SourceCaseKey:                item.ItemKey,
			SourceItemKey:                &item.ItemKey,
			EvidenceTier:                 "dataset",
			FailureClass:                 "dataset_example",
			FailureSummary:               "Golden dataset example",
			PayloadSnapshot:              item.Payload,
			Metadata:                     metadata,
		}); err != nil {
			return SyncDatasetRegressionSuiteResult{}, err
		}
		createdCases++
	}

	linkRow, err := queries.UpsertDatasetRegressionSuiteLink(ctx, repositorysqlc.UpsertDatasetRegressionSuiteLinkParams{
		DatasetID:         params.DatasetID,
		RegressionSuiteID: suite.ID,
		SyncedVersionID:   &params.VersionID,
	})
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, fmt.Errorf("upsert dataset regression suite link: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return SyncDatasetRegressionSuiteResult{}, fmt.Errorf("commit dataset regression sync transaction: %w", err)
	}

	suite.CaseCount, err = r.countRegressionCasesBySuiteID(ctx, suite.ID)
	if err != nil {
		return SyncDatasetRegressionSuiteResult{}, err
	}
	return SyncDatasetRegressionSuiteResult{
		Link:          mapDatasetRegressionSuiteLink(linkRow),
		Suite:         suite,
		CreatedCases:  createdCases,
		SkippedCases:  skippedCases,
		TotalExamples: len(materialized),
	}, nil
}

func (r *Repository) resolveDatasetRegressionSuiteWithQueries(
	ctx context.Context,
	queries *repositorysqlc.Queries,
	params SyncDatasetRegressionSuiteParams,
	dataset Dataset,
	challengePackID uuid.UUID,
) (RegressionSuite, error) {
	if params.RegressionSuiteID != nil {
		suite, err := r.getRegressionSuiteByIDWithQueries(ctx, queries, *params.RegressionSuiteID)
		if err != nil {
			return RegressionSuite{}, err
		}
		if suite.WorkspaceID != params.WorkspaceID {
			return RegressionSuite{}, ErrRegressionSuiteNotFound
		}
		return suite, nil
	}
	linkRow, err := queries.GetDatasetRegressionSuiteLinkByDatasetID(ctx, repositorysqlc.GetDatasetRegressionSuiteLinkByDatasetIDParams{
		DatasetID: params.DatasetID,
	})
	switch {
	case err == nil:
		return r.getRegressionSuiteByIDWithQueries(ctx, queries, linkRow.RegressionSuiteID)
	case errors.Is(err, pgx.ErrNoRows):
		// continue to create or reuse suite by name
	default:
		return RegressionSuite{}, err
	}

	name := datasetRegressionSuiteName(params.SuiteName, dataset.Name)
	suite, err := r.createRegressionSuiteWithQueries(ctx, queries, CreateRegressionSuiteParams{
		WorkspaceID:           params.WorkspaceID,
		SourceChallengePackID: challengePackID,
		Name:                  name,
		Description:           fmt.Sprintf("Regression suite synced from dataset %s.", dataset.Slug),
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "mixed_manual",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       params.Actor,
	})
	if errors.Is(err, ErrRegressionSuiteNameConflict) {
		return r.getRegressionSuiteByWorkspaceAndName(ctx, queries, params.WorkspaceID, name, challengePackID)
	}
	return suite, err
}

func datasetRegressionSuiteName(explicit *string, datasetName string) string {
	if explicit != nil {
		if name := strings.TrimSpace(*explicit); name != "" {
			return name
		}
	}
	return strings.TrimSpace(datasetName + " regression")
}

func (r *Repository) getRegressionSuiteByWorkspaceAndName(
	ctx context.Context,
	queries *repositorysqlc.Queries,
	workspaceID uuid.UUID,
	name string,
	challengePackID uuid.UUID,
) (RegressionSuite, error) {
	rows, err := queries.ListRegressionSuitesByWorkspaceID(ctx, repositorysqlc.ListRegressionSuitesByWorkspaceIDParams{
		WorkspaceID:  workspaceID,
		ResultLimit:  1000,
		ResultOffset: 0,
	})
	if err != nil {
		return RegressionSuite{}, fmt.Errorf("list regression suites by workspace id: %w", err)
	}
	trimmed := strings.TrimSpace(name)
	for _, row := range rows {
		if strings.TrimSpace(row.Name) != trimmed {
			continue
		}
		if row.SourceChallengePackID != challengePackID {
			return RegressionSuite{}, ErrRegressionSuiteNameConflict
		}
		suite, mapErr := mapRegressionSuite(row)
		if mapErr != nil {
			return RegressionSuite{}, mapErr
		}
		return suite, nil
	}
	return RegressionSuite{}, ErrRegressionSuiteNotFound
}

func regressionCasesByDatasetExampleID(cases []RegressionCase) map[string]struct{} {
	out := make(map[string]struct{}, len(cases))
	for _, item := range cases {
		if len(item.Metadata) == 0 {
			continue
		}
		var metadata map[string]any
		if err := json.Unmarshal(item.Metadata, &metadata); err != nil {
			continue
		}
		raw, ok := metadata["dataset_example_id"].(string)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		out[strings.TrimSpace(raw)] = struct{}{}
	}
	return out
}

func exampleByID(examples []DatasetExample, id uuid.UUID) *DatasetExample {
	for i := range examples {
		if examples[i].ID == id {
			return &examples[i]
		}
	}
	return nil
}

func datasetRegressionCaseTitle(example DatasetExample, itemKey string) string {
	if example.ExternalID != nil && strings.TrimSpace(*example.ExternalID) != "" {
		return strings.TrimSpace(*example.ExternalID)
	}
	return itemKey
}

func datasetRegressionCaseMetadata(example DatasetExample, versionID uuid.UUID) (json.RawMessage, error) {
	base := map[string]any{}
	if len(example.Metadata) > 0 {
		if err := json.Unmarshal(example.Metadata, &base); err != nil {
			return nil, fmt.Errorf("decode dataset example metadata: %w", err)
		}
	}
	base["dataset_example_id"] = example.ID.String()
	base["dataset_id"] = example.DatasetID.String()
	base["dataset_version_id"] = versionID.String()
	if example.SourceRunID != nil {
		base["source_run_id"] = example.SourceRunID.String()
	}
	if example.SourceTraceID != nil && strings.TrimSpace(*example.SourceTraceID) != "" {
		base["source_trace_id"] = strings.TrimSpace(*example.SourceTraceID)
	}
	if example.SourcePlatform != nil && strings.TrimSpace(*example.SourcePlatform) != "" {
		base["source_platform"] = strings.TrimSpace(*example.SourcePlatform)
	}
	encoded, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func mapDatasetRegressionSuiteLink(row repositorysqlc.DatasetRegressionSuiteLink) DatasetRegressionSuiteLink {
	return DatasetRegressionSuiteLink{
		DatasetID:         row.DatasetID,
		RegressionSuiteID: row.RegressionSuiteID,
		SyncedVersionID:   row.SyncedVersionID,
		CreatedAt:         pgTimeValue(row.CreatedAt),
		UpdatedAt:         pgTimeValue(row.UpdatedAt),
	}
}
