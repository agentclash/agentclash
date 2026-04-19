import type { ApiClient } from "./client";
import type {
  FailureReviewEvidenceTier,
  FailureReviewFailureClass,
  FailureReviewSeverity,
  ListRunFailuresResponse,
} from "./types";

export interface ListRunFailuresParams {
  agentId?: string;
  severity?: FailureReviewSeverity;
  failureClass?: FailureReviewFailureClass;
  evidenceTier?: FailureReviewEvidenceTier;
  challengeKey?: string;
  caseKey?: string;
  cursor?: string;
  limit?: number;
  signal?: AbortSignal;
}

/**
 * GET /v1/workspaces/{workspaceId}/runs/{runId}/failures
 * Cursor-paginated list of failure review items for a completed run.
 */
export function listRunFailures(
  api: ApiClient,
  workspaceId: string,
  runId: string,
  params: ListRunFailuresParams = {},
): Promise<ListRunFailuresResponse> {
  const {
    agentId,
    severity,
    failureClass,
    evidenceTier,
    challengeKey,
    caseKey,
    cursor,
    limit,
    signal,
  } = params;
  return api.get<ListRunFailuresResponse>(
    `/v1/workspaces/${workspaceId}/runs/${runId}/failures`,
    {
      signal,
      params: {
        agent_id: agentId,
        severity,
        failure_class: failureClass,
        evidence_tier: evidenceTier,
        challenge_key: challengeKey,
        case_key: caseKey,
        cursor,
        limit,
      },
    },
  );
}
