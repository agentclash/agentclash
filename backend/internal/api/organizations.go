package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OrganizationService interface {
	CreateOrganization(ctx context.Context, caller Caller, input CreateOrganizationInput) (OrganizationResult, error)
	GetOrganization(ctx context.Context, caller Caller, orgID uuid.UUID) (OrganizationResult, error)
	ListOrganizations(ctx context.Context, caller Caller, limit, offset int32) (ListOrganizationsResult, error)
	UpdateOrganization(ctx context.Context, caller Caller, orgID uuid.UUID, input UpdateOrganizationInput) (OrganizationResult, error)
}

type CreateOrganizationInput struct {
	Name string
	Slug *string
}

type UpdateOrganizationInput struct {
	Name   *string
	Status *string
}

type OrganizationResult struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListOrganizationsResult struct {
	Items  []OrganizationResult `json:"items"`
	Total  int64                `json:"total"`
	Limit  int32                `json:"limit"`
	Offset int32                `json:"offset"`
}

type OrganizationRepository interface {
	CountActiveOrgAdminMemberships(ctx context.Context, userID uuid.UUID) (int64, error)
	CreateOrganizationWithAdmin(ctx context.Context, input repository.CreateOrgWithAdminInput) (repository.OrganizationRow, error)
	GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (repository.OrganizationRow, error)
	ListOrganizationsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]repository.OrganizationRow, error)
	CountOrganizationsByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	UpdateOrganization(ctx context.Context, orgID uuid.UUID, input repository.UpdateOrgInput) (repository.OrganizationRow, error)
	ArchiveOrganizationCascade(ctx context.Context, orgID uuid.UUID) (repository.OrganizationRow, error)
}

const maxOrganizationsPerUser = 1

type OrganizationManager struct {
	orgAuthz OrganizationAuthorizer
	repo     OrganizationRepository
}

func NewOrganizationManager(orgAuthz OrganizationAuthorizer, repo OrganizationRepository) *OrganizationManager {
	return &OrganizationManager{orgAuthz: orgAuthz, repo: repo}
}

func (m *OrganizationManager) CreateOrganization(ctx context.Context, caller Caller, input CreateOrganizationInput) (OrganizationResult, error) {
	count, err := m.repo.CountActiveOrgAdminMemberships(ctx, caller.UserID)
	if err != nil {
		return OrganizationResult{}, err
	}
	if count >= maxOrganizationsPerUser {
		return OrganizationResult{}, repository.ErrOrganizationLimitReached
	}

	slug := ""
	if input.Slug != nil {
		if err := validateSlug(*input.Slug); err != nil {
			return OrganizationResult{}, err
		}
		slug = *input.Slug
	} else {
		slug = generateSlug(input.Name)
	}
	if err := validateSlug(slug); err != nil {
		return OrganizationResult{}, err
	}

	org, err := m.repo.CreateOrganizationWithAdmin(ctx, repository.CreateOrgWithAdminInput{
		Name:   input.Name,
		Slug:   slug,
		UserID: caller.UserID,
	})
	if err != nil {
		return OrganizationResult{}, err
	}

	return orgRowToResult(org), nil
}

func (m *OrganizationManager) GetOrganization(ctx context.Context, caller Caller, orgID uuid.UUID) (OrganizationResult, error) {
	if err := m.orgAuthz.AuthorizeOrganization(ctx, caller, orgID); err != nil {
		return OrganizationResult{}, err
	}

	org, err := m.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return OrganizationResult{}, err
	}

	return orgRowToResult(org), nil
}

