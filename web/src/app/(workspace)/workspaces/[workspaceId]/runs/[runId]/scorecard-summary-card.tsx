"use client";

import type { ScorecardResponse } from "@/lib/api/types";
import {
  Loader2,
  AlertTriangle,
  Target,
  Shield,
  Zap,
  DollarSign,
  CheckCircle2,
  XCircle,
  BarChart3,
} from "lucide-react";
import { scorePercent, scoreColor } from "@/lib/scores";

const LEGACY_DIM_META: Record<
  string,
  { label: string; icon: typeof Target }
> = {
  correctness: { label: "COR", icon: Target },
  reliability: { label: "REL", icon: Shield },
  latency: { label: "LAT", icon: Zap },
  cost: { label: "CST", icon: DollarSign },
};

interface ScorecardSummaryCardProps {
  scorecard: ScorecardResponse | null;
  loading?: boolean;
}

export function ScorecardSummaryCard({
  scorecard,
  loading,
}: ScorecardSummaryCardProps) {
  if (loading) {
    return (
      <div className="flex items-center gap-2 text-xs text-muted-foreground mt-2">
        <Loader2 className="size-3 animate-spin" />
        <span>Scoring...</span>
      </div>
    );
  }

  if (!scorecard || scorecard.state === "errored") {
    return (
      <div className="flex items-center gap-2 text-xs text-destructive/70 mt-2">
        <AlertTriangle className="size-3" />
        <span>Score unavailable</span>
      </div>
    );
  }

  if (scorecard.state === "pending") {
    return (
      <div className="flex items-center gap-2 text-xs text-muted-foreground mt-2">
        <Loader2 className="size-3 animate-spin" />
        <span>Scoring...</span>
      </div>
    );
  }

  // Build dimension list from scorecard document (supports custom dimensions).
  const dimensions = scorecard.scorecard?.dimensions ?? {};
  const dimKeys = Object.keys(dimensions).sort((a, b) => {
    // Legacy dims first in canonical order, then custom dims alphabetically.
    const order = ["correctness", "reliability", "latency", "cost"];
    const ai = order.indexOf(a);
    const bi = order.indexOf(b);
    if (ai !== -1 && bi !== -1) return ai - bi;
    if (ai !== -1) return -1;
    if (bi !== -1) return 1;
    return a.localeCompare(b);
  });

  return (
    <div className="mt-2 space-y-1.5">
      {/* Overall score + pass/fail */}
      {scorecard.overall_score != null && (
        <div className="flex items-center gap-1.5">
          <span
            className={`text-sm font-semibold ${scoreColor(scorecard.overall_score)}`}
          >
            {scorePercent(scorecard.overall_score)}
          </span>
          <span className="text-[10px] text-muted-foreground">overall</span>
          {scorecard.scorecard?.passed != null && (
            scorecard.scorecard.passed ? (
              <CheckCircle2 className="size-3 text-emerald-400" />
            ) : (
              <XCircle className="size-3 text-red-400" />
            )
          )}
          {scorecard.scorecard?.strategy && (
            <span className="text-[10px] text-muted-foreground/60 ml-1">
              {scorecard.scorecard.strategy}
            </span>
          )}
        </div>
      )}

      {/* Dimension breakdown — legacy + custom */}
      <div className="flex gap-2.5 flex-wrap">
        {dimKeys.map((key) => {
          const dim = dimensions[key];
          if (dim.state !== "available") return null;
          const meta = LEGACY_DIM_META[key];
          const Icon = meta?.icon ?? BarChart3;
          const label = meta?.label ?? key.slice(0, 3).toUpperCase();
          return (
            <div
              key={key}
              className="flex items-center gap-1 text-[10px]"
              title={key}
            >
              <Icon className="size-2.5 text-muted-foreground" />
              <span className={scoreColor(dim.score)}>{scorePercent(dim.score)}</span>
              <span className="text-muted-foreground/50">{label}</span>
            </div>
          );
        })}
      </div>

      {/* Token split */}
      {scorecard.scorecard?.metric_summary?.run_total_tokens != null && (
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[10px] text-muted-foreground">
          <span className="font-[family-name:var(--font-mono)]">{scorecard.scorecard.metric_summary.run_total_tokens.toLocaleString()} tokens</span>
          {(scorecard.scorecard.metric_summary.run_race_context_tokens ?? 0) > 0 && (
            <span className="text-muted-foreground/60 font-[family-name:var(--font-mono)]">
              ({scorecard.scorecard.metric_summary.run_agent_tokens?.toLocaleString() ?? 0} agent + {scorecard.scorecard.metric_summary.run_race_context_tokens.toLocaleString()} context)
            </span>
          )}
        </div>
      )}
    </div>
  );
}
