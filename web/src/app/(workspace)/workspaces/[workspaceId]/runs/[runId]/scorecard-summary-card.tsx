"use client";

import type { ScorecardResponse } from "@/lib/api/types";
import { Loader2, AlertTriangle, Target, Shield, Zap, DollarSign } from "lucide-react";
import { scorePercent, scoreColor } from "@/lib/scores";

const DIMS = [
  { key: "correctness_score" as const, label: "COR", icon: Target },
  { key: "reliability_score" as const, label: "REL", icon: Shield },
  { key: "latency_score" as const, label: "LAT", icon: Zap },
  { key: "cost_score" as const, label: "CST", icon: DollarSign },
];

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

  return (
    <div className="mt-2 space-y-1.5">
      {/* Overall score */}
      {scorecard.overall_score != null && (
        <div className="flex items-baseline gap-1.5">
          <span
            className={`text-sm font-semibold ${scoreColor(scorecard.overall_score)}`}
          >
            {scorePercent(scorecard.overall_score)}
          </span>
          <span className="text-[10px] text-muted-foreground">overall</span>
        </div>
      )}

      {/* Mini dimension breakdown */}
      <div className="flex gap-2.5 flex-wrap">
        {DIMS.map((dim) => {
          const score = scorecard[dim.key];
          const Icon = dim.icon;
          return (
            <div
              key={dim.key}
              className="flex items-center gap-1 text-[10px]"
              title={dim.label}
            >
              <Icon className="size-2.5 text-muted-foreground" />
              <span className={scoreColor(score)}>{scorePercent(score)}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