func (m *OrganizationManager) ListOrganizations(ctx context.Context, caller Caller, limit, offset int32) (ListOrganizationsResult, error) {
	orgs, err := m.repo.ListOrganizationsByUserID(ctx, caller.UserID, limit, offset)
	if err != nil {
		return ListOrganizationsResult{}, err
	}

	total, err := m.repo.CountOrganizationsByUserID(ctx, caller.UserID)
	if err != nil {
		return ListOrganizationsResult{}, err
	}

	items := make([]OrganizationResult, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, orgRowToResult(org))
	}

	return ListOrganizationsResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (m *OrganizationManager) UpdateOrganization(ctx context.Context, caller Caller, orgID uuid.UUID, input UpdateOrganizationInput) (OrganizationResult, error) {
	if err := m.orgAuthz.AuthorizeOrganizationAdmin(ctx, caller, orgID); err != nil {
		return OrganizationResult{}, err
	}

	// Archive cascade (archive org + all workspaces + all memberships).
	if input.Status != nil && *input.Status == "archived" {
		org, err := m.repo.ArchiveOrganizationCascade(ctx, orgID)
		if err != nil {
			return OrganizationResult{}, err
		}
		return orgRowToResult(org), nil
	}

	org, err := m.repo.UpdateOrganization(ctx, orgID, repository.UpdateOrgInput{
		Name:   input.Name,
		Status: input.Status,
	})
	if err != nil {
		return OrganizationResult{}, err
	}

	return orgRowToResult(org), nil
}

func orgRowToResult(org repository.OrganizationRow) OrganizationResult {
	return OrganizationResult{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		Status:    org.Status,
		CreatedAt: org.CreatedAt,
		UpdatedAt: org.UpdatedAt,
	}
}

// --- Handlers ---

func createOrganizationHandler(logger *slog.Logger, service OrganizationService) http.HandlerFunc {
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
			Name string  `json:"name"`
			Slug *string `json:"slug,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "name is required")
			return
		}

		result, err := service.CreateOrganization(r.Context(), caller, CreateOrganizationInput{
			Name: req.Name,
			Slug: req.Slug,
		})
		if err != nil {
			handleOrgError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}
}

func getOrganizationHandler(logger *slog.Logger, service OrganizationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		orgID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_organization_id", "organization ID is malformed")
			return
		}

		result, err := service.GetOrganization(r.Context(), caller, orgID)
		if err != nil {
			handleOrgError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func listOrganizationsHandler(logger *slog.Logger, service OrganizationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		limit := int32(50)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
				limit = int32(parsed)
				if limit > 100 {
					limit = 100
				}
			}
		}
		offset := int32(0)
		if raw := r.URL.Query().Get("offset"); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed >= 0 {
				offset = int32(parsed)
			}
		}

		result, err := service.ListOrganizations(r.Context(), caller, limit, offset)
		if err != nil {
			logger.Error("list organizations failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func updateOrganizationHandler(logger *slog.Logger, service OrganizationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}

		orgID, err := uuid.Parse(chi.URLParam(r, "organizationID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_organization_id", "organization ID is malformed")
			return
		}

		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		var req struct {
			Name   *string `json:"name,omitempty"`
			Status *string `json:"status,omitempty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
			return
		}
		if req.Name == nil && req.Status == nil {
			writeError(w, http.StatusBadRequest, "validation_error", "at least one of name or status is required")
			return
		}
		if req.Status != nil && *req.Status != "active" && *req.Status != "archived" {
			writeError(w, http.StatusBadRequest, "validation_error", "status must be active or archived")
			return
		}

		result, err := service.UpdateOrganization(r.Context(), caller, orgID, UpdateOrganizationInput{
			Name:   req.Name,
			Status: req.Status,
		})
		if err != nil {
			handleOrgError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func handleOrgError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
	case errors.Is(err, repository.ErrOrganizationNotFound):
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
	case errors.Is(err, repository.ErrOrganizationLimitReached):
		writeError(w, http.StatusConflict, "organization_limit_reached", "you have reached the maximum number of organizations")
	case errors.Is(err, repository.ErrSlugTaken):
		writeError(w, http.StatusConflict, "slug_taken", "an organization with this slug already exists")
	case errors.Is(err, ErrInvalidSlug):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	default:
		logger.Error("organization operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
