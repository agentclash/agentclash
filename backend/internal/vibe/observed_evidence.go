package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

// An additive view of the existing source owners. Entry identity uses internal
// source coordinates; display aliases may be omitted without merging entries.
type ObservedEntry struct {
	ScoreText     string          `json:"score_text,omitempty"`
	ValueJSON     string          `json:"value_json"`
	EntryID       string          `json:"entry_id"`
	Kind          string          `json:"kind"`
	Path          string          `json:"source_path"`
	Index         int             `json:"source_index"`
	EventID       string          `json:"event_id,omitempty"`
	SourceEventID *string         `json:"source_event_id,omitempty"`
	Sequence      int64           `json:"sequence,omitempty"`
	PayloadHash   string          `json:"payload_hash,omitempty"`
	Value         json.RawMessage `json:"value"`
}
type ObservedCase struct {
	CaseEvidenceBinding
	EntryID          string          `json:"entry_id"`
	SourceIndex      int             `json:"source_index"`
	Status           string          `json:"status"`
	CaptureScope     CaptureScope    `json:"capture_scope"`
	CaptureAssurance string          `json:"capture_assurance"`
	Entries          []ObservedEntry `json:"entries"`
}
type ObservedEvidence struct {
	Binding          ProjectResultBinding `json:"binding"`
	EvidenceMetadata EvidenceMetadata     `json:"evidence_metadata"`
	SourceHash       string               `json:"source_hash"`
	EvidenceHash     string               `json:"evidence_hash"`
	Status           string               `json:"status"`
	Cases            []ObservedCase       `json:"cases"`
	Omissions        []string             `json:"omissions"`
}

