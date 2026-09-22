package vibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CompletionReceipt describes effects that actually committed, independently of
// the paid activity's acknowledgement and the accounting settlement.
type CompletionReceipt struct {
	Version           int        `json:"version"`
	OperationID       uuid.UUID  `json:"operation_id"`
	SourceMessageID   uuid.UUID  `json:"source_message_id"`
	Action            string     `json:"action"`
	InputHash         string     `json:"input_hash"`
	CommandHash       string     `json:"command_hash"`
	EffectHash        string     `json:"effect_hash"`
	ArtifactID        *uuid.UUID `json:"artifact_id,omitempty"`
	MessageID         uuid.UUID  `json:"message_id"`
	CaseCount         int        `json:"case_count"`
	ChangedCaseCount  int        `json:"changed_case_count"`
	ValidationStatus  string     `json:"validation_status,omitempty"`
	ValidationVersion string     `json:"validation_version,omitempty"`
	Warnings          []string   `json:"warnings,omitempty"`
	CommittedAt       time.Time  `json:"committed_at"`
}

type DomainProblem struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	CaseKey string `json:"case_key,omitempty"`
	Message string `json:"message,omitempty"`
}

// DomainOutcome is a bounded validation journal attached to one provider output.
// It never changes the attempt's provider/accounting state.
type DomainOutcome struct {
	Stage            string          `json:"stage"`
	Status           string          `json:"status"`
	PromptVersion    string          `json:"prompt_version,omitempty"`
	SchemaVersion    string          `json:"schema_version,omitempty"`
	ValidatorVersion string          `json:"validator_version,omitempty"`
	InputHash        string          `json:"input_hash,omitempty"`
	CandidateHash    string          `json:"candidate_hash,omitempty"`
	Problems         []DomainProblem `json:"problems,omitempty"`
}

func (s *Store) RecordDomainOutcome(ctx context.Context, operationID uuid.UUID, step string, outcome DomainOutcome) error {
	if !boundedOutcomeName(outcome.Stage) || !boundedOutcomeName(outcome.Status) || len(outcome.Problems) > 32 || len(raw(outcome)) > 32<<10 {
		return fault("invalid_outcome", "The operation diagnostic exceeded its bounds.")
	}
	for _, problem := range outcome.Problems {
		if !boundedOutcomeName(problem.Code) || len(problem.Path) > 256 || len(problem.CaseKey) > MaxKeyBytes || len(problem.Message) > 1000 {
			return fault("invalid_outcome", "The operation diagnostic exceeded its bounds.")
		}
	}
	tag, err := s.DB.Exec(ctx, `UPDATE vibe_attempts SET domain_outcome=$3 WHERE operation_id=$1 AND step_key=$2 AND (domain_outcome IS NULL OR domain_outcome=$3::jsonb)`, operationID, step, raw(outcome))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fault("outcome_conflict", "The recorded operation outcome cannot be replaced.")
	}
	return nil
}

func boundedOutcomeName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

// RecordedResponse supplies only a proven-complete response to a DB-only
// recovery path. Missing or ambiguous attempts never authorize a dispatch.
func (s *Store) RecordedResponse(ctx context.Context, operationID uuid.UUID, step, requestHash string, format json.RawMessage) (provider.Response, error) {
	var state, output, savedHash string
	var cost *int64
	var completed *time.Time
	var issue, usage, policy []byte
	err := s.DB.QueryRow(ctx, `SELECT state,output,request_hash,actual_cost,completed_at,error,usage,policy FROM vibe_attempts WHERE operation_id=$1 AND step_key=$2`, operationID, step).Scan(&state, &output, &savedHash, &cost, &completed, &issue, &usage, &policy)
	if errors.Is(err, pgx.ErrNoRows) {
		return provider.Response{}, fault("recovery_unavailable", "This request has no complete recorded response to recover.")
	}
	if err != nil {
		return provider.Response{}, err
	}
	var saved struct {
		ResponseFormat json.RawMessage `json:"response_format"`
	}
	if state != "SUCCEEDED" || cost == nil || completed == nil || len(issue) > 0 && string(issue) != "null" || strings.TrimSpace(output) == "" || savedHash != requestHash || json.Unmarshal(policy, &saved) != nil || !sameJSON(saved.ResponseFormat, format) {
		return provider.Response{}, fault("recovery_unavailable", "The previous provider response is incomplete or does not match this request. It will not be repeated automatically.")
	}
	var response provider.Response
	if err = json.Unmarshal(usage, &response); err != nil {
		return provider.Response{}, fault("recovery_unavailable", "The recorded provider response cannot be recovered safely.")
	}
	if response.FinishReason == provider.FinishReasonMaxTokens || response.OutputText != "" && response.OutputText != output {
		return provider.Response{}, fault("recovery_unavailable", "The recorded provider response is incomplete or inconsistent.")
	}
	response.OutputText = output
	return response, nil
}

