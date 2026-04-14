package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Artifact struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	RunID           *uuid.UUID
	RunAgentID      *uuid.UUID
	ArtifactType    string
	StorageBucket   string
	StorageKey      string
	ContentType     *string
	SizeBytes       *int64
	ChecksumSHA256  *string
	Visibility      string
	RetentionStatus string
	Metadata        json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateArtifactParams struct {
	OrganizationID  uuid.UUID
	WorkspaceID     uuid.UUID
	RunID           *uuid.UUID
	RunAgentID      *uuid.UUID
	ArtifactType    string
	StorageBucket   string
	StorageKey      string
	ContentType     *string
	SizeBytes       *int64
	ChecksumSHA256  *string
	Visibility      string
	RetentionStatus string
	Metadata        json.RawMessage
}

func (r *Repository) CreateArtifact(ctx context.Context, params CreateArtifactParams) (Artifact, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO artifacts (
			organization_id,
			workspace_id,
			run_id,
			run_agent_id,
			artifact_type,
			storage_bucket,
			storage_key,
			content_type,
			size_bytes,
			checksum_sha256,
			visibility,
			retention_status,
			metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING
			id,
			organization_id,
			workspace_id,
			run_id,
			run_agent_id,
			artifact_type,
			storage_bucket,
			storage_key,
			content_type,
			size_bytes,
			checksum_sha256,
			visibility,
			retention_status,
			metadata,
			created_at,
			updated_at
	`,
		params.OrganizationID,
		params.WorkspaceID,
		params.RunID,
		params.RunAgentID,
		params.ArtifactType,
		params.StorageBucket,
		params.StorageKey,
		params.ContentType,
		params.SizeBytes,
		params.ChecksumSHA256,
		params.Visibility,
		params.RetentionStatus,
		defaultArtifactMetadata(params.Metadata),
	)

	artifact, err := scanArtifact(row)
	if err != nil {
		return Artifact{}, fmt.Errorf("create artifact: %w", err)
	}

	return artifact, nil
}

func (r *Repository) ListArtifactsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]Artifact, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			organization_id,
			workspace_id,
			run_id,
			run_agent_id,
			artifact_type,
			storage_bucket,
			storage_key,
			content_type,
			size_bytes,
			checksum_sha256,
			visibility,
			retention_status,
			metadata,
			created_at,
			updated_at
		FROM artifacts
		WHERE workspace_id = $1
		  AND retention_status = 'active'
		ORDER BY created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list artifacts by workspace: %w", err)
	}
	defer rows.Close()

	var artifacts []Artifact
	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, fmt.Errorf("list artifacts by workspace: scan: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list artifacts by workspace: rows: %w", err)
	}
	return artifacts, nil
}

func (r *Repository) GetArtifactByID(ctx context.Context, artifactID uuid.UUID) (Artifact, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			id,
			organization_id,
			workspace_id,
			run_id,
			run_agent_id,
			artifact_type,
			storage_bucket,
			storage_key,
			content_type,
			size_bytes,
			checksum_sha256,
			visibility,
			retention_status,
			metadata,
			created_at,
			updated_at
		FROM artifacts
		WHERE id = $1
	`, artifactID)

	artifact, err := scanArtifact(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Artifact{}, ErrArtifactNotFound
		}
		return Artifact{}, fmt.Errorf("get artifact by id: %w", err)
	}

	return artifact, nil
}

type artifactScanner interface {
	Scan(dest ...any) error
}

func scanArtifact(row artifactScanner) (Artifact, error) {
	var (
		artifact  Artifact
		metadata  []byte
		createdAt time.Time
		updatedAt time.Time
	)

	if err := row.Scan(
		&artifact.ID,
		&artifact.OrganizationID,
		&artifact.WorkspaceID,
		&artifact.RunID,
		&artifact.RunAgentID,
		&artifact.ArtifactType,
		&artifact.StorageBucket,
		&artifact.StorageKey,
		&artifact.ContentType,
		&artifact.SizeBytes,
		&artifact.ChecksumSHA256,
		&artifact.Visibility,
		&artifact.RetentionStatus,
		&metadata,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Artifact{}, err
	}

	artifact.Metadata = defaultArtifactMetadata(metadata)
	artifact.CreatedAt = createdAt.UTC()
	artifact.UpdatedAt = updatedAt.UTC()
	return artifact, nil
}

func defaultArtifactMetadata(value []byte) []byte {
	if len(value) == 0 {
		return []byte(`{}`)
	}
	return value
}
