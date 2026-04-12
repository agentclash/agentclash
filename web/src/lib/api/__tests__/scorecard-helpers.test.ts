import { describe, it, expect } from "vitest";
import type { ScorecardResponse } from "../types";
import { scorePercent, scoreColor, barWidth } from "../../scores";

// --- Fixtures ---

function makeScorecard(
  overrides: Partial<ScorecardResponse> = {},
): ScorecardResponse {
  return {
    state: "ready",
    run_agent_status: "completed",
    id: "sc-1",
    run_agent_id: "agent-1",
    run_id: "run-1",
    evaluation_spec_id: "eval-1",
    overall_score: 0.82,
    correctness_score: 0.95,
    reliability_score: 0.75,
    latency_score: 0.85,
    cost_score: 0.65,
    scorecard: {
      run_agent_id: "agent-1",
      evaluation_spec_id: "eval-1",
      status: "complete",
      warnings: [],
      dimensions: {
        correctness: { state: "available", score: 0.95 },
        reliability: { state: "available", score: 0.75 },
        latency: { state: "available", score: 0.85 },
        cost: { state: "available", score: 0.65 },
      },
      validator_summary: { total: 5, available: 5, pass: 4, fail: 1 },
      metric_summary: { total: 3, available: 3 },
    },
    created_at: "2026-04-13T00:00:00Z",
    updated_at: "2026-04-13T00:01:00Z",
    ...overrides,
  };
}

// --- Tests ---

describe("scorePercent", () => {
  it("formats a score as percentage", () => {
    expect(scorePercent(0.82)).toBe("82.0%");
    expect(scorePercent(1)).toBe("100.0%");
    expect(scorePercent(0)).toBe("0.0%");
    expect(scorePercent(0.333)).toBe("33.3%");
  });

  it("returns em-dash for null/undefined", () => {
    expect(scorePercent(undefined)).toBe("\u2014");
    expect(scorePercent(null as unknown as undefined)).toBe("\u2014");
  });
});

describe("scoreColor", () => {
  it("returns green for high scores (>= 0.8)", () => {
    expect(scoreColor(0.8)).toBe("text-emerald-400");
    expect(scoreColor(0.95)).toBe("text-emerald-400");
    expect(scoreColor(1)).toBe("text-emerald-400");
  });

  it("returns amber for medium scores (0.5 - 0.8)", () => {
    expect(scoreColor(0.5)).toBe("text-amber-400");
    expect(scoreColor(0.79)).toBe("text-amber-400");
  });

  it("returns red for low scores (< 0.5)", () => {
    expect(scoreColor(0)).toBe("text-red-400");
    expect(scoreColor(0.49)).toBe("text-red-400");
  });

  it("returns muted for null/undefined", () => {
    expect(scoreColor(undefined)).toBe("text-muted-foreground");
  });
});

describe("barWidth", () => {
  it("returns percentage string for valid scores", () => {
    expect(barWidth(0.82)).toBe("82.0%");
    expect(barWidth(1)).toBe("100.0%");
    expect(barWidth(0)).toBe("0.0%");
  });

  it("returns 0% for null/undefined", () => {
    expect(barWidth(undefined)).toBe("0%");
  });
});

describe("ScorecardResponse type validation", () => {
  it("has all required fields for ready state", () => {
    const sc = makeScorecard();
    expect(sc.state).toBe("ready");
    expect(sc.id).toBeDefined();
    expect(sc.run_agent_id).toBeDefined();
    expect(sc.run_id).toBeDefined();
    expect(sc.evaluation_spec_id).toBeDefined();
    expect(sc.overall_score).toBeDefined();
    expect(sc.scorecard).toBeDefined();
    expect(sc.created_at).toBeDefined();
    expect(sc.updated_at).toBeDefined();
  });

  it("allows missing scores for pending state", () => {
    const sc = makeScorecard({
      state: "pending",
      overall_score: undefined,
      correctness_score: undefined,
      reliability_score: undefined,
      latency_score: undefined,
      cost_score: undefined,
    });
    expect(sc.state).toBe("pending");
    expect(sc.overall_score).toBeUndefined();
  });

  it("includes message for errored state", () => {
    const sc = makeScorecard({
      state: "errored",
      message: "Agent failed before evaluation",
    });
    expect(sc.state).toBe("errored");
    expect(sc.message).toBe("Agent failed before evaluation");
  });
});

