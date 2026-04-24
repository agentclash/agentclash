"use client";

import { useMemo, useState } from "react";
import {
  AlertTriangle,
  BarChart3,
  BrainCircuit,
  CheckCircle2,
  Clock,
  Layers,
  Terminal,
  Trophy,
  Wrench,
  XCircle,
} from "lucide-react";

import {
  DimensionsDeck,
} from "@/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/agents/[runAgentId]/scorecard/components/dimensions-deck";
import {
  JudgesPanel,
  MetricsPanel,
  ValidatorsPanel,
  type InspectorTarget,
} from "@/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/agents/[runAgentId]/scorecard/components/evidence-panels";
import { Hero } from "@/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/agents/[runAgentId]/scorecard/components/hero";
import { InspectorSheet } from "@/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/agents/[runAgentId]/scorecard/components/inspector-sheet";
import { ScorecardSummaryCard } from "@/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/scorecard-summary-card";
import { ReplayTimeline } from "@/components/replay/replay-timeline";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TooltipProvider } from "@/components/ui/tooltip";
import { scorePercent } from "@/lib/scores";
import type {
  ReplayResponse,
  ReplaySummary,
  ReplayStep,
  Run,
  RunAgent,
  RunAgentStatus,
  ScorecardResponse,
} from "@/lib/api/types";

type SharedResource = Record<string, unknown>;
type PublicScorecardRecord = Record<string, unknown>;

const runStatusVariant: Record<
  string,
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

export function PublicShareRenderer({ resource }: { resource: SharedResource }) {
  if (resource.type === "challenge_pack_version") {
    return <PublicChallengePack resource={resource} />;
  }
  if (resource.type === "run_scorecard") {
    return <PublicRun resource={resource} />;
  }
  if (resource.type === "run_agent_replay") {
    return <PublicReplay resource={resource} />;
  }
  if (resource.type === "run_agent_scorecard") {
    return <PublicScorecard resource={resource} />;
  }
  return (
    <EmptyState
      icon={<AlertTriangle className="size-10 text-muted-foreground" />}
      title="Unsupported share"
      description="This shared AgentClash artifact type is not available yet."
    />
  );
}

function PublicChallengePack({ resource }: { resource: SharedResource }) {
  const pack = asRecord(resource.pack);
  const version = asRecord(resource.version);
  const yaml = useMemo(() => {
    return toYaml({
      pack,
      version: {
        version_number: version.version_number,
        lifecycle_status: version.lifecycle_status,
        manifest: version.manifest,
        input_sets: version.input_sets ?? [],
      },
    });
  }, [pack, version]);

  return (
    <section className="rounded-lg border border-border bg-card overflow-hidden">
      <div className="border-b border-border px-4 py-3">
        <h2 className="text-sm font-semibold">Challenge pack YAML</h2>
        <p className="mt-1 text-xs text-muted-foreground">
          {String(pack.name ?? "Challenge pack")} v
          {String(version.version_number ?? "")}
        </p>
      </div>
      <pre className="max-h-[76vh] overflow-auto p-4 text-xs leading-relaxed font-[family-name:var(--font-mono)] whitespace-pre-wrap">
        {yaml}
      </pre>
    </section>
  );
}

