package repository_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/secrets"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newSecretsRepo(t *testing.T, db *pgxpool.Pool) *repository.Repository {
	t.Helper()
	key := make([]byte, secrets.MasterKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate test master key: %v", err)
	}
	// Round-trip through base64 so we exercise the same constructor the
	// server uses at boot (NewAESGCMCipher from the env var).
	cipher, err := secrets.NewAESGCMCipher(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("construct test cipher: %v", err)
	}
	return repository.New(db).WithCipher(cipher)
}

type secretsFixture struct {
	organizationID uuid.UUID
	workspaceID    uuid.UUID
	userID         uuid.UUID
}

func seedSecretsFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) secretsFixture {
	t.Helper()
	if _, err := db.Exec(ctx, "TRUNCATE TABLE organizations, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("reset secrets fixture returned error: %v", err)
	}
	fixture := secretsFixture{
		organizationID: uuid.New(),
		workspaceID:    uuid.New(),
		userID:         uuid.New(),
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)
	`, fixture.organizationID, "Secrets Org", "secrets-org"); err != nil {
		t.Fatalf("insert organization returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO workspaces (id, organization_id, name, slug) VALUES ($1, $2, $3, $4)
	`, fixture.workspaceID, fixture.organizationID, "Secrets Workspace", "secrets-workspace"); err != nil {
		t.Fatalf("insert workspace returned error: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO users (id, workos_user_id, email, display_name) VALUES ($1, $2, $3, $4)
	`, fixture.userID, "workos-secrets-user", "secrets@example.com", "Secrets Owner"); err != nil {
		t.Fatalf("insert user returned error: %v", err)
	}
	return fixture
}

func TestRepositoryUpsertAndLoadWorkspaceSecret(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedSecretsFixture(t, ctx, db)
	repo := newSecretsRepo(t, db)

	if err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
		WorkspaceID: fixture.workspaceID,
		Key:         "DB_URL",
		Value:       "postgres://user:pass@host:5432/db",
		ActorUserID: &fixture.userID,
	}); err != nil {
		t.Fatalf("upsert returned error: %v", err)
	}

	loaded, err := repo.LoadWorkspaceSecrets(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}
	if loaded["DB_URL"] != "postgres://user:pass@host:5432/db" {
		t.Fatalf("loaded[DB_URL] = %q, want plaintext", loaded["DB_URL"])
	}

	// Listing returns metadata only — never the value.
	list, err := repo.ListWorkspaceSecrets(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if len(list) != 1 || list[0].Key != "DB_URL" {
		t.Fatalf("list = %+v, want [DB_URL]", list)
	}
	if list[0].CreatedBy == nil || *list[0].CreatedBy != fixture.userID {
		t.Fatalf("created_by = %v, want %s", list[0].CreatedBy, fixture.userID)
	}
}

func TestRepositoryUpsertWorkspaceSecret_OverwritesValue(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedSecretsFixture(t, ctx, db)
	repo := newSecretsRepo(t, db)

	params := repository.UpsertWorkspaceSecretParams{
		WorkspaceID: fixture.workspaceID,
		Key:         "API_KEY",
		Value:       "first",
		ActorUserID: &fixture.userID,
	}
	if err := repo.UpsertWorkspaceSecret(ctx, params); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	params.Value = "second"
	if err := repo.UpsertWorkspaceSecret(ctx, params); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	loaded, err := repo.LoadWorkspaceSecrets(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded["API_KEY"] != "second" {
		t.Fatalf("loaded[API_KEY] = %q, want \"second\"", loaded["API_KEY"])
	}
	list, err := repo.ListWorkspaceSecrets(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 row after overwrite, got %d", len(list))
	}
	if !list[0].UpdatedAt.After(list[0].CreatedAt) && !list[0].UpdatedAt.Equal(list[0].CreatedAt) {
		t.Fatalf("updated_at %v should be >= created_at %v", list[0].UpdatedAt, list[0].CreatedAt)
	}
}

func TestRepositoryDeleteWorkspaceSecret(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedSecretsFixture(t, ctx, db)
	repo := newSecretsRepo(t, db)

	if err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
		WorkspaceID: fixture.workspaceID,
		Key:         "TOKEN",
		Value:       "abc",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.DeleteWorkspaceSecret(ctx, fixture.workspaceID, "TOKEN"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.DeleteWorkspaceSecret(ctx, fixture.workspaceID, "TOKEN"); !errors.Is(err, repository.ErrWorkspaceSecretNotFound) {
		t.Fatalf("expected ErrWorkspaceSecretNotFound on second delete, got %v", err)
	}
	loaded, err := repo.LoadWorkspaceSecrets(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := loaded["TOKEN"]; ok {
		t.Fatalf("deleted secret still present in load")
	}
}

func TestRepositoryWorkspaceSecret_ScopedByWorkspace(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedSecretsFixture(t, ctx, db)
	repo := newSecretsRepo(t, db)

	otherWorkspaceID := uuid.New()
	if _, err := db.Exec(ctx, `
		INSERT INTO workspaces (id, organization_id, name, slug) VALUES ($1, $2, $3, $4)
	`, otherWorkspaceID, fixture.organizationID, "Other Workspace", "other-workspace"); err != nil {
		t.Fatalf("insert second workspace: %v", err)
	}

	if err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
		WorkspaceID: fixture.workspaceID,
		Key:         "SHARED_KEY",
		Value:       "one",
	}); err != nil {
		t.Fatalf("upsert in first workspace: %v", err)
	}
	if err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
		WorkspaceID: otherWorkspaceID,
		Key:         "SHARED_KEY",
		Value:       "two",
	}); err != nil {
		t.Fatalf("upsert in second workspace: %v", err)
	}

	first, err := repo.LoadWorkspaceSecrets(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("load first: %v", err)
	}
	second, err := repo.LoadWorkspaceSecrets(ctx, otherWorkspaceID)
	if err != nil {
		t.Fatalf("load second: %v", err)
	}
	if first["SHARED_KEY"] != "one" || second["SHARED_KEY"] != "two" {
		t.Fatalf("workspace isolation broken: first=%q second=%q", first["SHARED_KEY"], second["SHARED_KEY"])
	}
}

func TestRepositoryWorkspaceSecret_RejectsInvalidKey(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedSecretsFixture(t, ctx, db)
	repo := newSecretsRepo(t, db)

	badKeys := []string{"", "1STARTS_WITH_DIGIT", "has-dash", "has space", "hás-unicode"}
	for _, key := range badKeys {
		err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
			WorkspaceID: fixture.workspaceID,
			Key:         key,
			Value:       "whatever",
		})
		if !errors.Is(err, repository.ErrInvalidSecretKey) {
			t.Fatalf("key %q: expected ErrInvalidSecretKey, got %v", key, err)
		}
	}
}

func TestRepositoryWorkspaceSecret_ErrorsWithoutCipher(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedSecretsFixture(t, ctx, db)
	repo := repository.New(db) // no WithCipher

	if err := repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
		WorkspaceID: fixture.workspaceID,
		Key:         "ANY",
		Value:       "x",
	}); !errors.Is(err, repository.ErrSecretsCipherUnset) {
		t.Fatalf("upsert without cipher: expected ErrSecretsCipherUnset, got %v", err)
	}
	if _, err := repo.LoadWorkspaceSecrets(ctx, fixture.workspaceID); !errors.Is(err, repository.ErrSecretsCipherUnset) {
		t.Fatalf("load without cipher: expected ErrSecretsCipherUnset, got %v", err)
	}
}
