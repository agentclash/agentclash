package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	datasetadapters "github.com/agentclash/agentclash/backend/internal/datasets/adapters"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DatasetRepository interface {
	CreateDataset(context.Context, repository.CreateDatasetParams) (repository.Dataset, error)
	GetDatasetByID(context.Context, uuid.UUID) (repository.Dataset, error)
	ListDatasetsByWorkspaceID(context.Context, uuid.UUID, int32, int32) ([]repository.Dataset, error)
	CountDatasetsByWorkspaceID(context.Context, uuid.UUID) (int64, error)
	PatchDataset(context.Context, repository.PatchDatasetParams) (repository.Dataset, error)
	ArchiveDataset(context.Context, uuid.UUID) (repository.Dataset, error)
	UpsertDatasetExample(context.Context, repository.UpsertDatasetExampleParams) (repository.DatasetExample, error)
	GetDatasetExampleByID(context.Context, uuid.UUID) (repository.DatasetExample, error)
	ListDatasetExamplesByDatasetID(context.Context, repository.ListDatasetExamplesParams) ([]repository.DatasetExample, error)
	CountDatasetExamplesByDatasetID(context.Context, uuid.UUID, *domain.DatasetExampleStatus) (int64, error)
	PatchDatasetExample(context.Context, repository.PatchDatasetExampleParams) (repository.DatasetExample, error)
	CreateDatasetVersion(context.Context, repository.CreateDatasetVersionParams) (repository.DatasetVersion, error)
	ListDatasetVersionsByDatasetID(context.Context, uuid.UUID) ([]repository.DatasetVersion, error)
	GetDatasetVersionByID(context.Context, uuid.UUID) (repository.DatasetVersion, error)
	ListDatasetVersionExamples(context.Context, uuid.UUID) ([]repository.DatasetExample, error)
}

type DatasetService interface {
	CreateDataset(context.Context, Caller, CreateDatasetInput) (repository.Dataset, error)
	ListDatasets(context.Context, Caller, ListDatasetsInput) (ListDatasetsResult, error)
	GetDataset(context.Context, Caller, GetDatasetInput) (repository.Dataset, error)
	PatchDataset(context.Context, Caller, PatchDatasetInput) (repository.Dataset, error)
	DeleteDataset(context.Context, Caller, GetDatasetInput) error
	AddDatasetExample(context.Context, Caller, UpsertDatasetExampleInput) (repository.DatasetExample, error)
	ListDatasetExamples(context.Context, Caller, ListDatasetExamplesInput) (ListDatasetExamplesResult, error)
	PatchDatasetExample(context.Context, Caller, PatchDatasetExampleInput) (repository.DatasetExample, error)
	DeleteDatasetExample(context.Context, Caller, PatchDatasetExampleInput) (repository.DatasetExample, error)
	CreateDatasetVersion(context.Context, Caller, CreateDatasetVersionInput) (repository.DatasetVersion, error)
	ListDatasetVersions(context.Context, Caller, GetDatasetInput) ([]repository.DatasetVersion, error)
	GetDatasetVersion(context.Context, Caller, GetDatasetVersionInput) (repository.DatasetVersion, []repository.DatasetExample, error)
	ImportDataset(context.Context, Caller, DatasetImportInput) (DatasetImportResult, error)
	ExportDataset(context.Context, Caller, DatasetExportInput) (DatasetExportResult, error)
}

type DatasetManager struct {
	authorizer WorkspaceAuthorizer
	repo       DatasetRepository
}

func NewDatasetManager(authorizer WorkspaceAuthorizer, repo DatasetRepository) *DatasetManager {
	return &DatasetManager{authorizer: authorizer, repo: repo}
}

type CreateDatasetInput struct {
	WorkspaceID                   uuid.UUID
	Slug                          string
	Name                          string
	Description                   string
	InputSchema                   json.RawMessage
	InputSchemaEnforced           bool
	DefaultChallengePackVersionID *uuid.UUID
}

type ListDatasetsInput struct {
	WorkspaceID uuid.UUID
	Limit       int32
	Offset      int32
}

type ListDatasetsResult struct {
	Items  []repository.Dataset
	Total  int64
	Limit  int32
	Offset int32
}

type GetDatasetInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
}

type PatchDatasetInput struct {
	WorkspaceID                   uuid.UUID
	DatasetID                     uuid.UUID
	Slug                          *string
	Name                          *string
	Description                   *string
	InputSchema                   json.RawMessage
	InputSchemaEnforced           *bool
	DefaultChallengePackVersionID *uuid.UUID
}

