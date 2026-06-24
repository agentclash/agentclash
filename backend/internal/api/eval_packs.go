package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/evalpack"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type EvalPackReadRepository interface {
	ListVisibleEvalPacks(ctx context.Context, workspaceID uuid.UUID) ([]repository.EvalPackSummary, error)
	WorkspacePublicPacksEnabled(ctx context.Context, workspaceID uuid.UUID) (bool, error)
	ListRunnableChallengePVersionsByPackID(ctx context.Context, evalPackID uuid.UUID) ([]repository.EvalPackVersionSummary, error)
	GetRunnableEvalPackVersionByID(ctx context.Context, id uuid.UUID) (repository.RunnableEvalPackVersion, error)
	ListChallengeInputSetsByVersionID(ctx context.Context, evalPackVersionID uuid.UUID) ([]repository.ChallengeInputSetSummary, error)
}

type EvalPackReadService interface {
	ListEvalPacks(ctx context.Context) (ListEvalPacksResult, error)
	ListChallengeInputSets(ctx context.Context, evalPackVersionID uuid.UUID) (ListChallengeInputSetsResult, error)
}

type ListEvalPacksResult struct {
	Packs []EvalPackWithVersions
}

type ListChallengeInputSetsResult struct {
	InputSets []repository.ChallengeInputSetSummary
}

type EvalPackWithVersions struct {
	Pack     repository.EvalPackSummary
	Versions []repository.EvalPackVersionSummary
}

type EvalPackReadManager struct {
	repo EvalPackReadRepository
}

func NewEvalPackReadManager(repo EvalPackReadRepository) *EvalPackReadManager {
	return &EvalPackReadManager{
		repo: repo,
	}
}

func (m *EvalPackReadManager) ListEvalPacks(ctx context.Context) (ListEvalPacksResult, error) {
	workspaceID, err := WorkspaceIDFromContext(ctx)
	if err != nil {
		return ListEvalPacksResult{}, err
	}

	packs, err := m.repo.ListVisibleEvalPacks(ctx, workspaceID)
	if err != nil {
		return ListEvalPacksResult{}, fmt.Errorf("list eval packs: %w", err)
	}

	result := make([]EvalPackWithVersions, 0, len(packs))
	for _, pack := range packs {
		versions, versionsErr := m.repo.ListRunnableChallengePVersionsByPackID(ctx, pack.ID)
		if versionsErr != nil {
			return ListEvalPacksResult{}, fmt.Errorf("list runnable versions for pack %s: %w", pack.ID, versionsErr)
		}

		result = append(result, EvalPackWithVersions{
			Pack:     pack,
			Versions: versions,
		})
	}

	return ListEvalPacksResult{
		Packs: result,
	}, nil
}

func (m *EvalPackReadManager) ListChallengeInputSets(ctx context.Context, evalPackVersionID uuid.UUID) (ListChallengeInputSetsResult, error) {
	workspaceID, err := WorkspaceIDFromContext(ctx)
	if err != nil {
		return ListChallengeInputSetsResult{}, err
	}

	version, err := m.repo.GetRunnableEvalPackVersionByID(ctx, evalPackVersionID)
	if err != nil {
		return ListChallengeInputSetsResult{}, err
	}
	if version.WorkspaceID != nil && *version.WorkspaceID != workspaceID {
		return ListChallengeInputSetsResult{}, repository.ErrEvalPackVersionNotFound
	}
	if version.WorkspaceID == nil {
		enabled, accessErr := m.repo.WorkspacePublicPacksEnabled(ctx, workspaceID)
		if accessErr != nil {
			return ListChallengeInputSetsResult{}, accessErr
		}
		if !enabled {
			return ListChallengeInputSetsResult{}, repository.ErrEvalPackVersionNotFound
		}
	}

	inputSets, err := m.repo.ListChallengeInputSetsByVersionID(ctx, evalPackVersionID)
	if err != nil {
		return ListChallengeInputSetsResult{}, fmt.Errorf("list challenge input sets for version %s: %w", evalPackVersionID, err)
	}

	return ListChallengeInputSetsResult{
		InputSets: inputSets,
	}, nil
}

type EvalPackAuthoringRepository interface {
	GetArtifactByID(ctx context.Context, artifactID uuid.UUID) (repository.Artifact, error)
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	PublishEvalPackBundle(ctx context.Context, params repository.PublishEvalPackBundleParams) (repository.PublishedEvalPack, error)
	GetWorkspaceEvalPackVersionBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (uuid.UUID, uuid.UUID, bool, error)
}

