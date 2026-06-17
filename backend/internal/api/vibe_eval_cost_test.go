package api

import (
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

func f64p(v float64) *float64 { return &v }
func i64p(v int64) *int64     { return &v }

// managedLaneAt is a managed lane carrying the frozen alias output rate (per million).
func managedLaneAt(outputRatePerMillion float64) evalCostLane {
	return evalCostLane{DeploymentID: uuid.New(), Managed: true, ProviderKey: "anthropic", ProviderModelID: "claude", OutputRatePerMillion: outputRatePerMillion}
}
func managedLane() evalCostLane { return managedLaneAt(3.0) }
func byokLane() evalCostLane    { return evalCostLane{DeploymentID: uuid.New(), Managed: false} }

func TestEstimateEvalCost_MaxCostUSDWinsFirst(t *testing.T) {
	// max_cost_usd present → used even though a (huge) frozen rate is available; tokens ignored.
	est, err := estimateEvalCost([]evalCostLane{managedLaneAt(9_999.0)},
		scoring.RuntimeLimits{MaxCostUSD: f64p(2.50), MaxTotalTokens: i64p(1_000_000)})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if est.TotalMicros != 2_500_000 || est.Lanes[0].Basis != "max_cost_usd" {
		t.Fatalf("est = %+v, want 2.5M via max_cost_usd", est)
	}
}

func TestEstimateEvalCost_OutputRatePricing(t *testing.T) {
	// 1,000,000 tokens × $3/M (frozen output) = $3.00 = 3_000_000 micros.
	est, err := estimateEvalCost([]evalCostLane{managedLaneAt(3.0)}, scoring.RuntimeLimits{MaxTotalTokens: i64p(1_000_000)})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if est.TotalMicros != 3_000_000 || est.Lanes[0].Basis != "max_total_tokens_output_rate" {
		t.Fatalf("est = %+v, want 3M via output rate", est)
	}
}

func TestEstimateEvalCost_RoundsUp(t *testing.T) {
	// $0.0000005 → 0.5 micros → ceil to 1 (never under-reserve).
	est, err := estimateEvalCost([]evalCostLane{managedLane()}, scoring.RuntimeLimits{MaxCostUSD: f64p(0.0000005)})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if est.TotalMicros != 1 {
		t.Fatalf("total = %d, want 1 (rounded up)", est.TotalMicros)
	}
}

func TestEstimateEvalCost_ZeroFrozenRateBlocks(t *testing.T) {
	// Frozen alias rate 0 (model_aliases default) on a token-derived estimate → block, never free.
	_, err := estimateEvalCost([]evalCostLane{managedLaneAt(0)}, scoring.RuntimeLimits{MaxTotalTokens: i64p(1_000_000)})
	if !errors.Is(err, ErrVibeEvalCostEstimateUnavailable) {
		t.Fatalf("err = %v, want ErrVibeEvalCostEstimateUnavailable for zero frozen rate", err)
	}
}

func TestEstimateEvalCost_AllBYOKIsZero(t *testing.T) {
	est, err := estimateEvalCost([]evalCostLane{byokLane(), byokLane()}, scoring.RuntimeLimits{})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if est.TotalMicros != 0 || est.Lanes[0].Basis != "byok" || est.Lanes[1].Basis != "byok" {
		t.Fatalf("est = %+v, want zero, all byok", est)
	}
}

func TestEstimateEvalCost_MixedBillingBlocks(t *testing.T) {
	_, err := estimateEvalCost([]evalCostLane{managedLane(), byokLane()}, scoring.RuntimeLimits{MaxCostUSD: f64p(1)})
	if !errors.Is(err, ErrVibeEvalMixedBilling) {
		t.Fatalf("err = %v, want ErrVibeEvalMixedBilling", err)
	}
}

func TestEstimateEvalCost_NoBoundBlocks(t *testing.T) {
	// managed lane, neither max_cost_usd nor max_total_tokens → block.
	_, err := estimateEvalCost([]evalCostLane{managedLane()}, scoring.RuntimeLimits{})
	if !errors.Is(err, ErrVibeEvalCostEstimateUnavailable) {
		t.Fatalf("err = %v, want ErrVibeEvalCostEstimateUnavailable", err)
	}
}

func TestEstimateEvalCost_SumsManagedLanes(t *testing.T) {
	// Two managed lanes, max_cost_usd applies per lane → 2 × $1.50 = $3.00.
	est, err := estimateEvalCost([]evalCostLane{managedLane(), managedLane()}, scoring.RuntimeLimits{MaxCostUSD: f64p(1.50)})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if est.TotalMicros != 3_000_000 || len(est.Lanes) != 2 {
		t.Fatalf("est = %+v, want 3M across 2 lanes", est)
	}
}

func TestEstimateEvalCost_AuditMetadata(t *testing.T) {
	// token-rate basis carries provider/model, runtime limit, and the frozen output rate.
	est, err := estimateEvalCost([]evalCostLane{managedLaneAt(3.0)}, scoring.RuntimeLimits{MaxTotalTokens: i64p(2_000_000)})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	l := est.Lanes[0]
	if l.ProviderKey != "anthropic" || l.ProviderModelID != "claude" {
		t.Fatalf("lane provider/model = %s/%s", l.ProviderKey, l.ProviderModelID)
	}
	if l.RuntimeLimit != "max_total_tokens=2000000" {
		t.Fatalf("runtime_limit = %q", l.RuntimeLimit)
	}
	if l.OutputRatePerMillion == nil || *l.OutputRatePerMillion != 3.0 {
		t.Fatalf("output rate = %v, want 3.0", l.OutputRatePerMillion)
	}
	if l.Micros != 6_000_000 || est.TotalMicros != 6_000_000 {
		t.Fatalf("micros = %d / total %d, want 6M", l.Micros, est.TotalMicros)
	}
	// max_cost_usd basis records the limit and no output rate.
	est2, _ := estimateEvalCost([]evalCostLane{managedLaneAt(0)}, scoring.RuntimeLimits{MaxCostUSD: f64p(2.5)})
	if est2.Lanes[0].RuntimeLimit != "max_cost_usd=2.5" || est2.Lanes[0].OutputRatePerMillion != nil {
		t.Fatalf("max_cost_usd lane metadata = %+v", est2.Lanes[0])
	}
}
