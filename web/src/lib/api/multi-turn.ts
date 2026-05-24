import type { ApiClient } from "./client";

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
