"use client";

import type {
  LLMJudgeResult,
  MetricDetail,
  ValidatorDetail,
} from "@/lib/api/types";
import { Panel, PanelHeader } from "./panel";
import {
  Activity,
  Bot,
  FlaskConical,
  ChevronRight,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { scoreColor } from "@/lib/scores";
import { StateDot, normalizeState } from "./state-dot";
import { humanizeKey, parseJudgePayload } from "./utils";

export type InspectorTarget =
  | { kind: "validator"; detail: ValidatorDetail }
  | { kind: "metric"; detail: MetricDetail }
  | { kind: "judge"; detail: LLMJudgeResult };

export function ValidatorsPanel({
  validators,
  onInspect,
}: {
  validators: ValidatorDetail[];
  onInspect: (t: InspectorTarget) => void;
}) {
  if (validators.length === 0) return null;
  const passCount = validators.filter((v) => v.verdict === "pass").length;

  return (
    <Panel>
      <PanelHeader
        title="Validators"
        icon={<FlaskConical className="size-3.5" />}
        trailing={
          <span className="text-2xs text-white/45 font-[family-name:var(--font-mono)]">
            {passCount}/{validators.length}
          </span>
        }
      />
      <div className="divide-y divide-white/[0.05]">
        {validators.map((v) => (
          <Row
            key={v.key}
            onClick={() => onInspect({ kind: "validator", detail: v })}
          >
            <StateDot state={normalizeState(v.verdict, v.state)} />
            <span className="text-sm text-white/85 truncate">{v.key}</span>
            <span className="text-2xs uppercase tracking-[0.12em] text-white/35 shrink-0">
              {v.type.replace(/_/g, " ")}
            </span>
            <span className="flex-1" />
            {v.normalized_score != null && (
              <span
                className={cn(
                  "font-[family-name:var(--font-mono)] text-xs tabular-nums",
                  scoreColor(v.normalized_score),
                )}
              >
                {(v.normalized_score * 100).toFixed(1)}
              </span>
            )}
            <ChevronRight className="size-3.5 text-white/25 shrink-0" />
          </Row>
        ))}
      </div>
    </Panel>
  );
}

export function MetricsPanel({
  metrics,
  onInspect,
}: {
  metrics: MetricDetail[];
  onInspect: (t: InspectorTarget) => void;
}) {
  if (metrics.length === 0) return null;
  const availableCount = metrics.filter((m) => m.state === "available").length;

  return (
    <Panel>
      <PanelHeader
        title="Metrics"
        icon={<Activity className="size-3.5" />}
        trailing={
          <span className="text-2xs text-white/45 font-[family-name:var(--font-mono)]">
            {availableCount}/{metrics.length}
          </span>
        }
      />
      <div className="divide-y divide-white/[0.05]">
        {metrics.map((m) => (
          <Row
            key={m.key}
            onClick={() => onInspect({ kind: "metric", detail: m })}
          >
            <StateDot state={m.state === "available" ? "available" : "unavailable"} />
            <span className="text-sm text-white/85 truncate">{m.key}</span>
            <span className="text-2xs uppercase tracking-[0.12em] text-white/35 shrink-0">
              {m.collector.replace(/_/g, " ")}
            </span>
            <span className="flex-1" />
            <span
              className={cn(
                "font-[family-name:var(--font-mono)] text-xs tabular-nums",
                m.state === "available" ? "text-white/85" : "text-white/35",
              )}
            >
              {formatMetricValue(m)}
            </span>
            <ChevronRight className="size-3.5 text-white/25 shrink-0" />
          </Row>
        ))}
      </div>
    </Panel>
  );
}

export function JudgesPanel({
  judges,
  onInspect,
}: {
  judges: LLMJudgeResult[];
  onInspect: (t: InspectorTarget) => void;
}) {
  if (judges.length === 0) return null;
  const availableCount = judges.filter((j) => parseJudgePayload(j).available)
    .length;

  return (
    <Panel>
      <PanelHeader
        title="LLM Judges"
        icon={<Bot className="size-3.5" />}
        trailing={
          <span className="text-2xs text-white/45 font-[family-name:var(--font-mono)]">
            {availableCount}/{judges.length}
          </span>
        }
      />
      <div className="divide-y divide-white/[0.05]">
        {judges.map((judge) => {
          const parsed = parseJudgePayload(judge);
          return (
            <Row
              key={judge.id}
              onClick={() => onInspect({ kind: "judge", detail: judge })}
            >
              <StateDot
                state={parsed.available ? "available" : "unavailable"}
              />
              <span className="text-sm text-white/85 truncate">
                {judge.judge_key}
              </span>
              <span className="text-2xs uppercase tracking-[0.12em] text-white/35 shrink-0">
                {judge.mode.replace(/_/g, " ")}
              </span>
              <span className="flex-1" />
              <span className="hidden sm:inline font-[family-name:var(--font-mono)] text-2xs text-white/35 tabular-nums shrink-0">
                {judge.sample_count}×{judge.model_count}
              </span>
              {judge.confidence && (
                <span className="hidden md:inline text-2xs uppercase tracking-[0.12em] text-white/40 shrink-0">
                  {judge.confidence}
                </span>
              )}
              {judge.normalized_score != null && (
                <span
                  className={cn(
                    "font-[family-name:var(--font-mono)] text-xs tabular-nums shrink-0",
                    scoreColor(judge.normalized_score),
                  )}
                >
                  {(judge.normalized_score * 100).toFixed(1)}
                </span>
              )}
              <ChevronRight className="size-3.5 text-white/25 shrink-0" />
            </Row>
          );
        })}
      </div>
    </Panel>
  );
}

function Row({
  children,
  onClick,
  align = "center",
  className,
}: {
  children: React.ReactNode;
  onClick?: () => void;
  align?: "center" | "start";
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "w-full flex gap-3 px-4 h-11 text-left hover:bg-white/[0.025] transition-colors",
        align === "start" ? "items-start" : "items-center",
        className,
      )}
    >
      {children}
    </button>
  );
}

function formatMetricValue(m: MetricDetail): string {
  if (m.numeric_value != null) {
    const n = m.numeric_value;
    if (Math.abs(n) >= 1000) return n.toLocaleString();
    if (Number.isInteger(n)) return `${n}`;
    return n.toFixed(2);
  }
  if (m.boolean_value != null) return m.boolean_value ? "true" : "false";
  if (m.text_value != null) return m.text_value;
  return "—";
}

export { humanizeKey };
