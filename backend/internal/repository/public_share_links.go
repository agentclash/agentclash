package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PublicShareResourceType string

const (
	PublicShareResourceChallengePackVersion PublicShareResourceType = "challenge_pack_version"
	PublicShareResourceRunScorecard         PublicShareResourceType = "run_scorecard"
	PublicShareResourceRunAgentScorecard    PublicShareResourceType = "run_agent_scorecard"
	PublicShareResourceRunAgentReplay       PublicShareResourceType = "run_agent_replay"
)

type PublicShareLink struct {
	ID              uuid.UUID
	Key             string
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	ResourceType    PublicShareResourceType
	ResourceID      uuid.UUID
	CreatedByUserID *uuid.UUID
	IsActive        bool
	SearchIndexing  bool
	ViewCount       int64
	LastAccessedAt  *time.Time
	ExpiresAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	RevokedAt       *time.Time
}

type CreatePublicShareLinkParams struct {
	Key             string
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	ResourceType    PublicShareResourceType
	ResourceID      uuid.UUID
	CreatedByUserID *uuid.UUID
	SearchIndexing  bool
	ExpiresAt       *time.Time
}

type PublicChallengePackVersionSnapshot struct {
	PackID          uuid.UUID
	PackSlug        string
	PackName        string
	PackFamily      string
	PackDescription *string
	VersionID       uuid.UUID
	VersionNumber   int32
	LifecycleStatus string
	Manifest        json.RawMessage
	InputSets       []ChallengeInputSetSummary
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PublicRunScorecardSnapshot struct {
	Run             domain.Run
	Agents          []domain.RunAgent
	AgentScorecards []RunAgentScorecard
	Scorecard       RunScorecard
}

type PublicRunAgentScorecardSnapshot struct {
	Run             domain.Run
	RunAgent        domain.RunAgent
	SiblingAgents   []domain.RunAgent
	AgentScorecards []RunAgentScorecard
	Scorecard       RunAgentScorecard
}

type PublicRunAgentReplaySnapshot struct {
	Run      domain.Run
	RunAgent domain.RunAgent
	Replay   RunAgentReplay
}

func (r *Repository) CreatePublicShareLink(ctx context.Context, params CreatePublicShareLinkParams) (PublicShareLink, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO public_share_links (
			key,
			organization_id,
			workspace_id,
			resource_type,
			resource_id,
			created_by_user_id,
			search_indexing,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (resource_type, resource_id) WHERE is_active
		DO UPDATE SET updated_at = public_share_links.updated_at
		RETURNING id, key, organization_id, workspace_id, resource_type, resource_id, created_by_user_id, is_active, search_indexing, view_count, last_accessed_at, expires_at, created_at, updated_at, revoked_at
	`, params.Key, params.OrganizationID, params.WorkspaceID, string(params.ResourceType), params.ResourceID, params.CreatedByUserID, params.SearchIndexing, params.ExpiresAt)

	share, err := scanPublicShareLink(row)
	if err != nil {
		return PublicShareLink{}, fmt.Errorf("create public share link: %w", err)
	}
	return share, nil
}

func (r *Repository) RevokePublicShareLink(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE public_share_links
		SET is_active = false,
		    revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1
		  AND is_active
	`, id)
	if err != nil {
		return fmt.Errorf("revoke public share link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPublicShareLinkNotFound
	}
	return nil
}

func (r *Repository) GetPublicShareLinkByID(ctx context.Context, id uuid.UUID) (PublicShareLink, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, key, organization_id, workspace_id, resource_type, resource_id, created_by_user_id, is_active, search_indexing, view_count, last_accessed_at, expires_at, created_at, updated_at, revoked_at
		FROM public_share_links
		WHERE id = $1
		LIMIT 1
	`, id)
	share, err := scanPublicShareLink(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicShareLink{}, ErrPublicShareLinkNotFound
		}
		return PublicShareLink{}, fmt.Errorf("get public share link by id: %w", err)
	}
	return share, nil
}

func (r *Repository) GetActivePublicShareLinkByKey(ctx context.Context, key string) (PublicShareLink, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE public_share_links
		SET view_count = view_count + 1,
		    last_accessed_at = now()
		WHERE key = $1
		  AND is_active
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
		RETURNING id, key, organization_id, workspace_id, resource_type, resource_id, created_by_user_id, is_active, search_indexing, view_count, last_accessed_at, expires_at, created_at, updated_at, revoked_at
	`, key)
	share, err := scanPublicShareLink(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicShareLink{}, ErrPublicShareLinkNotFound
		}
		return PublicShareLink{}, fmt.Errorf("get active public share link by key: %w", err)
	}
	return share, nil
}

func (r *Repository) ListActivePublicShareLinksByResource(ctx context.Context, resourceType PublicShareResourceType, resourceID uuid.UUID) ([]PublicShareLink, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, key, organization_id, workspace_id, resource_type, resource_id, created_by_user_id, is_active, search_indexing, view_count, last_accessed_at, expires_at, created_at, updated_at, revoked_at
		FROM public_share_links
		WHERE resource_type = $1
		  AND resource_id = $2
		  AND is_active
		  AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, string(resourceType), resourceID)
	if err != nil {
		return nil, fmt.Errorf("list active public share links by resource: %w", err)
	}
	defer rows.Close()

	shares := []PublicShareLink{}
	for rows.Next() {
		share, scanErr := scanPublicShareLink(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan public share link: %w", scanErr)
		}
		shares = append(shares, share)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public share links: %w", err)
	}
	return shares, nil
}

