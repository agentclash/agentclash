package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

// createEvalSessionEstimateSchemaVersion is the version of the typed cost envelope stored on the
// confirmation (pc.Estimate) and bound at confirmed execution. Bump it on any breaking envelope change.
const createEvalSessionEstimateSchemaVersion = 1

// createEvalSessionEnvelope is the server-generated, versioned, non-secret cost envelope for a
// prospective eval session (4d-3, binding fork: option A — the materialized child plan). It carries the
// preallocated session id plus, IN PLAN ORDER, one child per expanded run with its preallocated run id,
// approved amount, billing mode, and the lane identity (matrix key / seed) the estimate enumerated. The
// confirmed execution binds these positionally onto a freshly-expanded plan and reserves exactly these
// amounts — never recomputed.
type createEvalSessionEnvelope struct {
	SchemaVersion   int                              `json:"schema_version"`
	Kind            string                           `json:"kind"` // "create_eval_session"
	EvalSessionID   uuid.UUID                        `json:"eval_session_id"`
	AggregateMicros int64                            `json:"aggregate_micros"`
	Children        []createEvalSessionChildEnvelope `json:"children"`
}

// createEvalSessionChildEnvelope is one child run's frozen plan entry. Non-secret: ids, amount, billing
// mode, lane identity, and the per-lane provider/model estimate metadata — never provider-account ids,
// credentials, prompts, or bundle content.
type createEvalSessionChildEnvelope struct {
	RunID        uuid.UUID              `json:"run_id"`
	AmountMicros int64                  `json:"amount_micros"`
	BillingMode  string                 `json:"billing_mode"` // managed | byok
	MatrixKey    string                 `json:"matrix_key,omitempty"`
	Seed         *int64                 `json:"seed,omitempty"`
	Lanes        []EvalCostLaneEstimate `json:"lanes,omitempty"`
}

// vibeEvalSessionCreator is the manager surface create_eval_session wraps (RunCreationManager satisfies
// it). Estimate prices every expanded child; CreateEvalSession reserves+creates them atomically.
type vibeEvalSessionCreator interface {
	EstimateEvalSessionCost(ctx context.Context, caller Caller, input CreateEvalSessionInput) (EvalSessionCostEstimate, error)
	CreateEvalSession(ctx context.Context, caller Caller, input CreateEvalSessionInput) (CreateEvalSessionResult, error)
}

// createEvalSessionTool is the cost-incurring guide tool that estimates a whole eval session at propose
// time (one aggregate card, per-child reservations), refuses unconfirmed execution, and reserves+creates
// the session + N child runs atomically on approval.
type createEvalSessionTool struct{ sessions vibeEvalSessionCreator }

var (
	_ vibeeval.Tool          = createEvalSessionTool{}
	_ vibeeval.CostEstimator = createEvalSessionTool{}
	_ vibeeval.ConfirmedTool = createEvalSessionTool{}
)

func (createEvalSessionTool) Name() string                { return "create_eval_session" }
func (createEvalSessionTool) Phases() []string            { return []string{vibeeval.PhaseRun} }
func (createEvalSessionTool) RiskTier() vibeeval.RiskTier { return vibeeval.CostIncurringTier }
func (createEvalSessionTool) RequiredAction() string      { return string(ActionCreateRun) }
func (createEvalSessionTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "create_eval_session",
		Description: "Create and start an eval session (repeated/matrixed runs) for a challenge_pack_version. Requires confirmation; reserves managed eval credit per child run for the approved cost.",
		// The arg shape mirrors the REST create-eval-session body (workspace comes from the conversation).
		Parameters: json.RawMessage(`{"type":"object","required":["challenge_pack_version_id","eval_session"],"properties":{"challenge_pack_version_id":{"type":"string"},"challenge_input_set_id":{"type":"string"},"participants":{"type":"array","items":{"type":"object"}},"execution_mode":{"type":"string"},"name":{"type":"string"},"max_iterations":{"type":"integer"},"eval_session":{"type":"object"}}}`),
	}
}

