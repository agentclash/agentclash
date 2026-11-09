package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
)

func TestVibeConfigEntryTrialIntegration(t *testing.T) {
	f, workspace, _ := sdkHarness(t)
	h := &VibeHandler{Service: f.svc, CookieSecret: strings.Repeat("s", 32)}
	for _, tt := range []struct {
		name, phase, kind, state                            string
		anonymous, stopped, saved, used, pending, noSession bool
	}{
		{name: "no session", noSession: true},
		{name: "unused session"},
		{name: "used initial check", kind: "check", state: "COMPLETED", anonymous: true, used: true},
		{name: "failed check still used", kind: "check", state: "FAILED", anonymous: true, used: true},
		{name: "paid check does not consume guest trial", kind: "check", state: "COMPLETED"},
		{name: "retest does not consume initial check", kind: "retest", state: "COMPLETED", anonymous: true},
		{name: "preparing", phase: "preparing", pending: true},
		{name: "clarifying", phase: "clarifying", pending: true},
		{name: "ready", phase: "ready", pending: true},
		{name: "trying", phase: "trying", pending: true},
		{name: "checking", phase: "checking", pending: true, kind: "check", state: "RUNNING", anonymous: true, used: true},
		{name: "complete", phase: "complete"},
		{name: "error", phase: "error"},
		{name: "stopped cycle", phase: "ready", stopped: true},
		{name: "saved trial still used", kind: "check", state: "COMPLETED", anonymous: true, saved: true, used: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cookie := entryGuestCookie(h)
			r := httptest.NewRequest("GET", "/config", nil)
			r.AddCookie(cookie)
			actor := h.anonymous(r)
			if !tt.noSession {
				s, err := f.svc.Store.CreateAgent(f.ctx, actor, nil, uuid.New(), "build", f.svc.Config.DefaultModels())
				if err != nil {
					t.Fatal(err)
				}
				if tt.phase != "" {
					cycle := uuid.New()
					if tt.stopped {
						_, err = f.db.Exec(f.ctx, `INSERT INTO vibe_cycle_quotes(id,session_id,request_hash,specification,max_cost,expires_at,stopped_at) VALUES($1,$2,'fixture','{}',0,now()+interval '1 hour',now())`, cycle, s.ID)
						if err != nil {
							t.Fatal(err)
						}
					}
					b, _ := json.Marshal(map[string]any{"phase": tt.phase, "cycle_id": cycle})
					_, err = f.db.Exec(f.ctx, `UPDATE vibe_sessions SET document=jsonb_set(document,'{build}',$2::jsonb) WHERE id=$1`, s.ID, b)
					if err != nil {
						t.Fatal(err)
					}
				}
				if tt.kind != "" {
					b, _ := json.Marshal(map[string]bool{"anonymous": tt.anonymous})
					_, err = f.db.Exec(f.ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) VALUES($1,$2,$3,$4,'fixture',$5,$6,'UNRESERVED','{}',$7,0,now()+interval '1 hour')`, uuid.New(), s.ID, actor, uuid.New(), tt.kind, tt.state, b)
					if err != nil {
						t.Fatal(err)
					}
				}
				if tt.saved {
					// Saving changes ownership but retains the original guest trial_key.
					var trialKey string
					err = f.db.QueryRow(f.ctx, `UPDATE vibe_sessions SET actor=$2,workspace_id=$3 WHERE id=$1 RETURNING trial_key`, s.ID, f.actor, workspace).Scan(&trialKey)
					if err != nil {
						t.Fatal(err)
					}
					if trialKey != actor {
						t.Fatalf("saved trial_key=%q, want %q", trialKey, actor)
					}
				}
			}
			var before, after string
			snapshot := `SELECT jsonb_build_array((SELECT count(*) FROM vibe_sessions),(SELECT count(*) FROM vibe_grants),(SELECT count(*) FROM vibe_operations))::text`
			if err := f.db.QueryRow(f.ctx, snapshot).Scan(&before); err != nil {
				t.Fatal(err)
			}
			assertEntryConfig(t, h, cookie, "", true, tt.used, tt.pending)
			assertEntryConfig(t, h, cookie, "Bearer signed-in", false, false, false)
			assertEntryConfig(t, h, entryGuestCookie(h), "", true, false, false)
			if err := f.db.QueryRow(f.ctx, snapshot).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatalf("config wrote state: %s -> %s", before, after)
			}
		})
	}
}

func TestVibeTrialKeyIndexIntegration(t *testing.T) {
	f := newVibeAPIHarness(t)
	var valid bool
	var definition string
	err := f.db.QueryRow(f.ctx, `SELECT i.indisvalid,pg_get_indexdef(i.indexrelid)
	 FROM pg_index i WHERE i.indexrelid=to_regclass('vibe_sessions_trial_key_idx')
	 AND i.indrelid='vibe_sessions'::regclass`).Scan(&valid, &definition)
	if err != nil {
		t.Fatal(err)
	}
	if !valid || !strings.Contains(definition, "USING btree (trial_key) WHERE (trial_key IS NOT NULL)") {
		t.Fatalf("invalid trial index: valid=%v definition=%s", valid, definition)
	}
}

func TestVibeSessionTitleIntegration(t *testing.T) {
	f := newVibeAPIHarness(t)
	h := &VibeHandler{Service: f.svc, CookieSecret: strings.Repeat("s", 32)}
	for _, tt := range []struct {
		name, door string
		title      *string
		status     int
		want       string
	}{
		{name: "valid", door: "build", title: entryTitle("  Returns agent  "), status: 201, want: "Returns agent"},
		{name: "Unicode 80 characters", door: "test", title: entryTitle(strings.Repeat("界", 80)), status: 201, want: strings.Repeat("界", 80)},
		{name: "padded 80 characters", door: "build", title: entryTitle("  " + strings.Repeat("a", 80) + "  "), status: 201, want: strings.Repeat("a", 80)},
		{name: "too long", door: "build", title: entryTitle(strings.Repeat("a", 81)), status: 400},
		{name: "control", door: "test", title: entryTitle("bad\nname"), status: 400},
		{name: "empty build", door: "build", title: entryTitle(""), status: 201},
		{name: "spaces improve", door: "test", title: entryTitle("   "), status: 201},
		{name: "absent build", door: "build", status: 201},
		{name: "absent improve", door: "test", status: 201},
		{name: "legacy title", title: entryTitle(" Legacy title "), status: 201, want: "Legacy title"},
		{name: "legacy default", status: 201},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cookie := entryGuestCookie(h)
			id := uuid.New()
			input := map[string]any{"id": id}
			if tt.door != "" {
				input["door"] = tt.door
			}
			if tt.title != nil {
				input["title"] = *tt.title
			}
			post := func() *httptest.ResponseRecorder {
				b, _ := json.Marshal(input)
				r := httptest.NewRequest("POST", "/sessions", bytes.NewReader(b))
				r.Header.Set("Content-Type", "application/json")
				r.AddCookie(cookie)
				w := httptest.NewRecorder()
				h.Routes().ServeHTTP(w, r)
				return w
			}
			w := post()
			if w.Code != tt.status {
				t.Fatalf("create: %d %s", w.Code, w.Body)
			}
			if tt.status == http.StatusBadRequest {
				if !strings.Contains(w.Body.String(), `"code":"invalid_request"`) {
					t.Fatal(w.Body)
				}
				var exists bool
				if err := f.db.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM vibe_sessions WHERE id=$1)`, id).Scan(&exists); err != nil || exists {
					t.Fatalf("invalid title created session: %v %v", exists, err)
				}
				return
			}
			want := tt.want
			if want == "" {
				prefix := "New evaluation "
				if tt.door == "build" {
					prefix = "New prototype "
				}
				want = prefix + id.String()[:8]
			}
			var session vibe.Session
			if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
				t.Fatal(err)
			}
			if session.Title != want {
				t.Fatalf("title=%q, want %q", session.Title, want)
			}
			var stored string
			if err := f.db.QueryRow(f.ctx, `SELECT title FROM vibe_sessions WHERE id=$1`, id).Scan(&stored); err != nil || stored != want {
				t.Fatalf("stored title=%q: %v", stored, err)
			}
			input["title"] = "Retry title"
			w = post()
			if w.Code != 201 {
				t.Fatalf("retry: %d %s", w.Code, w.Body)
			}
			if err := json.Unmarshal(w.Body.Bytes(), &session); err != nil {
				t.Fatal(err)
			}
			if session.Title != want {
				t.Fatalf("retry changed title to %q", session.Title)
			}
		})
	}
}

func entryTitle(s string) *string { return &s }