type EvalPackAuthoringService interface {
	ValidateBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (ValidateEvalPackResponse, error)
	PublishBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (PublishEvalPackResponse, error)
	InstantiateCatalogPack(ctx context.Context, workspaceID uuid.UUID, slug string) (InstantiateCatalogPackResponse, error)
}

type EvalPackAuthoringManager struct {
	repo  EvalPackAuthoringRepository
	store storage.Store
}

type ValidateEvalPackResponse struct {
	Valid  bool                    `json:"valid"`
	Errors []validationErrorDetail `json:"errors"`
}

type PublishEvalPackResponse struct {
	EvalPackID        uuid.UUID   `json:"eval_pack_id"`
	EvalPackVersionID uuid.UUID   `json:"eval_pack_version_id"`
	EvaluationSpecID       uuid.UUID   `json:"evaluation_spec_id"`
	InputSetIDs            []uuid.UUID `json:"input_set_ids"`
	BundleArtifactID       *uuid.UUID  `json:"bundle_artifact_id,omitempty"`
}

type EvalPackAuthoringValidationError struct {
	Errors []validationErrorDetail
}

func (e EvalPackAuthoringValidationError) Error() string {
	return "eval pack bundle has validation errors"
}

func NewEvalPackAuthoringManager(repo EvalPackAuthoringRepository, store storage.Store) *EvalPackAuthoringManager {
	return &EvalPackAuthoringManager{repo: repo, store: store}
}

func (m *EvalPackAuthoringManager) ValidateBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (ValidateEvalPackResponse, error) {
	bundle, err := evalpack.ParseYAML(bundleYAML)
	if err != nil {
		if validationErrs, ok := err.(evalpack.ValidationErrors); ok {
			return ValidateEvalPackResponse{
				Valid:  false,
				Errors: mapEvalPackValidationErrors(validationErrs),
			}, nil
		}
		return ValidateEvalPackResponse{}, err
	}

	validationErrs, err := m.validateStoredAssetReferences(ctx, workspaceID, bundle)
	if err != nil {
		return ValidateEvalPackResponse{}, err
	}
	if len(validationErrs) > 0 {
		return ValidateEvalPackResponse{
			Valid:  false,
			Errors: mapEvalPackValidationErrors(validationErrs),
		}, nil
	}

	return ValidateEvalPackResponse{Valid: true}, nil
}

func (m *EvalPackAuthoringManager) PublishBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (PublishEvalPackResponse, error) {
	bundle, err := evalpack.ParseYAML(bundleYAML)
	if err != nil {
		if validationErrs, ok := err.(evalpack.ValidationErrors); ok {
			return PublishEvalPackResponse{}, EvalPackAuthoringValidationError{
				Errors: mapEvalPackValidationErrors(validationErrs),
			}
		}
		return PublishEvalPackResponse{}, err
	}

	validationErrs, err := m.validateStoredAssetReferences(ctx, workspaceID, bundle)
	if err != nil {
		return PublishEvalPackResponse{}, err
	}
	if len(validationErrs) > 0 {
		return PublishEvalPackResponse{}, EvalPackAuthoringValidationError{
			Errors: mapEvalPackValidationErrors(validationErrs),
		}
	}

	organizationID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return PublishEvalPackResponse{}, err
	}

	bundleArtifact, cleanupBundleArtifact, err := m.storeBundleArtifact(ctx, workspaceID, bundle, bundleYAML)
	if err != nil {
		return PublishEvalPackResponse{}, err
	}
	if cleanupBundleArtifact != nil {
		defer func() {
			if cleanupBundleArtifact != nil {
				_ = cleanupBundleArtifact()
			}
		}()
	}

	published, err := m.repo.PublishEvalPackBundle(ctx, repository.PublishEvalPackBundleParams{
		OrganizationID: organizationID,
		WorkspaceID:    workspaceID,
		Bundle:         bundle,
		BundleArtifact: bundleArtifact,
	})
	if err != nil {
		return PublishEvalPackResponse{}, err
	}
	cleanupBundleArtifact = nil

	return PublishEvalPackResponse{
		EvalPackID:        published.EvalPackID,
		EvalPackVersionID: published.EvalPackVersionID,
		EvaluationSpecID:       published.EvaluationSpecID,
		InputSetIDs:            published.InputSetIDs,
		BundleArtifactID:       published.BundleArtifactID,
	}, nil
}

