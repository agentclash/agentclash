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

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/storage"
	"github.com/google/uuid"
)

type ChallengePackReadRepository interface {
	ListVisibleChallengePacks(ctx context.Context, workspaceID uuid.UUID) ([]repository.ChallengePackSummary, error)
	ListRunnableChallengePVersionsByPackID(ctx context.Context, challengePackID uuid.UUID) ([]repository.ChallengePackVersionSummary, error)
}

type ChallengePackReadService interface {
	ListChallengePacks(ctx context.Context) (ListChallengePacksResult, error)
}

type ListChallengePacksResult struct {
	Packs []ChallengePackWithVersions
}

type ChallengePackWithVersions struct {
	Pack     repository.ChallengePackSummary
	Versions []repository.ChallengePackVersionSummary
}

type ChallengePackReadManager struct {
	repo ChallengePackReadRepository
}

func NewChallengePackReadManager(repo ChallengePackReadRepository) *ChallengePackReadManager {
	return &ChallengePackReadManager{
		repo: repo,
	}
}

func (m *ChallengePackReadManager) ListChallengePacks(ctx context.Context) (ListChallengePacksResult, error) {
	workspaceID, err := WorkspaceIDFromContext(ctx)
	if err != nil {
		return ListChallengePacksResult{}, err
	}

	packs, err := m.repo.ListVisibleChallengePacks(ctx, workspaceID)
	if err != nil {
		return ListChallengePacksResult{}, fmt.Errorf("list challenge packs: %w", err)
	}

	result := make([]ChallengePackWithVersions, 0, len(packs))
	for _, pack := range packs {
		versions, versionsErr := m.repo.ListRunnableChallengePVersionsByPackID(ctx, pack.ID)
		if versionsErr != nil {
			return ListChallengePacksResult{}, fmt.Errorf("list runnable versions for pack %s: %w", pack.ID, versionsErr)
		}

		result = append(result, ChallengePackWithVersions{
			Pack:     pack,
			Versions: versions,
		})
	}

	return ListChallengePacksResult{
		Packs: result,
	}, nil
}

type ChallengePackAuthoringRepository interface {
	GetArtifactByID(ctx context.Context, artifactID uuid.UUID) (repository.Artifact, error)
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	PublishChallengePackBundle(ctx context.Context, params repository.PublishChallengePackBundleParams) (repository.PublishedChallengePack, error)
}

type ChallengePackAuthoringService interface {
	ValidateBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (ValidateChallengePackResponse, error)
	PublishBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (PublishChallengePackResponse, error)
}

type ChallengePackAuthoringManager struct {
	repo  ChallengePackAuthoringRepository
	store storage.Store
}

type ValidateChallengePackResponse struct {
	Valid  bool                    `json:"valid"`
	Errors []validationErrorDetail `json:"errors"`
}

type PublishChallengePackResponse struct {
	ChallengePackID        uuid.UUID   `json:"challenge_pack_id"`
	ChallengePackVersionID uuid.UUID   `json:"challenge_pack_version_id"`
	EvaluationSpecID       uuid.UUID   `json:"evaluation_spec_id"`
	InputSetIDs            []uuid.UUID `json:"input_set_ids"`
	BundleArtifactID       *uuid.UUID  `json:"bundle_artifact_id,omitempty"`
}

type ChallengePackAuthoringValidationError struct {
	Errors []validationErrorDetail
}

func (e ChallengePackAuthoringValidationError) Error() string {
	return "challenge pack bundle has validation errors"
}

func NewChallengePackAuthoringManager(repo ChallengePackAuthoringRepository, store storage.Store) *ChallengePackAuthoringManager {
	return &ChallengePackAuthoringManager{repo: repo, store: store}
}

func (m *ChallengePackAuthoringManager) ValidateBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (ValidateChallengePackResponse, error) {
	bundle, err := challengepack.ParseYAML(bundleYAML)
	if err != nil {
		if validationErrs, ok := err.(challengepack.ValidationErrors); ok {
			return ValidateChallengePackResponse{
				Valid:  false,
				Errors: mapChallengePackValidationErrors(validationErrs),
			}, nil
		}
		return ValidateChallengePackResponse{}, err
	}

	validationErrs, err := m.validateStoredAssetReferences(ctx, workspaceID, bundle)
	if err != nil {
		return ValidateChallengePackResponse{}, err
	}
	if len(validationErrs) > 0 {
		return ValidateChallengePackResponse{
			Valid:  false,
			Errors: mapChallengePackValidationErrors(validationErrs),
		}, nil
	}

	return ValidateChallengePackResponse{Valid: true}, nil
}

