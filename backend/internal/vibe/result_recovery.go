package vibe

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ResultKind string

const (
	ResultVibeOperation ResultKind = "vibe_operation"
	ResultRun           ResultKind = "run"
	ResultEvalSession   ResultKind = "eval_session"
	ResultSDKReport     ResultKind = "sdk_report"
)

type ResultRef struct {
	Kind ResultKind `json:"kind"`
	ID   uuid.UUID  `json:"id"`
}
type ResultRecovery struct {
	Source       ResultRef `json:"source_result_ref"`
	ArtifactID   uuid.UUID `json:"artifact_id"`
	EvidenceHash string    `json:"evidence_hash"`
}

func (s *Service) prepareResultRecovery(ctx context.Context, v Session, p *Plan) error {
	sub := p.Submission
	if sub.CaseKeys == nil && sub.RetrySourceResultRef == nil {
		return nil
	}
	recovery, err := readResultRecovery(ctx, s.Store.DB, v, sub, false)
	if err != nil {
		return err
	}
	p.Recovery = &recovery
	return nil
}

func readResultRecovery(ctx context.Context, q dbQuery, v Session, sub Submission, lock bool) (ResultRecovery, error) {
	bad := func() (ResultRecovery, error) {
		return ResultRecovery{}, fault("invalid_subset", "Choose unresolved cases from the exact saved result and version.")
	}
	if sub.Kind != "check" || sub.Purpose != "" || sub.RetryOf != nil || sub.BaselineID != nil || sub.CycleID != nil || sub.RetrySourceResultRef == nil || sub.RetrySourceResultRef.Kind != "vibe_operation" || sub.RetrySourceResultRef.ID == uuid.Nil || sub.ArtifactID == nil || len(sub.CaseKeys) == 0 || len(sub.CaseKeys) > LimitsFor(v.Anonymous).Cases {
		return bad()
	}
	seen := map[string]bool{}
	for _, key := range sub.CaseKeys {
		if key == "" || len(key) > MaxKeyBytes || seen[key] {
			return bad()
		}
		seen[key] = true
	}
	query := operationSelect + " WHERE id=$1"
	if lock {
		query += " FOR UPDATE"
	}
	source, err := scanOperation(q.QueryRow(ctx, query, sub.RetrySourceResultRef.ID))
	if err != nil || source.SessionID != v.ID {
		return bad()
	}
	if !source.State.Terminal() || (source.Kind != "check" && source.Kind != "retest") {
		return bad()
	}
	var old Plan
	if err = json.Unmarshal(source.Input, &old); err != nil {
		return ResultRecovery{}, err
	}
	if old.Artifact == nil || old.Artifact.ID != *sub.ArtifactID {
		return bad()
	}
	if sub.EvidenceSetID != nil && (old.Evidence == nil || old.Evidence.ID != *sub.EvidenceSetID) {
		return bad()
	}
	var uncertain bool
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vibe_attempts WHERE operation_id=$1 AND (actual_cost IS NULL OR completed_at IS NULL OR state IN ('DISPATCHING','UNCERTAIN') OR reconciliation_evidence->>'conflict'='true')) OR EXISTS(SELECT 1 FROM vibe_reservations WHERE operation_id=$1 AND settled_amount IS NULL)`, source.ID).Scan(&uncertain)
	if err != nil {
		return ResultRecovery{}, err
	}
	if uncertain || (source.Billing != Settled && source.Billing != Released) {
		return ResultRecovery{}, fault("retry_uncertain", "The original execution and provider costs must finish reconciling before retrying cases.")
	}
	originals := map[string]bool{}
	for _, key := range old.Cases {
		originals[key] = true
	}
	cases := []CaseResult{}
	for _, key := range sub.CaseKeys {
		if !originals[key] {
			return bad()
		}
		var data []byte
		if err = q.QueryRow(ctx, "SELECT result FROM vibe_case_results WHERE operation_id=$1 AND case_key=$2 AND version=$3", source.ID, key, sub.ArtifactID.String()).Scan(&data); err != nil {
			return bad()
		}
		var c CaseResult
		if err = json.Unmarshal(data, &c); err != nil {
			return ResultRecovery{}, err
		}
		if c.Error == nil && caseCompleted(c) && c.Verdict != Unknown {
			return ResultRecovery{}, fault("case_resolved", "A completed passing or failing case is not an unresolved retry. Run all again to repeat it.")
		}
		cases = append(cases, c)
	}
	return ResultRecovery{Source: *sub.RetrySourceResultRef, ArtifactID: *sub.ArtifactID, EvidenceHash: Hash(raw(cases))}, nil
}

func validateResultRecoveryAdmission(ctx context.Context, tx pgx.Tx, v Session, sub Submission, p Plan) error {
	current, err := readResultRecovery(ctx, tx, v, sub, true)
	if err != nil {
		return err
	}
	if p.Artifact == nil || p.Artifact.ID != current.ArtifactID || p.Recovery == nil || current != *p.Recovery || Hash(raw(p.Submission)) != Hash(raw(sub)) || !sameSelectedKeys(p.Cases, sub.CaseKeys) {
		return fault("quote_expired", "The source evidence or selected cases changed. Reload and get a fresh quote.")
	}
	if sub.RunQuoteID == nil {
		return fault("quote_expired", "Review the selected-case quote before running.")
	}
	return nil
}
func sameSelectedKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, k := range a {
		if seen[k] {
			return false
		}
		seen[k] = true
	}
	for _, k := range b {
		if !seen[k] {
			return false
		}
		delete(seen, k)
	}
	return len(seen) == 0
}

// Selection always retains original compiler order and validates every key before
// the first provider call. Nil supports historical full-run plans only.
func selectCompiledCases(all []challengepack.CaseDefinition, keys []string) ([]challengepack.CaseDefinition, error) {
	if keys == nil {
		return all, nil
	}
	out := []challengepack.CaseDefinition{}
	for _, c := range all {
		for _, k := range keys {
			if c.CaseKey == k {
				out = append(out, c)
				break
			}
		}
	}
	actual := []string{}
	for _, c := range out {
		actual = append(actual, c.CaseKey)
	}
	if len(keys) == 0 || !sameSelectedKeys(actual, keys) {
		return nil, fault("invalid_subset", "The selected cases are missing or duplicated in this version.")
	}
	return out, nil
}
func selectConversations(all []EvidenceConversation, keys []string) ([]EvidenceConversation, error) {
	if keys == nil {
		return all, nil
	}
	out := []EvidenceConversation{}
	for _, c := range all {
		for _, k := range keys {
			if c.Key == k {
				out = append(out, c)
				break
			}
		}
	}
	actual := []string{}
	for _, c := range out {
		actual = append(actual, c.Key)
	}
	if len(keys) == 0 || !sameSelectedKeys(actual, keys) {
		return nil, fault("invalid_subset", "The selected recorded chats are missing or duplicated.")
	}
	return out, nil
}
