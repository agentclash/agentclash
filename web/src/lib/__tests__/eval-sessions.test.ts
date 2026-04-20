import { describe, expect, it } from "vitest";

import {
  deriveEvalSessionMode,
  deriveEvalSessionTitle,
  formatEvalSessionValue,
  passMetricAggregateForEffectiveK,
  sortedAggregateDimensions,
} from "../eval-sessions";
import type { EvalSessionDetail, EvalSessionPassMetricSeries } from "../api/types";

describe("eval session helpers", () => {
  it("derives a title from the first child run name", () => {
    const detail = {
      eval_session: { id: "session-12345678" },
      runs: [{ name: "Repeated Eval [1/5]", execution_mode: "single_agent" }],
    } as EvalSessionDetail;

    expect(deriveEvalSessionTitle(detail)).toBe("Repeated Eval");
  });

  it("falls back to a short session id when no child run name exists", () => {
    const detail = {
      eval_session: { id: "session-12345678" },
      runs: [],
    } as EvalSessionDetail;

    expect(deriveEvalSessionTitle(detail)).toBe("Eval Session session-");
  });

  it("derives comparison mode from runs before aggregate fallback", () => {
    expect(
      deriveEvalSessionMode(
        [{ execution_mode: "comparison" }],
        { participants: [{ lane_index: 0, label: "A" }] },
      ),
    ).toBe("comparison");
  });

  it("falls back to aggregate participant count when run mode is absent", () => {
    expect(
      deriveEvalSessionMode([], {
        participants: [
          { lane_index: 0, label: "A" },
          { lane_index: 1, label: "B" },
        ],
      }),
    ).toBe("comparison");
  });

  it("formats probability-like values as percentages", () => {
    expect(formatEvalSessionValue(0.845)).toBe("84.5%");
  });

  it("formats larger scalar values as fixed decimals", () => {
    expect(formatEvalSessionValue(3.5)).toBe("3.50");
  });

  it("returns the metric aggregate for the effective k", () => {
    const series: EvalSessionPassMetricSeries = {
      effective_k: 5,
      by_k: {
        "1": {
          n: 5,
          mean: 0.4,
          median: 0.4,
          std_dev: 0.1,
          min: 0.3,
          max: 0.5,
          high_variance: false,
          high_variance_rule: "rule",
        },
        "5": {
          n: 5,
          mean: 0.9,
          median: 0.9,
          std_dev: 0.05,
          min: 0.8,
          max: 1,
          high_variance: false,
          high_variance_rule: "rule",
        },
      },
    };

    expect(passMetricAggregateForEffectiveK(series)?.mean).toBe(0.9);
  });

  it("sorts aggregate dimensions alphabetically", () => {
    const entries = sortedAggregateDimensions({
      schema_version: 1,
      child_run_count: 2,
      scored_child_count: 2,
      dimensions: {
        reliability: {
          n: 2,
          mean: 0.8,
          median: 0.8,
          std_dev: 0.1,
          min: 0.7,
          max: 0.9,
          high_variance: false,
          high_variance_rule: "rule",
        },
        correctness: {
          n: 2,
          mean: 0.9,
          median: 0.9,
          std_dev: 0.05,
          min: 0.85,
          max: 0.95,
          high_variance: false,
          high_variance_rule: "rule",
        },
      },
    });

    expect(entries.map(([key]) => key)).toEqual(["correctness", "reliability"]);
  });
});
