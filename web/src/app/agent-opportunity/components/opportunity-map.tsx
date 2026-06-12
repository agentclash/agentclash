"use client";

import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import type { AgentOpportunityReport } from "@/lib/agent-opportunity";

type UseCase = AgentOpportunityReport["useCases"][number];
type Level = UseCase["fit"];

// Impact/effort quadrant: high fit is at the top, low complexity on
// the left, so the top-left quadrant holds the quick wins.
const FIT_Y: Record<Level, number> = { high: 22, medium: 50, low: 78 };
const COMPLEXITY_X: Record<Level, number> = { low: 20, medium: 50, high: 80 };

const FIT_DOT: Record<Level, string> = {
  high: "bg-emerald-400 shadow-[0_0_14px_rgba(52,211,153,0.6)]",
  medium: "bg-amber-300 shadow-[0_0_14px_rgba(252,211,77,0.55)]",
  low: "bg-red-400 shadow-[0_0_14px_rgba(248,113,113,0.55)]",
};

const FIT_TEXT: Record<Level, string> = {
  high: "text-emerald-300",
  medium: "text-amber-200",
  low: "text-red-300",
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
    const spread = (position - (siblings.length - 1) / 2) * 11;
    return {
      useCase,
      index,
      x: COMPLEXITY_X[useCase.complexity] + spread,
      y: FIT_Y[useCase.fit],
    };
  });
}

const QUADRANTS = [
  {
    label: "Quick wins",
    position: "left-2.5 top-2",
    tint: "bg-emerald-400/[0.08]",
    text: "text-emerald-300/90",
  },
  {
    label: "Big bets",
    position: "right-2.5 top-2",
    tint: "bg-cyan-400/[0.06]",
    text: "text-cyan-300/80",
  },
  {
    label: "Low stakes",
    position: "bottom-2 left-2.5",
    tint: "",
    text: "text-white/45",
  },
  {
    label: "Avoid",
    position: "bottom-2 right-2.5",
    tint: "bg-red-400/[0.07]",
    text: "text-red-300/80",
  },
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
      <div className="relative aspect-[8/5] overflow-hidden border border-white/10 bg-[#0a0a0a]">
        <div className="absolute inset-0 grid grid-cols-2 grid-rows-2" aria-hidden>
          {QUADRANTS.map((quadrant) => (
            <div key={quadrant.label} className={quadrant.tint} />
          ))}
        </div>

        <div
          className="absolute left-1/2 top-0 h-full w-px border-l border-dashed border-white/15"
          aria-hidden
        />
        <div
          className="absolute left-0 top-1/2 h-px w-full border-t border-dashed border-white/15"
          aria-hidden
        />

        {QUADRANTS.map((quadrant) => (
          <span
            key={quadrant.label}
            className={cn(
              "pointer-events-none absolute font-mono text-[10px] uppercase tracking-[0.16em]",
              quadrant.position,
              quadrant.text,
            )}
          >
            {quadrant.label}
          </span>
        ))}

        {placed.map(({ useCase, index, x, y }, order) => (
          <span
            key={useCase.title}
            title={`${useCase.title} — ${useCase.fit} fit, ${useCase.complexity} complexity`}
            className={cn(
              "absolute flex size-7 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full font-mono text-[11px] font-semibold text-[#060606] transition-all duration-500 motion-reduce:transition-none",
              FIT_DOT[useCase.fit],
            )}
            style={{
              left: `${x}%`,
              top: mounted ? `${y}%` : "115%",
              opacity: mounted ? 1 : 0,
              transitionDelay: `${order * 90}ms`,
            }}
          >
            {index + 1}
          </span>
        ))}
      </div>

      <div className="mt-2 flex items-center justify-between font-mono text-[10px] uppercase tracking-[0.14em] text-white/50">
        <span>← Easier to build</span>
        <span>Harder to build →</span>
      </div>

      <ul className="mt-4 space-y-2">
        {placed.map(({ useCase, index }) => (
          <li key={useCase.title} className="flex items-center gap-2.5">
            <span
              className={cn(
                "flex size-5 shrink-0 items-center justify-center rounded-full font-mono text-[10px] font-semibold text-[#060606]",
                FIT_DOT[useCase.fit],
              )}
              aria-hidden
            >
              {index + 1}
            </span>
            <span className="min-w-0 truncate text-[13px] text-white/85">
              {useCase.title}
            </span>
            <span
              className={cn(
                "ml-auto shrink-0 font-mono text-[10px] uppercase tracking-[0.1em]",
                FIT_TEXT[useCase.fit],
              )}
            >
              {useCase.fit} fit
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
