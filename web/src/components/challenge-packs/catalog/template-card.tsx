"use client";

import { Badge } from "@/components/ui/badge";
import type { CatalogPack } from "../lib/types";

const difficultyVariant: Record<string, "default" | "secondary" | "outline"> = {
  easy: "secondary",
  medium: "outline",
  hard: "default",
  expert: "default",
};

function formatCost(usd?: number): string | null {
  if (usd == null) return null;
  return usd < 0.01 ? `~$${usd.toFixed(3)}` : `~$${usd.toFixed(2)}`;
}

function formatRuntime(ms?: number): string | null {
  if (ms == null) return null;
  return `~${Math.round(ms / 1000)}s`;
}

export function TemplateCard({ pack, onSelect }: { pack: CatalogPack; onSelect: () => void }) {
  const cost = formatCost(pack.estimated_cost_usd);
  const runtime = formatRuntime(pack.estimated_runtime_ms);

  return (
    <button
      type="button"
      onClick={onSelect}
      className="flex h-full flex-col rounded-xl border border-border bg-card p-4 text-left transition-colors hover:border-foreground/30 hover:bg-muted/30"
    >
      <span className="text-sm font-semibold">{pack.name}</span>
      {pack.description && (
        <p className="mt-1.5 line-clamp-3 text-sm text-muted-foreground">{pack.description}</p>
      )}
      <div className="mt-auto flex flex-wrap items-center gap-1.5 pt-3">
        <Badge variant="outline">{pack.family}</Badge>
        {pack.difficulty && (
          <Badge variant={difficultyVariant[pack.difficulty] ?? "outline"}>{pack.difficulty}</Badge>
        )}
        <Badge variant="outline">{pack.execution_mode}</Badge>
        {(cost || runtime) && (
          <span className="ml-auto text-xs text-muted-foreground">
            {[cost, runtime].filter(Boolean).join(" · ")}
          </span>
        )}
      </div>
    </button>
  );
}
