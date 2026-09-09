package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestVibeIntegrationSaveUsesSelectedModels(t *testing.T) {
	dsn := os.Getenv("VIBE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated migrated VIBE_TEST_DATABASE_URL")
	}
	if !strings.Contains(dsn, "vibe_test") {
		t.Fatal("refusing a non-test database")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &vibe.Store{DB: db}
	org, user, ws := uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO organizations(id,name,slug) VALUES($1,'Save fixture',$2)", []any{org, org.String()}},
		{"INSERT INTO workspaces(id,organization_id,name,slug) VALUES($1,$2,'Save fixture',$3)", []any{ws, org, ws.String()}},
		{"INSERT INTO users(id,workos_user_id,email) VALUES($1,$2,$3)", []any{user, user.String(), user.String() + "@example.invalid"}},
		{"INSERT INTO organization_memberships(organization_id,user_id,role,membership_status) VALUES($1,$2,'org_admin','active')", []any{org, user}},
	} {
		if _, err := db.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	actor := "user:" + user.String()
	session, err := store.CreateSession(ctx, actor, &ws, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	compiler := VibePackCompiler{}
	blueprint, err := compiler.Draft(vibe.DraftProposal{Title: "Copy helper", AgentPrompt: "Use only verified facts and a CTA.", Examples: []string{"Write a CTA without inventing product benefits."}, SuccessCriteria: "Uses only supplied facts and includes a CTA."}, vibe.LimitsFor(false))
	if err != nil {
		t.Fatal(err)
	}
	artifact := vibe.Artifact{ID: uuid.New(), Title: "Copy helper", AgentPrompt: "Use only verified facts and a CTA.", Blueprint: blueprint, Accepted: true, CreatedAt: time.Now()}
	if err := store.Edit(ctx, actor, session.ID, session.Revision, func(v *vibe.Session) error {
		v.Document.Artifacts = []vibe.Artifact{artifact}
		v.Document.ActiveArtifactID = &artifact.ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	session, err = store.GetSession(ctx, actor, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := vibe.Config{Profiles: map[string]vibe.ModelProfile{}}
	for _, id := range []string{"openai/gpt-4.1-mini", "openai/gpt-4.1", "openai/gpt-4o-mini"} {
		cfg.Profiles[id] = vibe.ModelProfile{ID: id, Route: "openai", InputNanoPerToken: 400, OutputNanoPerToken: 1600, Context: 128000, FramingAllowance: 2048, Conformed: true, ExpiresAt: time.Now().Add(time.Hour)}
	}
	svc := &vibe.Service{Store: store, Config: cfg, Compiler: compiler}
	handler := (&VibeHandler{Service: svc, Auth: NewDevelopmentAuthenticator()}).Routes()
	selected := vibe.Models{Assistant: "openai/gpt-4o-mini", Target: "openai/gpt-4.1", Evaluator: "openai/gpt-4o-mini"}
	payload := map[string]any{"revision": session.Revision, "artifact_id": artifact.ID, "workspace_id": ws, "models": selected}
	post := func(body map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/sessions/"+session.ID.String()+"/save", bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer development-fixture")
		r.Header.Set(headerUserID, user.String())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	response := post(payload)
	if response.Code != http.StatusOK {
		t.Fatalf("save rejected selected models: status=%d body=%s", response.Code, response.Body.String())
	}
	var saved struct {
		DraftID uuid.UUID `json:"draft_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	var target string
	var composition []byte
	if err := db.QueryRow(ctx, `SELECT v.model_spec->>'model',d.composition FROM vibe_saved_artifacts s JOIN agent_build_versions v ON v.id=s.build_version_id JOIN challenge_pack_drafts d ON d.id=s.draft_id WHERE s.draft_id=$1`, saved.DraftID).Scan(&target, &composition); err != nil {
		t.Fatal(err)
	}
	if target != selected.Target || !bytes.Contains(composition, []byte(selected.Evaluator)) {
		t.Fatalf("saved stale target/evaluator: target=%s composition=%s", target, composition)
	}
	session, err = store.GetSession(ctx, actor, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Document.Models != selected {
		t.Fatalf("snapshot retained stale models: %+v", session.Document.Models)
	}
	payload["revision"] = session.Revision
	serialized, _ := json.Marshal(session)
	var snapshot map[string]any
	if err := json.Unmarshal(serialized, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["saved_artifact_id"] != artifact.ID.String() {
		t.Fatalf("saved link lacks artifact identity: %s", serialized)
	}
	if m, ok := snapshot["saved_models"].(map[string]any); !ok || m["target"] != selected.Target || m["evaluator"] != selected.Evaluator {
		t.Fatalf("saved link lacks immutable model choices: %s", serialized)
	}
	t.Run("receipt is Vibe owned and immutable", func(t *testing.T) {
		var receipt []byte
		var canonicalHasReceipt bool
		if err := db.QueryRow(ctx, `SELECT to_jsonb(s)->'saved_models',v.model_spec ? 'vibe_models'
			FROM vibe_saved_artifacts s JOIN agent_build_versions v ON v.id=s.build_version_id WHERE s.draft_id=$1`, saved.DraftID).Scan(&receipt, &canonicalHasReceipt); err != nil {
			t.Fatal(err)
		}
		var got *vibe.Models
		if err := json.Unmarshal(receipt, &got); err != nil || got == nil || *got != selected || canonicalHasReceipt {
			t.Fatalf("save-time receipt is not Vibe-owned: receipt=%s canonical_has_receipt=%v err=%v", receipt, canonicalHasReceipt, err)
		}
		for _, replacement := range []any{vibeReliabilityJSON(t, vibe.DefaultModels()), nil} {
			if _, err := db.Exec(ctx, "UPDATE vibe_saved_artifacts SET saved_models=$2 WHERE draft_id=$1", saved.DraftID, replacement); err == nil || !strings.Contains(err.Error(), "saved model receipt is immutable") {
				t.Fatalf("receipt could be changed or removed: %v", err)
			}
		}
		current, err := store.GetSession(ctx, actor, session.ID)
		if err != nil || current.SavedModels == nil || *current.SavedModels != selected {
			t.Fatalf("rejected writes changed receipt: %+v %v", current.SavedModels, err)
		}
	})
	if len(session.Operations) != 0 {
		t.Fatal("saving dispatched work")
	}
	t.Run("repeat is idempotent but changed models are not silently ignored", func(t *testing.T) {
		payload["revision"] = session.Revision
		again := post(payload)
		if again.Code != http.StatusOK || again.Body.String() != response.Body.String() {
			t.Fatalf("repeat save changed its canonical identity: %d %s", again.Code, again.Body.String())
		}
		payload["models"] = vibe.DefaultModels()
		changed := post(payload)
		if changed.Code != http.StatusConflict || !strings.Contains(changed.Body.String(), "saved_model_conflict") {
			t.Fatalf("changed models reused a stale saved version: %d %s", changed.Code, changed.Body.String())
		}
		var versions int
		if err := db.QueryRow(ctx, "SELECT count(*) FROM vibe_saved_artifacts WHERE session_id=$1", session.ID).Scan(&versions); err != nil || versions != 1 {
			t.Fatalf("save retry created versions: %d %v", versions, err)
		}
	})
	t.Run("unsupported model cannot change saved state", func(t *testing.T) {
		payload["models"] = vibe.Models{Assistant: selected.Assistant, Target: "unapproved/provider-model", Evaluator: selected.Evaluator}
		bad := post(payload)
		if bad.Code != http.StatusServiceUnavailable || !strings.Contains(bad.Body.String(), "pricing_unavailable") {
			t.Fatalf("unsupported model accepted: %d %s", bad.Code, bad.Body.String())
		}
		current, err := store.GetSession(ctx, actor, session.ID)
		if err != nil || current.Revision != session.Revision || current.Document.Models != selected {
			t.Fatalf("failed save changed session: %+v %v", current, err)
		}
	})
	// The canonical draft PATCH uses this repository method to replace the
	// editable model_spec. A Vibe receipt must not trust any of its fields.
	repo := repository.New(db)
	mutateCanonical := func(t *testing.T, modelSpec json.RawMessage) {
		t.Helper()
		var versionID uuid.UUID
		if err := db.QueryRow(ctx, "SELECT build_version_id FROM vibe_saved_artifacts WHERE draft_id=$1", saved.DraftID).Scan(&versionID); err != nil {
			t.Fatal(err)
		}
		version, err := repo.GetAgentBuildVersionByID(ctx, versionID)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.UpdateAgentBuildVersionDraft(ctx, repository.UpdateAgentBuildVersionDraftParams{
			ID: version.ID, AgentKind: version.AgentKind, InterfaceSpec: version.InterfaceSpec,
			PolicySpec: version.PolicySpec, ReasoningSpec: version.ReasoningSpec, MemorySpec: version.MemorySpec,
			WorkflowSpec: version.WorkflowSpec, GuardrailSpec: version.GuardrailSpec, ModelSpec: modelSpec,
			OutputSchema: version.OutputSchema, TraceContract: version.TraceContract, PublicationSpec: version.PublicationSpec,
			Tools: version.Tools, KnowledgeSources: version.KnowledgeSources,
		}); err != nil {
			t.Fatal(err)
		}
		changed, err := repo.GetAgentBuildVersionByID(ctx, versionID)
		if err != nil {
			t.Fatal(err)
		}
		var got, want any
		if err := json.Unmarshal(changed.ModelSpec, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(modelSpec, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("canonical edit did not persist: got %s want %s", changed.ModelSpec, modelSpec)
		}
	}
	forged := vibe.Models{Assistant: "openai/gpt-4.1-mini", Target: "openai/gpt-4.1-mini", Evaluator: "openai/gpt-4.1-mini"}
	forgedSpec := vibeReliabilityJSON(t, map[string]any{"provider": "openrouter", "model": forged.Target, "vibe_models": forged})
	for _, tt := range []struct {
		name string
		spec json.RawMessage
	}{
		{"forged_valid_models", forgedSpec},
		{"malformed_non_object_receipt", json.RawMessage(`{"model":"openai/gpt-4.1-mini","vibe_models":"not an object"}`)},
		{"removed_receipt", json.RawMessage(`{"model":"openai/gpt-4.1-mini"}`)},
	} {
		t.Run("canonical_edit/"+tt.name, func(t *testing.T) {
			mutateCanonical(t, tt.spec)
			current, err := store.GetSession(ctx, actor, session.ID)
			if err != nil {
				t.Errorf("canonical edit broke session hydration: %v", err)
			} else if current.SavedModels == nil || *current.SavedModels != selected || current.SavedArtifactID == nil || *current.SavedArtifactID != artifact.ID || current.Document.Models != selected {
				t.Errorf("canonical edit rewrote save-time receipt: saved=%+v document=%+v", current.SavedModels, current.Document.Models)
			}
			payload["revision"], payload["models"] = session.Revision, selected
			if retry := post(payload); retry.Code != http.StatusOK || retry.Body.String() != response.Body.String() {
				t.Errorf("original choices stopped being idempotent: %d %s", retry.Code, retry.Body.String())
			}
			payload["models"] = forged
			if retry := post(payload); retry.Code != http.StatusConflict || !strings.Contains(retry.Body.String(), "saved_model_conflict") {
				t.Errorf("forged choices reused saved identity: %d %s", retry.Code, retry.Body.String())
			}
			delete(payload, "models")
			if retry := post(payload); retry.Code != http.StatusOK || retry.Body.String() != response.Body.String() {
				t.Errorf("implicit original choices stopped being idempotent: %d %s", retry.Code, retry.Body.String())
			}
			if err != nil {
				return // Hydration failed above; do not invalidate later fixtures.
			}
			// A subsequent normal conversation edit must remain possible.
			if err := store.Edit(ctx, actor, session.ID, session.Revision, func(v *vibe.Session) error {
				v.Document.Messages = append(v.Document.Messages, vibe.Message{ID: uuid.New(), Role: "user", Content: "Continue reviewing this draft.", CreatedAt: time.Now()})
				return nil
			}); err != nil {
				t.Fatalf("canonical edit made session unusable: %v", err)
			}
			session, err = store.GetSession(ctx, actor, session.ID)
			if err != nil {
				t.Fatalf("conversation edit could not be read back: %v", err)
			}
			payload["revision"] = session.Revision
		})
	}
	t.Run("legacy save remains readable without inventing model history", func(t *testing.T) {
		// Reinsert only the pre-receipt columns to model a genuine legacy
		// saved-artifact row. Mutable canonical/session fields are not history.
		if _, err := db.Exec(ctx, `WITH legacy AS (
			DELETE FROM vibe_saved_artifacts WHERE session_id=$1 AND artifact_id=$2 RETURNING *
		) INSERT INTO vibe_saved_artifacts(session_id,artifact_id,workspace_id,draft_id,build_id,build_version_id,created_at)
			SELECT session_id,artifact_id,workspace_id,draft_id,build_id,build_version_id,created_at FROM legacy`, session.ID, artifact.ID); err != nil {
			t.Fatal(err)
		}
		mutateCanonical(t, forgedSpec)
		legacy, err := store.GetSession(ctx, actor, session.ID)
		if err != nil || legacy.SavedArtifactID == nil || *legacy.SavedArtifactID != artifact.ID || legacy.SavedModels != nil {
			t.Errorf("legacy save invented model history: artifact=%v models=%+v err=%v", legacy.SavedArtifactID, legacy.SavedModels, err)
		}
		delete(payload, "models")
		if retry := post(payload); retry.Code != http.StatusOK || retry.Body.String() != response.Body.String() {
			t.Errorf("legacy retry failed: %d %s", retry.Code, retry.Body.String())
		}
		for _, models := range []vibe.Models{selected, forged} {
			payload["models"] = models
			if changed := post(payload); changed.Code != http.StatusConflict || !strings.Contains(changed.Body.String(), "saved_model_conflict") {
				t.Errorf("unrecorded saved model history treated as selected choices: %d %s", changed.Code, changed.Body.String())
			}
		}
	})
	t.Run("save rechecks revoked membership", func(t *testing.T) {
		if _, err := db.Exec(ctx, "DELETE FROM organization_memberships WHERE organization_id=$1 AND user_id=$2", org, user); err != nil {
			t.Fatal(err)
		}
		denied := post(payload)
		if denied.Code != http.StatusNotFound {
			t.Fatalf("revoked workspace membership saved: %d %s", denied.Code, denied.Body.String())
		}
	})
}