// parseCreateEvalSessionArgs decodes the tool args into a CreateEvalSessionInput for the conversation's
// workspace, reusing the shared REST builder so estimate and confirmed execution validate identically.
func parseCreateEvalSessionArgs(args json.RawMessage, workspaceID uuid.UUID) (CreateEvalSessionInput, error) {
	var body createEvalSessionRequest
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return CreateEvalSessionInput{}, fmt.Errorf("invalid args: %w", err)
	}
	if decoder.More() {
		return CreateEvalSessionInput{}, errors.New("args must contain exactly one JSON object")
	}
	return buildCreateEvalSessionInput(body, workspaceID)
}

// EstimateCost prices every child run from the same plan execution uses, then preallocates the session
// id + a stable run id per child and materializes them into the cost envelope. The per-child amounts are
// exactly what confirmed execution reserves.
func (t createEvalSessionTool) EstimateCost(ctx context.Context, _ vibeeval.Actor, conv vibeeval.Conversation, args json.RawMessage) (json.RawMessage, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return nil, err
	}
	input, err := parseCreateEvalSessionArgs(args, conv.WorkspaceID)
	if err != nil {
		return nil, err
	}
	estimate, err := t.sessions.EstimateEvalSessionCost(ctx, caller, input)
	if err != nil {
		return nil, err
	}
	children := make([]createEvalSessionChildEnvelope, 0, len(estimate.Children))
	for _, child := range estimate.Children {
		children = append(children, createEvalSessionChildEnvelope{
			RunID:        uuid.New(), // preallocated; stable across retry via the stored confirmation
			AmountMicros: child.AmountMicros,
			BillingMode:  child.BillingMode,
			MatrixKey:    child.MatrixKey,
			Seed:         child.Seed,
			Lanes:        child.Lanes,
		})
	}
	return json.Marshal(createEvalSessionEnvelope{
		SchemaVersion:   createEvalSessionEstimateSchemaVersion,
		Kind:            "create_eval_session",
		EvalSessionID:   uuid.New(),
		AggregateMicros: estimate.AggregateMicros,
		Children:        children,
	})
}

// Execute refuses: create_eval_session is cost-incurring and must run only through a confirmation that
// carries the approved per-child cost envelope.
func (createEvalSessionTool) Execute(context.Context, vibeeval.Actor, vibeeval.Conversation, json.RawMessage) (vibeeval.ToolOutput, error) {
	return vibeeval.ToolOutput{}, errors.New("create_eval_session requires confirmation: it cannot run without an approved cost estimate")
}