func observedID(ref ResultRef, participant, path string) string {
	return "observed:v1:" + Hash(raw(struct {
		Ref               ResultRef
		Participant, Path string
	}{ref, participant, path}))
}
func observedEntry(ref ResultRef, participant, kind, path string, index int, value json.RawMessage) ObservedEntry {
	entry := ObservedEntry{ValueJSON: string(value), EntryID: observedID(ref, participant, path), Kind: kind, Path: path, Index: index, Value: append(json.RawMessage(nil), value...)}
	// Preserve the original numeric lexeme for the outcome summary, including
	// precision that JavaScript numbers cannot represent. Full source remains below.
	if kind == "grade" {
		var fields map[string]json.RawMessage
		if json.Unmarshal(value, &fields) == nil {
			for _, key := range []string{"score", "normalized_score"} {
				var n json.Number
				v := fields[key]
				if len(v) > 0 && (v[0] == '-' || v[0] >= '0' && v[0] <= '9') && json.Unmarshal(v, &n) == nil {
					entry.ScoreText = string(n)
					break
				}
			}
		}
	}
	return entry
}
func finishObserved(out *ObservedEvidence) error {
	out.EvidenceHash = ""
	body := raw(out)
	if len(body) > canonicalMetadataMaxBytes {
		return fault("export_limit", "Selected observed evidence exceeds 8 MiB. Select a smaller result.")
	}
	out.EvidenceHash = Hash(body)
	return nil
}
func observedSDK(report SDKReport) (ObservedEvidence, error) {
	out := ObservedEvidence{Binding: report.Binding, EvidenceMetadata: report.EvidenceMetadata, SourceHash: report.ContentHash, Status: "imported", Cases: []ObservedCase{}, Omissions: []string{"SDK grades, metadata and capture are imported claims; originals remain unchanged. Reference corpus is not observed retrieval."}}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(report.SourceJSON), &doc); err != nil {
		return out, err
	}
	var cases []json.RawMessage
	if report.SourceKind == "agent_result" {
		cases = []json.RawMessage{json.RawMessage(report.SourceJSON)}
	} else if err := json.Unmarshal(doc["cases"], &cases); err != nil {
		return out, err
	}
	for i, body := range cases {
		var value map[string]json.RawMessage
		if err := json.Unmarshal(body, &value); err != nil {
			return out, err
		}
		sourceID := ""
		path := fmt.Sprintf("/cases/%d", i)
		agent := value["agent_result"]
		if report.SourceKind == "agent_result" {
			if len(report.Cases) != 1 {
				return out, fault("invalid_request", "Standalone source binding is unavailable.")
			}
			sourceID = report.Cases[0].SourceCaseID
			path = ""
			agent = body
		} else {
			var c struct {
				ID string `json:"case_id"`
			}
			if err := json.Unmarshal(value["case"], &c); err != nil {
				return out, err
			}
			sourceID = c.ID
		}
		var binding *SDKCaseBinding
		for n := range report.Cases {
			if report.Cases[n].SourceCaseID == sourceID {
				if binding != nil {
					return out, fault("invalid_request", "Ambiguous SDK source case.")
				}
				binding = &report.Cases[n]
			}
		}
		if binding == nil {
			return out, fault("not_found", "SDK source case binding is unavailable.")
		}
		c := ObservedCase{CaseEvidenceBinding: CaseEvidenceBinding{CaseKey: binding.CaseKey, SourceCaseID: sourceID}, EntryID: observedID(report.Binding.ResultRef, "", path), SourceIndex: i, Status: "ungraded", CaptureScope: report.EvidenceMetadata.CaptureScope, CaptureAssurance: "imported", Entries: []ObservedEntry{}}
		if binding.CaptureScope != nil {
			c.CaptureScope = *binding.CaptureScope
		}
		if report.SourceKind != "agent_result" {
			_ = json.Unmarshal(value["status"], &c.Status)
			var metrics []json.RawMessage
			if v, ok := value["metrics"]; ok {
				if err := json.Unmarshal(v, &metrics); err != nil {
					return out, err
				}
			}
			for j, metric := range metrics {
				c.Entries = append(c.Entries, observedEntry(report.Binding.ResultRef, "", "grade", fmt.Sprintf("%s/metrics/%d", path, j), j, metric))
			}
			if v, ok := value["error"]; ok && string(v) != "null" {
				c.Entries = append(c.Entries, observedEntry(report.Binding.ResultRef, "", "diagnostic", path+"/error", 0, v))
			}
		}
		var a map[string]json.RawMessage
		if len(agent) > 0 && string(agent) != "null" {
			if err := json.Unmarshal(agent, &a); err != nil {
				return out, err
			}
		}
		agentPath := path + "/agent_result"
		if report.SourceKind == "agent_result" {
			agentPath = ""
		}
		for _, field := range []struct{ key, kind string }{{"messages", "message"}, {"tool_calls", "tool_execution"}, {"retrieval_context", "retrieval"}} {
			var values []json.RawMessage
			if v, ok := a[field.key]; ok {
				if err := json.Unmarshal(v, &values); err != nil {
					return out, err
				}
			}
			for j, v := range values {
				c.Entries = append(c.Entries, observedEntry(report.Binding.ResultRef, "", field.kind, fmt.Sprintf("%s/%s/%d", agentPath, field.key, j), j, v))
			}
		}
		if v, ok := a["output"]; ok {
			c.Entries = append(c.Entries, observedEntry(report.Binding.ResultRef, "", "output", agentPath+"/output", 0, v))
		}
		out.Cases = append(out.Cases, c)
	}
	if len(cases) == 0 {
		out.Omissions = append(out.Omissions, "This source contains no cases; configuration/error diagnostics are retained in the original report.")
	}
	return out, finishObserved(&out)
}

