import type { ConversationState, ConversationChange } from "./vibe-conversation";

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
  status: "proposed" | "accepted" | "rejected" | "superseded" | "provided";
  source_message_id: string;
  proposal_message_id?: string;
  proposed_by?: string;
  accepted_by?: string;
  accepted_at?: string;
  supersedes_id?: string;
  change?: "add" | "replace" | "remove";
};
export type Journey = {
  mode?: "exploring" | "idea" | "existing";
  stack?: string;
  evidence?: string;
  preview_consent?: boolean;
};
export type Capability = {
  id: string;
  label: string;
  available: boolean;
  description: string;
  instructions?: string;
  criteria_instructions?: string;
  url?: string;
  example?: string;
  next_steps?: string[];
};
export type TestPlan = {
  title: string;
  objective: string;
  scenarios: { input: string; expected: string }[];
  evidence_needed: string[];
  next_steps: string[];
  local_test_code: string;
};
export type EvaluationProposal = {
  examples: string[];
  scenarios?: { input: string; expected: string }[];
  success_criteria: string;
};
export type Artifact = {
  policy_id?: string;
  validation?: {
    status: "supported" | "contradicted" | "unclear" | "unavailable";
    blueprint_hash: string;
    policy_hash: string;
    validator_version: string;
  };
  dismissed?: boolean;
  proposal_message_id?: string;
  quick_check?: boolean;
  conversation_evaluation?: {
    evidence_set_id: string;
    expectations: Expectation[];
  };
  summary?: string;
  criteria_requirement_ids?: string[];
  kind?: "agent_draft" | "test_plan" | "conversation_evaluation" | "test_suite";
  test_plan?: TestPlan;
  proposal?: EvaluationProposal & { title: string; agent_prompt: string };
  id: string;
  title: string;
  agent_prompt: string;
  blueprint: unknown;
  accepted: boolean;
  parent_id?: string;
  source_message_id: string;
  created_at?: string;
};
export type Verdict = "PASS" | "FAIL" | "UNKNOWN";
export type CaseResult = {
  expectations?: Expectation[];
  title?: string;
  messages?: EvidenceMessage[];
  expected?: string;
  expected_scope?: "shared";
  case_key: string;
  version: string;
  input: unknown;
  output: string;
  verdict: Verdict;
  checks: {
    key: string;
    verdict: Verdict;
    evidence: string;
    message_ids?: string[];
    error?: { message: string };
  }[];
  error?: { code?: string; message: string };
};
export type Operation = {
  completion_receipt?: {
    action: string;
    source_message_id: string;
    artifact_id?: string;
    case_count: number;
    changed_case_count: number;
  };
  retry_of_operation_id?: string;
  retryable?: boolean;
  conversation_decision?: { intent: string; source_message_id: string };
  source?: {
    kind: "provided_conversations" | "prompt";
    label: string;
    artifact_id: string;
    evidence_set_id?: string;
    comparison?: "updated_replies" | "rechecked";
  };
  baseline_id?: string;
  id: string;
  kind: string;
  state: string;
  billing: string;
  models: Models;
  max_cost_nano_usd: number;
  actual_cost_nano_usd: number | null;
  error?: {
    code: string;
    message: string;
    context?: {
      upper_bound: number;
      limit: number;
      largest_section: string;
      sections: Record<string, number>;
    };
  };
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
    conversation_state?: ConversationState;
    last_change?: ConversationChange;
    policies?: {
      id: string;
      source_version?: string;
      rules: { id: string; statement: string }[];
    }[];
    pending_policy_changes?: {
      operation_id: string;
      source_message_id: string;
      artifact_id?: string;
      status: "pending" | "applied";
      message: string;
    }[];
    test_journey?: boolean;
    evaluation_first?: boolean;
    evidence_sets?: EvidenceSet[];
    active_evidence_id?: string;
    journey?: Journey;
    messages: {
      id: string;
      role: string;
      content: string;
      cards?: unknown[];
      origin?: string;
      operation_id?: string;
      artifact_id?: string;
      preview_thread_id?: string;
    }[];
    requirements: Requirement[];
    artifacts: Artifact[];
    models: Models;
    active_artifact_id?: string;
  };
  operations: Operation[];
};
export type Expectation = { id: string; statement: string };
export type EvidenceMessage = {
  id: string;
  role: "user" | "assistant" | "system" | "tool" | "unknown";
  content: string;
};
export type EvidenceSet = {
  id: string;
  parent_id?: string;
  label: string;
  raw: string;
  context?: string;
  conversations: { key: string; title: string; messages: EvidenceMessage[] }[];
};
export type SavedCheck = {
  draft_id?: string;
  id: string;
  title: string;
  session_id: string;
  artifact_id: string;
  baseline_operation_id: string;
  workspace_id: string;
  source: Operation["source"];
  created_at: string;
};
export type VibeConfig = {
  interaction_actions?: boolean;
  capabilities?: Capability[];
  enabled: boolean;
  free_only?: boolean;
  local_testing?: boolean;
  models: Model[];
  defaults: Models;
  anonymous_limits?: Record<string, number>;
  signed_in_limits?: Record<string, number>;
  trial_budget_nano_usd?: number;
};

