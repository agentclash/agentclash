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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PublicShareRepository interface {
	CreatePublicShareLink(ctx context.Context, params repository.CreatePublicShareLinkParams) (repository.PublicShareLink, error)
	RevokePublicShareLink(ctx context.Context, id uuid.UUID) error
	SetPublicShareSearchIndexing(ctx context.Context, id uuid.UUID, enabled bool) error
	GetPublicShareLinkByID(ctx context.Context, id uuid.UUID) (repository.PublicShareLink, error)
	GetIndexablePublicShareLinkByID(ctx context.Context, id uuid.UUID) (repository.PublicShareLink, error)
	ListIndexablePublicShareLinks(ctx context.Context, after *uuid.UUID, limit int) ([]repository.PublicShareLink, error)
	GetActivePublicShareLinkByKey(ctx context.Context, key string) (repository.PublicShareLink, error)
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	GetRunnableChallengePackVersionByID(ctx context.Context, id uuid.UUID) (repository.RunnableChallengePackVersion, error)
	GetRunByID(ctx context.Context, id uuid.UUID) (domain.Run, error)
	GetRunScorecardByRunID(ctx context.Context, runID uuid.UUID) (repository.RunScorecard, error)
	GetRunAgentByID(ctx context.Context, id uuid.UUID) (domain.RunAgent, error)
	ListRunAgentsByRunID(ctx context.Context, runID uuid.UUID) ([]domain.RunAgent, error)
	GetRunAgentScorecardByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentScorecard, error)
	GetRunAgentReplayByRunAgentID(ctx context.Context, runAgentID uuid.UUID) (repository.RunAgentReplay, error)
	GetAgentTryoutByID(ctx context.Context, id uuid.UUID) (repository.AgentTryout, error)
	ListArtifactsByRunID(ctx context.Context, runID uuid.UUID) ([]repository.Artifact, error)
	GetPublicChallengePackVersionSnapshot(ctx context.Context, versionID uuid.UUID) (repository.PublicChallengePackVersionSnapshot, error)
	GetPublicRunScorecardSnapshot(ctx context.Context, runID uuid.UUID) (repository.PublicRunScorecardSnapshot, error)
	GetPublicRunAgentScorecardSnapshot(ctx context.Context, runAgentID uuid.UUID) (repository.PublicRunAgentScorecardSnapshot, error)
	GetPublicRunAgentReplaySnapshot(ctx context.Context, runAgentID uuid.UUID) (repository.PublicRunAgentReplaySnapshot, error)
}

// Optional capability preserves existing resource repository implementations.
type reportSnapshotRepository interface {
	GetPublicReportSnapshot(context.Context, uuid.UUID) (repository.PublicReportSnapshot, error)
	AuthorizeReportSnapshotWriter(context.Context, uuid.UUID, uuid.UUID) error
}

func (m *PublicShareManager) reportWriter(ctx context.Context, caller Caller, workspace uuid.UUID) error {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspace, ActionPublishPublicArtifact); err != nil {
		return err
	}
	repo, ok := m.repo.(reportSnapshotRepository)
	if !ok {
		return repository.ErrPublicShareLinkNotFound
	}
	if err := repo.AuthorizeReportSnapshotWriter(ctx, caller.UserID, workspace); err != nil {
		return ErrForbidden
	}
	return nil
}

type PublicShareService interface {
	CreateShareLink(ctx context.Context, caller Caller, input CreateShareLinkInput) (CreateShareLinkResult, error)
	RevokeShareLink(ctx context.Context, caller Caller, shareID uuid.UUID) error
	SetShareSearchIndexing(ctx context.Context, caller Caller, shareID uuid.UUID, enabled bool) error
	GetPublicShare(ctx context.Context, token string, baseURL string) (PublicSharePayload, error)
	GetPublication(ctx context.Context, id uuid.UUID) (PublicPublicationPayload, error)
	ListPublications(ctx context.Context, cursor *uuid.UUID, limit int) (PublicPublicationList, error)
}

type CreateShareLinkInput struct {
	ResourceType   repository.PublicShareResourceType
	ResourceID     uuid.UUID
	SearchIndexing bool
	ExpiresAt      *time.Time
}

type CreateShareLinkResult struct {
	Share          repository.PublicShareLink
	Token          string
	URL            string
	PublicationURL string
}

type PublicSharePayload struct {
	Share    publicShareLinkResponse `json:"share"`
	Resource any                     `json:"resource"`
}

type PublicPublicationPayload struct {
	Publication publicPublicationResponse `json:"publication"`
	Resource    any                       `json:"resource"`
}