func sameJSON(left, right json.RawMessage) bool {
	var a, b any
	if len(left) == 0 {
		left = json.RawMessage("null")
	}
	if len(right) == 0 {
		right = json.RawMessage("null")
	}
	return json.Unmarshal(left, &a) == nil && json.Unmarshal(right, &b) == nil && Hash(raw(a)) == Hash(raw(b))
}

// sourceMessageID keeps an explicit retry attached to its original visible turn.
func (p Plan) sourceMessageID() uuid.UUID {
	if p.Retry != nil && p.Retry.SourceMessageID != uuid.Nil {
		return p.Retry.SourceMessageID
	}
	return p.Submission.ClientID
}

// operationTimeout is frozen for new plans. Existing plans retain their original
// timeout even when the server's newer authoring allowance changes.
func (p Plan) operationTimeout() time.Duration {
	if p.AuthoringVersion >= 11 && p.ExecutionLimits != nil && p.ExecutionLimits.OperationSeconds > 0 {
		return time.Duration(p.ExecutionLimits.OperationSeconds) * time.Second
	}
	return LimitsFor(p.Anonymous).OperationTimeout()
}

func completionCommandHash(reply string, artifact *Artifact, requirements []Requirement, completion []AuthoringCompletion) string {
	// Identity and timestamps are assigned by the server; they are not domain
	// content and must not cause a replay of the same command to conflict.
	var content *Artifact
	if artifact != nil {
		copy := *artifact
		copy.ID, copy.CreatedAt, copy.SourceMessageID = uuid.Nil, time.Time{}, uuid.Nil
		copy.ProposalMessageID = nil
		content = &copy
	}
	return Hash(raw(struct {
		Reply        string
		Artifact     *Artifact
		Requirements []Requirement
		Completion   []AuthoringCompletion
	}{reply, content, requirements, completion}))
}

func completionReceipt(o Operation, p Plan, reply string, artifact *Artifact, requirements []Requirement, completion []AuthoringCompletion) CompletionReceipt {
	var receipt CompletionReceipt
	if len(completion) > 0 && completion[0].Outcome != nil {
		receipt = *completion[0].Outcome
		receipt.Warnings = append([]string(nil), receipt.Warnings...)
	}
	receipt.Version, receipt.OperationID, receipt.SourceMessageID = 1, o.ID, p.sourceMessageID()
	receipt.InputHash = Hash(o.Input)
	effectReply := reply
	if p.AuthoringVersion >= 11 && artifact != nil {
		effectReply = ""
	}
	receipt.EffectHash = completionCommandHash(effectReply, artifact, requirements, completion)
	if receipt.CommandHash == "" {
		receipt.CommandHash = completionCommandHash(reply, artifact, requirements, completion)
	}
	if receipt.Action == "" {
		receipt.Action = o.Kind
		if o.Decision != nil {
			receipt.Action = o.Decision.Intent
		}
	}
	receipt.ArtifactID = nil
	if artifact != nil {
		id := artifact.ID
		receipt.ArtifactID = &id
		var suite struct {
			Cases []json.RawMessage `json:"cases"`
		}
		if json.Unmarshal(artifact.Blueprint, &suite) == nil {
			receipt.CaseCount = len(suite.Cases)
		}
	}
	if receipt.ValidationStatus == "" && artifact != nil {
		if artifact.Validation != nil {
			receipt.ValidationStatus = artifact.Validation.Status
			receipt.ValidationVersion = artifact.Validation.ValidatorVersion
		} else {
			receipt.ValidationStatus = "legacy_unchecked"
		}
	}
	return receipt
}