type UpsertDatasetExampleInput struct {
	WorkspaceID    uuid.UUID
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
}

type ListDatasetExamplesInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	Status      *domain.DatasetExampleStatus
	Limit       int32
	Offset      int32
}

type ListDatasetExamplesResult struct {
	Items  []repository.DatasetExample
	Total  int64
	Limit  int32
	Offset int32
}

type PatchDatasetExampleInput struct {
	WorkspaceID    uuid.UUID
	DatasetID      uuid.UUID
	ExampleID      uuid.UUID
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
}

type CreateDatasetVersionInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	Label       *string
}

type GetDatasetVersionInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	VersionID   uuid.UUID
}

type DatasetImportMode string

const (
	DatasetImportModeAdd     DatasetImportMode = "add"
	DatasetImportModeReplace DatasetImportMode = "replace"
)

var ErrInvalidDatasetImportMode = errors.New("invalid dataset import mode")

type DatasetImportInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	Format      string
	Mode        DatasetImportMode
	Mapping     datasetadapters.Mapping
	DryRun      bool
	Data        []byte
}

type DatasetImportResult struct {
	Format        datasetadapters.Format      `json:"format"`
	DryRun        bool                        `json:"dry_run"`
	Mode          DatasetImportMode           `json:"mode"`
	Preview       []datasetadapters.Example   `json:"preview,omitempty"`
	Errors        []datasetadapters.RowError  `json:"errors,omitempty"`
	ImportedCount int                         `json:"imported_count"`
	Version       *repository.DatasetVersion  `json:"version,omitempty"`
	Examples      []repository.DatasetExample `json:"examples,omitempty"`
}

type DatasetExportInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	VersionID   *uuid.UUID
	Format      string
}

type DatasetExportResult struct {
	Format      datasetadapters.Format `json:"format"`
	ContentType string                 `json:"content_type"`
	Filename    string                 `json:"filename"`
	Data        []byte                 `json:"-"`
}

func (m *DatasetManager) CreateDataset(ctx context.Context, caller Caller, input CreateDatasetInput) (repository.Dataset, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageDatasets); err != nil {
		return repository.Dataset{}, err
	}
	return m.repo.CreateDataset(ctx, repository.CreateDatasetParams{
		WorkspaceID: input.WorkspaceID, Slug: input.Slug, Name: input.Name, Description: input.Description,
		InputSchema: input.InputSchema, InputSchemaEnforced: input.InputSchemaEnforced,
		DefaultChallengePackVersionID: input.DefaultChallengePackVersionID, CreatedBy: caller.UserID,
	})
}

func (m *DatasetManager) ListDatasets(ctx context.Context, caller Caller, input ListDatasetsInput) (ListDatasetsResult, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return ListDatasetsResult{}, err
	}
	items, err := m.repo.ListDatasetsByWorkspaceID(ctx, input.WorkspaceID, input.Limit, input.Offset)
	if err != nil {
		return ListDatasetsResult{}, err
	}
	total, err := m.repo.CountDatasetsByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return ListDatasetsResult{}, err
	}
	return ListDatasetsResult{Items: items, Total: total, Limit: input.Limit, Offset: input.Offset}, nil
}

func (m *DatasetManager) GetDataset(ctx context.Context, caller Caller, input GetDatasetInput) (repository.Dataset, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return repository.Dataset{}, err
	}
	dataset, err := m.repo.GetDatasetByID(ctx, input.DatasetID)
	if err != nil {
		return repository.Dataset{}, err
	}
	if dataset.WorkspaceID != input.WorkspaceID {
		return repository.Dataset{}, ErrForbidden
	}
	if dataset.ArchivedAt != nil {
		return repository.Dataset{}, repository.ErrDatasetNotFound
	}
	return dataset, nil
}

func (m *DatasetManager) PatchDataset(ctx context.Context, caller Caller, input PatchDatasetInput) (repository.Dataset, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageDatasets); err != nil {
		return repository.Dataset{}, err
	}
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.Dataset{}, err
	}
	return m.repo.PatchDataset(ctx, repository.PatchDatasetParams{
		ID: input.DatasetID, Slug: input.Slug, Name: input.Name, Description: input.Description, InputSchema: input.InputSchema,
		InputSchemaEnforced: input.InputSchemaEnforced, DefaultChallengePackVersionID: input.DefaultChallengePackVersionID,
	})
}

func (m *DatasetManager) DeleteDataset(ctx context.Context, caller Caller, input GetDatasetInput) error {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageDatasets); err != nil {
		return err
	}
	if _, err := m.GetDataset(ctx, caller, input); err != nil {
		return err
	}
	_, err := m.repo.ArchiveDataset(ctx, input.DatasetID)
	return err
}

