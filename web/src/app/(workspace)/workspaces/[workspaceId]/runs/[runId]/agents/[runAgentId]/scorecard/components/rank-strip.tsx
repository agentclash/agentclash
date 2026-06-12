"use client";

import Link from "next/link";
import type { RankingItem, RunRankingResponse } from "@/lib/api/types";
import { Panel, PanelHeader } from "./panel";
import { Trophy } from "lucide-react";
import { cn } from "@/lib/utils";
import { scoreColor } from "@/lib/scores";
import { signedDelta } from "./utils";

/**
 * Cross-agent comparison strip. Renders as one lane per sibling agent in the
 * same run, with the current agent highlighted. Each lane shows rank, label,
 * score, and delta-from-top so a user landing on a scorecard immediately knows
 * *relative* standing — the single biggest thing the previous scorecard omitted.
 *
 * Falls back gracefully when the ranking isn't yet available (pending run,
 * failed run, or errored ranking) by rendering nothing — the Hero still carries
 * the single-agent view on its own.
 */
export function RankStrip({
  ranking,
  workspaceId,
  runId,
  currentRunAgentId,
}: {
  ranking: RunRankingResponse | null;
  workspaceId: string;
  runId: string;
  currentRunAgentId: string;
}) {
  const items = ranking?.ranking?.items ?? [];
  if (items.length <= 1) return null;

  const current = items.find((i) => i.run_agent_id === currentRunAgentId);
  const topScore = items[0]?.sort_value ?? items[0]?.overall_score;
  const winnerId = ranking?.ranking?.winner?.run_agent_id;

  return (
    <Panel>
      <PanelHeader
        title="Comparison"
        icon={<Trophy className="size-3.5" />}
        trailing={
          current ? (
            <span className="text-2xs text-white/45 font-[family-name:var(--font-mono)]">
              #{current.rank ?? "—"} of {items.length}
            </span>
          ) : null
        }
      />
      <div className="divide-y divide-white/[0.05]">
        {items.map((item) => (
          <RankRow
            key={item.run_agent_id}
            item={item}
            isCurrent={item.run_agent_id === currentRunAgentId}
            isWinner={item.run_agent_id === winnerId}
            topScore={topScore}
            workspaceId={workspaceId}
            runId={runId}
          />
        ))}
      </div>
    </Panel>
  );
}

function RankRow({
  item,
  isCurrent,
  isWinner,
  topScore,
  workspaceId,
  runId,
}: {
  item: RankingItem;
  isCurrent: boolean;
  isWinner: boolean;
  topScore?: number;
  workspaceId: string;
  runId: string;
}) {
  const score = item.overall_score;
  const href = `/workspaces/${workspaceId}/runs/${runId}/agents/${item.run_agent_id}/scorecard`;
  const barPct =
    score != null && topScore && topScore > 0
      ? Math.min(100, (score / Math.max(topScore, score)) * 100)
      : 0;

  const content = (
    <>
      <div className="flex items-center gap-3 px-4 h-11 relative">
        {/* Rank badge */}
        <span
          className={cn(
            "font-[family-name:var(--font-mono)] text-2xs w-6 tabular-nums",
            isWinner ? "text-amber-300" : "text-white/35",
          )}
        >
          {item.rank != null ? `${item.rank}` : "—"}
        </span>

        {/* Label */}
        <span
          className={cn(
            "flex-1 min-w-0 truncate text-sm",
            isCurrent ? "text-white/95 font-medium" : "text-white/70",
          )}
        >
          {item.label}
          {isWinner && (
            <span className="ml-2 text-2xs uppercase tracking-[0.16em] text-amber-300/80">
              Winner
            </span>
          )}
          {isCurrent && !isWinner && (
            <span className="ml-2 text-2xs uppercase tracking-[0.16em] text-white/40">
              You
            </span>
          )}
        </span>

        {/* Passed chip */}
        {item.passed != null && (
          <span
            className={cn(
              "text-2xs uppercase tracking-[0.14em]",
              item.passed ? "text-emerald-400/80" : "text-red-400/80",
            )}
          >
            {item.passed ? "pass" : "fail"}
          </span>
        )}

        {/* Delta from top */}
        <span className="font-[family-name:var(--font-mono)] text-2xs text-white/40 tabular-nums w-14 text-right">
          {item.delta_from_top != null && item.delta_from_top !== 0
            ? signedDelta(item.delta_from_top)
            : ""}
        </span>

        {/* Score */}
        <span
          className={cn(
            "font-[family-name:var(--font-mono)] text-sm tabular-nums w-14 text-right",
            score == null ? "text-white/30" : scoreColor(score),
          )}
        >
          {score == null ? "—" : `${(score * 100).toFixed(1)}`}
        </span>
      </div>

      {/* Subtle bar under the row indicating score vs top */}
      <div className="h-px w-full relative">
        <div
          className={cn(
            "absolute left-0 top-0 h-px transition-all",
            isCurrent ? "bg-white/40" : "bg-white/15",
          )}
          style={{ width: `${barPct}%` }}
        />
      </div>

      {/* Current-row highlight strip on left */}
      {isCurrent && (
        <span className="pointer-events-none absolute left-0 top-0 bottom-0 w-[2px] bg-emerald-400/70" />
      )}
    </>
  );

  return (
    <Link
      href={href}
      className={cn(
        "block relative transition-colors",
        isCurrent ? "bg-white/[0.02]" : "hover:bg-white/[0.02]",
      )}
    >
      {content}
    </Link>
  );
}

