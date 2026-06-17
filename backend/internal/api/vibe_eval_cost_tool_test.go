package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibeeval"
	"github.com/google/uuid"
)

type fakeCostEstimator struct {
	got EstimateEvalCostInput
	est CostEstimate
	err error
}

func (f *fakeCostEstimator) EstimateEvalCost(_ context.Context, _ Caller, input EstimateEvalCostInput) (CostEstimate, error) {
	f.got = input
	return f.est, f.err
}

func TestEstimateEvalCostTool(t *testing.T) {
	fake := &fakeCostEstimator{est: CostEstimate{TotalMicros: 1_500_000, Lanes: []EvalCostLaneEstimate{{Micros: 1_500_000}}}}
	tool := estimateEvalCostTool{estimator: fake}

	if tool.RiskTier() != vibeeval.ReadTier {
		t.Fatalf("risk tier = %q, want read (no confirmation)", tool.RiskTier())
	}
	if tool.RequiredAction() != string(ActionReadWorkspace) {
		t.Fatalf("action = %q, want read_workspace", tool.RequiredAction())
	}

	conv := vibeeval.Conversation{ID: uuid.New(), WorkspaceID: uuid.New()}
	ctx := context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})
	vid, did := uuid.New(), uuid.New()
	out, err := tool.Execute(ctx, vibeeval.Actor{}, conv,
		json.RawMessage(`{"challenge_pack_version_id":"`+vid.String()+`","agent_deployment_ids":["`+did.String()+`"]}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if fake.got.WorkspaceID != conv.WorkspaceID || fake.got.ChallengePackVersionID != vid ||
		len(fake.got.AgentDeploymentIDs) != 1 || fake.got.AgentDeploymentIDs[0] != did {
		t.Fatalf("estimator input mismatch: %+v", fake.got)
	}
	if out.AuditResult["total_micros"] != int64(1_500_000) {
		t.Fatalf("audit total_micros = %v, want 1.5M", out.AuditResult["total_micros"])
	}
}

func TestEstimateEvalCostTool_RejectsBadUUID(t *testing.T) {
	tool := estimateEvalCostTool{estimator: &fakeCostEstimator{}}
	ctx := context.WithValue(context.Background(), callerContextKey{}, Caller{UserID: uuid.New()})
	_, err := tool.Execute(ctx, vibeeval.Actor{}, vibeeval.Conversation{WorkspaceID: uuid.New()},
		json.RawMessage(`{"challenge_pack_version_id":"nope","agent_deployment_ids":[]}`))
	if err == nil {
		t.Fatal("expected error for non-UUID challenge_pack_version_id")
	}
}