func (m *DatasetManager) AddDatasetExample(ctx context.Context, caller Caller, input UpsertDatasetExampleInput) (repository.DatasetExample, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageDatasets); err != nil {
		return repository.DatasetExample{}, err
	}
	dataset, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID})
	if err != nil {
		return repository.DatasetExample{}, err
	}
	if dataset.InputSchemaEnforced {
		if err := domain.ValidateDatasetInputAgainstSchema(dataset.InputSchema, input.Input); err != nil {
			return repository.DatasetExample{}, err
		}
	}
	return m.repo.UpsertDatasetExample(ctx, repository.UpsertDatasetExampleParams{
		DatasetID: input.DatasetID, ExternalID: input.ExternalID, Input: input.Input, Expected: input.Expected, Metadata: input.Metadata,
		Tags: input.Tags, Status: input.Status, Source: input.Source, SourceRunID: input.SourceRunID, SourceTraceID: input.SourceTraceID,
		SourcePlatform: input.SourcePlatform, ArtifactID: input.ArtifactID, Actor: caller.UserID,
	})
}

func (m *DatasetManager) ListDatasetExamples(ctx context.Context, caller Caller, input ListDatasetExamplesInput) (ListDatasetExamplesResult, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return ListDatasetExamplesResult{}, err
	}
	items, err := m.repo.ListDatasetExamplesByDatasetID(ctx, repository.ListDatasetExamplesParams{
		DatasetID: input.DatasetID, Status: input.Status, Limit: input.Limit, Offset: input.Offset,
	})
	if err != nil {
		return ListDatasetExamplesResult{}, err
	}
	total, err := m.repo.CountDatasetExamplesByDatasetID(ctx, input.DatasetID, input.Status)
	if err != nil {
		return ListDatasetExamplesResult{}, err
	}
	return ListDatasetExamplesResult{Items: items, Total: total, Limit: input.Limit, Offset: input.Offset}, nil
}

func (m *DatasetManager) PatchDatasetExample(ctx context.Context, caller Caller, input PatchDatasetExampleInput) (repository.DatasetExample, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageDatasets); err != nil {
		return repository.DatasetExample{}, err
	}
	dataset, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID})
	if err != nil {
		return repository.DatasetExample{}, err
	}
	example, err := m.repo.GetDatasetExampleByID(ctx, input.ExampleID)
	if err != nil {
		return repository.DatasetExample{}, err
	}
	if example.DatasetID != input.DatasetID {
		return repository.DatasetExample{}, ErrForbidden
	}
	if dataset.InputSchemaEnforced && len(input.Input) > 0 {
		if err := domain.ValidateDatasetInputAgainstSchema(dataset.InputSchema, input.Input); err != nil {
			return repository.DatasetExample{}, err
		}
	}
	return m.repo.PatchDatasetExample(ctx, repository.PatchDatasetExampleParams{
		ID: input.ExampleID, Input: input.Input, Expected: input.Expected, Metadata: input.Metadata, Tags: input.Tags,
		Status: input.Status, Source: input.Source, SourceRunID: input.SourceRunID, SourceTraceID: input.SourceTraceID,
		SourcePlatform: input.SourcePlatform, ArtifactID: input.ArtifactID, Actor: caller.UserID,
	})
}

func (m *DatasetManager) DeleteDatasetExample(ctx context.Context, caller Caller, input PatchDatasetExampleInput) (repository.DatasetExample, error) {
	status := domain.DatasetExampleStatusArchived
	input.Status = &status
	return m.PatchDatasetExample(ctx, caller, input)
}

func (m *DatasetManager) CreateDatasetVersion(ctx context.Context, caller Caller, input CreateDatasetVersionInput) (repository.DatasetVersion, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionManageDatasets); err != nil {
		return repository.DatasetVersion{}, err
	}
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.DatasetVersion{}, err
	}
	return m.repo.CreateDatasetVersion(ctx, repository.CreateDatasetVersionParams{DatasetID: input.DatasetID, Label: input.Label, Actor: caller.UserID})
}

func (m *DatasetManager) ListDatasetVersions(ctx context.Context, caller Caller, input GetDatasetInput) ([]repository.DatasetVersion, error) {
	if _, err := m.GetDataset(ctx, caller, input); err != nil {
		return nil, err
	}
	return m.repo.ListDatasetVersionsByDatasetID(ctx, input.DatasetID)
}