// ExecuteConfirmed validates the approved envelope, binds its preallocated ids + per-child reservations
// onto a freshly-expanded plan, and reserves+creates the session atomically (idempotent on the
// preallocated session id). A workflow-start failure after commit is an effect-created
// success-with-warning, never a terminal failure.
func (t createEvalSessionTool) ExecuteConfirmed(ctx context.Context, _ vibeeval.Actor, conv vibeeval.Conversation, args json.RawMessage, pc vibeeval.PendingConfirmation) (vibeeval.ToolOutput, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	// Trust the approved envelope: a malformed/missing/inconsistent estimate fails BEFORE any side effect.
	envelope, err := parseCreateEvalSessionEnvelope(pc.Estimate)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	input, err := parseCreateEvalSessionArgs(args, conv.WorkspaceID)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	input.EvalSessionID = envelope.EvalSessionID
	input.ChildReservations = make([]EvalSessionChildReservation, 0, len(envelope.Children))
	for _, child := range envelope.Children {
		lanes := make([]EvalSessionChildLane, 0, len(child.Lanes))
		for _, lane := range child.Lanes {
			lanes = append(lanes, EvalSessionChildLane{
				DeploymentID:              lane.DeploymentID,
				AgentDeploymentSnapshotID: lane.AgentDeploymentSnapshotID,
				Managed:                   lane.Managed,
			})
		}
		input.ChildReservations = append(input.ChildReservations, EvalSessionChildReservation{
			RunID:        child.RunID,
			AmountMicros: child.AmountMicros,
			MatrixKey:    child.MatrixKey,
			Seed:         child.Seed,
			Lanes:        lanes,
		})
	}

	result, err := t.sessions.CreateEvalSession(ctx, caller, input)
	if err != nil {
		// The session + child runs + reservations committed but the workflow failed to start: the
		// approved effect happened, so surface a warning that recovery is needed (never terminal failure).
		var startErr EvalSessionWorkflowStartError
		if errors.As(err, &startErr) {
			return vibeeval.ToolOutput{
				Result: map[string]any{"eval_session_id": startErr.Session.ID, "run_ids": startErr.RunIDs, "status": startErr.Session.Status, "warning": "eval session created and credit reserved, but workflow start failed; it will be retried"},
				AuditResult: map[string]any{
					"eval_session_id": startErr.Session.ID.String(),
					"run_count":       len(startErr.RunIDs),
					"reserved_micros": envelope.AggregateMicros,
					"workflow_start":  "failed_pending_recovery",
				},
			}, nil
		}
		return vibeeval.ToolOutput{}, err
	}
	return vibeeval.ToolOutput{
		Result: map[string]any{"eval_session_id": result.Session.ID, "run_ids": result.RunIDs, "status": result.Session.Status},
		AuditResult: map[string]any{
			"eval_session_id": result.Session.ID.String(),
			"run_count":       len(result.RunIDs),
			"reserved_micros": envelope.AggregateMicros,
		},
	}, nil
}

// parseCreateEvalSessionEnvelope strictly validates the approved cost envelope before any side effect
// (4d-3, pin #3): schema/kind, a session id, at least one child, no duplicate child run ids, the
// aggregate equals the sum of child amounts, and each child's billing mode agrees with its amount.
func parseCreateEvalSessionEnvelope(raw json.RawMessage) (createEvalSessionEnvelope, error) {
	if len(raw) == 0 {
		return createEvalSessionEnvelope{}, errors.New("approved cost estimate is missing")
	}
	var envelope createEvalSessionEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate is malformed: %w", err)
	}
	if envelope.SchemaVersion != createEvalSessionEstimateSchemaVersion || envelope.Kind != "create_eval_session" {
		return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate has unexpected schema (version=%d kind=%q)", envelope.SchemaVersion, envelope.Kind)
	}
	if envelope.EvalSessionID == uuid.Nil {
		return createEvalSessionEnvelope{}, errors.New("approved cost estimate has no eval session id")
	}
	if len(envelope.Children) == 0 {
		return createEvalSessionEnvelope{}, errors.New("approved cost estimate has no child runs")
	}
	seen := make(map[uuid.UUID]struct{}, len(envelope.Children))
	var sum int64
	for i, child := range envelope.Children {
		if child.RunID == uuid.Nil {
			return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate child %d has no run id", i)
		}
		if _, dup := seen[child.RunID]; dup {
			return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate has duplicate child run id %s", child.RunID)
		}
		seen[child.RunID] = struct{}{}
		if child.AmountMicros < 0 {
			return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate child %s has negative amount %d", child.RunID, child.AmountMicros)
		}
		// billing_mode must agree with the amount (managed>0, byok==0) so a corrupt envelope can't
		// reserve the wrong thing for a child.
		switch child.BillingMode {
		case "managed":
			if child.AmountMicros <= 0 {
				return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate child %s is managed but reserves no credit", child.RunID)
			}
		case "byok":
			if child.AmountMicros != 0 {
				return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate child %s is byok but reserves %d", child.RunID, child.AmountMicros)
			}
		default:
			return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate child %s has unknown billing_mode %q", child.RunID, child.BillingMode)
		}
		sum += child.AmountMicros
	}
	if sum != envelope.AggregateMicros {
		return createEvalSessionEnvelope{}, fmt.Errorf("approved cost estimate aggregate %d != sum of child amounts %d", envelope.AggregateMicros, sum)
	}
	return envelope, nil
}