type PublicPublicationList struct {
	Items      []PublicPublicationPayload `json:"items"`
	NextCursor *string                    `json:"next_cursor,omitempty"`
}

type publicPublicationResponse struct {
	ID            uuid.UUID  `json:"id"`
	ResourceType  string     `json:"resource_type"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CanonicalPath string     `json:"canonical_path"`
}

type PublicShareManager struct {
	authorizer     WorkspaceAuthorizer
	repo           PublicShareRepository
	frontendURL    string
	artifactSigner ArtifactContentSigner
	now            func() time.Time
}

func NewPublicShareManager(authorizer WorkspaceAuthorizer, repo PublicShareRepository, frontendURL string) *PublicShareManager {
	return &PublicShareManager{
		authorizer:  authorizer,
		repo:        repo,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		now:         time.Now,
	}
}

// WithArtifactSigner enables signed download URLs for template-allowlisted
// artifacts on shared tryout pages. When no signer is configured, shared
// payloads still list approved artifact descriptors but omit download URLs.
func (m *PublicShareManager) WithArtifactSigner(signer ArtifactContentSigner) *PublicShareManager {
	m.artifactSigner = signer
	return m
}

func (m *PublicShareManager) CreateShareLink(ctx context.Context, caller Caller, input CreateShareLinkInput) (CreateShareLinkResult, error) {
	if input.ResourceType == repository.PublicShareResourceReportSnapshot {
		if input.SearchIndexing {
			return CreateShareLinkResult{}, ErrForbidden
		}
		if input.ExpiresAt == nil || !input.ExpiresAt.After(m.clock()) {
			return CreateShareLinkResult{}, errReportShareExpiry
		}
	}
	organizationID, workspaceID, err := m.authorizedResourceScope(ctx, caller, input)
	if err != nil {
		return CreateShareLinkResult{}, err
	}
	if input.SearchIndexing {
		if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, workspaceID, ActionPublishPublicArtifact); err != nil {
			return CreateShareLinkResult{}, err
		}
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

	result := CreateShareLinkResult{
		Share: share,
		Token: share.Key,
		URL:   m.shareURL(share.Key),
	}
	if share.SearchIndexing {
		result.PublicationURL = m.publicationURL(share.ID)
	}
	return result, nil
}

func (m *PublicShareManager) RevokeShareLink(ctx context.Context, caller Caller, shareID uuid.UUID) error {
	share, err := m.repo.GetPublicShareLinkByID(ctx, shareID)
	if err != nil {
		return err
	}
	if err := m.authorizer.AuthorizeWorkspace(ctx, caller, share.WorkspaceID); err != nil {
		return err
	}
	if share.ResourceType == repository.PublicShareResourceReportSnapshot {
		if err := m.reportWriter(ctx, caller, share.WorkspaceID); err != nil {
			return err
		}
	}
	return m.repo.RevokePublicShareLink(ctx, shareID)
}

func (m *PublicShareManager) SetShareSearchIndexing(ctx context.Context, caller Caller, shareID uuid.UUID, enabled bool) error {
	share, err := m.repo.GetPublicShareLinkByID(ctx, shareID)
	if err != nil {
		return err
	}
	if share.ResourceType == repository.PublicShareResourceReportSnapshot {
		if err := m.reportWriter(ctx, caller, share.WorkspaceID); err != nil {
			return err
		}
		if enabled {
			return ErrForbidden
		}
	}

	if enabled {
		if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, share.WorkspaceID, ActionPublishPublicArtifact); err != nil {
			return err
		}
	} else if err := m.authorizer.AuthorizeWorkspace(ctx, caller, share.WorkspaceID); err != nil {
		return err
	}
	return m.repo.SetPublicShareSearchIndexing(ctx, shareID, enabled)
}

func (m *PublicShareManager) GetPublicShare(ctx context.Context, token string, baseURL string) (PublicSharePayload, error) {
	key := strings.TrimSpace(token)
	if key == "" {
		return PublicSharePayload{}, repository.ErrPublicShareLinkNotFound
	}
	share, err := m.repo.GetActivePublicShareLinkByKey(ctx, key)
	if err != nil {
		return PublicSharePayload{}, err
	}

	resource, err := m.publicResource(ctx, share, baseURL, true)
	if err != nil {
		return PublicSharePayload{}, err
	}
	if share.ResourceType == repository.PublicShareResourceReportSnapshot {
		share.SearchIndexing = false
	}
	return PublicSharePayload{
		Share:    mapPublicShareLink(share, ""),
		Resource: resource,
	}, nil
}

func (m *PublicShareManager) GetPublication(ctx context.Context, id uuid.UUID) (PublicPublicationPayload, error) {
	if id == uuid.Nil {
		return PublicPublicationPayload{}, repository.ErrPublicShareLinkNotFound
	}
	share, err := m.repo.GetIndexablePublicShareLinkByID(ctx, id)
	if err != nil {
		return PublicPublicationPayload{}, err
	}
	return m.publicationPayload(ctx, share)
}

func (m *PublicShareManager) ListPublications(ctx context.Context, cursor *uuid.UUID, limit int) (PublicPublicationList, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	shares, err := m.repo.ListIndexablePublicShareLinks(ctx, cursor, limit)
	if err != nil {
		return PublicPublicationList{}, err
	}

	items := make([]PublicPublicationPayload, 0, len(shares))
	for _, share := range shares {
		payload, payloadErr := m.publicationPayload(ctx, share)
		if payloadErr != nil {
			if publicationUnavailable(payloadErr) {
				continue
			}
			return PublicPublicationList{}, payloadErr
		}
		items = append(items, payload)
	}

	var nextCursor *string
	if len(shares) == limit && len(shares) > 0 {
		value := shares[len(shares)-1].ID.String()
		nextCursor = &value
	}
	return PublicPublicationList{Items: items, NextCursor: nextCursor}, nil
}

func publicationUnavailable(err error) bool {
	return errors.Is(err, repository.ErrPublicShareLinkNotFound) ||
		errors.Is(err, repository.ErrChallengePackVersionNotFound) ||
		errors.Is(err, repository.ErrRunNotFound) ||
		errors.Is(err, repository.ErrRunScorecardNotFound) ||
		errors.Is(err, repository.ErrRunAgentNotFound) ||
		errors.Is(err, repository.ErrRunAgentScorecardNotFound) ||
		errors.Is(err, repository.ErrRunAgentReplayNotFound) ||
		errors.Is(err, repository.ErrAgentTryoutNotFound)
}

func (m *PublicShareManager) publicationPayload(ctx context.Context, share repository.PublicShareLink) (PublicPublicationPayload, error) {
	if !m.publicationEligible(share) {
		return PublicPublicationPayload{}, repository.ErrPublicShareLinkNotFound
	}
	resource, resourceUpdatedAt, err := m.publicationResource(ctx, share)
	if err != nil {
		return PublicPublicationPayload{}, err
	}
	updatedAt := share.UpdatedAt
	if resourceUpdatedAt.After(updatedAt) {
		updatedAt = resourceUpdatedAt
	}
	return PublicPublicationPayload{
		Publication: publicPublicationResponse{
			ID:            share.ID,
			ResourceType:  string(share.ResourceType),
			ExpiresAt:     share.ExpiresAt,
			CreatedAt:     share.CreatedAt,
			UpdatedAt:     updatedAt,
			CanonicalPath: "/publications/" + share.ID.String(),
		},
		Resource: resource,
	}, nil
}

func (m *PublicShareManager) publicationEligible(share repository.PublicShareLink) bool {
	if share.ResourceType == repository.PublicShareResourceReportSnapshot || !share.IsActive || !share.SearchIndexing || share.RevokedAt != nil {
		return false
	}
	if share.ExpiresAt != nil && !share.ExpiresAt.After(m.clock()) {
		return false
	}
	return validPublicShareResourceType(share.ResourceType)
}

func (m *PublicShareManager) authorizedResourceScope(ctx context.Context, caller Caller, input CreateShareLinkInput) (uuid.UUID, uuid.UUID, error) {
	switch input.ResourceType {
	case repository.PublicShareResourceReportSnapshot:
		repo, ok := m.repo.(reportSnapshotRepository)
		if !ok {
			return uuid.Nil, uuid.Nil, repository.ErrPublicShareLinkNotFound
		}
		snapshot, err := repo.GetPublicReportSnapshot(ctx, input.ResourceID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if err = m.reportWriter(ctx, caller, snapshot.WorkspaceID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		return snapshot.OrganizationID, snapshot.WorkspaceID, nil

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
	case repository.PublicShareResourceAgentTryout:
		tryout, err := m.repo.GetAgentTryoutByID(ctx, input.ResourceID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if tryout.OrganizationID == nil || tryout.WorkspaceID == nil {
			return uuid.Nil, uuid.Nil, repository.ErrAgentTryoutNotFound
		}
		if err := m.authorizer.AuthorizeWorkspace(ctx, caller, *tryout.WorkspaceID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		if !tryout.RedactionStatus.ShareReady() {
			return uuid.Nil, uuid.Nil, ErrAgentTryoutRedactionNotReady
		}
		return *tryout.OrganizationID, *tryout.WorkspaceID, nil
	default:
		return uuid.Nil, uuid.Nil, errInvalidShareResourceType
	}
}

func (m *PublicShareManager) publicResource(ctx context.Context, share repository.PublicShareLink, baseURL string, includeSignedArtifactURLs bool) (any, error) {
	switch share.ResourceType {
	case repository.PublicShareResourceReportSnapshot:
		repo, ok := m.repo.(reportSnapshotRepository)
		if !ok {
			return nil, repository.ErrPublicShareLinkNotFound
		}
		snapshot, err := repo.GetPublicReportSnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, err
		}
		if snapshot.WorkspaceID != share.WorkspaceID || snapshot.OrganizationID != share.OrganizationID {
			return nil, repository.ErrPublicShareLinkNotFound
		}
		// Workspace/author records and live private notes are not public report content.
		var value struct {
			Content json.RawMessage `json:"content"`
			Hash    string          `json:"content_hash"`
		}
		if json.Unmarshal(snapshot.Body, &value) != nil || len(value.Content) == 0 {
			return nil, repository.ErrPublicShareLinkNotFound
		}
		return map[string]any{"type": "report_snapshot", "content": value.Content, "content_hash": value.Hash}, nil

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
	case repository.PublicShareResourceAgentTryout:
		tryout, err := m.repo.GetAgentTryoutByID(ctx, share.ResourceID)
		if err != nil {
			return nil, err
		}
		if tryout.WorkspaceID == nil {
			return nil, repository.ErrAgentTryoutNotFound
		}
		// Fail closed: a share whose redaction has regressed (or never passed)
		// must not render unredacted content, even though creation already
		// gated on this. Treat it as an unavailable share rather than leaking.
		if !tryout.RedactionStatus.ShareReady() {
			return nil, repository.ErrPublicShareLinkNotFound
		}
		return m.publicAgentTryout(ctx, tryout, baseURL, includeSignedArtifactURLs)
	default:
		return nil, repository.ErrPublicShareLinkNotFound
	}
}

// publicationResource is deliberately separate from capability-share
// serialization. Publication URLs are enumerable and indexable, so they expose
// only presentation fields and stable public aliases. Database UUIDs,
// evaluation-spec identifiers, capability material, and signed artifact URLs
// never cross this boundary.
func (m *PublicShareManager) publicationResource(ctx context.Context, share repository.PublicShareLink) (any, time.Time, error) {
	switch share.ResourceType {
	case repository.PublicShareResourceChallengePackVersion:
		snapshot, err := m.repo.GetPublicChallengePackVersionSnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, time.Time{}, err
		}
		if !publicationJSONObjectValid(snapshot.Manifest) {
			return nil, time.Time{}, repository.ErrPublicShareLinkNotFound
		}
		return mapPublicationChallengePackVersion(snapshot), snapshot.UpdatedAt, nil
	case repository.PublicShareResourceRunScorecard:
		snapshot, err := m.repo.GetPublicRunScorecardSnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, time.Time{}, err
		}
		if !publicationScorecardsValid(snapshot.Scorecard.Scorecard, snapshot.AgentScorecards) {
			return nil, time.Time{}, repository.ErrPublicShareLinkNotFound
		}
		return mapPublicationRunScorecard(snapshot), runScorecardSnapshotUpdatedAt(snapshot), nil
	case repository.PublicShareResourceRunAgentScorecard:
		snapshot, err := m.repo.GetPublicRunAgentScorecardSnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, time.Time{}, err
		}
		if !publicationJSONObjectValid(snapshot.Scorecard.Scorecard) || !publicationAgentScorecardsValid(snapshot.AgentScorecards) {
			return nil, time.Time{}, repository.ErrPublicShareLinkNotFound
		}
		return mapPublicationRunAgentScorecard(snapshot), runAgentScorecardSnapshotUpdatedAt(snapshot), nil
	case repository.PublicShareResourceRunAgentReplay:
		snapshot, err := m.repo.GetPublicRunAgentReplaySnapshot(ctx, share.ResourceID)
		if err != nil {
			return nil, time.Time{}, err
		}
		if !publicationJSONObjectValid(snapshot.Replay.Summary) {
			return nil, time.Time{}, repository.ErrPublicShareLinkNotFound
		}
		return mapPublicationRunAgentReplay(snapshot), latestTime(snapshot.Run.UpdatedAt, snapshot.RunAgent.UpdatedAt, snapshot.Replay.UpdatedAt), nil
	case repository.PublicShareResourceAgentTryout:
		tryout, err := m.repo.GetAgentTryoutByID(ctx, share.ResourceID)
		if err != nil {
			return nil, time.Time{}, err
		}
		if !tryout.RedactionStatus.ShareReady() {
			return nil, time.Time{}, repository.ErrPublicShareLinkNotFound
		}
		if !publicationJSONObjectValid(tryout.InputSnapshot) || !publicationJSONObjectValid(tryout.Summary) {
			return nil, time.Time{}, repository.ErrPublicShareLinkNotFound
		}
		publicTryout, err := m.publicAgentTryout(ctx, tryout, "", false)
		if err != nil {
			return nil, time.Time{}, err
		}
		return mapPublicationAgentTryout(publicTryout), tryout.UpdatedAt, nil
	default:
		return nil, time.Time{}, repository.ErrPublicShareLinkNotFound
	}
}

func publicationJSONObjectValid(raw json.RawMessage) bool {
	if len(raw) == 0 || !json.Valid(raw) {
		return false
	}
	var object map[string]any
	return json.Unmarshal(raw, &object) == nil && object != nil
}

func publicationAgentScorecardsValid(scorecards []repository.RunAgentScorecard) bool {
	for _, scorecard := range scorecards {
		if !publicationJSONObjectValid(scorecard.Scorecard) {
			return false
		}
	}
	return true
}

func publicationScorecardsValid(runScorecard json.RawMessage, agentScorecards []repository.RunAgentScorecard) bool {
	return publicationJSONObjectValid(runScorecard) && publicationAgentScorecardsValid(agentScorecards)
}

func runScorecardSnapshotUpdatedAt(snapshot repository.PublicRunScorecardSnapshot) time.Time {
	values := []time.Time{snapshot.Run.UpdatedAt, snapshot.Scorecard.UpdatedAt}
	for _, agent := range snapshot.Agents {
		values = append(values, agent.UpdatedAt)
	}
	for _, scorecard := range snapshot.AgentScorecards {
		values = append(values, scorecard.UpdatedAt)
	}
	return latestTime(values...)
}

func runAgentScorecardSnapshotUpdatedAt(snapshot repository.PublicRunAgentScorecardSnapshot) time.Time {
	values := []time.Time{snapshot.Run.UpdatedAt, snapshot.RunAgent.UpdatedAt, snapshot.Scorecard.UpdatedAt}
	for _, agent := range snapshot.SiblingAgents {
		values = append(values, agent.UpdatedAt)
	}
	for _, scorecard := range snapshot.AgentScorecards {
		values = append(values, scorecard.UpdatedAt)
	}
	return latestTime(values...)
}

func latestTime(values ...time.Time) time.Time {
	var latest time.Time
	for _, value := range values {
		if value.After(latest) {
			latest = value
		}
	}
	return latest
}

// publicAgentTryout builds the redaction-safe, shareable view of a tryout:
// the public payload (no org/workspace/user identifiers) with a defensively
// re-redacted summary, plus the template-allowlisted, redacted artifact
// descriptors (with signed download URLs when a signer is configured).
func (m *PublicShareManager) publicAgentTryout(ctx context.Context, tryout repository.AgentTryout, baseURL string, includeSignedArtifactURLs bool) (sharedAgentTryoutResponse, error) {
	payload := mapPublicAgentTryoutResponse(tryout)
	payload.Summary = redactTryoutSummaryForPublic(payload.Summary)
	resp := sharedAgentTryoutResponse{publicAgentTryoutResponse: payload}

	allow := parseTemplateArtifactAllowlist(tryout.TemplateSnapshot)
	if tryout.RunID == nil || len(allow) == 0 {
		return resp, nil
	}
	artifacts, err := m.repo.ListArtifactsByRunID(ctx, *tryout.RunID)
	if err != nil {
		return sharedAgentTryoutResponse{}, err
	}
	var signer ArtifactContentSigner
	if includeSignedArtifactURLs {
		signer = m.artifactSigner
	}
	resp.Artifacts = buildSharedTryoutArtifacts(artifacts, allow, signer, baseURL, m.clock())
	return resp, nil
}

func (m *PublicShareManager) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m *PublicShareManager) shareURL(token string) string {
	base := m.frontendURL
	if base == "" {
		base = "https://www.agentclash.dev"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "https://www.agentclash.dev/share/" + token
	}
	if parsed.Hostname() == "agentclash.dev" {
		parsed.Host = "www.agentclash.dev"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/share/" + token
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func (m *PublicShareManager) publicationURL(id uuid.UUID) string {
	base := m.frontendURL
	if base == "" {
		base = "https://www.agentclash.dev"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "https://www.agentclash.dev/publications/" + id.String()
	}
	if parsed.Hostname() == "agentclash.dev" {
		parsed.Host = "www.agentclash.dev"
	}
	parsed.Path = "/publications/" + id.String()
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
	Share          publicShareLinkResponse `json:"share"`
	Token          string                  `json:"token"`
	URL            string                  `json:"url"`
	PublicationURL string                  `json:"publication_url,omitempty"`
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
			Share:          mapPublicShareLink(result.Share, result.URL),
			Token:          result.Token,
			URL:            result.URL,
			PublicationURL: result.PublicationURL,
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

type updateShareLinkRequest struct {
	SearchIndexing *bool `json:"search_indexing"`
}

func updateShareLinkHandler(logger *slog.Logger, service PublicShareService) http.HandlerFunc {
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
		var request updateShareLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.SearchIndexing == nil {
			writeError(w, http.StatusBadRequest, "invalid_share_request", "search_indexing is required")
			return
		}
		if err := service.SetShareSearchIndexing(r.Context(), caller, shareID, *request.SearchIndexing); err != nil {
			writeShareError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

func getPublicShareHandler(logger *slog.Logger, service PublicShareService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		token := chi.URLParam(r, "token")
		result, err := service.GetPublicShare(r.Context(), token, requestBaseURL(r))
		if err != nil {
			writeShareError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func getPublicationHandler(logger *slog.Logger, service PublicShareService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		id, err := uuid.Parse(chi.URLParam(r, "publicationID"))
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "publication not found")
			return
		}
		result, err := service.GetPublication(r.Context(), id)
		if err != nil {
			writeShareError(w, logger, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func listPublicationsHandler(logger *slog.Logger, service PublicShareService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var cursor *uuid.UUID
		if value := strings.TrimSpace(r.URL.Query().Get("cursor")); value != "" {
			parsed, err := uuid.Parse(value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor must be a publication ID")
				return
			}
			cursor = &parsed
		}
		limit := 20
		if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 100 {
				writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
				return
			}
			limit = parsed
		}
		result, err := service.ListPublications(r.Context(), cursor, limit)
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
	case errors.Is(err, errReportShareExpiry):
		writeError(w, http.StatusBadRequest, "invalid_share_request", "report links require an expiry in the future")
	case errors.Is(err, errInvalidShareResourceType):
		writeError(w, http.StatusBadRequest, "invalid_share_request", "unsupported resource_type")
	case errors.Is(err, repository.ErrAgentTryoutNotFound):
		writeError(w, http.StatusNotFound, "not_found", "shared resource not found")
	case errors.Is(err, ErrAgentTryoutRedactionNotReady):
		writeError(w, http.StatusConflict, "agent_tryout_redaction_not_ready", "This tryout's evidence is still being redacted for safe sharing. Try again once it has finished.")
	default:
		logger.Error("public share request failed", "method", r.Method, "path", safeRequestLogPath(r.URL.Path), "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

var errReportShareExpiry = errors.New("report link requires future expiry")

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
	case repository.PublicShareResourceReportSnapshot,
		repository.PublicShareResourceChallengePackVersion,
		repository.PublicShareResourceRunScorecard,
		repository.PublicShareResourceRunAgentScorecard,
		repository.PublicShareResourceRunAgentReplay,
		repository.PublicShareResourceAgentTryout:
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

func mapPublicationChallengePackVersion(snapshot repository.PublicChallengePackVersionSnapshot) any {
	inputSets := make([]map[string]any, 0, len(snapshot.InputSets))
	for _, inputSet := range snapshot.InputSets {
		inputSets = append(inputSets, map[string]any{
			"input_key": inputSet.InputKey,
			"name":      inputSet.Name,
		})
	}
	return map[string]any{
		"type": "challenge_pack_version",
		"pack": map[string]any{
			"slug":        snapshot.PackSlug,
			"name":        snapshot.PackName,
			"family":      snapshot.PackFamily,
			"description": snapshot.PackDescription,
		},
		"version": map[string]any{
			"version_number":   snapshot.VersionNumber,
			"lifecycle_status": snapshot.LifecycleStatus,
			"manifest":         sanitizePublicationJSON(snapshot.Manifest),
			"input_sets":       inputSets,
			"created_at":       snapshot.CreatedAt,
			"updated_at":       snapshot.UpdatedAt,
		},
	}
}

func mapPublicationRunScorecard(snapshot repository.PublicRunScorecardSnapshot) any {
	aliases := publicationAgentAliases(snapshot.Agents)
	agents := make([]map[string]any, 0, len(snapshot.Agents))
	for _, agent := range snapshot.Agents {
		agents = append(agents, mapPublicationRunAgent(agent, aliases[agent.ID]))
	}
	scorecards := make([]map[string]any, 0, len(snapshot.AgentScorecards))
	for _, scorecard := range snapshot.AgentScorecards {
		alias, ok := aliases[scorecard.RunAgentID]
		if !ok {
			continue
		}
		scorecards = append(scorecards, mapPublicationRunAgentScorecardValue(scorecard, alias))
	}
	return map[string]any{
		"type":             "run_scorecard",
		"run":              mapPublicationRun(snapshot.Run),
		"agents":           agents,
		"agent_scorecards": scorecards,
		"scorecard":        mapPublicationRunScorecardValue(snapshot.Scorecard, aliases),
	}
}

func mapPublicationRunAgentScorecard(snapshot repository.PublicRunAgentScorecardSnapshot) any {
	allAgents := append([]domain.RunAgent{snapshot.RunAgent}, snapshot.SiblingAgents...)
	aliases := publicationAgentAliases(allAgents)
	currentAlias := aliases[snapshot.RunAgent.ID]
	siblings := make([]map[string]any, 0, len(snapshot.SiblingAgents))
	for _, agent := range snapshot.SiblingAgents {
		if agent.ID == snapshot.RunAgent.ID {
			continue
		}
		siblings = append(siblings, mapPublicationRunAgent(agent, aliases[agent.ID]))
	}
	scorecards := make([]map[string]any, 0, len(snapshot.AgentScorecards))
	for _, scorecard := range snapshot.AgentScorecards {
		alias, ok := aliases[scorecard.RunAgentID]
		if !ok {
			continue
		}
		scorecards = append(scorecards, mapPublicationRunAgentScorecardValue(scorecard, alias))
	}
	return map[string]any{
		"type":             "run_agent_scorecard",
		"run":              mapPublicationRun(snapshot.Run),
		"run_agent":        mapPublicationRunAgent(snapshot.RunAgent, currentAlias),
		"sibling_agents":   siblings,
		"agent_scorecards": scorecards,
		"scorecard":        mapPublicationRunAgentScorecardValue(snapshot.Scorecard, currentAlias),
	}
}

func mapPublicationRunAgentReplay(snapshot repository.PublicRunAgentReplaySnapshot) any {
	return map[string]any{
		"type":      "run_agent_replay",
		"run":       mapPublicationRun(snapshot.Run),
		"run_agent": mapPublicationRunAgent(snapshot.RunAgent, "agent-1"),
		"replay": map[string]any{
			"summary":                sanitizePublicationJSON(snapshot.Replay.Summary),
			"latest_sequence_number": snapshot.Replay.LatestSequenceNumber,
			"event_count":            snapshot.Replay.EventCount,
			"created_at":             snapshot.Replay.CreatedAt,
			"updated_at":             snapshot.Replay.UpdatedAt,
		},
	}
}

func mapPublicationAgentTryout(tryout sharedAgentTryoutResponse) any {
	artifacts := make([]map[string]any, 0, len(tryout.Artifacts))
	for _, artifact := range tryout.Artifacts {
		artifacts = append(artifacts, map[string]any{
			"key":             artifact.Key,
			"type":            artifact.Type,
			"path":            artifact.Path,
			"content_type":    artifact.ContentType,
			"size_bytes":      artifact.SizeBytes,
			"checksum_sha256": artifact.ChecksumSHA256,
		})
	}
	return map[string]any{
		"type":                 "agent_tryout",
		"template_slug":        tryout.TemplateSlug,
		"status":               tryout.Status,
		"input_snapshot":       sanitizePublicationJSON(tryout.InputSnapshot),
		"summary":              sanitizePublicationJSON(tryout.Summary),
		"redaction_status":     tryout.RedactionStatus,
		"cost_limit_usd":       tryout.CostLimitUSD,
		"actual_cost_usd":      tryout.ActualCostUSD,
		"latency_ms":           tryout.LatencyMS,
		"max_duration_seconds": tryout.MaxDurationSeconds,
		"created_at":           tryout.CreatedAt,
		"updated_at":           tryout.UpdatedAt,
		"artifacts":            artifacts,
	}
}

func publicationAgentAliases(agents []domain.RunAgent) map[uuid.UUID]string {
	aliases := make(map[uuid.UUID]string, len(agents))
	for _, agent := range agents {
		if _, exists := aliases[agent.ID]; exists {
			continue
		}
		aliases[agent.ID] = fmt.Sprintf("agent-%d", len(aliases)+1)
	}
	return aliases
}

func mapPublicationRun(run domain.Run) map[string]any {
	return map[string]any{
		"id":             "run",
		"name":           run.Name,
		"status":         run.Status,
		"execution_mode": run.ExecutionMode,
		"started_at":     run.StartedAt,
		"finished_at":    run.FinishedAt,
		"created_at":     run.CreatedAt,
	}
}

func mapPublicationRunAgent(agent domain.RunAgent, alias string) map[string]any {
	return map[string]any{
		"id":          alias,
		"run_id":      "run",
		"lane_index":  agent.LaneIndex,
		"label":       agent.Label,
		"status":      agent.Status,
		"started_at":  agent.StartedAt,
		"finished_at": agent.FinishedAt,
	}
}

func mapPublicationRunAgentScorecardValue(scorecard repository.RunAgentScorecard, agentAlias string) map[string]any {
	return map[string]any{
		"id":                "scorecard-" + agentAlias,
		"run_agent_id":      agentAlias,
		"overall_score":     scorecard.OverallScore,
		"correctness_score": scorecard.CorrectnessScore,
		"reliability_score": scorecard.ReliabilityScore,
		"latency_score":     scorecard.LatencyScore,
		"cost_score":        scorecard.CostScore,
		"behavioral_score":  scorecard.BehavioralScore,
		"passed":            scorecard.Passed,
		"scorecard":         sanitizePublicationJSON(scorecard.Scorecard),
		"created_at":        scorecard.CreatedAt,
		"updated_at":        scorecard.UpdatedAt,
	}
}

func mapPublicationRunScorecardValue(scorecard repository.RunScorecard, aliases map[uuid.UUID]string) map[string]any {
	winner := ""
	if scorecard.WinningRunAgentID != nil {
		winner = aliases[*scorecard.WinningRunAgentID]
	}
	return map[string]any{
		"winner":     winner,
		"scorecard":  sanitizePublicationJSON(scorecard.Scorecard),
		"created_at": scorecard.CreatedAt,
		"updated_at": scorecard.UpdatedAt,
	}
}

func sanitizePublicationJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	return sanitizePublicationValue(decoded)
}

func sanitizePublicationValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, child := range typed {
			if publicationSensitiveKey(key) {
				continue
			}
			clean[key] = sanitizePublicationValue(child)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, child := range typed {
			clean[index] = sanitizePublicationValue(child)
		}
		return clean
	case string:
		return redactPublicationSecretValues(typed)
	default:
		return typed
	}
}

var publicationSecretValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
}

// Match every PEM-style private-key marker, including EC, encrypted, and PGP
// blocks. The payload is intentionally replaced in full rather than trying to
// preserve surrounding text that may still contain key material.
var publicationPrivateKeyMarker = regexp.MustCompile(`(?is)-----BEGIN[^\r\n-]{0,100}PRIVATE KEY[^\r\n-]{0,100}-----`)

func redactPublicationSecretValues(value string) string {
	if publicationPrivateKeyMarker.MatchString(value) {
		return "[REDACTED]"
	}
	redacted := value
	for _, pattern := range publicationSecretValuePatterns {
		redacted = pattern.ReplaceAllString(redacted, "[REDACTED]")
	}
	return redacted
}

func publicationSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "id" || normalized == "token" || normalized == "authorization" ||
		normalized == "env" || normalized == "env_vars" || normalized == "environment" ||
		normalized == "environment_variables" ||
		strings.HasPrefix(normalized, "env_") || strings.HasSuffix(normalized, "_env") ||
		strings.Contains(normalized, "environment") || strings.HasSuffix(normalized, "_token") ||
		strings.HasSuffix(normalized, "_id") || strings.HasSuffix(normalized, "_ids") {
		return true
	}
	for _, fragment := range []string{
		"email", "phone", "postal_address", "street_address", "display_name",
		"profile_image", "avatar", "username",
	} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return sensitiveSummaryKey(normalized)
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
