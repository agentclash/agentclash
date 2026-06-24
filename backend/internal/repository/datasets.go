package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	repositorysqlc "github.com/agentclash/agentclash/backend/internal/repository/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Dataset struct {
	ID                            uuid.UUID       `json:"id"`
	OrganizationID                uuid.UUID       `json:"organization_id"`
	WorkspaceID                   uuid.UUID       `json:"workspace_id"`
	Slug                          string          `json:"slug"`
	Name                          string          `json:"name"`
	Description                   string          `json:"description"`
	InputSchema                   json.RawMessage `json:"input_schema,omitempty"`
	InputSchemaEnforced           bool            `json:"input_schema_enforced"`
	DefaultChallengePackVersionID *uuid.UUID      `json:"default_challenge_pack_version_id,omitempty"`
	ActiveExampleCount            int             `json:"active_example_count"`
	VersionCount                  int             `json:"version_count"`
	CreatedBy                     uuid.UUID       `json:"created_by"`
	CreatedAt                     time.Time       `json:"created_at"`
	UpdatedAt                     time.Time       `json:"updated_at"`
	ArchivedAt                    *time.Time      `json:"archived_at,omitempty"`
}

type DatasetExample struct {
	ID             uuid.UUID                   `json:"id"`
	DatasetID      uuid.UUID                   `json:"dataset_id"`
	ExternalID     *string                     `json:"external_id,omitempty"`
	Input          json.RawMessage             `json:"input"`
	Expected       json.RawMessage             `json:"expected,omitempty"`
	Metadata       json.RawMessage             `json:"metadata"`
	Tags           []string                    `json:"tags"`
	Status         domain.DatasetExampleStatus `json:"status"`
	Source         domain.DatasetExampleSource `json:"source"`
	SourceRunID    *uuid.UUID                  `json:"source_run_id,omitempty"`
	SourceTraceID  *string                     `json:"source_trace_id,omitempty"`
	SourcePlatform *string                     `json:"source_platform,omitempty"`
	ArtifactID     *uuid.UUID                  `json:"artifact_id,omitempty"`
	CreatedBy      uuid.UUID                   `json:"created_by"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

type DatasetVersion struct {
	ID               uuid.UUID `json:"id"`
	DatasetID        uuid.UUID `json:"dataset_id"`
	VersionNumber    int32     `json:"version_number"`
	Label            *string   `json:"label,omitempty"`
	ExampleCount     int32     `json:"example_count"`
	ManifestChecksum string    `json:"manifest_checksum"`
	CreatedBy        uuid.UUID `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
}

type CreateDatasetParams struct {
	WorkspaceID                   uuid.UUID
	Slug                          string
	Name                          string
	Description                   string
	InputSchema                   json.RawMessage
	InputSchemaEnforced           bool
	DefaultChallengePackVersionID *uuid.UUID
	CreatedBy                     uuid.UUID
}

type PatchDatasetParams struct {
	ID                            uuid.UUID
	Slug                          *string
	Name                          *string
	Description                   *string
	InputSchema                   json.RawMessage
	InputSchemaEnforced           *bool
	DefaultChallengePackVersionID *uuid.UUID
}

type UpsertDatasetExampleParams struct {
	DatasetID      uuid.UUID
	ExternalID     *string
	Input          json.RawMessage
	Expected       json.RawMessage
	Metadata       json.RawMessage
	Tags           []string
	Status         domain.DatasetExampleStatus
	Source         domain.DatasetExampleSource
	SourceRunID    *uuid.UUID
	SourceTraceID  *string
	SourcePlatform *string
	ArtifactID     *uuid.UUID
	Actor          uuid.UUID
}

type PatchDatasetExampleParams struct {
	ID             uuid.UUID
	Input          json.RawMessage
	Expected       json.RawMessage
	Metadata       json.RawMessage
	Tags           []string
	Status         *domain.DatasetExampleStatus
	Source         *domain.DatasetExampleSource
	SourceRunID    *uuid.UUID
	SourceTraceID  *string
	SourcePlatform *string
	ArtifactID     *uuid.UUID
	Actor          uuid.UUID
}

type CreateDatasetVersionParams struct {
	DatasetID uuid.UUID
	Label     *string
	Actor     uuid.UUID
}

type ListDatasetExamplesParams struct {
	DatasetID uuid.UUID
	Status    *domain.DatasetExampleStatus
	Limit     int32
	Offset    int32
}

