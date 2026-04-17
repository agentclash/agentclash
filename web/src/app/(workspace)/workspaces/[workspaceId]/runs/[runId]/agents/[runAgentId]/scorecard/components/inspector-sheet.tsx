"use client";

import {
  Sheet,
  SheetContent,
  SheetTitle,
} from "@/components/ui/sheet";
import type {
  LLMJudgeResult,
  MetricDetail,
  ScorecardSource,
  ValidatorDetail,
} from "@/lib/api/types";
import type { InspectorTarget } from "./evidence-panels";
import { scoreColor } from "@/lib/scores";
import { cn } from "@/lib/utils";
import { humanizeKey, parseJudgePayload, type JudgeCall } from "./utils";
import { StateDot, normalizeState } from "./state-dot";
import { AlertTriangle, ArrowUpRight, XCircle } from "lucide-react";
import { JudgeSampleCard } from "./judge-sample-card";
import { ValidatorEvidenceView } from "./validator-evidence";
import Link from "next/link";

/**
 * Single right-drawer that renders the deep detail of whatever the user clicked
 * on — validator, metric, or LLM judge. Having one Sheet (rather than one per
 * type) makes the main page feel like a stable cockpit while the inspector
 * slides in and out.
 *
 * Three renderers share a common frame: header with key + verdict/score,
 * scrollable body, tight footer for raw-payload toggle where relevant.
 */
export function InspectorSheet({
  target,
  onClose,
  replayBasePath,
}: {
  target: InspectorTarget | null;
  onClose: () => void;
  replayBasePath?: string;
}) {
  const open = target != null;

  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <SheetContent
        side="right"
        className="w-full sm:max-w-xl bg-[#0a0a0a] border-l border-white/[0.08] p-0 flex flex-col gap-0"
      >
        <SheetTitle className="sr-only">
          {target ? `Inspect ${target.kind}` : "Inspector"}
        </SheetTitle>
        {target?.kind === "validator" && (
          <ValidatorInspector
            detail={target.detail}
            replayBasePath={replayBasePath}
          />
        )}
        {target?.kind === "metric" && <MetricInspector detail={target.detail} />}
        {target?.kind === "judge" && <JudgeInspector detail={target.detail} />}
      </SheetContent>
    </Sheet>
  );
}

/* ---------------------------------------------------------------- Validator */

function ValidatorInspector({
  detail,
  replayBasePath,
}: {
  detail: ValidatorDetail;
  replayBasePath?: string;
}) {
  const stateKind = normalizeState(detail.verdict, detail.state);
  const hasScore = detail.normalized_score != null;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <InspectorHeader
        kicker="Validator"
        title={detail.key}
        subtitle={detail.type.replace(/_/g, " ")}
        stateKind={stateKind}
        score={detail.normalized_score}
      />

      <div className="flex-1 overflow-y-auto px-6 py-5 space-y-5">
        <DataRow
          label="Verdict"
          value={
            <span
              className={cn(
                "text-sm",
                detail.verdict === "pass"
                  ? "text-emerald-300"
                  : detail.verdict === "fail"
                    ? "text-red-300"
                    : "text-white/55",
              )}
            >
              {detail.verdict || "—"}
            </span>
          }
        />
        <DataRow
          label="State"
          value={<span className="text-sm text-white/75">{detail.state}</span>}
        />
        <DataRow
          label="Type"
          value={
            <span className="font-[family-name:var(--font-mono)] text-[11px] text-white/60">
              {detail.type}
            </span>
          }
        />
        {hasScore && (
          <DataRow
            label="Normalized"
            value={
              <div className="flex items-center gap-3 w-full">
                <div className="h-[3px] flex-1 bg-white/[0.06] rounded-full overflow-hidden">
                  <div
                    className={cn(
                      "h-full rounded-full",
                      detail.normalized_score! >= 0.8
                        ? "bg-emerald-500"
                        : detail.normalized_score! >= 0.5
                          ? "bg-amber-500"
                          : "bg-red-500",
                    )}
                    style={{
                      width: `${(detail.normalized_score! * 100).toFixed(1)}%`,
                    }}
                  />
                </div>
                <span
                  className={cn(
                    "font-[family-name:var(--font-mono)] text-sm tabular-nums",
                    scoreColor(detail.normalized_score),
                  )}
                >
                  {(detail.normalized_score! * 100).toFixed(1)}
                </span>
              </div>
            }
          />
        )}

        {detail.reason && (
          <Section title="Reason">
            <p className="text-sm text-white/75 leading-relaxed whitespace-pre-wrap">
              {detail.reason}
            </p>
          </Section>
        )}
        {detail.evidence && <ValidatorEvidenceView evidence={detail.evidence} />}
        <ReplaySourceLink
          source={detail.source}
          replayBasePath={replayBasePath}
        />
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ Metric */

function MetricInspector({ detail }: { detail: MetricDetail }) {
  const stateKind: "available" | "unavailable" | "error" =
    detail.state === "error"
      ? "error"
      : detail.state === "available"
        ? "available"
        : "unavailable";

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <InspectorHeader
        kicker="Metric"
        title={detail.key}
        subtitle={detail.collector.replace(/_/g, " ")}
        stateKind={stateKind}
      />

      <div className="flex-1 overflow-y-auto px-6 py-5 space-y-5">
        <DataRow
          label="Collector"
          value={
            <span className="font-[family-name:var(--font-mono)] text-[11px] text-white/60">
              {detail.collector}
            </span>
          }
        />
        <DataRow
          label="State"
          value={<span className="text-sm text-white/75">{detail.state}</span>}
        />
        <DataRow
          label="Value"
          value={<ValueDisplay detail={detail} />}
        />

        {detail.reason && (
          <Section title="Notes">
            <p className="text-sm text-white/75 leading-relaxed whitespace-pre-wrap">
              {detail.reason}
            </p>
          </Section>
        )}
      </div>
    </div>
  );
}

