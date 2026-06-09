"use client";

import Link from "next/link";
import {
  Activity,
  ArrowLeft,
  CheckCircle2,
  FileText,
  Flag,
  Gauge,
  Hammer,
  Play,
  Terminal,
} from "lucide-react";

import type {
  AgentTryout,
  AgentTryoutEventsResponse,
  TryoutTimelineEvent,
  TryoutTimelineEventType,
} from "@/lib/api/types";
import { useApiQuery } from "@/lib/api/swr";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { Badge } from "@/components/ui/badge";
import {
  formatTryoutCost,
  formatTryoutLatency,
  tryoutIsActive,
  tryoutModelLabel,
  tryoutStatusVariant,
} from "../status";
import { PromoteTryoutDialog } from "./promote-tryout-dialog";
import { RerunTryoutDialog } from "./rerun-tryout-dialog";

export function TryoutDetailClient({
  workspaceId,
  tryoutId,
}: {
  workspaceId: string;
  tryoutId: string;
}) {
  const tryoutPath = `/v1/workspaces/${workspaceId}/agent-tryouts/${tryoutId}`;

  const {
    data: tryout,
    error,
    isLoading,
  } = useApiQuery<AgentTryout>(tryoutPath, undefined, {
    refreshInterval: (current) =>
      current && tryoutIsActive(current.status) ? 2000 : 0,
  });

  const { data: eventsData } = useApiQuery<AgentTryoutEventsResponse>(
    `${tryoutPath}/events`,
    { limit: 200 },
    {
      refreshInterval: (current) =>
        current && tryoutIsActive(current.status) ? 2000 : 0,
    },
  );

  if (isLoading && !tryout) {
    return <WorkspaceListLoading rows={6} />;
  }

  if (error || !tryout) {
    return (
      <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
        Failed to load this tryout.
      </div>
    );
  }

  const events = eventsData?.events ?? [];
  const summaryMessage =
    typeof tryout.summary?.message === "string" ? tryout.summary.message : "";

  return (
    <div className="space-y-6">
      <div>
        <Link
          href={`/workspaces/${workspaceId}/agent-tryouts`}
          className="mb-3 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-3.5" />
          Agent Tryouts
        </Link>
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="flex items-center gap-2 text-lg font-semibold tracking-tight">
              {tryout.template_slug}
              <Badge variant={tryoutStatusVariant(tryout.status)}>
                {tryout.status}
              </Badge>
            </h1>
            {summaryMessage ? (
              <p className="mt-1 text-sm text-muted-foreground">
                {summaryMessage}
              </p>
            ) : null}
            {tryout.parent_tryout_id ? (
              <p className="mt-1 text-xs text-muted-foreground">
                Rerun of{" "}
                <Link
                  href={`/workspaces/${workspaceId}/agent-tryouts/${tryout.parent_tryout_id}`}
                  className="underline-offset-2 hover:underline"
                >
                  {tryout.parent_tryout_id.slice(0, 8)}
                </Link>
              </p>
            ) : null}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <RerunTryoutDialog workspaceId={workspaceId} tryout={tryout} />
            <PromoteTryoutDialog workspaceId={workspaceId} tryout={tryout} />
          </div>
        </div>
      </div>

      <dl className="grid grid-cols-2 gap-4 rounded-lg border border-border p-4 text-sm sm:grid-cols-4">
        <Stat label="Model" value={tryoutModelLabel(tryout)} />
        <Stat label="Cost" value={formatTryoutCost(tryout)} />
        <Stat label="Latency" value={formatTryoutLatency(tryout.latency_ms)} />
        <Stat
          label="Launched"
          value={new Date(tryout.created_at).toLocaleString()}
        />
      </dl>

      <section>
        <h2 className="mb-3 text-sm font-semibold tracking-tight">Timeline</h2>
        {events.length === 0 ? (
          <div className="rounded-lg border border-border p-4 text-sm text-muted-foreground">
            {tryoutIsActive(tryout.status)
              ? "Waiting for the agent to start…"
              : "No timeline events were recorded for this tryout."}
          </div>
        ) : (
          <ol className="rounded-lg border border-border">
            {events.map((event) => (
              <TimelineRow key={event.cursor} event={event} />
            ))}
          </ol>
        )}
      </section>

      <section>
        <h2 className="mb-3 text-sm font-semibold tracking-tight">Input</h2>
        <pre className="overflow-x-auto rounded-lg border border-border bg-muted/30 p-4 text-xs">
          {JSON.stringify(tryout.input_snapshot, null, 2)}
        </pre>
      </section>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="mt-0.5 font-medium">{value}</dd>
    </div>
  );
}

const eventIcons: Record<TryoutTimelineEventType, typeof Activity> = {
  started: Play,
  planning: Activity,
  tool_call: Hammer,
  sandbox_command: Terminal,
  file_written: FileText,
  file_activity: FileText,
  validation: CheckCircle2,
  scoring: Gauge,
  finished: Flag,
  activity: Activity,
};

function TimelineRow({ event }: { event: TryoutTimelineEvent }) {
  const Icon = eventIcons[event.type] ?? Activity;
  return (
    <li className="flex items-start gap-3 border-b border-border px-4 py-2.5 text-sm last:border-b-0">
      <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <p className="truncate">{event.summary}</p>
      </div>
      <time className="shrink-0 text-xs text-muted-foreground">
        {new Date(event.occurred_at).toLocaleTimeString()}
      </time>
    </li>
  );
}
