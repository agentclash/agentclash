export type Models = { assistant: string; target: string; evaluator: string };
export type Model = {
  id: string;
  name: string;
  input_nano_per_token: number;
  output_nano_per_token: number;
};
export type Requirement = {
  id: string;
  statement: string;
  status: "proposed" | "accepted" | "rejected" | "superseded";
  source_message_id: string;
};
export type Artifact = {
  id: string;
  title: string;
  agent_prompt: string;
  blueprint: unknown;
  accepted: boolean;
  parent_id?: string;
  source_message_id: string;
};
export type Verdict = "PASS" | "FAIL" | "UNKNOWN";
export type CaseResult = {
  case_key: string;
  version: string;
  input: unknown;
  output: string;
  verdict: Verdict;
  checks: {
    key: string;
    verdict: Verdict;
    evidence: string;
    error?: { message: string };
  }[];
  error?: { message: string };
};
export type Operation = {
  baseline_id?: string;
  id: string;
  kind: string;
  state: string;
  billing: string;
  models: Models;
  max_cost_nano_usd: number;
  actual_cost_nano_usd: number | null;
  error?: { code: string; message: string };
  results: CaseResult[];
  scorecard?: {
    passed: number;
    failed: number;
    unknown: number;
    total: number;
    evaluated: number;
    pass_rate: number | null;
    coverage: number;
    checks_expected?: number;
    checks_evaluated?: number;
    incomplete_cases?: number;
  };
};
export type Session = {
  event_cursor?: number;
  id: string;
  revision: number;
  anonymous: boolean;
  workspace_id?: string;
  saved_draft_id?: string;
  saved_artifact_id?: string;
  saved_models?: Models;
  document: {
    messages: { id: string; role: string; content: string }[];
    requirements: Requirement[];
    artifacts: Artifact[];
    models: Models;
    active_artifact_id?: string;
  };
  operations: Operation[];
};
export type VibeConfig = {
  enabled: boolean;
  models: Model[];
  defaults: Models;
};
export const defaultModels: Models = {
  assistant: "openai/gpt-4.1-mini",
  target: "openai/gpt-4.1-mini",
  evaluator: "openai/gpt-4.1-mini",
};
export const terminal = (state: string) =>
  ["COMPLETED", "PARTIAL", "FAILED", "CANCELLED", "EXPIRED"].includes(state);
export const dollars = (nano: number | null) =>
  nano === null
    ? "Reconciling"
    : new Intl.NumberFormat("en-US", {
        style: "currency",
        currency: "USD",
        minimumFractionDigits: 2,
        maximumFractionDigits: 4,
      }).format(nano / 1e9);

export class VibeError extends Error {
  constructor(
    public code: string,
    message: string,
    public status?: number,
  ) {
    super(message);
  }
}
function baseURL() {
  return (
    process.env.NEXT_PUBLIC_API_URL ||
    (typeof window !== "undefined" &&
    (window.location.hostname === "agentclash.dev" ||
      window.location.hostname.endsWith(".agentclash.dev"))
      ? "https://api.agentclash.dev"
      : "http://localhost:8080")
  ).replace(/\/$/, "");
}
export async function vibeFetch<T>(
  path: string,
  token?: string | null,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (!(options.body instanceof Blob))
    headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const response = await fetch(`${baseURL()}/v1/vibe${path}`, {
    ...options,
    headers,
    credentials: "include",
    cache: "no-store",
  });
  const result = await response.json();
  if (!response.ok)
    throw new VibeError(
      result.error?.code || "request_failed",
      result.error?.message || "Could not complete the request.",
      response.status,
    );
  return result;
}

// Parser consumes complete SSE frames only. A fragmented UTF-8 character or
// stale cursor cannot submit a message or restart a check.
export async function watchVibe(
  id: string,
  token: string | null | undefined,
  signal: AbortSignal,
  onSnapshot: (session: Session) => void,
): Promise<void> {
  const headers = new Headers();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const response = await fetch(`${baseURL()}/v1/vibe/sessions/${id}/events`, {
    credentials: "include",
    headers,
    signal,
    cache: "no-store",
  });
  if (!response.ok || !response.body)
    throw new Error("Connection interrupted. Reconnecting to saved progress…");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      if (buffer.length > 32 * 1024 * 1024)
        throw new Error("Event snapshot exceeds the client limit.");
      let end: number;
      while ((end = buffer.indexOf("\n\n")) >= 0) {
        const frame = buffer.slice(0, end);
        buffer = buffer.slice(end + 2);
        if (!frame.includes("event: snapshot")) continue;
        const data = frame
          .split("\n")
          .filter((line) => line.startsWith("data: "))
          .map((line) => line.slice(6))
          .join("\n");
        if (data) onSnapshot(JSON.parse(data) as Session);
      }
    }
  } finally {
    reader.releaseLock();
  }
}
