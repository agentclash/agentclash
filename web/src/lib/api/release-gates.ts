import type { ApiClient } from "./client";
import type {
  EvaluateReleaseGateRequest,
  EvaluateReleaseGateResponse,
  ListReleaseGatesResponse,
  RegressionGateRules,
} from "./types";

/**
 * GET /v1/release-gates — list the release gates that have been evaluated
 * against a given baseline/candidate pair.
 */
export function listReleaseGates(
  api: ApiClient,
  baselineRunId: string,
  candidateRunId: string,
  signal?: AbortSignal,
): Promise<ListReleaseGatesResponse> {
  return api.get<ListReleaseGatesResponse>("/v1/release-gates", {
    signal,
    params: {
      baseline_run_id: baselineRunId,
      candidate_run_id: candidateRunId,
    },
  });
}

/** POST /v1/release-gates/evaluate — evaluate a policy against a comparison. */
export function evaluateReleaseGate(
  api: ApiClient,
  request: EvaluateReleaseGateRequest,
  signal?: AbortSignal,
): Promise<EvaluateReleaseGateResponse> {
  return api.post<EvaluateReleaseGateResponse>(
    "/v1/release-gates/evaluate",
    request,
    { signal },
  );
}

export interface RegressionGateRulesDraft {
  noBlockingRegressionFailure: boolean;
  noNewBlockingFailureVsBaseline: boolean;
  maxWarningRegressionFailures: number | null;
  suiteIds: string[];
}

export const EMPTY_REGRESSION_GATE_RULES_DRAFT: RegressionGateRulesDraft = {
  noBlockingRegressionFailure: false,
  noNewBlockingFailureVsBaseline: false,
  maxWarningRegressionFailures: null,
  suiteIds: [],
};

/**
 * Convert the structured UI form into the wire `RegressionGateRules` shape
 * sent on a release-gate policy, or `undefined` when the draft represents
 * the "unset" state. This mirrors backend `normalizeRegressionGateRules`
 * in releasegate.go: non-negative integer for the cap, trimmed suite ids,
 * and omission when every field is its zero value so we do not change
 * the policy fingerprint of legacy policies.
 */
export function normalizeRegressionGateRules(
  draft: RegressionGateRulesDraft,
): RegressionGateRules | undefined {
  const suiteIds = (draft.suiteIds ?? [])
    .map((s) => s.trim())
    .filter((s) => s.length > 0);

  const cap = draft.maxWarningRegressionFailures;
  const hasCap = cap != null && Number.isFinite(cap) && cap >= 0;

  const rulesOn =
    draft.noBlockingRegressionFailure ||
    draft.noNewBlockingFailureVsBaseline ||
    hasCap;

  if (!rulesOn && suiteIds.length === 0) {
    return undefined;
  }

  const result: RegressionGateRules = {};
  if (draft.noBlockingRegressionFailure) {
    result.no_blocking_regression_failure = true;
  }
  if (draft.noNewBlockingFailureVsBaseline) {
    result.no_new_blocking_failure_vs_baseline = true;
  }
  if (hasCap) {
    result.max_warning_regression_failures = Math.trunc(cap);
  }
  if (suiteIds.length > 0) {
    result.suite_ids = suiteIds;
  }
  return result;
}

/** Inverse of `normalizeRegressionGateRules` — used when hydrating the
 * structured form from a policy JSON the user edited by hand. */
export function regressionGateRulesToDraft(
  rules: RegressionGateRules | undefined,
): RegressionGateRulesDraft {
  if (!rules) return { ...EMPTY_REGRESSION_GATE_RULES_DRAFT };
  return {
    noBlockingRegressionFailure:
      rules.no_blocking_regression_failure === true,
    noNewBlockingFailureVsBaseline:
      rules.no_new_blocking_failure_vs_baseline === true,
    maxWarningRegressionFailures:
      typeof rules.max_warning_regression_failures === "number"
        ? rules.max_warning_regression_failures
        : null,
    suiteIds: Array.isArray(rules.suite_ids) ? [...rules.suite_ids] : [],
  };
}

export const REGRESSION_RULE_LABELS: Record<string, string> = {
  no_blocking_regression_failure: "Blocking regression failure",
  no_new_blocking_failure_vs_baseline: "New blocking failure vs baseline",
  max_warning_regression_failures: "Warning threshold exceeded",
};

export function regressionRuleLabel(rule: string): string {
  return REGRESSION_RULE_LABELS[rule] ?? rule;
}

export const REGRESSION_BLOCKING_RULES: ReadonlySet<string> = new Set([
  "no_blocking_regression_failure",
  "no_new_blocking_failure_vs_baseline",
]);
