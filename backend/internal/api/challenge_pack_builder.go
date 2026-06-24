package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ChallengePackBuilderRepository is the data access the visual pack builder
// needs: CRUD over reusable pieces, CRUD over in-progress drafts, and bulk
// piece resolution at compile/publish time.
type ChallengePackBuilderRepository interface {
	CreateChallengePiece(ctx context.Context, params repository.CreateChallengePieceParams) (repository.ChallengePiece, error)
	GetChallengePieceByID(ctx context.Context, id uuid.UUID) (repository.ChallengePiece, error)
	ListChallengePieces(ctx context.Context, workspaceID uuid.UUID, kind *string) ([]repository.ChallengePiece, error)
	ListChallengePiecesByIDs(ctx context.Context, workspaceID uuid.UUID, ids []uuid.UUID) ([]repository.ChallengePiece, error)
	PatchChallengePiece(ctx context.Context, params repository.PatchChallengePieceParams) (repository.ChallengePiece, error)
	ArchiveChallengePiece(ctx context.Context, workspaceID, id uuid.UUID) error
	CreateChallengePackDraft(ctx context.Context, params repository.CreateChallengePackDraftParams) (repository.ChallengePackDraft, error)
	GetChallengePackDraftByID(ctx context.Context, id uuid.UUID) (repository.ChallengePackDraft, error)
	ListChallengePackDrafts(ctx context.Context, workspaceID uuid.UUID) ([]repository.ChallengePackDraft, error)
	PatchChallengePackDraft(ctx context.Context, params repository.PatchChallengePackDraftParams) (repository.ChallengePackDraft, error)
	DeleteChallengePackDraft(ctx context.Context, id uuid.UUID) error
	// Used to hydrate a draft from an already-published pack (edit-in-builder).
	GetRunnableChallengePackVersionByID(ctx context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error)
	WorkspacePublicPacksEnabled(ctx context.Context, workspaceID uuid.UUID) (bool, error)
}

// ChallengePackBuilderService is the full builder surface: reusable piece CRUD,
// draft CRUD, and the compile/publish steps that snapshot a draft into a
// runnable challenge pack version.
type ChallengePackBuilderService interface {
	ListPieces(ctx context.Context, caller Caller, workspaceID uuid.UUID, kind string) ([]repository.ChallengePiece, error)
	CreatePiece(ctx context.Context, caller Caller, input CreatePieceInput) (repository.ChallengePiece, error)
	GetPiece(ctx context.Context, caller Caller, workspaceID, pieceID uuid.UUID) (repository.ChallengePiece, error)
	PatchPiece(ctx context.Context, caller Caller, input PatchPieceInput) (repository.ChallengePiece, error)
	ArchivePiece(ctx context.Context, caller Caller, workspaceID, pieceID uuid.UUID) error

	ListDrafts(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.ChallengePackDraft, error)
	CreateDraft(ctx context.Context, caller Caller, input CreateDraftInput) (repository.ChallengePackDraft, error)
	GetDraft(ctx context.Context, caller Caller, workspaceID, draftID uuid.UUID) (repository.ChallengePackDraft, error)
	PatchDraft(ctx context.Context, caller Caller, input PatchDraftInput) (repository.ChallengePackDraft, error)
	DeleteDraft(ctx context.Context, caller Caller, workspaceID, draftID uuid.UUID) error
	CompileDraft(ctx context.Context, caller Caller, workspaceID, draftID uuid.UUID) (CompileDraftResult, error)
	PublishDraft(ctx context.Context, caller Caller, workspaceID, draftID uuid.UUID) (PublishChallengePackResponse, error)
}

type CreatePieceInput struct {
	WorkspaceID uuid.UUID
	Kind        string
	Slug        string
	Name        string
	Description string
	Definition  json.RawMessage
	CreatedBy   uuid.UUID
}

type PatchPieceInput struct {
	WorkspaceID uuid.UUID
	PieceID     uuid.UUID
	Name        *string
	Slug        *string
	Description *string
	Definition  json.RawMessage
}

type CreateDraftInput struct {
	WorkspaceID     uuid.UUID
	Name            string
	ExecutionMode   string
	ChallengePackID *uuid.UUID
	Composition     json.RawMessage
	CreatedBy       uuid.UUID
	// FromChallengePackVersionID, when set, hydrates the draft's composition from
	// an already-published pack version (edit-in-builder). Mutually exclusive
	// with Composition.
	FromChallengePackVersionID *uuid.UUID
}

