"use client";

import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AgentOpportunityRisk } from "@/lib/agent-opportunity";

const SEVERITIES: AgentOpportunityRisk["severity"][] = [
  "low",
  "medium",
  "high",
];

const CELL_FILL: Record<AgentOpportunityRisk["severity"], string> = {
  low: "bg-emerald-400/90 shadow-[0_0_10px_rgba(52,211,153,0.45)]",
  medium: "bg-amber-300 shadow-[0_0_10px_rgba(252,211,77,0.45)]",
  high: "bg-red-400 shadow-[0_0_12px_rgba(248,113,113,0.55)]",
};

export function RiskHeatmap({
  risks,
  className,
}: {
  risks: AgentOpportunityRisk[];
  className?: string;
}) {
  return (
    <div className={className}>
      <div className="grid grid-cols-[minmax(0,1fr)_repeat(3,2.5rem)] items-end gap-x-1 border-b border-white/10 pb-2">
        <span className="font-mono text-[10px] uppercase tracking-[0.16em] text-white/55">
          Failure mode
        </span>
        {SEVERITIES.map((severity) => (
          <span
            key={severity}
            className="text-center font-mono text-[10px] uppercase tracking-[0.1em] text-white/55"
          >
            {severity === "medium" ? "med" : severity}
          </span>
        ))}
      </div>

      <div className="divide-y divide-white/[0.06]">
        {risks.map((risk) => (
          <details key={risk.risk} className="group">
            <summary className="grid cursor-pointer list-none grid-cols-[minmax(0,1fr)_repeat(3,2.5rem)] items-center gap-x-1 py-3 transition-colors marker:content-none hover:bg-white/[0.03] [&::-webkit-details-marker]:hidden">
              <span className="flex min-w-0 items-center gap-2 pr-2">
                <ChevronDown className="size-3.5 shrink-0 text-white/40 transition-transform group-open:rotate-180" />
                <span className="truncate text-[13px] text-white/85">
                  {risk.risk}
                </span>
              </span>
              {SEVERITIES.map((severity) => (
                <span key={severity} className="flex justify-center">
                  <span
                    className={cn(
                      "h-4 w-8 rounded-[2px]",
                      severity === risk.severity
                        ? CELL_FILL[severity]
                        : "border border-white/10",
                    )}
                  />
                </span>
              ))}
            </summary>
            <p className="pb-3 pl-[22px] pr-2 text-[13px] leading-6 text-white/60">
              {risk.mitigation}
            </p>
          </details>
        ))}
      </div>
    </div>
  );
}
