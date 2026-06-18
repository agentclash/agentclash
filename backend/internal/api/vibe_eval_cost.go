package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

// EstimateEvalCostInput estimates the managed eval-credit reservation for a prospective run.
type EstimateEvalCostInput struct {
	WorkspaceID            uuid.UUID
	ChallengePackVersionID uuid.UUID
	AgentDeploymentIDs     []uuid.UUID
}

// EstimateEvalCost computes the conservative managed-spend ceiling for a prospective run from the
// frozen deployment snapshots + the challenge pack's runtime limits. Read-only (no reservation, no run).
func (m *RunCreationManager) EstimateEvalCost(ctx context.Context, caller Caller, input EstimateEvalCostInput) (CostEstimate, error) {
	if err := AuthorizeWorkspaceAction(ctx, m.authorizer, caller, input.WorkspaceID, ActionReadWorkspace); err != nil {
		return CostEstimate{}, err
	}
	if len(input.AgentDeploymentIDs) == 0 {
		return CostEstimate{}, RunCreationValidationError{Code: "validation_error", Message: "at least one agent deployment is required"}
	}
	// Reject duplicate deployment ids (mirrors CreateRun) — otherwise a dup would double-count a lane.
	seen := make(map[uuid.UUID]struct{}, len(input.AgentDeploymentIDs))
	for _, id := range input.AgentDeploymentIDs {
		if _, ok := seen[id]; ok {
			return CostEstimate{}, RunCreationValidationError{Code: "invalid_agent_deployment_ids", Message: "agent_deployment_ids must not contain duplicates"}
		}
		seen[id] = struct{}{}
	}

	// Visibility: the challenge-pack version must be visible to the workspace (same rule as CreateRun).
	version, err := m.repo.GetRunnableChallengePackVersionByID(ctx, input.ChallengePackVersionID)
	if err != nil {
		// Normalize not-found to a client validation error (the id isn't a runnable version for them).
		if errors.Is(err, repository.ErrChallengePackVersionNotFound) {
			return CostEstimate{}, RunCreationValidationError{Code: "invalid_challenge_pack_version_id", Message: "challenge_pack_version_id must be a runnable version visible to the selected workspace"}
		}
		return CostEstimate{}, err
	}
	if version.WorkspaceID != nil && *version.WorkspaceID != input.WorkspaceID {
		return CostEstimate{}, RunCreationValidationError{Code: "invalid_challenge_pack_version_id", Message: "challenge_pack_version_id must be visible to the selected workspace"}
	}
	if version.WorkspaceID == nil {
		publicPacks, accessErr := m.repo.WorkspacePublicPacksEnabled(ctx, input.WorkspaceID)
		if accessErr != nil {
			return CostEstimate{}, fmt.Errorf("load workspace public pack access: %w", accessErr)
		}
		if !publicPacks {
			return CostEstimate{}, RunCreationValidationError{Code: "invalid_challenge_pack_version_id", Message: "challenge_pack_version_id must be visible to the selected workspace"}
		}
	}
	limits := challengePackRuntimeLimits(version.Manifest)

	// The query is workspace-scoped; a short result means some ids are not visible to this workspace.
	deployments, err := m.repo.ListRunnableDeploymentsWithLatestSnapshot(ctx, input.WorkspaceID, input.AgentDeploymentIDs)
	if err != nil {
		return CostEstimate{}, err
	}
	if len(deployments) != len(input.AgentDeploymentIDs) {
		return CostEstimate{}, RunCreationValidationError{Code: "invalid_agent_deployment_ids", Message: "one or more agent_deployment_ids are not visible to the selected workspace"}
	}

	lanes := make([]evalCostLane, 0, len(deployments))
	for _, d := range deployments {
		lanes = append(lanes, evalCostLane{
			DeploymentID:         d.ID,
			Managed:              d.SourceProviderAccountID == nil, // a frozen source provider account ⇒ BYOK
			ProviderKey:          d.ProviderKey,
			ProviderModelID:      d.ProviderModelID,
			OutputRatePerMillion: d.OutputCostPerMillionTokens,
		})
	}
	return estimateEvalCost(lanes, limits)
}

// challengePackRuntimeLimits extracts max_cost_usd / max_total_tokens from a challenge-pack version
// manifest, preferring evaluation_spec.runtime_limits over a top-level runtime_limits block.
func challengePackRuntimeLimits(manifest json.RawMessage) scoring.RuntimeLimits {
	var document struct {
		RuntimeLimits  scoring.RuntimeLimits `json:"runtime_limits"`
		EvaluationSpec struct {
			RuntimeLimits scoring.RuntimeLimits `json:"runtime_limits"`
		} `json:"evaluation_spec"`
	}
	if len(manifest) == 0 {
		return scoring.RuntimeLimits{}
	}
	if err := json.Unmarshal(manifest, &document); err != nil {
		return scoring.RuntimeLimits{}
	}
	limits := document.EvaluationSpec.RuntimeLimits
	if limits.MaxCostUSD == nil {
		limits.MaxCostUSD = document.RuntimeLimits.MaxCostUSD
	}
	if limits.MaxTotalTokens == nil {
		limits.MaxTotalTokens = document.RuntimeLimits.MaxTotalTokens
	}
	return limits
}

// Eval-cost estimation errors (4d-1). Both map to a 400-style validation failure for the guide/REST.
var (
	// ErrVibeEvalMixedBilling blocks a run whose lanes mix managed + BYOK billing (unsupported for now).
	ErrVibeEvalMixedBilling = errors.New("mixed_billing_unsupported")
	// ErrVibeEvalCostEstimateUnavailable blocks a managed cost-incurring run with no bounded estimate.
	ErrVibeEvalCostEstimateUnavailable = errors.New("cost_estimate_unavailable")
)

