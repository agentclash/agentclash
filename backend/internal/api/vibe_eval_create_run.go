package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

// evalCreditReservationFor builds a reservation request for run creation: only when a run id is
// preallocated AND a positive managed amount is approved (BYOK/REST → nil, no reservation).
func evalCreditReservationFor(runID uuid.UUID, micros int64) *repository.EvalCreditReservation {
	if runID == uuid.Nil || micros <= 0 {
		return nil
	}
	return &repository.EvalCreditReservation{AmountMicros: micros, Key: "run:" + runID.String()}
}

// createRunEstimateEnvelope is the typed, versioned cost envelope stored on the confirmation
// (pc.Estimate) at propose time and bound at confirmed execution. The preallocated RunID makes the
// reservation key and the run share a stable id across retry; AmountMicros is exactly what gets
// reserved (never recomputed at execution).
type createRunEstimateEnvelope struct {
	SchemaVersion int                    `json:"schema_version"`
	Kind          string                 `json:"kind"` // "create_run"
	AmountMicros  int64                  `json:"amount_micros"`
	RunID         uuid.UUID              `json:"run_id"`
	BillingMode   string                 `json:"billing_mode"` // "managed" | "byok"
	Lanes         []EvalCostLaneEstimate `json:"lanes,omitempty"`
}

const createRunEstimateSchemaVersion = 1

// vibeEvalRunCreator is the manager surface create_run wraps (RunCreationManager satisfies it).
type vibeEvalRunCreator interface {
	EstimateEvalCost(ctx context.Context, caller Caller, input EstimateEvalCostInput) (CostEstimate, error)
	CreateRun(ctx context.Context, caller Caller, input CreateRunInput) (CreateRunResult, error)
}

type createRunArgs struct {
	ChallengePackVersionID string   `json:"challenge_pack_version_id"`
	AgentDeploymentIDs     []string `json:"agent_deployment_ids"`
}

func (a createRunArgs) parse() (uuid.UUID, []uuid.UUID, error) {
	versionID, err := uuid.Parse(a.ChallengePackVersionID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("challenge_pack_version_id is not a UUID: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(a.AgentDeploymentIDs))
	for _, raw := range a.AgentDeploymentIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, nil, fmt.Errorf("agent_deployment_id %q is not a UUID: %w", raw, err)
		}
		ids = append(ids, id)
	}
	return versionID, ids, nil
}

// createRunTool is the first cost-incurring guide tool. It estimates at propose time (CostEstimator),
// refuses unconfirmed execution (Execute), and reserves+creates atomically on approval (ConfirmedTool).
type createRunTool struct{ runs vibeEvalRunCreator }

var (
	_ vibeeval.Tool          = createRunTool{}
	_ vibeeval.CostEstimator = createRunTool{}
	_ vibeeval.ConfirmedTool = createRunTool{}
)

func (createRunTool) Name() string                { return "create_run" }
func (createRunTool) Phases() []string            { return []string{vibeeval.PhaseRun} }
func (createRunTool) RiskTier() vibeeval.RiskTier { return vibeeval.CostIncurringTier }
func (createRunTool) RequiredAction() string      { return string(ActionCreateRun) }
func (createRunTool) Definition() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "create_run",
		Description: "Create and start an eval run for a challenge_pack_version and agent deployments. Requires confirmation; reserves managed eval credit for the approved cost.",
		Parameters:  json.RawMessage(`{"type":"object","required":["challenge_pack_version_id","agent_deployment_ids"],"properties":{"challenge_pack_version_id":{"type":"string"},"agent_deployment_ids":{"type":"array","items":{"type":"string"}}}}`),
	}
}

// EstimateCost computes the approved cost envelope at propose time: it prices the run and preallocates
// the run id. The amount here is exactly what confirmed execution will reserve.
func (t createRunTool) EstimateCost(ctx context.Context, _ vibeeval.Actor, conv vibeeval.Conversation, args json.RawMessage) (json.RawMessage, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var in createRunArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	versionID, deploymentIDs, err := in.parse()
	if err != nil {
		return nil, err
	}
	est, err := t.runs.EstimateEvalCost(ctx, caller, EstimateEvalCostInput{
		WorkspaceID: conv.WorkspaceID, ChallengePackVersionID: versionID, AgentDeploymentIDs: deploymentIDs,
	})
	if err != nil {
		return nil, err
	}
	billingMode := "managed"
	if est.TotalMicros == 0 {
		billingMode = "byok"
	}
	return json.Marshal(createRunEstimateEnvelope{
		SchemaVersion: createRunEstimateSchemaVersion,
		Kind:          "create_run",
		AmountMicros:  est.TotalMicros,
		RunID:         uuid.New(), // preallocated; stable across retry via the stored confirmation
		BillingMode:   billingMode,
		Lanes:         est.Lanes,
	})
}

