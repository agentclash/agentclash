package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CLIToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  string
	Name       string
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
}

type DeviceAuthCode struct {
	ID         uuid.UUID
	DeviceCode string
	UserCode   string
	Status     string // pending | approved | denied | expired
	UserID     *uuid.UUID
	CLITokenID *uuid.UUID
	RawToken   *string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// GetUserByID returns an active (non-archived) user by internal ID.
func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (User, error) {
	var user User
	err := r.db.QueryRow(ctx, `
		SELECT id, workos_user_id, email, COALESCE(display_name, '')
		FROM users
		WHERE id = $1 AND archived_at IS NULL
	`, userID).Scan(&user.ID, &user.WorkOSUserID, &user.Email, &user.DisplayName)
	if err != nil {
		if err == pgx.ErrNoRows {
			return User{}, fmt.Errorf("user not found: %s", userID)
		}
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

// CreateCLIToken inserts a new CLI token row.
func (r *Repository) CreateCLIToken(ctx context.Context, userID uuid.UUID, tokenHash, name string, expiresAt *time.Time) (CLIToken, error) {
	var token CLIToken
	err := r.db.QueryRow(ctx, `
		INSERT INTO cli_tokens (user_id, token_hash, name, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, token_hash, name, last_used_at, expires_at, created_at, revoked_at
	`, userID, tokenHash, name, expiresAt).Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.Name,
		&token.LastUsedAt, &token.ExpiresAt, &token.CreatedAt, &token.RevokedAt,
	)
	if err != nil {
		return CLIToken{}, fmt.Errorf("create cli token: %w", err)
	}
	return token, nil
}

// GetCLITokenByHash looks up a CLI token by its SHA-256 hash.
func (r *Repository) GetCLITokenByHash(ctx context.Context, tokenHash string) (CLIToken, error) {
	var token CLIToken
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, name, last_used_at, expires_at, created_at, revoked_at
		FROM cli_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.Name,
		&token.LastUsedAt, &token.ExpiresAt, &token.CreatedAt, &token.RevokedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return CLIToken{}, ErrCLITokenNotFound
		}
		return CLIToken{}, fmt.Errorf("get cli token by hash: %w", err)
	}
	return token, nil
}

// ListCLITokensByUserID returns all non-revoked CLI tokens for a user.
func (r *Repository) ListCLITokensByUserID(ctx context.Context, userID uuid.UUID) ([]CLIToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, token_hash, name, last_used_at, expires_at, created_at, revoked_at
		FROM cli_tokens
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list cli tokens: %w", err)
	}
	defer rows.Close()

	var out []CLIToken
	for rows.Next() {
		var token CLIToken
		if err := rows.Scan(
			&token.ID, &token.UserID, &token.TokenHash, &token.Name,
			&token.LastUsedAt, &token.ExpiresAt, &token.CreatedAt, &token.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cli token: %w", err)
		}
		out = append(out, token)
	}
	return out, rows.Err()
}

// RevokeCLIToken sets revoked_at on a CLI token. Only the owning user can revoke.
func (r *Repository) RevokeCLIToken(ctx context.Context, tokenID, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE cli_tokens
		SET revoked_at = now()
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`, tokenID, userID)
	if err != nil {
		return fmt.Errorf("revoke cli token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCLITokenNotFound
	}
	return nil
}

// TouchCLITokenLastUsed updates the last_used_at timestamp.
func (r *Repository) TouchCLITokenLastUsed(ctx context.Context, tokenID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE cli_tokens SET last_used_at = now() WHERE id = $1
	`, tokenID)
	return err
}

// CreateDeviceAuthCode inserts a new device authorization code.
func (r *Repository) CreateDeviceAuthCode(ctx context.Context, deviceCode, userCode string, expiresAt time.Time) (DeviceAuthCode, error) {
	var code DeviceAuthCode
	err := r.db.QueryRow(ctx, `
		INSERT INTO device_auth_codes (device_code, user_code, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, device_code, user_code, status, user_id, cli_token_id, raw_token, expires_at, created_at
	`, deviceCode, userCode, expiresAt).Scan(
		&code.ID, &code.DeviceCode, &code.UserCode, &code.Status,
		&code.UserID, &code.CLITokenID, &code.RawToken, &code.ExpiresAt, &code.CreatedAt,
	)
	if err != nil {
		return DeviceAuthCode{}, fmt.Errorf("create device auth code: %w", err)
	}
	return code, nil
}