// Only the generated single-judge preview is edited here. Imported contracts
// retain their complete raw representation and use the advanced builder.
export function editableEvaluation(
  blueprint: unknown,
): EvaluationProposal | null {
  if (!blueprint || typeof blueprint !== "object") return null;
  const b = blueprint as {
    cases?: {
      payload?: { question?: unknown };
      expectations?: { key: string; kind: string; value?: unknown }[];
    }[];
    judges?: {
      key?: string;
      mode?: string;
      rubric?: string;
      assertion?: unknown;
      context_from?: string[];
    }[];
    validators?: { key?: string; type?: string; expected_from?: string }[];
    dimensions?: unknown[];
  };
  if (
    !Array.isArray(b.cases) ||
    !b.cases.length ||
    !b.cases.every((c) => typeof c.payload?.question === "string") ||
    !Array.isArray(b.judges) ||
    b.judges.length !== 1 ||
    b.judges[0].key !== "behavior" ||
    (b.judges[0].mode !== undefined && b.judges[0].mode !== "assertion") ||
    !!b.judges[0].rubric ||
    typeof b.judges[0].assertion !== "string" ||
    !Array.isArray(b.validators) ||
    b.validators.length !== 1 ||
    b.validators[0].key !== "has_answer" ||
    b.dimensions?.length !== 2
  )
    return null;
  const only = (value: object, keys: string[]) =>
    Object.keys(value).every((key) => keys.includes(key));
  // Custom contracts remain read-only. The backend also requires a lossless
  // round trip before accepting edits, including fields unknown to this client.
  if (
    b.cases.some(
      (c) =>
        !only(c, ["key", "payload", "expectations"]) ||
        !only(c.payload!, ["question"]),
    ) ||
    !only(b.judges[0], [
      "key",
      "mode",
      "rubric",
      "assertion",
      "context_from",
    ]) ||
    b.validators.some(
      (v) => !only(v, ["key", "type", "target", "expected_from"]),
    ) ||
    b.dimensions.some(
      (d) =>
        !d ||
        typeof d !== "object" ||
        !only(d, ["key", "source", "validators", "judge_key"]),
    )
  )
    return null;
  const contextual = b.judges[0].context_from;
  if (contextual?.length) {
    if (
      contextual.length !== 1 ||
      contextual[0] !== "case.expectations.expected_behavior" ||
      !b.cases.every(
        (c) =>
          c.expectations?.length === 1 &&
          c.expectations[0].key === "expected_behavior" &&
          only(c.expectations[0], ["key", "kind", "value"]) &&
          c.expectations[0].kind === "text" &&
          typeof c.expectations[0].value === "string",
      )
    )
      return null;
    return {
      examples: [],
      scenarios: b.cases.map((c) => ({
        input: c.payload!.question as string,
        expected: c.expectations![0].value as string,
      })),
      success_criteria: b.judges[0].assertion.startsWith(scenarioCriteriaPrefix)
        ? b.judges[0].assertion.slice(scenarioCriteriaPrefix.length)
        : b.judges[0].assertion,
    };
  }
  if (b.cases.some((c) => c.expectations?.length)) return null;
  return {
    examples: b.cases.map((c) => c.payload!.question as string),
    success_criteria: b.judges[0].assertion,
  };
}
const scenarioCriteriaPrefix =
  "The response meets this case's expected_behavior and the following shared rules:\n\n";

