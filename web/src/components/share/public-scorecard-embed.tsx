"use client";

import {
  BarChart3,
  CheckCircle2,
  Clock3,
  DollarSign,
  ShieldCheck,
  Target,
  Trophy,
  XCircle,
  Zap,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { barColor, barWidth, scorePercent } from "@/lib/scores";

type SharedResource = Record<string, unknown>;
type ScorecardRecord = Record<string, unknown>;

type PublicRun = {
  id: string;
  name: string;
  status: string;
  execution_mode?: string;
  started_at?: string;
  finished_at?: string;
};

type PublicAgent = {
  id: string;
  label: string;
  lane_index: number;
  status: string;
  started_at?: string;
  finished_at?: string;
};

const dimensionMeta: Record<
  string,
  { label: string; icon: typeof Target }
> = {
  correctness: { label: "Correctness", icon: Target },
  reliability: { label: "Reliability", icon: ShieldCheck },
  latency: { label: "Latency", icon: Zap },
  cost: { label: "Cost", icon: DollarSign },
  behavioral: { label: "Behavior", icon: BarChart3 },
};

export function PublicScorecardEmbed({
  resource,
}: {
  resource: SharedResource;
}) {
  if (resource.type === "run_agent_scorecard") {
    return <AgentScorecardEmbed resource={resource} />;
  }
  return <RunScorecardEmbed resource={resource} />;
}

function RunScorecardEmbed({ resource }: { resource: SharedResource }) {
  const run = toRun(resource.run);
  const agents = asArray(resource.agents).map(toAgent).sort(sortAgents);
  const scorecards = scorecardsByAgent(asArray(resource.agent_scorecards));
  const runScorecard = asRecord(resource.scorecard);
  const winningAgentID = String(runScorecard.winning_run_agent_id ?? "");
  const ranked = rankAgents(agents, scorecards, winningAgentID);
  const winner = ranked.find((item) => item.agent.id === winningAgentID)?.agent;
  const topScore = optionalNumber(ranked[0]?.scorecard?.overall_score);

  return (
    <article
      className="min-h-screen bg-background text-foreground"
      data-agentclash-embed="scorecard"
    >
      <div className="mx-auto w-full max-w-3xl border border-border bg-card">
        <EmbedHeader
          title={run.name || "Shared run"}
          eyebrow="AgentClash scorecard"
          status={run.status}
          score={topScore}
          passed={scorePassed(ranked[0]?.scorecard)}
        />

        <section className="grid gap-0 border-t border-border sm:grid-cols-3">
          <MetricCell label="Agents" value={String(agents.length)} />
          <MetricCell label="Winner" value={winner?.label ?? "Pending"} />
          <MetricCell label="Duration" value={formatDuration(run.started_at, run.finished_at)} />
        </section>

        {ranked.length > 0 ? (
          <section className="border-t border-border">
            <div className="px-4 py-3 text-sm font-medium">Ranking</div>
            <div className="divide-y divide-border">
              {ranked.map(({ agent, scorecard }, index) => (
                <AgentRow
                  key={agent.id}
                  rank={index + 1}
                  agent={agent}
                  scorecard={scorecard}
                  winner={agent.id === winningAgentID}
                />
              ))}
            </div>
          </section>
        ) : (
          <EmptyEmbedState text="Scorecard results are not ready yet." />
        )}

        <EmbedFooter />
      </div>
    </article>
  );
}

function AgentScorecardEmbed({ resource }: { resource: SharedResource }) {
  const run = toRun(resource.run);
  const agent = toAgent(resource.run_agent);
  const scorecard = asRecord(resource.scorecard);
  const dimensions = scorecardDimensions(scorecard);

  return (
    <article
      className="min-h-screen bg-background text-foreground"
      data-agentclash-embed="scorecard"
    >
      <div className="mx-auto w-full max-w-2xl border border-border bg-card">
        <EmbedHeader
          title={agent.label || "Shared agent"}
          eyebrow={run.name || "AgentClash scorecard"}
          status={agent.status}
          score={optionalNumber(scorecard.overall_score)}
          passed={scorePassed(scorecard)}
        />

        <section className="grid gap-0 border-t border-border sm:grid-cols-3">
          <MetricCell label="Lane" value={`#${agent.lane_index}`} />
          <MetricCell label="Duration" value={formatDuration(agent.started_at, agent.finished_at)} />
          <MetricCell label="Run status" value={run.status || "Unknown"} />
        </section>

        {dimensions.length > 0 ? (
          <section className="border-t border-border p-4">
            <div className="mb-3 text-sm font-medium">Dimensions</div>
            <div className="space-y-3">
              {dimensions.map((dimension) => (
                <DimensionBar key={dimension.key} dimension={dimension} />
              ))}
            </div>
          </section>
        ) : (
          <EmptyEmbedState text="Dimension scores are not available." />
        )}

        <EmbedFooter />
      </div>
    </article>
  );
}

function EmbedHeader({
  title,
  eyebrow,
  status,
  score,
  passed,
}: {
  title: string;
  eyebrow: string;
  status: string;
  score?: number;
  passed?: boolean;
}) {
  return (
    <header className="grid gap-4 p-4 sm:grid-cols-[1fr_auto] sm:items-start">
      <div className="min-w-0">
        <div className="mb-2 flex flex-wrap items-center gap-2">
          <span className="text-xs font-medium text-muted-foreground">
            {eyebrow}
          </span>
          {status && <Badge variant="outline">{status}</Badge>}
        </div>
        <h1 className="truncate text-lg font-semibold">{title}</h1>
      </div>
      <div className="flex items-center gap-3 sm:justify-end">
        <PassBadge passed={passed} />
        <div className="text-right">
          <div className="text-2xl font-semibold tabular-nums">
            {scorePercent(score)}
          </div>
          <div className="text-xs text-muted-foreground">overall</div>
        </div>
      </div>
    </header>
  );
}

function AgentRow({
  rank,
  agent,
  scorecard,
  winner,
}: {
  rank: number;
  agent: PublicAgent;
  scorecard: ScorecardRecord;
  winner: boolean;
}) {
  const score = optionalNumber(scorecard.overall_score);
  return (
    <div className="grid grid-cols-[2.5rem_1fr_auto] items-center gap-3 px-4 py-3">
      <div className="flex items-center gap-1 text-sm font-medium">
        {winner ? <Trophy className="size-3.5 text-emerald-500" /> : null}
        <span>#{rank}</span>
      </div>
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-sm font-medium">{agent.label}</span>
          {winner ? (
            <span className="shrink-0 rounded-sm bg-emerald-500/10 px-1.5 py-0.5 text-xs text-emerald-600 dark:text-emerald-400">
              Winner
            </span>
          ) : null}
        </div>
        <div className="text-xs text-muted-foreground">
          Lane #{agent.lane_index} · {agent.status}
        </div>
      </div>
      <div className="w-24 text-right">
        <div className="font-medium tabular-nums">{scorePercent(score)}</div>
        <ScoreBar score={score} />
      </div>
    </div>
  );
}

function DimensionBar({
  dimension,
}: {
  dimension: { key: string; label: string; score?: number; icon: typeof Target };
}) {
  const Icon = dimension.icon;
  return (
    <div>
      <div className="mb-1 flex items-center justify-between gap-3 text-sm">
        <div className="flex min-w-0 items-center gap-2">
          <Icon className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="truncate">{dimension.label}</span>
        </div>
        <span className="font-medium tabular-nums">
          {scorePercent(dimension.score)}
        </span>
      </div>
      <ScoreBar score={dimension.score} />
    </div>
  );
}

function MetricCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-b border-border px-4 py-3 sm:border-b-0 sm:border-r sm:last:border-r-0">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-sm font-medium">{value}</div>
    </div>
  );
}