func (m *ChallengePackAuthoringManager) PublishBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (PublishChallengePackResponse, error) {
	bundle, err := challengepack.ParseYAML(bundleYAML)
	if err != nil {
		if validationErrs, ok := err.(challengepack.ValidationErrors); ok {
			return PublishChallengePackResponse{}, ChallengePackAuthoringValidationError{
				Errors: mapChallengePackValidationErrors(validationErrs),
			}
		}
		return PublishChallengePackResponse{}, err
	}

	validationErrs, err := m.validateStoredAssetReferences(ctx, workspaceID, bundle)
	if err != nil {
		return PublishChallengePackResponse{}, err
	}
	if len(validationErrs) > 0 {
		return PublishChallengePackResponse{}, ChallengePackAuthoringValidationError{
			Errors: mapChallengePackValidationErrors(validationErrs),
		}
	}

	organizationID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return PublishChallengePackResponse{}, err
	}

	bundleArtifact, cleanupBundleArtifact, err := m.storeBundleArtifact(ctx, workspaceID, bundle, bundleYAML)
	if err != nil {
		return PublishChallengePackResponse{}, err
	}
	if cleanupBundleArtifact != nil {
		defer func() {
			if cleanupBundleArtifact != nil {
				_ = cleanupBundleArtifact()
			}
		}()
	}

	published, err := m.repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
		OrganizationID: organizationID,
		WorkspaceID:    workspaceID,
		Bundle:         bundle,
		BundleArtifact: bundleArtifact,
	})
	if err != nil {
		return PublishChallengePackResponse{}, err
	}
	cleanupBundleArtifact = nil

	return PublishChallengePackResponse{
		ChallengePackID:        published.ChallengePackID,
		ChallengePackVersionID: published.ChallengePackVersionID,
		EvaluationSpecID:       published.EvaluationSpecID,
		InputSetIDs:            published.InputSetIDs,
		BundleArtifactID:       published.BundleArtifactID,
	}, nil
}

func mapChallengePackValidationErrors(errs challengepack.ValidationErrors) []validationErrorDetail {
	details := make([]validationErrorDetail, 0, len(errs))
	for _, err := range errs {
		details = append(details, validationErrorDetail{
			Field:   err.Field,
			Message: err.Message,
		})
	}
	return details
}

type challengePackResponse struct {
	ID          uuid.UUID                      `json:"id"`
	Name        string                         `json:"name"`
	Description *string                        `json:"description,omitempty"`
	Versions    []challengePackVersionResponse `json:"versions"`
	CreatedAt   time.Time                      `json:"created_at"`
	UpdatedAt   time.Time                      `json:"updated_at"`
}

