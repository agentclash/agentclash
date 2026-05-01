"use client";

import { Fragment, type FormEvent, useMemo, useState } from "react";
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
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/ui/page-header";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Activity,
  ChevronDown,
  Loader2,
  MessageSquare,
  PackageCheck,
  Play,
  Send,
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

export function AgentHarnessesClient({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();
  const [runningHarnessId, setRunningHarnessId] = useState<string | null>(null);
  const [chatHarnessId, setChatHarnessId] = useState<string>("");
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
  const selectedChatHarness =
    harnesses.find((harness) => harness.id === chatHarnessId) ?? harnesses[0];
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

  async function startHarnessExecution(harnessId: string, message?: string) {
    setRunError(null);
    setRunningHarnessId(harnessId);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post(
        `/v1/workspaces/${workspaceId}/agent-harnesses/${harnessId}/executions`,
        message?.trim() ? { message: message.trim() } : {},
      );
      await Promise.all([
        mutate(workspaceResourceKeys.agentHarnesses(workspaceId)),
        mutate(workspaceResourceKeys.agentHarnessExecutions(workspaceId)),
      ]);
    } catch (err) {
      setRunError(
        err instanceof Error ? err.message : "Failed to start agent harness",
      );
    } finally {
      setRunningHarnessId(null);
    }
  }

  async function handleChatSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedChatHarness || !chatMessage.trim()) return;
    await startHarnessExecution(selectedChatHarness.id, chatMessage);
    setChatMessage("");
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Agent Harnesses"
        breadcrumbs={[{ label: "Agent Harnesses" }]}
        actions={<CreateAgentHarnessDialog workspaceId={workspaceId} />}
      />

      {runError ? (
        <div className="mb-4 rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
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
          description="Create a Codex harness to evaluate long-running autonomous coding tasks without writing a challenge pack."
        />
      ) : (
        <>
          <form
            onSubmit={handleChatSubmit}
            className="space-y-3 rounded-lg border border-border bg-card p-4"
          >
            <div className="flex flex-col gap-3 md:flex-row md:items-center">
              <div className="flex min-w-0 flex-1 items-center gap-2">
                <MessageSquare className="size-4 text-muted-foreground" />
                <select
                  value={selectedChatHarness?.id ?? ""}
                  onChange={(event) => setChatHarnessId(event.target.value)}
                  className="h-8 min-w-0 rounded-lg border border-input bg-transparent px-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
                >
                  {harnesses.map((harness) => (
                    <option key={harness.id} value={harness.id}>
                      {harness.name}
                    </option>
                  ))}
                </select>
              </div>
              {selectedChatHarness ? (
                <Badge
                  variant={
                    statusVariant[selectedChatHarness.auth_mode] ?? "outline"
                  }
                >
                  {authLabel[selectedChatHarness.auth_mode] ??
                    selectedChatHarness.auth_mode}
                </Badge>
              ) : null}
            </div>
            <div className="flex flex-col gap-2 md:flex-row">
              <Input
                value={chatMessage}
                onChange={(event) => setChatMessage(event.target.value)}
                placeholder="Ask this harness to inspect, patch, or test something..."
                className="min-h-9 flex-1"
              />
              <Button
                type="submit"
                disabled={
                  !selectedChatHarness ||
                  !chatMessage.trim() ||
                  runningHarnessId === selectedChatHarness.id
                }
              >
                {runningHarnessId === selectedChatHarness?.id ? (
                  <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
                ) : (
                  <Send data-icon="inline-start" className="size-4" />
                )}
                Send
              </Button>
            </div>
          </form>

          <div className="rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Auth</TableHead>
                  <TableHead>Codex</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Live Activity</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="w-28 text-right">Run</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {harnesses.map((harness) => {
                  const latestExecution = latestExecutionByHarness.get(
                    harness.id,
                  );
                  const latestEvent = latestExecution
                    ? latestAgentHarnessEvent(latestExecution)
                    : undefined;
                  const isRunning = runningHarnessId === harness.id;
                  const isExpanded =
                    !!latestExecution &&
                    expandedExecutionId === latestExecution.id;
                  return (
                    <Fragment key={harness.id}>
                      <TableRow>
                        <TableCell>
                          <div className="font-medium">{harness.name}</div>
                          <div className="max-w-xl truncate text-xs text-muted-foreground">
                            {harness.task_prompt}
                          </div>
                        </TableCell>
                        <TableCell>
                          {authLabel[harness.auth_mode] ?? harness.auth_mode}
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {harness.codex_model
                            ? `${harness.codex_template} / ${harness.codex_model}`
                            : harness.codex_template}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={statusVariant[harness.status] ?? "outline"}
                          >
                            {harness.status}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {latestExecution ? (
                            <div className="min-w-64 space-y-2">
                              <div className="flex flex-wrap items-center gap-2">
                                <Badge
                                  variant={
                                    statusVariant[latestExecution.status] ??
                                    "outline"
                                  }
                                >
                                  {latestExecution.status}
                                </Badge>
                                <span className="text-xs text-muted-foreground">
                                  started{" "}
                                  {formatDateTime(latestExecution.created_at)}
                                </span>
                              </div>
                              {latestEvent ? (
                                <div className="space-y-1">
                                  <div className="flex items-center gap-2 text-sm">
                                    <Activity className="size-4 text-muted-foreground" />
                                    <span className="font-medium">
                                      {formatEventType(latestEvent.event_type)}
                                    </span>
                                  </div>
                                  <p className="line-clamp-2 max-w-lg text-xs text-muted-foreground">
                                    {eventSummary(latestEvent)}
                                  </p>
                                  <div className="text-xs text-muted-foreground">
                                    {formatDateTime(latestEvent.occurred_at)}
                                  </div>
                                </div>
                              ) : (
                                <div className="text-xs text-muted-foreground">
                                  Waiting for the first execution event...
                                </div>
                              )}
                            </div>
                          ) : (
                            <span className="text-sm text-muted-foreground">
                              Never run
                            </span>
                          )}
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {new Date(harness.updated_at).toLocaleString()}
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex justify-end gap-2">
                            <Button
                              type="button"
                              size="icon-sm"
                              variant="ghost"
                              aria-label={`Show activity for ${harness.name}`}
                              disabled={!latestExecution}
                              onClick={() =>
                                setExpandedExecutionId(
                                  isExpanded || !latestExecution
                                    ? null
                                    : latestExecution.id,
                                )
                              }
                            >
                              <ChevronDown
                                className={`size-4 transition-transform ${
                                  isExpanded ? "rotate-180" : ""
                                }`}
                              />
                            </Button>
                            <Button
                              type="button"
                              size="icon-sm"
                              variant="outline"
                              aria-label={`Run ${harness.name}`}
                              disabled={
                                isRunning || harness.status === "archived"
                              }
                              onClick={() =>
                                void startHarnessExecution(harness.id)
                              }
                            >
                              {isRunning ? (
                                <Loader2 className="size-4 animate-spin" />
                              ) : (
                                <Play className="size-4" />
                              )}
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                      {isExpanded && latestExecution ? (
                        <TableRow>
                          <TableCell colSpan={7} className="bg-muted/30 p-0">
                            <AgentHarnessExecutionTimeline
                              execution={latestExecution}
                            />
                          </TableCell>
                        </TableRow>
                      ) : null}
                    </Fragment>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        </>
      )}
    </div>
  );
}

function AgentHarnessExecutionTimeline({
  execution,
}: {
  execution: AgentHarnessExecution;
}) {
  const events = [...(execution.events ?? [])].sort(
    (a, b) => a.sequence_number - b.sequence_number,
  );
  return (
    <div className="space-y-3 px-4 py-3">
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="font-medium">Execution timeline</span>
        <Badge variant={statusVariant[execution.status] ?? "outline"}>
          {execution.status}
        </Badge>
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
              className="grid gap-3 rounded-md border border-border bg-background p-3 md:grid-cols-[11rem_1fr]"
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

function latestAgentHarnessEvent(execution: AgentHarnessExecution) {
  return [...(execution.events ?? [])].sort(
    (a, b) => b.sequence_number - a.sequence_number,
  )[0];
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
  const preferredKeys = [
    "type",
    "message",
    "decision",
    "summary",
    "result",
    "error",
    "command",
    "tool",
    "exit_code",
    "score",
    "passed",
    "failed",
    "skipped",
    "changed_files",
    "working_directory",
    "raw",
  ];
  const lines = preferredKeys
    .flatMap((key) => {
      if (!(key in payload)) return [];
      return [`${formatPayloadKey(key)}: ${formatPayloadValue(payload[key])}`];
    })
    .filter(Boolean);
  if (lines.length > 0) return lines.join("\n");
  return `${event.actor_type} emitted ${event.event_type}`;
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
    return value.map((item) => String(item)).join(" ");
  }
  if (typeof value === "string") {
    return value.trim() || "(empty)";
  }
  if (value === null || value === undefined) {
    return "(empty)";
  }
  if (typeof value === "object") {
    return JSON.stringify(value);
  }
  return String(value);
}

function formatDateTime(value: string) {
  return new Date(value).toLocaleString();
}
