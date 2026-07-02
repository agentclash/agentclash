package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/go-chi/chi/v5"
)

var errCatalogPackNotFound = errors.New("challenge pack catalog entry not found")

// challengePackCatalogListHandler serves the curated library of ready-to-run
// challenge packs — the full-pack analogue of GET /challenge-piece-library. It
// is a global, read-only catalog (not workspace-scoped), so it needs no
// workspace authorization beyond being an authenticated request. The heavy YAML
// body is omitted from list responses; fetch the detail endpoint for it.
func challengePackCatalogListHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := challengepack.Catalog()
		if err != nil {
			logger.Error("load challenge pack catalog failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		family := strings.TrimSpace(r.URL.Query().Get("family"))
		category := strings.TrimSpace(r.URL.Query().Get("category"))

		items := make([]challengepack.CatalogEntry, 0, len(entries))
		for _, entry := range entries {
			if family != "" && !strings.EqualFold(entry.Family, family) {
				continue
			}
			if category != "" && !strings.EqualFold(entry.Category, category) {
				continue
			}
			items = append(items, entry.Summary())
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// challengePackCatalogDetailHandler returns one catalog entry including its raw
// runnable YAML, for the library detail view.
func challengePackCatalogDetailHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimSpace(chi.URLParam(r, "slug"))
		entry, ok, err := challengepack.CatalogBySlug(slug)
		if err != nil {
			logger.Error("load challenge pack catalog entry failed", "slug", slug, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "challenge pack catalog entry not found")
			return
		}
		writeJSON(w, http.StatusOK, entry)
	}
}

// instantiateChallengePackCatalogHandler clones a curated catalog pack into the
// caller's workspace as a runnable, fully-owned pack.
//
// It deliberately does NOT apply the FeaturePrivateChallengePacks entitlement
// gate that publishChallengePackHandler uses: the curated library is the free
// activation on-ramp, and the paywall stays on authoring (raw-YAML publish +
// builder publish). To gate it later, thread an EntitlementGateService in here
// and check the feature exactly as publishChallengePackHandler does.
func instantiateChallengePackCatalogHandler(logger *slog.Logger, service ChallengePackAuthoringService, authorizer WorkspaceAuthorizer) http.HandlerFunc {
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

		if err := AuthorizeWorkspaceAction(r.Context(), authorizer, caller, workspaceID, ActionPublishChallengePack); err != nil {
			writeAuthzError(w, err)
			return
		}

		slug := strings.TrimSpace(chi.URLParam(r, "slug"))
		result, err := service.InstantiateCatalogPack(r.Context(), workspaceID, slug)
		if err != nil {
			var validationErr ChallengePackAuthoringValidationError
			switch {
			case errors.Is(err, errCatalogPackNotFound):
				writeError(w, http.StatusNotFound, "not_found", "challenge pack catalog entry not found")
			case errors.As(err, &validationErr):
				// A catalog pack that fails validation is a server-side content
				// bug (CI guards against it), not a client error.
				logger.Error("catalog pack failed validation on instantiate", "slug", slug, "errors", validationErr.Errors)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			case errors.Is(err, repository.ErrChallengePackMetadataConflict):
				writeError(w, http.StatusConflict, "challenge_pack_metadata_conflict", err.Error())
			case errors.Is(err, repository.ErrChallengePackVersionExists):
				writeError(w, http.StatusConflict, "challenge_pack_version_exists", err.Error())
			default:
				logger.Error("instantiate challenge pack catalog request failed", "slug", slug, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			return
		}

		status := http.StatusCreated
		if result.AlreadyExisted {
			status = http.StatusOK
		}
		writeJSON(w, status, result)
	}
}