function ValueDisplay({ detail }: { detail: MetricDetail }) {
  if (detail.numeric_value != null) {
    return (
      <span className="font-[family-name:var(--font-mono)] text-xl tabular-nums text-white/95">
        {detail.numeric_value.toLocaleString(undefined, {
          maximumFractionDigits: 6,
        })}
      </span>
    );
  }
  if (detail.boolean_value != null) {
    return (
      <span
        className={cn(
          "text-sm font-medium",
          detail.boolean_value ? "text-emerald-300" : "text-red-300",
        )}
      >
        {detail.boolean_value ? "true" : "false"}
      </span>
    );
  }
  if (detail.text_value != null) {
    return (
      <pre className="text-[12px] text-white/80 whitespace-pre-wrap font-[family-name:var(--font-mono)] max-h-64 overflow-y-auto">
        {detail.text_value}
      </pre>
    );
  }
  return <span className="text-white/35 text-sm">—</span>;
}

/* ------------------------------------------------------------------- Judge */

function JudgeInspector({ detail }: { detail: LLMJudgeResult }) {
  const parsed = parseJudgePayload(detail);
  const stateKind = parsed.available ? "available" : "unavailable";

  // Group calls by model so the detail view reads as "per-model, each of their
  // samples" rather than one flat list.
  const callsByModel = new Map<string, JudgeCall[]>();
  for (const c of parsed.calls) {
    if (!callsByModel.has(c.model)) callsByModel.set(c.model, []);
    callsByModel.get(c.model)!.push(c);
  }

  // Precompute min / mean / max of successful scores for the variance strip.
  const allScores = parsed.calls
    .map((c) => c.score)
    .filter((s): s is number => typeof s === "number");
  const min = allScores.length ? Math.min(...allScores) : undefined;
  const max = allScores.length ? Math.max(...allScores) : undefined;
  const mean = allScores.length
    ? allScores.reduce((a, b) => a + b, 0) / allScores.length
    : undefined;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <InspectorHeader
        kicker="LLM Judge"
        title={detail.judge_key}
        subtitle={detail.mode.replace(/_/g, " ")}
        stateKind={stateKind}
        score={detail.normalized_score}
      />

      <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
        {/* Top stats */}
        <div className="grid grid-cols-3 gap-px bg-white/[0.06] border border-white/[0.06] rounded-md overflow-hidden">
          <Stat
            label="Samples"
            value={`${detail.sample_count}`}
          />
          <Stat
            label="Models"
            value={`${detail.model_count}`}
          />
          <Stat
            label="Confidence"
            value={detail.confidence ?? "—"}
          />
        </div>

        {/* Variance spread */}
        {allScores.length > 1 && min != null && max != null && mean != null && (
          <Section title="Score spread">
            <VarianceSpread
              min={min}
              max={max}
              mean={mean}
              variance={detail.variance ?? undefined}
            />
          </Section>
        )}

        {/* Reason / unable-to-judge */}
        {parsed.reason && (
          <div className="flex items-start gap-2 border border-red-500/25 bg-red-500/[0.04] rounded-md px-3 py-2.5">
            <XCircle className="size-3.5 text-red-400 shrink-0 mt-0.5" />
            <div className="text-[12px] text-red-200/85 leading-snug">
              {parsed.reason}
              {parsed.unableToJudgeCount != null && parsed.unableToJudgeCount > 0 && (
                <span className="ml-2 text-red-300/70 font-[family-name:var(--font-mono)]">
                  {parsed.unableToJudgeCount} unable to judge
                </span>
              )}
            </div>
          </div>
        )}

        {parsed.warnings.length > 0 && (
          <div className="flex items-start gap-2 border border-amber-500/25 bg-amber-500/[0.04] rounded-md px-3 py-2.5">
            <AlertTriangle className="size-3.5 text-amber-400 shrink-0 mt-0.5" />
            <ul className="flex-1 space-y-1">
              {parsed.warnings.map((w, i) => (
                <li key={i} className="text-[11px] text-amber-200/80 leading-snug">
                  {w}
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Model scores */}
        {parsed.modelScores.length > 0 && (
          <Section title="Per-model scores">
            <div className="divide-y divide-white/[0.05] border border-white/[0.06] rounded-md">
              {parsed.modelScores.map((ms) => (
                <div
                  key={ms.model}
                  className="flex items-center gap-3 px-3 h-9"
                >
                  <span className="text-[12px] text-white/75 truncate flex-1 font-[family-name:var(--font-mono)]">
                    {ms.model}
                  </span>
                  <div className="h-[3px] w-28 bg-white/[0.06] rounded-full overflow-hidden">
                    <div
                      className={cn(
                        "h-full rounded-full",
                        ms.score >= 0.8
                          ? "bg-emerald-500"
                          : ms.score >= 0.5
                            ? "bg-amber-500"
                            : "bg-red-500",
                      )}
                      style={{ width: `${(ms.score * 100).toFixed(1)}%` }}
                    />
                  </div>
                  <span
                    className={cn(
                      "font-[family-name:var(--font-mono)] text-xs tabular-nums w-12 text-right",
                      scoreColor(ms.score),
                    )}
                  >
                    {(ms.score * 100).toFixed(1)}
                  </span>
                </div>
              ))}
            </div>
          </Section>
        )}

        {/* Per-sample breakdown — each sample is a card that parses and
            visualises the judge's response (verdict, rationale, evidence) */}
        {callsByModel.size > 0 && (
          <Section title="Samples">
            <div className="space-y-3">
              {Array.from(callsByModel.entries()).map(([model, calls]) => (
                <ModelSampleGroup
                  key={model}
                  model={model}
                  calls={calls}
                  defaultOpen={callsByModel.size === 1}
                />
              ))}
            </div>
          </Section>
        )}
      </div>
    </div>
  );
}

function ModelSampleGroup({
  model,
  calls,
  defaultOpen,
}: {
  model: string;
  calls: JudgeCall[];
  defaultOpen: boolean;
}) {
  const scores = calls
    .map((c) => c.score)
    .filter((s): s is number => typeof s === "number");
  const avg =
    scores.length > 0 ? scores.reduce((a, b) => a + b, 0) / scores.length : null;

  return (
    <details open={defaultOpen} className="group">
      <summary className="flex items-center gap-3 px-3 h-9 cursor-pointer border border-white/[0.06] rounded-md hover:bg-white/[0.02] group-open:rounded-b-none group-open:border-b-0">
        <span className="font-[family-name:var(--font-mono)] text-[12px] text-white/75 flex-1 truncate">
          {model}
        </span>
        {avg != null && (
          <span
            className={cn(
              "font-[family-name:var(--font-mono)] text-[11px] tabular-nums",
              scoreColor(avg),
            )}
          >
            {(avg * 100).toFixed(1)}
          </span>
        )}
        <span className="text-[10px] text-white/40 uppercase tracking-[0.12em]">
          {calls.length}
          {calls.length === 1 ? " sample" : " samples"}
        </span>
      </summary>
      <div className="space-y-2 border border-t-0 border-white/[0.06] rounded-b-md p-2 bg-black/20">
        {calls.map((c, i) => (
          <JudgeSampleCard key={`${c.sampleIndex ?? i}`} call={c} />
        ))}
      </div>
    </details>
  );
}

function VarianceSpread({
  min,
  max,
  mean,
  variance,
}: {
  min: number;
  max: number;
  mean: number;
  variance?: number;
}) {
  const minPct = min * 100;
  const maxPct = max * 100;
  const meanPct = mean * 100;
  return (
    <div>
      <div className="relative h-7">
        {/* Full-range track */}
        <div className="absolute inset-x-0 top-1/2 -translate-y-1/2 h-px bg-white/[0.08]" />
        {/* Spread */}
        <div
          className="absolute top-1/2 -translate-y-1/2 h-[3px] bg-white/25 rounded-full"
          style={{ left: `${minPct}%`, width: `${maxPct - minPct}%` }}
        />
        {/* Min / max ticks */}
        <Tick pct={minPct} label={`${minPct.toFixed(1)}`} />
        <Tick pct={maxPct} label={`${maxPct.toFixed(1)}`} />
        {/* Mean indicator */}
        <div
          className="absolute top-1/2 -translate-y-1/2 w-[2px] h-4 bg-emerald-400 rounded-full"
          style={{ left: `${meanPct}%` }}
        />
      </div>
      <div className="flex justify-between text-[10px] mt-1.5 uppercase tracking-[0.14em] text-white/35">
        <span>min</span>
        <span className="text-emerald-300/80">
          mean {meanPct.toFixed(1)}
          {variance != null ? ` · σ² ${variance.toFixed(4)}` : ""}
        </span>
        <span>max</span>
      </div>
    </div>
  );
}

function Tick({ pct, label }: { pct: number; label: string }) {
  return (
    <div
      className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2"
      style={{ left: `${pct}%` }}
    >
      <div className="w-[2px] h-2 bg-white/40 mx-auto" />
      <div className="text-[9px] text-white/40 font-[family-name:var(--font-mono)] tabular-nums mt-1 whitespace-nowrap text-center">
        {label}
      </div>
    </div>
  );
}

/* --------------------------------------------------------- Shared pieces */

function InspectorHeader({
  kicker,
  title,
  subtitle,
  stateKind,
  score,
}: {
  kicker: string;
  title: string;
  subtitle?: string;
  stateKind:
    | "pass"
    | "fail"
    | "error"
    | "unavailable"
    | "available"
    | "pending";
  score?: number;
}) {
  return (
    <div className="border-b border-white/[0.06] px-6 pt-5 pb-4">
      <div className="flex items-center gap-2 text-[10px] uppercase tracking-[0.18em] text-white/40 mb-2">
        <StateDot state={stateKind} />
        {kicker}
        {subtitle && (
          <>
            <span className="text-white/20">·</span>
            <span className="normal-case tracking-normal text-white/55">
              {subtitle}
            </span>
          </>
        )}
      </div>
      <div className="flex items-baseline justify-between gap-4">
        <h2 className="text-[17px] leading-tight text-white/95 truncate font-medium tracking-[-0.01em]">
          {humanizeKey(title)}
        </h2>
        {score != null && (
          <span
            className={cn(
              "font-[family-name:var(--font-mono)] text-2xl tabular-nums",
              scoreColor(score),
            )}
          >
            {(score * 100).toFixed(1)}
          </span>
        )}
      </div>
      <div className="mt-0.5 text-[11px] text-white/35 font-[family-name:var(--font-mono)] truncate">
        {title}
      </div>
    </div>
  );
}

function DataRow({
  label,
  value,
}: {
  label: string;
  value: React.ReactNode;
}) {
  return (
    <div className="flex items-start gap-4 border-b border-white/[0.04] pb-3">
      <span className="text-[10px] uppercase tracking-[0.16em] text-white/35 w-24 shrink-0 pt-1">
        {label}
      </span>
      <div className="flex-1 min-w-0">{value}</div>
    </div>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <h3 className="text-[10px] uppercase tracking-[0.2em] text-white/55 mb-2.5 font-medium">
        {title}
      </h3>
      {children}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-[#060606] px-3 py-2.5 flex flex-col gap-0.5">
      <span className="text-[9px] uppercase tracking-[0.16em] text-white/35">
        {label}
      </span>
      <span className="font-[family-name:var(--font-mono)] text-sm text-white/90 tabular-nums">
        {value}
      </span>
    </div>
  );
}

const sourceKindLabel: Record<ScorecardSource["kind"], string> = {
  run_event: "Run event",
  tool_call: "Tool call",
  final_output: "Final output",
};

function ReplaySourceLink({
  source,
  replayBasePath,
}: {
  source?: ScorecardSource;
  replayBasePath?: string;
}) {
  if (!source || source.sequence == null) return null;

  const label = sourceKindLabel[source.kind] ?? "Run event";
  const href = replayBasePath
    ? `${replayBasePath}?step=${source.sequence}`
    : undefined;
  const meta = source.event_type || source.field_path;

  const body = (
    <div className="flex items-center justify-between gap-3">
      <div className="flex flex-col gap-0.5 min-w-0">
        <span className="text-[10px] uppercase tracking-[0.18em] text-white/45">
          {label}
        </span>
        <span className="font-[family-name:var(--font-mono)] text-[12px] text-white/85 truncate">
          #{source.sequence}
          {meta ? ` · ${meta}` : ""}
        </span>
      </div>
      {href && (
        <span className="flex items-center gap-1 text-[11px] uppercase tracking-[0.18em] text-white/65 group-hover:text-white">
          View in replay
          <ArrowUpRight className="size-3.5" />
        </span>
      )}
    </div>
  );

  if (href) {
    return (
      <Link
        href={href}
        className="group block rounded-2xl border border-white/[0.08] bg-white/[0.02] px-4 py-3 hover:border-white/[0.18] hover:bg-white/[0.04] transition-colors"
      >
        {body}
      </Link>
    );
  }

  return (
    <div className="rounded-2xl border border-white/[0.06] bg-white/[0.015] px-4 py-3">
      {body}
    </div>
  );
}
