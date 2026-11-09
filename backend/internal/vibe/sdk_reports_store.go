package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe/access"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Retained SDK receipts and original definitions share the existing per-project
// stored-file allowance. Session locking serializes both kinds of admission.
func retainedSourceBytes(ctx context.Context, tx pgx.Tx, session uuid.UUID) (int, error) {
	var used int
	err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT sum(octet_length(content)) FROM vibe_import_originals WHERE session_id=$1),0)+COALESCE((SELECT sum(storage_bytes) FROM vibe_sdk_reports WHERE session_id=$1),0)+COALESCE((SELECT sum(octet_length(body::text)+octet_length(request_hash)) FROM vibe_grade_reviews WHERE session_id=$1 AND result_kind='sdk_report'),0)+COALESCE((SELECT sum(octet_length(artifact::text)+octet_length(request_hash)+octet_length(source_evidence_hash)+octet_length(source_case_id)) FROM vibe_sdk_promotions WHERE session_id=$1),0)`, session).Scan(&used)
	return used, err
}
func sdkArtifact(v Session, id uuid.UUID) (Artifact, error) {
	for _, a := range v.Document.Artifacts {
		if a.ID == id {
			return a, nil
		}
	}
	return Artifact{}, fault("not_found", "The report's test version is unavailable in this project.")
}
func sdkAccess(ctx context.Context, tx pgx.Tx, actor string, session uuid.UUID, write bool) (Session, error) {
	if _, err := access.Lookup(ctx, tx, actor, session, write, write); err != nil {
		return Session{}, accessFault(err)
	}
	v, err := scanSession(tx.QueryRow(ctx, sessionSelect, session))
	if err != nil {
		return v, err
	}
	if strings.HasPrefix(actor, "anon:") || v.WorkspaceID == nil {
		return v, fault("forbidden", "Sign in and save this project to a workspace before importing SDK evidence.")
	}
	return v, nil
}

// Mapping reads use retained receipts, never the editable draft's lifetime.
func resolveSDKBindings(ctx context.Context, tx pgx.Tx, v Session, a Artifact, in *SDKReportImport) error {
	rows, err := tx.Query(ctx, `SELECT case_keys FROM vibe_pack_sources WHERE session_id=$1 AND artifact_id=$2 AND workspace_id=$3`, v.ID, a.ID, v.WorkspaceID)
	if err != nil {
		return err
	}
	tuples := map[CaseMapping]bool{}
	saved := false
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			break
		}
		var cases []CaseMapping
		if err = json.Unmarshal(b, &cases); err != nil {
			break
		}
		saved = true
		for _, c := range cases {
			tuples[c] = true
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	if !saved {
		for _, c := range declaredCaseMapping(a) {
			tuples[c] = true
		}
	}
	seen := map[CaseMapping]bool{}
	for i := range in.Metadata.Cases {
		c := &in.Metadata.Cases[i]
		matches := []CaseMapping{}
		for tuple := range tuples {
			if tuple.CaseKey == c.CaseKey && (c.InputSetKey == nil || *c.InputSetKey == tuple.InputSetKey) && (c.ChallengeKey == nil || *c.ChallengeKey == tuple.ChallengeKey) {
				matches = append(matches, tuple)
			}
		}
		if len(matches) != 1 {
			return fault("invalid_import", "Each source case must resolve to one exact test case. Supply input-set and challenge keys for ambiguous cases.")
		}
		tuple := matches[0]
		if seen[tuple] {
			return fault("invalid_import", "Two source cases cannot bind to the same test case.")
		}
		seen[tuple] = true
		c.InputSetKey = &tuple.InputSetKey
		c.ChallengeKey = &tuple.ChallengeKey
		if c.PublishedCaseID != nil && in.Metadata.ChallengePackVersionID == nil {
			return fault("invalid_import", "A published case requires its exact published version.")
		}
		if in.Metadata.ChallengePackVersionID != nil {
			var ids []uuid.UUID
			r, e := tx.Query(ctx, `SELECT DISTINCT b.published_case_id FROM vibe_published_case_bindings b JOIN vibe_pack_sources p ON p.draft_id=b.draft_id WHERE p.session_id=$1 AND p.artifact_id=$2 AND p.workspace_id=$3 AND b.artifact_id=$2 AND b.challenge_pack_version_id=$4 AND b.case_key=$5 AND b.input_set_key=$6 AND b.challenge_key=$7`, v.ID, a.ID, v.WorkspaceID, in.Metadata.ChallengePackVersionID, tuple.CaseKey, tuple.InputSetKey, tuple.ChallengeKey)
			if e != nil {
				return e
			}
			for r.Next() {
				var id uuid.UUID
				if e = r.Scan(&id); e != nil {
					break
				}
				ids = append(ids, id)
			}
			if e == nil {
				e = r.Err()
			}
			r.Close()
			if e != nil {
				return e
			}
			if len(ids) != 1 || c.PublishedCaseID != nil && *c.PublishedCaseID != ids[0] {
				return fault("invalid_import", "The published case does not belong to this exact project version and tuple.")
			}
			c.PublishedCaseID = &ids[0]
		}
	}
	// A zero-case diagnostic still cannot claim a foreign published version.
	if len(in.Metadata.Cases) == 0 && in.Metadata.ChallengePackVersionID != nil {
		var ok bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_published_case_bindings b JOIN vibe_pack_sources p ON p.draft_id=b.draft_id WHERE p.session_id=$1 AND p.artifact_id=$2 AND p.workspace_id=$3 AND b.artifact_id=$2 AND b.challenge_pack_version_id=$4)`, v.ID, a.ID, v.WorkspaceID, in.Metadata.ChallengePackVersionID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return invalidSDK()
		}
	}
	return nil
}
func (s *Store) ImportSDKReport(ctx context.Context, actor string, session uuid.UUID, b []byte) (SDKReport, error) {
	in, err := DecodeSDKReport(b)
	if err != nil {
		return SDKReport{}, err
	}
	sort.Slice(in.Metadata.Cases, func(i, j int) bool { return in.Metadata.Cases[i].SourceCaseID < in.Metadata.Cases[j].SourceCaseID })
	requestHash := Hash(raw(in))
	if in.ExternalReportID == "" {
		in.ExternalReportID = "content:" + requestHash
	}
	var out SDKReport
	err = s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, e := sdkAccess(ctx, tx, actor, session, true)
		if e != nil {
			return e
		}
		a, e := sdkArtifact(v, in.Metadata.ArtifactID)
		if e != nil {
			return e
		}
		var previousHash string
		var body []byte
		e = tx.QueryRow(ctx, `SELECT request_hash,body FROM vibe_sdk_reports WHERE session_id=$1 AND external_report_id=$2`, session, in.ExternalReportID).Scan(&previousHash, &body)
		if e == nil {
			if previousHash != requestHash {
				return fault("idempotency_conflict", "This report identity already has different original bytes or metadata.")
			}
			return json.Unmarshal(body, &out)
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if e = resolveSDKBindings(ctx, tx, v, a, &in); e != nil {
			return e
		}
		id := uuid.New()
		ref := ResultRef{Kind: ResultSDKReport, ID: id}
		out = SDKReport{ID: id, Binding: ProjectResultBinding{SessionID: session, ArtifactID: a.ID, ResultRef: ref, ChallengePackVersionID: in.Metadata.ChallengePackVersionID}, EvidenceMetadata: EvidenceMetadata{ResultRef: ref, EvidenceProvenance: EvidenceProvenance{Origin: OriginImported, Fingerprints: in.Metadata.Fingerprints, CaptureScope: in.Metadata.CaptureScope, ApplicationRevision: in.Metadata.ApplicationRevision}}, SourceKind: in.SourceKind, SourceSchemaVersion: 1, ExternalReportID: in.ExternalReportID, ContentHash: Hash([]byte(in.SourceJSON)), Size: len(in.SourceJSON), CreatedAt: timestamp(), Cases: in.Metadata.Cases}
		if in.SourceKind == "eval_report" {
			var doc map[string]json.RawMessage
			_ = json.Unmarshal([]byte(in.SourceJSON), &doc)
			out.Summary = doc["summary"]
		} else {
			out.Diagnostics = []string{"This agent result has no imported grade."}
		}
		body = raw(out)
		projection, e := retainedJSONBytes(ctx, tx, body)
		if e != nil {
			return e
		}
		size := len(in.SourceJSON) + projection + len(in.ExternalReportID) + len(requestHash) + len(out.ContentHash)
		var count int
		if e = tx.QueryRow(ctx, `SELECT count(*) FROM vibe_sdk_reports WHERE session_id=$1`, session).Scan(&count); e != nil {
			return e
		}
		used, e := retainedSourceBytes(ctx, tx, session)
		if e != nil {
			return e
		}
		if count >= SDKReportCount || used+size > LimitsFor(false).StoredBytes {
			return fault("attachment_limit", "This project's retained report or file allowance is full. Export evidence before starting another project.")
		}
		_, e = tx.Exec(ctx, `INSERT INTO vibe_sdk_reports(id,session_id,actor,workspace_id,artifact_id,external_report_id,request_hash,source_json,content_sha256,body,storage_bytes,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, session, actor, v.WorkspaceID, a.ID, in.ExternalReportID, requestHash, []byte(in.SourceJSON), out.ContentHash, body, size, out.CreatedAt)
		if e != nil {
			return e
		}
		return event(ctx, tx, session, nil, "report.imported")
	})
	return out, err
}
func sdkReportTx(ctx context.Context, tx pgx.Tx, session, id uuid.UUID) (SDKReport, error) {
	var r SDKReport
	var b, source []byte
	err := tx.QueryRow(ctx, `SELECT body,source_json FROM vibe_sdk_reports WHERE session_id=$1 AND id=$2`, session, id).Scan(&b, &source)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, fault("not_found", "SDK report is unavailable in this project.")
	}
	if err == nil {
		err = json.Unmarshal(b, &r)
		r.SourceJSON = string(source)
	}
	return r, err
}
func (s *Store) SDKReport(ctx context.Context, actor string, session, id uuid.UUID) (SDKReport, error) {
	var out SDKReport
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, e := sdkAccess(ctx, tx, actor, session, false); e != nil {
			return e
		}
		var e error
		out, e = sdkReportTx(ctx, tx, session, id)
		return e
	})
	return out, err
}
func (s *Store) SDKReports(ctx context.Context, actor string, session uuid.UUID) ([]SDKReport, error) {
	out := []SDKReport{}
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, e := sdkAccess(ctx, tx, actor, session, false); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, `SELECT body FROM vibe_sdk_reports WHERE session_id=$1 ORDER BY created_at,id LIMIT $2`, session, SDKReportCount+1)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var b []byte
			if e = rows.Scan(&b); e != nil {
				return e
			}
			var r SDKReport
			if e = json.Unmarshal(b, &r); e != nil {
				return e
			}
			out = append(out, r)
		}
		if len(out) > SDKReportCount {
			return fault("export_limit", "Report collection exceeds its bounded allowance.")
		}
		return rows.Err()
	})
	return out, err
}

// Count the stored JSONB text projection, including PostgreSQL's serialization,
// rather than the compact wire representation. Every caller holds the session.
func retainedJSONBytes(ctx context.Context, tx pgx.Tx, body []byte) (int, error) {
	var size int
	err := tx.QueryRow(ctx, "SELECT octet_length($1::jsonb::text)", body).Scan(&size)
	return size, err
}
func admitSDKProjection(ctx context.Context, tx pgx.Tx, session uuid.UUID, body []byte, extra int) error {
	size, err := retainedJSONBytes(ctx, tx, body)
	if err != nil {
		return err
	}
	used, err := retainedSourceBytes(ctx, tx, session)
	if err != nil {
		return err
	}
	if used+size+extra > LimitsFor(false).StoredBytes {
		return fault("attachment_limit", "This project's retained report and evidence allowance is full.")
	}
	return nil
}
