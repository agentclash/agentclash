package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
)

type GradeReviewInput struct {
	SourceCaseID       string       `json:"source_case_id,omitempty"`
	SourceEvidenceHash string       `json:"source_evidence_hash"`
	ClientID           uuid.UUID    `json:"client_id"`
	ResultRef          ResultRef    `json:"result_ref"`
	ArtifactID         uuid.UUID    `json:"artifact_id"`
	CaseKey            string       `json:"case_key"`
	CheckKey           string       `json:"check_key,omitempty"`
	Reason             string       `json:"reason"`
	Quotes             []ReplyQuote `json:"quotes,omitempty"`
}
type GradeReview struct {
	GradeReviewInput
	ID              uuid.UUID  `json:"id"`
	CreatedAt       time.Time  `json:"created_at"`
	EvidenceHash    string     `json:"evidence_hash"`
	Evidence        CaseResult `json:"evidence"`
	EvidenceChanged bool       `json:"evidence_changed"`
}

func (s *Store) AddGradeReview(ctx context.Context, actor string, session uuid.UUID, in GradeReviewInput) (GradeReview, error) {
	var review GradeReview
	if len(in.SourceCaseID) > 256 || !utf8.ValidString(in.SourceCaseID) || in.ClientID == uuid.Nil || in.ResultRef.ID == uuid.Nil || (in.ResultRef.Kind != ResultVibeOperation && in.ResultRef.Kind != ResultSDKReport) || in.ArtifactID == uuid.Nil || len(in.CaseKey) == 0 || len(in.CaseKey) > MaxKeyBytes || len(in.CheckKey) > MaxKeyBytes || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 4096 || !utf8.ValidString(in.Reason) || len(in.Quotes) > 20 {
		return review, fault("invalid_request", "Choose a saved case and describe the review in at most 4,096 bytes.")
	}
	requestHash := Hash(raw(in))
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect+" FOR UPDATE", session))
		if err != nil {
			return err
		}
		if actor != v.Actor {
			return fault("not_found", "Project is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, true); err != nil {
			return err
		}
		var hash string
		var data []byte
		err = tx.QueryRow(ctx, "SELECT request_hash,body FROM vibe_grade_reviews WHERE session_id=$1 AND client_id=$2", session, in.ClientID).Scan(&hash, &data)
		if err == nil {
			if hash != requestHash {
				return fault("idempotency_conflict", "This review ID already contains different evidence or comments.")
			}
			return json.Unmarshal(data, &review)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var c CaseResult
		var operationID *uuid.UUID
		if in.ResultRef.Kind == ResultSDKReport {
			report, e := sdkReportTx(ctx, tx, session, in.ResultRef.ID)
			if e != nil {
				return e
			}
			if report.Binding.ArtifactID != in.ArtifactID {
				return fault("invalid_request", "Choose the exact tested version.")
			}
			c, err = SDKReportCase(report, in.SourceCaseID, in.CaseKey)
			if err != nil {
				return err
			}
		} else {
			o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", in.ResultRef.ID))
			if err != nil || o.SessionID != session {
				return fault("not_found", "The reviewed result is unavailable in this project.")
			}
			if !o.State.Terminal() {
				return fault("invalid_request", "Wait for this result to finish before reviewing its grade.")
			}
			var plan Plan
			if err = json.Unmarshal(o.Input, &plan); err != nil {
				return err
			}
			if plan.Artifact == nil || plan.Artifact.ID != in.ArtifactID {
				return fault("invalid_request", "Choose the exact tested version.")
			}
			if err = tx.QueryRow(ctx, "SELECT result FROM vibe_case_results WHERE operation_id=$1 AND case_key=$2 AND version=$3", o.ID, in.CaseKey, in.ArtifactID.String()).Scan(&data); err != nil {
				return fault("not_found", "The reviewed case is unavailable.")
			}
			if err = json.Unmarshal(data, &c); err != nil {
				return err
			}
			operationID = &o.ID
		}

		if in.SourceEvidenceHash == "" || in.SourceEvidenceHash != Hash(raw(c)) {
			return fault("evidence_changed", "The saved case changed after you opened it. Reload its evidence before submitting a new review.")
		}
		if in.CheckKey != "" {
			found := false
			for _, check := range c.Checks {
				if check.Key == in.CheckKey {
					found = true
				}
			}
			if !found {
				return fault("invalid_request", "That check is not part of the saved case.")
			}
		}
		messages := map[string]string{}
		if c.Output != "" {
			messages["output"] = c.Output
		}
		for _, m := range c.Messages {
			messages[m.ID] = m.Content
		}
		for _, q := range in.Quotes {
			body, ok := messages[q.MessageID]
			if !ok || len(q.MessageID) > MaxKeyBytes || strings.TrimSpace(q.Text) == "" || len(q.Text) > 4096 || !strings.Contains(body, q.Text) {
				return fault("invalid_request", "A supporting quote is not in the exact saved case.")
			}
		}
		var count int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM vibe_grade_reviews WHERE session_id=$1", session).Scan(&count); err != nil {
			return err
		}
		if count >= 1000 {
			return fault("document_limit", "This project has reached its review limit.")
		}
		review = GradeReview{GradeReviewInput: in, ID: uuid.New(), CreatedAt: timestamp(), Evidence: c, EvidenceHash: Hash(raw(c))}
		if len(raw(review)) > 768<<10 {
			return fault("invalid_request", "This case exceeds the bounded review snapshot size.")
		}
		if in.ResultRef.Kind == ResultSDKReport {
			if err = admitSDKProjection(ctx, tx, session, raw(review), len(requestHash)); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, "INSERT INTO vibe_grade_reviews(id,session_id,client_id,request_hash,result_kind,result_id,body) VALUES($1,$2,$3,$4,$5,$6,$7)", review.ID, session, in.ClientID, requestHash, in.ResultRef.Kind, in.ResultRef.ID, raw(review)); err != nil {
			return err
		}
		return event(ctx, tx, session, operationID, "grade.reviewed")
	})
	return review, err
}

