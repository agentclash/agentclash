package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestPublicShareManager_CreateChallengePackShareAuthorizesWorkspace(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	versionID := uuid.New()
	repo := newFakePublicShareRepository(orgID, workspaceID)
	repo.version = repository.RunnableChallengePackVersion{
		ID:              versionID,
		ChallengePackID: uuid.New(),
		WorkspaceID:     &workspaceID,
	}
	manager := NewPublicShareManager(NewCallerWorkspaceAuthorizer(), repo, "https://agentclash.dev")

	result, err := manager.CreateShareLink(ctx, callerWithWorkspace(workspaceID), CreateShareLinkInput{
		ResourceType: repository.PublicShareResourceChallengePackVersion,
		ResourceID:   versionID,
	})
	if err != nil {
		t.Fatalf("CreateShareLink returned error: %v", err)
	}
	if result.Share.ResourceType != repository.PublicShareResourceChallengePackVersion {
		t.Fatalf("resource type = %q", result.Share.ResourceType)
	}
	if result.Share.WorkspaceID != workspaceID || result.Share.OrganizationID != orgID {
		t.Fatalf("share scope = org %s workspace %s, want org %s workspace %s", result.Share.OrganizationID, result.Share.WorkspaceID, orgID, workspaceID)
	}
	if result.Token == "" || result.URL == "" {
		t.Fatalf("token/url should be populated: token=%q url=%q", result.Token, result.URL)
	}
}

