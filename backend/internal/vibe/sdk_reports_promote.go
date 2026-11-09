package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
)

type SDKPromotion struct {
	ClientID           uuid.UUID `json:"client_id"`
	Revision           int64     `json:"revision"`
	SourceCaseID       string    `json:"source_case_id"`
	SourceEvidenceHash string    `json:"source_evidence_hash"`
	Expected           string    `json:"expected"`
	Reviewed           bool      `json:"reviewed"`
}

func (s *Service) PromoteSDKExpectation(ctx context.Context, actor string, session, reportID uuid.UUID, in SDKPromotion) (Artifact, error) {
	var out Artifact
	if !in.Reviewed || in.ClientID == uuid.Nil || in.Revision < 0 || in.SourceCaseID == "" || len(in.SourceCaseID) > 256 || strings.TrimSpace(in.Expected) == "" || len(in.Expected) > 4096 {
		return out, fault("invalid_request", "Review the exact imported expectation before creating a new test version.")
	}
	hash := Hash(raw(struct {
		Report  uuid.UUID
		Request SDKPromotion
	}{reportID, in}))
	err := s.Store.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, e := sdkAccess(ctx, tx, actor, session, true)
		if e != nil {
			return e
		}
		var oldHash string
		var body []byte
		e = tx.QueryRow(ctx, `SELECT request_hash,artifact FROM vibe_sdk_promotions WHERE session_id=$1 AND client_id=$2`, session, in.ClientID).Scan(&oldHash, &body)
		if e == nil {
			if oldHash != hash {
				return fault("idempotency_conflict", "This review action already has different content.")
			}
			return json.Unmarshal(body, &out)
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if v.Revision != in.Revision {
			return fault("revision_conflict", "Reload before promoting this expectation.")
		}
		var busy bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_operations WHERE session_id=$1 AND state IN ('AWAITING_APPROVAL','QUEUED','RUNNING','CANCELLING','FINALIZING'))`, session).Scan(&busy); e != nil {
			return e
		}
		if busy {
			return fault("operation_running", "Finish or stop the current operation before changing expectations.")
		}
		r, e := sdkReportTx(ctx, tx, session, reportID)
		if e != nil {
			return e
		}
		if r.ContentHash != in.SourceEvidenceHash {
			return fault("evidence_changed", "Review the original imported report again.")
		}
		c, e := SDKReportCase(r, in.SourceCaseID, "")
		if e != nil {
			return e
		}
		if c.Expected != in.Expected {
			return fault("evidence_changed", "The reviewed expectation differs from the original imported expectation.")
		}
		// The simple editor keys by case key; never let a repeated tuple update several cases.
		n := 0
		for _, b := range r.Cases {
			if b.CaseKey == c.CaseKey {
				n++
			}
		}
		if n != 1 {
			return fault("unsupported_capability", "Promote ambiguous canonical cases through an explicit advanced edit.")
		}
		a, e := sdkArtifact(v, r.Binding.ArtifactID)
		if e != nil {
			return e
		}
		blueprint, e := PatchTestSuite(a.Blueprint, []CaseChange{{Action: "update", CaseKey: c.CaseKey, Expected: &in.Expected}}, nil, s.Config.Limits(false))
		if e != nil {
			return fault("unsupported_capability", "These test definitions need an explicit advanced edit; their original expectations are unchanged.")
		}
		// Validate the exact edited test definition without calling any model.
		if _, e = s.Compiler.Compile(blueprint, s.Config.DefaultModels().Evaluator, uuid.New(), s.Config.Limits(false)); e != nil {
			return fault("invalid_request", "The reviewed expectation cannot compile with these tests.")
		}
		out = a
		out.ID = uuid.New()
		out.ParentID = &a.ID
		out.Blueprint = blueprint
		out.CreatedAt = timestamp()
		out.Accepted = false
		out.Original = nil
		out.Validation = nil
		out.Proposal = nil
		out.Provenance = "reviewed_sdk_expectation"
		AppendTestRevision(&v, out, "Reviewed imported expectation from SDK report "+reportID.String()+" for case "+c.CaseKey+".")
		out = v.Document.Artifacts[len(v.Document.Artifacts)-1]
		if e = admitSDKProjection(ctx, tx, session, raw(out), len(hash)+len(in.SourceEvidenceHash)+len(in.SourceCaseID)); e != nil {
			return e
		}
		if e = s.Store.updateDocument(ctx, tx, v); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO vibe_sdk_promotions(session_id,client_id,report_id,source_case_id,source_evidence_hash,request_hash,artifact) VALUES($1,$2,$3,$4,$5,$6,$7)`, session, in.ClientID, reportID, in.SourceCaseID, in.SourceEvidenceHash, hash, raw(out)); e != nil {
			return e
		}
		return event(ctx, tx, session, nil, "report.expectation_promoted")
	})
	return out, err
}
