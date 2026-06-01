import type { ApiClient } from "./client";
import type { TranscriptResponse } from "./types";

/**
 * Fetch the reconstructed multi-turn conversation transcript for a run-agent.
 * Allows 409 so a failed run's partial transcript is still readable from the
 * response body.
 */
export async function getRunAgentTranscript(
  api: ApiClient,
  runAgentId: string,
): Promise<TranscriptResponse> {
  return api.get<TranscriptResponse>(`/v1/replays/${runAgentId}/transcript`, {
    allowedStatuses: [202, 409],
  });
}

export interface HumanTurnStatus {
  awaiting_human: boolean;
  turn_index?: number;
  phase_id?: string;
  prompt_hint?: string;
}

export async function getHumanTurnStatus(
  api: ApiClient,
  workspaceId: string,
  runId: string,
  runAgentId: string,
): Promise<HumanTurnStatus> {
  return api.get<HumanTurnStatus>(
    `/v1/workspaces/${workspaceId}/runs/${runId}/run-agents/${runAgentId}/turns/status`,
  );
}

export async function submitHumanTurn(
  api: ApiClient,
  workspaceId: string,
  runId: string,
  runAgentId: string,
  message: string,
): Promise<{ status: string }> {
  return api.post<{ status: string }>(
    `/v1/workspaces/${workspaceId}/runs/${runId}/run-agents/${runAgentId}/turns`,
    { message },
  );
}
