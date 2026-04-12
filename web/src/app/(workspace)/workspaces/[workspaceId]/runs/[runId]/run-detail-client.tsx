"use client";

import { useState, useEffect, useCallback } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type {
  Run,
  RunStatus,
  RunAgent,
  RunAgentStatus,
  RunRankingResponse,
  RankingItem,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  ExternalLink,
  Trophy,
  Clock,
  Play,
  CheckCircle2,
  XCircle,
  Loader2,
  AlertTriangle,
} from "lucide-react";

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

const SORT_OPTIONS = [
  { key: "composite", label: "Composite" },
  { key: "correctness", label: "Correctness" },
  { key: "reliability", label: "Reliability" },
  { key: "latency", label: "Latency" },
  { key: "cost", label: "Cost" },
] as const;

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

function scorePercent(score?: number): string {
  if (score == null) return "\u2014";
  return `${(score * 100).toFixed(1)}%`;
}

function deltaLabel(delta?: number): string {
  if (delta == null || delta === 0) return "";
  const pct = (delta * 100).toFixed(1);
  return delta > 0 ? `+${pct}%` : `${pct}%`;
}

// --- Component ---

interface RunDetailClientProps {
  initialRun: Run;
  initialAgents: RunAgent[];
  apiBaseUrl: string;
}

export function RunDetailClient({
  initialRun,
  initialAgents,
  apiBaseUrl,
}: RunDetailClientProps) {
  const { getAccessToken } = useAccessToken();
  const [run, setRun] = useState<Run>(initialRun);
  const [agents, setAgents] = useState<RunAgent[]>(initialAgents);
  const [ranking, setRanking] = useState<RunRankingResponse | null>(null);
  const [sortBy, setSortBy] = useState("composite");

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

  // Auto-refresh
  useEffect(() => {
    if (!isActive) return;
    const interval = setInterval(fetchAll, POLL_MS);
    return () => clearInterval(interval);
  }, [isActive, fetchAll]);

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
          <Badge variant="outline">
            {run.execution_mode === "comparison"
              ? "Comparison"
              : "Single Agent"}
          </Badge>
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
        <div
          className={`grid gap-3 ${
            agents.length === 1 ? "grid-cols-1" : "grid-cols-1 md:grid-cols-2"
          }`}
        >
          {sortedAgents.map((agent) => {
            const isFailed = agent.status === "failed";
            const isRunning = ACTIVE_AGENT_STATUSES.includes(agent.status);
            const isWinner =
              ranking?.ranking?.winner?.run_agent_id === agent.id;

            return (
              <div
                key={agent.id}
                className={`rounded-lg border p-4 ${
                  isWinner
                    ? "border-emerald-500/40 bg-emerald-500/5"
                    : isFailed
                      ? "border-destructive/30 bg-destructive/5"
                      : "border-border"
                }`}
              >
                {/* Card header */}
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    {isWinner && (
                      <Trophy className="size-4 text-emerald-400" />
                    )}
                    <span className="font-medium text-sm">{agent.label}</span>
                    <span className="text-xs text-muted-foreground/50">
                      #{agent.lane_index}
                    </span>
                  </div>
                  <Badge
                    variant={agentStatusVariant[agent.status] ?? "outline"}
                  >
                    {isRunning && (
                      <Loader2
                        data-icon="inline-start"
                        className="size-3 animate-spin"
                      />
                    )}
                    {agent.status}
                  </Badge>
                </div>

                {/* Timing */}
                <div className="flex gap-4 text-xs text-muted-foreground mb-2">
                  {agent.started_at && (
                    <span>
                      {agent.finished_at
                        ? formatDuration(agent.started_at, agent.finished_at)
                        : `Running ${formatDuration(agent.started_at)}`}
                    </span>
                  )}
                  {!agent.started_at && agent.queued_at && (
                    <span>Queued</span>
                  )}
                </div>

                {/* Failure reason — auto-expanded */}
                {isFailed && agent.failure_reason && (
                  <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive mb-2">
                    <div className="flex items-center gap-1.5 font-medium mb-0.5">
                      <XCircle className="size-3.5" />
                      Failed
                    </div>
                    <p className="text-destructive/80">
                      {agent.failure_reason}
                    </p>
                  </div>
                )}

                {/* Action links */}
                <div className="flex gap-2 mt-1">
                  <a
                    href={`${apiBaseUrl}/v1/replays/${agent.id}/viewer`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
                  >
                    <Play className="size-3" />
                    Replay
                    <ExternalLink className="size-2.5" />
                  </a>
                  <a
                    href={`${apiBaseUrl}/v1/scorecards/${agent.id}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
                  >
                    <CheckCircle2 className="size-3" />
                    Scorecard
                    <ExternalLink className="size-2.5" />
                  </a>
                </div>
              </div>
            );
          })}
        </div>
      </div>

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
              {/* Sort pills */}
              <div className="flex gap-1.5 mb-3">
                {SORT_OPTIONS.map((opt) => (
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

              <div className="rounded-lg border border-border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-16">Rank</TableHead>
                      <TableHead>Agent</TableHead>
                      <TableHead className="text-right">Composite</TableHead>
                      <TableHead className="text-right">Correctness</TableHead>
                      <TableHead className="text-right">Reliability</TableHead>
                      <TableHead className="text-right">Latency</TableHead>
                      <TableHead className="text-right">Cost</TableHead>
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
                            <div>{scorePercent(item.composite_score)}</div>
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
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
            </>
          ) : null}
        </div>
      )}
    </div>
  );
}
