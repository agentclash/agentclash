package vibe

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestIntegrationVibeKeepBriefAndPermissions(t *testing.T) {
	s := integrationStore(t)
	v := approvalRegressionWorkspace(t, s)
	ctx := context.Background()
	a := Artifact{ID: uuid.New(), Kind: "test_plan", Title: "PDF to Markdown", TestPlan: &TestPlan{Objective: "Preserve headings and tables", EvidenceNeeded: []string{"Original PDF and converted Markdown"}}}
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document.Artifacts = append(v.Document.Artifacts, a); return nil }); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	saved, err := s.SaveBrief(ctx, v.Actor, v.ID, v.Revision, *v.WorkspaceID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Exec(ctx, "DELETE FROM vibe_saved_briefs WHERE session_id=$1", v.ID) })
	after, _ := s.GetSession(ctx, v.Actor, v.ID)
	again, err := s.SaveBrief(ctx, v.Actor, v.ID, v.Revision, *v.WorkspaceID, a.ID)
	if err != nil || again.ID != saved.ID {
		t.Fatal("lost acknowledgment did not recover exact brief", err)
	}
	latest, _ := s.GetSession(ctx, v.Actor, v.ID)
	if latest.Revision != after.Revision || latest.EventCursor != after.EventCursor || len(latest.Operations) != 0 {
		t.Fatal("brief retry changed state or ran AI")
	}
	list, err := s.ListChecks(ctx, v.Actor, *v.WorkspaceID)
	if err != nil || len(list) != 1 || list[0].Kind != "brief" || list[0].ArtifactID != a.ID || list[0].DraftID != nil {
		t.Fatal("brief reopen lost identity", err, list)
	}
	var stored Artifact
	var b []byte
	if err = s.DB.QueryRow(ctx, "SELECT artifact FROM vibe_saved_briefs WHERE id=$1", saved.ID).Scan(&b); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &stored); err != nil || stored.TestPlan.Objective != a.TestPlan.Objective {
		t.Fatal("brief snapshot changed", err)
	}
	other := approvalRegressionWorkspace(t, s)
	if _, err = s.SaveBrief(ctx, other.Actor, v.ID, v.Revision, *v.WorkspaceID, a.ID); err == nil {
		t.Fatal("another actor saved private brief")
	}
	if _, err = s.SaveBrief(ctx, v.Actor, v.ID, latest.Revision, *v.WorkspaceID, uuid.New()); err == nil {
		t.Fatal("unknown brief accepted")
	}
	if _, err = s.DB.Exec(ctx, "UPDATE organization_memberships SET membership_status='suspended' WHERE user_id=$1", v.Actor[5:]); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveBrief(ctx, v.Actor, v.ID, v.Revision, *v.WorkspaceID, a.ID); err == nil {
		t.Fatal("receipt bypassed revoked workspace membership")
	}
	list, err = s.ListChecks(ctx, v.Actor, uuid.Nil)
	if err != nil || len(list) != 0 {
		t.Fatal("revoked workspace leaked saved brief", err)
	}
}

func TestIntegrationVibeKeepExactPackAndRun(t *testing.T) {
	s := integrationStore(t)
	v := approvalRegressionWorkspace(t, s)
	ctx := context.Background()
	a := Artifact{ID: uuid.New(), Kind: "test_suite", Title: "Imported identity", Blueprint: json.RawMessage(`{"cases":[{"key":"original-case"}],"judges":[{"key":"original-judge"}]}`)}
	if err := s.Edit(ctx, v.Actor, v.ID, v.Revision, func(v *Session) error { v.Document.Artifacts = append(v.Document.Artifacts, a); return nil }); err != nil {
		t.Fatal(err)
	}
	v, _ = s.GetSession(ctx, v.Actor, v.ID)
	run := func(artifact Artifact) uuid.UUID {
		t.Helper()
		id := uuid.New()
		_, err := s.DB.Exec(ctx, `INSERT INTO vibe_operations(id,session_id,actor,client_id,request_hash,kind,state,billing,models,input,max_cost,deadline) VALUES($1,$2,$3,$4,'fixture','check','COMPLETED','SETTLED',$5,$6,0,now())`, id, v.ID, v.Actor, uuid.New(), raw(DefaultModels()), raw(Plan{Artifact: &artifact}))
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	baseline := run(a)
	wrong := uuid.New()
	if _, err := s.saveDraft(ctx, v.Actor, v.ID, v.Revision, *v.WorkspaceID, a, a.Blueprint, DefaultModels(), true, &wrong, true); err == nil {
		t.Fatal("unknown baseline accepted")
	}
	draft, err := s.saveDraft(ctx, v.Actor, v.ID, v.Revision, *v.WorkspaceID, a, a.Blueprint, DefaultModels(), true, &baseline, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Exec(ctx, "DELETE FROM vibe_saved_artifacts WHERE session_id=$1", v.ID) })
	before, _ := s.GetSession(ctx, v.Actor, v.ID)
	again, err := s.saveDraft(ctx, v.Actor, v.ID, v.Revision, *v.WorkspaceID, a, a.Blueprint, DefaultModels(), true, &baseline, true)
	if err != nil || again != draft {
		t.Fatal("duplicate save failed", err)
	}
	latest, _ := s.GetSession(ctx, v.Actor, v.ID)
	if before.Revision != latest.Revision || before.EventCursor != latest.EventCursor {
		t.Fatal("receipt retry wrote another save event")
	}
	newer := run(a)
	list, err := s.ListChecks(ctx, v.Actor, *v.WorkspaceID)
	if err != nil || len(list) != 1 || list[0].BaselineID != baseline || list[0].ArtifactID != a.ID || *list[0].DraftID != draft {
		t.Fatal("new run moved frozen saved link", err, list)
	}
	if _, err = s.saveDraft(ctx, v.Actor, v.ID, latest.Revision, *v.WorkspaceID, a, a.Blueprint, DefaultModels(), true, &newer, true); err == nil {
		t.Fatal("different baseline silently overwrote saved run")
	}
	changed := DefaultModels()
	changed.Target = "different-model"
	if _, err = s.SavedDraftReceipt(ctx, v.Actor, v.ID, *v.WorkspaceID, a.ID, changed, true, &baseline); err == nil {
		t.Fatal("different models reused receipt")
	}
	var composition []byte
	if err = s.DB.QueryRow(ctx, "SELECT composition FROM challenge_pack_drafts WHERE id=$1", draft).Scan(&composition); err != nil {
		t.Fatal(err)
	}
	want, _ := CanonicalJSONHash(a.Blueprint)
	got, _ := CanonicalJSONHash(composition)
	if want != got {
		t.Fatal("saved pack changed case or grading identities")
	}
}
