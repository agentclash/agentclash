import type { ApiClient } from "@/lib/api/client";
import type {
  AgentBuild,
  AgentBuildVersion,
  AgentDeploymentCreateResponse,
} from "@/lib/api/types";

/**
 * Collapses the build -> version -> ready -> deployment chain into one request.
 * The caller supplies what an agent fundamentally is (a name and instructions,
 * or a template) and what it runs on (a model on a provider account against a
 * runtime profile); the backend derives the rest.
 *
 * Either `instructions` or `template` must be set.
 */
export interface QuickCreateAgentInput {
  name: string;
  description?: string;
  instructions?: string;
  agent_kind?: string;
  template?: string;
  model_spec?: unknown;
  runtime_profile_id: string;
  provider_account_id: string;
  model: string;
  deployment_config?: unknown;
}

export interface QuickCreateAgentResponse {
  build: AgentBuild;
  version: AgentBuildVersion;
  deployment: AgentDeploymentCreateResponse;
}

export function quickCreateAgent(
  api: ApiClient,
  workspaceId: string,
  input: QuickCreateAgentInput,
): Promise<QuickCreateAgentResponse> {
  return api.post<QuickCreateAgentResponse>(
    `/v1/workspaces/${workspaceId}/agent-builds/quick-create`,
    input,
  );
}