// Execute refuses: create_run is cost-incurring and must run only through a confirmation that carries
// the approved cost envelope.
func (createRunTool) Execute(context.Context, vibeeval.Actor, vibeeval.Conversation, json.RawMessage) (vibeeval.ToolOutput, error) {
	return vibeeval.ToolOutput{}, errors.New("create_run requires confirmation: it cannot run without an approved cost estimate")
}

// ExecuteConfirmed reserves exactly the approved amount and creates the run atomically (idempotent on
// the preallocated run id). A workflow-start failure after the run/reservation committed is an
// effect-created success-with-warning, never a terminal failure.
func (t createRunTool) ExecuteConfirmed(ctx context.Context, _ vibeeval.Actor, conv vibeeval.Conversation, args json.RawMessage, pc vibeeval.PendingConfirmation) (vibeeval.ToolOutput, error) {
	caller, err := CallerFromContext(ctx)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	// Trust the approved envelope: a malformed/missing estimate fails BEFORE any side effect.
	envelope, err := parseCreateRunEnvelope(pc.Estimate)
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}
	var in createRunArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return vibeeval.ToolOutput{}, fmt.Errorf("invalid bound args: %w", err)
	}
	versionID, deploymentIDs, err := in.parse()
	if err != nil {
		return vibeeval.ToolOutput{}, err
	}

	result, err := t.runs.CreateRun(ctx, caller, CreateRunInput{
		WorkspaceID:            conv.WorkspaceID,
		ChallengePackVersionID: versionID,
		AgentDeploymentIDs:     deploymentIDs,
		RunID:                  envelope.RunID,
		ReservationMicros:      envelope.AmountMicros, // exactly the approved amount; 0 for BYOK
	})
	if err != nil {
		// The run + reservation committed but the workflow failed to start: the approved effect
		// happened, so finalize succeeded (with a warning) and surface that recovery is needed.
		var startErr RunWorkflowStartError
		if errors.As(err, &startErr) {
			return vibeeval.ToolOutput{
				Result:      map[string]any{"run_id": startErr.Run.ID, "status": startErr.Run.Status, "warning": "run created and credit reserved, but workflow start failed; it will be retried"},
				AuditResult: map[string]any{"run_id": startErr.Run.ID.String(), "reserved_micros": envelope.AmountMicros, "workflow_start": "failed_pending_recovery"},
			}, nil
		}
		return vibeeval.ToolOutput{}, err
	}
	return vibeeval.ToolOutput{
		Result:      map[string]any{"run_id": result.Run.ID, "status": result.Run.Status},
		AuditResult: map[string]any{"run_id": result.Run.ID.String(), "reserved_micros": envelope.AmountMicros, "billing_mode": envelope.BillingMode},
	}, nil
}

func parseCreateRunEnvelope(raw json.RawMessage) (createRunEstimateEnvelope, error) {
	if len(raw) == 0 {
		return createRunEstimateEnvelope{}, errors.New("approved cost estimate is missing")
	}
	var envelope createRunEstimateEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return createRunEstimateEnvelope{}, fmt.Errorf("approved cost estimate is malformed: %w", err)
	}
	if envelope.SchemaVersion != createRunEstimateSchemaVersion || envelope.Kind != "create_run" {
		return createRunEstimateEnvelope{}, fmt.Errorf("approved cost estimate has unexpected schema (version=%d kind=%q)", envelope.SchemaVersion, envelope.Kind)
	}
	if envelope.RunID == uuid.Nil {
		return createRunEstimateEnvelope{}, errors.New("approved cost estimate has no run id")
	}
	if envelope.AmountMicros < 0 {
		return createRunEstimateEnvelope{}, fmt.Errorf("approved cost estimate has negative amount %d", envelope.AmountMicros)
	}
	// billing_mode must agree with the amount: a corrupt envelope (byok+positive, managed+zero) would
	// otherwise reserve the wrong thing — fail before any side effect.
	switch envelope.BillingMode {
	case "managed":
		if envelope.AmountMicros <= 0 {
			return createRunEstimateEnvelope{}, errors.New("approved cost estimate is managed but reserves no credit")
		}
	case "byok":
		if envelope.AmountMicros != 0 {
			return createRunEstimateEnvelope{}, fmt.Errorf("approved cost estimate is byok but reserves %d", envelope.AmountMicros)
		}
	default:
		return createRunEstimateEnvelope{}, fmt.Errorf("approved cost estimate has unknown billing_mode %q", envelope.BillingMode)
	}
	return envelope, nil
}
