package api

import (
	"context"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"testing"
)

// Test fixture mounts the actual share handlers, authorizer and repository.
func mountBrowserReportSnapshots(t *testing.T, router chi.Router, db *pgxpool.Pool, store *vibe.Store, identity Authenticator) {
	t.Helper()
	for _, row := range []struct{ id, role string }{{"a8000000-0000-4000-8000-000000000004", "workspace_member"}, {"a8000000-0000-4000-8000-000000000005", "workspace_viewer"}} {
		for _, q := range []string{
			`INSERT INTO users(id,workos_user_id,email) VALUES($1::uuid,$1::text,$1::text||'@example.invalid')`,
			`INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES('a8000000-0000-4000-8000-000000000002',$1,'org_member','active')`,
		} {
			if _, err := db.Exec(context.Background(), q, row.id); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(context.Background(), `INSERT INTO workspace_memberships(workspace_id,organization_id,user_id,role,membership_status) VALUES('a8000000-0000-4000-8000-000000000003','a8000000-0000-4000-8000-000000000002',$1,$2,'active')`, row.id, row.role); err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.New(db)
	manager := NewPublicShareManager(NewCallerWorkspaceAuthorizer(repo), repo, browserFixtureEnv("VIBE_BROWSER_WEB_ORIGIN", "http://127.0.0.1:53518"))
	router.Group(func(r chi.Router) {
		r.Use(authenticateRequest(slog.Default(), browserConnectorAuth{identity: identity, repo: repo}))
		r.Get("/v1/auth/session", sessionHandler)
		r.Post("/v1/share-links", createShareLinkHandler(slog.Default(), manager))
		r.Patch("/v1/share-links/{shareID}", updateShareLinkHandler(slog.Default(), manager))
		r.Delete("/v1/share-links/{shareID}", revokeShareLinkHandler(slog.Default(), manager))
	})
	router.Get("/public/shares/{token}", getPublicShareHandler(slog.Default(), manager))
	router.Post("/__fixture/report-cleanup", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.URL.Query().Get("session"))
		if err != nil {
			http.Error(w, "session required", 400)
			return
		}
		if err = store.CleanupProjects(r.Context()); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		var raw int
		if err = db.QueryRow(r.Context(), "SELECT count(*) FROM vibe_sdk_reports WHERE session_id=$1", id).Scan(&raw); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		browserFixtureJSON(w, map[string]any{"raw_reports": raw})
	})
	router.Post("/__fixture/report-expire", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.URL.Query().Get("link"))
		if err != nil {
			http.Error(w, "link required", 400)
			return
		}
		if _, err = db.Exec(r.Context(), "UPDATE public_share_links SET expires_at=now()-interval '1 second' WHERE id=$1 AND resource_type='report_snapshot'", id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		browserFixtureJSON(w, map[string]any{"expired": true})
	})
}
