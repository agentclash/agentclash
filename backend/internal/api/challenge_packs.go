package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type ChallengePackReadRepository interface {
	ListChallengePacks(ctx context.Context) ([]repository.ChallengePackSummary, error)
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
	packs, err := m.repo.ListChallengePacks(ctx)
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
