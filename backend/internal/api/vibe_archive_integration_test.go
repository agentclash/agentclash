package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestVibeIntegrationArchiveIsReadOnlyAndContinuesWithoutOldScores(t *testing.T) {
	if os.Getenv("VIBE_TEST_DATABASE_URL") == "" {
		t.Skip("requires isolated test database")
	}
	ctx := context.Background()
	db := browserFixtureDatabase(t, ctx)
	cfg := vibe.Config{TwoDoor: true, LocalTesting: true}
	store := vibe.NewStore(db, cfg)
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	h := &VibeHandler{Service: &vibe.Service{Store: store, Config: cfg, Compiler: VibePackCompiler{}, Gate: vibe.Gate{Redis: rc}}, CookieSecret: strings.Repeat("s", 32)}
	w := httptest.NewRecorder()
	actor, err := h.issue(w, httptest.NewRequest("POST", "/sessions", nil))
	if err != nil {
		t.Fatal(err)
	}
	cookie := w.Result().Cookies()[0]
	old, err := store.CreateSession(ctx, actor, nil, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	a := vibe.Artifact{ID: uuid.New(), Kind: "test_suite", Title: "Earlier returns agent", AgentPrompt: "Only unopened items qualify.", Blueprint: json.RawMessage(vibeBlueprint)}
	if err = store.Edit(ctx, actor, old.ID, old.Revision, func(v *vibe.Session) error {
		v.Document.Artifacts = []vibe.Artifact{a}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	routes := h.Routes()
	call := func(method, path string, body any, withCookie bool) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if withCookie {
			r.AddCookie(cookie)
		}
		out := httptest.NewRecorder()
		routes.ServeHTTP(out, r)
		return out
	}
	path := "/sessions/" + old.ID.String()
	if r := call("GET", path, nil, true); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, suffix := range []string{"/messages", "/import", "/evidence", "/build-quote"} {
		r := call("POST", path+suffix, map[string]any{}, true)
		if !strings.Contains(r.Body.String(), "Continue in V1") {
			t.Fatal("archive write reached execution", suffix, r.Code, r.Body.String())
		}
	}
	operation := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) VALUES($1,$2,$3,$4,'archive-fixture','check','AWAITING_APPROVAL','UNRESERVED','{}','{}',0,now()+interval '1 hour')`, operation, old.ID, actor, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if r := call("POST", "/operations/"+operation.String()+"/approve", map[string]any{}, true); r.Code != 409 {
		t.Fatal("archive approval bypass", r.Code, r.Body.String())
	}
	if r := call("POST", "/operations/"+operation.String()+"/stop", map[string]any{}, true); r.Code != 200 {
		t.Fatal("archive Stop was blocked", r.Code, r.Body.String())
	}
	request := map[string]any{"client_id": uuid.New(), "artifact_id": a.ID}
	if r := call("POST", path+"/continue", request, false); r.Code < 400 {
		t.Fatal("copy lacked owner authorization")
	}
	r := call("POST", path+"/continue", request, true)
	if r.Code != 201 {
		t.Fatal(r.Code, r.Body.String())
	}
	var copied vibe.Session
	if err = json.Unmarshal(r.Body.Bytes(), &copied); err != nil {
		t.Fatal(err)
	}
	if copied.Document.FormatVersion != 1 || len(copied.Operations) != 0 || copied.Document.Artifacts[0].AgentPrompt != a.AgentPrompt {
		t.Fatal("archive copy lost target or invented execution")
	}
	r = call("POST", "/sessions/"+copied.ID.String()+"/evidence", map[string]any{}, true)
	if !strings.Contains(r.Body.String(), "not available in V1") {
		t.Fatal("retired intake is still writable", r.Body.String())
	}
}
