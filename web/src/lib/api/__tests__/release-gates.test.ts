import { beforeEach, describe, expect, it, vi } from "vitest";

import { createApiClient } from "../client";
import {
  EMPTY_REGRESSION_GATE_RULES_DRAFT,
  evaluateReleaseGate,
  listReleaseGates,
  normalizeRegressionGateRules,
  regressionGateRulesToDraft,
  regressionRuleLabel,
} from "../release-gates";
import type {
  EvaluateReleaseGateResponse,
  ListReleaseGatesResponse,
} from "../types";

vi.stubEnv("NEXT_PUBLIC_API_URL", "http://localhost:8080");

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

beforeEach(() => {
  mockFetch.mockReset();
});

function jsonResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("Release-gate API helpers", () => {
  it("listReleaseGates hits /v1/release-gates with the correct query params", async () => {
    const payload: ListReleaseGatesResponse = {
      baseline_run_id: "b",
      candidate_run_id: "c",
      release_gates: [],
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(payload));

    const api = createApiClient("token");
    const result = await listReleaseGates(api, "b", "c");

    expect(result).toEqual(payload);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/release-gates?baseline_run_id=b&candidate_run_id=c",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer token",
        }),
      }),
    );
  });

  it("evaluateReleaseGate posts the policy and preserves regression_violations in the response", async () => {
    const payload: EvaluateReleaseGateResponse = {
      baseline_run_id: "b",
      candidate_run_id: "c",
      release_gate: {
        id: "gate-1",
        run_comparison_id: "cmp-1",
        policy_key: "default",
        policy_version: 1,
        policy_fingerprint: "fp",
        policy_snapshot: {
          policy_key: "default",
          policy_version: 1,
        },
        verdict: "fail",
        reason_code: "regression_blocking_failure",
        summary: "blocking regression failure",
        evidence_status: "sufficient",
        evaluation_details: {
          policy_key: "default",
          policy_version: 1,
          comparison_status: "comparable",
          regression_violations: [
            {
              rule: "no_blocking_regression_failure",
              severity: "blocking",
              regression_case_id: "case-1",
              suite_id: "suite-1",
              evidence: {
                scoring_result_id: "scoring-1",
                scoring_result_type: "validator_result",
              },
            },
          ],
        },
        generated_at: "2026-04-19T00:00:00Z",
        updated_at: "2026-04-19T00:00:00Z",
      },
    };
    mockFetch.mockResolvedValueOnce(jsonResponse(payload));

    const api = createApiClient("token");
    const result = await evaluateReleaseGate(api, {
      baseline_run_id: "b",
      candidate_run_id: "c",
      policy: {
        policy_key: "default",
        policy_version: 1,
        regression_gate_rules: {
          no_blocking_regression_failure: true,
        },
      },
    });

    expect(result.release_gate.evaluation_details.regression_violations).toEqual(
      [
        {
          rule: "no_blocking_regression_failure",
          severity: "blocking",
          regression_case_id: "case-1",
          suite_id: "suite-1",
          evidence: {
            scoring_result_id: "scoring-1",
            scoring_result_type: "validator_result",
          },
        },
      ],
    );
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/v1/release-gates/evaluate",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
        }),
        body: JSON.stringify({
          baseline_run_id: "b",
          candidate_run_id: "c",
          policy: {
            policy_key: "default",
            policy_version: 1,
            regression_gate_rules: {
              no_blocking_regression_failure: true,
            },
          },
        }),
      }),
    );
  });
});

describe("normalizeRegressionGateRules", () => {
  it("returns undefined for the empty draft so the policy fingerprint stays stable", () => {
    expect(
      normalizeRegressionGateRules({ ...EMPTY_REGRESSION_GATE_RULES_DRAFT }),
    ).toBeUndefined();
  });

  it("omits zero-value flags and empties while keeping enabled rules", () => {
    expect(
      normalizeRegressionGateRules({
        noBlockingRegressionFailure: true,
        noNewBlockingFailureVsBaseline: false,
        maxWarningRegressionFailures: null,
        suiteIds: [],
      }),
    ).toEqual({
      no_blocking_regression_failure: true,
    });
  });

  it("preserves a non-negative warning cap and trims suite ids", () => {
    expect(
      normalizeRegressionGateRules({
        noBlockingRegressionFailure: false,
        noNewBlockingFailureVsBaseline: true,
        maxWarningRegressionFailures: 3,
        suiteIds: [" suite-1 ", "", "suite-2"],
      }),
    ).toEqual({
      no_new_blocking_failure_vs_baseline: true,
      max_warning_regression_failures: 3,
      suite_ids: ["suite-1", "suite-2"],
    });
  });

  it("drops a negative warning cap as if it were unset", () => {
    expect(
      normalizeRegressionGateRules({
        noBlockingRegressionFailure: false,
        noNewBlockingFailureVsBaseline: false,
        maxWarningRegressionFailures: -1,
        suiteIds: [],
      }),
    ).toBeUndefined();
  });

  it("emits scope-only rules when the user only picks suites", () => {
    expect(
      normalizeRegressionGateRules({
        noBlockingRegressionFailure: false,
        noNewBlockingFailureVsBaseline: false,
        maxWarningRegressionFailures: null,
        suiteIds: ["suite-1"],
      }),
    ).toEqual({ suite_ids: ["suite-1"] });
  });

  it("round-trips through regressionGateRulesToDraft", () => {
    const draft = regressionGateRulesToDraft({
      no_blocking_regression_failure: true,
      max_warning_regression_failures: 5,
      suite_ids: ["suite-1", "suite-2"],
    });
    expect(draft).toEqual({
      noBlockingRegressionFailure: true,
      noNewBlockingFailureVsBaseline: false,
      maxWarningRegressionFailures: 5,
      suiteIds: ["suite-1", "suite-2"],
    });
    expect(normalizeRegressionGateRules(draft)).toEqual({
      no_blocking_regression_failure: true,
      max_warning_regression_failures: 5,
      suite_ids: ["suite-1", "suite-2"],
    });
  });
});

describe("regressionRuleLabel", () => {
  it("maps backend rule keys to human labels", () => {
    expect(regressionRuleLabel("no_blocking_regression_failure")).toBe(
      "Blocking regression failure",
    );
    expect(regressionRuleLabel("max_warning_regression_failures")).toBe(
      "Warning threshold exceeded",
    );
  });

  it("falls back to the raw key on unknown rules", () => {
    expect(regressionRuleLabel("future_rule")).toBe("future_rule");
  });
});
