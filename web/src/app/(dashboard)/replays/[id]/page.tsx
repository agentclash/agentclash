"use client";

import { useEffect, useState, use, useCallback } from "react";
import Link from "next/link";
import { api, type ReplayResponse, type ReplayStep } from "@/lib/api/client";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ArrowLeft, ChevronDown, ChevronRight, Loader2, AlertCircle } from "lucide-react";

function StepTypeBadge({ type }: { type?: string }) {
  const styles: Record<string, string> = {
    system: "text-blue-400 bg-blue-400/10",
    provider: "text-purple-400 bg-purple-400/10",
    tool: "text-emerald-400 bg-emerald-400/10",
    scoring: "text-amber-400 bg-amber-400/10",
  };
  const style = styles[type || ""] || "text-text-3 bg-surface";

  return (
    <span className={`inline-flex items-center text-[10px] font-semibold uppercase tracking-[0.06em] px-2 py-0.5 rounded ${style}`}>
      {type || "unknown"}
    </span>
  );
}

function StepStatusBadge({ status }: { status?: string }) {
  const styles: Record<string, string> = {
    completed: "text-status-pass",
    running: "text-text-3",
    failed: "text-status-fail",
    interrupted: "text-status-warn",
  };

  return (
    <span className={`text-[10px] font-[family-name:var(--font-mono)] ${styles[status || ""] || "text-text-3"}`}>
      {status || "—"}
    </span>
  );
}

