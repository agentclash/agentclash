package vibe

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
)

func TestIntegrationResultEvidenceReadinessRequiresMigration(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	schema := "result_ready_" + uuid.New().String()[:8]
	if _, err := s.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.DB.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	cfg := s.DB.Config()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"vibe_sessions", "vibe_operations", "vibe_outbox", "vibe_attempts", "vibe_accounts"} {
		if _, err = db.Exec(ctx, "CREATE TABLE "+name+" (LIKE public."+name+")"); err != nil {
			t.Fatal(err)
		}
	}
	r := Readiness{DB: db}
	if r.DependencyChecks(ctx)["vibe_schema"] == "ok" {
		t.Fatal("pre-result migration schema advertised ready")
	}
	for _, name := range []string{"vibe_evidence_receipts", "vibe_grade_reviews", "vibe_validation_reviews"} {
		if _, err = db.Exec(ctx, "CREATE TABLE "+name+" (LIKE public."+name+")"); err != nil {
			t.Fatal(err)
		}
	}
	if r.DependencyChecks(ctx)["vibe_schema"] == "ok" {
		t.Fatal("pre-library migration advertised ready")
	}
	for _, name := range []string{"vibe_import_originals", "vibe_pack_sources", "vibe_published_case_bindings"} {
		if _, err = db.Exec(ctx, "CREATE TABLE "+name+" (LIKE public."+name+")"); err != nil {
			t.Fatal(err)
		}
	}
	if r.DependencyChecks(ctx)["vibe_schema"] == "ok" {
		t.Fatal("pre-SDK migration advertised ready")
	}
	for _, name := range []string{"vibe_sdk_reports", "vibe_sdk_promotions"} {
		if _, err = db.Exec(ctx, "CREATE TABLE "+name+" (LIKE public."+name+")"); err != nil {
			t.Fatal(err)
		}
	}
	if r.DependencyChecks(ctx)["vibe_schema"] != "ok" {
		t.Fatal("migrated schema unavailable")
	}
	if _, err = db.Exec(ctx, "ALTER TABLE vibe_sdk_reports DROP COLUMN source_json"); err != nil {
		t.Fatal(err)
	}
	if r.DependencyChecks(ctx)["vibe_schema"] == "ok" {
		t.Fatal("missing SDK original column advertised ready")
	}
	if _, err = db.Exec(ctx, "ALTER TABLE vibe_sdk_reports ADD COLUMN source_json bytea"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "ALTER TABLE vibe_grade_reviews DROP COLUMN request_hash"); err != nil {
		t.Fatal(err)
	}
	if r.DependencyChecks(ctx)["vibe_schema"] == "ok" {
		t.Fatal("missing review receipt column advertised ready")
	}
}
