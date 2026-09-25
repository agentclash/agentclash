package vibe

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

// A quote is a dry preparation of the real graph, never an execution. Its
// fingerprint excludes the client request ID but binds all execution choices.
type RunQuote struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	Revision    int64     `json:"revision"`
	Fingerprint string    `json:"fingerprint"`
	MaxCost     int64     `json:"max_cost_nano_usd"`
	Cases       int       `json:"cases"`
	Calls       int       `json:"calls"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func runFingerprint(p Plan) string {
	return Hash(raw(struct {
		Kind     string
		Purpose  string
		Models   Models
		Artifact *Artifact
		Evidence *EvidenceSet
		Baseline *uuid.UUID
		Cost     int64
		Calls    int
		Grading  *GradingContract
		Target   *TargetConfiguration
		Limits   *Limits
	}{p.Submission.Kind, p.Submission.Purpose, p.Submission.Models, p.Artifact, p.Evidence, p.Submission.BaselineID, p.MaxCost, p.Calls, p.Grading, p.TargetConfig, p.ExecutionLimits}))
}
func (s *Service) QuoteRun(ctx context.Context, actor string, id uuid.UUID, sub Submission) (RunQuote, error) {
	if sub.Kind != "check" && sub.Kind != "retest" {
		return RunQuote{}, fault("invalid_request", "Choose examples to run first.")
	}
	sub.estimateOnly = true
	sub.RunQuoteID = nil
	sub.CycleID = nil
	o, err := s.Prepare(ctx, actor, id, sub)
	if err != nil {
		return RunQuote{}, err
	}
	var p Plan
	if err = json.Unmarshal(o.Input, &p); err != nil {
		return RunQuote{}, err
	}
	if p.MaxCost > MaxOperationCost {
		return RunQuote{}, fault("budget_limit", "Choose a smaller batch before running.")
	}
	q := RunQuote{ID: uuid.New(), Kind: "run", Revision: sub.Revision, Fingerprint: runFingerprint(p), MaxCost: p.MaxCost, Cases: len(p.Cases), Calls: p.Calls, ExpiresAt: timestamp().Add(10 * time.Minute)}
	_, err = s.Store.DB.Exec(ctx, `INSERT INTO vibe_cycle_quotes(id,session_id,request_hash,specification,max_cost,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, q.ID, id, q.Fingerprint, raw(q), q.MaxCost, q.ExpiresAt)
	return q, err
}
func admitRunQuote(ctx context.Context, tx pgx.Tx, v Session, p Plan, o Operation) error {
	if p.Submission.RunQuoteID == nil {
		return nil
	}
	var bytes []byte
	var authorized *time.Time
	if err := tx.QueryRow(ctx, "SELECT specification,authorized_at FROM vibe_cycle_quotes WHERE id=$1 AND session_id=$2 FOR UPDATE", *p.Submission.RunQuoteID, v.ID).Scan(&bytes, &authorized); err != nil {
		return fault("quote_expired", "Get a fresh estimate before running.")
	}
	var q RunQuote
	if json.Unmarshal(bytes, &q) != nil || q.Kind != "run" || q.Revision != v.Revision || timestamp().After(q.ExpiresAt) || q.Fingerprint != runFingerprint(p) {
		return fault("quote_expired", "The examples, instructions, models or price changed. Get a fresh estimate.")
	}
	if authorized != nil {
		return fault("idempotency_conflict", "This run was already submitted. Reload its progress.")
	}
	if _, err := tx.Exec(ctx, "INSERT INTO vibe_cycle_steps(cycle_id,step,operation_id) VALUES($1,'check',$2)", q.ID, o.ID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "UPDATE vibe_cycle_quotes SET authorized_at=now() WHERE id=$1", q.ID)
	return err
}
