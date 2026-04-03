package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
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
	PublishChallengePackBundle(ctx context.Context, params repository.PublishChallengePackBundleParams) (repository.PublishedChallengePack, error)
}

type ChallengePackAuthoringService interface {
	ValidateBundle(ctx context.Context, bundleYAML []byte) (ValidateChallengePackResponse, error)
	PublishBundle(ctx context.Context, workspaceID uuid.UUID, bundleYAML []byte) (PublishChallengePackResponse, error)
}

type ChallengePackAuthoringManager struct {
	repo ChallengePackAuthoringRepository
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
}

type ChallengePackAuthoringValidationError struct {
	Errors []validationErrorDetail
}

func (e ChallengePackAuthoringValidationError) Error() string {
	return "challenge pack bundle has validation errors"
}

func NewChallengePackAuthoringManager(repo ChallengePackAuthoringRepository) *ChallengePackAuthoringManager {
	return &ChallengePackAuthoringManager{repo: repo}
}

func (m *ChallengePackAuthoringManager) ValidateBundle(_ context.Context, bundleYAML []byte) (ValidateChallengePackResponse, error) {
	if _, err := challengepack.ParseYAML(bundleYAML); err != nil {
		if validationErrs, ok := err.(challengepack.ValidationErrors); ok {
			return ValidateChallengePackResponse{
				Valid:  false,
				Errors: mapChallengePackValidationErrors(validationErrs),
			}, nil
		}
		return ValidateChallengePackResponse{}, err
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

	published, err := m.repo.PublishChallengePackBundle(ctx, repository.PublishChallengePackBundleParams{
		WorkspaceID: workspaceID,
		Bundle:      bundle,
	})
	if err != nil {
		return PublishChallengePackResponse{}, err
	}

	return PublishChallengePackResponse{
		ChallengePackID:        published.ChallengePackID,
		ChallengePackVersionID: published.ChallengePackVersionID,
		EvaluationSpecID:       published.EvaluationSpecID,
		InputSetIDs:            published.InputSetIDs,
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
		body, err := readChallengePackBundleRequestBody(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_challenge_pack_bundle", err.Error())
			return
		}

		result, err := service.ValidateBundle(r.Context(), body)
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