func (m *DatasetManager) GetDatasetVersion(ctx context.Context, caller Caller, input GetDatasetVersionInput) (repository.DatasetVersion, []repository.DatasetExample, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return repository.DatasetVersion{}, nil, err
	}
	version, err := m.repo.GetDatasetVersionByID(ctx, input.VersionID)
	if err != nil {
		return repository.DatasetVersion{}, nil, err
	}
	if version.DatasetID != input.DatasetID {
		return repository.DatasetVersion{}, nil, ErrForbidden
	}
	examples, err := m.repo.ListDatasetVersionExamples(ctx, input.VersionID)
	return version, examples, err
}

func (m *DatasetManager) ImportDataset(ctx context.Context, caller Caller, input DatasetImportInput) (DatasetImportResult, error) {
	if input.Mode == "" {
		input.Mode = DatasetImportModeAdd
	}
	if input.Mode != DatasetImportModeAdd && input.Mode != DatasetImportModeReplace {
		return DatasetImportResult{}, ErrInvalidDatasetImportMode
	}
	dataset, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID})
	if err != nil {
		return DatasetImportResult{}, err
	}
	normalized, err := datasetadapters.Import(input.Format, input.Data, input.Mapping)
	if err != nil {
		return DatasetImportResult{}, err
	}
	result := DatasetImportResult{Format: normalized.Format, DryRun: input.DryRun, Mode: input.Mode, Preview: normalized.Examples, Errors: normalized.Errors}
	for i, example := range normalized.Examples {
		if dataset.InputSchemaEnforced {
			if err := domain.ValidateDatasetInputAgainstSchema(dataset.InputSchema, example.Input); err != nil {
				result.Errors = append(result.Errors, datasetadapters.RowError{Row: i + 1, Field: "input", Message: err.Error()})
			}
		}
	}
	if len(result.Errors) > 0 || input.DryRun {
		return result, nil
	}
	if input.Mode == DatasetImportModeReplace {
		if err := m.archiveExamplesMissingFromImport(ctx, caller, input, normalized.Examples); err != nil {
			return DatasetImportResult{}, err
		}
	}
	imported := make([]repository.DatasetExample, 0, len(normalized.Examples))
	source := domain.DatasetExampleSourceImport
	for _, example := range normalized.Examples {
		upserted, err := m.repo.UpsertDatasetExample(ctx, repository.UpsertDatasetExampleParams{
			DatasetID: input.DatasetID, ExternalID: example.ExternalID, Input: example.Input, Expected: example.Expected, Metadata: example.Metadata,
			Tags: example.Tags, Status: domain.DatasetExampleStatusActive, Source: source, Actor: caller.UserID,
		})
		if err != nil {
			return DatasetImportResult{}, err
		}
		imported = append(imported, upserted)
	}
	version, err := m.repo.CreateDatasetVersion(ctx, repository.CreateDatasetVersionParams{
		DatasetID: input.DatasetID,
		Label:     datasetStringPtr("import:" + string(normalized.Format)),
		Actor:     caller.UserID,
	})
	if err != nil {
		return DatasetImportResult{}, err
	}
	result.ImportedCount = len(imported)
	result.Version = &version
	result.Examples = imported
	return result, nil
}

func (m *DatasetManager) archiveExamplesMissingFromImport(ctx context.Context, caller Caller, input DatasetImportInput, examples []datasetadapters.Example) error {
	incoming := make(map[string]struct{})
	for _, example := range examples {
		if example.ExternalID != nil && strings.TrimSpace(*example.ExternalID) != "" {
			incoming[strings.TrimSpace(*example.ExternalID)] = struct{}{}
		}
	}
	active := domain.DatasetExampleStatusActive
	existing, err := m.repo.ListDatasetExamplesByDatasetID(ctx, repository.ListDatasetExamplesParams{DatasetID: input.DatasetID, Status: &active, Limit: 100000, Offset: 0})
	if err != nil {
		return err
	}
	archived := domain.DatasetExampleStatusArchived
	for _, example := range existing {
		if example.ExternalID != nil {
			if _, keep := incoming[strings.TrimSpace(*example.ExternalID)]; keep {
				continue
			}
		}
		if _, err := m.repo.PatchDatasetExample(ctx, repository.PatchDatasetExampleParams{ID: example.ID, Status: &archived, Actor: caller.UserID}); err != nil {
			return err
		}
	}
	return nil
}

