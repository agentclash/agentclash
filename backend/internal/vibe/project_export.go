package vibe

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
)

type ExportRequest struct {
	Scope      string     `json:"scope"`
	ArtifactID uuid.UUID  `json:"artifact_id"`
	ResultID   *uuid.UUID `json:"result_id,omitempty"`
}
type ExportItem struct {
	Kind            string     `json:"kind"`
	ID              string     `json:"id"`
	Hash            string     `json:"hash,omitempty"`
	OperationID     *uuid.UUID `json:"operation_id,omitempty"`
	ArtifactID      *uuid.UUID `json:"artifact_id,omitempty"`
	CaseKey         string     `json:"case_key,omitempty"`
	Missing         string     `json:"missing,omitempty"`
	EvidenceChanged *bool      `json:"evidence_changed,omitempty"`
}
type ExportPreparation struct {
	Format       string            `json:"format"`
	SessionID    uuid.UUID         `json:"session_id"`
	Title        string            `json:"title"`
	CapturedAt   time.Time         `json:"captured_at"`
	Selection    ExportRequest     `json:"selection"`
	ArtifactID   uuid.UUID         `json:"artifact_id"`
	Artifacts    []Artifact        `json:"artifacts"`
	Rules        []PolicySnapshot  `json:"rules"`
	Runs         []Operation       `json:"runs"`
	Items        []ExportItem      `json:"items"`
	CaseMappings []json.RawMessage `json:"case_mappings"`
	Omissions    []string          `json:"omissions"`
}

