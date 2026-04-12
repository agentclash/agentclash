"use client";

import { useState, useEffect, useCallback } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import type {
  Run,
  RunAgent,
  RunAgentStatus,
  ReplayResponse,
  ReplayStep,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { ReplayTimeline } from "@/components/replay/replay-timeline";
import { toast } from "sonner";
import {
  Loader2,
  AlertTriangle,
  Clock,
  BrainCircuit,
  Wrench,
  Terminal,
  BarChart3,
  Layers,
} from "lucide-react";

const POLL_MS = 5000;

const agentStatusVariant: Record<
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

interface ReplayViewerClientProps {
  initialReplay: ReplayResponse;
  run: Run;
  agent: RunAgent;
  workspaceId: string;
}

export function ReplayViewerClient({
  initialReplay,
  run,
  agent,
  workspaceId,
}: ReplayViewerClientProps) {
  const { getAccessToken } = useAccessToken();
  const [replayData, setReplayData] = useState<ReplayResponse>(initialReplay);
  const [steps, setSteps] = useState<ReplayStep[]>(initialReplay.steps);
  const [isLoadingMore, setIsLoadingMore] = useState(false);

  const isPending = replayData.state === "pending";
  const isErrored = replayData.state === "errored";
  const isReady = replayData.state === "ready";

  // Auto-poll when pending
  useEffect(() => {
    if (!isPending) return;
    const interval = setInterval(async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await api.get<ReplayResponse>(
          `/v1/replays/${agent.id}`,
          { params: { limit: 50 }, allowedStatuses: [409] },
        );
        setReplayData(res);
        setSteps(res.steps);
      } catch {
        // Silently retry on next poll
      }
    }, POLL_MS);
    return () => clearInterval(interval);
  }, [isPending, getAccessToken, agent.id]);

  const handleLoadMore = useCallback(async () => {
    if (!replayData.pagination.has_more || isLoadingMore) return;
    setIsLoadingMore(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await api.get<ReplayResponse>(
        `/v1/replays/${agent.id}`,
        {
          params: {
            cursor: replayData.pagination.next_cursor,
            limit: 50,
          },
          allowedStatuses: [409],
        },
      );
      setReplayData(res);
      setSteps((prev) => [...prev, ...res.steps]);
    } catch {
      toast.error("Failed to load more steps");
    } finally {
      setIsLoadingMore(false);
    }
  }, [getAccessToken, agent.id, replayData.pagination, isLoadingMore]);

  const counts = replayData.replay?.summary?.counts;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-3 mb-1">
          <h1 className="text-lg font-semibold tracking-tight">
            {agent.label}
          </h1>
          <Badge
            variant={
              agentStatusVariant[agent.status as RunAgentStatus] ?? "outline"
            }
          >
            {agent.status}
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground">
          Replay for{" "}
          <Link
            href={`/workspaces/${workspaceId}/runs/${run.id}`}
            className="hover:text-foreground transition-colors underline underline-offset-2"
          >
            {run.name}
          </Link>
        </p>
      </div>

      {/* Pending banner */}
      {isPending && (
        <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm">
          <div className="flex items-center gap-2 text-amber-600 dark:text-amber-400">
            <Loader2 className="size-4 animate-spin" />
            <span className="font-medium">Replay pending</span>
          </div>
          {replayData.message && (
            <p className="mt-1 text-muted-foreground">
              {replayData.message}
            </p>
          )}
        </div>
      )}

      {/* Errored banner */}
      {isErrored && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm">
          <div className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="size-4" />
            <span className="font-medium">Replay unavailable</span>
          </div>
          {replayData.message && (
            <p className="mt-1 text-destructive/80">
              {replayData.message}
            </p>
          )}
        </div>
      )}

      {/* Summary stats */}
      {isReady && counts && (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
          <StatCard
            icon={<Layers className="size-4" />}
            label="Total Steps"
            value={counts.replay_steps}
          />
          <StatCard
            icon={<BrainCircuit className="size-4" />}
            label="Model Calls"
            value={counts.model_calls}
          />
          <StatCard
            icon={<Wrench className="size-4" />}
            label="Tool Calls"
            value={counts.tool_calls}
          />
          <StatCard
            icon={<Terminal className="size-4" />}
            label="Sandbox Cmds"
            value={counts.sandbox_commands}
          />
          <StatCard
            icon={<BarChart3 className="size-4" />}
            label="Scoring"
            value={counts.scoring_events}
          />
        </div>
      )}

      {/* Terminal state */}
      {isReady && replayData.replay?.summary?.terminal_state && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Clock className="size-3.5" />
          <span>{replayData.replay.summary.terminal_state.headline}</span>
          {replayData.replay.summary.terminal_state.error_message && (
            <span className="text-destructive">
              — {replayData.replay.summary.terminal_state.error_message}
            </span>
          )}
        </div>
      )}

      {/* Timeline */}
      {isReady && (
        <ReplayTimeline
          steps={steps}
          hasMore={replayData.pagination.has_more}
          isLoadingMore={isLoadingMore}
          onLoadMore={handleLoadMore}
        />
      )}
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: number;
}) {
  return (
    <div className="rounded-lg border border-border px-3 py-2">
      <div className="flex items-center gap-1.5 text-muted-foreground mb-0.5">
        {icon}
        <span className="text-xs">{label}</span>
      </div>
      <span className="text-lg font-semibold tabular-nums">{value}</span>
    </div>
  );
}
