package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

func TestVibeIntegrationSessionListWorkspaceFilter(t *testing.T) {
	if os.Getenv("VIBE_TEST_DATABASE_URL") == "" {
		t.Skip("requires isolated migrated child database")
	}
	ctx := context.Background()
	db := browserFixtureDatabase(t, ctx)
	store := vibe.NewStore(db, vibe.Config{})
	user, other, org, a, b := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", []any{user, user.String(), user.String() + "@example.invalid"}},
		{"INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", []any{other, other.String(), other.String() + "@example.invalid"}},
		{"INSERT INTO organizations(id,name,slug) VALUES($1,'List fixture',$2)", []any{org, org.String()}},
		{"INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES($1,$2,'org_admin','active'),($1,$3,'org_admin','active')", []any{org, user, other}},
		{"INSERT INTO workspaces(id,organization_id,name,slug) VALUES($1,$2,'A',$3),($4,$2,'B',$5)", []any{a, org, a.String(), b, b.String()}},
	} {
		if _, err := db.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	actor := "user:" + user.String()
	for _, ws := range []*uuid.UUID{&a, &b, nil} {
		if _, err := store.CreateAgent(ctx, actor, ws, uuid.New(), "test", vibe.DefaultModels()); err != nil {
			t.Fatal(err)
		}
	}
	otherID := uuid.New()
	if _, err := store.CreateAgent(ctx, "user:"+other.String(), &a, otherID, "build", vibe.DefaultModels()); err != nil {
		t.Fatal(err)
	}
	// More recent unrelated rows must not consume the selected workspace's limit.
	document, err := json.Marshal(vibe.Document{Evaluation: &vibe.EvaluationContext{ID: uuid.New(), Door: "test"}, Models: vibe.DefaultModels()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO vibe_sessions(id,actor,workspace_id,document) SELECT gen_random_uuid(),$1,$2,$3::jsonb FROM generate_series(1,101)`, actor, b, document); err != nil {
		t.Fatal(err)
	}

	handler := (&VibeHandler{Service: &vibe.Service{Store: store}, Auth: NewDevelopmentAuthenticator()}).Routes()
	for _, tc := range []struct {
		query         string
		status, count int
		workspace     *uuid.UUID
	}{
		{"?workspace=" + a.String(), 200, 1, &a},
		{"?workspace=" + b.String(), 200, 100, &b},
		{"", 200, 100, nil},
		{"?workspace=" + uuid.NewString(), 200, 0, nil},
		{"?workspace=invalid", 400, 0, nil},
	} {
		t.Run(tc.query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/sessions"+tc.query, nil)
			r.Header.Set("Authorization", "Bearer fixture")
			r.Header.Set(headerUserID, user.String())
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if w.Code != 200 {
				return
			}
			var items []vibe.Session
			if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
				t.Fatal(err)
			}
			if len(items) != tc.count {
				t.Fatalf("got %d rows, want %d", len(items), tc.count)
			}
			for _, item := range items {
				if item.ID == otherID {
					t.Fatal("another actor's row returned")
				}
				if tc.workspace != nil && (item.WorkspaceID == nil || *item.WorkspaceID != *tc.workspace) {
					t.Fatal("another workspace's row returned")
				}
			}
		})
	}
}