func TestPublicShareManager_CreateShareRejectsCrossWorkspaceCaller(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()
	runID := uuid.New()
	repo := newFakePublicShareRepository(orgID, workspaceID)
	repo.run = domain.Run{
		ID:             runID,
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Name:           "private run",
		Status:         domain.RunStatusCompleted,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	repo.runScorecard = repository.RunScorecard{ID: uuid.New(), RunID: runID, Scorecard: json.RawMessage(`{"ok":true}`)}
	manager := NewPublicShareManager(NewCallerWorkspaceAuthorizer(), repo, "https://agentclash.dev")

	_, err := manager.CreateShareLink(ctx, callerWithWorkspace(otherWorkspaceID), CreateShareLinkInput{
		ResourceType: repository.PublicShareResourceRunScorecard,
		ResourceID:   runID,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateShareLink error = %v, want ErrForbidden", err)
	}
}

func TestPublicShareManager_GetPublicShareReturnsNarrowPayload(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	runID := uuid.New()
	repo := newFakePublicShareRepository(orgID, workspaceID)
	repo.share = repository.PublicShareLink{
		ID:             uuid.New(),
		Key:            "share-key",
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		ResourceType:   repository.PublicShareResourceRunScorecard,
		ResourceID:     runID,
		IsActive:       true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	repo.run = domain.Run{
		ID:             runID,
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Name:           "blog run",
		Status:         domain.RunStatusCompleted,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	repo.runScorecard = repository.RunScorecard{ID: uuid.New(), RunID: runID, Scorecard: json.RawMessage(`{"summary":"public"}`)}
	manager := NewPublicShareManager(NewCallerWorkspaceAuthorizer(), repo, "https://agentclash.dev")

	payload, err := manager.GetPublicShare(ctx, "share-key")
	if err != nil {
		t.Fatalf("GetPublicShare returned error: %v", err)
	}
	if payload.Share.ResourceType != string(repository.PublicShareResourceRunScorecard) {
		t.Fatalf("share resource type = %q", payload.Share.ResourceType)
	}
	encoded, err := json.Marshal(payload.Resource)
	if err != nil {
		t.Fatalf("marshal payload resource: %v", err)
	}
	if string(encoded) == "" || !json.Valid(encoded) {
		t.Fatalf("payload is not valid JSON: %s", encoded)
	}
	if contains := jsonContainsKey(encoded, "workspace_id"); contains {
		t.Fatalf("public payload leaked workspace_id: %s", encoded)
	}
}

func TestPublicShareManager_GetPublicShareKeepsReplayAgentDistinct(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	workspaceID := uuid.New()
	runID := uuid.New()
	firstAgentID := uuid.New()
	secondAgentID := uuid.New()
	repo := newFakePublicShareRepository(orgID, workspaceID)
	repo.share = repository.PublicShareLink{
		ID:             uuid.New(),
		Key:            "second-agent-replay",
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		ResourceType:   repository.PublicShareResourceRunAgentReplay,
		ResourceID:     secondAgentID,
		IsActive:       true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	repo.run = domain.Run{
		ID:             runID,
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Name:           "distinct agents",
		Status:         domain.RunStatusCompleted,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	repo.runAgent = domain.RunAgent{
		ID:             secondAgentID,
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		RunID:          runID,
		LaneIndex:      1,
		Label:          "anthropic",
		Status:         domain.RunAgentStatusCompleted,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	repo.replay = repository.RunAgentReplay{
		ID:         uuid.New(),
		RunAgentID: secondAgentID,
		Summary:    json.RawMessage(`{"steps":[{"headline":"second agent step"}]}`),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	manager := NewPublicShareManager(NewCallerWorkspaceAuthorizer(), repo, "https://agentclash.dev")

	payload, err := manager.GetPublicShare(ctx, "second-agent-replay")
	if err != nil {
		t.Fatalf("GetPublicShare returned error: %v", err)
	}
	resource, ok := payload.Resource.(map[string]any)
	if !ok {
		t.Fatalf("resource type = %T, want map", payload.Resource)
	}
	runAgent, ok := resource["run_agent"].(map[string]any)
	if !ok {
		t.Fatalf("run_agent type = %T, want map", resource["run_agent"])
	}
	if got := runAgent["id"]; got != secondAgentID {
		t.Fatalf("shared replay agent id = %v, want %s and not %s", got, secondAgentID, firstAgentID)
	}
}

func callerWithWorkspace(workspaceID uuid.UUID) Caller {
	userID := uuid.New()
	return Caller{
		UserID: userID,
		WorkspaceMemberships: map[uuid.UUID]WorkspaceMembership{
			workspaceID: {WorkspaceID: workspaceID, Role: "workspace_admin"},
		},
		OrganizationMemberships: map[uuid.UUID]OrganizationMembership{},
	}
}

func jsonContainsKey(data []byte, key string) bool {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false
	}
	return jsonValueContainsKey(value, key)
}

func jsonValueContainsKey(value any, key string) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, ok := typed[key]; ok {
			return true
		}
		for _, child := range typed {
			if jsonValueContainsKey(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if jsonValueContainsKey(child, key) {
				return true
			}
		}
	}
	return false
}

type fakePublicShareRepository struct {
	orgID          uuid.UUID
	workspaceID    uuid.UUID
	share          repository.PublicShareLink
	version        repository.RunnableChallengePackVersion
	run            domain.Run
	runAgent       domain.RunAgent
	runScorecard   repository.RunScorecard
	agentScorecard repository.RunAgentScorecard
	replay         repository.RunAgentReplay
}

func newFakePublicShareRepository(orgID, workspaceID uuid.UUID) *fakePublicShareRepository {
	return &fakePublicShareRepository{orgID: orgID, workspaceID: workspaceID}
}

func (r *fakePublicShareRepository) CreatePublicShareLink(_ context.Context, params repository.CreatePublicShareLinkParams) (repository.PublicShareLink, error) {
	r.share = repository.PublicShareLink{
		ID:              uuid.New(),
		Key:             params.Key,
		OrganizationID:  params.OrganizationID,
		WorkspaceID:     params.WorkspaceID,
		ResourceType:    params.ResourceType,
		ResourceID:      params.ResourceID,
		CreatedByUserID: params.CreatedByUserID,
		IsActive:        true,
		SearchIndexing:  params.SearchIndexing,
		ExpiresAt:       params.ExpiresAt,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	return r.share, nil
}

func (r *fakePublicShareRepository) RevokePublicShareLink(_ context.Context, id uuid.UUID) error {
	if r.share.ID != id {
		return repository.ErrPublicShareLinkNotFound
	}
	r.share.IsActive = false
	return nil
}

func (r *fakePublicShareRepository) GetPublicShareLinkByID(_ context.Context, id uuid.UUID) (repository.PublicShareLink, error) {
	if r.share.ID != id {
		return repository.PublicShareLink{}, repository.ErrPublicShareLinkNotFound
	}
	return r.share, nil
}

func (r *fakePublicShareRepository) GetActivePublicShareLinkByKey(_ context.Context, key string) (repository.PublicShareLink, error) {
	if r.share.Key != key || !r.share.IsActive {
		return repository.PublicShareLink{}, repository.ErrPublicShareLinkNotFound
	}
	r.share.ViewCount++
	return r.share, nil
}

func (r *fakePublicShareRepository) GetOrganizationIDByWorkspaceID(_ context.Context, workspaceID uuid.UUID) (uuid.UUID, error) {
	if workspaceID != r.workspaceID {
		return uuid.Nil, repository.ErrWorkspaceSecretNotFound
	}
	return r.orgID, nil
}

func (r *fakePublicShareRepository) GetRunnableChallengePackVersionByID(_ context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error) {
	if r.version.ID != id {
		return repository.RunnableChallengePackVersion{}, repository.ErrChallengePackVersionNotFound
	}
	return r.version, nil
}

func (r *fakePublicShareRepository) GetRunByID(_ context.Context, id uuid.UUID) (domain.Run, error) {
	if r.run.ID != id {
		return domain.Run{}, repository.ErrRunNotFound
	}
	return r.run, nil
}

func (r *fakePublicShareRepository) GetRunScorecardByRunID(_ context.Context, runID uuid.UUID) (repository.RunScorecard, error) {
	if r.runScorecard.RunID != runID {
		return repository.RunScorecard{}, repository.ErrRunScorecardNotFound
	}
	return r.runScorecard, nil
}

func (r *fakePublicShareRepository) GetRunAgentByID(_ context.Context, id uuid.UUID) (domain.RunAgent, error) {
	if r.runAgent.ID != id {
		return domain.RunAgent{}, repository.ErrRunAgentNotFound
	}
	return r.runAgent, nil
}

func (r *fakePublicShareRepository) ListRunAgentsByRunID(_ context.Context, runID uuid.UUID) ([]domain.RunAgent, error) {
	if r.run.ID != runID {
		return nil, repository.ErrRunNotFound
	}
	agents := []domain.RunAgent{}
	if r.runAgent.ID != uuid.Nil {
		agents = append(agents, r.runAgent)
	}
	return agents, nil
}

func (r *fakePublicShareRepository) GetRunAgentScorecardByRunAgentID(_ context.Context, runAgentID uuid.UUID) (repository.RunAgentScorecard, error) {
	if r.agentScorecard.RunAgentID != runAgentID {
		return repository.RunAgentScorecard{}, repository.ErrRunAgentScorecardNotFound
	}
	return r.agentScorecard, nil
}

func (r *fakePublicShareRepository) GetRunAgentReplayByRunAgentID(_ context.Context, runAgentID uuid.UUID) (repository.RunAgentReplay, error) {
	if r.replay.RunAgentID != runAgentID {
		return repository.RunAgentReplay{}, repository.ErrRunAgentReplayNotFound
	}
	return r.replay, nil
}

func (r *fakePublicShareRepository) GetPublicChallengePackVersionSnapshot(context.Context, uuid.UUID) (repository.PublicChallengePackVersionSnapshot, error) {
	return repository.PublicChallengePackVersionSnapshot{
		PackID:          uuid.New(),
		PackSlug:        "pack",
		PackName:        "Pack",
		PackFamily:      "evals",
		VersionID:       r.version.ID,
		VersionNumber:   1,
		LifecycleStatus: "runnable",
		Manifest:        json.RawMessage(`{"schema_version":1}`),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

func (r *fakePublicShareRepository) GetPublicRunScorecardSnapshot(_ context.Context, runID uuid.UUID) (repository.PublicRunScorecardSnapshot, error) {
	if r.run.ID != runID {
		return repository.PublicRunScorecardSnapshot{}, repository.ErrRunNotFound
	}
	return repository.PublicRunScorecardSnapshot{Run: r.run, Agents: []domain.RunAgent{r.runAgent}, AgentScorecards: []repository.RunAgentScorecard{r.agentScorecard}, Scorecard: r.runScorecard}, nil
}

func (r *fakePublicShareRepository) GetPublicRunAgentScorecardSnapshot(_ context.Context, runAgentID uuid.UUID) (repository.PublicRunAgentScorecardSnapshot, error) {
	if r.runAgent.ID != runAgentID {
		return repository.PublicRunAgentScorecardSnapshot{}, repository.ErrRunAgentNotFound
	}
	return repository.PublicRunAgentScorecardSnapshot{Run: r.run, RunAgent: r.runAgent, SiblingAgents: []domain.RunAgent{r.runAgent}, AgentScorecards: []repository.RunAgentScorecard{r.agentScorecard}, Scorecard: r.agentScorecard}, nil
}

func (r *fakePublicShareRepository) GetPublicRunAgentReplaySnapshot(_ context.Context, runAgentID uuid.UUID) (repository.PublicRunAgentReplaySnapshot, error) {
	if r.runAgent.ID != runAgentID {
		return repository.PublicRunAgentReplaySnapshot{}, repository.ErrRunAgentNotFound
	}
	return repository.PublicRunAgentReplaySnapshot{Run: r.run, RunAgent: r.runAgent, Replay: r.replay}, nil
}
