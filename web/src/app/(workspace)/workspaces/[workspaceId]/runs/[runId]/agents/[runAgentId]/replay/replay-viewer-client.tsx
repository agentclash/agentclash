"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import { CreatePublicShareButton } from "@/components/share/create-public-share-button";
import type {
  Run,
  RunAgent,
  RunAgentStatus,
  ReplayResponse,
  ReplayStep,
  TranscriptResponse,
} from "@/lib/api/types";
import { isAgentAwaitingHumanInput, isRunActive } from "@/lib/run-status";
import { getRunAgentTranscript } from "@/lib/api/multi-turn";
import { Badge } from "@/components/ui/badge";
import { ReplayTimeline } from "@/components/replay/replay-timeline";
import { ConversationTranscript } from "@/components/replay/conversation-transcript";
import { DownloadTranscriptButton } from "@/components/replay/download-transcript-button";
import { AwaitingHumanBanner } from "@/components/replay/awaiting-human-banner";
import { Panel } from "../scorecard/components/panel";
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
const HUMAN_TURN_POLL_MS = 3000;

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
  const [liveAgent, setLiveAgent] = useState<RunAgent>(agent);
  const [liveRunStatus, setLiveRunStatus] = useState<Run["status"]>(run.status);
  const [transcript, setTranscript] = useState<TranscriptResponse | null>(null);

  // When the inspector sheet deep-links in with ?step=<sequence>, the timeline
  // highlights + auto-scrolls to that step. NaN is filtered by the consumer.
  const searchParams = useSearchParams();
  const highlightSequence = useMemo(() => {
    const raw = searchParams?.get("step");
    if (!raw) return undefined;
    const parsed = Number.parseInt(raw, 10);
    return Number.isFinite(parsed) ? parsed : undefined;
  }, [searchParams]);

  const isPending = replayData.state === "pending";
  const isErrored = replayData.state === "errored";
  const isReady = replayData.state === "ready";
  const isRunActiveStatus = isRunActive(liveRunStatus);
  const awaitingHumanEnabled = isAgentAwaitingHumanInput(liveAgent.status);

  // Keep agent/run status fresh while the run is active so the human-input banner can appear.
  useEffect(() => {
    if (!isRunActiveStatus) return;
    const refreshLiveStatus = async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const [runRes, agentsRes] = await Promise.all([
          api.get<Run>(`/v1/runs/${run.id}`),
          api.get<{ items: RunAgent[] }>(`/v1/runs/${run.id}/agents`),
        ]);
        setLiveRunStatus(runRes.status);
        const nextAgent = agentsRes.items.find((item) => item.id === agent.id);
        if (nextAgent) setLiveAgent(nextAgent);
      } catch {
        // Ignore polling errors; replay page stays usable.
      }
    };
    void refreshLiveStatus();
    const interval = setInterval(() => void refreshLiveStatus(), HUMAN_TURN_POLL_MS);
    return () => clearInterval(interval);
  }, [isRunActiveStatus, getAccessToken, run.id, agent.id]);

  // Fetch the multi-turn transcript. Re-fetch while the run is active so the
  // conversation grows live; single-turn runs simply return zero turns.
  useEffect(() => {
    let cancelled = false;
    const fetchTranscript = async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await getRunAgentTranscript(api, agent.id);
        if (!cancelled) setTranscript(res);
      } catch {
        // Transcript is supplementary; ignore errors so the replay stays usable.
      }
    };
    void fetchTranscript();
    // Always return a cleanup that flips `cancelled`, even on the inactive
    // path. Otherwise a still-in-flight fetch from a prior render can resolve
    // after a newer one and clobber the transcript with stale data.
    const interval = isRunActiveStatus
      ? setInterval(() => void fetchTranscript(), HUMAN_TURN_POLL_MS)
      : undefined;
    return () => {
      cancelled = true;
      if (interval !== undefined) clearInterval(interval);
    };
  }, [getAccessToken, agent.id, isRunActiveStatus]);

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
      <Panel className="overflow-hidden mb-6">
        <div className="px-5 pt-4 pb-3 border-b border-white/[0.06]">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <h1 className="font-[family-name:var(--font-display)] text-2xl leading-none tracking-[-0.01em] text-white/95">
                {liveAgent.label}
              </h1>
              <Badge
                variant={
                  agentStatusVariant[liveAgent.status as RunAgentStatus] ?? "outline"
                }
                className="bg-white/5 text-white/80 border-white/10 hover:bg-white/10"
              >
                {liveAgent.status}
              </Badge>
            </div>
            {isReady && (
              <CreatePublicShareButton
                resourceType="run_agent_replay"
                resourceId={agent.id}
                label="Share replay"
                size="xs"
              />
            )}
          </div>
          <p className="mt-2 text-2xs uppercase tracking-[0.14em] text-white/40">
            Replay for{" "}
            <Link
              href={`/workspaces/${workspaceId}/runs/${run.id}`}
              className="text-white/60 hover:text-white transition-colors underline underline-offset-2"
            >
              {run.name}
            </Link>
          </p>
        </div>
      </Panel>

      {/* Pending banner */}
      {isPending && (
        <Panel tone="warn" className="px-4 py-3 text-sm">
          <div className="flex items-center gap-2 text-amber-500">
            <Loader2 className="size-4 animate-spin" />
            <span className="font-medium uppercase tracking-[0.14em] text-2xs">Replay pending</span>
          </div>
          {replayData.message && (
            <p className="mt-1.5 text-amber-500/70 text-xs">
              {replayData.message}
            </p>
          )}
        </Panel>
      )}

      {/* Errored banner */}
      {isErrored && (
        <Panel tone="danger" className="px-4 py-3 text-sm">
          <div className="flex items-center gap-2 text-red-500">
            <AlertTriangle className="size-4" />
            <span className="font-medium uppercase tracking-[0.14em] text-2xs">Replay unavailable</span>
          </div>
          {replayData.message && (
            <p className="mt-1.5 text-red-500/70 text-xs">
              {replayData.message}
            </p>
          )}
        </Panel>
      )}

      <AwaitingHumanBanner
        getAccessToken={getAccessToken}
        workspaceId={workspaceId}
        runId={run.id}
        runAgentId={agent.id}
        enabled={awaitingHumanEnabled}
      />

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
        <Panel className="flex items-center gap-3 px-4 py-3 text-2xs uppercase tracking-[0.14em] text-white/60">
          <Clock className="size-4 text-white/30" />
          <span>{replayData.replay.summary.terminal_state.headline}</span>
          {replayData.replay.summary.terminal_state.error_message && (
            <span className="text-red-400 normal-case tracking-normal text-xs ml-2">
              — {replayData.replay.summary.terminal_state.error_message}
            </span>
          )}
        </Panel>
      )}

      {/* Multi-turn conversation transcript (only present for multi_turn runs) */}
      {transcript && transcript.turns.length > 0 && (
        <ConversationTranscript
          turns={transcript.turns}
          notice={
            transcript.state === "errored" ? transcript.message : undefined
          }
          trailing={
            <DownloadTranscriptButton
              turns={transcript.turns}
              meta={{
                agentLabel: liveAgent.label,
                runName: run.name,
                runId: run.id,
                runAgentId: agent.id,
              }}
            />
          }
        />
      )}

      {/* Timeline */}
      {isReady && (
        <ReplayTimeline
          steps={steps}
          hasMore={replayData.pagination.has_more}
          isLoadingMore={isLoadingMore}
          onLoadMore={handleLoadMore}
          highlightSequence={highlightSequence}
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
    <Panel className="px-4 py-3 flex flex-col gap-1.5">
      <div className="flex items-center gap-2 text-2xs uppercase tracking-[0.14em] text-white/40">
        <div className="text-white/30">{icon}</div>
        <span>{label}</span>
      </div>
      <span className="text-xl font-[family-name:var(--font-mono)] text-white/90 tabular-nums">{value}</span>
    </Panel>
  );
}
