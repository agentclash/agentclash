"use client";

import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import type { AgentOpportunityReport } from "@/lib/agent-opportunity";

type UseCase = AgentOpportunityReport["useCases"][number];
type Level = UseCase["fit"];

const FIT_Y: Record<Level, number> = { high: 22, medium: 50, low: 78 };
const COMPLEXITY_X: Record<Level, number> = { low: 20, medium: 50, high: 80 };

const FIT_DOT: Record<Level, string> = {
  high: "bg-white",
  medium: "bg-white/50",
  low: "bg-white/25",
};

const FIT_TEXT: Record<Level, string> = {
  high: "text-white",
  medium: "text-white/60",
  low: "text-white/40",
};

type PlacedUseCase = {
  useCase: UseCase;
  index: number;
  x: number;
  y: number;
};

function placeUseCases(useCases: UseCase[]): PlacedUseCase[] {
  const groups = new Map<string, number[]>();
  useCases.forEach((useCase, index) => {
    const key = `${useCase.fit}-${useCase.complexity}`;
    groups.set(key, [...(groups.get(key) ?? []), index]);
  });

  return useCases.map((useCase, index) => {
    const siblings = groups.get(`${useCase.fit}-${useCase.complexity}`)!;
    const position = siblings.indexOf(index);
    const spread = (position - (siblings.length - 1) / 2) * 12;
    return {
      useCase,
      index,
      x: COMPLEXITY_X[useCase.complexity] + spread,
      y: FIT_Y[useCase.fit],
    };
  });
}

const QUADRANTS = [
  { label: "Quick wins", position: "left-3 top-3", text: "text-white/70" },
  { label: "Big bets", position: "right-3 top-3", text: "text-white/50" },
  { label: "Low stakes", position: "bottom-3 left-3", text: "text-white/40" },
  { label: "Avoid", position: "bottom-3 right-3", text: "text-white/30" },
];

export function OpportunityMap({
  useCases,
  className,
}: {
  useCases: UseCase[];
  className?: string;
}) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    const frame = requestAnimationFrame(() => setMounted(true));
    return () => cancelAnimationFrame(frame);
  }, []);

  const placed = placeUseCases(useCases);

  return (
    <div className={className}>
      <div className="relative aspect-[8/5] overflow-hidden border border-white/[0.08] bg-[#080808]">
        {/* Midlines */}
        <div
          className="absolute left-1/2 top-0 h-full w-px bg-white/[0.08]"
          aria-hidden
        />
        <div
          className="absolute left-0 top-1/2 h-px w-full bg-white/[0.08]"
          aria-hidden
        />

        {/* Quadrant labels */}
        {QUADRANTS.map((quadrant) => (
          <span
            key={quadrant.label}
            className={cn(
              "pointer-events-none absolute font-mono text-[9px] uppercase tracking-[0.18em]",
              quadrant.position,
              quadrant.text,
            )}
          >
            {quadrant.label}
          </span>
        ))}

        {/* Use case dots — clean squares */}
        {placed.map(({ useCase, index, x, y }, order) => (
          <span
            key={useCase.title}
            title={`${useCase.title} — ${useCase.fit} fit, ${useCase.complexity} complexity`}
            className={cn(
              "absolute flex size-6 -translate-x-1/2 -translate-y-1/2 items-center justify-center font-mono text-[10px] font-medium text-[#080606] transition-all duration-500 motion-reduce:transition-none",
              FIT_DOT[useCase.fit],
            )}
            style={{
              left: `${x}%`,
              top: mounted ? `${y}%` : "110%",
              opacity: mounted ? 1 : 0,
              transitionDelay: `${order * 80}ms`,
            }}
          >
            {index + 1}
          </span>
        ))}
      </div>

      {/* Axis labels */}
      <div className="mt-2 flex items-center justify-between font-mono text-[9px] uppercase tracking-[0.14em] text-white/40">
        <span>← Easier</span>
        <span>Harder →</span>
      </div>

      {/* Legend */}
      <ul className="mt-4 space-y-1.5">
        {placed.map(({ useCase, index }) => (
          <li key={useCase.title} className="flex items-center gap-3">
            <span
              className={cn(
                "flex size-4 shrink-0 items-center justify-center font-mono text-[9px] font-medium text-[#080606]",
                FIT_DOT[useCase.fit],
              )}
              aria-hidden
            >
              {index + 1}
            </span>
            <span className="min-w-0 truncate text-[13px] text-white/80">
              {useCase.title}
            </span>
            <span
              className={cn(
                "ml-auto shrink-0 font-mono text-[9px] uppercase tracking-[0.12em]",
                FIT_TEXT[useCase.fit],
              )}
            >
              {useCase.fit}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