type PatchDraftInput struct {
	WorkspaceID   uuid.UUID
	DraftID       uuid.UUID
	Name          *string
	ExecutionMode *string
	Composition   json.RawMessage
}

// CompileDraftResult is the outcome of resolving + validating a draft without
// publishing: a readable spec card, the equivalent YAML (for the escape
// hatch), and a validation verdict. Compile never fails on an incomplete draft
// — it reports the missing pieces in Errors so the builder can render them.
type CompileDraftResult struct {
	Valid    bool
	Errors   []validationErrorDetail
	SpecCard challengepack.SpecCard
	YAML     string
}

// ChallengePackBuilderValidationError carries field-level validation problems
// for a piece or composition; handlers render it as a 400 with a structured
// error list, matching the YAML authoring path.
type ChallengePackBuilderValidationError struct {
	Errors []validationErrorDetail
}

func (e ChallengePackBuilderValidationError) Error() string {
	return "challenge pack builder input has validation errors"
}

func builderValidationError(field, message string) ChallengePackBuilderValidationError {
	return ChallengePackBuilderValidationError{Errors: []validationErrorDetail{{Field: field, Message: message}}}
}

type ChallengePackBuilderManager struct {
	authorizer WorkspaceAuthorizer
	repo       ChallengePackBuilderRepository
	authoring  ChallengePackAuthoringService
}

// NewChallengePackBuilderManager wires the builder. authoring is reused for the
// publish step so drafts go through the exact same validate → snapshot →
// publish path as the raw-YAML escape hatch.
func NewChallengePackBuilderManager(authorizer WorkspaceAuthorizer, repo ChallengePackBuilderRepository, authoring ChallengePackAuthoringService) *ChallengePackBuilderManager {
	return &ChallengePackBuilderManager{authorizer: authorizer, repo: repo, authoring: authoring}
}

func (m *ChallengePackBuilderManager) authorize(ctx context.Context, caller Caller, workspaceID uuid.UUID) error {
	return AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionPublishChallengePack)
}