function ReplayStepItem({ step, index }: { step: ReplayStep; index: number }) {
  const [expanded, setExpanded] = useState(false);

  const headline = step.headline ||
    (step.tool_name ? `Tool call: ${step.tool_name}` : null) ||
    (step.model_id ? `Model call to ${step.model_id}` : null) ||
    `Step ${step.sequence_number ?? index + 1}`;

  return (
    <div className="border-b border-border last:border-b-0">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-surface/50 transition-colors cursor-pointer"
      >
        <span className="font-[family-name:var(--font-mono)] text-[11px] text-text-4 w-6 text-right shrink-0">
          {step.sequence_number ?? index + 1}
        </span>
        <div className="flex-1 min-w-0">
          <p className="text-sm text-text-1 truncate">{headline}</p>
          <div className="flex items-center gap-2 mt-0.5">
            <StepTypeBadge type={step.type} />
            <StepStatusBadge status={step.status} />
            {step.duration_ms != null && (
              <span className="text-[10px] font-[family-name:var(--font-mono)] text-text-3">
                {step.duration_ms}ms
              </span>
            )}
          </div>
        </div>
        {expanded ? (
          <ChevronDown className="size-3.5 text-text-3 shrink-0" />
        ) : (
          <ChevronRight className="size-3.5 text-text-3 shrink-0" />
        )}
      </button>

      {expanded && (
        <div className="px-4 pb-3 pl-[52px]">
          <div className="rounded-lg border border-border bg-surface/30 p-3 space-y-1.5">
            {step.provider_key && (
              <DetailRow label="Provider" value={step.provider_key} />
            )}
            {step.model_id && (
              <DetailRow label="Model" value={step.model_id} />
            )}
            {step.tool_name && (
              <DetailRow label="Tool" value={step.tool_name} />
            )}
            {step.timestamp && (
              <DetailRow label="Timestamp" value={new Date(step.timestamp).toLocaleString()} />
            )}
            {step.error_message && (
              <div className="mt-2 p-2 rounded bg-status-fail/5 border border-status-fail/10">
                <p className="text-[11px] text-status-fail font-[family-name:var(--font-mono)]">
                  {step.error_message}
                </p>
              </div>
            )}
            {/* Show any extra fields */}
            {Object.entries(step).filter(([k]) =>
              !["sequence_number", "headline", "type", "status", "timestamp", "duration_ms", "provider_key", "model_id", "tool_name", "error_message"].includes(k)
            ).map(([key, value]) => (
              <DetailRow key={key} label={key} value={typeof value === "object" ? JSON.stringify(value) : String(value)} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start gap-3">
      <span className="text-[10px] text-text-3 uppercase tracking-wider w-20 shrink-0 pt-0.5">
        {label}
      </span>
      <span className="text-[11px] text-text-2 font-[family-name:var(--font-mono)] break-all">
        {value}
      </span>
    </div>
  );
}

export default function ReplayPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: runAgentId } = use(params);
  const [replay, setReplay] = useState<ReplayResponse | null>(null);
  const [allSteps, setAllSteps] = useState<ReplayStep[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      setLoading(true);
      try {
        const data = await api.getReplay(runAgentId, 0, 50);
        setReplay(data);
        setAllSteps(data.steps);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load replay");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [runAgentId]);

  const loadMore = useCallback(async () => {
    if (!replay?.pagination.has_more || loadingMore) return;
    setLoadingMore(true);
    try {
      const nextCursor = replay.pagination.next_cursor
        ? parseInt(replay.pagination.next_cursor)
        : allSteps.length;
      const data = await api.getReplay(runAgentId, nextCursor, 50);
      setReplay(data);
      setAllSteps((prev) => [...prev, ...data.steps]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load more");
    } finally {
      setLoadingMore(false);
    }
  }, [replay, loadingMore, allSteps.length, runAgentId]);

  if (loading) {
    return (
      <div className="max-w-4xl space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-64" />
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-16" />
        ))}
      </div>
    );
  }

  if (error && !replay) {
    return (
      <div className="max-w-4xl">
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-6">
          <p className="text-sm text-status-fail">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <div className="mb-4">
        {replay?.run_id && (
          <Link
            href={`/runs/${replay.run_id}`}
            className="inline-flex items-center gap-1.5 text-xs text-text-3 hover:text-text-1 transition-colors"
          >
            <ArrowLeft className="size-3" />
            Back to run
          </Link>
        )}
      </div>

      <PageHeader
        eyebrow="Replay"
        title={`Agent ${runAgentId.slice(0, 8)}`}
        description={
          replay?.replay
            ? `${replay.replay.event_count} events recorded`
            : undefined
        }
      />

      {/* State banner */}
      {replay?.state === "pending" && (
        <div className="rounded-lg border border-border bg-surface/50 p-4 mb-6 flex items-center gap-3">
          <Loader2 className="size-4 text-text-3 animate-spin" />
          <p className="text-sm text-text-3">
            {replay.message || "Replay is being generated..."}
          </p>
        </div>
      )}

      {replay?.state === "errored" && (
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-4 mb-6 flex items-center gap-3">
          <AlertCircle className="size-4 text-status-fail" />
          <p className="text-sm text-status-fail">
            {replay.message || "Replay generation failed"}
          </p>
        </div>
      )}

      {/* Steps timeline */}
      {allSteps.length > 0 ? (
        <div className="rounded-xl border border-border overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-2.5 bg-surface border-b border-border">
            <div className="flex items-center gap-2">
              <div className="flex gap-[5px]">
                <span className="w-2 h-2 rounded-full bg-text-4" />
                <span className="w-2 h-2 rounded-full bg-text-4" />
                <span className="w-2 h-2 rounded-full bg-text-4" />
              </div>
              <span className="text-[11px] text-text-3 ml-2">replay timeline</span>
            </div>
            <span className="text-[11px] text-text-4 font-[family-name:var(--font-mono)]">
              {allSteps.length} / {replay?.pagination.total_steps ?? "?"} steps
            </span>
          </div>

          {allSteps.map((step, i) => (
            <ReplayStepItem key={step.sequence_number ?? i} step={step} index={i} />
          ))}

          {/* Load more */}
          {replay?.pagination.has_more && (
            <div className="px-4 py-3 bg-surface border-t border-border">
              <Button
                variant="ghost"
                size="sm"
                onClick={loadMore}
                disabled={loadingMore}
                className="w-full"
              >
                {loadingMore ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : (
                  "Load more steps"
                )}
              </Button>
            </div>
          )}
        </div>
      ) : replay?.state === "ready" ? (
        <div className="text-center py-12 text-text-3 text-sm">
          No steps recorded
        </div>
      ) : null}
    </div>
  );
}
