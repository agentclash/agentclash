package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type WorkspaceSecretMetadata struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Key         string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   *uuid.UUID
	UpdatedBy   *uuid.UUID
}

type UpsertWorkspaceSecretParams struct {
	WorkspaceID uuid.UUID
	Key         string
	Value       string
	ActorUserID *uuid.UUID
}

var secretKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// IsValidSecretKey mirrors the CHECK constraint on workspace_secrets.key so
// callers can reject bad input before a round-trip to Postgres.
func IsValidSecretKey(key string) bool {
	if len(key) == 0 || len(key) > 128 {
		return false
	}
	return secretKeyPattern.MatchString(key)
}

func (r *Repository) ListWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceSecretMetadata, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, workspace_id, key, created_at, updated_at, created_by, updated_by
		FROM workspace_secrets
		WHERE workspace_id = $1
		ORDER BY key ASC
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace secrets: %w", err)
	}
	defer rows.Close()

	var out []WorkspaceSecretMetadata
	for rows.Next() {
		var metadata WorkspaceSecretMetadata
		if err := rows.Scan(
			&metadata.ID,
			&metadata.WorkspaceID,
			&metadata.Key,
			&metadata.CreatedAt,
			&metadata.UpdatedAt,
			&metadata.CreatedBy,
			&metadata.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan workspace secret metadata: %w", err)
		}
		out = append(out, metadata)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace secrets: %w", err)
	}
	return out, nil
}

func (r *Repository) UpsertWorkspaceSecret(ctx context.Context, params UpsertWorkspaceSecretParams) error {
	if r.cipher == nil {
		return ErrSecretsCipherUnset
	}
	if !IsValidSecretKey(params.Key) {
		return ErrInvalidSecretKey
	}
	ciphertext, err := r.cipher.Encrypt([]byte(params.Value))
	if err != nil {
		return fmt.Errorf("encrypt workspace secret: %w", err)
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO workspace_secrets (workspace_id, key, encrypted_value, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (workspace_id, key) DO UPDATE
		SET encrypted_value = EXCLUDED.encrypted_value,
		    updated_by = EXCLUDED.updated_by,
		    updated_at = now()
	`, params.WorkspaceID, params.Key, ciphertext, params.ActorUserID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return ErrInvalidSecretKey
		}
		return fmt.Errorf("upsert workspace secret: %w", err)
	}
	return nil
}

func (r *Repository) DeleteWorkspaceSecret(ctx context.Context, workspaceID uuid.UUID, key string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM workspace_secrets
		WHERE workspace_id = $1 AND key = $2
	`, workspaceID, key)
	if err != nil {
		return fmt.Errorf("delete workspace secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrWorkspaceSecretNotFound
	}
	return nil
}

// LoadWorkspaceSecrets returns every secret in the workspace as a decrypted
// plaintext map. It is only meant for internal engine use (resolving
// ${secrets.X} at run start); it MUST NOT be exposed through the HTTP API.
func (r *Repository) LoadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error) {
	if r.cipher == nil {
		return nil, ErrSecretsCipherUnset
	}
	rows, err := r.db.Query(ctx, `
		SELECT key, encrypted_value
		FROM workspace_secrets
		WHERE workspace_id = $1
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("load workspace secrets: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var key string
		var ciphertext []byte
		if err := rows.Scan(&key, &ciphertext); err != nil {
			return nil, fmt.Errorf("scan workspace secret: %w", err)
		}
		plaintext, err := r.cipher.Decrypt(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt workspace secret %q: %w", key, err)
		}
		out[key] = string(plaintext)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace secrets: %w", err)
	}
	return out, nil
}
