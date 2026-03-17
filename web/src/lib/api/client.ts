const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  params?: Record<string, string>;
  headers?: Record<string, string>;
};

function getDevAuthHeaders(): Record<string, string> {
  if (typeof window === "undefined") return {};
  const stored = localStorage.getItem("agentclash_dev_auth");
  if (!stored) return {};
  try {
    const auth = JSON.parse(stored);
    const headers: Record<string, string> = {};
    if (auth.userId) headers["X-Agentclash-User-Id"] = auth.userId;
    if (auth.email) headers["X-Agentclash-User-Email"] = auth.email;
    if (auth.displayName) headers["X-Agentclash-User-Display-Name"] = auth.displayName;
    if (auth.workspaceMemberships) headers["X-Agentclash-Workspace-Memberships"] = auth.workspaceMemberships;
    return headers;
  } catch {
    return {};
  }
}

export async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, params, headers = {} } = options;

  let url = `${API_BASE}/v1${path}`;
  if (params) {
    const searchParams = new URLSearchParams(params);
    url += `?${searchParams.toString()}`;
  }

  const authHeaders = getDevAuthHeaders();
  const fetchOptions: RequestInit = {
    method,
    headers: {
      ...authHeaders,
      ...headers,
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
  };

  if (body) {
    fetchOptions.body = JSON.stringify(body);
  }

  const response = await fetch(url, fetchOptions);

  if (!response.ok) {
    let code = "unknown_error";
    let message = `HTTP ${response.status}`;
    try {
      const errorBody = await response.json();
      if (errorBody.error) {
        code = errorBody.error.code || code;
        message = errorBody.error.message || message;
      }
    } catch {
      // ignore parse errors
    }
    throw new ApiError(response.status, code, message);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

// Typed API methods
export const api = {
  // Health
  health: () => apiFetch<{ ok: boolean; service: string }>("/healthz".replace("/v1", ""), { method: "GET" }),

  // Auth
  getSession: () => apiFetch<SessionResponse>("/auth/session"),

  // Runs
  listRuns: (workspaceId: string, limit = 20, offset = 0) =>
    apiFetch<ListRunsResponse>(`/workspaces/${workspaceId}/runs`, {
      params: { limit: String(limit), offset: String(offset) },
    }),

  getRun: (runId: string) => apiFetch<RunResponse>(`/runs/${runId}`),

  createRun: (input: CreateRunInput) =>
    apiFetch<RunResponse>("/runs", { method: "POST", body: input }),

  listRunAgents: (runId: string) =>
    apiFetch<ListRunAgentsResponse>(`/runs/${runId}/agents`),

  // Replays
  getReplay: (runAgentId: string, cursor = 0, limit = 50) =>
    apiFetch<ReplayResponse>(`/replays/${runAgentId}`, {
      params: { cursor: String(cursor), limit: String(limit) },
    }),

  // Scorecards
  getScorecard: (runAgentId: string) =>
    apiFetch<ScorecardResponse>(`/scorecards/${runAgentId}`),

  // Compare
  getComparison: (baselineRunId: string, candidateRunId: string, baselineAgentId?: string, candidateAgentId?: string) => {
    const params: Record<string, string> = {
      baseline_run_id: baselineRunId,
      candidate_run_id: candidateRunId,
    };
    if (baselineAgentId) params.baseline_run_agent_id = baselineAgentId;
    if (candidateAgentId) params.candidate_run_agent_id = candidateAgentId;
    return apiFetch<CompareResponse>("/compare", { params });
  },

  // Workspace
  checkWorkspaceAccess: (workspaceId: string) =>
    apiFetch<{ ok: boolean; workspace_id: string }>(`/workspaces/${workspaceId}/auth-check`),

  // Agent Deployments
  listAgentDeployments: (workspaceId: string) =>
    apiFetch<ListAgentDeploymentsResponse>(`/workspaces/${workspaceId}/agent-deployments`),

  // Challenge Packs
  listChallengePacks: (workspaceId: string) =>
    apiFetch<ListChallengePacksResponse>(`/workspaces/${workspaceId}/challenge-packs`),
};

// Response types
export type SessionResponse = {
  user_id: string;
  workos_user_id?: string;
  email?: string;
  display_name?: string;
  workspace_memberships: { workspace_id: string; role: string }[];
};

export type RunStatus = "draft" | "queued" | "provisioning" | "running" | "scoring" | "completed" | "failed" | "cancelled";
export type RunAgentStatus = "queued" | "ready" | "executing" | "evaluating" | "completed" | "failed";

export type RunResponse = {
  id: string;
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  name: string;
  status: RunStatus;
  execution_mode: string;
  temporal_workflow_id?: string;
  temporal_run_id?: string;
  queued_at?: string;
  started_at?: string;
  finished_at?: string;
  cancelled_at?: string;
  failed_at?: string;
  created_at: string;
  updated_at: string;
  links: { self: string; agents: string };
};

export type ListRunsResponse = {
  items: RunResponse[];
  total: number;
  limit: number;
  offset: number;
};

export type RunAgentResponse = {
  id: string;
  run_id: string;
  lane_index: number;
  label: string;
  agent_deployment_id: string;
  agent_deployment_snapshot_id: string;
  status: RunAgentStatus;
  queued_at?: string;
  started_at?: string;
  finished_at?: string;
  failure_reason?: string;
  created_at: string;
  updated_at: string;
};

export type ListRunAgentsResponse = {
  items: RunAgentResponse[];
};

export type ReplayState = "ready" | "pending" | "errored";

export type ReplayStep = {
  sequence_number?: number;
  headline?: string;
  type?: string;
  status?: string;
  timestamp?: string;
  duration_ms?: number;
  provider_key?: string;
  model_id?: string;
  tool_name?: string;
  error_message?: string;
  [key: string]: unknown;
};

export type ReplayResponse = {
  state: ReplayState;
  message?: string;
  run_agent_id: string;
  run_id: string;
  run_agent_status: string;
  replay?: {
    id: string;
    artifact_id?: string;
    summary: Record<string, unknown>;
    latest_sequence_number: number;
    event_count: number;
    created_at: string;
    updated_at: string;
  };
  steps: ReplayStep[];
  pagination: {
    next_cursor?: string;
    limit: number;
    total_steps: number;
    has_more: boolean;
  };
};

export type ScorecardResponse = {
  state: ReplayState;
  message?: string;
  id?: string;
  run_agent_id: string;
  run_id: string;
  run_agent_status: string;
  evaluation_spec_id?: string;
  overall_score?: number;
  correctness_score?: number;
  reliability_score?: number;
  latency_score?: number;
  cost_score?: number;
  scorecard?: Record<string, unknown>;
  created_at?: string;
  updated_at?: string;
};

export type KeyDelta = {
  metric: string;
  baseline_value: number;
  candidate_value: number;
  delta: number;
  better_direction: "higher" | "lower";
  outcome: "better" | "worse" | "same" | "unknown";
  state: "available" | "unavailable";
};

export type CompareResponse = {
  state: "comparable" | "partial_evidence" | "not_comparable";
  status: "comparable" | "not_comparable";
  reason_code?: string;
  baseline_run_id: string;
  candidate_run_id: string;
  baseline_run_agent_id?: string;
  candidate_run_agent_id?: string;
  generated_at: string;
  key_deltas: KeyDelta[];
  regression_reasons: string[];
  evidence_quality: {
    missing_fields: string[];
    warnings: string[];
  };
  summary: Record<string, unknown>;
  links: { viewer: string };
};

export type CreateRunInput = {
  workspace_id: string;
  challenge_pack_version_id: string;
  challenge_input_set_id?: string;
  name?: string;
  agent_deployment_ids: string[];
};

export type AgentDeployment = {
  id: string;
  name: string;
  status: string;
  latest_snapshot_id?: string;
  created_at: string;
  updated_at: string;
};

export type ListAgentDeploymentsResponse = {
  items: AgentDeployment[];
};

export type ChallengePackVersion = {
  id: string;
  challenge_pack_id: string;
  version_label: string;
  lifecycle_status: string;
  created_at: string;
  updated_at: string;
};

export type ChallengePack = {
  id: string;
  name: string;
  description?: string;
  versions: ChallengePackVersion[];
  created_at: string;
  updated_at: string;
};

export type ListChallengePacksResponse = {
  items: ChallengePack[];
};