func (s *Store) GradeReviews(ctx context.Context, actor string, session uuid.UUID, result *uuid.UUID) ([]GradeReview, error) {
	out := []GradeReview{}
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		v, err := scanSession(tx.QueryRow(ctx, sessionSelect, session))
		if err != nil {
			return err
		}
		if actor != v.Actor {
			return fault("not_found", "Project is unavailable.")
		}
		if err = authorize(ctx, tx, actor, v.WorkspaceID, false); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT r.body,c.result FROM vibe_grade_reviews r LEFT JOIN vibe_case_results c ON r.result_kind='vibe_operation' AND c.operation_id=r.result_id AND c.case_key=r.body->>'case_key' AND c.version=r.body->>'artifact_id' WHERE r.session_id=$1 AND ($2::uuid IS NULL OR r.result_id=$2) ORDER BY r.created_at DESC,r.id DESC LIMIT 50`, session, result)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var b, current []byte
			if err = rows.Scan(&b, &current); err != nil {
				return err
			}
			var r GradeReview
			var c CaseResult
			if err = json.Unmarshal(b, &r); err != nil {
				return err
			}
			r.EvidenceChanged = len(current) == 0
			if len(current) > 0 {
				if err = json.Unmarshal(current, &c); err != nil {
					return err
				}
				r.EvidenceChanged = Hash(raw(c)) != r.EvidenceHash
			}
			out = append(out, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for i := range out {
			if out[i].ResultRef.Kind != ResultSDKReport {
				continue
			}
			report, e := sdkReportTx(ctx, tx, session, out[i].ResultRef.ID)
			if e != nil {
				out[i].EvidenceChanged = true
				continue
			}
			c, e := SDKReportCase(report, out[i].SourceCaseID, out[i].CaseKey)
			out[i].EvidenceChanged = e != nil || Hash(raw(c)) != out[i].EvidenceHash
		}
		return nil
	})
	return out, err
}