// InstantiateCatalogPackResponse is returned when a curated catalog pack is
// cloned into a workspace. On a fresh clone it carries the full publish result;
// when the workspace already has the pack (AlreadyExisted), it carries the
// existing pack + its latest runnable version so the caller can run it right away.
type InstantiateCatalogPackResponse struct {
	EvalPackID        uuid.UUID   `json:"eval_pack_id"`
	EvalPackVersionID uuid.UUID   `json:"eval_pack_version_id"`
	EvaluationSpecID       *uuid.UUID  `json:"evaluation_spec_id,omitempty"`
	InputSetIDs            []uuid.UUID `json:"input_set_ids,omitempty"`
	Slug                   string      `json:"slug"`
	AlreadyExisted         bool        `json:"already_existed"`
	Runnable               bool        `json:"runnable"`
}

// InstantiateCatalogPack clones a curated library pack (by slug) into the
// workspace by republishing its embedded YAML through the normal authoring
// path, so the result is an ordinary, runnable, fully-owned workspace pack the
// user can run immediately or open in the builder to tweak.
//
// It is idempotent: adding the same template twice returns the existing pack
// instead of failing on the (eval_pack_id, version_number) uniqueness
// constraint.
func (m *EvalPackAuthoringManager) InstantiateCatalogPack(ctx context.Context, workspaceID uuid.UUID, slug string) (InstantiateCatalogPackResponse, error) {
	entry, ok, err := evalpack.CatalogBySlug(slug)
	if err != nil {
		return InstantiateCatalogPackResponse{}, err
	}
	if !ok {
		return InstantiateCatalogPackResponse{}, errCatalogPackNotFound
	}

	published, err := m.PublishBundle(ctx, workspaceID, []byte(entry.YAML))
	if err != nil {
		if errors.Is(err, repository.ErrEvalPackVersionExists) {
			packID, versionID, found, lookupErr := m.repo.GetWorkspaceEvalPackVersionBySlug(ctx, workspaceID, entry.Slug)
			if lookupErr != nil {
				return InstantiateCatalogPackResponse{}, lookupErr
			}
			if found {
				return InstantiateCatalogPackResponse{
					EvalPackID:        packID,
					EvalPackVersionID: versionID,
					Slug:                   entry.Slug,
					AlreadyExisted:         true,
					Runnable:               true,
				}, nil
			}
			return InstantiateCatalogPackResponse{}, fmt.Errorf("catalog pack %q already exists but has no runnable version: %w", entry.Slug, repository.ErrEvalPackVersionExists)
		}
		return InstantiateCatalogPackResponse{}, err
	}

	return InstantiateCatalogPackResponse{
		EvalPackID:        published.EvalPackID,
		EvalPackVersionID: published.EvalPackVersionID,
		EvaluationSpecID:       &published.EvaluationSpecID,
		InputSetIDs:            published.InputSetIDs,
		Slug:                   entry.Slug,
		AlreadyExisted:         false,
		Runnable:               true,
	}, nil
}

func mapEvalPackValidationErrors(errs evalpack.ValidationErrors) []validationErrorDetail {
	details := make([]validationErrorDetail, 0, len(errs))
	for _, err := range errs {
		details = append(details, validationErrorDetail{
			Field:   err.Field,
			Message: err.Message,
		})
	}
	return details
}

type evalPackResponse struct {
	ID          uuid.UUID                      `json:"id"`
	Name        string                         `json:"name"`
	Slug        string                         `json:"slug"`
	Description *string                        `json:"description,omitempty"`
	Versions    []evalPackVersionResponse `json:"versions"`
	CreatedAt   time.Time                      `json:"created_at"`
	UpdatedAt   time.Time                      `json:"updated_at"`
}

