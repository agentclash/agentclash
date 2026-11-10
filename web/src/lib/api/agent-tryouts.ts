import type { ApiClient } from "./client";
import type {
  AgentTryout,
  AgentTryoutCompareResult,
  AgentTryoutEventsResponse,
  AgentTryoutInputAttachment,
  AgentTryoutPromotionResult,
  ListAgentTryoutTemplatesResponse,
  CreateAgentTryoutInput,
  ListAgentTryoutArtifactsResponse,
  PromoteAgentTryoutInput,
  RerunAgentTryoutInput,
} from "./types";

export function workspaceAgentTryoutsPath(workspaceId: string): string {
  return `/v1/workspaces/${workspaceId}/agent-tryouts`;
}

const creatorAccessProtocol = "cookie-v1" as const;

// Discovery is deliberately fresh per operation, including polling. A cached
// legacy decision must never silently survive a backend upgrade or host change.
export async function listAgentTryoutTemplates(
  api: ApiClient,
): Promise<ListAgentTryoutTemplatesResponse> {
  const catalog = await api.get<ListAgentTryoutTemplatesResponse>("/v1/agent-tryout-templates", {
    credentials: "omit", cache: "no-store", signal: AbortSignal.timeout(10_000),
  });
  if (!catalog || !Array.isArray(catalog.items) ||
      !catalog.items.every(item => item && typeof item === "object" && typeof item.slug === "string") ||
      (Object.hasOwn(catalog, "creator_access_protocol") && catalog.creator_access_protocol !== creatorAccessProtocol)) {
    throw new Error("Reload or update this client. The tryout API returned an unsupported creator access protocol.");
  }
  return catalog;
}

async function publicTryoutTransport(api: ApiClient) {
  const catalog = await listAgentTryoutTemplates(api);
  const protocol = catalog.creator_access_protocol;
  return {
    protocol,
    options: { credentials: protocol === creatorAccessProtocol ? "include" as const : "omit" as const, cache: "no-store" as const },
  };
}

function publicTryoutResult(result: AgentTryout, protocol: string | undefined): AgentTryout {
  // Older APIs may still expose ID-only claim handlers. Never offer that save.
  return { ...result, save_eligible: protocol === creatorAccessProtocol && result.save_eligible === true };
}

export async function createAnonymousAgentTryout(
  api: ApiClient,
  input: CreateAgentTryoutInput,
): Promise<AgentTryout> {
  const { protocol, options } = await publicTryoutTransport(api);
  const result = await api.post<AgentTryout>("/v1/agent-tryouts", { ...input, creator_access_protocol: protocol }, options);
  return publicTryoutResult(result, protocol);
}

export function uploadAgentTryoutInputAttachment(
  api: ApiClient,
  file: File,
): Promise<AgentTryoutInputAttachment> {
  const formData = new FormData();
  formData.append("file", file, file.name);
  return api.postMultipart<AgentTryoutInputAttachment>(
    "/v1/agent-tryout-input-attachments",
    formData,
  );
}

export async function getPublicAgentTryout(
  api: ApiClient,
  tryoutId: string,
): Promise<AgentTryout> {
  const { protocol, options } = await publicTryoutTransport(api);
  return publicTryoutResult(await api.get<AgentTryout>(`/v1/agent-tryouts/${tryoutId}`, options), protocol);
}

export async function submitAgentTryoutTurn(
  api: ApiClient,
  tryoutId: string,
  input: { message?: string; end?: boolean },
): Promise<{ status: string }> {
  const { options } = await publicTryoutTransport(api);
  return api.post<{ status: string }>(
    `/v1/agent-tryouts/${tryoutId}/turns`,
    input,
    options,
  );
}

export async function getPublicAgentTryoutEvents(
  api: ApiClient,
  tryoutId: string,
  opts?: { after?: number; limit?: number },
): Promise<AgentTryoutEventsResponse> {
  const { options } = await publicTryoutTransport(api);
  return api.get<AgentTryoutEventsResponse>(
    `/v1/agent-tryouts/${tryoutId}/events`,
    { ...options, params: { after: opts?.after, limit: opts?.limit } },
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

export async function claimAgentTryout(api: ApiClient, tryoutId: string, workspaceId: string): Promise<AgentTryout> {
  const { protocol, options } = await publicTryoutTransport(api);
  if (protocol !== creatorAccessProtocol) {
    throw new Error("This API cannot safely save this result. Reload or update before starting a new tryout.");
  }
  return api.post<AgentTryout>(`/v1/agent-tryouts/${tryoutId}/claim`, { workspace_id: workspaceId }, options);
}

export function getSavedAgentTryout(api: ApiClient, tryoutId: string): Promise<AgentTryout> {
  return api.get<AgentTryout>(`/v1/agent-tryouts/${tryoutId}/saved`, { cache: "no-store" });
}
