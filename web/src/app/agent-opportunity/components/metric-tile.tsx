"use client";

import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

function toneClass(tone: "good" | "warn" | "bad" | "neutral") {
  switch (tone) {
    case "good":
      return "text-emerald-300";
    case "warn":
      return "text-amber-300";
    case "bad":
      return "text-red-300";
    default:
      return "text-white/75";
  }
}

function barClass(tone: "good" | "warn" | "bad" | "neutral") {
  switch (tone) {
    case "good":
      return "bg-emerald-500";
    case "warn":
      return "bg-amber-500";
    case "bad":
      return "bg-red-500";
    default:
      return "bg-white/40";
  }
}

export function MetricTile({
  icon: Icon,
  label,
  value,
  hint,
  score,
  tone,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  hint: string;
  score: number;
  tone: "good" | "warn" | "bad" | "neutral";
}) {
  return (
    <article className="flex min-w-0 flex-col gap-3 bg-[#060606] p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <Icon className="size-3.5 shrink-0 text-white/35" />
          <p className="truncate text-[11px] uppercase tracking-[0.14em] text-white/40">
            {label}
          </p>
        </div>
        <span
          className={cn(
            "font-[family-name:var(--font-mono)] text-[11px] tabular-nums",
            toneClass(tone),
          )}
        >
          {score}
        </span>
      </div>
      <div>
        <p className="text-sm font-medium text-white">{value}</p>
        <p className="mt-1 text-xs leading-5 text-white/45">{hint}</p>
      </div>
      <div className="h-[3px] overflow-hidden rounded-full bg-white/[0.05]">
        <div
          className={cn("h-full rounded-full transition-all", barClass(tone))}
          style={{ width: `${Math.max(8, Math.min(100, score))}%` }}
        />
      </div>
    </article>
  );
}