// evalCostLane is one deployment lane classified for billing. Managed=false means BYOK (the lane has a
// frozen source provider account), which never consumes eval credit. OutputRatePerMillion is the rate
// FROZEN on the snapshot's model alias (model_aliases.output_cost_per_million_tokens) — never a live
// catalog read — so reservation pricing cannot drift after deployment.
type evalCostLane struct {
	DeploymentID         uuid.UUID
	Managed              bool
	ProviderKey          string
	ProviderModelID      string
	OutputRatePerMillion float64
}

// EvalCostLaneEstimate is the per-lane breakdown shown on the confirmation card and recorded in the
// estimate/audit metadata. Non-secret only: provider/model, the runtime limit applied, the output rate
// used, and the lane micros — never provider-account IDs or credential references.
type EvalCostLaneEstimate struct {
	DeploymentID         uuid.UUID `json:"deployment_id"`
	Managed              bool      `json:"managed"`
	ProviderKey          string    `json:"provider_key,omitempty"`
	ProviderModelID      string    `json:"provider_model_id,omitempty"`
	Basis                string    `json:"basis"`                             // byok | max_cost_usd | max_total_tokens_output_rate
	RuntimeLimit         string    `json:"runtime_limit,omitempty"`           // e.g. "max_cost_usd=2.5" | "max_total_tokens=1000000"
	OutputRatePerMillion *float64  `json:"output_rate_per_million,omitempty"` // only for the token-rate basis
	Micros               int64     `json:"micros"`
}

// CostEstimate is the conservative managed-spend ceiling for a run (the reservation amount).
type CostEstimate struct {
	TotalMicros int64                  `json:"total_micros"`
	Lanes       []EvalCostLaneEstimate `json:"lanes"`
}

// estimateEvalCost computes the conservative managed-spend ceiling (#875 §8, 4d-1, option A):
//   - mixed managed+BYOK lanes → ErrVibeEvalMixedBilling (blocked for now);
//   - all-BYOK → 0 (BYOK never consumes credit);
//   - per managed lane: max_cost_usd wins; else max_total_tokens priced ENTIRELY at the model's output
//     rate (worst-case ceiling); else block. A managed model with no catalog price blocks (never free).
//
// USD → integer micros, always rounded UP (never under-reserve).
func estimateEvalCost(lanes []evalCostLane, limits scoring.RuntimeLimits) (CostEstimate, error) {
	var managed, byok int
	for _, l := range lanes {
		if l.Managed {
			managed++
		} else {
			byok++
		}
	}
	if managed > 0 && byok > 0 {
		return CostEstimate{}, fmt.Errorf("%w: a run must be all-managed or all-BYOK", ErrVibeEvalMixedBilling)
	}

	estimate := CostEstimate{Lanes: make([]EvalCostLaneEstimate, 0, len(lanes))}
	for _, l := range lanes {
		if !l.Managed {
			estimate.Lanes = append(estimate.Lanes, EvalCostLaneEstimate{
				DeploymentID: l.DeploymentID, Managed: false, ProviderKey: l.ProviderKey,
				ProviderModelID: l.ProviderModelID, Micros: 0, Basis: "byok",
			})
			continue
		}
		laneEst, err := laneManagedEstimate(l, limits)
		if err != nil {
			return CostEstimate{}, err
		}
		estimate.TotalMicros += laneEst.Micros
		estimate.Lanes = append(estimate.Lanes, laneEst)
	}
	return estimate, nil
}

func laneManagedEstimate(l evalCostLane, limits scoring.RuntimeLimits) (EvalCostLaneEstimate, error) {
	est := EvalCostLaneEstimate{DeploymentID: l.DeploymentID, Managed: true, ProviderKey: l.ProviderKey, ProviderModelID: l.ProviderModelID}
	if limits.MaxCostUSD != nil {
		est.Basis = "max_cost_usd"
		est.RuntimeLimit = fmt.Sprintf("max_cost_usd=%g", *limits.MaxCostUSD)
		est.Micros = usdToMicrosCeil(*limits.MaxCostUSD)
		return est, nil
	}
	if limits.MaxTotalTokens != nil {
		// A non-positive FROZEN alias rate blocks: model_aliases defaults pricing to 0, which would make
		// a managed run free by accident. Never treat a zero/negative frozen rate as $0.
		if l.OutputRatePerMillion <= 0 {
			return EvalCostLaneEstimate{}, fmt.Errorf("%w: no positive frozen output rate for managed model %s/%s", ErrVibeEvalCostEstimateUnavailable, l.ProviderKey, l.ProviderModelID)
		}
		rate := l.OutputRatePerMillion
		est.Basis = "max_total_tokens_output_rate"
		est.RuntimeLimit = fmt.Sprintf("max_total_tokens=%d", *limits.MaxTotalTokens)
		est.OutputRatePerMillion = &rate
		// Conservative: every token at the frozen output rate (the priciest), per-million.
		usd := float64(*limits.MaxTotalTokens) * rate / 1_000_000.0
		est.Micros = usdToMicrosCeil(usd)
		return est, nil
	}
	return EvalCostLaneEstimate{}, fmt.Errorf("%w: spec declares neither max_cost_usd nor max_total_tokens", ErrVibeEvalCostEstimateUnavailable)
}

// usdToMicrosCeil converts USD to integer micros, rounding UP so the reservation never under-covers.
func usdToMicrosCeil(usd float64) int64 {
	if usd <= 0 {
		return 0
	}
	return int64(math.Ceil(usd * 1_000_000.0))
}