function PassBadge({ passed }: { passed?: boolean }) {
  if (passed == null) return null;
  if (passed) {
    return (
      <div className="flex items-center gap-1.5 text-sm text-emerald-600 dark:text-emerald-400">
        <CheckCircle2 className="size-4" />
        <span>Pass</span>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-1.5 text-sm text-red-600 dark:text-red-400">
      <XCircle className="size-4" />
      <span>Fail</span>
    </div>
  );
}

function ScoreBar({ score }: { score?: number }) {
  return (
    <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-muted">
      <div className={`h-full ${barColor(score)}`} style={{ width: barWidth(score) }} />
    </div>
  );
}

function EmptyEmbedState({ text }: { text: string }) {
  return (
    <section className="border-t border-border px-4 py-6 text-sm text-muted-foreground">
      {text}
    </section>
  );
}

function EmbedFooter() {
  return (
    <footer className="flex items-center justify-between border-t border-border px-4 py-3 text-xs text-muted-foreground">
      <span>AgentClash</span>
      <span className="flex items-center gap-1.5">
        <Clock3 className="size-3" />
        Public share
      </span>
    </footer>
  );
}

function scorecardsByAgent(scorecards: unknown[]): Record<string, ScorecardRecord> {
  return Object.fromEntries(
    scorecards
      .map(asRecord)
      .map((scorecard) => [String(scorecard.run_agent_id ?? ""), scorecard])
      .filter(([runAgentID]) => runAgentID !== ""),
  );
}

function rankAgents(
  agents: PublicAgent[],
  scorecards: Record<string, ScorecardRecord>,
  winningAgentID: string,
) {
  return agents
    .map((agent) => ({ agent, scorecard: scorecards[agent.id] ?? {} }))
    .sort((a, b) => {
      if (a.agent.id === winningAgentID) return -1;
      if (b.agent.id === winningAgentID) return 1;
      return (
        (optionalNumber(b.scorecard.overall_score) ?? -1) -
          (optionalNumber(a.scorecard.overall_score) ?? -1) ||
        a.agent.lane_index - b.agent.lane_index
      );
    });
}

function scorecardDimensions(scorecard: ScorecardRecord) {
  const innerScorecard = asRecord(scorecard.scorecard);
  const dimensions = asRecord(innerScorecard.dimensions);
  return Object.entries(dimensions)
    .map(([key, raw]) => {
      const dimension = asRecord(raw);
      const meta = dimensionMeta[key] ?? {
        label: labelFromKey(key),
        icon: BarChart3,
      };
      return {
        key,
        label: meta.label,
        icon: meta.icon,
        score: optionalNumber(dimension.score),
        state: String(dimension.state ?? ""),
      };
    })
    .filter((dimension) => dimension.state === "available")
    .sort((a, b) => {
      const order = ["correctness", "reliability", "latency", "cost", "behavioral"];
      const aIndex = order.indexOf(a.key);
      const bIndex = order.indexOf(b.key);
      if (aIndex !== -1 && bIndex !== -1) return aIndex - bIndex;
      if (aIndex !== -1) return -1;
      if (bIndex !== -1) return 1;
      return a.label.localeCompare(b.label);
    });
}

function scorePassed(scorecard?: ScorecardRecord) {
  if (!scorecard) return undefined;
  if (typeof scorecard.passed === "boolean") return scorecard.passed;
  const innerScorecard = asRecord(scorecard.scorecard);
  return typeof innerScorecard.passed === "boolean"
    ? innerScorecard.passed
    : undefined;
}

function toRun(value: unknown): PublicRun {
  const run = asRecord(value);
  return {
    id: String(run.id ?? ""),
    name: String(run.name ?? ""),
    status: String(run.status ?? ""),
    execution_mode: optionalString(run.execution_mode),
    started_at: optionalString(run.started_at),
    finished_at: optionalString(run.finished_at),
  };
}

function toAgent(value: unknown): PublicAgent {
  const agent = asRecord(value);
  return {
    id: String(agent.id ?? ""),
    label: String(agent.label ?? "Agent"),
    lane_index: optionalNumber(agent.lane_index) ?? 0,
    status: String(agent.status ?? ""),
    started_at: optionalString(agent.started_at),
    finished_at: optionalString(agent.finished_at),
  };
}

function sortAgents(a: PublicAgent, b: PublicAgent) {
  return a.lane_index - b.lane_index;
}

function formatDuration(start?: string, end?: string): string {
  if (!start || !end) return "-";
  const startTime = new Date(start).getTime();
  const endTime = new Date(end).getTime();
  const ms = Math.max(0, endTime - startTime);
  if (ms < 1000) return "<1s";
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${seconds % 60}s`;
}

function labelFromKey(key: string) {
  return key
    .split(/[_-]/)
    .filter(Boolean)
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ");
}

function optionalString(value: unknown) {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function optionalNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}
