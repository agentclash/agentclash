"use client";

import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import type { OpportunityMetrics } from "../report-metrics";

type Row = {
  key: keyof OpportunityMetrics;
  label: string;
};

const ROWS: Row[] = [
  { key: "workflowFit", label: "Workflow fit" },
  { key: "roiSignal", label: "ROI signal" },
  { key: "evalReadiness", label: "Eval readiness" },
  { key: "riskProfile", label: "Risk safety" },
];

export function DimensionRadar({
  metrics,
  className,
}: {
  metrics: OpportunityMetrics;
  className?: string;
}) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    const frame = requestAnimationFrame(() => setMounted(true));
    return () => cancelAnimationFrame(frame);
  }, []);

  return (
    <div
      className={cn("flex flex-col gap-4", className)}
      role="img"
      aria-label={`Dimension profile: ${ROWS.map(
        (row) => `${row.label} ${metrics[row.key]} of 100`,
      ).join(", ")}`}
    >
      {ROWS.map((row) => {
        const value = metrics[row.key];
        return (
          <div key={row.key} className="flex items-center gap-4">
            <span className="w-28 shrink-0 text-right font-mono text-[10px] uppercase tracking-[0.14em] text-white/40">
              {row.label}
            </span>
            <div className="relative flex-1 overflow-hidden rounded-sm bg-white/[0.06]">
              <div
                className="h-2 rounded-sm bg-white transition-[width] duration-700 ease-out motion-reduce:transition-none"
                style={{
                  width: mounted ? `${Math.max(2, Math.min(100, value))}%` : "0%",
                }}
              />
            </div>
            <span className="w-10 shrink-0 font-mono text-[13px] font-medium tabular-nums text-white/90">
              {value}
            </span>
          </div>
        );
      })}
    </div>
  );
}