const datasetSlugUniqueIndex = "datasets_workspace_id_slug_key"

func (r *Repository) CreateDataset(ctx context.Context, params CreateDatasetParams) (Dataset, error) {
	row, err := r.queries.CreateDataset(ctx, repositorysqlc.CreateDatasetParams{
		WorkspaceID:                   params.WorkspaceID,
		Slug:                          strings.TrimSpace(params.Slug),
		Name:                          strings.TrimSpace(params.Name),
		Description:                   params.Description,
		InputSchema:                   nullableJSON(params.InputSchema),
		InputSchemaEnforced:           params.InputSchemaEnforced,
		DefaultChallengePackVersionID: params.DefaultChallengePackVersionID,
		CreatedBy:                     params.CreatedBy,
	})
	if err != nil {
		if isDatasetSlugConflict(err) {
			return Dataset{}, ErrDatasetSlugConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Dataset{}, ErrDatasetNotFound
		}
		return Dataset{}, fmt.Errorf("create dataset: %w", err)
	}
	return mapDataset(row)
}

func (r *Repository) GetDatasetByID(ctx context.Context, id uuid.UUID) (Dataset, error) {
	row, err := r.queries.GetDatasetByID(ctx, repositorysqlc.GetDatasetByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Dataset{}, ErrDatasetNotFound
		}
		return Dataset{}, fmt.Errorf("get dataset by id: %w", err)
	}
	return mapDatasetDetailRow(row)
}

func (r *Repository) ListDatasetsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, limit, offset int32) ([]Dataset, error) {
	rows, err := r.queries.ListDatasetsByWorkspaceID(ctx, repositorysqlc.ListDatasetsByWorkspaceIDParams{
		WorkspaceID: workspaceID, ResultLimit: limit, ResultOffset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list datasets by workspace id: %w", err)
	}
	out := make([]Dataset, 0, len(rows))
	for _, row := range rows {
		dataset, err := mapDatasetListRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, dataset)
	}
	return out, nil
}

func (r *Repository) CountDatasetsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (int64, error) {
	count, err := r.queries.CountDatasetsByWorkspaceID(ctx, repositorysqlc.CountDatasetsByWorkspaceIDParams{WorkspaceID: workspaceID})
	if err != nil {
		return 0, fmt.Errorf("count datasets by workspace id: %w", err)
	}
	return count, nil
}

func (r *Repository) PatchDataset(ctx context.Context, params PatchDatasetParams) (Dataset, error) {
	row, err := r.queries.PatchDataset(ctx, repositorysqlc.PatchDatasetParams{
		ID:                            params.ID,
		Slug:                          trimStringPtr(params.Slug),
		Name:                          trimStringPtr(params.Name),
		Description:                   params.Description,
		InputSchema:                   nullableJSON(params.InputSchema),
		InputSchemaEnforced:           params.InputSchemaEnforced,
		DefaultChallengePackVersionID: params.DefaultChallengePackVersionID,
	})
	if err != nil {
		if isDatasetSlugConflict(err) {
			return Dataset{}, ErrDatasetSlugConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Dataset{}, ErrDatasetNotFound
		}
		return Dataset{}, fmt.Errorf("patch dataset: %w", err)
	}
	return mapDataset(row)
}

func (r *Repository) ArchiveDataset(ctx context.Context, id uuid.UUID) (Dataset, error) {
	row, err := r.queries.ArchiveDataset(ctx, repositorysqlc.ArchiveDatasetParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Dataset{}, ErrDatasetNotFound
		}
		return Dataset{}, fmt.Errorf("archive dataset: %w", err)
	}
	return mapDataset(row)
}

