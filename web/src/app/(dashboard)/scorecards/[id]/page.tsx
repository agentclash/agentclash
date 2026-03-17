"use client";

import { useEffect, useState, use } from "react";
import Link from "next/link";
import { api, type ScorecardResponse } from "@/lib/api/client";
import { PageHeader } from "@/components/layout/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { ArrowLeft, Loader2, AlertCircle } from "lucide-react";

type ScoreDimension = {
  label: string;
  value: number | null | undefined;
  key: string;
};

function ScoreBar({ value, max = 1.0 }: { value: number; max?: number }) {
  const pct = Math.min(100, Math.max(0, (value / max) * 100));
  const color =
    pct >= 80 ? "bg-status-pass" : pct >= 50 ? "bg-status-warn" : "bg-status-fail";

  return (
    <div className="flex items-center gap-3 w-full max-w-xs">
      <div className="flex-1 h-1.5 rounded-full bg-surface overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${color}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function formatScore(value: number | null | undefined): string {
  if (value == null) return "—";
  return value.toFixed(4);
}

export default function ScorecardPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: runAgentId } = use(params);
  const [scorecard, setScorecard] = useState<ScorecardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      setLoading(true);
      try {
        const data = await api.getScorecard(runAgentId);
        setScorecard(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load scorecard");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [runAgentId]);

  if (loading) {
    return (
      <div className="max-w-3xl space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-64" />
        <Skeleton className="h-64 mt-6" />
      </div>
    );
  }

  if (error && !scorecard) {
    return (
      <div className="max-w-3xl">
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-6">
          <p className="text-sm text-status-fail">{error}</p>
        </div>
      </div>
    );
  }

  const dimensions: ScoreDimension[] = [
    { label: "correctness", value: scorecard?.correctness_score, key: "correctness" },
    { label: "reliability", value: scorecard?.reliability_score, key: "reliability" },
    { label: "latency", value: scorecard?.latency_score, key: "latency" },
    { label: "cost", value: scorecard?.cost_score, key: "cost" },
  ];

  return (
    <div className="max-w-3xl">
      <div className="mb-4">
        {scorecard?.run_id && (
          <Link
            href={`/runs/${scorecard.run_id}`}
            className="inline-flex items-center gap-1.5 text-xs text-text-3 hover:text-text-1 transition-colors"
          >
            <ArrowLeft className="size-3" />
            Back to run
          </Link>
        )}
      </div>

      <PageHeader
        eyebrow="Scorecard"
        title={`Agent ${runAgentId.slice(0, 8)}`}
      />

      {/* State banner */}
      {scorecard?.state === "pending" && (
        <div className="rounded-lg border border-border bg-surface/50 p-4 mb-6 flex items-center gap-3">
          <Loader2 className="size-4 text-text-3 animate-spin" />
          <p className="text-sm text-text-3">
            {scorecard.message || "Scorecard is being generated..."}
          </p>
        </div>
      )}

      {scorecard?.state === "errored" && (
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-4 mb-6 flex items-center gap-3">
          <AlertCircle className="size-4 text-status-fail" />
          <p className="text-sm text-status-fail">
            {scorecard.message || "Scorecard generation failed"}
          </p>
        </div>
      )}

      {/* Score table */}
      {scorecard?.state === "ready" && (
        <div className="rounded-xl border border-border overflow-hidden font-[family-name:var(--font-mono)] text-[13px]">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-2.5 bg-surface border-b border-border">
            <div className="flex items-center gap-2">
              <div className="flex gap-[5px]">
                <span className="w-2 h-2 rounded-full bg-text-4" />
                <span className="w-2 h-2 rounded-full bg-text-4" />
                <span className="w-2 h-2 rounded-full bg-text-4" />
              </div>
              <span className="text-[11px] text-text-3 ml-2">scorecard</span>
            </div>
            <span className="text-[11px] text-text-4">
              {scorecard.run_agent_status}
            </span>
          </div>

          {/* Column headers */}
          <div className="grid grid-cols-[1fr_100px_1fr] px-4 py-1.5 border-b border-border">
            <span className="text-[10px] font-medium uppercase tracking-[0.06em] text-text-4">
              dimension
            </span>
            <span className="text-[10px] font-medium uppercase tracking-[0.06em] text-text-4 text-right">
              score
            </span>
            <span className="text-[10px] font-medium uppercase tracking-[0.06em] text-text-4 text-right pr-2">
              bar
            </span>
          </div>

          {/* Dimension rows */}
          {dimensions.map((dim) => (
            <div
              key={dim.key}
              className="grid grid-cols-[1fr_100px_1fr] items-center px-4 py-2.5 border-b border-border last:border-b-0"
            >
              <span className="text-text-2">{dim.label}</span>
              <span className="text-right tabular-nums font-medium text-text-1">
                {formatScore(dim.value)}
              </span>
              <div className="flex justify-end pr-2">
                {dim.value != null ? (
                  <ScoreBar value={dim.value} />
                ) : (
                  <span className="text-text-4 text-[11px]">unavailable</span>
                )}
              </div>
            </div>
          ))}

          {/* Overall footer */}
          <div className="flex items-center justify-between px-4 py-3 bg-surface border-t border-border">
            <span className="text-[11px] text-text-3">overall</span>
            <span className="text-lg font-semibold text-text-1 tabular-nums">
              {formatScore(scorecard.overall_score)}
            </span>
          </div>
        </div>
      )}
    </div>
  );
}