func normalizeCanonicalEvent(ref ResultRef, agent uuid.UUID, event CanonicalEvent, payload []byte) []ObservedEntry {
	var p map[string]json.RawMessage
	if json.Unmarshal(payload, &p) != nil {
		return nil
	}
	out := []ObservedEntry{}
	base := "/events/" + event.ID
	add := func(kind, path string, index int, v json.RawMessage) {
		if len(v) == 0 {
			return
		}
		entry := observedEntry(ref, agent.String(), kind, base+path, index, v)
		entry.EventID = event.ID
		entry.SourceEventID = event.SourceID
		entry.Sequence = event.Sequence
		entry.PayloadHash = event.PayloadHash
		out = append(out, entry)
	}
	switch event.Type {
	case "model.call.completed":
		add("output", "/output_text", 0, p["output_text"])
		var calls []json.RawMessage
		_ = json.Unmarshal(p["tool_calls"], &calls)
		for i, v := range calls {
			add("tool_proposal", fmt.Sprintf("/tool_calls/%d", i), i, v)
		}
	case "model.tool_calls.proposed":
		var calls []json.RawMessage
		_ = json.Unmarshal(p["tool_calls"], &calls)
		for i, v := range calls {
			add("tool_proposal", fmt.Sprintf("/tool_calls/%d", i), i, v)
		}
	case "tool.call.completed":
		add("tool_execution", "", 0, payload)
	case "tool.call.failed", "tool.call.started":
		add("tool_attempt", "", 0, payload)
	case "turn.user.message", "turn.assistant.message":
		add("message", "", 0, payload)
	case "system.output.finalized":
		if v, ok := p["final_output"]; ok {
			add("output", "/final_output", 0, v)
		} else {
			add("output", "/output", 0, p["output"])
		}
	}
	// Canonical event v1 has no typed retrieval receipt. Arbitrary keys in mapped
	// HTTP output or tool results never become observed retrieval automatically.
	return out
}
func observedCanonicalTx(ctx context.Context, tx pgx.Tx, v Session, ref ResultRef, artifact *uuid.UUID) (ObservedEvidence, error) {
	source, err := canonicalResultTx(ctx, tx, v, ref, artifact)
	if err != nil {
		return ObservedEvidence{}, err
	}
	out := ObservedEvidence{Binding: source.Binding, EvidenceMetadata: source.EvidenceMetadata, SourceHash: source.EvidenceHash, Status: source.Status, Cases: []ObservedCase{}, Omissions: []string{"Only retained canonical event payloads and stored grade receipts are normalized. Reference corpus and arbitrary tool-output fields are not retrieval receipts.", "Full upstream HTTP/callback wire bytes and offloaded bodies are not included. External capture and seed acknowledgment remain unknown."}}
	retainedBytes := 0
	for _, participant := range source.Participants {
		start := len(out.Cases)
		for i, binding := range participant.Cases {
			out.Cases = append(out.Cases, ObservedCase{CaseEvidenceBinding: binding.CaseEvidenceBinding, EntryID: observedID(ref, participant.RunAgentID.String(), "/cases/"+binding.SourceCaseID), SourceIndex: i, Status: "unknown", CaptureScope: UnknownCaptureScope(), CaptureAssurance: "platform observations; coverage unknown", Entries: []ObservedEntry{}})
		}
		unbound := ObservedCase{CaseEvidenceBinding: CaseEvidenceBinding{RunAgentID: &participant.RunAgentID}, EntryID: observedID(ref, participant.RunAgentID.String(), "/unbound"), SourceIndex: -1, Status: participant.Status, CaptureScope: UnknownCaptureScope(), CaptureAssurance: "unbound participant observations", Entries: []ObservedEntry{}}
		// Metadata already authorizes and bounds these identities. Stream their
		// payloads in the same order and transaction, without a query per event.
		eventIDs := make([]string, len(participant.Events))
		for i, event := range participant.Events {
			eventIDs[i] = event.ID
		}
		events, err := tx.Query(ctx, `SELECT e.payload FROM unnest($2::text[]) WITH ORDINALITY AS selected(id,position) JOIN run_events e ON e.id=selected.id::bigint AND e.run_agent_id=$1 ORDER BY selected.position`, participant.RunAgentID, eventIDs)
		if err != nil {
			return out, err
		}
		for _, event := range participant.Events {
			if !events.Next() {
				err = events.Err()
				events.Close()
				if err == nil {
					err = pgx.ErrNoRows
				}
				return out, err
			}
			var payload []byte
			if err = events.Scan(&payload); err != nil {
				events.Close()
				return out, err
			}
			retainedBytes += len(payload)
			if retainedBytes > canonicalMetadataMaxBytes {
				events.Close()
				return out, fault("export_limit", "Selected observed payloads exceed 8 MiB.")
			}
			entries := normalizeCanonicalEvent(ref, participant.RunAgentID, event, payload)
			if len(entries) == 0 {
				continue
			}
			var binding struct {
				SourceCaseID string `json:"source_case_id"`
				CaseKey      string `json:"case_key"`
				ChallengeKey string `json:"challenge_key"`
			}
			_ = json.Unmarshal(payload, &binding)
			chosen := -1
			for i, c := range participant.Cases {
				matches := binding.SourceCaseID != "" && binding.SourceCaseID == c.SourceCaseID
				if binding.SourceCaseID == "" && binding.CaseKey != "" {
					matches = c.CaseKey == binding.CaseKey && (binding.ChallengeKey == "" || binding.ChallengeKey == c.ChallengeKey)
				}
				if matches {
					if chosen != -1 {
						chosen = -2
						break
					}
					chosen = i
				}
			}
			target := &unbound
			if chosen >= 0 {
				target = &out.Cases[start+chosen]
			}
			target.Entries = append(target.Entries, entries...)
			for _, entry := range entries {
				switch entry.Kind {
				case "message":
					target.CaptureScope.Messages = DimensionCapture{Scope: "partial", Instrumentation: "instrumented"}
				case "tool_execution", "tool_attempt":
					target.CaptureScope.Tools = DimensionCapture{Scope: "partial", Instrumentation: "instrumented"}
				}
			}
		}
		events.Close()
		if err = events.Err(); err != nil {
			return out, err
		}
		// Warehouse grades use local keys. Bind only an unambiguous key, keeping
		// ambiguous grades participant-scoped rather than claiming another case.
		rows, e := tx.Query(ctx, `SELECT id::text,case_key,verdict,jsonb_build_object('score',score,'correctness',correctness,'verdict',verdict,'failure_class',failure_class) FROM case_results WHERE run_agent_id=$1 ORDER BY id`, participant.RunAgentID)
		if e != nil {
			return out, e
		}
		for rows.Next() {
			var id, key, status string
			var grade []byte
			if e = rows.Scan(&id, &key, &status, &grade); e != nil {
				break
			}
			chosen := -1
			for i, c := range participant.Cases {
				if c.CaseKey == key {
					if chosen != -1 {
						chosen = -2
						break
					}
					chosen = i
				}
			}
			target := &unbound
			if chosen >= 0 {
				target = &out.Cases[start+chosen]
				target.Status = status
			}
			target.Entries = append(target.Entries, observedEntry(ref, participant.RunAgentID.String(), "grade", "/case_results/"+id, 0, grade))
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return out, e
		}
		// Canonical validator receipts carry challenge identity, which is only
		// a case binding when this participant has exactly one case for it.
		// Preserve the complete original receipt and never turn one validator's
		// pass into a synthesized whole-case pass.
		grades, e := tx.Query(ctx, `SELECT id::text,challenge_identity_id,jsonb_build_object('judge_key',judge_key,'verdict',verdict,'normalized_score',normalized_score,'raw_output',raw_output) FROM judge_results WHERE run_agent_id=$1 ORDER BY judge_key,id LIMIT 5001`, participant.RunAgentID)
		if e != nil {
			return out, e
		}
		gradeCount := 0
		for grades.Next() {
			var id string
			var challenge *uuid.UUID
			var value []byte
			if e = grades.Scan(&id, &challenge, &value); e != nil {
				break
			}
			gradeCount++
			retainedBytes += len(value)
			if retainedBytes > canonicalMetadataMaxBytes {
				e = fault("export_limit", "Selected grade evidence exceeds 8 MiB.")
				break
			}
			if gradeCount > 5000 {
				e = fault("export_limit", "Selected grade evidence exceeds one view.")
				break
			}
			chosen := -1
			if challenge != nil {
				for i, c := range participant.Cases {
					if c.ChallengeIdentityID == *challenge {
						if chosen != -1 {
							chosen = -2
							break
						}
						chosen = i
					}
				}
			}
			target := &unbound
			if chosen >= 0 {
				target = &out.Cases[start+chosen]
				if target.Status == "unknown" {
					target.Status = "graded"
				}
			}
			target.Entries = append(target.Entries, observedEntry(ref, participant.RunAgentID.String(), "grade", "/judge_results/"+id, gradeCount-1, value))
		}
		if e == nil {
			e = grades.Err()
		}
		grades.Close()
		if e != nil {
			return out, e
		}
		var scoreID string
		var scorecard []byte
		e = tx.QueryRow(ctx, `SELECT id::text,scorecard FROM run_agent_scorecards WHERE run_agent_id=$1`, participant.RunAgentID).Scan(&scoreID, &scorecard)
		if e == nil {
			unbound.Entries = append(unbound.Entries, observedEntry(ref, participant.RunAgentID.String(), "grade", "/run_agent_scorecards/"+scoreID, 0, scorecard))
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
		if len(unbound.Entries) > 0 {
			out.Cases = append(out.Cases, unbound)
		}
		if len(raw(out)) > canonicalMetadataMaxBytes {
			return out, fault("export_limit", "Selected observed evidence exceeds 8 MiB.")
		}
	}
	return out, finishObserved(&out)
}
func observedOperationTx(ctx context.Context, tx pgx.Tx, v Session, ref ResultRef, artifact *uuid.UUID) (ObservedEvidence, error) {
	out := ObservedEvidence{Cases: []ObservedCase{}, Omissions: []string{"Saved references and evaluator context are not observed retrieval. Tool/retrieval coverage is unknown unless separately captured."}}
	o, err := scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 AND session_id=$2", ref.ID, v.ID))
	if err != nil {
		return out, err
	}
	if o.ResultScope == nil || o.EvidenceMetadata == nil {
		return out, fault("not_found", "Result source scope is unavailable.")
	}
	if artifact != nil && *artifact != o.ResultScope.ArtifactID {
		return out, fault("not_found", "Result version does not match.")
	}
	out.Binding = ProjectResultBinding{SessionID: v.ID, ArtifactID: o.ResultScope.ArtifactID, ResultRef: ref}
	out.EvidenceMetadata = *o.EvidenceMetadata
	out.Status = string(o.State)
	for i, key := range o.ResultScope.CaseKeys {
		var body []byte
		err = tx.QueryRow(ctx, `SELECT result FROM vibe_case_results WHERE operation_id=$1 AND case_key=$2`, o.ID, key).Scan(&body)
		if err == pgx.ErrNoRows {
			out.Cases = append(out.Cases, ObservedCase{CaseEvidenceBinding: CaseEvidenceBinding{CaseKey: key, SourceCaseID: key}, EntryID: observedID(ref, "", fmt.Sprintf("/cases/%d", i)), SourceIndex: i, Status: "not_recorded", CaptureScope: UnknownCaptureScope(), CaptureAssurance: "no retained result", Entries: []ObservedEntry{}})
			continue
		}
		if err != nil {
			return out, err
		}
		var result CaseResult
		if err = json.Unmarshal(body, &result); err != nil {
			return out, err
		}
		c := ObservedCase{CaseEvidenceBinding: CaseEvidenceBinding{CaseKey: key, SourceCaseID: key}, EntryID: observedID(ref, "", fmt.Sprintf("/cases/%d", i)), SourceIndex: i, Status: string(result.Verdict), CaptureScope: out.EvidenceMetadata.CaptureScope, CaptureAssurance: "retained conversation; other coverage unknown", Entries: []ObservedEntry{}}
		base := fmt.Sprintf("/cases/%d", i)
		c.Entries = append(c.Entries, observedEntry(ref, "", "grade", base+"/grade", i, raw(struct {
			Verdict Verdict       `json:"verdict"`
			Checks  []CheckResult `json:"checks"`
			Error   *Fault        `json:"error,omitempty"`
		}{result.Verdict, result.Checks, result.Error})))
		for j, m := range result.Messages {
			c.Entries = append(c.Entries, observedEntry(ref, "", "message", fmt.Sprintf("%s/messages/%d", base, j), j, raw(m)))
		}
		if len(result.Messages) > 0 {
			c.CaptureScope.Messages = DimensionCapture{Scope: "partial", Instrumentation: "instrumented"}
		}
		c.Entries = append(c.Entries, observedEntry(ref, "", "output", base+"/output", 0, raw(result.Output)))
		out.Cases = append(out.Cases, c)
	}
	out.SourceHash = Hash(raw(out.Cases))
	return out, finishObserved(&out)
}
func observedEvidenceTx(ctx context.Context, tx pgx.Tx, v Session, ref ResultRef, artifact *uuid.UUID) (ObservedEvidence, error) {
	switch ref.Kind {
	case ResultRun, ResultEvalSession:
		return observedCanonicalTx(ctx, tx, v, ref, artifact)
	case ResultSDKReport:
		if v.Anonymous || v.WorkspaceID == nil {
			return ObservedEvidence{}, fault("not_found", "SDK source unavailable.")
		}
		report, err := sdkReportTx(ctx, tx, v.ID, ref.ID)
		if err != nil {
			return ObservedEvidence{}, err
		}
		if artifact != nil && report.Binding.ArtifactID != *artifact {
			return ObservedEvidence{}, fault("not_found", "SDK version mismatch.")
		}
		return observedSDK(report)
	case ResultVibeOperation:
		return observedOperationTx(ctx, tx, v, ref, artifact)
	default:
		return ObservedEvidence{}, fault("invalid_request", "Choose a supported result source.")
	}
}
func (s *Store) ObservedEvidence(ctx context.Context, actor string, session uuid.UUID, ref ResultRef, artifact *uuid.UUID) (ObservedEvidence, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ObservedEvidence{}, err
	}
	defer tx.Rollback(ctx)
	v, err := scanSession(tx.QueryRow(ctx, sessionSelect, session))
	if err != nil {
		return ObservedEvidence{}, err
	}
	if v.Actor != actor {
		return ObservedEvidence{}, fault("not_found", "Project unavailable.")
	}
	if err = authorize(ctx, tx, actor, v.WorkspaceID, false); err != nil {
		return ObservedEvidence{}, err
	}
	out, err := observedEvidenceTx(ctx, tx, v, ref, artifact)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
