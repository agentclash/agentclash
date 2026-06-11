import type { ApiClient } from "./client";
import type {
  AgentTryout,
  AgentTryoutCompareResult,
  AgentTryoutEventsResponse,
  AgentTryoutPromotionResult,
  AgentTryoutTemplate,
  CreateAgentTryoutInput,
  ListAgentTryoutArtifactsResponse,
  PromoteAgentTryoutInput,
  RerunAgentTryoutInput,
} from "./types";

export function workspaceAgentTryoutsPath(workspaceId: string): string {
  return `/v1/workspaces/${workspaceId}/agent-tryouts`;
}

export function listAgentTryoutTemplates(
  api: ApiClient,
): Promise<{ items: AgentTryoutTemplate[] }> {
  return api.get<{ items: AgentTryoutTemplate[] }>("/v1/agent-tryout-templates");
}

export function createAnonymousAgentTryout(
  api: ApiClient,
  input: CreateAgentTryoutInput,
): Promise<AgentTryout> {
  return api.post<AgentTryout>("/v1/agent-tryouts", input);
}

export function getPublicAgentTryout(
  api: ApiClient,
  tryoutId: string,
): Promise<AgentTryout> {
  return api.get<AgentTryout>(`/v1/agent-tryouts/${tryoutId}`);
}

export function getPublicAgentTryoutEvents(
  api: ApiClient,
  tryoutId: string,
  opts?: { after?: number; limit?: number },
): Promise<AgentTryoutEventsResponse> {
  return api.get<AgentTryoutEventsResponse>(
    `/v1/agent-tryouts/${tryoutId}/events`,
    { params: { after: opts?.after, limit: opts?.limit } },
  );
}

export function createWorkspaceAgentTryout(
  api: ApiClient,
  workspaceId: string,
  input: CreateAgentTryoutInput,
): Promise<AgentTryout> {
  return api.post<AgentTryout>(workspaceAgentTryoutsPath(workspaceId), input);
}

export function listWorkspaceAgentTryouts(
  api: ApiClient,
  workspaceId: string,
  opts?: { limit?: number; offset?: number },
): Promise<{ items: AgentTryout[] }> {
  return api.get<{ items: AgentTryout[] }>(workspaceAgentTryoutsPath(workspaceId), {
    params: { limit: opts?.limit, offset: opts?.offset },
  });
}

export function getWorkspaceAgentTryout(
  api: ApiClient,
  workspaceId: string,
  tryoutId: string,
): Promise<AgentTryout> {
  return api.get<AgentTryout>(
    `${workspaceAgentTryoutsPath(workspaceId)}/${tryoutId}`,
  );
}

export function getWorkspaceAgentTryoutEvents(
  api: ApiClient,
  workspaceId: string,
  tryoutId: string,
  opts?: { after?: number; limit?: number },
): Promise<AgentTryoutEventsResponse> {
  return api.get<AgentTryoutEventsResponse>(
    `${workspaceAgentTryoutsPath(workspaceId)}/${tryoutId}/events`,
    { params: { after: opts?.after, limit: opts?.limit } },
  );
}

export function listWorkspaceAgentTryoutArtifacts(
  api: ApiClient,
  workspaceId: string,
  tryoutId: string,
): Promise<ListAgentTryoutArtifactsResponse> {
  return api.get<ListAgentTryoutArtifactsResponse>(
    `${workspaceAgentTryoutsPath(workspaceId)}/${tryoutId}/artifacts`,
  );
}

export function rerunAgentTryout(
  api: ApiClient,
  tryoutId: string,
  input: RerunAgentTryoutInput,
): Promise<AgentTryout> {
  return api.post<AgentTryout>(`/v1/agent-tryouts/${tryoutId}/rerun`, input);
}

export function promoteAgentTryoutToEval(
  api: ApiClient,
  tryoutId: string,
  input: PromoteAgentTryoutInput = {},
): Promise<AgentTryoutPromotionResult> {
  return api.post<AgentTryoutPromotionResult>(
    `/v1/agent-tryouts/${tryoutId}/promote-to-eval`,
    input,
  );
}

export function compareAgentTryouts(
  api: ApiClient,
  workspaceId: string,
  tryoutIds: string[],
): Promise<AgentTryoutCompareResult> {
  return api.post<AgentTryoutCompareResult>(
    `${workspaceAgentTryoutsPath(workspaceId)}/compare`,
    { tryout_ids: tryoutIds },
  );
}
