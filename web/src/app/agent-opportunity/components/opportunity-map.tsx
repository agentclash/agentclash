"use client";

import { cn } from "@/lib/utils";
import type { AgentOpportunityReport } from "@/lib/agent-opportunity";

type UseCase = AgentOpportunityReport["useCases"][number];
type Level = UseCase["fit"];

// Top row is high fit; left column is low complexity, so the
// build zone (high value, low effort) sits top-left like an
// effort/impact matrix.
const FIT_ROWS: Level[] = ["high", "medium", "low"];
const COMPLEXITY_COLS: Level[] = ["low", "medium", "high"];

const HATCH =
  "repeating-linear-gradient(135deg, rgba(255,255,255,0.055) 0, rgba(255,255,255,0.055) 1px, transparent 1px, transparent 7px)";

export function OpportunityMap({
  useCases,
  className,
}: {
  useCases: UseCase[];
  className?: string;
}) {
  return (
    <div className={className}>
      <div className="flex gap-2">
        <div className="flex w-4 shrink-0 items-center justify-center">
          <span className="rotate-180 font-mono text-[9px] uppercase tracking-[0.2em] text-white/30 [writing-mode:vertical-rl]">
            Fit →
          </span>
        </div>

        <div className="min-w-0 flex-1">
          <div className="grid grid-cols-3 gap-px border border-white/[0.08] bg-white/[0.06]">
            {FIT_ROWS.map((fit) =>
              COMPLEXITY_COLS.map((complexity) => {
                const matches = useCases
                  .map((useCase, index) => ({ useCase, index }))
                  .filter(
                    ({ useCase }) =>
                      useCase.fit === fit && useCase.complexity === complexity,
                  );
                const isBuildZone = fit === "high" && complexity === "low";
                const isAvoidZone = fit === "low" && complexity === "high";

                return (
                  <div
                    key={`${fit}-${complexity}`}
                    className="relative flex min-h-[76px] flex-wrap content-center items-center justify-center gap-1.5 bg-[#080808] p-2"
                    style={isBuildZone ? { backgroundImage: HATCH } : undefined}
                  >
                    {isBuildZone ? (
                      <span className="pointer-events-none absolute left-1.5 top-1.5 font-mono text-[8px] uppercase tracking-[0.18em] text-white/40">
                        Build zone
                      </span>
                    ) : null}
                    {isAvoidZone ? (
                      <span className="pointer-events-none absolute bottom-1.5 right-1.5 font-mono text-[8px] uppercase tracking-[0.18em] text-white/20">
                        Avoid
                      </span>
                    ) : null}
                    {matches.map(({ useCase, index }) => (
                      <span
                        key={useCase.title}
                        title={useCase.title}
                        className={cn(
                          "border px-1.5 py-0.5 font-mono text-[10px] tabular-nums",
                          fit === "high"
                            ? "border-white/40 bg-white text-[#060606]"
                            : "border-white/25 bg-[#0c0c0c] text-white/80",
                        )}
                      >
                        {String(index + 1).padStart(2, "0")}
                      </span>
                    ))}
                  </div>
                );
              }),
            )}
          </div>

          <div className="mt-1.5 grid grid-cols-3 font-mono text-[9px] uppercase tracking-[0.16em] text-white/30">
            <span>Low</span>
            <span className="text-center">Complexity →</span>
            <span className="text-right">High</span>
          </div>
        </div>
      </div>
    </div>
  );
}
