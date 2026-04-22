"use client";

import Link from "next/link";
import {
  Loader2,
  Trophy,
  XCircle,
  CheckCircle2,
  Play,
  BrainCircuit,
  Wrench,
  Terminal,
  BarChart3,
  Activity,
  Hash,
  Coins,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { UploadArtifactDialog } from "@/components/artifacts/upload-artifact-dialog";
import { LiveEventTicker } from "@/components/arena/live-event-ticker";
import { cn } from "@/lib/utils";
import type { RunAgent, RunAgentStatus } from "@/lib/api/types";
import type { ArenaLaneState } from "@/hooks/use-agent-arena";
import type { ArenaEventKind } from "@/lib/arena/event-formatter";

const AGENT_STATUS_VARIANT: Record<
  RunAgentStatus,
  "default" | "secondary" | "outline" | "destructive"
> = {
  queued: "secondary",
  ready: "secondary",
  executing: "outline",
  evaluating: "outline",
  completed: "default",
  failed: "destructive",
};

const ACTIVE_STATUSES: RunAgentStatus[] = [
  "queued",
  "ready",
  "executing",
  "evaluating",
];

const NOW_DOING_ICON: Record<
  ArenaEventKind,
  React.ComponentType<{ className?: string }>
> = {
  model: BrainCircuit,
  tool: Wrench,
  sandbox: Terminal,
  file: Terminal,
  scoring: BarChart3,
  system: Activity,
  unknown: Activity,
};

function formatElapsed(start?: string, end?: string): string {
  if (!start) return "\u2014";
  const startMs = new Date(start).getTime();
  const endMs = end ? new Date(end).getTime() : Date.now();
  const ms = Math.max(0, endMs - startMs);
  if (ms < 1000) return "<1s";
  const secs = Math.floor(ms / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  return `${mins}m ${secs % 60}s`;
}

function compactNumber(n: number): string {
  if (n < 1000) return n.toString();
  if (n < 1_000_000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "k";
  return (n / 1_000_000).toFixed(1).replace(/\.0$/, "") + "M";
}

interface LiveAgentLaneProps {
  agent: RunAgent;
  lane: ArenaLaneState;
  isWinner: boolean;
  workspaceId: string;
  runId: string;
  /** Rendered below the live sections when the agent is terminal (e.g. scorecard). */
  footer?: React.ReactNode;
}

export function LiveAgentLane({
  agent,
  lane,
  isWinner,
  workspaceId,
  runId,
  footer,
}: LiveAgentLaneProps) {
  const isActive = ACTIVE_STATUSES.includes(agent.status);
  const isFailed = agent.status === "failed";
  const isTerminal = !isActive;

  const nowDoingIcon = lane.nowDoing
    ? NOW_DOING_ICON[lane.nowDoing.kind] ?? Activity
    : null;
  const NowIcon = nowDoingIcon;

  return (
    <div
      className={cn(
        "rounded-lg border p-4 transition-colors",
        isWinner
          ? "border-emerald-500/40 bg-emerald-500/5"
          : isFailed
            ? "border-destructive/30 bg-destructive/5"
            : isActive
              ? "border-primary/40 bg-primary/5 shadow-sm"
              : "border-border",
      )}
    >
      {/* === Header === */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2 min-w-0">
          {isWinner && (
            <Trophy className="size-4 shrink-0 text-emerald-400" />
          )}
          <span className="font-medium text-sm truncate">{agent.label}</span>
          <span className="text-xs text-muted-foreground/50">
            #{agent.lane_index}
          </span>
        </div>
        <Badge variant={AGENT_STATUS_VARIANT[agent.status] ?? "outline"}>
          {isActive && (
            <Loader2
              data-icon="inline-start"
              className="size-3 animate-spin"
            />
          )}
          {agent.status}
        </Badge>
      </div>

      {/* === Live metrics strip === */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground mb-3">
        {agent.started_at && (
          <span className="tabular-nums">
            {formatElapsed(agent.started_at, agent.finished_at)}
          </span>
        )}
        {lane.stepIndex > 0 && (
          <span className="inline-flex items-center gap-1">
            <Hash className="size-3" />
            Step {lane.stepIndex}
          </span>
        )}
        {lane.modelCalls > 0 && (
          <span className="inline-flex items-center gap-1">
            <BrainCircuit className="size-3" />
            {lane.modelCalls}
          </span>
        )}
        {lane.toolCalls > 0 && (
          <span className="inline-flex items-center gap-1">
            <Wrench className="size-3" />
            {lane.toolCalls}
          </span>
        )}
        {lane.totalTokens > 0 && (
          <span className="inline-flex items-center gap-1">
            <Coins className="size-3" />
            {compactNumber(lane.totalTokens)} tok
          </span>
        )}
      </div>

      {/* === Now-doing banner === */}
      {isActive && (
        <div
          className={cn(
            "mb-3 flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs",
            lane.nowDoing
              ? "border-primary/30 bg-primary/10 text-foreground"
              : "border-border bg-muted/30 text-muted-foreground",
          )}
        >
          <span className="relative flex size-2 shrink-0">
            <span
              className={cn(
                "absolute inline-flex size-2 rounded-full opacity-75",
                lane.nowDoing ? "animate-ping bg-primary" : "bg-muted-foreground/40",
              )}
            />
            <span
              className={cn(
                "relative inline-flex size-2 rounded-full",
                lane.nowDoing ? "bg-primary" : "bg-muted-foreground/60",
              )}
            />
          </span>
          {NowIcon && <NowIcon className="size-3.5 shrink-0" />}
          <span className="min-w-0 flex-1 truncate font-medium">
            {lane.nowDoing?.label ?? "Waiting for next action\u2026"}
          </span>
        </div>
      )}

      {/* === Streaming model output === */}
      {isActive && lane.streamingOutput && (
        <div className="mb-3">
          <div className="text-[10px] uppercase tracking-[0.16em] text-muted-foreground mb-1">
            Streaming
          </div>
          <pre className="max-h-32 overflow-auto rounded-md bg-background border border-border px-2.5 py-1.5 text-xs font-[family-name:var(--font-mono)] whitespace-pre-wrap break-words">
            {lane.streamingOutput}
          </pre>
        </div>
      )}

      {/* === Live ticker === */}
      {(isActive || lane.ticker.length > 0) && (
        <div className="mb-3">
          <div className="text-[10px] uppercase tracking-[0.16em] text-muted-foreground mb-1">
            Activity
          </div>
          <LiveEventTicker entries={lane.ticker} />
        </div>
      )}

      {/* === Failure banner === */}
      {isFailed && agent.failure_reason && (
        <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive mb-2">
          <div className="flex items-center gap-1.5 font-medium mb-0.5">
            <XCircle className="size-3.5" />
            Failed
          </div>
          <p className="text-destructive/80">{agent.failure_reason}</p>
        </div>
      )}

      {/* === Terminal-only footer (scorecard, etc.) === */}
      {isTerminal && footer}

      {/* === Action links === */}
      <div className="flex gap-2 mt-2">
        <Link
          href={`/workspaces/${workspaceId}/runs/${runId}/agents/${agent.id}/replay`}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
        >
          <Play className="size-3" />
          Replay
        </Link>
        <Link
          href={`/workspaces/${workspaceId}/runs/${runId}/agents/${agent.id}/scorecard`}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
        >
          <CheckCircle2 className="size-3" />
          Scorecard
        </Link>
        <UploadArtifactDialog
          workspaceId={workspaceId}
          runId={runId}
          runAgentId={agent.id}
        />
      </div>
    </div>
  );
}