// Preparation freezes source summaries, expected IDs and hashes in one snapshot.
// It allocates no job, charges no credits and never visits unrelated runs by default.
func (s *Store) PrepareExport(ctx context.Context, actor string, id uuid.UUID, in ExportRequest) (ExportPreparation, error) {
	out := ExportPreparation{Format: "agentclash-evaluation-v2", SessionID: id, ArtifactID: in.ArtifactID, Selection: in, Artifacts: []Artifact{}, Rules: []PolicySnapshot{}, Runs: []Operation{}, Items: []ExportItem{}, CaseMappings: []json.RawMessage{}, Omissions: []string{"Guide and trial conversations, source evidence sets, unsent local drafts and input material file bodies are omitted. This archive contains the chosen versions, rules, results, reviews and uploaded originals."}}
	if in.Scope != "selected" && in.Scope != "history" || in.ArtifactID == uuid.Nil {
		return out, fault("invalid_request", "Choose an exact version and export scope.")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	v, err := scanSession(tx.QueryRow(ctx, sessionSelect, id))
	if err != nil {
		return out, err
	}
	if v.Actor != actor {
		return out, fault("not_found", "Project is unavailable.")
	}
	if err = authorize(ctx, tx, actor, v.WorkspaceID, false); err != nil {
		return out, err
	}
	if in.Scope == "selected" {
		out.Omissions = append(out.Omissions, "Other versions and unselected runs and their grade reviews are omitted.")
	}
	out.Title = v.Title
	out.CapturedAt = timestamp()
	found := false
	selected := map[uuid.UUID]bool{}
	for _, a := range v.Document.Artifacts {
		if a.ID == in.ArtifactID {
			found = true
		}
		if a.ID == in.ArtifactID || in.Scope == "history" {
			out.Artifacts = append(out.Artifacts, a)
			selected[a.ID] = true
			if a.Original != nil {
				id := a.ID
				out.Items = append(out.Items, ExportItem{Kind: "original", ID: a.Original.ID.String(), ArtifactID: &id, Hash: a.Original.Hash})
			}
		}
	}
	if !found {
		return out, fault("not_found", "The chosen version is unavailable.")
	}
	for _, p := range v.Document.Policies {
		if in.Scope == "history" {
			out.Rules = append(out.Rules, p)
			continue
		}
		for _, a := range out.Artifacts {
			if a.PolicyID != nil && *a.PolicyID == p.ID {
				out.Rules = append(out.Rules, p)
				break
			}
		}
	}
	rows, err := tx.Query(ctx, operationSummarySelect+" WHERE session_id=$1 AND ($2 OR id=$3) AND kind IN ('check','retest') ORDER BY created_at,id LIMIT 5001", id, in.Scope == "history", in.ResultID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		o, e := scanOperation(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		version := uuid.Nil
		if o.ResultScope != nil {
			version = o.ResultScope.ArtifactID
		} else if o.Source != nil {
			version = o.Source.ArtifactID
		}
		if version != uuid.Nil && !selected[version] {
			rows.Close()
			return out, fault("invalid_request", "The chosen result does not belong to the chosen version.")
		}
		out.Runs = append(out.Runs, o)
		if len(out.Runs) > 5000 {
			rows.Close()
			return out, fault("export_limit", "This history exceeds one export. Export individual versions instead.")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	resultFound := in.ResultID == nil
	for _, operation := range out.Runs {
		if in.ResultID != nil && operation.ID == *in.ResultID {
			resultFound = true
		}
	}
	if !resultFound {
		return out, fault("not_found", "The chosen result is unavailable.")
	}
	for i := range out.Runs {
		o := &out.Runs[i]
		cases, err := tx.Query(ctx, "SELECT result FROM vibe_case_results WHERE operation_id=$1 ORDER BY case_key", o.ID)
		if err != nil {
			return out, err
		}
		byKey := map[string]CaseResult{}
		for cases.Next() {
			var b []byte
			if err = cases.Scan(&b); err != nil {
				break
			}
			var c CaseResult
			if err = json.Unmarshal(b, &c); err != nil {
				break
			}
			c.EvidenceHash = ""
			byKey[c.CaseKey] = c
		}
		if err == nil {
			err = cases.Err()
		}
		cases.Close()
		if err != nil {
			return out, err
		}
		o.Results = []CaseResult{}
		opID := o.ID
		artifactID := in.ArtifactID
		var keys []string
		if o.ResultScope != nil {
			artifactID = o.ResultScope.ArtifactID
			keys = o.ResultScope.CaseKeys
		} else {
			if o.Source != nil && o.Source.ArtifactID != uuid.Nil {
				artifactID = o.Source.ArtifactID
			}
			for key := range byKey {
				keys = append(keys, key)
			}
			sort.Strings(keys)
		}
		if o.ResultScope == nil || len(o.ResultScope.CaseKeys) == 0 {
			out.Items = append(out.Items, ExportItem{Kind: "scope", ID: o.ID.String(), OperationID: &opID, Missing: "The original admitted case scope is unavailable."})
		}
		for _, key := range keys {
			item := ExportItem{Kind: "case", ID: o.ID.String() + ":" + key, OperationID: &opID, ArtifactID: &artifactID, CaseKey: key}
			c, ok := byKey[key]
			if in.Scope == "history" && o.ResultScope == nil {
				if version, e := uuid.Parse(c.Version); e == nil && selected[version] {
					item.ArtifactID = &version
				}
			}
			if !ok {
				item.Missing = "Saved case body is unavailable."
			} else if c.Version != item.ArtifactID.String() {
				item.Missing = "Saved case version differs from the admitted version."
			} else {
				item.Hash = Hash(raw(c))
				summary := c
				summary.Input = nil
				summary.Output = ""
				summary.Messages = nil
				summary.Expected = ""
				summary.Expectations = nil
				summary.Checks = append([]CheckResult(nil), c.Checks...)
				for j := range summary.Checks {
					summary.Checks[j].Evidence = ""
					summary.Checks[j].Finding = nil
					summary.Checks[j].MessageIDs = nil
				}
				o.Results = append(o.Results, summary)
			}
			out.Items = append(out.Items, item)
			if len(out.Items) > 5000 {
				return out, fault("export_limit", "This history exceeds one export. Export individual versions instead.")
			}
		}
		score := Aggregate(o.Results)
		o.Scorecard = &score
		reviews, e := tx.Query(ctx, "SELECT id,body FROM vibe_grade_reviews WHERE session_id=$1 AND result_kind='vibe_operation' AND result_id=$2 ORDER BY created_at,id LIMIT 5001", id, o.ID)
		if e != nil {
			return out, e
		}
		for reviews.Next() {
			var rid uuid.UUID
			var b []byte
			if e = reviews.Scan(&rid, &b); e != nil {
				break
			}
			var r GradeReview
			if e = json.Unmarshal(b, &r); e != nil {
				break
			}
			c, ok := byKey[r.CaseKey]
			changed := !ok || Hash(raw(c)) != r.EvidenceHash
			reviewArtifact := r.ArtifactID
			out.Items = append(out.Items, ExportItem{Kind: "grade_review", ID: rid.String(), OperationID: &opID, ArtifactID: &reviewArtifact, Hash: Hash(raw(r)), EvidenceChanged: &changed})
			if len(out.Items) > 5000 {
				e = fault("export_limit", "This history exceeds one export. Export individual versions instead.")
				break
			}
		}
		if e == nil {
			e = reviews.Err()
		}
		reviews.Close()
		if e != nil {
			return out, e
		}
	}
	validation, e := tx.Query(ctx, "SELECT id,body FROM vibe_validation_reviews WHERE session_id=$1 AND ($2 OR (body->>'candidate_artifact_id')=$3 OR (body->>'prior_artifact_id')=$3) ORDER BY created_at,id LIMIT 5001", id, in.Scope == "history", in.ArtifactID.String())
	if e != nil {
		return out, e
	}
	for validation.Next() {
		var rid uuid.UUID
		var b []byte
		if e = validation.Scan(&rid, &b); e != nil {
			break
		}
		var r ValidationReview
		if e = json.Unmarshal(b, &r); e != nil {
			break
		}
		out.Items = append(out.Items, ExportItem{Kind: "validation_review", ID: rid.String(), Hash: Hash(raw(r))})
		if len(out.Items) > 5000 {
			e = fault("export_limit", "This history exceeds one export. Export individual versions instead.")
			break
		}
	}
	if e == nil {
		e = validation.Err()
	}
	validation.Close()
	if e != nil {
		return out, e
	}
	mapping, e := tx.Query(ctx, `SELECT jsonb_build_object('artifact_id',p.artifact_id,'draft_id',p.draft_id,'composition_hash',p.composition_hash,'cases',p.case_keys,'published',COALESCE((SELECT jsonb_agg(to_jsonb(b)) FROM vibe_published_case_bindings b WHERE b.draft_id=p.draft_id),'[]'::jsonb)) FROM vibe_pack_sources p WHERE p.session_id=$1 AND ($2 OR p.artifact_id=$3)`, id, in.Scope == "history", in.ArtifactID)
	if e != nil {
		return out, e
	}
	for mapping.Next() {
		var b []byte
		if e = mapping.Scan(&b); e != nil {
			break
		}
		out.CaseMappings = append(out.CaseMappings, json.RawMessage(b))
	}
	if e == nil {
		e = mapping.Err()
	}
	mapping.Close()
	if e != nil {
		return out, e
	}
	for _, a := range out.Artifacts {
		known := false
		for _, m := range out.CaseMappings {
			var row struct {
				ArtifactID uuid.UUID `json:"artifact_id"`
			}
			_ = json.Unmarshal(m, &row)
			if row.ArtifactID == a.ID {
				known = true
			}
		}
		if !known {
			out.CaseMappings = append(out.CaseMappings, raw(map[string]any{"artifact_id": a.ID, "cases": declaredCaseMapping(a), "published": []any{}, "status": "unresolved", "reason": "No immutable publication binding is known for this version."}))
		}
	}
	if len(out.Items) > 5000 {
		return out, fault("export_limit", "This history exceeds one export. Export individual versions instead.")
	}
	return out, tx.Commit(ctx)
}
func (s *Store) ExportReview(ctx context.Context, actor string, session, id uuid.UUID, kind, expected string) (json.RawMessage, error) {
	var data json.RawMessage
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, e := scanSession(tx.QueryRow(ctx, sessionSelect, session))
		if e != nil {
			return e
		}
		if v.Actor != actor {
			return fault("not_found", "Project is unavailable.")
		}
		if e = authorize(ctx, tx, actor, v.WorkspaceID, false); e != nil {
			return e
		}
		table := "vibe_grade_reviews"
		if kind == "validation_review" {
			table = "vibe_validation_reviews"
		} else if kind != "grade_review" {
			return fault("invalid_request", "Choose an exported review.")
		}
		var b []byte
		if e = tx.QueryRow(ctx, fmt.Sprintf("SELECT body FROM %s WHERE session_id=$1 AND id=$2", table), session, id).Scan(&b); e == pgx.ErrNoRows {
			return fault("not_found", "Review is unavailable.")
		} else if e != nil {
			return e
		}
		if kind == "grade_review" {
			var r GradeReview
			if e = json.Unmarshal(b, &r); e != nil {
				return e
			}
			data = raw(r)
		} else {
			var r ValidationReview
			if e = json.Unmarshal(b, &r); e != nil {
				return e
			}
			data = raw(r)
		}
		if expected == "" || Hash(data) != expected {
			return fault("evidence_changed", "The selected review changed. Prepare the export again.")
		}
		return nil
	})
	return data, err
}

func declaredCaseMapping(a Artifact) []CaseMapping {
	out := []CaseMapping{}
	var shape struct {
		Cases []struct {
			Key          string `json:"key"`
			CaseKey      string `json:"case_key"`
			ChallengeKey string `json:"challenge_key"`
		} `json:"cases"`
		InputSets []challengepack.InputSetDefinition `json:"input_sets"`
	}
	if json.Unmarshal(a.Blueprint, &shape) != nil {
		return out
	}
	for _, c := range shape.Cases {
		key := c.CaseKey
		if key == "" {
			key = c.Key
		}
		if key != "" {
			out = append(out, CaseMapping{CaseKey: key, ChallengeKey: c.ChallengeKey})
		}
	}
	for _, set := range shape.InputSets {
		for _, c := range set.Cases {
			out = append(out, CaseMapping{CaseKey: c.EffectiveKey(), InputSetKey: set.Key, ChallengeKey: c.ChallengeKey})
		}
	}
	return out
}
