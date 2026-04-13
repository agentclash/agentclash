"use client";

import { useState, useMemo } from "react";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { KpiStrip } from "./kpi-strip";
import { ExperimentResults } from "./experiment-results";
import {
  AlertCircle,
  ChevronDown,
  ChevronRight,
  FlaskConical,
  Loader2,
} from "lucide-react";
import type {
  PlaygroundExperiment,
  PlaygroundExperimentResult,
} from "@/lib/api/types";

function statusVariant(
  status: string,
): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "completed":
      return "default";
    case "running":
      return "secondary";
    case "failed":
      return "destructive";
    default:
      return "outline";
  }
}

function parseSummary(summary: Record<string, unknown>) {
  return {
    totalCases: (summary.total_cases as number) ?? 0,
    completedCases: (summary.completed_cases as number) ?? 0,
    failedCases: (summary.failed_cases as number) ?? 0,
    error: (summary.error as string) ?? null,
  };
}

interface ExperimentListProps {
  experiments: PlaygroundExperiment[];
  resultsByExperimentId: Record<string, PlaygroundExperimentResult[]>;
  isPolling: boolean;
  onFetchResults?: (experimentId: string) => Promise<void>;
}

export function ExperimentList({
  experiments,
  resultsByExperimentId,
  isPolling,
  onFetchResults,
}: ExperimentListProps) {
  const [expandedId, setExpandedId] = useState<string | null>(null);

  function handleToggle(expId: string) {
    const willExpand = expandedId !== expId;
    setExpandedId(willExpand ? expId : null);
    // Fetch results on-demand when expanding if not already loaded
    if (willExpand && !resultsByExperimentId[expId] && onFetchResults) {
      onFetchResults(expId);
    }
  }

  if (experiments.length === 0) {
    return (
      <EmptyState
        icon={<FlaskConical className="size-10" />}
        title="No experiments yet"
        description="Launch an experiment to run your prompt against a model and see scored results."
      />
    );
  }

  return (
    <div className="space-y-3">
      {isPolling && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 className="size-3 animate-spin" />
          Watching for updates...
        </div>
      )}

      {experiments.map((exp) => {
        const results = resultsByExperimentId[exp.id] ?? [];
        const isActive = exp.status === "queued" || exp.status === "running";
        const isExpanded = expandedId === exp.id;
        const summary = parseSummary(exp.summary);

        return (
          <div
            key={exp.id}
            className="rounded-lg border border-border overflow-hidden"
          >
            <button
              type="button"
              onClick={() => handleToggle(exp.id)}
              className="flex w-full items-center gap-3 p-4 text-left hover:bg-muted/30 transition-colors"
            >
              {isExpanded ? (
                <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
              ) : (
                <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
              )}

              <div className="flex-1 min-w-0 space-y-1.5">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium truncate">
                    {exp.name}
                  </span>
                  <Badge variant={statusVariant(exp.status)}>
                    {isActive && (
                      <Loader2 className="mr-1 size-3 animate-spin" />
                    )}
                    {exp.status}
                  </Badge>
                  {isActive && summary.totalCases > 0 && (
                    <span className="text-xs text-muted-foreground">
                      {summary.completedCases + summary.failedCases}/
                      {summary.totalCases} cases
                    </span>
                  )}
                </div>

                {exp.status === "failed" && summary.error && (
                  <div className="flex items-start gap-1.5 text-xs text-destructive">
                    <AlertCircle className="mt-0.5 size-3 shrink-0" />
                    <span className="line-clamp-2">{summary.error}</span>
                  </div>
                )}

                {exp.status === "completed" || results.length > 0 ? (
                  <ExperimentSummaryStrip results={results} />
                ) : isActive ? (
                  <KpiStrip loading />
                ) : null}
              </div>

              <span className="shrink-0 text-xs text-muted-foreground">
                {exp.queued_at
                  ? new Date(exp.queued_at).toLocaleString()
                  : "just now"}
              </span>
            </button>

            {isExpanded && (
              <div className="border-t border-border bg-muted/10 p-4">
                {exp.status === "failed" && summary.error && (
                  <div className="mb-4 flex items-start gap-2 rounded-md border border-destructive/20 bg-destructive/5 p-3">
                    <AlertCircle className="mt-0.5 size-4 shrink-0 text-destructive" />
                    <div className="text-sm">
                      <p className="font-medium text-destructive">Experiment failed</p>
                      <p className="mt-1 text-destructive/80">{summary.error}</p>
                    </div>
                  </div>
                )}
                {results.length > 0 ? (
                  <ExperimentResults results={results} />
                ) : isActive ? (
                  <div className="space-y-3">
                    <Skeleton className="h-24 w-full" />
                    <Skeleton className="h-24 w-full" />
                  </div>
                ) : !summary.error ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">
                    No results available.
                  </p>
                ) : null}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function ExperimentSummaryStrip({
  results,
}: {
  results: PlaygroundExperimentResult[];
}) {
  const stats = useMemo(() => {
    if (results.length === 0) return null;

    const avgLatency =
      results.reduce((s, r) => s + r.latency_ms, 0) / results.length;
    const totalTokens = results.reduce((s, r) => s + r.total_tokens, 0);
    const totalCost = results.reduce((s, r) => s + (r.cost_usd ?? 0), 0);

    const dimAccum: Record<string, { total: number; count: number }> = {};
    for (const r of results) {
      if (!r.dimension_scores) continue;
      for (const [dim, score] of Object.entries(r.dimension_scores)) {
        if (score == null) continue;
        const acc = dimAccum[dim] ?? { total: 0, count: 0 };
        acc.total += score;
        acc.count++;
        dimAccum[dim] = acc;
      }
    }
    const avgDimensions: Record<string, number | null> = {};
    for (const [dim, acc] of Object.entries(dimAccum)) {
      avgDimensions[dim] = acc.count > 0 ? acc.total / acc.count : null;
    }

    return { avgLatency, totalTokens, totalCost, avgDimensions };
  }, [results]);

  if (!stats) return null;

  return (
    <KpiStrip
      latencyMs={Math.round(stats.avgLatency)}
      totalTokens={stats.totalTokens}
      costUsd={stats.totalCost}
      dimensions={stats.avgDimensions}
    />
  );
}
