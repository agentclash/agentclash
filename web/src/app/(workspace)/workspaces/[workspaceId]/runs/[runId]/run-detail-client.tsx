"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";

import { LiveAgentLane } from "@/components/arena/live-agent-lane";
import { LiveCommentarySidebar } from "@/components/arena/live-commentary-sidebar";
import { RaceModeArena } from "@/components/arena/race-mode";
import { UploadArtifactDialog } from "@/components/artifacts/upload-artifact-dialog";
import { CreatePublicShareButton } from "@/components/share/create-public-share-button";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { VoiceModeBadges } from "@/components/voice/voice-mode-badges";
import { useAgentArena, EMPTY_LANE } from "@/hooks/use-agent-arena";
import { useAgentCommentary } from "@/hooks/use-agent-commentary";
import { useArenaMode } from "@/hooks/use-arena-mode";
import { useRunEvents, type RunEvent } from "@/hooks/use-run-events";
import { createApiClient } from "@/lib/api/client";
import { scorePercent } from "@/lib/scores";
import {
  ACTIVE_AGENT_STATUSES,
  isAgentAwaitingHumanInput,
  isRunActive,
} from "@/lib/run-status";
import { isVoiceRun, voiceRunMode, voiceRunTransport } from "@/lib/voice-evals";
import type {
  Run,
  RunStatus,
  RunAgent,
  RunAgentStatus,
  RunRankingResponse,
  RankingItem,
  ScorecardResponse,
} from "@/lib/api/types";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import Link from "next/link";
import {
  Trophy,
  Clock,
  CheckCircle2,
  XCircle,
  Loader2,
  AlertTriangle,
  AlertOctagon,
  Radio,
  MessageSquareText,
  Flag,
  GitBranch,
  GitPullRequest,
  ExternalLink,
} from "lucide-react";

import { CompareRunPicker } from "./compare-run-picker";
import { Panel } from "./agents/[runAgentId]/scorecard/components/panel";
import { RunRankingInsightsCard } from "./run-ranking-insights-card";
import { ScorecardSummaryCard } from "./scorecard-summary-card";
import { AwaitingHumanBanner } from "@/components/replay/awaiting-human-banner";

// --- Status variant maps ---

const runStatusVariant: Record<
  RunStatus,
  "default" | "secondary" | "outline" | "destructive"
> = {
  draft: "outline",
  queued: "secondary",
  provisioning: "secondary",
  running: "outline",
  scoring: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
};

const POLL_MS = 5000;

const LEGACY_SORT_OPTIONS = [
  { key: "composite", label: "Composite" },
  { key: "correctness", label: "Correctness" },
  { key: "reliability", label: "Reliability" },
  { key: "latency", label: "Latency" },
  { key: "cost", label: "Cost" },
];

/** Build sort options from ranking data — legacy 4 dims + any custom dimensions. */
function buildSortOptions(
  ranking: RunRankingResponse | null,
): { key: string; label: string }[] {
  if (!ranking?.ranking?.items?.length) return LEGACY_SORT_OPTIONS;
  const customKeys = new Set<string>();
  for (const item of ranking.ranking.items) {
    if (!item.dimensions) continue;
    for (const key of Object.keys(item.dimensions)) {
      if (!["correctness", "reliability", "latency", "cost"].includes(key)) {
        customKeys.add(key);
      }
    }
  }
  if (customKeys.size === 0) return LEGACY_SORT_OPTIONS;
  const custom = [...customKeys].sort().map((key) => ({
    key,
    label: key.charAt(0).toUpperCase() + key.slice(1).replace(/_/g, " "),
  }));
  return [...LEGACY_SORT_OPTIONS, ...custom];
}
// --- Helpers ---