func (m *ChallengePackBuilderManager) ListPieces(ctx context.Context, caller Caller, workspaceID uuid.UUID, kind string) ([]repository.ChallengePiece, error) {
	if err := m.authorize(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	var kindFilter *string
	if kind = strings.TrimSpace(kind); kind != "" {
		if !isValidPieceKind(kind) {
			return nil, builderValidationError("kind", "must be one of validator, judge, input_set, challenge")
		}
		kindFilter = &kind
	}
	return m.repo.ListChallengePieces(ctx, workspaceID, kindFilter)
}

func (m *ChallengePackBuilderManager) CreatePiece(ctx context.Context, caller Caller, input CreatePieceInput) (repository.ChallengePiece, error) {
	if err := m.authorize(ctx, caller, input.WorkspaceID); err != nil {
		return repository.ChallengePiece{}, err
	}
	if !isValidPieceKind(input.Kind) {
		return repository.ChallengePiece{}, builderValidationError("kind", "must be one of validator, judge, input_set, challenge")
	}
	if strings.TrimSpace(input.Slug) == "" {
		return repository.ChallengePiece{}, builderValidationError("slug", "is required")
	}
	if strings.TrimSpace(input.Name) == "" {
		return repository.ChallengePiece{}, builderValidationError("name", "is required")
	}
	if errs := validatePieceDefinition(input.Kind, input.Definition); len(errs) > 0 {
		return repository.ChallengePiece{}, ChallengePackBuilderValidationError{Errors: errs}
	}
	return m.repo.CreateChallengePiece(ctx, repository.CreateChallengePieceParams{
		WorkspaceID:     input.WorkspaceID,
		Kind:            input.Kind,
		Slug:            input.Slug,
		Name:            input.Name,
		Description:     input.Description,
		Definition:      input.Definition,
		CreatedByUserID: optionalUserID(input.CreatedBy),
	})
}

func (m *ChallengePackBuilderManager) GetPiece(ctx context.Context, caller Caller, workspaceID, pieceID uuid.UUID) (repository.ChallengePiece, error) {
	if err := m.authorize(ctx, caller, workspaceID); err != nil {
		return repository.ChallengePiece{}, err
	}
	piece, err := m.repo.GetChallengePieceByID(ctx, pieceID)
	if err != nil {
		return repository.ChallengePiece{}, err
	}
	if piece.WorkspaceID != workspaceID {
		return repository.ChallengePiece{}, repository.ErrChallengePieceNotFound
	}
	return piece, nil
}

func (m *ChallengePackBuilderManager) PatchPiece(ctx context.Context, caller Caller, input PatchPieceInput) (repository.ChallengePiece, error) {
	existing, err := m.GetPiece(ctx, caller, input.WorkspaceID, input.PieceID)
	if err != nil {
		return repository.ChallengePiece{}, err
	}
	if len(input.Definition) > 0 {
		if errs := validatePieceDefinition(existing.Kind, input.Definition); len(errs) > 0 {
			return repository.ChallengePiece{}, ChallengePackBuilderValidationError{Errors: errs}
		}
	}
	return m.repo.PatchChallengePiece(ctx, repository.PatchChallengePieceParams{
		ID:          input.PieceID,
		WorkspaceID: input.WorkspaceID,
		Name:        input.Name,
		Slug:        input.Slug,
		Description: input.Description,
		Definition:  input.Definition,
	})
}

func (m *ChallengePackBuilderManager) ArchivePiece(ctx context.Context, caller Caller, workspaceID, pieceID uuid.UUID) error {
	if _, err := m.GetPiece(ctx, caller, workspaceID, pieceID); err != nil {
		return err
	}
	return m.repo.ArchiveChallengePiece(ctx, workspaceID, pieceID)
}

func (m *ChallengePackBuilderManager) ListDrafts(ctx context.Context, caller Caller, workspaceID uuid.UUID) ([]repository.ChallengePackDraft, error) {
	if err := m.authorize(ctx, caller, workspaceID); err != nil {
		return nil, err
	}
	return m.repo.ListChallengePackDrafts(ctx, workspaceID)
}

func (m *ChallengePackBuilderManager) CreateDraft(ctx context.Context, caller Caller, input CreateDraftInput) (repository.ChallengePackDraft, error) {
	if err := m.authorize(ctx, caller, input.WorkspaceID); err != nil {
		return repository.ChallengePackDraft{}, err
	}
	if input.ExecutionMode != "" && !isValidExecutionMode(input.ExecutionMode) {
		return repository.ChallengePackDraft{}, builderValidationError("execution_mode", "must be one of native, prompt_eval, responses, multi_turn")
	}

	name := input.Name
	executionMode := input.ExecutionMode
	composition := input.Composition
	challengePackID := input.ChallengePackID

	// Edit-in-builder: hydrate the composition from an already-published pack.
	if input.FromChallengePackVersionID != nil {
		if len(composition) > 0 {
			return repository.ChallengePackDraft{}, builderValidationError("composition", "must be omitted when from_challenge_pack_version_id is set")
		}
		hydrated, err := m.hydrateDraftFromVersion(ctx, input.WorkspaceID, *input.FromChallengePackVersionID)
		if err != nil {
			return repository.ChallengePackDraft{}, err
		}
		composition = hydrated.composition
		challengePackID = &hydrated.challengePackID
		if executionMode == "" {
			executionMode = hydrated.executionMode
		}
		if strings.TrimSpace(name) == "" {
			name = hydrated.name
		}
	}

	if strings.TrimSpace(name) == "" {
		return repository.ChallengePackDraft{}, builderValidationError("name", "is required")
	}

	return m.repo.CreateChallengePackDraft(ctx, repository.CreateChallengePackDraftParams{
		WorkspaceID:     input.WorkspaceID,
		Name:            name,
		ExecutionMode:   executionMode,
		ChallengePackID: challengePackID,
		Composition:     composition,
		CreatedByUserID: optionalUserID(input.CreatedBy),
	})
}

type hydratedDraft struct {
	composition     json.RawMessage
	name            string
	executionMode   string
	challengePackID uuid.UUID
}

// hydrateDraftFromVersion loads a published pack version, decompiles its
// manifest into a builder composition, and returns it for seeding a new draft.
// Visibility mirrors run creation: a workspace can only open its own packs, or
// global packs when public packs are enabled.
func (m *ChallengePackBuilderManager) hydrateDraftFromVersion(ctx context.Context, workspaceID, versionID uuid.UUID) (hydratedDraft, error) {
	version, err := m.repo.GetRunnableChallengePackVersionByID(ctx, versionID)
	if err != nil {
		return hydratedDraft{}, err
	}
	if version.WorkspaceID != nil && *version.WorkspaceID != workspaceID {
		return hydratedDraft{}, repository.ErrChallengePackVersionNotFound
	}
	if version.WorkspaceID == nil {
		enabled, accessErr := m.repo.WorkspacePublicPacksEnabled(ctx, workspaceID)
		if accessErr != nil {
			return hydratedDraft{}, accessErr
		}
		if !enabled {
			return hydratedDraft{}, repository.ErrChallengePackVersionNotFound
		}
	}

	bundle, err := challengepack.ManifestToBundle(version.Manifest)
	if err != nil {
		return hydratedDraft{}, fmt.Errorf("reconstruct bundle from manifest: %w", err)
	}
	comp, err := challengepack.BundleToComposition(bundle)
	if err != nil {
		return hydratedDraft{}, fmt.Errorf("decompose bundle into composition: %w", err)
	}
	encoded, err := json.Marshal(comp)
	if err != nil {
		return hydratedDraft{}, fmt.Errorf("marshal hydrated composition: %w", err)
	}

	return hydratedDraft{
		composition:     encoded,
		name:            bundle.Pack.Name,
		executionMode:   bundle.Version.ExecutionMode,
		challengePackID: version.ChallengePackID,
	}, nil
}

func (m *ChallengePackBuilderManager) GetDraft(ctx context.Context, caller Caller, workspaceID, draftID uuid.UUID) (repository.ChallengePackDraft, error) {
	if err := m.authorize(ctx, caller, workspaceID); err != nil {
		return repository.ChallengePackDraft{}, err
	}
	draft, err := m.repo.GetChallengePackDraftByID(ctx, draftID)
	if err != nil {
		return repository.ChallengePackDraft{}, err
	}
	if draft.WorkspaceID != workspaceID {
		return repository.ChallengePackDraft{}, repository.ErrChallengePackDraftNotFound
	}
	return draft, nil
}

func (m *ChallengePackBuilderManager) PatchDraft(ctx context.Context, caller Caller, input PatchDraftInput) (repository.ChallengePackDraft, error) {
	if _, err := m.GetDraft(ctx, caller, input.WorkspaceID, input.DraftID); err != nil {
		return repository.ChallengePackDraft{}, err
	}
	if input.ExecutionMode != nil && !isValidExecutionMode(*input.ExecutionMode) {
		return repository.ChallengePackDraft{}, builderValidationError("execution_mode", "must be one of native, prompt_eval, responses, multi_turn")
	}
	return m.repo.PatchChallengePackDraft(ctx, repository.PatchChallengePackDraftParams{
		ID:            input.DraftID,
		Name:          input.Name,
		ExecutionMode: input.ExecutionMode,
		Composition:   input.Composition,
	})
}

func (m *ChallengePackBuilderManager) DeleteDraft(ctx context.Context, caller Caller, workspaceID, draftID uuid.UUID) error {
	if _, err := m.GetDraft(ctx, caller, workspaceID, draftID); err != nil {
		return err
	}
	return m.repo.DeleteChallengePackDraft(ctx, draftID)
}

func (m *ChallengePackBuilderManager) CompileDraft(ctx context.Context, caller Caller, workspaceID, draftID uuid.UUID) (CompileDraftResult, error) {
	draft, err := m.GetDraft(ctx, caller, workspaceID, draftID)
	if err != nil {
		return CompileDraftResult{}, err
	}

	bundle, composeErr := m.composeDraft(ctx, draft)
	if composeErr != nil {
		return CompileDraftResult{
			Valid:  false,
			Errors: []validationErrorDetail{{Field: "composition", Message: composeErr.Error()}},
		}, nil
	}

	result := CompileDraftResult{SpecCard: challengepack.SpecCardSummary(bundle)}
	if yamlBytes, yamlErr := challengepack.BundleYAML(bundle); yamlErr == nil {
		result.YAML = string(yamlBytes)
	}

	if validationErr := challengepack.ValidateBundle(bundle); validationErr != nil {
		var verrs challengepack.ValidationErrors
		if errors.As(validationErr, &verrs) {
			result.Errors = mapChallengePackValidationErrors(verrs)
			return result, nil
		}
		return CompileDraftResult{}, validationErr
	}

	result.Valid = true
	return result, nil
}

func (m *ChallengePackBuilderManager) PublishDraft(ctx context.Context, caller Caller, workspaceID, draftID uuid.UUID) (PublishChallengePackResponse, error) {
	draft, err := m.GetDraft(ctx, caller, workspaceID, draftID)
	if err != nil {
		return PublishChallengePackResponse{}, err
	}

	bundle, composeErr := m.composeDraft(ctx, draft)
	if composeErr != nil {
		return PublishChallengePackResponse{}, builderValidationError("composition", composeErr.Error())
	}

	bundleYAML, err := challengepack.BundleYAML(bundle)
	if err != nil {
		return PublishChallengePackResponse{}, fmt.Errorf("render draft bundle yaml: %w", err)
	}

	// Reuse the raw-YAML publish path: it re-validates, stores the bundle
	// artifact, and writes the immutable manifest in one transaction.
	resp, err := m.authoring.PublishBundle(ctx, workspaceID, bundleYAML)
	if err != nil {
		return PublishChallengePackResponse{}, err
	}

	// Best-effort: mark the draft published and link it to the new version.
	// The pack is already published; failing this bookkeeping must not fail
	// the request.
	publishedStatus := "published"
	_, _ = m.repo.PatchChallengePackDraft(ctx, repository.PatchChallengePackDraftParams{
		ID:                     draftID,
		Status:                 &publishedStatus,
		LastPublishedVersionID: &resp.ChallengePackVersionID,
		ChallengePackID:        &resp.ChallengePackID,
	})

	return resp, nil
}

// composeDraft resolves a draft's referenced library pieces and assembles the
// runnable Bundle (without validating it — callers validate as needed).
func (m *ChallengePackBuilderManager) composeDraft(ctx context.Context, draft repository.ChallengePackDraft) (challengepack.Bundle, error) {
	var composition challengepack.Composition
	if len(draft.Composition) > 0 {
		if err := json.Unmarshal(draft.Composition, &composition); err != nil {
			return challengepack.Bundle{}, fmt.Errorf("decode draft composition: %w", err)
		}
	}

	resolved := challengepack.ResolvedPieces{}
	if ids := composition.ReferencedPieceIDs(); len(ids) > 0 {
		pieces, err := m.repo.ListChallengePiecesByIDs(ctx, draft.WorkspaceID, ids)
		if err != nil {
			return challengepack.Bundle{}, fmt.Errorf("resolve referenced pieces: %w", err)
		}
		// The query is already workspace-scoped, so every returned piece
		// belongs to the draft's workspace.
		for _, piece := range pieces {
			resolved[piece.ID] = piece.Definition
		}
	}

	return challengepack.ComposeBundle(composition, resolved)
}

func optionalUserID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func isValidPieceKind(kind string) bool {
	switch kind {
	case repository.ChallengePieceKindValidator,
		repository.ChallengePieceKindJudge,
		repository.ChallengePieceKindInputSet,
		repository.ChallengePieceKindChallenge:
		return true
	default:
		return false
	}
}

func isValidExecutionMode(mode string) bool {
	switch mode {
	case challengepack.ExecutionModeNative,
		challengepack.ExecutionModePromptEval,
		challengepack.ExecutionModeResponses,
		challengepack.ExecutionModeMultiTurn:
		return true
	default:
		return false
	}
}

// validatePieceDefinition does shape-only validation: a lone piece must decode
// into its kind's struct. Full cross-reference validation (a validator wired
// into a scorecard) happens at compile time, mirroring how the tools UI defers
// full validation to publish.
func validatePieceDefinition(kind string, definition json.RawMessage) []validationErrorDetail {
	if len(strings.TrimSpace(string(definition))) == 0 {
		return []validationErrorDetail{{Field: "definition", Message: "is required"}}
	}
	var target any
	switch kind {
	case repository.ChallengePieceKindValidator:
		target = &scoring.ValidatorDeclaration{}
	case repository.ChallengePieceKindJudge:
		target = &scoring.LLMJudgeDeclaration{}
	case repository.ChallengePieceKindChallenge:
		target = &challengepack.ChallengeDefinition{}
	case repository.ChallengePieceKindInputSet:
		target = &challengepack.InputSetDefinition{}
	default:
		return []validationErrorDetail{{Field: "kind", Message: "unknown piece kind"}}
	}
	if err := json.Unmarshal(definition, target); err != nil {
		return []validationErrorDetail{{Field: "definition", Message: fmt.Sprintf("must be a valid %s definition: %v", kind, err)}}
	}
	return nil
}

// --- HTTP handlers ---

func listChallengePiecesHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		pieces, err := service.ListPieces(r.Context(), caller, workspaceID, r.URL.Query().Get("kind"))
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		items := make([]challengePieceResponse, 0, len(pieces))
		for _, piece := range pieces {
			items = append(items, buildChallengePieceResponse(piece))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func createChallengePieceHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var req struct {
			Kind        string          `json:"kind"`
			Slug        string          `json:"slug"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Definition  json.RawMessage `json:"definition"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		piece, err := service.CreatePiece(r.Context(), caller, CreatePieceInput{
			WorkspaceID: workspaceID,
			Kind:        req.Kind,
			Slug:        req.Slug,
			Name:        req.Name,
			Description: req.Description,
			Definition:  req.Definition,
			CreatedBy:   caller.UserID,
		})
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, buildChallengePieceResponse(piece))
	}
}

func getChallengePieceHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		pieceID, err := uuid.Parse(chi.URLParam(r, "pieceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_piece_id", "piece id must be a valid UUID")
			return
		}
		piece, err := service.GetPiece(r.Context(), caller, workspaceID, pieceID)
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, buildChallengePieceResponse(piece))
	}
}

func patchChallengePieceHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		pieceID, err := uuid.Parse(chi.URLParam(r, "pieceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_piece_id", "piece id must be a valid UUID")
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var req struct {
			Name        *string         `json:"name"`
			Slug        *string         `json:"slug"`
			Description *string         `json:"description"`
			Definition  json.RawMessage `json:"definition"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		piece, err := service.PatchPiece(r.Context(), caller, PatchPieceInput{
			WorkspaceID: workspaceID,
			PieceID:     pieceID,
			Name:        req.Name,
			Slug:        req.Slug,
			Description: req.Description,
			Definition:  req.Definition,
		})
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, buildChallengePieceResponse(piece))
	}
}

func deleteChallengePieceHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		pieceID, err := uuid.Parse(chi.URLParam(r, "pieceID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_piece_id", "piece id must be a valid UUID")
			return
		}
		if err := service.ArchivePiece(r.Context(), caller, workspaceID, pieceID); err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listChallengePackDraftsHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		drafts, err := service.ListDrafts(r.Context(), caller, workspaceID)
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		items := make([]challengePackDraftResponse, 0, len(drafts))
		for _, draft := range drafts {
			items = append(items, buildChallengePackDraftResponse(draft))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func createChallengePackDraftHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var req struct {
			Name                       string          `json:"name"`
			ExecutionMode              string          `json:"execution_mode"`
			ChallengePackID            *uuid.UUID      `json:"challenge_pack_id"`
			Composition                json.RawMessage `json:"composition"`
			FromChallengePackVersionID *uuid.UUID      `json:"from_challenge_pack_version_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		draft, err := service.CreateDraft(r.Context(), caller, CreateDraftInput{
			WorkspaceID:                workspaceID,
			Name:                       req.Name,
			ExecutionMode:              req.ExecutionMode,
			ChallengePackID:            req.ChallengePackID,
			Composition:                req.Composition,
			CreatedBy:                  caller.UserID,
			FromChallengePackVersionID: req.FromChallengePackVersionID,
		})
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, buildChallengePackDraftResponse(draft))
	}
}

func getChallengePackDraftHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		draftID, err := uuid.Parse(chi.URLParam(r, "draftID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_draft_id", "draft id must be a valid UUID")
			return
		}
		draft, err := service.GetDraft(r.Context(), caller, workspaceID, draftID)
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, buildChallengePackDraftResponse(draft))
	}
}

func patchChallengePackDraftHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		draftID, err := uuid.Parse(chi.URLParam(r, "draftID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_draft_id", "draft id must be a valid UUID")
			return
		}
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}
		var req struct {
			Name          *string         `json:"name"`
			ExecutionMode *string         `json:"execution_mode"`
			Composition   json.RawMessage `json:"composition"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
			return
		}
		draft, err := service.PatchDraft(r.Context(), caller, PatchDraftInput{
			WorkspaceID:   workspaceID,
			DraftID:       draftID,
			Name:          req.Name,
			ExecutionMode: req.ExecutionMode,
			Composition:   req.Composition,
		})
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, buildChallengePackDraftResponse(draft))
	}
}

func deleteChallengePackDraftHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		draftID, err := uuid.Parse(chi.URLParam(r, "draftID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_draft_id", "draft id must be a valid UUID")
			return
		}
		if err := service.DeleteDraft(r.Context(), caller, workspaceID, draftID); err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func compileChallengePackDraftHandler(logger *slog.Logger, service ChallengePackBuilderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		draftID, err := uuid.Parse(chi.URLParam(r, "draftID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_draft_id", "draft id must be a valid UUID")
			return
		}
		result, err := service.CompileDraft(r.Context(), caller, workspaceID, draftID)
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, compileDraftResponse{
			Valid:    result.Valid,
			Errors:   nonNilValidationErrors(result.Errors),
			SpecCard: result.SpecCard,
			YAML:     result.YAML,
		})
	}
}

func publishChallengePackDraftHandler(logger *slog.Logger, service ChallengePackBuilderService, entitlementGate EntitlementGateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, workspaceID, ok := builderRequestContext(w, r)
		if !ok {
			return
		}
		draftID, err := uuid.Parse(chi.URLParam(r, "draftID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_draft_id", "draft id must be a valid UUID")
			return
		}
		if entitlementGate != nil {
			if err := entitlementGate.CheckWorkspaceFeature(r.Context(), workspaceID, billingpkg.FeaturePrivateChallengePacks); err != nil {
				writeChallengePackEntitlementError(w, logger, err)
				return
			}
		}
		result, err := service.PublishDraft(r.Context(), caller, workspaceID, draftID)
		if err != nil {
			handleChallengePackBuilderError(w, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

// builderRequestContext resolves the caller and the URL workspace id, writing
// the appropriate error response and returning ok=false on failure.
func builderRequestContext(w http.ResponseWriter, r *http.Request) (Caller, uuid.UUID, bool) {
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

func handleChallengePackBuilderError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var builderValidation ChallengePackBuilderValidationError
	var authoringValidation ChallengePackAuthoringValidationError
	switch {
	case errors.Is(err, errChallengePackBuilderUnavailable):
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "challenge pack builder is not configured")
	case errors.As(err, &builderValidation):
		writeJSON(w, http.StatusBadRequest, ValidateChallengePackResponse{Valid: false, Errors: builderValidation.Errors})
	case errors.As(err, &authoringValidation):
		writeJSON(w, http.StatusBadRequest, ValidateChallengePackResponse{Valid: false, Errors: authoringValidation.Errors})
	case errors.Is(err, ErrUnauthenticated), errors.Is(err, ErrCallerMissing), errors.Is(err, ErrForbidden):
		writeAuthzError(w, err)
	case errors.Is(err, repository.ErrChallengePieceNotFound):
		writeError(w, http.StatusNotFound, "challenge_piece_not_found", "challenge piece not found")
	case errors.Is(err, repository.ErrChallengePackDraftNotFound):
		writeError(w, http.StatusNotFound, "challenge_pack_draft_not_found", "challenge pack draft not found")
	case errors.Is(err, repository.ErrChallengePackVersionNotFound):
		writeError(w, http.StatusNotFound, "challenge_pack_version_not_found", "challenge pack version not found")
	case errors.Is(err, repository.ErrChallengePieceSlugConflict):
		writeError(w, http.StatusConflict, "challenge_piece_slug_conflict", "a piece with this slug already exists in the workspace")
	case errors.Is(err, repository.ErrChallengePackVersionExists):
		writeError(w, http.StatusConflict, "challenge_pack_version_exists", "a pack version with this number already exists")
	case errors.Is(err, repository.ErrChallengePackMetadataConflict):
		writeError(w, http.StatusConflict, "challenge_pack_metadata_conflict", "pack metadata conflicts with an existing pack")
	default:
		logger.Error("challenge pack builder operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// --- response shapes ---

type challengePieceResponse struct {
	ID              uuid.UUID       `json:"id"`
	WorkspaceID     uuid.UUID       `json:"workspace_id"`
	Kind            string          `json:"kind"`
	Slug            string          `json:"slug"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Definition      json.RawMessage `json:"definition"`
	LifecycleStatus string          `json:"lifecycle_status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type challengePackDraftResponse struct {
	ID                     uuid.UUID       `json:"id"`
	WorkspaceID            uuid.UUID       `json:"workspace_id"`
	Name                   string          `json:"name"`
	ExecutionMode          string          `json:"execution_mode"`
	ChallengePackID        *uuid.UUID      `json:"challenge_pack_id,omitempty"`
	Composition            json.RawMessage `json:"composition"`
	Status                 string          `json:"status"`
	LastPublishedVersionID *uuid.UUID      `json:"last_published_version_id,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

type compileDraftResponse struct {
	Valid    bool                    `json:"valid"`
	Errors   []validationErrorDetail `json:"errors"`
	SpecCard challengepack.SpecCard  `json:"spec_card"`
	YAML     string                  `json:"yaml"`
}

func buildChallengePieceResponse(piece repository.ChallengePiece) challengePieceResponse {
	definition := piece.Definition
	if len(definition) == 0 {
		definition = json.RawMessage("{}")
	}
	return challengePieceResponse{
		ID:              piece.ID,
		WorkspaceID:     piece.WorkspaceID,
		Kind:            piece.Kind,
		Slug:            piece.Slug,
		Name:            piece.Name,
		Description:     piece.Description,
		Definition:      definition,
		LifecycleStatus: piece.LifecycleStatus,
		CreatedAt:       piece.CreatedAt,
		UpdatedAt:       piece.UpdatedAt,
	}
}

func buildChallengePackDraftResponse(draft repository.ChallengePackDraft) challengePackDraftResponse {
	composition := draft.Composition
	if len(composition) == 0 {
		composition = json.RawMessage("{}")
	}
	return challengePackDraftResponse{
		ID:                     draft.ID,
		WorkspaceID:            draft.WorkspaceID,
		Name:                   draft.Name,
		ExecutionMode:          draft.ExecutionMode,
		ChallengePackID:        draft.ChallengePackID,
		Composition:            composition,
		Status:                 draft.Status,
		LastPublishedVersionID: draft.LastPublishedVersionID,
		CreatedAt:              draft.CreatedAt,
		UpdatedAt:              draft.UpdatedAt,
	}
}

func nonNilValidationErrors(errs []validationErrorDetail) []validationErrorDetail {
	if errs == nil {
		return []validationErrorDetail{}
	}
	return errs
}

// noopChallengePackBuilderService is used when the builder is not wired (e.g.
// the lightweight test router).
type noopChallengePackBuilderService struct{}

var errChallengePackBuilderUnavailable = errors.New("challenge pack builder service not configured")

func (noopChallengePackBuilderService) ListPieces(context.Context, Caller, uuid.UUID, string) ([]repository.ChallengePiece, error) {
	return nil, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) CreatePiece(context.Context, Caller, CreatePieceInput) (repository.ChallengePiece, error) {
	return repository.ChallengePiece{}, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) GetPiece(context.Context, Caller, uuid.UUID, uuid.UUID) (repository.ChallengePiece, error) {
	return repository.ChallengePiece{}, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) PatchPiece(context.Context, Caller, PatchPieceInput) (repository.ChallengePiece, error) {
	return repository.ChallengePiece{}, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) ArchivePiece(context.Context, Caller, uuid.UUID, uuid.UUID) error {
	return errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) ListDrafts(context.Context, Caller, uuid.UUID) ([]repository.ChallengePackDraft, error) {
	return nil, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) CreateDraft(context.Context, Caller, CreateDraftInput) (repository.ChallengePackDraft, error) {
	return repository.ChallengePackDraft{}, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) GetDraft(context.Context, Caller, uuid.UUID, uuid.UUID) (repository.ChallengePackDraft, error) {
	return repository.ChallengePackDraft{}, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) PatchDraft(context.Context, Caller, PatchDraftInput) (repository.ChallengePackDraft, error) {
	return repository.ChallengePackDraft{}, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) DeleteDraft(context.Context, Caller, uuid.UUID, uuid.UUID) error {
	return errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) CompileDraft(context.Context, Caller, uuid.UUID, uuid.UUID) (CompileDraftResult, error) {
	return CompileDraftResult{}, errChallengePackBuilderUnavailable
}

func (noopChallengePackBuilderService) PublishDraft(context.Context, Caller, uuid.UUID, uuid.UUID) (PublishChallengePackResponse, error) {
	return PublishChallengePackResponse{}, errChallengePackBuilderUnavailable
}
