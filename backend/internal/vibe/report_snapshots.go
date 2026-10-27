package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/agentclash/agentclash/backend/internal/vibe/access"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
)

// Selectors are private request data. Only the server-projected copy is retained.
type ReportEntrySelection struct {
	EntryID     string `json:"entry_id"`
	IncludeText bool   `json:"include_text"`
}
type ReportCaseSelection struct {
	EntryID string                 `json:"entry_id"`
	Entries []ReportEntrySelection `json:"entries"`
}
type ReportSelection struct {
	ClientID     uuid.UUID             `json:"client_id"`
	ArtifactID   uuid.UUID             `json:"artifact_id"`
	ResultRef    ResultRef             `json:"result_ref"`
	EvidenceHash string                `json:"evidence_hash"`
	PreviewHash  string                `json:"preview_hash,omitempty"`
	Cases        []ReportCaseSelection `json:"cases"`
}
type ReportFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type ReportEntry struct {
	ID           string       `json:"entry_id"`
	Kind         string       `json:"kind"`
	Label        string       `json:"label"`
	Facts        []ReportFact `json:"facts"`
	Text         []ReportFact `json:"text"`
	TextSelected bool         `json:"text_selected"`
}
type ReportCase struct {
	ID             string        `json:"entry_id"`
	Label          string        `json:"label"`
	Status         string        `json:"status"`
	Capture        CaptureScope  `json:"capture_scope"`
	Assurance      string        `json:"capture_assurance"`
	Entries        []ReportEntry `json:"entries"`
	OmittedEntries int           `json:"omitted_entries"`
}
type ReportContent struct {
	Fingerprints    *Fingerprints `json:"fingerprints,omitempty"`
	SourceResultRef *ResultRef    `json:"source_result_ref,omitempty"`
	ResultRef       ResultRef     `json:"result_ref"`
	ArtifactID      uuid.UUID     `json:"artifact_id"`
	SourceHash      string        `json:"source_hash"`
	EvidenceHash    string        `json:"evidence_hash"`
	Origin          ResultOrigin  `json:"origin"`
	Status          string        `json:"status"`
	Cases           []ReportCase  `json:"cases"`
	OmittedCases    int           `json:"omitted_cases"`
	Omissions       []string      `json:"omissions"`
}
type ReportSnapshot struct {
	ID          uuid.UUID     `json:"id"`
	WorkspaceID uuid.UUID     `json:"workspace_id"`
	CreatedAt   time.Time     `json:"created_at"`
	ContentHash string        `json:"content_hash"`
	Content     ReportContent `json:"content"`
}
type ReportNoteInput struct {
	ClientID uuid.UUID `json:"client_id"`
	TargetID string    `json:"target_id"`
	Text     string    `json:"text"`
}
type ReportNote struct {
	ID        uuid.UUID `json:"id"`
	AuthorID  uuid.UUID `json:"author_id"`
	TargetID  string    `json:"target_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func reportStatus(s string) string {
	switch s {
	case "PASS", "FAIL", "UNKNOWN", "passed", "failed", "skipped", "error", "ungraded", "graded", "unknown", "not_recorded", "imported", "queued", "running", "provisioning", "scoring", "aggregating", "completed", "cancelled", "canceled", "draft", "ready", "executing", "CREATED", "VALIDATING", "AWAITING_INPUT", "RESERVED", "AWAITING_APPROVAL", "QUEUED", "RUNNING", "FINALIZING", "CANCELLING", "COMPLETED", "PARTIAL", "FAILED", "CANCELLED", "EXPIRED":
		return s
	}
	return "unknown"
}
func reportGradeFacts(value json.RawMessage) []ReportFact {
	facts := []ReportFact{}
	var fields map[string]json.RawMessage
	if json.Unmarshal(value, &fields) != nil {
		return facts
	}
	for _, key := range []string{"passed", "verdict", "status", "score", "normalized_score", "correctness", "reliability_score", "latency_score", "cost_score", "overall_score"} {
		b := fields[key]
		if len(b) == 0 {
			continue
		}
		if key == "passed" {
			var v bool
			if (string(b) == "true" || string(b) == "false") && json.Unmarshal(b, &v) == nil {
				facts = append(facts, ReportFact{key, string(b)})
			}
			continue
		}
		if key == "verdict" || key == "status" {
			var v string
			if json.Unmarshal(b, &v) == nil {
				facts = append(facts, ReportFact{key, reportStatus(v)})
			}
			continue
		}
		var n json.Number
		if (b[0] == '-' || b[0] >= '0' && b[0] <= '9') && json.Unmarshal(b, &n) == nil {
			facts = append(facts, ReportFact{key, string(n)})
		}
	}
	var checks []json.RawMessage
	if json.Unmarshal(fields["checks"], &checks) == nil {
		for i, b := range checks {
			var c struct {
				Verdict string `json:"verdict"`
			}
			if json.Unmarshal(b, &c) == nil {
				facts = append(facts, ReportFact{fmt.Sprintf("Check %d", i+1), reportStatus(c.Verdict)})
			}
		}
	}
	return facts
}

// Only named text leaves are selectable. Never recurse arbitrary metadata,
// config/header maps or copy whole grade/tool/connector objects.
func reportEntryText(e ObservedEntry) []ReportFact {
	out := []ReportFact{}
	add := func(label string, b json.RawMessage) {
		var s string
		if json.Unmarshal(b, &s) == nil && s != "" {
			out = append(out, ReportFact{label, s})
		}
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(e.Value, &fields)
	switch e.Kind {
	case "output", "diagnostic":
		add("Text", e.Value)
	case "message":
		add("Message", fields["content"])
		add("Message", fields["text"])
	case "tool_execution", "tool_attempt", "tool_proposal":
		for _, k := range []string{"name", "arguments_json", "arguments", "result", "output", "error"} {
			add(k, fields[k])
		}
	case "retrieval":
		for _, k := range []string{"content", "source"} {
			add(k, fields[k])
		}
	case "grade":
		var grade func(map[string]json.RawMessage, string)
		grade = func(f map[string]json.RawMessage, prefix string) {
			for _, k := range []string{"name", "key", "judge_key", "reason", "evidence", "raw_output"} {
				add(prefix+k, f[k])
			}
			var issue map[string]json.RawMessage
			if json.Unmarshal(f["error"], &issue) == nil {
				add(prefix+"error", issue["message"])
			} else {
				add(prefix+"error", f["error"])
			}
			var finding struct {
				Missing string `json:"missing"`
				Quotes  []struct {
					Text string `json:"text"`
				} `json:"quotes"`
			}
			if json.Unmarshal(f["finding"], &finding) == nil {
				if finding.Missing != "" {
					out = append(out, ReportFact{prefix + "missing", finding.Missing})
				}
				for _, q := range finding.Quotes {
					out = append(out, ReportFact{prefix + "quote", q.Text})
				}
			}
		}
		grade(fields, "")
		var checks []map[string]json.RawMessage
		_ = json.Unmarshal(fields["checks"], &checks)
		for i, c := range checks {
			grade(c, fmt.Sprintf("Check %d ", i+1))
		}
	}
	return out
}
func projectReport(source ObservedEvidence, in ReportSelection) (ReportContent, error) {
	out := ReportContent{ResultRef: source.Binding.ResultRef, ArtifactID: source.Binding.ArtifactID, SourceHash: source.SourceHash, EvidenceHash: source.EvidenceHash, Origin: source.EvidenceMetadata.Origin, Status: reportStatus(source.Status), Cases: []ReportCase{}, Omissions: []string{"Only the selected cases, entries and scalar grade facts are copied. Other cases and entries are omitted.", "Source labels and text are excluded unless explicitly selected. Declared execution metadata, raw inputs, whole JSON objects, configuration and credentials are excluded.", "Capture describes the original source, not this selected subset. Excluded observations do not mean observed none.", "This is an independent workspace copy. Source deletion does not remove this report or revoke its public links."}}
	// SDK declared fingerprints are uploader strings, not verified platform hashes.
	if source.Binding.ResultRef.Kind != ResultSDKReport {
		fingerprints := source.EvidenceMetadata.Fingerprints
		out.Fingerprints = &fingerprints
		out.SourceResultRef = source.EvidenceMetadata.SourceResultRef
	}
	out.Omissions = append(out.Omissions, source.Omissions...)
	if len(in.Cases) == 0 || len(in.Cases) > 200 {
		return out, fault("invalid_request", "Select between one and 200 cases.")
	}
	seen := map[string]bool{}
	count := 0
	for i, sel := range in.Cases {
		if seen[sel.EntryID] {
			return out, fault("invalid_request", "Select each case once.")
		}
		seen[sel.EntryID] = true
		var c *ObservedCase
		for n := range source.Cases {
			if source.Cases[n].EntryID == sel.EntryID {
				c = &source.Cases[n]
				break
			}
		}
		if c == nil {
			return out, fault("evidence_changed", "A selected case is unavailable.")
		}
		row := ReportCase{ID: c.EntryID, Label: fmt.Sprintf("Case %d", i+1), Status: reportStatus(c.Status), Capture: c.CaptureScope, Assurance: c.CaptureAssurance, Entries: []ReportEntry{}}
		if c.SourceIndex < 0 {
			row.Label = fmt.Sprintf("Participant evidence %d", i+1)
		}
		entries := map[string]bool{}
		for j, es := range sel.Entries {
			count++
			if count > 1000 || entries[es.EntryID] {
				return out, fault("invalid_request", "Select at most 1,000 distinct entries.")
			}
			entries[es.EntryID] = true
			var e *ObservedEntry
			for n := range c.Entries {
				if c.Entries[n].EntryID == es.EntryID {
					e = &c.Entries[n]
					break
				}
			}
			if e == nil {
				return out, fault("evidence_changed", "A selected entry is unavailable.")
			}
			item := ReportEntry{ID: e.EntryID, Kind: e.Kind, Label: fmt.Sprintf("Entry %d", j+1), Facts: []ReportFact{}, Text: []ReportFact{}, TextSelected: es.IncludeText}
			if e.Kind == "grade" {
				item.Label = fmt.Sprintf("Grade %d", j+1)
				item.Facts = reportGradeFacts(e.Value)
			}
			if es.IncludeText {
				item.Text = reportEntryText(*e)
			}
			row.Entries = append(row.Entries, item)
		}
		row.OmittedEntries = len(c.Entries) - len(sel.Entries)
		out.Cases = append(out.Cases, row)
	}
	out.OmittedCases = len(source.Cases) - len(in.Cases)
	if len(raw(out)) > 1<<20 {
		return out, fault("document_limit", "Selected report exceeds 1 MiB. Select fewer entries.")
	}
	return out, nil
}
func (s *Store) PrepareReportSnapshot(ctx context.Context, actor string, session uuid.UUID, in ReportSelection, create bool) (ReportSnapshot, error) {
	var out ReportSnapshot
	if in.ClientID == uuid.Nil || in.ArtifactID == uuid.Nil || in.ResultRef.ID == uuid.Nil || len(in.EvidenceHash) != 64 {
		return out, fault("invalid_request", "Choose an exact source and preview its selected evidence.")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	accessSession, err := access.Lookup(ctx, tx, actor, session, true, true)
	if err != nil {
		return out, accessFault(err)
	}
	if accessSession.WorkspaceID == nil || accessSession.Anonymous {
		return out, fault("forbidden", "Save this project to a workspace before sharing a report.")
	}
	requestHash := Hash(raw(in))
	if create {
		var body []byte
		var prior string
		err = tx.QueryRow(ctx, "SELECT body,request_hash FROM report_snapshots WHERE workspace_id=$1 AND actor=$2 AND client_id=$3", accessSession.WorkspaceID, actor, in.ClientID).Scan(&body, &prior)
		if err == nil {
			if prior != requestHash {
				return out, fault("idempotency_conflict", "This report request already contains a different selection.")
			}
			err = json.Unmarshal(body, &out)
			if err == nil {
				err = tx.Commit(ctx)
			}
			return out, err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
	}
	v, err := scanSession(tx.QueryRow(ctx, sessionSelect, session))
	if err != nil {
		return out, err
	}
	source, err := observedEvidenceTx(ctx, tx, v, in.ResultRef, &in.ArtifactID)
	if err != nil {
		return out, err
	}
	if source.EvidenceHash != in.EvidenceHash {
		return out, fault("evidence_changed", "Source evidence changed. Refresh and preview the selection again.")
	}
	content, err := projectReport(source, in)
	if err != nil {
		return out, err
	}
	out = ReportSnapshot{WorkspaceID: *v.WorkspaceID, Content: content, ContentHash: Hash(raw(content))}
	if create {
		if in.PreviewHash == "" || in.PreviewHash != out.ContentHash {
			return out, fault("evidence_changed", "Preview this exact selection before sharing it.")
		}
		var count int
		var size int
		if err = tx.QueryRow(ctx, "SELECT count(*),COALESCE(sum(octet_length(body::text)),0) FROM report_snapshots WHERE workspace_id=$1", v.WorkspaceID).Scan(&count, &size); err != nil {
			return out, err
		}
		// Serializable predicate reads enforce workspace limits across source parents.
		if count >= 1000 || size+len(raw(out)) > 64<<20 {
			return out, fault("document_limit", "This workspace report allowance is full.")
		}
		out.ID = uuid.New()
		out.CreatedAt = timestamp()
		_, err = tx.Exec(ctx, `INSERT INTO report_snapshots(id,workspace_id,organization_id,actor,client_id,request_hash,content_hash,body,created_at) SELECT $1,w.id,w.organization_id,$3,$4,$5,$6,$7,$8 FROM workspaces w WHERE w.id=$2`, out.ID, out.WorkspaceID, actor, in.ClientID, requestHash, out.ContentHash, raw(out), out.CreatedAt)
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}
func (s *Store) ReportSnapshot(ctx context.Context, actor string, workspace, id uuid.UUID) (ReportSnapshot, error) {
	var out ReportSnapshot
	if err := access.Authorize(ctx, s.DB, actor, &workspace, false); err != nil {
		return out, accessFault(err)
	}
	var b []byte
	err := s.DB.QueryRow(ctx, "SELECT body FROM report_snapshots WHERE workspace_id=$1 AND id=$2", workspace, id).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, fault("not_found", "Report is unavailable.")
	}
	if err == nil {
		err = json.Unmarshal(b, &out)
	}
	return out, err
}
func (s *Store) ReportSnapshots(ctx context.Context, actor string, workspace uuid.UUID) ([]ReportSnapshot, error) {
	out := []ReportSnapshot{}
	if err := access.Authorize(ctx, s.DB, actor, &workspace, false); err != nil {
		return out, accessFault(err)
	}
	rows, err := s.DB.Query(ctx, "SELECT id,workspace_id,content_hash,created_at FROM report_snapshots WHERE workspace_id=$1 ORDER BY created_at DESC,id LIMIT 1000", workspace)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var r ReportSnapshot
		if err = rows.Scan(&r.ID, &r.WorkspaceID, &r.ContentHash, &r.CreatedAt); err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) ReportNotes(ctx context.Context, actor string, workspace, id uuid.UUID) ([]ReportNote, error) {
	out := []ReportNote{}
	if _, err := s.ReportSnapshot(ctx, actor, workspace, id); err != nil {
		return out, err
	}
	rows, err := s.DB.Query(ctx, "SELECT id,author_id,target_id,text,created_at FROM report_snapshot_notes WHERE snapshot_id=$1 ORDER BY created_at,id LIMIT 1000", id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var n ReportNote
		if err = rows.Scan(&n.ID, &n.AuthorID, &n.TargetID, &n.Text, &n.CreatedAt); err != nil {
			return out, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
func (s *Store) AddReportNote(ctx context.Context, actor string, workspace, id uuid.UUID, in ReportNoteInput) (ReportNote, error) {
	var out ReportNote
	if in.ClientID == uuid.Nil || strings.TrimSpace(in.Text) == "" || len(in.Text) > 4096 || !utf8.ValidString(in.Text) {
		return out, fault("invalid_request", "Write a note of at most 4,096 bytes.")
	}
	user, err := uuid.Parse(strings.TrimPrefix(actor, "user:"))
	if err != nil {
		return out, fault("forbidden", "Sign in to annotate.")
	}
	err = s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if e := access.Authorize(ctx, tx, actor, &workspace, true); e != nil {
			return accessFault(e)
		}
		var b []byte
		if e := tx.QueryRow(ctx, "SELECT body FROM report_snapshots WHERE workspace_id=$1 AND id=$2 FOR SHARE", workspace, id).Scan(&b); e != nil {
			return fault("not_found", "Report is unavailable.")
		}
		var report ReportSnapshot
		if e := json.Unmarshal(b, &report); e != nil {
			return e
		}
		found := in.TargetID == "report"
		for _, c := range report.Content.Cases {
			if c.ID == in.TargetID {
				found = true
			}
			for _, e := range c.Entries {
				if e.ID == in.TargetID {
					found = true
				}
			}
		}
		if !found {
			return fault("invalid_request", "Choose a target included in this report.")
		}
		if _, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,93))", id.String()); e != nil {
			return e
		}
		e := tx.QueryRow(ctx, "SELECT id,author_id,target_id,text,created_at FROM report_snapshot_notes WHERE snapshot_id=$1 AND author_id=$2 AND client_id=$3", id, user, in.ClientID).Scan(&out.ID, &out.AuthorID, &out.TargetID, &out.Text, &out.CreatedAt)
		if e == nil {
			if out.TargetID != in.TargetID || out.Text != in.Text {
				return fault("idempotency_conflict", "This note request already contains different text.")
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		var count int
		if e = tx.QueryRow(ctx, "SELECT count(*) FROM report_snapshot_notes WHERE snapshot_id=$1", id).Scan(&count); e != nil {
			return e
		}
		if count >= 1000 {
			return fault("document_limit", "This report has reached its note allowance.")
		}
		out = ReportNote{ID: uuid.New(), AuthorID: user, TargetID: in.TargetID, Text: in.Text, CreatedAt: timestamp()}
		_, e = tx.Exec(ctx, "INSERT INTO report_snapshot_notes(id,snapshot_id,author_id,client_id,target_id,text,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)", out.ID, id, user, in.ClientID, in.TargetID, in.Text, out.CreatedAt)
		return e
	})
	return out, err
}