func (m *DatasetManager) ExportDataset(ctx context.Context, caller Caller, input DatasetExportInput) (DatasetExportResult, error) {
	if _, err := m.GetDataset(ctx, caller, GetDatasetInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID}); err != nil {
		return DatasetExportResult{}, err
	}
	var examples []repository.DatasetExample
	if input.VersionID != nil {
		version, versionExamples, err := m.GetDatasetVersion(ctx, caller, GetDatasetVersionInput{WorkspaceID: input.WorkspaceID, DatasetID: input.DatasetID, VersionID: *input.VersionID})
		if err != nil {
			return DatasetExportResult{}, err
		}
		if version.DatasetID != input.DatasetID {
			return DatasetExportResult{}, ErrForbidden
		}
		examples = versionExamples
	} else {
		active := domain.DatasetExampleStatusActive
		current, err := m.repo.ListDatasetExamplesByDatasetID(ctx, repository.ListDatasetExamplesParams{DatasetID: input.DatasetID, Status: &active, Limit: 100000, Offset: 0})
		if err != nil {
			return DatasetExportResult{}, err
		}
		examples = current
	}
	exportExamples := make([]datasetadapters.Example, 0, len(examples))
	for _, example := range examples {
		exportExamples = append(exportExamples, datasetadapters.Example{
			ExternalID: example.ExternalID,
			Input:      example.Input,
			Expected:   example.Expected,
			Metadata:   example.Metadata,
			Tags:       example.Tags,
		})
	}
	data, contentType, err := datasetadapters.Export(input.Format, exportExamples)
	if err != nil {
		return DatasetExportResult{}, err
	}
	format, _ := datasetadapters.Import(input.Format, nil, datasetadapters.Mapping{})
	ext := "jsonl"
	if strings.Contains(contentType, "csv") {
		ext = "csv"
	}
	return DatasetExportResult{Format: format.Format, ContentType: contentType, Filename: "dataset-" + input.DatasetID.String() + "." + ext, Data: data}, nil
}

type datasetListResponse struct {
	Items  []repository.Dataset `json:"items"`
	Total  int64                `json:"total"`
	Limit  int32                `json:"limit"`
	Offset int32                `json:"offset"`
}

type datasetExampleListResponse struct {
	Items  []repository.DatasetExample `json:"items"`
	Total  int64                       `json:"total"`
	Limit  int32                       `json:"limit"`
	Offset int32                       `json:"offset"`
}

type datasetVersionDetailResponse struct {
	Version  repository.DatasetVersion   `json:"version"`
	Examples []repository.DatasetExample `json:"examples"`
}

func createDatasetHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := datasetRequestContext(w, r)
		if !ok {
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var req struct {
			Slug                          string          `json:"slug"`
			Name                          string          `json:"name"`
			Description                   string          `json:"description"`
			InputSchema                   json.RawMessage `json:"input_schema"`
			InputSchemaEnforced           bool            `json:"input_schema_enforced"`
			DefaultChallengePackVersionID *uuid.UUID      `json:"default_challenge_pack_version_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(req.Slug) == "" || strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "slug and name are required")
			return
		}
		dataset, err := service.CreateDataset(r.Context(), caller, CreateDatasetInput{
			WorkspaceID: workspaceID, Slug: req.Slug, Name: req.Name, Description: req.Description, InputSchema: req.InputSchema,
			InputSchemaEnforced: req.InputSchemaEnforced, DefaultChallengePackVersionID: req.DefaultChallengePackVersionID,
		})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, dataset)
	}
}

func listDatasetsHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := datasetRequestContext(w, r)
		if !ok {
			return
		}
		limit, offset, ok := paginationFromRequest(w, r)
		if !ok {
			return
		}
		result, err := service.ListDatasets(r.Context(), caller, ListDatasetsInput{WorkspaceID: workspaceID, Limit: limit, Offset: offset})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, datasetListResponse(result))
	}
}

func getDatasetHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		dataset, err := service.GetDataset(r.Context(), caller, GetDatasetInput{WorkspaceID: workspaceID, DatasetID: datasetID})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, dataset)
	}
}

func patchDatasetHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		var req struct {
			Slug                          *string         `json:"slug"`
			Name                          *string         `json:"name"`
			Description                   *string         `json:"description"`
			InputSchema                   json.RawMessage `json:"input_schema"`
			InputSchemaEnforced           *bool           `json:"input_schema_enforced"`
			DefaultChallengePackVersionID *uuid.UUID      `json:"default_challenge_pack_version_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		dataset, err := service.PatchDataset(r.Context(), caller, PatchDatasetInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, Slug: req.Slug, Name: req.Name, Description: req.Description,
			InputSchema: req.InputSchema, InputSchemaEnforced: req.InputSchemaEnforced, DefaultChallengePackVersionID: req.DefaultChallengePackVersionID,
		})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, dataset)
	}
}

func deleteDatasetHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		if err := service.DeleteDataset(r.Context(), caller, GetDatasetInput{WorkspaceID: workspaceID, DatasetID: datasetID}); err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func addDatasetExampleHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		req, ok := decodeDatasetExampleRequest(w, r)
		if !ok {
			return
		}
		example, err := service.AddDatasetExample(r.Context(), caller, req.toUpsertInput(workspaceID, datasetID))
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, example)
	}
}

func listDatasetExamplesHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		limit, offset, ok := paginationFromRequest(w, r)
		if !ok {
			return
		}
		var status *domain.DatasetExampleStatus
		if raw := r.URL.Query().Get("status"); raw != "" {
			parsed, err := domain.ParseDatasetExampleStatus(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "status must be active, archived, or muted")
				return
			}
			status = &parsed
		}
		result, err := service.ListDatasetExamples(r.Context(), caller, ListDatasetExamplesInput{WorkspaceID: workspaceID, DatasetID: datasetID, Status: status, Limit: limit, Offset: offset})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, datasetExampleListResponse(result))
	}
}

func patchDatasetExampleHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, exampleID, ok := datasetExamplePathContext(w, r)
		if !ok {
			return
		}
		req, ok := decodeDatasetExampleRequest(w, r)
		if !ok {
			return
		}
		example, err := service.PatchDatasetExample(r.Context(), caller, req.toPatchInput(workspaceID, datasetID, exampleID))
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, example)
	}
}

func deleteDatasetExampleHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, exampleID, ok := datasetExamplePathContext(w, r)
		if !ok {
			return
		}
		example, err := service.DeleteDatasetExample(r.Context(), caller, PatchDatasetExampleInput{WorkspaceID: workspaceID, DatasetID: datasetID, ExampleID: exampleID})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, example)
	}
}

func createDatasetVersionHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		var req struct {
			Label *string `json:"label"`
		}
		if r.Body != nil && r.ContentLength != 0 {
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
				return
			}
		}
		version, err := service.CreateDatasetVersion(r.Context(), caller, CreateDatasetVersionInput{WorkspaceID: workspaceID, DatasetID: datasetID, Label: req.Label})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, version)
	}
}

func listDatasetVersionsHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		versions, err := service.ListDatasetVersions(r.Context(), caller, GetDatasetInput{WorkspaceID: workspaceID, DatasetID: datasetID})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": versions})
	}
}

func getDatasetVersionHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		versionID, err := uuid.Parse(chi.URLParam(r, "versionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_version_id", "version ID is malformed")
			return
		}
		version, examples, err := service.GetDatasetVersion(r.Context(), caller, GetDatasetVersionInput{WorkspaceID: workspaceID, DatasetID: datasetID, VersionID: versionID})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, datasetVersionDetailResponse{Version: version, Examples: examples})
	}
}

func importDatasetHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		req, ok := decodeDatasetImportRequest(w, r)
		if !ok {
			return
		}
		result, err := service.ImportDataset(r.Context(), caller, DatasetImportInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, Format: req.Format, Mode: req.Mode, Mapping: req.Mapping, DryRun: req.DryRun, Data: req.Data,
		})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func exportDatasetHandler(logger *slog.Logger, service DatasetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
		if !ok {
			return
		}
		var versionID *uuid.UUID
		if raw := firstQueryValue(r, "version_id", "version"); raw != "" {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_version_id", "version ID is malformed")
				return
			}
			versionID = &parsed
		}
		result, err := service.ExportDataset(r.Context(), caller, DatasetExportInput{
			WorkspaceID: workspaceID, DatasetID: datasetID, VersionID: versionID, Format: firstQueryValue(r, "format"),
		})
		if err != nil {
			handleDatasetError(w, logger, err)
			return
		}
		w.Header().Set("Content-Type", result.ContentType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+result.Filename+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(result.Data)
	}
}

type datasetImportRequest struct {
	Format  string
	Mode    DatasetImportMode
	Mapping datasetadapters.Mapping
	DryRun  bool
	Data    []byte
}

