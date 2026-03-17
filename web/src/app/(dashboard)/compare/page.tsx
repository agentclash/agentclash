"use client";

import { useEffect, useState, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { api, type CompareResponse, type KeyDelta } from "@/lib/api/client";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Loader2, AlertCircle, ArrowRight } from "lucide-react";

function DeltaValue({ delta }: { delta: KeyDelta }) {
  const isRegression = delta.outcome === "worse";
  const isImprovement = delta.outcome === "better";
  const sign = delta.delta > 0 ? "+" : "";
  const color = isRegression
    ? "text-ds-accent font-semibold"
    : isImprovement
      ? "text-status-pass"
      : "text-text-3";

  return (
    <span className={`tabular-nums ${color}`}>
      {sign}{delta.delta.toFixed(4)}
    </span>
  );
}

function VerdictBadge({ status, reasons }: { status: string; reasons: string[] }) {
  const isPass = status === "comparable" && reasons.length === 0;
  const isWarn = status === "comparable" && reasons.length > 0;

  if (isPass) {
    return (
      <span className="inline-flex items-center text-[11px] font-semibold uppercase tracking-[0.06em] px-2.5 py-1 rounded text-status-pass bg-status-pass/10">
        PASS
      </span>
    );
  }
  if (isWarn) {
    return (
      <span className="inline-flex items-center text-[11px] font-semibold uppercase tracking-[0.06em] px-2.5 py-1 rounded text-ds-accent bg-ds-accent/10">
        WARN — {reasons[0]}
      </span>
    );
  }
  return (
    <span className="inline-flex items-center text-[11px] font-semibold uppercase tracking-[0.06em] px-2.5 py-1 rounded text-status-fail bg-status-fail/10">
      NOT COMPARABLE
    </span>
  );
}

export default function ComparePage() {
  const searchParams = useSearchParams();
  const [baselineId, setBaselineId] = useState(searchParams.get("baseline") || "");
  const [candidateId, setCandidateId] = useState(searchParams.get("candidate") || "");
  const [comparison, setComparison] = useState<CompareResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const runComparison = useCallback(async () => {
    if (!baselineId || !candidateId) return;
    setLoading(true);
    setError("");
    setComparison(null);
    try {
      const data = await api.getComparison(baselineId, candidateId);
      setComparison(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Comparison failed");
    } finally {
      setLoading(false);
    }
  }, [baselineId, candidateId]);

  // Auto-load if params present
  useEffect(() => {
    if (baselineId && candidateId) {
      runComparison();
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="max-w-4xl">
      <PageHeader
        eyebrow="Analysis"
        title="Compare Runs"
        description="Compare baseline vs candidate across all scoring dimensions"
      />

      {/* Input form */}
      <div className="rounded-xl border border-border bg-card p-5 mb-8">
        <div className="grid grid-cols-1 md:grid-cols-[1fr_auto_1fr_auto] gap-4 items-end">
          <div className="space-y-2">
            <Label className="text-xs text-text-3">Baseline Run ID</Label>
            <Input
              value={baselineId}
              onChange={(e) => setBaselineId(e.target.value)}
              placeholder="baseline run UUID"
              className="font-[family-name:var(--font-mono)] text-xs"
            />
          </div>
          <div className="flex items-center justify-center">
            <ArrowRight className="size-4 text-text-4" />
          </div>
          <div className="space-y-2">
            <Label className="text-xs text-text-3">Candidate Run ID</Label>
            <Input
              value={candidateId}
              onChange={(e) => setCandidateId(e.target.value)}
              placeholder="candidate run UUID"
              className="font-[family-name:var(--font-mono)] text-xs"
            />
          </div>
          <Button
            onClick={runComparison}
            disabled={!baselineId || !candidateId || loading}
            size="sm"
          >
            {loading ? <Loader2 className="size-3.5 animate-spin" /> : "Compare"}
          </Button>
        </div>
      </div>

      {error && (
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-4 mb-6 flex items-center gap-3">
          <AlertCircle className="size-4 text-status-fail" />
          <p className="text-sm text-status-fail">{error}</p>
        </div>
      )}

      {loading && !comparison && (
        <div className="space-y-2">
          <Skeleton className="h-10 w-full" />
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {/* Comparison table */}
      {comparison && (
        <div className="rounded-xl border border-border overflow-hidden font-[family-name:var(--font-mono)] text-[13px]">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-2.5 bg-surface border-b border-border">
            <div className="flex items-center gap-2">
              <div className="flex gap-[5px]">
                <span className="w-2 h-2 rounded-full bg-text-4" />
                <span className="w-2 h-2 rounded-full bg-text-4" />
                <span className="w-2 h-2 rounded-full bg-text-4" />
              </div>
              <span className="text-[11px] text-text-3 ml-2">run comparison</span>
            </div>
            <span className="text-[11px] text-text-4">
              {comparison.baseline_run_id.slice(0, 8)} vs {comparison.candidate_run_id.slice(0, 8)}
            </span>
          </div>

          {/* Column headers */}
          <div className="grid grid-cols-[140px_100px_100px_100px] px-4 py-1.5 border-b border-border">
            <span className="text-[10px] font-medium uppercase tracking-[0.06em] text-text-4" />
            <span className="text-[10px] font-medium uppercase tracking-[0.06em] text-text-4 text-right">
              base
            </span>
            <span className="text-[10px] font-medium uppercase tracking-[0.06em] text-text-4 text-right">
              cand
            </span>
            <span className="text-[10px] font-medium uppercase tracking-[0.06em] text-text-4 text-right">
              delta
            </span>
          </div>

          {/* Delta rows */}
          {comparison.key_deltas.map((delta) => (
            <div
              key={delta.metric}
              className="grid grid-cols-[140px_100px_100px_100px] px-4 py-2 border-b border-border last:border-b-0"
            >
              <span className="text-text-2">{delta.metric}</span>
              <span className="text-right text-text-3 tabular-nums">
                {delta.baseline_value.toFixed(4)}
              </span>
              <span className="text-right text-text-3 tabular-nums">
                {delta.candidate_value.toFixed(4)}
              </span>
              <span className="text-right">
                <DeltaValue delta={delta} />
              </span>
            </div>
          ))}

          {comparison.key_deltas.length === 0 && (
            <div className="px-4 py-6 text-center text-text-3 text-xs">
              No dimension data available
            </div>
          )}

          {/* Verdict footer */}
          <div className="flex items-center justify-between px-4 py-3 bg-surface border-t border-border">
            <span className="text-[11px] text-text-3">verdict</span>
            <VerdictBadge
              status={comparison.status}
              reasons={comparison.regression_reasons}
            />
          </div>
        </div>
      )}

      {/* Evidence quality warnings */}
      {comparison?.evidence_quality?.warnings?.length ? (
        <div className="mt-4 rounded-lg border border-border p-4">
          <p className="text-[10px] uppercase tracking-[0.14em] text-text-4 font-semibold mb-2">
            Evidence Quality
          </p>
          {comparison.evidence_quality.warnings.map((w, i) => (
            <p key={i} className="text-xs text-text-3">
              {w}
            </p>
          ))}
        </div>
      ) : null}
    </div>
  );
}
