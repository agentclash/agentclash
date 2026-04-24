package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PublicShareRepository interface {
	CreatePublicShareLink(ctx context.Context, params repository.CreatePublicShareLinkParams) (repository.PublicShareLink, error)
	RevokePublicShareLink(ctx context.Context, id uuid.UUID) error
	GetPublicShareLinkByID(ctx context.Context, id uuid.UUID) (repository.PublicShareLink, error)
	GetActivePublicShareLinkByKey(ctx context.Context, key string) (repository.PublicShareLink, error)
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	GetRunnableChallengePackVersionByID(ctx context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error)
	GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error)
	GetRunScorecardByRunID(ctx context.Context, runID uuid.UUID) (repository.RunScorecard, error)
	GetRunAgentByID(ctx context.Context, id uuid.UUID) (domain.RunAgent, error)
	ListRunAgentsByRunID(ctx context.Context, runID uuid.UUID) ([]domain.RunAgent, error)
	GetRunAgentScorecardByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentScorecard, error)
	GetRunAgentReplayByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentReplay, error)
	GetPublicChallengePackVersionSnapshot(ctx context.Context, versionID uuid.UUID) (repository.PublicChallengePackVersionSnapshot, error)
	GetPublicRunScorecardSnapshot(ctx context.Context, runID uuid.UUID) (repository.PublicRunScorecardSnapshot, error)
	GetPublicRunAgentScorecardSnapshot(ctx context.Context, runAgentID uuid.UUID) (repository.PublicRunAgentScorecardSnapshot, error)
	GetPublicRunAgentReplaySnapshot(ctx context.Context, runAgentID uuid.UUID) (repository.PublicRunAgentReplaySnapshot, error)
}

type PublicShareService interface {
	CreateShareLink(ctx context.Context, caller Caller, input CreateShareLinkInput) (CreateShareLinkResult, error)
	RevokeShareLink(ctx context.Context, caller Caller, shareID uuid.UUID) error
	GetPublicShare(ctx context.Context, token string) (PublicSharePayload, error)
}

type CreateShareLinkInput struct {
	ResourceType   repository.PublicShareResourceType
	ResourceID     uuid.UUID
	SearchIndexing bool
	ExpiresAt      *time.Time
}

type CreateShareLinkResult struct {
	Share repository.PublicShareLink
	Token string
	URL   string
}

type PublicSharePayload struct {
	Share    publicShareLinkResponse `json:"share"`
	Resource any                     `json:"resource"`
}

type PublicShareManager struct {
	authorizer  WorkspaceAuthorizer
	repo        PublicShareRepository
	frontendURL string
}