func decodeDatasetImportRequest(w http.ResponseWriter, r *http.Request) (datasetImportRequest, bool) {
	req := datasetImportRequest{
		Format: firstQueryValue(r, "format"),
		Mode:   DatasetImportMode(firstQueryValue(r, "mode")),
		DryRun: parseBoolQuery(r, "dry_run"),
	}
	if req.Mode == "" {
		req.Mode = DatasetImportModeAdd
	}
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "multipart body is invalid")
			return req, false
		}
		req.Format = datasetFirstNonEmpty(req.Format, r.FormValue("format"))
		req.Mode = DatasetImportMode(datasetFirstNonEmpty(string(req.Mode), r.FormValue("mode")))
		req.DryRun = req.DryRun || parseBoolString(r.FormValue("dry_run"))
		if raw := datasetFirstNonEmpty(r.FormValue("mapping"), r.FormValue("map")); raw != "" {
			if err := json.Unmarshal([]byte(raw), &req.Mapping); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_mapping", "mapping must be valid JSON")
				return req, false
			}
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing_file", "multipart form must include file")
			return req, false
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_file", "could not read upload")
			return req, false
		}
		req.Data = data
		return req, true
	}
	if raw := firstQueryValue(r, "mapping", "map"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &req.Mapping); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_mapping", "mapping must be valid JSON")
			return req, false
		}
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "could not read request body")
		return req, false
	}
	req.Data = data
	return req, true
}

type datasetExampleRequest struct {
	ExternalID     *string         `json:"external_id"`
	Input          json.RawMessage `json:"input"`
	Expected       json.RawMessage `json:"expected"`
	Metadata       json.RawMessage `json:"metadata"`
	Tags           []string        `json:"tags"`
	Status         *string         `json:"status"`
	Source         *string         `json:"source"`
	SourceRunID    *uuid.UUID      `json:"source_run_id"`
	SourceTraceID  *string         `json:"source_trace_id"`
	SourcePlatform *string         `json:"source_platform"`
	ArtifactID     *uuid.UUID      `json:"artifact_id"`
}

func decodeDatasetExampleRequest(w http.ResponseWriter, r *http.Request) (datasetExampleRequest, bool) {
	var req datasetExampleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return req, false
	}
	if req.Status != nil {
		if _, err := domain.ParseDatasetExampleStatus(*req.Status); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "status must be active, archived, or muted")
			return req, false
		}
	}
	if req.Source != nil {
		if _, err := domain.ParseDatasetExampleSource(*req.Source); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "source must be manual, import, trace, synthetic, or promotion")
			return req, false
		}
	}
	return req, true
}

func (r datasetExampleRequest) toUpsertInput(workspaceID, datasetID uuid.UUID) UpsertDatasetExampleInput {
	status := domain.DatasetExampleStatusActive
	if r.Status != nil {
		status, _ = domain.ParseDatasetExampleStatus(*r.Status)
	}
	source := domain.DatasetExampleSourceManual
	if r.Source != nil {
		source, _ = domain.ParseDatasetExampleSource(*r.Source)
	}
	return UpsertDatasetExampleInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, ExternalID: r.ExternalID, Input: r.Input, Expected: r.Expected, Metadata: r.Metadata,
		Tags: r.Tags, Status: status, Source: source, SourceRunID: r.SourceRunID, SourceTraceID: r.SourceTraceID, SourcePlatform: r.SourcePlatform, ArtifactID: r.ArtifactID,
	}
}

func (r datasetExampleRequest) toPatchInput(workspaceID, datasetID, exampleID uuid.UUID) PatchDatasetExampleInput {
	var status *domain.DatasetExampleStatus
	if r.Status != nil {
		parsed, _ := domain.ParseDatasetExampleStatus(*r.Status)
		status = &parsed
	}
	var source *domain.DatasetExampleSource
	if r.Source != nil {
		parsed, _ := domain.ParseDatasetExampleSource(*r.Source)
		source = &parsed
	}
	return PatchDatasetExampleInput{
		WorkspaceID: workspaceID, DatasetID: datasetID, ExampleID: exampleID, Input: r.Input, Expected: r.Expected, Metadata: r.Metadata,
		Tags: r.Tags, Status: status, Source: source, SourceRunID: r.SourceRunID, SourceTraceID: r.SourceTraceID, SourcePlatform: r.SourcePlatform, ArtifactID: r.ArtifactID,
	}
}

func datasetRequestContext(w http.ResponseWriter, r *http.Request) (Caller, uuid.UUID, bool) {
	caller, err := CallerFromContext(r.Context())
	if err != nil {
		writeAuthzError(w, err)
		return Caller{}, uuid.Nil, false
	}
	workspaceID, err := workspaceIDFromURLParam("workspaceID")(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_workspace_id", "workspace ID is malformed")
		return Caller{}, uuid.Nil, false
	}
	return caller, workspaceID, true
}

func datasetPathContext(w http.ResponseWriter, r *http.Request) (Caller, uuid.UUID, uuid.UUID, bool) {
	caller, workspaceID, ok := datasetRequestContext(w, r)
	if !ok {
		return Caller{}, uuid.Nil, uuid.Nil, false
	}
	datasetID, err := uuid.Parse(chi.URLParam(r, "datasetID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_dataset_id", "dataset ID is malformed")
		return Caller{}, uuid.Nil, uuid.Nil, false
	}
	return caller, workspaceID, datasetID, true
}