func (r *Repository) UpsertDatasetExample(ctx context.Context, params UpsertDatasetExampleParams) (DatasetExample, error) {
	if !params.Status.Valid() {
		return DatasetExample{}, fmt.Errorf("%w: %q", domain.ErrInvalidDatasetExampleStatus, params.Status)
	}
	if !params.Source.Valid() {
		return DatasetExample{}, fmt.Errorf("%w: %q", domain.ErrInvalidDatasetExampleSource, params.Source)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return DatasetExample{}, err
	}
	defer tx.Rollback(ctx)
	row, err := upsertDatasetExampleWithQueries(ctx, r.queries.WithTx(tx), params)
	if err != nil {
		return DatasetExample{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetExample{}, err
	}
	return mapDatasetExample(row)
}

func datasetExampleTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func upsertDatasetExampleWithQueries(ctx context.Context, q *repositorysqlc.Queries, params UpsertDatasetExampleParams) (repositorysqlc.DatasetExample, error) {
	tags := datasetExampleTags(params.Tags)
	var before *repositorysqlc.DatasetExample
	if params.ExternalID != nil && strings.TrimSpace(*params.ExternalID) != "" {
		row, err := q.GetDatasetExampleByExternalID(ctx, repositorysqlc.GetDatasetExampleByExternalIDParams{
			DatasetID:  params.DatasetID,
			ExternalID: params.ExternalID,
		})
		if err == nil {
			before = &row
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return repositorysqlc.DatasetExample{}, fmt.Errorf("get dataset example by external id: %w", err)
		}
	}

	var row repositorysqlc.DatasetExample
	operation := "insert"
	var beforeJSON []byte
	var err error
	if before != nil {
		operation = "update"
		beforeJSON = datasetExampleRevisionJSON(*before)
		row, err = q.UpdateDatasetExample(ctx, repositorysqlc.UpdateDatasetExampleParams{
			ID: before.ID, Input: params.Input, Expected: nullableJSON(params.Expected), Metadata: datasetDefaultJSONObject(params.Metadata),
			Tags: tags, Status: string(params.Status), Source: string(params.Source), SourceRunID: params.SourceRunID,
			SourceTraceID: params.SourceTraceID, SourcePlatform: params.SourcePlatform, ArtifactID: params.ArtifactID,
		})
	} else {
		row, err = q.InsertDatasetExample(ctx, repositorysqlc.InsertDatasetExampleParams{
			DatasetID: params.DatasetID, ExternalID: cleanStringPtr(params.ExternalID), Input: params.Input, Expected: nullableJSON(params.Expected),
			Metadata: datasetDefaultJSONObject(params.Metadata), Tags: tags, Status: string(params.Status), Source: string(params.Source),
			SourceRunID: params.SourceRunID, SourceTraceID: params.SourceTraceID, SourcePlatform: params.SourcePlatform,
			ArtifactID: params.ArtifactID, CreatedBy: params.Actor,
		})
	}
	if err != nil {
		return repositorysqlc.DatasetExample{}, mapDatasetExampleUpsertError(params.Source, err)
	}
	_, err = q.InsertDatasetExampleRevision(ctx, repositorysqlc.InsertDatasetExampleRevisionParams{
		DatasetID: params.DatasetID, ExampleID: &row.ID, Operation: operation, Before: beforeJSON,
		After: datasetExampleRevisionJSON(row), Actor: params.Actor,
	})
	if err != nil {
		return repositorysqlc.DatasetExample{}, fmt.Errorf("insert dataset example revision: %w", err)
	}
	return row, nil
}

func (r *Repository) GetDatasetExampleByID(ctx context.Context, id uuid.UUID) (DatasetExample, error) {
	row, err := r.queries.GetDatasetExampleByID(ctx, repositorysqlc.GetDatasetExampleByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetExample{}, ErrDatasetExampleNotFound
		}
		return DatasetExample{}, fmt.Errorf("get dataset example by id: %w", err)
	}
	return mapDatasetExample(row)
}

