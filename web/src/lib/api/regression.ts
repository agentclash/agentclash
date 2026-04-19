import type { ApiClient, PaginatedResponse } from "./client";
import type {
  FailureReviewFailureClass,
  FailureReviewPromotionMode,
  RegressionCase,
  RegressionSeverity,
  RegressionSuite,
} from "./types";

export interface ListRegressionSuitesParams {
  limit?: number;
  offset?: number;
}

export interface PromotionOverridesInput {
  judgeThresholdOverrides?: Record<string, number | undefined>;
  assertionToggles?: Record<string, boolean | undefined>;
}

export interface PromoteFailureInput {
  suite_id: string;
  promotion_mode: FailureReviewPromotionMode;
  title: string;
  failure_summary?: string;
  severity?: RegressionSeverity;
  validator_overrides?: Record<string, unknown>;
}

export interface PromoteFailureResult {
  case: RegressionCase;
  created: boolean;
}

export function listRegressionSuites(
  api: ApiClient,
  workspaceId: string,
  params: ListRegressionSuitesParams = {},
): Promise<PaginatedResponse<RegressionSuite>> {
  return api.paginated<RegressionSuite>(
    `/v1/workspaces/${workspaceId}/regression-suites`,
    params,
  );
}

export async function promoteFailure(
  api: ApiClient,
  workspaceId: string,
  runId: string,
  challengeIdentityId: string,
  input: PromoteFailureInput,
): Promise<PromoteFailureResult> {
  const response = await api.postWithMeta<RegressionCase>(
    `/v1/workspaces/${workspaceId}/runs/${runId}/failures/${challengeIdentityId}/promote`,
    input,
    { allowedStatuses: [200, 201] },
  );

  return {
    case: response.data,
    created: response.status === 201,
  };
}

export function defaultPromotionSeverityForFailure(
  failureClass: FailureReviewFailureClass,
): RegressionSeverity {
  switch (failureClass) {
    case "policy_violation":
    case "sandbox_failure":
      return "blocking";
    default:
      return "warning";
  }
}

export function buildPromotionOverrides(
  input: PromotionOverridesInput,
): Record<string, unknown> | undefined {
  const judgeThresholdOverrides = Object.fromEntries(
    Object.entries(input.judgeThresholdOverrides ?? {}).filter(
      ([, value]) => value != null,
    ),
  );
  const assertionToggles = Object.fromEntries(
    Object.entries(input.assertionToggles ?? {}).filter(
      ([, value]) => value != null,
    ),
  );

  const overrides: Record<string, unknown> = {};
  if (Object.keys(judgeThresholdOverrides).length > 0) {
    overrides.judge_threshold_overrides = judgeThresholdOverrides;
  }
  if (Object.keys(assertionToggles).length > 0) {
    overrides.assertion_toggles = assertionToggles;
  }

  return Object.keys(overrides).length > 0 ? overrides : undefined;
}