function PublicRun({ resource }: { resource: SharedResource }) {
  const run = resource.run as Run;
  const agents = sortedAgents(asArray<RunAgent>(resource.agents));
  const scorecards = scorecardsByAgent(
    asArray<PublicScorecardRecord>(resource.agent_scorecards),
    run,
  );
  const ranked = rankAgentsByScore(agents, scorecards);

  return (
    <div className="space-y-8">
      <div>
        <div className="flex flex-wrap items-center gap-3 mb-2">
          <h1 className="text-lg font-semibold tracking-tight">{run.name}</h1>
          <Badge variant={runStatusVariant[run.status] ?? "outline"}>
            {run.status}
          </Badge>
          <Badge variant="outline">
            {run.execution_mode === "comparison" ? "Comparison" : "Single Agent"}
          </Badge>
        </div>
        <div className="flex flex-wrap gap-6 text-sm text-muted-foreground">
          <RunDuration run={run} />
          <div>
            Agents:{" "}
            <span className="text-foreground font-medium">{agents.length}</span>
          </div>
          {run.finished_at && (
            <div>Finished: {new Date(run.finished_at).toLocaleTimeString()}</div>
          )}
        </div>
      </div>

      <section>
        <h2 className="text-sm font-semibold mb-3">Agent Lanes</h2>
        <div
          className={`grid gap-3 ${
            agents.length === 1 ? "grid-cols-1" : "grid-cols-1 md:grid-cols-2"
          }`}
        >
          {agents.map((agent) => (
            <PublicAgentLane
              key={agent.id}
              agent={agent}
              isWinner={ranked[0]?.agent.id === agent.id}
              scorecard={scorecards[agent.id] ?? null}
            />
          ))}
        </div>
      </section>

      {ranked.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold mb-3">Ranking</h2>
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
                  <TableHead className="text-center">Pass</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {ranked.map(({ agent, scorecard }, index) => (
                  <TableRow
                    key={agent.id}
                    className={index === 0 ? "bg-emerald-500/5" : undefined}
                  >
                    <TableCell className="font-medium">
                      {index === 0 && (
                        <Trophy className="size-3.5 text-emerald-400 inline mr-1.5" />
                      )}
                      {index + 1}
                    </TableCell>
                    <TableCell className="font-medium">
                      {agent.label}
                      <span className="text-xs text-muted-foreground/50 ml-1.5">
                        #{agent.lane_index}
                      </span>
                    </TableCell>
                    <TableCell className="text-right">
                      {scorePercent(scorecard?.overall_score)}
                    </TableCell>
                    <TableCell className="text-right">
                      {scorePercent(scorecard?.correctness_score)}
                    </TableCell>
                    <TableCell className="text-right">
                      {scorePercent(scorecard?.reliability_score)}
                    </TableCell>
                    <TableCell className="text-right">
                      {scorePercent(scorecard?.latency_score)}
                    </TableCell>
                    <TableCell className="text-right">
                      {scorePercent(scorecard?.cost_score)}
                    </TableCell>
                    <TableCell className="text-center">
                      {scorecard?.scorecard?.passed != null ? (
                        scorecard.scorecard.passed ? (
                          <CheckCircle2 className="size-4 text-emerald-400 inline" />
                        ) : (
                          <XCircle className="size-4 text-red-400 inline" />
                        )
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </section>
      )}
    </div>
  );
}

function PublicReplay({ resource }: { resource: SharedResource }) {
  const run = resource.run as Run;
  const agent = resource.run_agent as RunAgent;
  const replay = buildReplayResponse(resource);
  const counts = replay.replay?.summary?.counts;

  return (
    <div className="space-y-6">
      <div>
        <div className="flex items-center gap-3 mb-1">
          <h1 className="text-lg font-semibold tracking-tight">{agent.label}</h1>
          <Badge variant={agentStatusVariant[agent.status] ?? "outline"}>
            {agent.status}
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground">Replay for {run.name}</p>
      </div>

      {counts && (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
          <StatCard icon={<Layers className="size-4" />} label="Total Steps" value={counts.replay_steps} />
          <StatCard icon={<BrainCircuit className="size-4" />} label="Model Calls" value={counts.model_calls} />
          <StatCard icon={<Wrench className="size-4" />} label="Tool Calls" value={counts.tool_calls} />
          <StatCard icon={<Terminal className="size-4" />} label="Sandbox Cmds" value={counts.sandbox_commands} />
          <StatCard icon={<BarChart3 className="size-4" />} label="Scoring" value={counts.scoring_events} />
        </div>
      )}

      {replay.replay?.summary?.terminal_state && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Clock className="size-3.5" />
          <span>{replay.replay.summary.terminal_state.headline}</span>
          {replay.replay.summary.terminal_state.error_message && (
            <span className="text-destructive">
              - {replay.replay.summary.terminal_state.error_message}
            </span>
          )}
        </div>
      )}

      <ReplayTimeline
        steps={replay.steps.map((step) => ({ ...step, artifact_ids: [] }))}
        hasMore={false}
        isLoadingMore={false}
        onLoadMore={() => undefined}
      />
    </div>
  );
}

function PublicScorecard({ resource }: { resource: SharedResource }) {
  const run = resource.run as Run;
  const agent = resource.run_agent as RunAgent;
  const scorecard = toScorecardResponse(asRecord(resource.scorecard), run, agent);
  const [inspected, setInspected] = useState<InspectorTarget | null>(null);
  const validators = scorecard.scorecard?.validator_details ?? [];
  const metrics = scorecard.scorecard?.metric_details ?? [];
  const judges = scorecard.llm_judge_results ?? [];

  return (
    <TooltipProvider delay={250}>
      <div className="space-y-5 rounded-lg bg-[#050505] p-4 text-white">
        <Hero run={run} agent={agent} scorecard={scorecard} />
        <PublicComparisonStrip
          agents={asArray<RunAgent>(resource.sibling_agents)}
          scorecards={scorecardsByAgent(
            asArray<PublicScorecardRecord>(resource.agent_scorecards),
            run,
          )}
          currentRunAgentId={agent.id}
        />
        <DimensionsDeck scorecard={scorecard} />
        <ValidatorsPanel validators={validators} onInspect={setInspected} />
        <MetricsPanel metrics={metrics} onInspect={setInspected} />
        <JudgesPanel judges={judges} onInspect={setInspected} />
        <InspectorSheet target={inspected} onClose={() => setInspected(null)} />
      </div>
    </TooltipProvider>
  );
}

function PublicComparisonStrip({
  agents,
  scorecards,
  currentRunAgentId,
}: {
  agents: RunAgent[];
  scorecards: Record<string, ScorecardResponse>;
  currentRunAgentId: string;
}) {
  const ranked = rankAgentsByScore(sortedAgents(agents), scorecards);
  if (ranked.length <= 1) return null;
  return (
    <div className="rounded-md border border-white/[0.08] bg-white/[0.015]">
      <div className="flex items-center gap-2.5 px-4 h-11 border-b border-white/[0.06]">
        <Trophy className="size-3.5 text-white/40" />
        <h2 className="text-[11px] leading-none text-white/75 uppercase tracking-[0.22em] font-medium">
          Comparison
        </h2>
      </div>
      <div className="divide-y divide-white/[0.05]">
        {ranked.map(({ agent, scorecard }, index) => {
          const current = agent.id === currentRunAgentId;
          return (
            <div
              key={agent.id}
              className={`flex items-center gap-3 px-4 py-3 ${
                current ? "bg-white/[0.04]" : ""
              }`}
            >
              <span className="w-8 text-[11px] font-[family-name:var(--font-mono)] text-white/45">
                #{index + 1}
              </span>
              <span className="min-w-0 flex-1 truncate text-sm text-white/85">
                {agent.label}
              </span>
              <span className="text-xs font-[family-name:var(--font-mono)] text-white/65">
                {scorePercent(scorecard?.overall_score)}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function PublicAgentLane({
  agent,
  isWinner,
  scorecard,
}: {
  agent: RunAgent;
  isWinner: boolean;
  scorecard: ScorecardResponse | null;
}) {
  const isFailed = agent.status === "failed";
  return (
    <div
      className={`rounded-lg border p-4 ${
        isWinner
          ? "border-emerald-500/40 bg-emerald-500/5"
          : isFailed
            ? "border-destructive/30 bg-destructive/5"
            : "border-border"
      }`}
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2 min-w-0">
          {isWinner && <Trophy className="size-4 shrink-0 text-emerald-400" />}
          <span className="font-medium text-sm truncate">{agent.label}</span>
          <span className="text-xs text-muted-foreground/50">
            #{agent.lane_index}
          </span>
        </div>
        <Badge variant={agentStatusVariant[agent.status] ?? "outline"}>
          {agent.status}
        </Badge>
      </div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground mb-3">
        <span>{formatAgentDuration(agent)}</span>
      </div>
      {isFailed && agent.failure_reason && (
        <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive mb-2">
          <div className="flex items-center gap-1.5 font-medium mb-0.5">
            <XCircle className="size-3.5" />
            Failed
          </div>
          <p className="text-destructive/80">{agent.failure_reason}</p>
        </div>
      )}
      <ScorecardSummaryCard scorecard={scorecard} />
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

function RunDuration({ run }: { run: Run }) {
  if (!run.started_at) return <div>Waiting to start</div>;
  return (
    <div className="flex items-center gap-1.5">
      <Clock className="size-3.5" />
      Duration:{" "}
      <span className="text-foreground font-medium">
        {formatDuration(run.started_at, run.finished_at)}
      </span>
    </div>
  );
}

function sortedAgents(agents: RunAgent[]) {
  return [...agents].sort((a, b) => a.lane_index - b.lane_index);
}

function rankAgentsByScore(
  agents: RunAgent[],
  scorecards: Record<string, ScorecardResponse>,
) {
  return agents
    .map((agent) => ({ agent, scorecard: scorecards[agent.id] }))
    .filter(({ scorecard }) => scorecard?.state === "ready")
    .sort(
      (a, b) =>
        (b.scorecard?.overall_score ?? -1) -
          (a.scorecard?.overall_score ?? -1) ||
        a.agent.lane_index - b.agent.lane_index,
    );
}

function scorecardsByAgent(
  rawScorecards: PublicScorecardRecord[],
  run: Run,
): Record<string, ScorecardResponse> {
  return Object.fromEntries(
    rawScorecards.map((scorecard) => [
      String(scorecard.run_agent_id),
      toScorecardResponse(scorecard, run),
    ]),
  );
}

function toScorecardResponse(
  raw: PublicScorecardRecord,
  run: Run,
  agent?: RunAgent,
): ScorecardResponse {
  return {
    state: raw.state === "pending" || raw.state === "errored" ? raw.state : "ready",
    message: typeof raw.message === "string" ? raw.message : undefined,
    run_agent_status: agent?.status ?? "completed",
    id: String(raw.id ?? ""),
    run_agent_id: String(raw.run_agent_id ?? ""),
    run_id: run.id,
    evaluation_spec_id: String(raw.evaluation_spec_id ?? ""),
    overall_score: optionalNumber(raw.overall_score),
    correctness_score: optionalNumber(raw.correctness_score),
    reliability_score: optionalNumber(raw.reliability_score),
    latency_score: optionalNumber(raw.latency_score),
    cost_score: optionalNumber(raw.cost_score),
    behavioral_score: optionalNumber(raw.behavioral_score),
    llm_judge_results: Array.isArray(raw.llm_judge_results)
      ? raw.llm_judge_results
      : [],
    scorecard: raw.scorecard as ScorecardResponse["scorecard"],
    created_at: String(raw.created_at ?? ""),
    updated_at: String(raw.updated_at ?? ""),
  };
}

function optionalNumber(value: unknown): number | undefined {
  return typeof value === "number" ? value : undefined;
}

function buildReplayResponse(resource: SharedResource): ReplayResponse {
  const run = resource.run as Run;
  const agent = resource.run_agent as RunAgent;
  const rawReplay = asRecord(resource.replay);
  const summary = asRecord(rawReplay.summary);
  const rawSteps = asArray<ReplayStep>(summary.steps);
  return {
    state: "ready",
    run_agent_id: agent.id,
    run_id: run.id,
    run_agent_status: agent.status,
    replay: {
      id: String(rawReplay.id ?? ""),
      summary: summary as unknown as ReplaySummary,
      latest_sequence_number: optionalNumber(rawReplay.latest_sequence_number),
      event_count: optionalNumber(rawReplay.event_count) ?? rawSteps.length,
      created_at: String(rawReplay.created_at ?? ""),
      updated_at: String(rawReplay.updated_at ?? ""),
    },
    steps: rawSteps,
    pagination: {
      limit: rawSteps.length,
      total_steps: rawSteps.length,
      has_more: false,
    },
  };
}

function formatAgentDuration(agent: RunAgent) {
  return formatDuration(agent.started_at, agent.finished_at);
}

function formatDuration(start?: string, end?: string): string {
  if (!start) return "-";
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const ms = Math.max(0, e - s);
  if (ms < 1000) return "<1s";
  const secs = Math.floor(ms / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  return `${mins}m ${secs % 60}s`;
}

function toYaml(value: unknown, indent = 0): string {
  const pad = " ".repeat(indent);
  if (Array.isArray(value)) {
    if (value.length === 0) return "[]\n";
    return value
      .map((item) => {
        if (isScalar(item)) return `${pad}- ${formatScalar(item)}\n`;
        return `${pad}-\n${toYaml(item, indent + 2)}`;
      })
      .join("");
  }
  if (value && typeof value === "object") {
    return Object.entries(value as Record<string, unknown>)
      .filter(([, child]) => child !== undefined)
      .map(([key, child]) => {
        if (isScalar(child)) return `${pad}${key}: ${formatScalar(child)}\n`;
        return `${pad}${key}:\n${toYaml(child, indent + 2)}`;
      })
      .join("");
  }
  return `${pad}${formatScalar(value)}\n`;
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function asArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : [];
}

function isScalar(value: unknown) {
  return value == null || typeof value !== "object";
}

function formatScalar(value: unknown) {
  if (value == null) return "null";
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  const text = String(value);
  if (/^[a-zA-Z0-9_.:/@ -]+$/.test(text) && text.trim() === text) {
    return text;
  }
  return JSON.stringify(text);
}
