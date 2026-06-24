"use client";

import { type FormEvent, useMemo, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type {
  AgentHarness,
  AgentHarnessExecution,
  AgentHarnessExecutionEvent,
} from "@/lib/api/types";
import { useApiListQuery, useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import {
  Activity,
  AlertCircle,
  Bot,
  CheckCircle2,
  ChevronDown,
  Clock3,
  Loader2,
  MessageSquare,
  PackageCheck,
  Send,
  Settings2,
  TerminalSquare,
} from "lucide-react";
import { CreateAgentHarnessDialog } from "./create-agent-harness-dialog";

const authLabel: Record<string, string> = {
  api_key_secret: "API key secret",
};

const statusVariant: Record<string, "default" | "secondary" | "outline"> = {
  active: "default",
  draft: "outline",
  archived: "secondary",
  completed: "default",
  failed: "secondary",
  cancelled: "secondary",
  queued: "outline",
  provisioning: "outline",
  running: "default",
  scoring: "default",
};

const phaseOrder = [
  "sandbox",
  "repository",
  "setup",
  "agent",
  "diff",
  "validation",
  "pull-request",
] as const;

type HarnessPhase = (typeof phaseOrder)[number];

type HarnessPhaseState = "waiting" | "running" | "done" | "failed";

const phaseLabels: Record<HarnessPhase, string> = {
  sandbox: "Sandbox",
  repository: "Repository",
  setup: "Setup",
  agent: "Agent work",
  diff: "Diff",
  validation: "Validation",
  "pull-request": "Pull request",
};

export function AgentHarnessesClient({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();
  const [runningHarnessId, setRunningHarnessId] = useState<string | null>(null);
  const [selectedHarnessId, setSelectedHarnessId] = useState<string>("");
  const [chatMessage, setChatMessage] = useState("");
  const [expandedExecutionId, setExpandedExecutionId] = useState<string | null>(
    null,
  );
  const [runError, setRunError] = useState<string | null>(null);
  const { data, error, isLoading } = useApiListQuery<AgentHarness>(
    `/v1/workspaces/${workspaceId}/agent-harnesses`,
  );
  const { data: executionsData } = useApiListQuery<AgentHarnessExecution>(
    `/v1/workspaces/${workspaceId}/agent-harness-executions`,
    undefined,
    { refreshInterval: 2500 },
  );
  const harnesses = data?.items ?? [];
  const selectedHarness =
    harnesses.find((harness) => harness.id === selectedHarnessId) ??
    harnesses[0];
  const latestExecutionByHarness = useMemo(() => {
    const latest = new Map<string, AgentHarnessExecution>();
    for (const execution of executionsData?.items ?? []) {
      const current = latest.get(execution.agent_harness_id);
      if (!current || execution.created_at > current.created_at) {
        latest.set(execution.agent_harness_id, execution);
      }
    }
    return latest;
  }, [executionsData?.items]);
  const latestExecution = selectedHarness
    ? latestExecutionByHarness.get(selectedHarness.id)
    : undefined;
  const latestEvent = latestExecution
    ? latestAgentHarnessEvent(latestExecution)
    : undefined;
  const failureMessage = latestExecution
    ? executionFailureMessage(latestExecution)
    : "";
  const isRunning = selectedHarness
    ? runningHarnessId === selectedHarness.id
    : false;

  async function startHarnessExecution(harnessId: string, message?: string) {
    setRunError(null);
    setRunningHarnessId(harnessId);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post(
        `/v1/workspaces/${workspaceId}/agent-harnesses/${harnessId}/executions`,
        message && message.trim() ? { message } : {},
      );
      await Promise.all([
        mutate(workspaceResourceKeys.agentHarnesses(workspaceId)),
        mutate(workspaceResourceKeys.agentHarnessExecutions(workspaceId)),
      ]);
      return true;
    } catch (err) {
      setRunError(
        err instanceof Error ? err.message : "Failed to start agent harness",
      );
      return false;
    } finally {
      setRunningHarnessId(null);
    }
  }

  async function handleChatSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedHarness || !chatMessage.trim()) return;
    const started = await startHarnessExecution(selectedHarness.id, chatMessage);
    if (started) setChatMessage("");
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Agent Harnesses"
        breadcrumbs={[{ label: "Agent Harnesses" }]}
        actions={<CreateAgentHarnessDialog workspaceId={workspaceId} />}
      />

      {runError ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          {runError}
        </div>
      ) : null}

      {isLoading && !data ? (
        <WorkspaceListLoading rows={6} />
      ) : error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load agent harnesses.
        </div>
      ) : harnesses.length === 0 ? (
        <EmptyState
          icon={<PackageCheck className="size-10" />}
          title="No agent harnesses yet"
          description="Create a coding harness to evaluate long-running autonomous work without writing an eval pack."
        />
      ) : (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
          <section className="min-w-0 rounded-lg border border-border bg-card">
            <div className="border-b border-border px-4 py-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <MessageSquare className="size-4 text-muted-foreground" />
                    Chat with a coding harness
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Describe a task, paste a GitHub issue URL, or ask for a
                    follow-up on the latest run.
                  </p>
                </div>
                {selectedHarness ? (
                  <Badge variant={statusVariant[selectedHarness.status] ?? "outline"}>
                    {selectedHarness.status}
                  </Badge>
                ) : null}
              </div>
            </div>

            <div className="space-y-4 p-4">
              <HarnessSelector
                harnesses={harnesses}
                selectedHarness={selectedHarness}
                onSelect={setSelectedHarnessId}
              />

              <div className="rounded-lg border border-border bg-background p-4">
                <div className="flex items-start gap-3">
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                    <Bot className="size-4" />
                  </div>
                  <div className="min-w-0 flex-1 space-y-3">
                    <div>
                      <div className="text-sm font-medium">
                        {selectedHarness?.name ?? "Select a harness"}
                      </div>
                      <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">
                        {selectedHarness?.task_prompt ??
                          "Pick a harness, then describe the work you want it to attempt."}
                      </p>
                    </div>
                    <form onSubmit={handleChatSubmit} className="space-y-2">
                      <textarea
                        aria-label="Agent harness task message"
                        value={chatMessage}
                        onChange={(event) => setChatMessage(event.target.value)}
                        spellCheck={false}
                        className="min-h-28 w-full resize-y rounded-lg border border-input bg-transparent px-3 py-2 text-sm leading-relaxed focus:outline-none focus:ring-2 focus:ring-ring/50"
                        placeholder="Ask this harness to inspect, patch, test, or paste a GitHub issue URL..."
                      />
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div className="text-xs text-muted-foreground">
                          Sends as a normal harness run message. Advanced setup stays in New Harness.
                        </div>
                        <Button
                          type="submit"
                          disabled={
                            !selectedHarness ||
                            !chatMessage.trim() ||
                            runningHarnessId === selectedHarness.id
                          }
                        >
                          {isRunning ? (
                            <Loader2
                              data-icon="inline-start"
                              className="size-4 animate-spin"
                            />
                          ) : (
                            <Send data-icon="inline-start" className="size-4" />
                          )}
                          {isRunning ? "Sending..." : "Send"}
                        </Button>
                      </div>
                    </form>
                  </div>
                </div>
              </div>

              {latestExecution ? (
                <LatestRunPanel
                  execution={latestExecution}
                  latestEvent={latestEvent}
                  failureMessage={failureMessage}
                  isExpanded={expandedExecutionId === latestExecution.id}
                  onToggleExpanded={() =>
                    setExpandedExecutionId((current) =>
                      current === latestExecution.id ? null : latestExecution.id,
                    )
                  }
                />
              ) : (
                <div className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
                  No runs yet for this harness. Send a task to start watching live activity here.
                </div>
              )}
            </div>
          </section>

          <aside className="space-y-4">
            <div className="rounded-lg border border-border bg-card p-4">
              <div className="flex items-center gap-2 text-sm font-medium">
                <Settings2 className="size-4 text-muted-foreground" />
                Setup context
              </div>
              {selectedHarness ? (
                <dl className="mt-3 space-y-3 text-sm">
                  <ContextRow
                    label="Auth"
                    value={authLabel[selectedHarness.auth_mode] ?? selectedHarness.auth_mode}
                  />
                  <ContextRow
                    label="Runner"
                    value={formatHarnessRunner(selectedHarness)}
                  />
                  <ContextRow
                    label="Repository"
                    value={formatHarnessRepository(selectedHarness)}
                  />
                  <ContextRow
                    label="Base branch"
                    value={selectedHarness.base_branch ?? "main"}
                  />
                  <ContextRow
                    label="Updated"
                    value={formatDateTime(selectedHarness.updated_at)}
                  />
                </dl>
              ) : null}
            </div>

            <div className="rounded-lg border border-border bg-card p-4">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div className="text-sm font-medium">Harnesses</div>
                <Badge variant="outline">{harnesses.length}</Badge>
              </div>
              <div className="space-y-2">
                {harnesses.map((harness) => (
                  <button
                    key={harness.id}
                    type="button"
                    onClick={() => setSelectedHarnessId(harness.id)}
                    className={`w-full rounded-lg border p-3 text-left text-sm transition-colors ${
                      selectedHarness?.id === harness.id
                        ? "border-primary bg-primary/5"
                        : "border-border hover:bg-muted/50"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate font-medium">{harness.name}</span>
                      <Badge variant={statusVariant[harness.status] ?? "outline"}>
                        {harness.status}
                      </Badge>
                    </div>
                    <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">
                      {harness.task_prompt}
                    </div>
                  </button>
                ))}
              </div>
            </div>
          </aside>
        </div>
      )}
    </div>
  );
}

function HarnessSelector({
  harnesses,
  selectedHarness,
  onSelect,
}: {
  harnesses: AgentHarness[];
  selectedHarness?: AgentHarness;
  onSelect: (harnessId: string) => void;
}) {
  return (
    <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
      <label className="text-sm font-medium" htmlFor="agent-harness-select">
        Active harness
      </label>
      <select
        id="agent-harness-select"
        value={selectedHarness?.id ?? ""}
        onChange={(event) => onSelect(event.target.value)}
        className="h-10 min-w-0 rounded-lg border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50 md:w-80"
      >
        {harnesses.map((harness) => (
          <option key={harness.id} value={harness.id}>
            {harness.name}
          </option>
        ))}
      </select>
    </div>
  );
}

function LatestRunPanel({
  execution,
  latestEvent,
  failureMessage,
  isExpanded,
  onToggleExpanded,
}: {
  execution: AgentHarnessExecution;
  latestEvent?: AgentHarnessExecutionEvent;
  failureMessage: string;
  isExpanded: boolean;
  onToggleExpanded: () => void;
}) {
  return (
    <div className="rounded-lg border border-border bg-background">
      <div className="border-b border-border p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-medium">
              <Activity className="size-4 text-muted-foreground" />
              Live activity
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              Started {formatDateTime(execution.created_at)}
            </p>
          </div>
          <Badge variant={statusVariant[execution.status] ?? "outline"}>
            {execution.status}
          </Badge>
        </div>
      </div>

      <div className="space-y-4 p-4">
        {execution.status === "failed" ? (
          <FailureNotice
            message={failureMessage}
            stage={execution.failure_stage}
          />
        ) : null}

        <PhaseProgress execution={execution} />

        {latestEvent ? (
          <div className="rounded-lg border border-border p-3">
            <div className="flex flex-wrap items-center gap-2 text-sm">
              <TerminalSquare className="size-4 text-muted-foreground" />
              <span className="font-medium">
                {formatEventType(latestEvent.event_type)}
              </span>
            </div>
            <p className="mt-2 whitespace-pre-wrap break-words text-sm text-muted-foreground">
              {eventSummary(latestEvent)}
            </p>
            <div className="mt-2 text-xs text-muted-foreground">
              {formatDateTime(latestEvent.occurred_at)}
            </div>
          </div>
        ) : (
          <div className="rounded-lg border border-dashed border-border p-3 text-sm text-muted-foreground">
            Waiting for the first execution event...
          </div>
        )}

        <Button
          type="button"
          variant="outline"
          onClick={onToggleExpanded}
          aria-label="Show summarized activity details"
        >
          <ChevronDown
            data-icon="inline-start"
            className={`size-4 transition-transform ${isExpanded ? "rotate-180" : ""}`}
          />
          {isExpanded ? "Hide summarized details" : "Show summarized details"}
        </Button>

        {isExpanded ? <AgentHarnessExecutionTimeline execution={execution} /> : null}
      </div>
    </div>
  );
}

function FailureNotice({
  message,
  stage,
}: {
  message: string;
  stage?: AgentHarnessExecution["failure_stage"];
}) {
  const title = stage ? `${failureStageLabel(stage)} failed` : "This run needs attention";
  return (
    <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-3">
      <div className="flex items-start gap-2">
        <AlertCircle className="mt-0.5 size-4 shrink-0 text-destructive" />
        <div className="min-w-0">
          <div className="text-sm font-medium text-destructive">
            {title}
          </div>
          <p className="mt-1 whitespace-pre-wrap break-words text-sm text-destructive/90">
            {message || "Open the summarized activity details and check the latest failed step."}
          </p>
        </div>
      </div>
    </div>
  );
}

function failureStageLabel(stage: NonNullable<AgentHarnessExecution["failure_stage"]>) {
  switch (stage) {
    case "setup":
      return "Setup";
    case "agent":
      return "Agent";
    case "validator":
      return "Validation";
    case "repository":
      return "Repository";
    case "infrastructure":
      return "Infrastructure";
  }
}

function PhaseProgress({ execution }: { execution: AgentHarnessExecution }) {
  const phases = phaseStates(execution);
  return (
    <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
      {phaseOrder.map((phase) => (
        <div
          key={phase}
          className="flex items-center gap-2 rounded-lg border border-border p-2 text-sm"
        >
          {phaseIcon(phases[phase])}
          <span className="min-w-0 flex-1 truncate">{phaseLabels[phase]}</span>
          <span className="text-xs capitalize text-muted-foreground">
            {phases[phase]}
          </span>
        </div>
      ))}
    </div>
  );
}

function AgentHarnessExecutionTimeline({
  execution,
}: {
  execution: AgentHarnessExecution;
}) {
  const events = executionEvents(execution);
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="font-medium">Summarized activity details</span>
        <span className="text-xs text-muted-foreground">
          {events.length} {events.length === 1 ? "event" : "events"}
        </span>
      </div>
      {events.length === 0 ? (
        <div className="rounded-md border border-dashed border-border p-3 text-sm text-muted-foreground">
          No execution events recorded yet.
        </div>
      ) : (
        <ol className="space-y-2">
          {events.map((event) => (
            <li
              key={event.id}
              className="grid gap-3 rounded-md border border-border p-3 md:grid-cols-[10rem_1fr]"
            >
              <div className="space-y-1">
                <div className="text-xs font-medium text-muted-foreground">
                  #{event.sequence_number} · {event.actor_type}
                </div>
                <div className="text-xs text-muted-foreground">
                  {formatDateTime(event.occurred_at)}
                </div>
              </div>
              <div className="min-w-0 space-y-1">
                <div className="text-sm font-medium">
                  {formatEventType(event.event_type)}
                </div>
                <p className="whitespace-pre-wrap break-words text-sm text-muted-foreground">
                  {eventSummary(event)}
                </p>
              </div>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

function ContextRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="max-w-48 truncate text-right font-medium">{value}</dd>
    </div>
  );
}

function latestAgentHarnessEvent(execution: AgentHarnessExecution) {
  return executionEvents(execution).at(-1);
}

function executionEvents(execution: AgentHarnessExecution) {
  return [...(execution.events ?? [])].sort(
    (a, b) => a.sequence_number - b.sequence_number,
  );
}

function formatHarnessRunner(harness: AgentHarness) {
  return harness.codex_model
    ? `${harness.codex_template} / ${harness.codex_model}`
    : harness.codex_template;
}

function formatHarnessRepository(harness: AgentHarness) {
  return (
    harness.repository_full_name ??
    parseRepositoryName(harness.repository_url ?? "") ??
    "Not configured"
  );
}

function formatEventType(eventType: string) {
  return eventType
    .split(".")
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" · ");
}

function eventSummary(event: AgentHarnessExecutionEvent) {
  const payload = payloadObject(event.payload);
  const preferredKeys = eventSummaryKeys(event.event_type);
  const lines = preferredKeys
    .flatMap((key) => {
      if (!(key in payload)) return [];
      return [`${formatPayloadKey(key)}: ${formatPayloadValue(payload[key])}`];
    })
    .filter(Boolean);
  if (lines.length > 0) return lines.join("\n");
  return `${event.actor_type} emitted ${event.event_type}`;
}

function eventSummaryKeys(eventType: string) {
  if (eventType === "codex.exec.output" || eventType === "claude.exec.output") {
    return ["type", "decision", "summary", "tool", "exit_code"];
  }
  if (
    eventType.startsWith("artifact.") ||
    eventType.startsWith("git.diff") ||
    eventType.startsWith("validator.command.exec")
  ) {
    return [
      "changed_files",
      "working_directory",
      "exit_code",
      "score",
      "passed",
      "failed",
      "skipped",
    ];
  }
  return [
    "type",
    "message",
    "decision",
    "summary",
    "result",
    "error",
    "command",
    "tool",
    "exit_code",
    "overall_score",
    "score",
    "passed",
    "failed",
    "skipped",
    "changed_files",
    "working_directory",
  ];
}

function executionFailureMessage(execution: AgentHarnessExecution) {
  if (execution.error_message?.trim()) return execution.error_message.trim();
  const failedEvent = [...executionEvents(execution)]
    .reverse()
    .find((event) => event.event_type.endsWith(".failed"));
  if (!failedEvent) return "";
  const payload = payloadObject(failedEvent.payload);
  for (const key of ["error", "message"] as const) {
    const value = payload[key];
    if (typeof value === "string" && value.trim()) return truncateText(value, 220);
  }
  return "";
}

function phaseStates(execution: AgentHarnessExecution) {
  const states: Record<HarnessPhase, HarnessPhaseState> = {
    sandbox: "waiting",
    repository: "waiting",
    setup: "waiting",
    agent: "waiting",
    diff: "waiting",
    validation: "waiting",
    "pull-request": "waiting",
  };
  for (const event of executionEvents(execution)) {
    const phase = eventPhase(event.event_type);
    if (!phase) continue;
    if (event.event_type.endsWith(".failed")) {
      states[phase] = "failed";
    } else if (event.event_type.endsWith(".started")) {
      states[phase] = states[phase] === "done" ? "done" : "running";
    } else if (
      event.event_type.endsWith(".completed") ||
      event.event_type.endsWith(".passed") ||
      event.event_type.endsWith(".created")
    ) {
      states[phase] = "done";
    } else if (states[phase] === "waiting") {
      states[phase] = "running";
    }
  }
  if (execution.status === "completed") {
    for (const phase of phaseOrder) {
      if (states[phase] === "running") states[phase] = "done";
    }
  }
  return states;
}

function eventPhase(eventType: string): HarnessPhase | null {
  if (eventType.startsWith("sandbox.")) return "sandbox";
  if (eventType.startsWith("repository.") || eventType.startsWith("github.git_auth")) {
    return "repository";
  }
  if (eventType.startsWith("setup.")) return "setup";
  if (
    eventType.startsWith("codex.") ||
    eventType.startsWith("claude.") ||
    eventType.includes(".exec")
  ) {
    return "agent";
  }
  if (eventType.startsWith("git.") || eventType.startsWith("artifact.")) {
    return "diff";
  }
  if (
    eventType.startsWith("validator.") ||
    eventType.startsWith("scoring.") ||
    eventType.startsWith("scorecard.") ||
    eventType.startsWith("llm_judges.")
  ) {
    return "validation";
  }
  if (eventType.startsWith("github.pull_request.")) return "pull-request";
  return null;
}

function phaseIcon(state: HarnessPhaseState) {
  switch (state) {
    case "done":
      return <CheckCircle2 className="size-4 text-emerald-500" />;
    case "failed":
      return <AlertCircle className="size-4 text-destructive" />;
    case "running":
      return <Loader2 className="size-4 animate-spin text-primary" />;
    case "waiting":
      return <Clock3 className="size-4 text-muted-foreground" />;
  }
}

function payloadObject(payload: unknown): Record<string, unknown> {
  if (payload && typeof payload === "object" && !Array.isArray(payload)) {
    return payload as Record<string, unknown>;
  }
  return {};
}

function formatPayloadKey(key: string) {
  return key
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function formatPayloadValue(value: unknown) {
  if (Array.isArray(value)) {
    return truncateText(value.map((item) => String(item)).join(" "), 180);
  }
  if (typeof value === "string") {
    return truncateText(value.trim() || "(empty)", 180);
  }
  if (value === null || value === undefined) {
    return "(empty)";
  }
  if (typeof value === "object") {
    return "(summary unavailable)";
  }
  return String(value);
}

function truncateText(value: string, maxLength: number) {
  if (value.length <= maxLength) return value;
  return `${value.slice(0, maxLength - 1)}...`;
}

function formatDateTime(value: string) {
  return new Date(value).toLocaleString();
}

function parseRepositoryName(repositoryURL: string) {
  const trimmed = repositoryURL.trim().replace(/\.git$/i, "");
  if (!trimmed) return undefined;
  const scpPath = trimmed.match(/^[^@]+@[^:]+:(.+)$/)?.[1];
  if (scpPath) {
    const segments = scpPath.split("/").filter(Boolean);
    return segments.length >= 2 ? segments.slice(-2).join("/") : undefined;
  }
  try {
    const url = new URL(trimmed);
    const segments = url.pathname.split("/").filter(Boolean);
    return segments.length >= 2 ? segments.slice(-2).join("/") : undefined;
  } catch {
    const segments = trimmed.split("/").filter(Boolean);
    return segments.length >= 2 ? segments.slice(-2).join("/") : undefined;
  }
}
