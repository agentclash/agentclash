"use client";

import type {
  Run,
  RunAgent,
  ScorecardResponse,
} from "@/lib/api/types";
import { ScoreMeter } from "./score-meter";
import { Panel } from "./panel";
import { formatDuration, formatTimestamp, sortDimensionKeys } from "./utils";
import { CheckCircle2, XCircle, AlertTriangle, Clock } from "lucide-react";

export function Hero({
  run,
  agent,
  scorecard,
}: {
  run: Run;
  agent: RunAgent;
  scorecard: ScorecardResponse;
}) {
  const doc = scorecard.scorecard;
  const dims = doc?.dimensions ?? {};
  const dimKeys = sortDimensionKeys(Object.keys(dims));
  const segments = dimKeys.map((key) => ({
    key,
    score: dims[key].score,
  }));

  const duration = formatDuration(agent.started_at, agent.finished_at);
  const scoredAt = formatTimestamp(scorecard.updated_at);

  const passed = doc?.passed;
  const strategy = doc?.strategy;
  const warnings = doc?.warnings ?? [];

  const validatorTotal = doc?.validator_details?.length ?? 0;
  const validatorPass = (doc?.validator_details ?? []).filter(
    (v) => v.verdict === "pass",
  ).length;

  return (
    <Panel className="overflow-hidden">
      {/* Top strip: agent + run identifiers */}
      <div className="flex items-center justify-between px-5 pt-4 pb-3 border-b border-white/[0.06]">
        <div className="flex items-baseline gap-3 min-w-0">
          <h1 className="font-[family-name:var(--font-display)] text-2xl leading-none tracking-[-0.01em] text-white/95 truncate">
            {agent.label}
          </h1>
          <span className="text-white/30 text-sm">·</span>
          <span className="text-xs text-white/50 font-[family-name:var(--font-mono)] truncate">
            {run.name}
          </span>
        </div>
        <div className="flex items-center gap-2 text-[10px] uppercase tracking-[0.14em] text-white/35">
          <Clock className="size-3" />
          <span className="font-[family-name:var(--font-mono)] normal-case tracking-normal text-white/55">
            {duration}
          </span>
          <span className="text-white/20">·</span>
          <span className="font-[family-name:var(--font-mono)] normal-case tracking-normal text-white/45">
            {scoredAt}
          </span>
        </div>
      </div>

      {/* Main body: meter + verdict + dimension strip */}
      <div className="grid grid-cols-[auto_1fr] gap-6 p-6">
        <div className="flex flex-col items-center gap-3">
          <ScoreMeter overall={scorecard.overall_score} dimensions={segments} size={180} />
          {passed != null && <VerdictPill passed={passed} />}
        </div>

        <div className="flex flex-col gap-4 min-w-0">
          {/* Meta row */}
          <div className="flex flex-wrap items-center gap-x-5 gap-y-1.5 text-[11px]">
            {strategy && (
              <MetaChip label="Strategy" value={strategy} />
            )}
            <MetaChip
              label="Validators"
              value={`${validatorPass}/${validatorTotal}`}
              valueTone={
                validatorTotal === 0
                  ? "muted"
                  : validatorPass === validatorTotal
                    ? "good"
                    : validatorPass === 0
                      ? "bad"
                      : "warn"
              }
            />
            <MetaChip
              label="Dimensions"
              value={`${dimKeys.length}`}
            />
            <MetaChip
              label="Spec"
              value={scorecard.evaluation_spec_id.slice(0, 8)}
              mono
            />
            {doc?.metric_summary?.run_total_tokens != null && (
              <div className="flex items-baseline gap-1.5">
                <span className="text-[9px] uppercase tracking-[0.18em] text-white/35">Tokens</span>
                <span className="text-xs text-white/80 font-[family-name:var(--font-mono)]">
                  {doc.metric_summary.run_total_tokens.toLocaleString()}
                  {(doc.metric_summary.run_race_context_tokens ?? 0) > 0 && (
                    <span className="text-white/40 ml-1">
                      ({doc.metric_summary.run_agent_tokens?.toLocaleString() ?? 0} agent + {doc.metric_summary.run_race_context_tokens.toLocaleString()} context)
                    </span>
                  )}
                </span>
              </div>
            )}
          </div>

          {/* Overall reason — full width, no cramping */}
          {doc?.overall_reason && (
            <p className="text-sm text-white/65 leading-relaxed max-w-2xl">
              {doc.overall_reason}
            </p>
          )}

          {/* Warnings promoted to hero */}
          {warnings.length > 0 && (
            <div className="flex items-start gap-2 border border-amber-500/25 bg-amber-500/[0.04] rounded-md px-3 py-2">
              <AlertTriangle className="size-3.5 text-amber-400 shrink-0 mt-0.5" />
              <div className="flex-1 min-w-0">
                <div className="text-[11px] uppercase tracking-[0.14em] text-amber-400/85 mb-1">
                  {warnings.length} {warnings.length === 1 ? "warning" : "warnings"}
                </div>
                <ul className="space-y-0.5">
                  {warnings.map((w, i) => (
                    <li
                      key={i}
                      className="text-[12px] text-amber-200/75 leading-snug"
                    >
                      {w}
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          )}
        </div>
      </div>
    </Panel>
  );
}

function VerdictPill({ passed }: { passed: boolean }) {
  if (passed) {
    return (
      <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full border border-emerald-500/30 bg-emerald-500/[0.08]">
        <CheckCircle2 className="size-3 text-emerald-400" />
        <span className="text-[10px] uppercase tracking-[0.16em] text-emerald-300">
          Passed
        </span>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full border border-red-500/30 bg-red-500/[0.08]">
      <XCircle className="size-3 text-red-400" />
      <span className="text-[10px] uppercase tracking-[0.16em] text-red-300">
        Failed
      </span>
    </div>
  );
}

function MetaChip({
  label,
  value,
  valueTone = "default",
  mono = false,
}: {
  label: string;
  value: string;
  valueTone?: "default" | "muted" | "good" | "warn" | "bad";
  mono?: boolean;
}) {
  const tone = {
    default: "text-white/80",
    muted: "text-white/40",
    good: "text-emerald-300",
    warn: "text-amber-300",
    bad: "text-red-300",
  }[valueTone];
  return (
    <div className="flex items-baseline gap-1.5">
      <span className="text-[9px] uppercase tracking-[0.18em] text-white/35">
        {label}
      </span>
      <span
        className={`text-xs ${tone} ${mono ? "font-[family-name:var(--font-mono)]" : ""}`}
      >
        {value}
      </span>
    </div>
  );
}