func (r *Repository) ListDatasetExamplesByDatasetID(ctx context.Context, params ListDatasetExamplesParams) ([]DatasetExample, error) {
	var status *string
	if params.Status != nil {
		value := string(*params.Status)
		status = &value
	}
	rows, err := r.queries.ListDatasetExamplesByDatasetID(ctx, repositorysqlc.ListDatasetExamplesByDatasetIDParams{
		DatasetID: params.DatasetID, Status: status, ResultLimit: params.Limit, ResultOffset: params.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list dataset examples by dataset id: %w", err)
	}
	out := make([]DatasetExample, 0, len(rows))
	for _, row := range rows {
		example, err := mapDatasetExample(row)
		if err != nil {
			return nil, err
		}
		out = append(out, example)
	}
	return out, nil
}

func (r *Repository) CountDatasetExamplesByDatasetID(ctx context.Context, datasetID uuid.UUID, status *domain.DatasetExampleStatus) (int64, error) {
	var raw *string
	if status != nil {
		value := string(*status)
		raw = &value
	}
	count, err := r.queries.CountDatasetExamplesByDatasetID(ctx, repositorysqlc.CountDatasetExamplesByDatasetIDParams{DatasetID: datasetID, Status: raw})
	if err != nil {
		return 0, fmt.Errorf("count dataset examples by dataset id: %w", err)
	}
	return count, nil
}

func (r *Repository) PatchDatasetExample(ctx context.Context, params PatchDatasetExampleParams) (DatasetExample, error) {
	before, err := r.queries.GetDatasetExampleByID(ctx, repositorysqlc.GetDatasetExampleByIDParams{ID: params.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetExample{}, ErrDatasetExampleNotFound
		}
		return DatasetExample{}, fmt.Errorf("get dataset example by id: %w", err)
	}

	var status *string
	if params.Status != nil {
		if !params.Status.Valid() {
			return DatasetExample{}, fmt.Errorf("%w: %q", domain.ErrInvalidDatasetExampleStatus, *params.Status)
		}
		value := string(*params.Status)
		status = &value
	}
	var source *string
	if params.Source != nil {
		if !params.Source.Valid() {
			return DatasetExample{}, fmt.Errorf("%w: %q", domain.ErrInvalidDatasetExampleSource, *params.Source)
		}
		value := string(*params.Source)
		source = &value
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return DatasetExample{}, err
	}
	defer tx.Rollback(ctx)
	q := r.queries.WithTx(tx)
	row, err := q.PatchDatasetExample(ctx, repositorysqlc.PatchDatasetExampleParams{
		ID: params.ID, Input: nullableJSON(params.Input), Expected: nullableJSON(params.Expected), Metadata: nullableJSON(params.Metadata),
		Tags: params.Tags, Status: status, Source: source, SourceRunID: params.SourceRunID, SourceTraceID: params.SourceTraceID,
		SourcePlatform: params.SourcePlatform, ArtifactID: params.ArtifactID,
	})
	if err != nil {
		return DatasetExample{}, fmt.Errorf("patch dataset example: %w", err)
	}
	_, err = q.InsertDatasetExampleRevision(ctx, repositorysqlc.InsertDatasetExampleRevisionParams{
		DatasetID: row.DatasetID, ExampleID: &row.ID, Operation: "update", Before: datasetExampleRevisionJSON(before),
		After: datasetExampleRevisionJSON(row), Actor: params.Actor,
	})
	if err != nil {
		return DatasetExample{}, fmt.Errorf("insert dataset example revision: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetExample{}, err
	}
	return mapDatasetExample(row)
}

func (r *Repository) CreateDatasetVersion(ctx context.Context, params CreateDatasetVersionParams) (DatasetVersion, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return DatasetVersion{}, err
	}
	defer tx.Rollback(ctx)
	versionRow, err := createDatasetVersionWithQueries(ctx, r.queries.WithTx(tx), params)
	if err != nil {
		return DatasetVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetVersion{}, err
	}
	return mapDatasetVersion(versionRow)
}

func createDatasetVersionWithQueries(ctx context.Context, q *repositorysqlc.Queries, params CreateDatasetVersionParams) (repositorysqlc.DatasetVersion, error) {
	if _, err := q.LockActiveDatasetForVersion(ctx, repositorysqlc.LockActiveDatasetForVersionParams{ID: params.DatasetID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repositorysqlc.DatasetVersion{}, ErrDatasetNotFound
		}
		return repositorysqlc.DatasetVersion{}, fmt.Errorf("lock active dataset for version: %w", err)
	}

	revisions, err := q.ListLatestDatasetExampleRevisions(ctx, repositorysqlc.ListLatestDatasetExampleRevisionsParams{DatasetID: params.DatasetID})
	if err != nil {
		return repositorysqlc.DatasetVersion{}, fmt.Errorf("list latest dataset example revisions: %w", err)
	}
	next, err := q.NextDatasetVersionNumber(ctx, repositorysqlc.NextDatasetVersionNumberParams{DatasetID: params.DatasetID})
	if err != nil {
		return repositorysqlc.DatasetVersion{}, fmt.Errorf("next dataset version number: %w", err)
	}
	version, err := q.CreateDatasetVersion(ctx, repositorysqlc.CreateDatasetVersionParams{
		DatasetID: params.DatasetID, VersionNumber: next, Label: params.Label, ExampleCount: int32(len(revisions)),
		ManifestChecksum: datasetManifestChecksum(revisions), CreatedBy: params.Actor,
	})
	if err != nil {
		return repositorysqlc.DatasetVersion{}, fmt.Errorf("create dataset version: %w", err)
	}
	for _, revision := range revisions {
		if revision.ExampleID == nil {
			continue
		}
		if err := q.InsertDatasetVersionExample(ctx, repositorysqlc.InsertDatasetVersionExampleParams{
			VersionID: version.ID, ExampleID: *revision.ExampleID, RevisionID: revision.ID,
		}); err != nil {
			return repositorysqlc.DatasetVersion{}, fmt.Errorf("insert dataset version example: %w", err)
		}
	}
	return version, nil
}

func (r *Repository) ListDatasetVersionsByDatasetID(ctx context.Context, datasetID uuid.UUID) ([]DatasetVersion, error) {
	rows, err := r.queries.ListDatasetVersionsByDatasetID(ctx, repositorysqlc.ListDatasetVersionsByDatasetIDParams{DatasetID: datasetID})
	if err != nil {
		return nil, fmt.Errorf("list dataset versions by dataset id: %w", err)
	}
	out := make([]DatasetVersion, 0, len(rows))
	for _, row := range rows {
		version, err := mapDatasetVersion(row)
		if err != nil {
			return nil, err
		}
		out = append(out, version)
	}
	return out, nil
}

func (r *Repository) GetDatasetVersionByID(ctx context.Context, id uuid.UUID) (DatasetVersion, error) {
	row, err := r.queries.GetDatasetVersionByID(ctx, repositorysqlc.GetDatasetVersionByIDParams{ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetVersion{}, ErrDatasetVersionNotFound
		}
		return DatasetVersion{}, fmt.Errorf("get dataset version by id: %w", err)
	}
	return mapDatasetVersion(row)
}

func (r *Repository) ListDatasetVersionExamples(ctx context.Context, versionID uuid.UUID) ([]DatasetExample, error) {
	rows, err := r.queries.ListDatasetVersionExamples(ctx, repositorysqlc.ListDatasetVersionExamplesParams{VersionID: versionID})
	if err != nil {
		return nil, fmt.Errorf("list dataset version examples: %w", err)
	}
	out := make([]DatasetExample, 0, len(rows))
	for _, row := range rows {
		example, err := mapDatasetExample(row)
		if err != nil {
			return nil, err
		}
		out = append(out, example)
	}
	return out, nil
}

func mapDataset(row repositorysqlc.Dataset) (Dataset, error) {
	createdAt, err := requiredTime("datasets.created_at", row.CreatedAt)
	if err != nil {
		return Dataset{}, err
	}
	updatedAt, err := requiredTime("datasets.updated_at", row.UpdatedAt)
	if err != nil {
		return Dataset{}, err
	}
	return Dataset{
		ID: row.ID, OrganizationID: row.OrganizationID, WorkspaceID: row.WorkspaceID, Slug: row.Slug, Name: row.Name,
		Description: row.Description, InputSchema: cloneBytes(row.InputSchema), InputSchemaEnforced: row.InputSchemaEnforced,
		DefaultChallengePackVersionID: row.DefaultChallengePackVersionID, CreatedBy: row.CreatedBy, CreatedAt: createdAt,
		UpdatedAt: updatedAt, ArchivedAt: optionalTime(row.ArchivedAt),
	}, nil
}

func mapDatasetListRow(row repositorysqlc.ListDatasetsByWorkspaceIDRow) (Dataset, error) {
	return mapDatasetDetailRow(repositorysqlc.GetDatasetByIDRow{
		ID: row.ID, OrganizationID: row.OrganizationID, WorkspaceID: row.WorkspaceID, Slug: row.Slug, Name: row.Name,
		Description: row.Description, InputSchema: row.InputSchema, InputSchemaEnforced: row.InputSchemaEnforced,
		DefaultChallengePackVersionID: row.DefaultChallengePackVersionID, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt, ArchivedAt: row.ArchivedAt, ActiveExampleCount: row.ActiveExampleCount, VersionCount: row.VersionCount,
	})
}

func mapDatasetDetailRow(row repositorysqlc.GetDatasetByIDRow) (Dataset, error) {
	dataset, err := mapDataset(repositorysqlc.Dataset{
		ID: row.ID, OrganizationID: row.OrganizationID, WorkspaceID: row.WorkspaceID, Slug: row.Slug, Name: row.Name,
		Description: row.Description, InputSchema: row.InputSchema, InputSchemaEnforced: row.InputSchemaEnforced,
		DefaultChallengePackVersionID: row.DefaultChallengePackVersionID, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt, ArchivedAt: row.ArchivedAt,
	})
	if err != nil {
		return Dataset{}, err
	}
	dataset.ActiveExampleCount = int(row.ActiveExampleCount)
	dataset.VersionCount = int(row.VersionCount)
	return dataset, nil
}

func mapDatasetExample(row repositorysqlc.DatasetExample) (DatasetExample, error) {
	status, err := domain.ParseDatasetExampleStatus(row.Status)
	if err != nil {
		return DatasetExample{}, err
	}
	source, err := domain.ParseDatasetExampleSource(row.Source)
	if err != nil {
		return DatasetExample{}, err
	}
	createdAt, err := requiredTime("dataset_examples.created_at", row.CreatedAt)
	if err != nil {
		return DatasetExample{}, err
	}
	updatedAt, err := requiredTime("dataset_examples.updated_at", row.UpdatedAt)
	if err != nil {
		return DatasetExample{}, err
	}
	return DatasetExample{
		ID: row.ID, DatasetID: row.DatasetID, ExternalID: row.ExternalID, Input: cloneBytes(row.Input), Expected: cloneBytes(row.Expected),
		Metadata: datasetDefaultJSONObject(row.Metadata), Tags: row.Tags, Status: status, Source: source, SourceRunID: row.SourceRunID,
		SourceTraceID: row.SourceTraceID, SourcePlatform: row.SourcePlatform, ArtifactID: row.ArtifactID, CreatedBy: row.CreatedBy,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func mapDatasetVersion(row repositorysqlc.DatasetVersion) (DatasetVersion, error) {
	createdAt, err := requiredTime("dataset_versions.created_at", row.CreatedAt)
	if err != nil {
		return DatasetVersion{}, err
	}
	return DatasetVersion{
		ID: row.ID, DatasetID: row.DatasetID, VersionNumber: row.VersionNumber, Label: row.Label,
		ExampleCount: row.ExampleCount, ManifestChecksum: row.ManifestChecksum, CreatedBy: row.CreatedBy, CreatedAt: createdAt,
	}, nil
}

func datasetExampleRevisionJSON(row repositorysqlc.DatasetExample) []byte {
	payload, _ := json.Marshal(map[string]any{
		"id": row.ID, "dataset_id": row.DatasetID, "external_id": row.ExternalID, "input": json.RawMessage(row.Input),
		"expected": nullableRaw(row.Expected), "metadata": json.RawMessage(datasetDefaultJSONObject(row.Metadata)), "tags": row.Tags,
		"status": row.Status, "source": row.Source, "source_run_id": row.SourceRunID, "source_trace_id": row.SourceTraceID,
		"source_platform": row.SourcePlatform, "artifact_id": row.ArtifactID,
	})
	return payload
}

func datasetManifestChecksum(revisions []repositorysqlc.DatasetExampleRevision) string {
	h := sha256.New()
	for _, revision := range revisions {
		h.Write([]byte(revision.ID.String()))
		if revision.ExampleID != nil {
			h.Write([]byte(revision.ExampleID.String()))
		}
		h.Write(revision.After)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func mapDatasetExampleUpsertError(source domain.DatasetExampleSource, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		switch {
		case strings.Contains(pgErr.ConstraintName, "source"):
			return fmt.Errorf("%w: %q", domain.ErrInvalidDatasetExampleSource, source)
		case strings.Contains(pgErr.ConstraintName, "status"):
			return fmt.Errorf("%w: invalid status", domain.ErrInvalidDatasetExampleStatus)
		}
	}
	return fmt.Errorf("upsert dataset example: %w", err)
}

func nullableJSON(raw json.RawMessage) []byte {
	if len(strings.TrimSpace(string(raw))) == 0 || string(raw) == "null" {
		return nil
	}
	return cloneBytes(raw)
}

func datasetDefaultJSONObject(raw json.RawMessage) []byte {
	if len(strings.TrimSpace(string(raw))) == 0 || string(raw) == "null" {
		return []byte(`{}`)
	}
	return cloneBytes(raw)
}

func nullableRaw(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func cloneBytes(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

func cleanStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func isDatasetSlugConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == datasetSlugUniqueIndex
}