type challengePackVersionResponse struct {
	ID              uuid.UUID `json:"id"`
	ChallengePackID uuid.UUID `json:"challenge_pack_id"`
	VersionNumber   int32     `json:"version_number"`
	LifecycleStatus string    `json:"lifecycle_status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type listChallengePacksResponse struct {
	Items []challengePackResponse `json:"items"`
}

func listChallengePacksHandler(logger *slog.Logger, service ChallengePackReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := service.ListChallengePacks(r.Context())
		if err != nil {
			logger.Error("list challenge packs request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		responseItems := make([]challengePackResponse, 0, len(result.Packs))
		for _, packWithVersions := range result.Packs {
			versions := make([]challengePackVersionResponse, 0, len(packWithVersions.Versions))
			for _, v := range packWithVersions.Versions {
				versions = append(versions, challengePackVersionResponse{
					ID:              v.ID,
					ChallengePackID: v.ChallengePackID,
					VersionNumber:   v.VersionNumber,
					LifecycleStatus: v.LifecycleStatus,
					CreatedAt:       v.CreatedAt,
					UpdatedAt:       v.UpdatedAt,
				})
			}

			responseItems = append(responseItems, challengePackResponse{
				ID:          packWithVersions.Pack.ID,
				Name:        packWithVersions.Pack.Name,
				Description: packWithVersions.Pack.Description,
				Versions:    versions,
				CreatedAt:   packWithVersions.Pack.CreatedAt,
				UpdatedAt:   packWithVersions.Pack.UpdatedAt,
			})
		}

		writeJSON(w, http.StatusOK, listChallengePacksResponse{Items: responseItems})
	}
}

func validateChallengePackHandler(logger *slog.Logger, service ChallengePackAuthoringService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		body, err := readChallengePackBundleRequestBody(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_challenge_pack_bundle", err.Error())
			return
		}

		result, err := service.ValidateBundle(r.Context(), workspaceID, body)
		if err != nil {
			logger.Error("validate challenge pack request failed",
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

func publishChallengePackHandler(logger *slog.Logger, service ChallengePackAuthoringService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, err := WorkspaceIDFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		body, err := readChallengePackBundleRequestBody(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_challenge_pack_bundle", err.Error())
			return
		}

		result, err := service.PublishBundle(r.Context(), workspaceID, body)
		if err != nil {
			var validationErr ChallengePackAuthoringValidationError
			switch {
			case errors.As(err, &validationErr):
				writeJSON(w, http.StatusBadRequest, ValidateChallengePackResponse{Valid: false, Errors: validationErr.Errors})
				return
			case errors.Is(err, repository.ErrChallengePackVersionExists):
				writeError(w, http.StatusConflict, "challenge_pack_version_exists", err.Error())
				return
			case errors.Is(err, repository.ErrChallengePackMetadataConflict):
				writeError(w, http.StatusConflict, "challenge_pack_metadata_conflict", err.Error())
				return
			default:
				logger.Error("publish challenge pack request failed",
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

func readChallengePackBundleRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	const maxChallengePackBundleBytes = 1 << 20

	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxChallengePackBundleBytes))
	if err != nil {
		return nil, fmt.Errorf("request body must be valid yaml and at most %d bytes", maxChallengePackBundleBytes)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("request body is required")
	}

	return body, nil
}

func (m *ChallengePackAuthoringManager) validateStoredAssetReferences(ctx context.Context, workspaceID uuid.UUID, bundle challengepack.Bundle) (challengepack.ValidationErrors, error) {
	var errs challengepack.ValidationErrors

	for _, asset := range challengePackAssetLocations(bundle) {
		if asset.ref.ArtifactID == nil {
			continue
		}

		storedArtifact, err := m.repo.GetArtifactByID(ctx, *asset.ref.ArtifactID)
		if err != nil {
			if errors.Is(err, repository.ErrArtifactNotFound) {
				errs = append(errs, challengepack.ValidationError{
					Field:   asset.field + ".artifact_id",
					Message: "must reference an existing artifact",
				})
				continue
			}
			return nil, fmt.Errorf("get challenge pack asset artifact %s: %w", asset.ref.ArtifactID.String(), err)
		}

		if storedArtifact.WorkspaceID != workspaceID {
			errs = append(errs, challengepack.ValidationError{
				Field:   asset.field + ".artifact_id",
				Message: "must belong to the workspace",
			})
		}
	}

	return errs, nil
}

type challengePackAssetLocation struct {
	field string
	ref   challengepack.AssetReference
}

func challengePackAssetLocations(bundle challengepack.Bundle) []challengePackAssetLocation {
	locations := make([]challengePackAssetLocation, 0, len(bundle.Version.Assets))
	for i, asset := range bundle.Version.Assets {
		locations = append(locations, challengePackAssetLocation{
			field: fmt.Sprintf("version.assets[%d]", i),
			ref:   asset,
		})
	}
	for i, challenge := range bundle.Challenges {
		for j, asset := range challenge.Assets {
			locations = append(locations, challengePackAssetLocation{
				field: fmt.Sprintf("challenges[%d].assets[%d]", i, j),
				ref:   asset,
			})
		}
	}
	for i, inputSet := range bundle.InputSets {
		for j, item := range inputSet.Cases {
			for k, asset := range item.Assets {
				locations = append(locations, challengePackAssetLocation{
					field: fmt.Sprintf("input_sets[%d].cases[%d].assets[%d]", i, j, k),
					ref:   asset,
				})
			}
		}
	}
	return locations
}

func (m *ChallengePackAuthoringManager) storeBundleArtifact(ctx context.Context, workspaceID uuid.UUID, bundle challengepack.Bundle, bundleYAML []byte) (*repository.CreateArtifactParams, func() error, error) {
	if m.store == nil {
		return nil, nil, nil
	}

	checksum := checksum(bundleYAML)
	objectKey := buildArtifactStorageKey(workspaceID, "challenge_pack_bundle", checksum)
	contentType := "application/yaml"
	sizeBytes := int64(len(bundleYAML))
	objectMeta, err := m.store.PutObject(ctx, storage.PutObjectInput{
		Key:         objectKey,
		Body:        bytes.NewReader(bundleYAML),
		SizeBytes:   sizeBytes,
		ContentType: contentType,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("store challenge pack bundle artifact: %w", err)
	}

	filename := fmt.Sprintf("%s-v%d.yaml", bundle.Pack.Slug, bundle.Version.Number)
	metadataJSON, err := json.Marshal(map[string]any{
		"filename":            filename,
		"challenge_pack_slug": bundle.Pack.Slug,
		"challenge_pack_name": bundle.Pack.Name,
		"version_number":      bundle.Version.Number,
		"artifact_role":       "challenge_pack_bundle",
		"authored_format":     "yaml",
		"manifest_schema":     1,
	})
	if err != nil {
		_ = m.store.DeleteObject(ctx, objectMeta.Key)
		return nil, nil, fmt.Errorf("marshal challenge pack bundle artifact metadata: %w", err)
	}

	return &repository.CreateArtifactParams{
			ArtifactType:    "challenge_pack_bundle",
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