func NewPublicShareManager(authorizer WorkspaceAuthorizer, repo PublicShareRepository, frontendURL string) *PublicShareManager {
	return &PublicShareManager{
		authorizer:  authorizer,
		repo:        repo,
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

func (m *PublicShareManager) CreateShareLink(ctx context.Context, caller Caller, input CreateShareLinkInput) (CreateShareLinkResult, error) {
	organizationID, workspaceID, err := m.authorizedResourceScope(ctx, caller, input)
	if err != nil {
		return CreateShareLinkResult{}, err
	}

	key, err := newShareKey()
	if err != nil {
		return CreateShareLinkResult{}, err
	}
	callerID := caller.UserID
	share, err := m.repo.CreatePublicShareLink(ctx, repository.CreatePublicShareLinkParams{
		Key:             key,
		OrganizationID:  organizationID,
		WorkspaceID:     workspaceID,
		ResourceType:    input.ResourceType,
		ResourceID:      input.ResourceID,
		CreatedByUserID: &callerID,
		SearchIndexing:  input.SearchIndexing,
		ExpiresAt:       input.ExpiresAt,
	})
	if err != nil {
		return CreateShareLinkResult{}, err
	}

	return CreateShareLinkResult{
		Share: share,
		Token: share.Key,
		URL:   m.shareURL(share.Key),
	}, nil
}

func (m *PublicShareManager) RevokeShareLink(ctx context.Context, caller Caller, shareID uuid.UUID) error {
	share, err := m.repo.GetPublicShareLinkByID(ctx, shareID)
	if err != nil {
		return err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, share.WorkspaceID); err != nil {
		return err
	}
	return m.repo.RevokePublicShareLink(ctx, shareID)
}

func (m *PublicShareManager) GetPublicShare(ctx context.Context, token string) (PublicSharePayload, error) {
	key := strings.TrimSpace(token)
	if key == "" {
		return PublicSharePayload{}, repository.ErrPublicShareLinkNotFound
	}
	share, err := m.repo.GetActivePublicShareLinkByKey(ctx, key)
	if err != nil {
		return PublicSharePayload{}, err
	}

	resource, err := m.publicResource(ctx, share)
	if err != nil {
		return PublicSharePayload{}, err
	}
	return PublicSharePayload{
		Share:    mapPublicShareLink(share, ""),
		Resource: resource,
	}, nil
}

func (m *PublicShareManager) authorizedResourceScope(ctx context.Context, caller Caller, input CreateShareLinkInput) (uuid.UUID, uuid.UUID, error) {
	switch input.ResourceType {
	case repository.PublicShareResourceChallengePackVersion:
		version, err := m.repo.GetRunnableChallengePackVersionByID(ctx, input.ResourceID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if version.WorkspaceID == nil {
			return uuid.Nil, uuid.Nil, repository.ErrChallengePackVersionNotFound
		}
		if err := m.authorizer.AuthorizeWorkspace(ctx, caller, *version.WorkspaceID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		organizationID, err := m.repo.GetOrganizationIDByWorkspaceID(ctx, *version.WorkspaceID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return organizationID, *version.WorkspaceID, nil
	case repository.PublicShareResourceRunScorecard:
		run, err := m.repo.GetRunByID(ctx, input.ResourceID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if err := m.authorizer.AuthorizeWorkspace(ctx, caller, run.WorkspaceID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if _, err := m.repo.GetRunScorecardByRunID(ctx, run.ID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return run.OrganizationID, run.WorkspaceID, nil
	case repository.PublicShareResourceRunAgentScorecard:
		runAgent, err := m.repo.GetRunAgentByID(ctx, input.ResourceID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if err := m.authorizer.AuthorizeWorkspace(ctx, caller, runAgent.WorkspaceID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if _, err := m.repo.GetRunAgentScorecardByRunAgentID(ctx, runAgent.ID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return runAgent.OrganizationID, runAgent.WorkspaceID, nil
	case repository.PublicShareResourceRunAgentReplay:
		runAgent, err := m.repo.GetRunAgentByID(ctx, input.ResourceID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if err := m.authorizer.AuthorizeWorkspace(ctx, caller, runAgent.WorkspaceID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if _, err := m.repo.GetRunAgentReplayByRunAgentID(ctx, runAgent.ID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return runAgent.OrganizationID, runAgent.WorkspaceID, nil
	default:
		return uuid.Nil, uuid.Nil, errInvalidShareResourceType
	}
}

func (m *PublicShareManager) publicResource(ctx context.Context, share repository.PublicShareLink) (any, error) {
	switch share.ResourceType {
	case repository.PublicShareResourceChallengePackVersion:
		snapshot, err := m.repo.GetPublicChallengePackVersionSnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, err
		}
		return mapPublicChallengePackVersion(snapshot), nil
	case repository.PublicShareResourceRunScorecard:
		snapshot, err := m.repo.GetPublicRunScorecardSnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, err
		}
		return mapPublicRunScorecard(snapshot), nil
	case repository.PublicShareResourceRunAgentScorecard:
		snapshot, err := m.repo.GetPublicRunAgentScorecardSnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, err
		}
		return mapPublicRunAgentScorecard(snapshot), nil
	case repository.PublicShareResourceRunAgentReplay:
		snapshot, err := m.repo.GetPublicRunAgentReplaySnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, err
		}
		return mapPublicRunAgentReplay(snapshot), nil
	default:
		return nil, repository.ErrPublicShareLinkNotFound
	}
}

func (m *PublicShareManager) shareURL(token string) string {
	base := m.frontendURL
	if base == "" {
		base = "https://agentclash.dev"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "https://agentclash.dev/share/" + token
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/share/" + token
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

type createShareLinkRequest struct {
	ResourceType   string     `json:"resource_type"`
	ResourceID     uuid.UUID  `json:"resource_id"`
	SearchIndexing bool       `json:"search_indexing"`
	ExpiresAt      *time.Time `json:"expires_at"`
}

type createShareLinkResponse struct {
	Share publicShareLinkResponse `json:"share"`
	Token string                  `json:"token"`
	URL   string                  `json:"url"`
}

type publicShareLinkResponse struct {
	ID             uuid.UUID  `json:"id"`
	ResourceType   string     `json:"resource_type"`
	ResourceID     uuid.UUID  `json:"resource_id"`
	SearchIndexing bool       `json:"search_indexing"`
	ViewCount      int64      `json:"view_count"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	URL            string     `json:"url,omitempty"`
}

func createShareLinkHandler(logger *slog.Logger, service PublicShareService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		var request createShareLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_share_request", "request body must be valid JSON")
			return
		}
		resourceType := repository.PublicShareResourceType(strings.TrimSpace(request.ResourceType))
		if !validPublicShareResourceType(resourceType) || request.ResourceID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "invalid_share_request", "resource_type and resource_id are required")
			return
		}
		result, err := service.CreateShareLink(r.Context(), caller, CreateShareLinkInput{
			ResourceType:   resourceType,
			ResourceID:     request.ResourceID,
			SearchIndexing: request.SearchIndexing,
			ExpiresAt:      request.ExpiresAt,
		})
		if err != nil {
			writeShareError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, createShareLinkResponse{
			Share: mapPublicShareLink(result.Share, result.URL),
			Token: result.Token,
			URL:   result.URL,
		})
	}
}

func revokeShareLinkHandler(logger *slog.Logger, service PublicShareService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := CallerFromContext(r.Context())
		if err != nil {
			writeAuthzError(w, err)
			return
		}
		shareID, err := uuid.Parse(chi.URLParam(r, "shareID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_share_id", "share id must be a valid UUID")
			return
		}
		if err := service.RevokeShareLink(r.Context(), caller, shareID); err != nil {
			writeShareError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

func getPublicShareHandler(logger *slog.Logger, service PublicShareService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		result, err := service.GetPublicShare(r.Context(), token)
		if err != nil {
			writeShareError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func writeShareError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "workspace access denied")
	case errors.Is(err, repository.ErrPublicShareLinkNotFound),
		errors.Is(err, repository.ErrChallengePackVersionNotFound),
		errors.Is(err, repository.ErrRunNotFound),
		errors.Is(err, repository.ErrRunScorecardNotFound),
		errors.Is(err, repository.ErrRunAgentNotFound),
		errors.Is(err, repository.ErrRunAgentScorecardNotFound),
		errors.Is(err, repository.ErrRunAgentReplayNotFound):
		writeError(w, http.StatusNotFound, "not_found", "shared resource not found")
	case errors.Is(err, errInvalidShareResourceType):
		writeError(w, http.StatusBadRequest, "invalid_share_request", "unsupported resource_type")
	default:
		logger.Error("public share request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

var errInvalidShareResourceType = errors.New("invalid public share resource type")

func newShareKey() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate share key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func validPublicShareResourceType(resourceType repository.PublicShareResourceType) bool {
	switch resourceType {
	case repository.PublicShareResourceChallengePackVersion,
		repository.PublicShareResourceRunScorecard,
		repository.PublicShareResourceRunAgentScorecard,
		repository.PublicShareResourceRunAgentReplay:
		return true
	default:
		return false
	}
}

func mapPublicShareLink(share repository.PublicShareLink, shareURL string) publicShareLinkResponse {
	return publicShareLinkResponse{
		ID:             share.ID,
		ResourceType:   string(share.ResourceType),
		ResourceID:     share.ResourceID,
		SearchIndexing: share.SearchIndexing,
		ViewCount:      share.ViewCount,
		ExpiresAt:      share.ExpiresAt,
		CreatedAt:      share.CreatedAt,
		UpdatedAt:      share.UpdatedAt,
		URL:            shareURL,
	}
}

func mapPublicChallengePackVersion(snapshot repository.PublicChallengePackVersionSnapshot) any {
	inputSets := make([]challengeInputSetResponse, 0, len(snapshot.InputSets))
	for _, inputSet := range snapshot.InputSets {
		inputSets = append(inputSets, challengeInputSetResponse{
			ID:                     inputSet.ID,
			ChallengePackVersionID: inputSet.ChallengePackVersionID,
			InputKey:               inputSet.InputKey,
			Name:                   inputSet.Name,
		})
	}
	return map[string]any{
		"type": "challenge_pack_version",
		"pack": map[string]any{
			"id":          snapshot.PackID,
			"slug":        snapshot.PackSlug,
			"name":        snapshot.PackName,
			"family":      snapshot.PackFamily,
			"description": snapshot.PackDescription,
		},
		"version": map[string]any{
			"id":               snapshot.VersionID,
			"version_number":   snapshot.VersionNumber,
			"lifecycle_status": snapshot.LifecycleStatus,
			"manifest":         snapshot.Manifest,
			"input_sets":       inputSets,
			"created_at":       snapshot.CreatedAt,
			"updated_at":       snapshot.UpdatedAt,
		},
	}
}

func mapPublicRunScorecard(snapshot repository.PublicRunScorecardSnapshot) any {
	return map[string]any{
		"type":             "run_scorecard",
		"run":              mapPublicRun(snapshot.Run),
		"agents":           mapPublicRunAgents(snapshot.Agents),
		"agent_scorecards": mapPublicRunAgentScorecards(snapshot.AgentScorecards),
		"scorecard":        mapRunScorecardResponse(snapshot.Scorecard),
	}
}

func mapPublicRunAgentScorecard(snapshot repository.PublicRunAgentScorecardSnapshot) any {
	return map[string]any{
		"type":             "run_agent_scorecard",
		"run":              mapPublicRun(snapshot.Run),
		"run_agent":        mapPublicRunAgent(snapshot.RunAgent),
		"sibling_agents":   mapPublicRunAgents(snapshot.SiblingAgents),
		"agent_scorecards": mapPublicRunAgentScorecards(snapshot.AgentScorecards),
		"scorecard":        mapRunAgentScorecardPublicResponse(snapshot.Scorecard),
	}
}

func mapPublicRunAgentReplay(snapshot repository.PublicRunAgentReplaySnapshot) any {
	return map[string]any{
		"type":      "run_agent_replay",
		"run":       mapPublicRun(snapshot.Run),
		"run_agent": mapPublicRunAgent(snapshot.RunAgent),
		"replay": map[string]any{
			"id":                     snapshot.Replay.ID,
			"run_agent_id":           snapshot.Replay.RunAgentID,
			"summary":                snapshot.Replay.Summary,
			"latest_sequence_number": snapshot.Replay.LatestSequenceNumber,
			"event_count":            snapshot.Replay.EventCount,
			"created_at":             snapshot.Replay.CreatedAt,
			"updated_at":             snapshot.Replay.UpdatedAt,
		},
	}
}

func mapPublicRun(run domain.Run) map[string]any {
	return map[string]any{
		"id":                        run.ID,
		"challenge_pack_version_id": run.ChallengePackVersionID,
		"challenge_input_set_id":    run.ChallengeInputSetID,
		"name":                      run.Name,
		"status":                    run.Status,
		"execution_mode":            run.ExecutionMode,
		"started_at":                run.StartedAt,
		"finished_at":               run.FinishedAt,
		"created_at":                run.CreatedAt,
	}
}

func mapPublicRunAgent(runAgent domain.RunAgent) map[string]any {
	return map[string]any{
		"id":             runAgent.ID,
		"run_id":         runAgent.RunID,
		"lane_index":     runAgent.LaneIndex,
		"label":          runAgent.Label,
		"status":         runAgent.Status,
		"started_at":     runAgent.StartedAt,
		"finished_at":    runAgent.FinishedAt,
		"failure_reason": runAgent.FailureReason,
	}
}

func mapPublicRunAgents(agents []domain.RunAgent) []map[string]any {
	items := make([]map[string]any, 0, len(agents))
	for _, agent := range agents {
		items = append(items, mapPublicRunAgent(agent))
	}
	return items
}

func mapPublicRunAgentScorecards(scorecards []repository.RunAgentScorecard) []map[string]any {
	items := make([]map[string]any, 0, len(scorecards))
	for _, scorecard := range scorecards {
		items = append(items, mapRunAgentScorecardPublicResponse(scorecard))
	}
	return items
}

func mapRunScorecardResponse(scorecard repository.RunScorecard) map[string]any {
	return map[string]any{
		"id":                   scorecard.ID,
		"run_id":               scorecard.RunID,
		"evaluation_spec_id":   scorecard.EvaluationSpecID,
		"winning_run_agent_id": scorecard.WinningRunAgentID,
		"scorecard":            scorecard.Scorecard,
		"created_at":           scorecard.CreatedAt,
		"updated_at":           scorecard.UpdatedAt,
	}
}

func mapRunAgentScorecardPublicResponse(scorecard repository.RunAgentScorecard) map[string]any {
	return map[string]any{
		"id":                 scorecard.ID,
		"run_agent_id":       scorecard.RunAgentID,
		"evaluation_spec_id": scorecard.EvaluationSpecID,
		"overall_score":      scorecard.OverallScore,
		"correctness_score":  scorecard.CorrectnessScore,
		"reliability_score":  scorecard.ReliabilityScore,
		"latency_score":      scorecard.LatencyScore,
		"cost_score":         scorecard.CostScore,
		"behavioral_score":   scorecard.BehavioralScore,
		"passed":             scorecard.Passed,
		"scorecard":          scorecard.Scorecard,
		"created_at":         scorecard.CreatedAt,
		"updated_at":         scorecard.UpdatedAt,
	}
}