// Existing in-flight plans may have committed just before this migration. Their
// operation-linked assistant message proves completion without running a model
// or retrospectively claiming that the suite passed newer validation.
func (s *Store) recoverLegacyCompletion(ctx context.Context, tx pgx.Tx, o *Operation) error {
	if o.Completion != nil || (o.Kind != "message" && o.Kind != "build" && o.Kind != "playground") {
		return nil
	}
	var p Plan
	if err := json.Unmarshal(o.Input, &p); err != nil {
		return err
	}
	if p.AuthoringVersion >= 11 {
		return nil
	}
	v, err := scanSession(tx.QueryRow(ctx, sessionSelect, o.SessionID))
	if err != nil {
		return err
	}
	var message *Message
	for i := range v.Document.Messages {
		m := &v.Document.Messages[i]
		if m.Role == "assistant" && m.OperationID != nil && *m.OperationID == o.ID {
			if message != nil {
				return nil // Ambiguous historical effects require inspection.
			}
			message = m
		}
	}
	if message == nil {
		return nil
	}
	var artifact *Artifact
	if message.ArtifactID != nil {
		for i := range v.Document.Artifacts {
			if v.Document.Artifacts[i].ID == *message.ArtifactID {
				artifact = &v.Document.Artifacts[i]
				break
			}
		}
		if artifact == nil {
			return nil
		}
	}
	receipt := completionReceipt(*o, p, message.Content, artifact, nil, nil)
	receipt.MessageID, receipt.CommittedAt = message.ID, message.CreatedAt
	receipt.Warnings = []string{"Completion recovered from the original saved response; newer suite validation was not applied."}
	if _, err = tx.Exec(ctx, "UPDATE vibe_operations SET completion_receipt=$2 WHERE id=$1 AND completion_receipt IS NULL", o.ID, raw(receipt)); err != nil {
		return err
	}
	o.Completion = &receipt
	return nil
}

func mutationAcknowledgement(receipt CompletionReceipt) string {
	switch receipt.Action {
	case "prepare_tests":
		if receipt.CaseCount == 1 {
			return "1 test is ready."
		}
		return fmt.Sprintf("%d tests are ready.", receipt.CaseCount)
	case "edit_tests":
		if receipt.ChangedCaseCount == 0 {
			return "Updated the shared test rules."
		}
		if receipt.ChangedCaseCount == 1 {
			return "Updated 1 test."
		}
		return fmt.Sprintf("Updated %d tests.", receipt.ChangedCaseCount)
	case "suggest_fix":
		return "Prepared an instruction change. Run the same tests to check it."
	}
	return ""
}

// finishCommitted is shared by finalization and Stop while the operation row is
// locked. Once the document committed, a late interruption cannot undo it.
func finishCommitted(ctx context.Context, tx pgx.Tx, o Operation) error {
	if o.State != Completed {
		if o.State.Terminal() {
			// A legacy finalizer could mark already-saved work failed. The
			// recovered receipt proves its effect; correcting its status does
			// not repeat that effect or release uncertain accounting.
			if _, err := tx.Exec(ctx, "UPDATE vibe_operations SET state='COMPLETED' WHERE id=$1 AND completion_receipt IS NOT NULL", o.ID); err != nil {
				return err
			}
		} else {
			if o.State == Running {
				if err := transition(ctx, tx, o.ID, Finalizing); err != nil {
					return err
				}
			} else if o.State != Finalizing {
				return fault("completion_conflict", "The committed operation has an inconsistent execution state.")
			}
			if err := transition(ctx, tx, o.ID, Completed); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE vibe_operations SET error=NULL,completed_at=COALESCE(completed_at,now()) WHERE id=$1", o.ID); err != nil {
			return err
		}
		if err := event(ctx, tx, o.SessionID, &o.ID, "operation.finished"); err != nil {
			return err
		}
	}
	return settle(ctx, tx, o.ID)
}

func validateCompletionReceipt(receipt CompletionReceipt) error {
	if !boundedOutcomeName(receipt.Action) || len(receipt.CommandHash) != 64 || receipt.CaseCount < 0 || receipt.ChangedCaseCount < 0 || len(receipt.Warnings) > 16 {
		return fault("invalid_completion", "The completion result is invalid.")
	}
	for _, warning := range receipt.Warnings {
		if strings.TrimSpace(warning) == "" || len(warning) > 1000 {
			return fault("invalid_completion", "The completion warning is invalid.")
		}
	}
	return nil
}
