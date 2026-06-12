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
  low: "bg-white/25",
  medium: "bg-white/60",
  high: "bg-white",
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
      <div className="grid grid-cols-[minmax(0,1fr)_repeat(3,2.25rem)] items-end gap-x-1 border-b border-white/[0.08] pb-2">
        <span className="font-mono text-[9px] uppercase tracking-[0.18em] text-white/30">
          Failure mode
        </span>
        {SEVERITIES.map((severity) => (
          <span
            key={severity}
            className="text-center font-mono text-[9px] uppercase tracking-[0.12em] text-white/30"
          >
            {severity === "medium" ? "med" : severity}
          </span>
        ))}
      </div>

      <div className="divide-y divide-white/[0.05]">
        {risks.map((risk) => (
          <details key={risk.risk} className="group">
            <summary className="grid cursor-pointer list-none grid-cols-[minmax(0,1fr)_repeat(3,2.25rem)] items-center gap-x-1 py-2.5 transition-colors marker:content-none hover:bg-white/[0.02] [&::-webkit-details-marker]:hidden">
              <span className="flex min-w-0 items-center gap-1.5 pr-2">
                <ChevronDown className="size-3 shrink-0 text-white/25 transition-transform group-open:rotate-180" />
                <span className="truncate text-[13px] text-white/75">
                  {risk.risk}
                </span>
              </span>
              {SEVERITIES.map((severity) => (
                <span key={severity} className="flex justify-center">
                  <span
                    className={cn(
                      "h-3.5 w-7",
                      severity === risk.severity
                        ? CELL_FILL[severity]
                        : "border border-white/[0.07]",
                    )}
                  />
                </span>
              ))}
            </summary>
            <p className="pb-3 pl-[18px] pr-2 text-xs leading-5 text-white/45">
              {risk.mitigation}
            </p>
          </details>
        ))}
      </div>
    </div>
  );
}
