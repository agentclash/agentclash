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
  low: "bg-white",
  medium: "bg-white/50",
  high: "bg-white/25",
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
      <div className="grid grid-cols-[minmax(0,1fr)_repeat(3,2rem)] items-end gap-x-2 border-b border-white/[0.08] pb-2">
        <span className="font-mono text-[9px] uppercase tracking-[0.16em] text-white/40">
          Risk
        </span>
        {SEVERITIES.map((severity) => (
          <span
            key={severity}
            className="text-center font-mono text-[9px] uppercase tracking-[0.1em] text-white/40"
          >
            {severity === "medium" ? "med" : severity}
          </span>
        ))}
      </div>

      <div className="divide-y divide-white/[0.05]">
        {risks.map((risk) => (
          <details key={risk.risk} className="group">
            <summary className="grid cursor-pointer list-none grid-cols-[minmax(0,1fr)_repeat(3,2rem)] items-center gap-x-2 py-3 transition-colors marker:content-none hover:bg-white/[0.02] [&::-webkit-details-marker]:hidden">
              <span className="flex min-w-0 items-center gap-2 pr-2">
                <ChevronDown className="size-3.5 shrink-0 text-white/30 transition-transform group-open:rotate-180" />
                <span className="truncate text-[13px] text-white/80">
                  {risk.risk}
                </span>
              </span>
              {SEVERITIES.map((severity) => (
                <span key={severity} className="flex justify-center">
                  <span
                    className={cn(
                      "h-3.5 w-7 rounded-sm",
                      severity === risk.severity
                        ? CELL_FILL[severity]
                        : "border border-white/[0.06]",
                    )}
                  />
                </span>
              ))}
            </summary>
            <p className="pb-3 pl-[22px] pr-2 text-[13px] leading-6 text-white/50">
              {risk.mitigation}
            </p>
          </details>
        ))}
      </div>
    </div>
  );
}
