import type { AgentTryout, AgentTryoutStatus } from "@/lib/api/types";

export function tryoutStatusVariant(
  status: AgentTryoutStatus,
): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "completed":
      return "default";
    case "failed":
      return "destructive";
    case "cancelled":
      return "outline";
    default:
      return "secondary";
  }
}

export function tryoutIsActive(status: AgentTryoutStatus): boolean {
  return status === "queued" || status === "running";
}

export function tryoutModelLabel(tryout: {
  selected_model_policy?: unknown;
}): string {
  const policy = tryout.selected_model_policy;
  if (!policy || typeof policy !== "object") return "hosted default";
  const { models, mode } = policy as {
    models?: { provider?: string; model?: string }[];
    mode?: string;
  };
  if (Array.isArray(models) && models.length > 0) {
    return models
      .map((item) => [item.provider, item.model].filter(Boolean).join("/"))
      .join(", ");
  }
  return typeof mode === "string" && mode.trim() ? mode : "hosted default";
}

export function formatTryoutCost(tryout: AgentTryout): string {
  if (typeof tryout.actual_cost_usd === "number") {
    return `$${tryout.actual_cost_usd.toFixed(2)}`;
  }
  return `≤ $${tryout.cost_limit_usd.toFixed(2)}`;
}

export function formatTryoutLatency(latencyMs: number | undefined): string {
  if (typeof latencyMs !== "number") return "—";
  if (latencyMs < 1000) return `${latencyMs}ms`;
  return `${(latencyMs / 1000).toFixed(1)}s`;
}