func datasetExamplePathContext(w http.ResponseWriter, r *http.Request) (Caller, uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	caller, workspaceID, datasetID, ok := datasetPathContext(w, r)
	if !ok {
		return Caller{}, uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	exampleID, err := uuid.Parse(chi.URLParam(r, "exampleID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_example_id", "example ID is malformed")
		return Caller{}, uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return caller, workspaceID, datasetID, exampleID, true
}

func paginationFromRequest(w http.ResponseWriter, r *http.Request) (int32, int32, bool) {
	limit := int32(50)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "limit must be a positive integer")
			return 0, 0, false
		}
		limit = int32(parsed)
	}
	if limit > 100 {
		limit = 100
	}
	offset := int32(0)
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "offset must be a non-negative integer")
			return 0, 0, false
		}
		offset = int32(parsed)
	}
	return limit, offset, true
}

func handleDatasetError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		writeAuthzError(w, err)
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "not allowed")
	case errors.Is(err, repository.ErrDatasetNotFound), errors.Is(err, repository.ErrDatasetExampleNotFound), errors.Is(err, repository.ErrDatasetVersionNotFound):
		writeError(w, http.StatusNotFound, "not_found", "dataset resource not found")
	case errors.Is(err, repository.ErrDatasetSlugConflict):
		writeError(w, http.StatusConflict, "slug_conflict", "dataset slug already exists in this workspace")
	case errors.Is(err, ErrInvalidDatasetImportMode):
		writeError(w, http.StatusBadRequest, "validation_error", "mode must be add or replace")
	case errors.Is(err, domain.ErrInvalidDatasetExampleStatus), errors.Is(err, domain.ErrInvalidDatasetExampleSource), errors.Is(err, domain.ErrInvalidDatasetInputSchema), errors.Is(err, domain.ErrDatasetInputSchemaViolation):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	default:
		logger.Error("dataset request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func firstQueryValue(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(r.URL.Query().Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func datasetFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseBoolQuery(r *http.Request, name string) bool {
	return parseBoolString(r.URL.Query().Get(name))
}

func parseBoolString(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func datasetStringPtr(value string) *string {
	return &value
}

type noopDatasetService struct{}

func (noopDatasetService) CreateDataset(context.Context, Caller, CreateDatasetInput) (repository.Dataset, error) {
	return repository.Dataset{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) ListDatasets(context.Context, Caller, ListDatasetsInput) (ListDatasetsResult, error) {
	return ListDatasetsResult{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) GetDataset(context.Context, Caller, GetDatasetInput) (repository.Dataset, error) {
	return repository.Dataset{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) PatchDataset(context.Context, Caller, PatchDatasetInput) (repository.Dataset, error) {
	return repository.Dataset{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) DeleteDataset(context.Context, Caller, GetDatasetInput) error {
	return errors.New("dataset service is not configured")
}
func (noopDatasetService) AddDatasetExample(context.Context, Caller, UpsertDatasetExampleInput) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) ListDatasetExamples(context.Context, Caller, ListDatasetExamplesInput) (ListDatasetExamplesResult, error) {
	return ListDatasetExamplesResult{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) PatchDatasetExample(context.Context, Caller, PatchDatasetExampleInput) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) DeleteDatasetExample(context.Context, Caller, PatchDatasetExampleInput) (repository.DatasetExample, error) {
	return repository.DatasetExample{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) CreateDatasetVersion(context.Context, Caller, CreateDatasetVersionInput) (repository.DatasetVersion, error) {
	return repository.DatasetVersion{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) ListDatasetVersions(context.Context, Caller, GetDatasetInput) ([]repository.DatasetVersion, error) {
	return nil, errors.New("dataset service is not configured")
}
func (noopDatasetService) GetDatasetVersion(context.Context, Caller, GetDatasetVersionInput) (repository.DatasetVersion, []repository.DatasetExample, error) {
	return repository.DatasetVersion{}, nil, errors.New("dataset service is not configured")
}
func (noopDatasetService) ImportDataset(context.Context, Caller, DatasetImportInput) (DatasetImportResult, error) {
	return DatasetImportResult{}, errors.New("dataset service is not configured")
}
func (noopDatasetService) ExportDataset(context.Context, Caller, DatasetExportInput) (DatasetExportResult, error) {
	return DatasetExportResult{}, errors.New("dataset service is not configured")
}