export function caseInput(input: unknown): string {
  if (typeof input === "string") return input;
  if (
    input &&
    typeof input === "object" &&
    Object.keys(input).length === 1 &&
    "question" in input &&
    typeof input.question === "string"
  )
    return input.question;
  return input == null
    ? "Input has not been loaded."
    : JSON.stringify(input, null, 2);
}

export function exportAgent(artifact: Artifact, models: Models) {
  downloadVibe(
    "agentclash-agent.json",
    JSON.stringify(
      {
        format: "agentclash-vibe-v1",
        agent_prompt: artifact.agent_prompt,
        evaluation: artifact.blueprint,
        models,
      },
      null,
      2,
    ),
  );
}
export function downloadVibe(
  name: string,
  text: string,
  type = "application/json",
) {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  anchor.click();
  URL.revokeObjectURL(url);
}
export const defaultModels: Models = {
  assistant: "openai/gpt-4.1-mini",
  target: "openai/gpt-4.1-mini",
  evaluator: "openai/gpt-4.1-mini",
};
export const terminal = (state: string) =>
  ["COMPLETED", "PARTIAL", "FAILED", "CANCELLED", "EXPIRED"].includes(state);
export function completionAcknowledgement(operation: Operation): string | undefined {
  const receipt = operation.completion_receipt;
  if (!receipt) return undefined;
  if (receipt.action === "prepare_tests")
    return `${receipt.case_count} ${receipt.case_count === 1 ? "test is" : "tests are"} ready.`;
  if (receipt.action === "edit_tests")
    return receipt.changed_case_count > 0
      ? `Updated ${receipt.changed_case_count} ${receipt.changed_case_count === 1 ? "test" : "tests"}.`
      : "Updated the test rules.";
  if (receipt.action === "suggest_fix") return "Prepared a suggested fix.";
  return undefined;
}

export function retryVibeOperation(
  sessionID: string,
  operationID: string,
  request: { client_id: string; revision: number },
  token?: string | null,
) {
  return vibeFetch<Operation>(
    `/sessions/${encodeURIComponent(sessionID)}/operations/${encodeURIComponent(operationID)}/retry`,
    token,
    { method: "POST", body: JSON.stringify(request) },
  );
}
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
export function resolveVibeBaseURL(
  configured: string | undefined,
  hostname?: string,
) {
  const base = (
    configured ||
    (hostname === "agentclash.dev" || hostname?.endsWith(".agentclash.dev")
      ? "https://api.agentclash.dev"
      : "http://localhost:8080")
  ).replace(/\/$/, "");
  // SameSite=Strict trial cookies require the UI and API to use the same
  // loopback hostname. localhost and 127.0.0.1 are different browser sites.
  const loopback = (host?: string): host is "localhost" | "127.0.0.1" =>
    host === "localhost" || host === "127.0.0.1";
  if (loopback(hostname)) {
    const url = new URL(base);
    if (loopback(url.hostname)) {
      url.hostname = hostname;
      return url.toString().replace(/\/$/, "");
    }
  }
  return base;
}
function baseURL() {
  return resolveVibeBaseURL(
    process.env.NEXT_PUBLIC_API_URL,
    typeof window !== "undefined" ? window.location.hostname : undefined,
  );
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
  if (!response.ok) {
    const result = await response.json().catch(() => null);
    throw new VibeError(
      result?.error?.code || "request_failed",
      result?.error?.message || "Connection interrupted.",
      response.status,
    );
  }
  if (!response.body)
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
