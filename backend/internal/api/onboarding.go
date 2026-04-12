package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type OnboardingService interface {
	Onboard(ctx context.Context, caller Caller, input OnboardInput) (OnboardResult, error)
}

type OnboardInput struct {
	OrganizationName string
	OrganizationSlug *string
	WorkspaceName    string
	WorkspaceSlug    *string
}

type OnboardResult struct {
	Organization OrganizationResult `json:"organization"`
	Workspace    WorkspaceResult    `json:"workspace"`
}

type OnboardingRepository interface {
	CountActiveOrgAdminMemberships(ctx context.Context, userID uuid.UUID) (int64, error)
	Onboard(ctx context.Context, input repository.OnboardInput) (repository.OnboardResult, error)
}

type OnboardingManager struct {
	repo OnboardingRepository
}

func NewOnboardingManager(repo OnboardingRepository) *OnboardingManager {
	return &OnboardingManager{repo: repo}
}

func (m *OnboardingManager) Onboard(ctx context.Context, caller Caller, input OnboardInput) (OnboardResult, error) {
	count, err := m.repo.CountActiveOrgAdminMemberships(ctx, caller.UserID)
	if err != nil {
		return OnboardResult{}, err
	}
	if count > 0 {
		return OnboardResult{}, repository.ErrAlreadyOnboarded
	}

	orgSlug := ""
	if input.OrganizationSlug != nil {
		if err := validateSlug(*input.OrganizationSlug); err != nil {
			return OnboardResult{}, err
		}
		orgSlug = *input.OrganizationSlug
	} else {
		orgSlug = generateSlug(input.OrganizationName)
	}
	if err := validateSlug(orgSlug); err != nil {
		return OnboardResult{}, err
	}

	wsSlug := ""
	if input.WorkspaceSlug != nil {
		if err := validateSlug(*input.WorkspaceSlug); err != nil {
			return OnboardResult{}, err
		}
		wsSlug = *input.WorkspaceSlug
	} else {
		wsSlug = generateSlug(input.WorkspaceName)
	}
	if err := validateSlug(wsSlug); err != nil {
		return OnboardResult{}, err
	}

	result, err := m.repo.Onboard(ctx, repository.OnboardInput{
		UserID:           caller.UserID,
		OrganizationName: input.OrganizationName,
		OrganizationSlug: orgSlug,
		WorkspaceName:    input.WorkspaceName,
		WorkspaceSlug:    wsSlug,
	})
	if err != nil {
		return OnboardResult{}, err
	}

	return OnboardResult{
		Organization: OrganizationResult{
			ID:        result.Organization.ID,
			Name:      result.Organization.Name,
			Slug:      result.Organization.Slug,
			Status:    result.Organization.Status,
			CreatedAt: result.Organization.CreatedAt,
			UpdatedAt: result.Organization.UpdatedAt,
		},
		Workspace: WorkspaceResult{
			ID:             result.Workspace.ID,
			OrganizationID: result.Workspace.OrganizationID,
			Name:           result.Workspace.Name,
			Slug:           result.Workspace.Slug,
			Status:         result.Workspace.Status,
			CreatedAt:      result.Workspace.CreatedAt,
			UpdatedAt:      result.Workspace.UpdatedAt,
		},
	}, nil
}

func onboardHandler(logger *slog.Logger, service OnboardingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			OrganizationName string  `json:"organization_name"`
			OrganizationSlug *string `json:"organization_slug,omitempty"`
			WorkspaceName    string  `json:"workspace_name"`
			WorkspaceSlug    *string `json:"workspace_slug,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if req.OrganizationName == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "organization_name is required")
			return
		}
		if req.WorkspaceName == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "workspace_name is required")
			return
		}

		result, err := service.Onboard(r.Context(), caller, OnboardInput{
			OrganizationName: req.OrganizationName,
			OrganizationSlug: req.OrganizationSlug,
			WorkspaceName:    req.WorkspaceName,
			WorkspaceSlug:    req.WorkspaceSlug,
		})
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrAlreadyOnboarded):
				writeError(w, http.StatusConflict, "already_onboarded", "you already have an organization")
			case errors.Is(err, repository.ErrSlugTaken):
				writeError(w, http.StatusConflict, "slug_taken", "an organization or workspace with this slug already exists")
			case errors.Is(err, ErrInvalidSlug):
				writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			default:
				logger.Error("onboarding failed", "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}
}