describe("ScorecardDocument dimensions", () => {
  it("maps all four standard dimensions", () => {
    const sc = makeScorecard();
    const dims = sc.scorecard.dimensions;
    expect(Object.keys(dims)).toEqual(
      expect.arrayContaining([
        "correctness",
        "reliability",
        "latency",
        "cost",
      ]),
    );
  });

  it("each available dimension has state and score", () => {
    const sc = makeScorecard();
    const dims = sc.scorecard.dimensions;
    for (const dim of Object.values(dims)) {
      expect(dim.state).toBe("available");
      expect(typeof dim.score).toBe("number");
    }
  });

  it("handles unavailable dimensions", () => {
    const sc = makeScorecard();
    sc.scorecard.dimensions.latency = { state: "unavailable" };
    expect(sc.scorecard.dimensions.latency.state).toBe("unavailable");
    expect(sc.scorecard.dimensions.latency.score).toBeUndefined();
  });

  it("handles error dimensions with reason", () => {
    const sc = makeScorecard();
    sc.scorecard.dimensions.cost = {
      state: "error",
      reason: "provider timeout",
    };
    expect(sc.scorecard.dimensions.cost.state).toBe("error");
    expect(sc.scorecard.dimensions.cost.reason).toBe("provider timeout");
    expect(sc.scorecard.dimensions.cost.score).toBeUndefined();
  });
});

describe("ScorecardDocument summaries", () => {
  it("validator summary includes pass/fail counts", () => {
    const sc = makeScorecard();
    const vs = sc.scorecard.validator_summary;
    expect(vs.total).toBe(5);
    expect(vs.pass).toBe(4);
    expect(vs.fail).toBe(1);
  });

  it("metric summary includes total and available counts", () => {
    const sc = makeScorecard();
    const ms = sc.scorecard.metric_summary;
    expect(ms.total).toBe(3);
    expect(ms.available).toBe(3);
  });
});

describe("Scorecard edge cases", () => {
  it("handles perfect scores (all 1.0)", () => {
    const sc = makeScorecard({
      overall_score: 1.0,
      correctness_score: 1.0,
      reliability_score: 1.0,
      latency_score: 1.0,
      cost_score: 1.0,
    });
    expect(scorePercent(sc.overall_score)).toBe("100.0%");
    expect(scoreColor(sc.overall_score)).toBe("text-emerald-400");
  });

  it("handles zero scores", () => {
    const sc = makeScorecard({
      overall_score: 0,
      correctness_score: 0,
      reliability_score: 0,
      latency_score: 0,
      cost_score: 0,
    });
    expect(scorePercent(sc.overall_score)).toBe("0.0%");
    expect(scoreColor(sc.overall_score)).toBe("text-red-400");
  });

  it("handles scorecard with warnings", () => {
    const sc = makeScorecard();
    sc.scorecard.warnings = [
      "latency data incomplete",
      "cost estimation used fallback model",
    ];
    expect(sc.scorecard.warnings).toHaveLength(2);
    expect(sc.scorecard.warnings![0]).toBe("latency data incomplete");
  });

  it("handles partial evaluation status", () => {
    const sc = makeScorecard();
    sc.scorecard.status = "partial";
    expect(sc.scorecard.status).toBe("partial");
  });

  it("handles failed evaluation status", () => {
    const sc = makeScorecard();
    sc.scorecard.status = "failed";
    expect(sc.scorecard.status).toBe("failed");
  });
});
