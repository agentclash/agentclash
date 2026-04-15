package repository

import (
	"context"
	"encoding/base64"
	"errors"
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
	Status     string
	UserID     *uuid.UUID
	CLITokenID *uuid.UUID
	RawToken   *string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

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
		if errors.Is(err, pgx.ErrNoRows) {
			return CLIToken{}, ErrCLITokenNotFound
		}
		return CLIToken{}, fmt.Errorf("get cli token by hash: %w", err)
	}
	return token, nil
}

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

func (r *Repository) TouchCLITokenLastUsed(ctx context.Context, tokenID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE cli_tokens SET last_used_at = now() WHERE id = $1`, tokenID)
	return err
}

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
		if errors.Is(err, pgx.ErrNoRows) {
			return DeviceAuthCode{}, ErrDeviceCodeNotFound
		}
		return DeviceAuthCode{}, fmt.Errorf("get device auth code: %w", err)
	}
	return code, nil
}

func (r *Repository) ApproveDeviceAuthCodeWithToken(ctx context.Context, userCode string, userID uuid.UUID, tokenHash, tokenName, rawToken string, expiresAt *time.Time) (CLIToken, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CLIToken{}, fmt.Errorf("begin cli auth approval tx: %w", err)
	}
	defer rollback(ctx, tx)

	var codeID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM device_auth_codes
		WHERE user_code = $1 AND status = 'pending'
		FOR UPDATE
	`, userCode).Scan(&codeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CLIToken{}, ErrDeviceCodeNotFound
		}
		return CLIToken{}, fmt.Errorf("lookup device auth code for approval: %w", err)
	}

	var token CLIToken
	err = tx.QueryRow(ctx, `
		INSERT INTO cli_tokens (user_id, token_hash, name, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, token_hash, name, last_used_at, expires_at, created_at, revoked_at
	`, userID, tokenHash, tokenName, expiresAt).Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.Name,
		&token.LastUsedAt, &token.ExpiresAt, &token.CreatedAt, &token.RevokedAt,
	)
	if err != nil {
		return CLIToken{}, fmt.Errorf("create cli token during device approval: %w", err)
	}

	storedToken := rawToken
	if r.cipher != nil {
		ciphertext, encErr := r.cipher.Encrypt([]byte(rawToken))
		if encErr != nil {
			return CLIToken{}, fmt.Errorf("encrypt device raw token: %w", encErr)
		}
		storedToken = base64.StdEncoding.EncodeToString(ciphertext)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE device_auth_codes
		SET status = 'approved', user_id = $2, cli_token_id = $3, raw_token = $4
		WHERE id = $1 AND status = 'pending' AND expires_at > now()
	`, codeID, userID, token.ID, storedToken)
	if err != nil {
		return CLIToken{}, fmt.Errorf("approve device auth code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var expired bool
		_ = tx.QueryRow(ctx, `SELECT expires_at <= now() FROM device_auth_codes WHERE id = $1`, codeID).Scan(&expired)
		if expired {
			return CLIToken{}, ErrDeviceCodeExpired
		}
		return CLIToken{}, ErrDeviceCodeNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return CLIToken{}, fmt.Errorf("commit cli auth approval tx: %w", err)
	}
	return token, nil
}

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
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("consume device raw token: %w", err)
	}
	if rawToken == nil {
		return "", nil
	}
	if r.cipher != nil {
		ciphertext, err := base64.StdEncoding.DecodeString(*rawToken)
		if err != nil {
			return "", fmt.Errorf("decode device raw token: %w", err)
		}
		plaintext, err := r.cipher.Decrypt(ciphertext)
		if err != nil {
			return "", fmt.Errorf("decrypt device raw token: %w", err)
		}
		return string(plaintext), nil
	}
	return *rawToken, nil
}

func (r *Repository) ExpireDeviceAuthCode(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE device_auth_codes
		SET status = 'expired', raw_token = NULL
		WHERE id = $1 AND status = 'pending'
	`, id)
	return err
}

func (r *Repository) ExpireStaleDeviceAuthCodes(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin stale cli auth cleanup tx: %w", err)
	}
	defer rollback(ctx, tx)

	if _, err := tx.Exec(ctx, `
		UPDATE device_auth_codes
		SET status = 'expired', raw_token = NULL
		WHERE expires_at <= now() AND status IN ('pending', 'approved')
	`); err != nil {
		return fmt.Errorf("expire stale device auth codes: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE cli_tokens ct
		SET revoked_at = now()
		FROM device_auth_codes dac
		WHERE dac.cli_token_id = ct.id
		  AND dac.status = 'expired'
		  AND dac.expires_at <= now()
		  AND ct.revoked_at IS NULL
		  AND ct.last_used_at IS NULL
	`); err != nil {
		return fmt.Errorf("revoke stale cli tokens: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit stale cli auth cleanup tx: %w", err)
	}
	return nil
}
