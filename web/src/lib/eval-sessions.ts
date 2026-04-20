import type {
  EvalSessionAggregateResult,
  EvalSessionDetail,
  EvalSessionMetricAggregate,
  EvalSessionPassMetricSeries,
} from "@/lib/api/types";

function stripRepeatedRunSuffix(value: string): string {
  return value.replace(/\s*\[\d+\/\d+\]\s*$/, "").trim();
}

export function shortEvalSessionId(id: string): string {
  return id.slice(0, 8);
}

export function deriveEvalSessionTitle(detail: Pick<EvalSessionDetail, "eval_session" | "runs">): string {
  const firstRunName = detail.runs[0]?.name?.trim();
  if (firstRunName) {
    const baseName = stripRepeatedRunSuffix(firstRunName);
    if (baseName) return baseName;
  }
  return `Eval Session ${shortEvalSessionId(detail.eval_session.id)}`;
}

export function deriveEvalSessionMode(
  runs: Array<{ execution_mode?: string }> | undefined,
  aggregateResult?: EvalSessionAggregateResult | null,
): "single_agent" | "comparison" | null {
  const runMode = runs?.find((run) => run.execution_mode)?.execution_mode;
  if (runMode === "single_agent" || runMode === "comparison") {
    return runMode;
  }
  if ((aggregateResult?.participants?.length ?? 0) > 1) {
    return "comparison";
  }
  if ((aggregateResult?.participants?.length ?? 0) === 1) {
    return "single_agent";
  }
  return null;
}

export function formatEvalSessionMetricName(value: string): string {
  return value
    .replace(/_/g, " ")
    .replace(/\b\w/g, (match) => match.toUpperCase());
}

export function formatEvalSessionValue(value?: number | null): string {
  if (value == null || Number.isNaN(value)) return "—";
  if (Math.abs(value) <= 1) return `${(value * 100).toFixed(1)}%`;
  return value.toFixed(2);
}

export function formatEvalSessionRate(value?: number | null): string {
  if (value == null || Number.isNaN(value)) return "—";
  return `${(value * 100).toFixed(1)}%`;
}

export function formatEvalSessionRange(
  aggregate?: Pick<EvalSessionMetricAggregate, "interval"> | null,
): string {
  if (!aggregate?.interval) return "—";
  return `${formatEvalSessionValue(aggregate.interval.lower)} - ${formatEvalSessionValue(aggregate.interval.upper)}`;
}

export function passMetricAggregateForEffectiveK(
  series?: EvalSessionPassMetricSeries | null,
): EvalSessionMetricAggregate | null {
  if (!series) return null;
  return series.by_k[String(series.effective_k)] ?? null;
}

export function sortedAggregateDimensions(
  aggregateResult?: EvalSessionAggregateResult | null,
): Array<[string, EvalSessionMetricAggregate]> {
  return Object.entries(aggregateResult?.dimensions ?? {}).sort(([left], [right]) =>
    left.localeCompare(right),
  );
}
