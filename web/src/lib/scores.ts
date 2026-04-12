/** Shared score formatting and color helpers used by scorecard components. */

export function scorePercent(score?: number): string {
  if (score == null) return "\u2014";
  return `${(score * 100).toFixed(1)}%`;
}

export function scoreColor(score?: number): string {
  if (score == null) return "text-muted-foreground";
  if (score >= 0.8) return "text-emerald-400";
  if (score >= 0.5) return "text-amber-400";
  return "text-red-400";
}

export function barWidth(score?: number): string {
  if (score == null) return "0%";
  return `${(score * 100).toFixed(1)}%`;
}

export function barColor(score?: number): string {
  if (score == null) return "bg-muted";
  if (score >= 0.8) return "bg-emerald-500";
  if (score >= 0.5) return "bg-amber-500";
  return "bg-red-500";
}