type evalPackVersionResponse struct {
	ID                  uuid.UUID       `json:"id"`
	EvalPackID     uuid.UUID       `json:"eval_pack_id"`
	VersionNumber       int32           `json:"version_number"`
	LifecycleStatus     string          `json:"lifecycle_status"`
	DeploymentDefaults  json.RawMessage `json:"deployment_defaults,omitempty"`
	Modality            string          `json:"modality,omitempty"`
	InterfaceTransports []string        `json:"interface_transports,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type listEvalPacksResponse struct {
	Items []evalPackResponse `json:"items"`
}

type challengeInputSetResponse struct {
	ID                     uuid.UUID `json:"id"`
	EvalPackVersionID uuid.UUID `json:"eval_pack_version_id"`
	InputKey               string    `json:"input_key"`
	Name                   string    `json:"name"`
}

type listChallengeInputSetsResponse struct {
	Items []challengeInputSetResponse `json:"items"`
}

func listEvalPacksHandler(logger *slog.Logger, service EvalPackReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := service.ListEvalPacks(r.Context())
		if err != nil {
			logger.Error("list eval packs request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		responseItems := make([]evalPackResponse, 0, len(result.Packs))
		for _, packWithVersions := range result.Packs {
			versions := make([]evalPackVersionResponse, 0, len(packWithVersions.Versions))
			for _, v := range packWithVersions.Versions {
				versions = append(versions, evalPackVersionResponse{
					ID:                  v.ID,
					EvalPackID:     v.EvalPackID,
					VersionNumber:       v.VersionNumber,
					LifecycleStatus:     v.LifecycleStatus,
					DeploymentDefaults:  v.DeploymentDefaults,
					Modality:            v.Modality,
					InterfaceTransports: append([]string(nil), v.InterfaceTransports...),
					CreatedAt:           v.CreatedAt,
					UpdatedAt:           v.UpdatedAt,
				})
			}

			responseItems = append(responseItems, evalPackResponse{
				ID:          packWithVersions.Pack.ID,
				Name:        packWithVersions.Pack.Name,
				Slug:        packWithVersions.Pack.Slug,
				Description: packWithVersions.Pack.Description,
				Versions:    versions,
				CreatedAt:   packWithVersions.Pack.CreatedAt,
				UpdatedAt:   packWithVersions.Pack.UpdatedAt,
			})
		}

		writeJSON(w, http.StatusOK, listEvalPacksResponse{Items: responseItems})
	}
}

func listChallengeInputSetsHandler(logger *slog.Logger, service EvalPackReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versionID, err := uuid.Parse(chi.URLParam(r, "versionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_eval_pack_version_id", "eval pack version id must be a valid UUID")
			return
		}

		result, err := service.ListChallengeInputSets(r.Context(), versionID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrEvalPackVersionNotFound):
				writeError(w, http.StatusNotFound, "not_found", "eval pack version not found")
			default:
				logger.Error("list challenge input sets request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"eval_pack_version_id", versionID,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		responseItems := make([]challengeInputSetResponse, 0, len(result.InputSets))
		for _, inputSet := range result.InputSets {
			responseItems = append(responseItems, challengeInputSetResponse{
				ID:                     inputSet.ID,
				EvalPackVersionID: inputSet.EvalPackVersionID,
				InputKey:               inputSet.InputKey,
				Name:                   inputSet.Name,
			})
		}

		writeJSON(w, http.StatusOK, listChallengeInputSetsResponse{Items: responseItems})
	}
}

func validateEvalPackHandler(logger *slog.Logger, service EvalPackAuthoringService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		body, err := readEvalPackBundleRequestBody(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_eval_pack_bundle", err.Error())
			return
		}

		result, err := service.ValidateBundle(r.Context(), workspaceID, body)
		if err != nil {
			logger.Error("validate eval pack request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		status := http.StatusOK
		if !result.Valid {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, result)
	}
}

func publishEvalPackHandler(logger *slog.Logger, service EvalPackAuthoringService, authorizer WorkspaceAuthorizer, entitlementGate EntitlementGateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, workspaceID, ActionPublishEvalPack); err != nil {
			writeAuthzError(w, err)
			return
		}

		if entitlementGate != nil {
			if err := entitlementGate.CheckWorkspaceFeature(r.Context(), workspaceID, billingpkg.FeaturePrivateEvalPacks); err != nil {
				writeEvalPackEntitlementError(w, logger, err)
				return
			}
		}

		body, err := readEvalPackBundleRequestBody(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_eval_pack_bundle", err.Error())
			return
		}

		result, err := service.PublishBundle(r.Context(), workspaceID, body)
		if err != nil {
			var validationErr EvalPackAuthoringValidationError
			switch {
			case errors.As(err, &validationErr):
				writeJSON(w, http.StatusBadRequest, ValidateEvalPackResponse{Valid: false, Errors: validationErr.Errors})
				return
			case errors.Is(err, repository.ErrEvalPackVersionExists):
				writeError(w, http.StatusConflict, "eval_pack_version_exists", err.Error())
				return
			case errors.Is(err, repository.ErrEvalPackMetadataConflict):
				writeError(w, http.StatusConflict, "eval_pack_metadata_conflict", err.Error())
				return
			default:
				logger.Error("publish eval pack request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"error", err,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}
		}

		writeJSON(w, http.StatusCreated, result)
	}
}

func writeEvalPackEntitlementError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var gateErr billingpkg.GateError
	switch {
	case errors.As(err, &gateErr):
		writeBillingGateError(w, gateErr.Decision)
	default:
		logger.Error("eval pack entitlement gate failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func readEvalPackBundleRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	const maxEvalPackBundleBytes = 1 << 20

	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxEvalPackBundleBytes))
	if err != nil {
		return nil, fmt.Errorf("request body must be valid yaml and at most %d bytes", maxEvalPackBundleBytes)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("request body is required")
	}

	return body, nil
}

func (m *EvalPackAuthoringManager) validateStoredAssetReferences(ctx context.Context, workspaceID uuid.UUID, bundle evalpack.Bundle) (evalpack.ValidationErrors, error) {
	var errs evalpack.ValidationErrors

	for _, asset := range evalPackAssetLocations(bundle) {
		if asset.ref.ArtifactID == nil {
			continue
		}

		storedArtifact, err := m.repo.GetArtifactByID(ctx, *asset.ref.ArtifactID)
		if err != nil {
			if errors.Is(err, repository.ErrArtifactNotFound) {
				errs = append(errs, evalpack.ValidationError{
					Field:   asset.field + ".artifact_id",
					Message: "must reference an existing artifact",
				})
				continue
			}
			return nil, fmt.Errorf("get eval pack asset artifact %s: %w", asset.ref.ArtifactID.String(), err)
		}

		if storedArtifact.WorkspaceID != workspaceID {
			errs = append(errs, evalpack.ValidationError{
				Field:   asset.field + ".artifact_id",
				Message: "must belong to the workspace",
			})
		}
	}

	return errs, nil
}

type evalPackAssetLocation struct {
	field string
	ref   evalpack.AssetReference
}

func evalPackAssetLocations(bundle evalpack.Bundle) []evalPackAssetLocation {
	locations := make([]evalPackAssetLocation, 0, len(bundle.Version.Assets))
	for i, asset := range bundle.Version.Assets {
		locations = append(locations, evalPackAssetLocation{
			field: fmt.Sprintf("version.assets[%d]", i),
			ref:   asset,
		})
	}
	for i, challenge := range bundle.Challenges {
		for j, asset := range challenge.Assets {
			locations = append(locations, evalPackAssetLocation{
				field: fmt.Sprintf("challenges[%d].assets[%d]", i, j),
				ref:   asset,
			})
		}
	}
	for i, inputSet := range bundle.InputSets {
		for j, item := range inputSet.Cases {
			for k, asset := range item.Assets {
				locations = append(locations, evalPackAssetLocation{
					field: fmt.Sprintf("input_sets[%d].cases[%d].assets[%d]", i, j, k),
					ref:   asset,
				})
			}
		}
	}
	return locations
}

func (m *EvalPackAuthoringManager) storeBundleArtifact(ctx context.Context, workspaceID uuid.UUID, bundle evalpack.Bundle, bundleYAML []byte) (*repository.CreateArtifactParams, func() error, error) {
	if m.store == nil {
		return nil, nil, nil
	}

	checksum := checksum(bundleYAML)
	objectKey := buildArtifactStorageKey(workspaceID, "eval_pack_bundle", checksum)
	contentType := "application/yaml"
	sizeBytes := int64(len(bundleYAML))
	objectMeta, err := m.store.PutObject(ctx, storage.PutObjectInput{
		Key:         objectKey,
		Body:        bytes.NewReader(bundleYAML),
		SizeBytes:   sizeBytes,
		ContentType: contentType,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("store eval pack bundle artifact: %w", err)
	}

	filename := fmt.Sprintf("%s-v%d.yaml", bundle.Pack.Slug, bundle.Version.Number)
	metadataJSON, err := json.Marshal(map[string]any{
		"filename":            filename,
		"eval_pack_slug": bundle.Pack.Slug,
		"eval_pack_name": bundle.Pack.Name,
		"version_number":      bundle.Version.Number,
		"artifact_role":       "eval_pack_bundle",
		"authored_format":     "yaml",
		"manifest_schema":     1,
	})
	if err != nil {
		_ = m.store.DeleteObject(ctx, objectMeta.Key)
		return nil, nil, fmt.Errorf("marshal eval pack bundle artifact metadata: %w", err)
	}

	return &repository.CreateArtifactParams{
			ArtifactType:    "eval_pack_bundle",
			StorageBucket:   objectMeta.Bucket,
			StorageKey:      objectMeta.Key,
			ContentType:     &contentType,
			SizeBytes:       &sizeBytes,
			ChecksumSHA256:  &checksum,
			Visibility:      defaultArtifactVisibility,
			RetentionStatus: defaultArtifactRetentionStatus,
			Metadata:        metadataJSON,
		}, func() error {
			return m.store.DeleteObject(ctx, objectMeta.Key)
		}, nil
}

func checksum(value []byte) string {
	sum := sha256.Sum256(value)
	return fmt.Sprintf("%x", sum[:])
}
