"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";

import { LiveAgentLane } from "@/components/arena/live-agent-lane";
import { LiveCommentarySidebar } from "@/components/arena/live-commentary-sidebar";
import { RaceModeArena } from "@/components/arena/race-mode";
import { UploadArtifactDialog } from "@/components/artifacts/upload-artifact-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useAgentArena, EMPTY_LANE } from "@/hooks/use-agent-arena";
import { useAgentCommentary } from "@/hooks/use-agent-commentary";
import { useArenaMode } from "@/hooks/use-arena-mode";
import { useRunEvents, type RunEvent } from "@/hooks/use-run-events";
import { createApiClient } from "@/lib/api/client";
import { scorePercent } from "@/lib/scores";
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
} from "lucide-react";

import { CompareRunPicker } from "./compare-run-picker";
import { RunRankingInsightsCard } from "./run-ranking-insights-card";
import { ScorecardSummaryCard } from "./scorecard-summary-card";

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

const ACTIVE_RUN_STATUSES: RunStatus[] = [
  "queued",
  "provisioning",
  "running",
  "scoring",
];
const ACTIVE_AGENT_STATUSES: RunAgentStatus[] = [
  "queued",
  "ready",
  "executing",
  "evaluating",
];
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
    ACTIVE_RUN_STATUSES.includes(run.status) ||
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

  return (
    <div className="space-y-8">
      {/* === Header === */}
      <div>
        <div className="flex items-center gap-3 mb-2">
          <h1 className="text-lg font-semibold tracking-tight">{run.name}</h1>
          <Badge variant={runStatusVariant[run.status] ?? "outline"}>
            {isActive && (
              <Loader2
                data-icon="inline-start"
                className="size-3 animate-spin"
              />
            )}
            {run.status}
          </Badge>
          {isActive && sseConnected && (
            <span
              className="inline-flex items-center gap-1 rounded-md border border-primary/30 bg-primary/10 px-1.5 h-6 text-[10px] font-medium uppercase tracking-wider text-primary"
              title="Streaming live events"
            >
              <Radio className="size-3 animate-pulse" />
              Live
            </span>
          )}
          <Badge variant="outline">
            {run.execution_mode === "comparison"
              ? "Comparison"
              : "Single Agent"}
          </Badge>
          <CompareRunPicker
            currentRunId={run.id}
            workspaceId={workspaceId}
          />
          <UploadArtifactDialog
            workspaceId={workspaceId}
            runId={run.id}
          />
          <Button
            variant={showCommentary ? "default" : "outline"}
            size="sm"
            onClick={() =>
              setShowCommentary((current) => {
                if (current) resetCommentary();
                return !current;
              })
            }
            aria-pressed={showCommentary}
          >
            <MessageSquareText className="size-3.5" />
            Commentary {showCommentary ? "On" : "Off"}
          </Button>
          <Button
            variant={arenaMode === "race" ? "default" : "outline"}
            size="sm"
            onClick={() =>
              setArenaMode(arenaMode === "race" ? "dev" : "race")
            }
            aria-pressed={arenaMode === "race"}
            title={
              arenaMode === "race"
                ? "Switch to development (classic) view"
                : "Switch to race mode — the broadcast view"
            }
          >
            <Flag className="size-3.5" />
            {arenaMode === "race" ? "Race Mode" : "Dev Mode"}
          </Button>
          <Link
            href={`/workspaces/${workspaceId}/runs/${run.id}/failures`}
            className="inline-flex items-center gap-1.5 rounded-md border border-border px-2.5 h-8 text-xs text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors"
          >
            <AlertOctagon className="size-3.5" />
            Failures
          </Link>
        </div>

        {/* KPI strip */}
        <div className="flex flex-wrap gap-6 text-sm text-muted-foreground">
          <div className="flex items-center gap-1.5">
            <Clock className="size-3.5" />
            {run.started_at ? (
              <span>
                Duration:{" "}
                <span className="text-foreground font-medium">
                  {formatDuration(run.started_at, run.finished_at)}
                </span>
              </span>
            ) : (
              <span>Waiting to start</span>
            )}
          </div>
          <div>
            Agents:{" "}
            <span className="text-foreground font-medium">{agents.length}</span>
          </div>
          <div>
            Run ID:{" "}
            <code className="text-xs font-[family-name:var(--font-mono)]">
              {run.id.slice(0, 8)}
            </code>
          </div>
          {run.queued_at && (
            <div>
              Queued: {new Date(run.queued_at).toLocaleTimeString()}
            </div>
          )}
          {run.finished_at && (
            <div>
              Finished: {new Date(run.finished_at).toLocaleTimeString()}
            </div>
          )}
        </div>
      </div>

      {/* === Agent Lanes === */}
      <div>
        <h2 className="text-sm font-semibold mb-3">Agent Lanes</h2>
        {arenaMode === "race" ? (
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
            <h2 className="text-sm font-semibold mb-3">Regression Coverage</h2>
            <div className="space-y-3">
              {run.regression_coverage.suites.length > 0 && (
                <div className="rounded-lg border border-border overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Suite</TableHead>
                        <TableHead className="text-right">Cases</TableHead>
                        <TableHead className="text-right">Pass</TableHead>
                        <TableHead className="text-right">Fail</TableHead>
                        <TableHead className="text-right">Pending</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {run.regression_coverage.suites.map((suite) => {
                        const pendingCount =
                          suite.case_count - suite.pass_count - suite.fail_count;
                        return (
                          <TableRow key={suite.id}>
                            <TableCell className="font-medium">
                              {suite.name}
                            </TableCell>
                            <TableCell className="text-right">
                              {suite.case_count}
                            </TableCell>
                            <TableCell className="text-right">
                              {suite.pass_count}
                            </TableCell>
                            <TableCell className="text-right">
                              {suite.fail_count}
                            </TableCell>
                            <TableCell className="text-right">
                              {pendingCount}
                            </TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                </div>
              )}

              {run.regression_coverage.unmatched_cases.length > 0 && (
                <div className="rounded-lg border border-border p-4">
                  <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2">
                    Unmatched Cases
                  </h3>
                  <div className="flex flex-wrap gap-2">
                    {run.regression_coverage.unmatched_cases.map((item) => (
                      <Badge
                        key={item.id}
                        variant={outcomeVariant(item.outcome)}
                      >
                        {item.title}: {item.outcome}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

      {/* === Ranking === */}
      {(run.status === "completed" ||
        run.status === "failed" ||
        run.status === "cancelled") && (
        <div>
          <h2 className="text-sm font-semibold mb-3">Ranking</h2>

          {!ranking || ranking.state === "pending" ? (
            <div className="rounded-lg border border-border p-6 text-center text-sm text-muted-foreground">
              <Loader2 className="size-5 animate-spin mx-auto mb-2" />
              Scoring in progress...
            </div>
          ) : ranking.state === "errored" ? (
            <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-6 text-center text-sm text-destructive">
              <AlertTriangle className="size-5 mx-auto mb-2" />
              {ranking.message || "Ranking unavailable for this run."}
            </div>
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
                    <div className="flex gap-1.5 mb-3 flex-wrap">
                      {sortOptions.map((opt) => (
                        <Button
                          key={opt.key}
                          variant={sortBy === opt.key ? "default" : "outline"}
                          size="xs"
                          onClick={() => handleSortChange(opt.key)}
                        >
                          {opt.label}
                        </Button>
                      ))}
                    </div>

                    {/* Strategy badge (if present) */}
                    {ranking.ranking.items[0]?.strategy && (
                      <div className="flex items-center gap-2 mb-3 text-xs text-muted-foreground">
                        <span>Strategy:</span>
                        <Badge variant="outline">
                          {ranking.ranking.items[0].strategy}
                        </Badge>
                      </div>
                    )}

                    <div className="rounded-lg border border-border overflow-x-auto">
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead className="w-16">Rank</TableHead>
                            <TableHead>Agent</TableHead>
                            <TableHead className="text-right">Overall</TableHead>
                            <TableHead className="text-right">Correctness</TableHead>
                            <TableHead className="text-right">Reliability</TableHead>
                            <TableHead className="text-right">Latency</TableHead>
                            <TableHead className="text-right">Cost</TableHead>
                            {customDimKeys.map((key) => (
                              <TableHead key={key} className="text-right">
                                {key.charAt(0).toUpperCase() + key.slice(1).replace(/_/g, " ")}
                              </TableHead>
                            ))}
                            <TableHead className="text-center">Pass</TableHead>
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
                                  isItemWinner ? "bg-emerald-500/5" : undefined
                                }
                              >
                                <TableCell className="font-medium">
                                  {isItemWinner && (
                                    <Trophy className="size-3.5 text-emerald-400 inline mr-1.5" />
                                  )}
                                  {item.rank}
                                </TableCell>
                                <TableCell className="font-medium">
                                  {item.label}
                                  <span className="text-xs text-muted-foreground/50 ml-1.5">
                                    #{item.lane_index}
                                  </span>
                                </TableCell>
                                <TableCell className="text-right">
                                  <div>{scorePercent(item.overall_score ?? item.composite_score)}</div>
                                  {item.delta_from_top != null &&
                                    item.delta_from_top !== 0 && (
                                      <div className="text-xs text-muted-foreground">
                                        {deltaLabel(item.delta_from_top)}
                                      </div>
                                    )}
                                </TableCell>
                                <TableCell className="text-right">
                                  {scorePercent(item.correctness_score)}
                                </TableCell>
                                <TableCell className="text-right">
                                  {scorePercent(item.reliability_score)}
                                </TableCell>
                                <TableCell className="text-right">
                                  {scorePercent(item.latency_score)}
                                </TableCell>
                                <TableCell className="text-right">
                                  {scorePercent(item.cost_score)}
                                </TableCell>
                                {customDimKeys.map((key) => (
                                  <TableCell key={key} className="text-right">
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
                                    <span className="text-muted-foreground">{"\u2014"}</span>
                                  )}
                                </TableCell>
                              </TableRow>
                            );
                          })}
                        </TableBody>
                      </Table>
                    </div>
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