// GetDeviceAuthCodeByDeviceCode looks up by the secret device code (CLI polling).
func (r *Repository) GetDeviceAuthCodeByDeviceCode(ctx context.Context, deviceCode string) (DeviceAuthCode, error) {
	var code DeviceAuthCode
	err := r.db.QueryRow(ctx, `
		SELECT id, device_code, user_code, status, user_id, cli_token_id, raw_token, expires_at, created_at
		FROM device_auth_codes
		WHERE device_code = $1
	`, deviceCode).Scan(
		&code.ID, &code.DeviceCode, &code.UserCode, &code.Status,
		&code.UserID, &code.CLITokenID, &code.RawToken, &code.ExpiresAt, &code.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return DeviceAuthCode{}, ErrDeviceCodeNotFound
		}
		return DeviceAuthCode{}, fmt.Errorf("get device auth code: %w", err)
	}
	return code, nil
}

// GetDeviceAuthCodeByUserCode looks up by the user-facing code (web app approval).
func (r *Repository) GetDeviceAuthCodeByUserCode(ctx context.Context, userCode string) (DeviceAuthCode, error) {
	var code DeviceAuthCode
	err := r.db.QueryRow(ctx, `
		SELECT id, device_code, user_code, status, user_id, cli_token_id, raw_token, expires_at, created_at
		FROM device_auth_codes
		WHERE user_code = $1 AND status = 'pending'
	`, userCode).Scan(
		&code.ID, &code.DeviceCode, &code.UserCode, &code.Status,
		&code.UserID, &code.CLITokenID, &code.RawToken, &code.ExpiresAt, &code.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return DeviceAuthCode{}, ErrDeviceCodeNotFound
		}
		return DeviceAuthCode{}, fmt.Errorf("get device auth code by user code: %w", err)
	}
	return code, nil
}

// ApproveDeviceAuthCode atomically transitions a pending device code to approved,
// links the CLI token, and stores the raw token for one-time retrieval by the polling CLI.
func (r *Repository) ApproveDeviceAuthCode(ctx context.Context, id, userID uuid.UUID, cliTokenID uuid.UUID, rawToken string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE device_auth_codes
		SET status = 'approved', user_id = $2, cli_token_id = $3, raw_token = $4
		WHERE id = $1 AND status = 'pending' AND expires_at > now()
	`, id, userID, cliTokenID, rawToken)
	if err != nil {
		return fmt.Errorf("approve device auth code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDeviceCodeNotFound
	}
	return nil
}

// ConsumeDeviceRawToken atomically reads and NULLs the raw_token.
// Uses a FROM subquery to capture the pre-update value, since PostgreSQL's
// RETURNING clause returns post-update values (which would be NULL).
func (r *Repository) ConsumeDeviceRawToken(ctx context.Context, id uuid.UUID) (string, error) {
	var rawToken *string
	err := r.db.QueryRow(ctx, `
		UPDATE device_auth_codes d
		SET raw_token = NULL
		FROM (SELECT id, raw_token FROM device_auth_codes WHERE id = $1 AND raw_token IS NOT NULL FOR UPDATE) old
		WHERE d.id = old.id
		RETURNING old.raw_token
	`, id).Scan(&rawToken)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("consume device raw token: %w", err)
	}
	if rawToken == nil {
		return "", nil
	}
	return *rawToken, nil
}

// ExpireDeviceAuthCode transitions a pending device code to expired.
func (r *Repository) ExpireDeviceAuthCode(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE device_auth_codes SET status = 'expired'
		WHERE id = $1 AND status = 'pending'
	`, id)
	return err
}