function formatDuration(start: string, end?: string): string {
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const ms = e - s;
  if (ms < 1000) return "<1s";
  const secs = Math.floor(ms / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  const remSecs = secs % 60;
  return `${mins}m ${remSecs}s`;
}

function deltaLabel(delta?: number): string {
  if (delta == null || delta === 0) return "";
  const pct = (delta * 100).toFixed(1);
  return delta > 0 ? `+${pct}%` : `${pct}%`;
}

function outcomeVariant(outcome: "pending" | "pass" | "fail") {
  switch (outcome) {
    case "pass":
      return "default";
    case "fail":
      return "destructive";
    default:
      return "outline";
  }
}

function shortCommit(sha?: string): string | null {
  if (!sha) return null;
  return sha.length > 7 ? sha.slice(0, 7) : sha;
}

function safeHTTPURL(raw?: string): string | null {
  if (!raw) return null;
  try {
    const url = new URL(raw);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    return url.toString();
  } catch {
    return null;
  }
}

// --- Component ---

interface RunDetailClientProps {
  initialRun: Run;
  initialAgents: RunAgent[];
  workspaceId: string;
}

export function RunDetailClient({
  initialRun,
  initialAgents,
  workspaceId,
}: RunDetailClientProps) {
  const { getAccessToken } = useAccessToken();
  const [run, setRun] = useState<Run>(initialRun);
  const [agents, setAgents] = useState<RunAgent[]>(initialAgents);
  const [ranking, setRanking] = useState<RunRankingResponse | null>(null);
  const [sortBy, setSortBy] = useState("composite");
  const [scorecards, setScorecards] = useState<
    Record<string, ScorecardResponse | null>
  >({});
  const [showCommentary, setShowCommentary] = useState(false);
  const [arenaMode, setArenaMode] = useArenaMode();

  const isActive =
    isRunActive(run.status) ||
    agents.some((a) => ACTIVE_AGENT_STATUSES.includes(a.status));

  const fetchAll = useCallback(async () => {
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const [runRes, agentsRes] = await Promise.all([
        api.get<Run>(`/v1/runs/${run.id}`),
        api.get<{ items: RunAgent[] }>(`/v1/runs/${run.id}/agents`),
      ]);
      setRun(runRes);
      setAgents(agentsRes.items);
    } catch {
      // Silently fail on poll
    }
  }, [getAccessToken, run.id]);

  // SSE token for live event streaming.
  const [sseToken, setSseToken] = useState<string>();
  useEffect(() => {
    if (!isActive) return;
    let cancelled = false;
    getAccessToken().then((t) => {
      if (!cancelled) setSseToken(t);
    });
    return () => {
      cancelled = true;
    };
  }, [isActive, getAccessToken]);

  // Live per-lane arena projection from the SSE event stream.
  const { lanes: arenaLanes, handleEvent: handleArenaEvent } = useAgentArena();
  const {
    entries: commentaryEntries,
    handleEvent: handleCommentaryEvent,
    reset: resetCommentary,
  } = useAgentCommentary(agents);

  // Debounced refetch: collapse rapid SSE events into a single fetch.
  const fetchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const handleSSEEvent = useCallback(
    (event: RunEvent) => {
      handleArenaEvent(event);
      if (showCommentary) {
        handleCommentaryEvent(event);
      }
      if (fetchTimerRef.current) clearTimeout(fetchTimerRef.current);
      fetchTimerRef.current = setTimeout(fetchAll, 300);
    },
    [fetchAll, handleArenaEvent, handleCommentaryEvent, showCommentary],
  );

  useEffect(() => {
    return () => {
      if (fetchTimerRef.current) clearTimeout(fetchTimerRef.current);
    };
  }, []);

  // Live event streaming via SSE (graceful degradation: falls back to polling).
  const { connected: sseConnected } = useRunEvents({
    runId: run.id,
    token: sseToken,
    enabled: isActive,
    onEvent: handleSSEEvent,
  });

  // Auto-refresh: 30s safety-net when SSE is connected, 5s polling otherwise.
  useEffect(() => {
    if (!isActive) return;
    const interval = setInterval(
      fetchAll,
      sseConnected ? 30_000 : POLL_MS,
    );
    return () => clearInterval(interval);
  }, [isActive, fetchAll, sseConnected]);

  // Fetch ranking when run reaches a terminal state or on mount if already terminal
  const isTerminal =
    run.status === "completed" ||
    run.status === "failed" ||
    run.status === "cancelled";

  useEffect(() => {
    if (!isTerminal) return;
    let cancelled = false;
    (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const res = await api.get<RunRankingResponse>(
          `/v1/runs/${run.id}/ranking`,
          { params: { sort_by: sortBy } },
        );
        if (!cancelled) setRanking(res);
      } catch {
        if (!cancelled) setRanking(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isTerminal, sortBy, getAccessToken, run.id]);

  // Fetch scorecards for completed/failed agents.
  // Derive a stable key from agent IDs + statuses to avoid re-firing on every poll.
  const completedAgentKey = agents
    .filter((a) => a.status === "completed" || a.status === "failed")
    .map((a) => a.id)
    .sort()
    .join(",");

  useEffect(() => {
    if (!completedAgentKey) return;
    const ids = completedAgentKey.split(",");
    let cancelled = false;
    (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const results = await Promise.all(
          ids.map((id) =>
            api
              .get<ScorecardResponse>(`/v1/scorecards/${id}`, {
                allowedStatuses: [202, 409],
              })
              .catch(() => null),
          ),
        );
        if (cancelled) return;
        const map: Record<string, ScorecardResponse | null> = {};
        ids.forEach((id, i) => {
          map[id] = results[i];
        });
        setScorecards(map);
      } catch {
        // Silently fail
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [completedAgentKey, getAccessToken]);

  function handleSortChange(sort: string) {
    setSortBy(sort);
  }

  const sortedAgents = [...agents].sort(
    (a, b) => a.lane_index - b.lane_index,
  );
  const ciMetadata = run.ci_metadata;
  const ciCommit = shortCommit(ciMetadata?.commit_sha);
  const ciWorkflowURL = safeHTTPURL(ciMetadata?.workflow_run_url);
  const voiceRun = isVoiceRun(run);

  return (
    <div className="space-y-8">
      {/* === Header === */}
      <Panel className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-4 px-5 pt-4 pb-3 border-b border-white/[0.06]">
          <div className="flex items-center gap-3">
            <h1 className="font-[family-name:var(--font-display)] text-2xl leading-none tracking-[-0.01em] text-white/95 truncate">
              {run.name}
            </h1>
            <Badge variant={runStatusVariant[run.status] ?? "outline"} className="bg-white/5 text-white/80 border-white/10 hover:bg-white/10">
              {isActive && (
                <Loader2
                  data-icon="inline-start"
                  className="size-3 animate-spin mr-1"
                />
              )}
              {run.status}
            </Badge>
            {isActive && sseConnected && (
              <span
                className="inline-flex items-center gap-1 rounded-md border border-emerald-500/30 bg-emerald-500/10 px-1.5 h-6 text-2xs font-medium uppercase tracking-wider text-emerald-400"
                title="Streaming live events"
              >
                <Radio className="size-3 animate-pulse" />
                Live
              </span>
            )}
            <Badge variant="outline" className="bg-white/5 text-white/60 border-white/10">
              {run.execution_mode === "comparison"
                ? "Comparison"
                : "Single Agent"}
            </Badge>
            {voiceRun && (
              <VoiceModeBadges
                modality={run.voice?.modality ?? run.modality}
                mode={voiceRunMode(run)}
                transport={voiceRunTransport(run)}
              />
            )}
          </div>
          
          <div className="flex items-center gap-2">
            <CompareRunPicker
              currentRunId={run.id}
              workspaceId={workspaceId}
            />
            <UploadArtifactDialog
              workspaceId={workspaceId}
              runId={run.id}
            />
            <CreatePublicShareButton
              resourceType="run_scorecard"
              resourceId={run.id}
              label="Share scorecard"
              disabled={!isTerminal}
            />
            <Button
              variant={showCommentary ? "default" : "outline"}
              size="sm"
              className={showCommentary ? "" : "bg-transparent border-white/10 text-white/70 hover:bg-white/5 hover:text-white"}
              onClick={() =>
                setShowCommentary((current) => {
                  if (current) resetCommentary();
                  return !current;
                })
              }
              aria-pressed={showCommentary}
            >
              <MessageSquareText className="size-3.5 mr-1.5" />
              Commentary {showCommentary ? "On" : "Off"}
            </Button>
            <Button
              variant={arenaMode === "broadcast" ? "default" : "outline"}
              size="sm"
              className={arenaMode === "broadcast" ? "" : "bg-transparent border-white/10 text-white/70 hover:bg-white/5 hover:text-white"}
              onClick={() =>
                setArenaMode(arenaMode === "broadcast" ? "dev" : "broadcast")
              }
              aria-pressed={arenaMode === "broadcast"}
              title={
                arenaMode === "broadcast"
                  ? "Switch to development (classic) view"
                  : "Switch to broadcast mode — the live comparison view"
              }
            >
              <Flag className="size-3.5 mr-1.5" />
              {arenaMode === "broadcast" ? "Broadcast Mode" : "Dev Mode"}
            </Button>
            <Link
              href={`/workspaces/${workspaceId}/runs/${run.id}/failures`}
              className="inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-transparent px-2.5 h-8 text-xs text-white/70 hover:text-white hover:bg-white/5 transition-colors"
            >
              <AlertOctagon className="size-3.5" />
              Failures
            </Link>
          </div>
        </div>

        {/* KPI strip */}
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2 px-5 py-3 text-2xs uppercase tracking-[0.14em] text-white/40 bg-black/20">
          <div className="flex items-center gap-2">
            <Clock className="size-3.5" />
            {run.started_at ? (
              <span>
                Duration:{" "}
                <span className="font-[family-name:var(--font-mono)] normal-case tracking-normal text-white/70">
                  {formatDuration(run.started_at, run.finished_at)}
                </span>
              </span>
            ) : (
              <span>Waiting to start</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <span>Agents:</span>
            <span className="font-[family-name:var(--font-mono)] normal-case tracking-normal text-white/70">{agents.length}</span>
          </div>
          {voiceRun && (
            <div className="flex items-center gap-2">
              <span>Voice:</span>
              <span className="font-[family-name:var(--font-mono)] normal-case tracking-normal text-white/70">
                {voiceRunMode(run) || "voice"}
                {voiceRunTransport(run) ? ` / ${voiceRunTransport(run)}` : ""}
              </span>
            </div>
          )}
          <div className="flex items-center gap-2">
            <span>Run ID:</span>
            <code className="font-[family-name:var(--font-mono)] normal-case tracking-normal text-white/70">
              {run.id.slice(0, 8)}
            </code>
          </div>
          {ciMetadata && (
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 normal-case tracking-normal text-white/70">
              {ciMetadata.repository && (
                <span className="inline-flex items-center gap-1">
                  <GitBranch className="size-3.5 text-white/40" />
                  {ciMetadata.repository}
                  {ciMetadata.branch ? `:${ciMetadata.branch}` : ""}
                </span>
              )}
              {ciMetadata.pull_request_number && (
                <span className="inline-flex items-center gap-1">
                  <GitPullRequest className="size-3.5 text-white/40" />
                  PR #{ciMetadata.pull_request_number}
                </span>
              )}
              {ciCommit && (
                <code className="font-[family-name:var(--font-mono)] text-white/70">
                  {ciCommit}
                </code>
              )}
              {ciWorkflowURL && (
                <a
                  href={ciWorkflowURL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-white/80 hover:text-white transition-colors"
                >
                  {ciMetadata.workflow ?? "Workflow"}
                  <ExternalLink className="size-3" />
                </a>
              )}
            </div>
          )}
          {run.queued_at && (
            <div className="flex items-center gap-2">
              <span>Queued:</span>
              <span className="font-[family-name:var(--font-mono)] normal-case tracking-normal text-white/70">
                {new Date(run.queued_at).toLocaleTimeString()}
              </span>
            </div>
          )}
          {run.finished_at && (
            <div className="flex items-center gap-2">
              <span>Finished:</span>
              <span className="font-[family-name:var(--font-mono)] normal-case tracking-normal text-white/70">
                {new Date(run.finished_at).toLocaleTimeString()}
              </span>
            </div>
          )}
        </div>
      </Panel>

      {isActive &&
        sortedAgents
          .filter((a) => isAgentAwaitingHumanInput(a.status))
          .map((a) => (
            <AwaitingHumanBanner
              key={a.id}
              getAccessToken={getAccessToken}
              workspaceId={workspaceId}
              runId={run.id}
              runAgentId={a.id}
              enabled
            />
          ))}

      {/* === Agent Lanes === */}
      <div>
        <h2 className="text-2xs leading-none text-white/75 uppercase tracking-[0.22em] font-medium mb-4">Agent Lanes</h2>
        {arenaMode === "broadcast" ? (
          <RaceModeArena
            agents={sortedAgents}
            lanes={arenaLanes}
            workspaceId={workspaceId}
            runId={run.id}
            winnerAgentId={ranking?.ranking?.winner?.run_agent_id}
            showCommentary={showCommentary}
            commentaryEntries={commentaryEntries}
            isActive={isActive}
            laneFooters={Object.fromEntries(
              sortedAgents
                .filter(
                  (a) => a.status === "completed" || a.status === "failed",
                )
                .map((a) => [
                  a.id,
                  <ScorecardSummaryCard
                    key={a.id}
                    scorecard={scorecards[a.id] ?? null}
                    loading={!(a.id in scorecards)}
                  />,
                ]),
            )}
          />
        ) : (
          <div
            className={`grid gap-4 ${
              showCommentary
                ? "grid-cols-1 xl:grid-cols-[minmax(0,2fr)_minmax(18rem,24rem)]"
                : "grid-cols-1"
            }`}
          >
            <div
              className={`grid gap-3 ${
                agents.length === 1
                  ? "grid-cols-1"
                  : "grid-cols-1 md:grid-cols-2"
              }`}
            >
              {sortedAgents.map((agent) => {
                const isWinner =
                  ranking?.ranking?.winner?.run_agent_id === agent.id;
                const laneState = arenaLanes[agent.id] ?? EMPTY_LANE;
                const isTerminal =
                  agent.status === "completed" || agent.status === "failed";
                const footer = isTerminal ? (
                  <ScorecardSummaryCard
                    scorecard={scorecards[agent.id] ?? null}
                    loading={!(agent.id in scorecards)}
                  />
                ) : null;

                return (
                  <LiveAgentLane
                    key={agent.id}
                    agent={agent}
                    lane={laneState}
                    isWinner={isWinner}
                    workspaceId={workspaceId}
                    runId={run.id}
                    footer={footer}
                  />
                );
              })}
            </div>
            {showCommentary && (
              <div className="xl:sticky xl:top-4 xl:self-start">
                <LiveCommentarySidebar
                  entries={commentaryEntries}
                  isActive={isActive}
                />
              </div>
            )}
          </div>
        )}
      </div>

      {run.regression_coverage &&
        (run.regression_coverage.suites.length > 0 ||
          run.regression_coverage.unmatched_cases.length > 0) && (
          <div>
            <h2 className="text-2xs leading-none text-white/75 uppercase tracking-[0.22em] font-medium mb-3">Regression Coverage</h2>
            <div className="space-y-3">
              {run.regression_coverage.suites.length > 0 && (
                <Panel className="overflow-x-auto">
                  <Table className="text-white/70">
                    <TableHeader className="border-white/10 hover:bg-transparent">
                      <TableRow className="border-white/10 hover:bg-transparent">
                        <TableHead className="text-white/40 uppercase tracking-[0.14em] text-2xs">Suite</TableHead>
                        <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Cases</TableHead>
                        <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Pass</TableHead>
                        <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Fail</TableHead>
                        <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Pending</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {run.regression_coverage.suites.map((suite) => {
                        const pendingCount =
                          suite.case_count - suite.pass_count - suite.fail_count;
                        return (
                          <TableRow key={suite.id} className="border-white/5 hover:bg-white/[0.02]">
                            <TableCell className="font-medium text-white/90">
                              {suite.name}
                            </TableCell>
                            <TableCell className="text-right font-[family-name:var(--font-mono)]">
                              {suite.case_count}
                            </TableCell>
                            <TableCell className="text-right font-[family-name:var(--font-mono)] text-emerald-400">
                              {suite.pass_count}
                            </TableCell>
                            <TableCell className="text-right font-[family-name:var(--font-mono)] text-red-400">
                              {suite.fail_count}
                            </TableCell>
                            <TableCell className="text-right font-[family-name:var(--font-mono)] text-white/40">
                              {pendingCount}
                            </TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                </Panel>
              )}

              {run.regression_coverage.unmatched_cases.length > 0 && (
                <Panel className="p-4">
                  <h3 className="text-2xs font-medium uppercase tracking-[0.14em] text-white/40 mb-3">
                    Unmatched Cases
                  </h3>
                  <div className="flex flex-wrap gap-2">
                    {run.regression_coverage.unmatched_cases.map((item) => (
                      <Badge
                        key={item.id}
                        variant={outcomeVariant(item.outcome)}
                        className="bg-white/5 text-white/70 border-white/10"
                      >
                        {item.title}: {item.outcome}
                      </Badge>
                    ))}
                  </div>
                </Panel>
              )}
            </div>
          </div>
        )}

      {/* === Ranking === */}
      {(run.status === "completed" ||
        run.status === "failed" ||
        run.status === "cancelled") && (
        <div>
          <h2 className="text-2xs leading-none text-white/75 uppercase tracking-[0.22em] font-medium mb-4">Ranking</h2>

          {!ranking || ranking.state === "pending" ? (
            <Panel className="p-8 text-center text-2xs uppercase tracking-[0.14em] text-white/40">
              <Loader2 className="size-5 animate-spin mx-auto mb-3 text-white/20" />
              Scoring in progress...
            </Panel>
          ) : ranking.state === "errored" ? (
            <Panel tone="danger" className="p-8 text-center text-2xs uppercase tracking-[0.14em] text-red-400/80">
              <AlertTriangle className="size-5 mx-auto mb-3 text-red-400/50" />
              {ranking.message || "Ranking unavailable for this run."}
            </Panel>
          ) : ranking.ranking ? (
            <>
              {run.status === "completed" &&
              run.execution_mode === "comparison" ? (
                <RunRankingInsightsCard
                  workspaceId={workspaceId}
                  run={run}
                  ranking={ranking}
                />
              ) : null}

              {/* Sort pills — includes custom dimensions from ranking data */}
              {(() => {
                const sortOptions = buildSortOptions(ranking);
                const customDimKeys = sortOptions
                  .filter(
                    (o) =>
                      !["composite", "correctness", "reliability", "latency", "cost"].includes(o.key),
                  )
                  .map((o) => o.key);

                return (
                  <>
                    <div className="flex gap-2 mb-4 flex-wrap">
                      {sortOptions.map((opt) => (
                        <Button
                          key={opt.key}
                          variant={sortBy === opt.key ? "default" : "outline"}
                          size="sm"
                          className={sortBy === opt.key ? "bg-white/10 text-white hover:bg-white/20" : "bg-transparent border-white/10 text-white/50 hover:text-white hover:bg-white/5"}
                          onClick={() => handleSortChange(opt.key)}
                        >
                          {opt.label}
                        </Button>
                      ))}
                    </div>

                    {/* Strategy badge (if present) */}
                    {ranking.ranking.items[0]?.strategy && (
                      <div className="flex items-center gap-2 mb-4 text-2xs uppercase tracking-[0.14em] text-white/40">
                        <span>Strategy:</span>
                        <Badge variant="outline" className="bg-white/5 text-white/70 border-white/10">
                          {ranking.ranking.items[0].strategy}
                        </Badge>
                      </div>
                    )}

                    <Panel className="overflow-x-auto">
                      <Table className="text-white/70">
                        <TableHeader className="border-white/10 hover:bg-transparent">
                          <TableRow className="border-white/10 hover:bg-transparent">
                            <TableHead className="w-16 text-white/40 uppercase tracking-[0.14em] text-2xs">Rank</TableHead>
                            <TableHead className="text-white/40 uppercase tracking-[0.14em] text-2xs">Agent</TableHead>
                            <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Overall</TableHead>
                            <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Correctness</TableHead>
                            <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Reliability</TableHead>
                            <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Latency</TableHead>
                            <TableHead className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">Cost</TableHead>
                            {customDimKeys.map((key) => (
                              <TableHead key={key} className="text-right text-white/40 uppercase tracking-[0.14em] text-2xs">
                                {key.charAt(0).toUpperCase() + key.slice(1).replace(/_/g, " ")}
                              </TableHead>
                            ))}
                            <TableHead className="text-center text-white/40 uppercase tracking-[0.14em] text-2xs">Pass</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {ranking.ranking.items.map((item: RankingItem) => {
                            const isItemWinner =
                              ranking.ranking?.winner?.run_agent_id ===
                              item.run_agent_id;
                            return (
                              <TableRow
                                key={item.run_agent_id}
                                className={
                                  isItemWinner ? "bg-emerald-500/5 border-white/5 hover:bg-emerald-500/10" : "border-white/5 hover:bg-white/[0.02]"
                                }
                              >
                                <TableCell className="font-medium text-white/90">
                                  {isItemWinner && (
                                    <Trophy className="size-3.5 text-emerald-400 inline mr-2" />
                                  )}
                                  {item.rank}
                                </TableCell>
                                <TableCell className="font-medium text-white/90">
                                  {item.label}
                                  <span className="text-2xs text-white/30 ml-2 font-[family-name:var(--font-mono)]">
                                    #{item.lane_index}
                                  </span>
                                </TableCell>
                                <TableCell className="text-right">
                                  <div className="font-[family-name:var(--font-mono)] text-white/90">{scorePercent(item.overall_score ?? item.composite_score)}</div>
                                  {item.delta_from_top != null &&
                                    item.delta_from_top !== 0 && (
                                      <div className="text-2xs text-white/40 font-[family-name:var(--font-mono)] mt-0.5">
                                        {deltaLabel(item.delta_from_top)}
                                      </div>
                                    )}
                                </TableCell>
                                <TableCell className="text-right font-[family-name:var(--font-mono)]">
                                  {scorePercent(item.correctness_score)}
                                </TableCell>
                                <TableCell className="text-right font-[family-name:var(--font-mono)]">
                                  {scorePercent(item.reliability_score)}
                                </TableCell>
                                <TableCell className="text-right font-[family-name:var(--font-mono)]">
                                  {scorePercent(item.latency_score)}
                                </TableCell>
                                <TableCell className="text-right font-[family-name:var(--font-mono)]">
                                  {scorePercent(item.cost_score)}
                                </TableCell>
                                {customDimKeys.map((key) => (
                                  <TableCell key={key} className="text-right font-[family-name:var(--font-mono)]">
                                    {scorePercent(item.dimensions?.[key]?.score)}
                                  </TableCell>
                                ))}
                                <TableCell className="text-center">
                                  {item.passed != null ? (
                                    item.passed ? (
                                      <CheckCircle2 className="size-4 text-emerald-400 inline" />
                                    ) : (
                                      <XCircle className="size-4 text-red-400 inline" />
                                    )
                                  ) : (
                                    <span className="text-white/20">{"\u2014"}</span>
                                  )}
                                </TableCell>
                              </TableRow>
                            );
                          })}
                        </TableBody>
                      </Table>
                    </Panel>
                  </>
                );
              })()}
            </>
          ) : null}
        </div>
      )}
    </div>
  );
}