func (r *Repository) GetPublicChallengePackVersionSnapshot(ctx context.Context, versionID uuid.UUID) (PublicChallengePackVersionSnapshot, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			cp.id,
			cp.slug,
			cp.name,
			cp.family,
			cp.description,
			cpv.id,
			cpv.version_number,
			cpv.lifecycle_status,
			cpv.manifest,
			cpv.created_at,
			cpv.updated_at
		FROM challenge_pack_versions cpv
		JOIN challenge_packs cp ON cp.id = cpv.challenge_pack_id
		WHERE cpv.id = $1
		  AND cpv.archived_at IS NULL
		  AND cp.archived_at IS NULL
		LIMIT 1
	`, versionID)

	var snapshot PublicChallengePackVersionSnapshot
	if err := row.Scan(
		&snapshot.PackID,
		&snapshot.PackSlug,
		&snapshot.PackName,
		&snapshot.PackFamily,
		&snapshot.PackDescription,
		&snapshot.VersionID,
		&snapshot.VersionNumber,
		&snapshot.LifecycleStatus,
		&snapshot.Manifest,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicChallengePackVersionSnapshot{}, ErrChallengePackVersionNotFound
		}
		return PublicChallengePackVersionSnapshot{}, fmt.Errorf("get public challenge pack version snapshot: %w", err)
	}

	inputSets, err := r.ListChallengeInputSetsByVersionID(ctx, versionID)
	if err != nil {
		return PublicChallengePackVersionSnapshot{}, err
	}
	snapshot.InputSets = inputSets
	return snapshot, nil
}

func (r *Repository) GetPublicRunScorecardSnapshot(ctx context.Context, runID uuid.UUID) (PublicRunScorecardSnapshot, error) {
	run, err := r.GetRunByID(ctx, runID)
	if err != nil {
		return PublicRunScorecardSnapshot{}, err
	}
	agents, err := r.ListRunAgentsByRunID(ctx, runID)
	if err != nil {
		return PublicRunScorecardSnapshot{}, err
	}
	agentScorecards, err := r.listAvailableRunAgentScorecards(ctx, agents)
	if err != nil {
		return PublicRunScorecardSnapshot{}, err
	}
	scorecard, err := r.GetRunScorecardByRunID(ctx, runID)
	if err != nil {
		return PublicRunScorecardSnapshot{}, err
	}
	return PublicRunScorecardSnapshot{Run: run, Agents: agents, AgentScorecards: agentScorecards, Scorecard: scorecard}, nil
}

func (r *Repository) GetPublicRunAgentScorecardSnapshot(ctx context.Context, runAgentID uuid.UUID) (PublicRunAgentScorecardSnapshot, error) {
	runAgent, err := r.GetRunAgentByID(ctx, runAgentID)
	if err != nil {
		return PublicRunAgentScorecardSnapshot{}, err
	}
	run, err := r.GetRunByID(ctx, runAgent.RunID)
	if err != nil {
		return PublicRunAgentScorecardSnapshot{}, err
	}
	agents, err := r.ListRunAgentsByRunID(ctx, run.ID)
	if err != nil {
		return PublicRunAgentScorecardSnapshot{}, err
	}
	agentScorecards, err := r.listAvailableRunAgentScorecards(ctx, agents)
	if err != nil {
		return PublicRunAgentScorecardSnapshot{}, err
	}
	scorecard, err := r.GetRunAgentScorecardByRunAgentID(ctx, runAgentID)
	if err != nil {
		return PublicRunAgentScorecardSnapshot{}, err
	}
	return PublicRunAgentScorecardSnapshot{Run: run, RunAgent: runAgent, SiblingAgents: agents, AgentScorecards: agentScorecards, Scorecard: scorecard}, nil
}

func (r *Repository) GetPublicRunAgentReplaySnapshot(ctx context.Context, runAgentID uuid.UUID) (PublicRunAgentReplaySnapshot, error) {
	runAgent, err := r.GetRunAgentByID(ctx, runAgentID)
	if err != nil {
		return PublicRunAgentReplaySnapshot{}, err
	}
	run, err := r.GetRunByID(ctx, runAgent.RunID)
	if err != nil {
		return PublicRunAgentReplaySnapshot{}, err
	}
	replay, err := r.GetRunAgentReplayByRunAgentID(ctx, runAgentID)
	if err != nil {
		return PublicRunAgentReplaySnapshot{}, err
	}
	return PublicRunAgentReplaySnapshot{Run: run, RunAgent: runAgent, Replay: replay}, nil
}

type publicShareScanner interface {
	Scan(dest ...any) error
}

func scanPublicShareLink(row publicShareScanner) (PublicShareLink, error) {
	var share PublicShareLink
	var resourceType string
	if err := row.Scan(
		&share.ID,
		&share.Key,
		&share.OrganizationID,
		&share.WorkspaceID,
		&resourceType,
		&share.ResourceID,
		&share.CreatedByUserID,
		&share.IsActive,
		&share.SearchIndexing,
		&share.ViewCount,
		&share.LastAccessedAt,
		&share.ExpiresAt,
		&share.CreatedAt,
		&share.UpdatedAt,
		&share.RevokedAt,
	); err != nil {
		return PublicShareLink{}, err
	}
	share.ResourceType = PublicShareResourceType(resourceType)
	return share, nil
}

func (r *Repository) listAvailableRunAgentScorecards(ctx context.Context, agents []domain.RunAgent) ([]RunAgentScorecard, error) {
	scorecards := make([]RunAgentScorecard, 0, len(agents))
	for _, agent := range agents {
		scorecard, err := r.GetRunAgentScorecardByRunAgentID(ctx, agent.ID)
		if err != nil {
			if errors.Is(err, ErrRunAgentScorecardNotFound) {
				continue
			}
			return nil, err
		}
		scorecards = append(scorecards, scorecard)
	}
	return scorecards, nil
}
