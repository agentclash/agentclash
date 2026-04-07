package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PublishChallengePackBundleParams struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Bundle         challengepack.Bundle
	BundleArtifact *CreateArtifactParams
}

type PublishedChallengePack struct {
	ChallengePackID        uuid.UUID
	ChallengePackVersionID uuid.UUID
	EvaluationSpecID       uuid.UUID
	InputSetIDs            []uuid.UUID
	BundleArtifactID       *uuid.UUID
}

func (r *Repository) ListVisibleChallengePacks(ctx context.Context, workspaceID uuid.UUID) ([]ChallengePackSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM challenge_packs
		WHERE archived_at IS NULL
		  AND (workspace_id IS NULL OR workspace_id = $1)
		ORDER BY name ASC
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list visible challenge packs: %w", err)
	}
	defer rows.Close()

	var packs []ChallengePackSummary
	for rows.Next() {
		var pack ChallengePackSummary
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&pack.ID, &pack.Name, &pack.Description, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan visible challenge pack: %w", err)
		}
		pack.CreatedAt = createdAt
		pack.UpdatedAt = updatedAt
		packs = append(packs, pack)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visible challenge packs: %w", err)
	}

	return packs, nil
}

func (r *Repository) PublishChallengePackBundle(ctx context.Context, params PublishChallengePackBundleParams) (PublishedChallengePack, error) {
	manifest, err := challengepack.ManifestJSON(params.Bundle)
	if err != nil {
		return PublishedChallengePack{}, fmt.Errorf("build challenge pack manifest: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return PublishedChallengePack{}, fmt.Errorf("begin publish challenge pack transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	packID, err := resolveOrCreateChallengePack(ctx, tx, params.WorkspaceID, params.Bundle)
	if err != nil {
		return PublishedChallengePack{}, err
	}

	versionID := uuid.New()
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		INSERT INTO challenge_pack_versions (
			id,
			challenge_pack_id,
			version_number,
			lifecycle_status,
			manifest_checksum,
			manifest,
			published_at
		)
		VALUES ($1, $2, $3, 'runnable', $4, $5, $6)
	`, versionID, packID, params.Bundle.Version.Number, checksum(manifest), manifest, now); err != nil {
		if isDuplicateVersionError(err) {
			return PublishedChallengePack{}, ErrChallengePackVersionExists
		}
		return PublishedChallengePack{}, fmt.Errorf("insert challenge pack version: %w", err)
	}

	specDefinition, err := json.Marshal(params.Bundle.Version.EvaluationSpec)
	if err != nil {
		return PublishedChallengePack{}, fmt.Errorf("marshal evaluation spec definition: %w", err)
	}

	var evaluationSpecID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO evaluation_specs (
			challenge_pack_version_id,
			name,
			version_number,
			judge_mode,
			definition
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, versionID, params.Bundle.Version.EvaluationSpec.Name, params.Bundle.Version.EvaluationSpec.VersionNumber, string(params.Bundle.Version.EvaluationSpec.JudgeMode), specDefinition).Scan(&evaluationSpecID); err != nil {
		return PublishedChallengePack{}, fmt.Errorf("insert evaluation spec: %w", err)
	}

	challengeIdentityByKey := make(map[string]uuid.UUID, len(params.Bundle.Challenges))
	for index, challenge := range params.Bundle.Challenges {
		challengeIdentityID, resolveErr := resolveOrCreateChallengeIdentity(ctx, tx, packID, challenge)
		if resolveErr != nil {
			return PublishedChallengePack{}, resolveErr
		}
		challengeIdentityByKey[challenge.Key] = challengeIdentityID

		definitionJSON, marshalErr := json.Marshal(challenge.Definition)
		if marshalErr != nil {
			return PublishedChallengePack{}, fmt.Errorf("marshal challenge definition for %s: %w", challenge.Key, marshalErr)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO challenge_pack_version_challenges (
				id,
				challenge_pack_version_id,
				challenge_pack_id,
				challenge_identity_id,
				execution_order,
				title_snapshot,
				category_snapshot,
				difficulty_snapshot,
				challenge_definition
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, uuid.New(), versionID, packID, challengeIdentityID, index, challenge.Title, challenge.Category, challenge.Difficulty, definitionJSON); err != nil {
			return PublishedChallengePack{}, fmt.Errorf("insert challenge version snapshot for %s: %w", challenge.Key, err)
		}
	}

	inputSetIDs := make([]uuid.UUID, 0, len(params.Bundle.InputSets))
	for _, inputSet := range params.Bundle.InputSets {
		itemsJSON, err := json.Marshal(inputSet.Cases)
		if err != nil {
			return PublishedChallengePack{}, fmt.Errorf("marshal input set %s checksum payload: %w", inputSet.Key, err)
		}

		inputSetID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO challenge_input_sets (
				id,
				challenge_pack_version_id,
				input_key,
				name,
				description,
				input_checksum
			)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, inputSetID, versionID, inputSet.Key, inputSet.Name, inputSet.Description, checksum(itemsJSON)); err != nil {
			return PublishedChallengePack{}, fmt.Errorf("insert challenge input set %s: %w", inputSet.Key, err)
		}
		inputSetIDs = append(inputSetIDs, inputSetID)

		for _, item := range inputSet.Cases {
			payloadJSON, err := item.StoredPayload()
			if err != nil {
				return PublishedChallengePack{}, fmt.Errorf("marshal input item %s/%s: %w", item.ChallengeKey, item.EffectiveKey(), err)
			}

			challengeIdentityID, ok := challengeIdentityByKey[item.ChallengeKey]
			if !ok {
				return PublishedChallengePack{}, fmt.Errorf("missing challenge identity for key %s during publish", item.ChallengeKey)
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO challenge_input_items (
					id,
					challenge_input_set_id,
					challenge_pack_version_id,
					challenge_identity_id,
					item_key,
					payload
				)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, uuid.New(), inputSetID, versionID, challengeIdentityID, item.EffectiveKey(), payloadJSON); err != nil {
				return PublishedChallengePack{}, fmt.Errorf("insert challenge input item %s/%s: %w", item.ChallengeKey, item.EffectiveKey(), err)
			}
		}
	}

	published := PublishedChallengePack{
		ChallengePackID:        packID,
		ChallengePackVersionID: versionID,
		EvaluationSpecID:       evaluationSpecID,
		InputSetIDs:            inputSetIDs,
	}

	if params.BundleArtifact != nil {
		artifactParams := *params.BundleArtifact
		artifactParams.OrganizationID = params.OrganizationID
		artifactParams.WorkspaceID = params.WorkspaceID

		metadata := map[string]any{
			"challenge_pack_id":         packID.String(),
			"challenge_pack_version_id": versionID.String(),
			"challenge_pack_slug":       params.Bundle.Pack.Slug,
			"challenge_pack_name":       params.Bundle.Pack.Name,
			"version_number":            params.Bundle.Version.Number,
		}

		if len(artifactParams.Metadata) > 0 {
			var existing map[string]any
			if err := json.Unmarshal(artifactParams.Metadata, &existing); err != nil {
				return PublishedChallengePack{}, fmt.Errorf("decode challenge pack bundle artifact metadata: %w", err)
			}
			for key, value := range existing {
				metadata[key] = value
			}
		}

		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return PublishedChallengePack{}, fmt.Errorf("marshal challenge pack bundle artifact metadata: %w", err)
		}
		artifactParams.Metadata = metadataJSON

		artifact, err := createArtifactTx(ctx, tx, artifactParams)
		if err != nil {
			return PublishedChallengePack{}, fmt.Errorf("create challenge pack bundle artifact: %w", err)
		}
		published.BundleArtifactID = &artifact.ID
	}

	if err := tx.Commit(ctx); err != nil {
		return PublishedChallengePack{}, fmt.Errorf("commit challenge pack publish: %w", err)
	}

	return published, nil
}

func resolveOrCreateChallengePack(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, bundle challengepack.Bundle) (uuid.UUID, error) {
	var (
		packID         uuid.UUID
		existingName   string
		existingFamily string
	)
	if err := tx.QueryRow(ctx, `
		INSERT INTO challenge_packs (
			workspace_id,
			slug,
			name,
			family,
			description,
			lifecycle_status
		)
		VALUES ($1, $2, $3, $4, $5, 'active')
		ON CONFLICT (workspace_id, slug) WHERE workspace_id IS NOT NULL
		DO UPDATE SET slug = EXCLUDED.slug
		RETURNING id, name, family
	`, workspaceID, bundle.Pack.Slug, bundle.Pack.Name, bundle.Pack.Family, bundle.Pack.Description).Scan(&packID, &existingName, &existingFamily); err != nil {
		return uuid.Nil, fmt.Errorf("insert challenge pack: %w", err)
	}

	if existingName != bundle.Pack.Name || existingFamily != bundle.Pack.Family {
		return uuid.Nil, ErrChallengePackMetadataConflict
	}

	return packID, nil
}

func isDuplicateVersionError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func createArtifactTx(ctx context.Context, tx pgx.Tx, params CreateArtifactParams) (Artifact, error) {
	row := tx.QueryRow(ctx, `
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
		return Artifact{}, err
	}

	return artifact, nil
}

func resolveOrCreateChallengeIdentity(ctx context.Context, tx pgx.Tx, challengePackID uuid.UUID, challenge challengepack.ChallengeDefinition) (uuid.UUID, error) {
	var existingID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM challenge_identities
		WHERE challenge_pack_id = $1
		  AND challenge_key = $2
		  AND archived_at IS NULL
		LIMIT 1
	`, challengePackID, challenge.Key).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("resolve challenge identity %s: %w", challenge.Key, err)
	}

	var insertedID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO challenge_identities (
			challenge_pack_id,
			challenge_key,
			name,
			category,
			difficulty,
			description
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, challengePackID, challenge.Key, challenge.Title, challenge.Category, challenge.Difficulty, challenge.Instructions).Scan(&insertedID); err != nil {
		return uuid.Nil, fmt.Errorf("insert challenge identity %s: %w", challenge.Key, err)
	}

	return insertedID, nil
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
