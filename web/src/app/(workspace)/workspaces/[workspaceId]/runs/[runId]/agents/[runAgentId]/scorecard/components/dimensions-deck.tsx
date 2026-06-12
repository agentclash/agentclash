"use client";

import type { ScorecardResponse } from "@/lib/api/types";
import { Panel, PanelHeader } from "./panel";
import { Gauge, ArrowUp, ArrowDown } from "lucide-react";
import { cn } from "@/lib/utils";
import { barColor, scoreColor } from "@/lib/scores";
import { humanizeKey, sortDimensionKeys } from "./utils";

/**
 * Dimension cards rendered in a responsive grid.
 *
 * Each card shows:
 *   - dimension key (display-serif, humanised)
 *   - score (big, mono, coloured)
 *   - direction (↑/↓ with label)
 *   - state badge when not available
 *   - progress bar
 *   - reason, wrapped
 *
 * This replaces the flat stacked-bar list with something that reads as a deck
 * of measurements rather than a table — fewer rows to scan, richer per-unit detail.
 */
export function DimensionsDeck({
  scorecard,
}: {
  scorecard: ScorecardResponse;
}) {
  const doc = scorecard.scorecard;
  if (!doc?.dimensions) return null;
  const keys = sortDimensionKeys(Object.keys(doc.dimensions));
  if (keys.length === 0) return null;

  return (
    <Panel>
      <PanelHeader
        title="Dimensions"
        icon={<Gauge className="size-3.5" />}
        trailing={
          <span className="text-2xs uppercase tracking-[0.14em] text-white/40">
            {keys.length} {keys.length === 1 ? "axis" : "axes"}
          </span>
        }
      />
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-px bg-white/[0.05]">
        {keys.map((key) => {
          const dim = doc.dimensions[key];
          return (
            <DimensionCard key={key} dimKey={key} dim={dim} />
          );
        })}
      </div>
    </Panel>
  );
}

function DimensionCard({
  dimKey,
  dim,
}: {
  dimKey: string;
  dim: ScorecardResponse["scorecard"]["dimensions"][string];
}) {
  const isAvailable = dim.state === "available";
  const direction = dim.better_direction;

  return (
    <div className="bg-[#060606] p-4 flex flex-col gap-3 min-h-[112px]">
      <div className="flex items-start justify-between gap-2">
        <div className="flex flex-col gap-1 min-w-0">
          <h3 className="text-sm leading-none text-white/90 truncate font-medium tracking-[-0.005em]">
            {humanizeKey(dimKey)}
          </h3>
          {direction && (
            <span className="flex items-center gap-1 text-2xs uppercase tracking-[0.14em] text-white/35">
              {direction === "higher" ? (
                <ArrowUp className="size-2.5" />
              ) : (
                <ArrowDown className="size-2.5" />
              )}
              {direction} is better
            </span>
          )}
        </div>
        {!isAvailable && (
          <span
            className={cn(
              "text-2xs uppercase tracking-[0.14em] px-1.5 py-0.5 rounded border",
              dim.state === "error"
                ? "text-red-300 border-red-500/30 bg-red-500/[0.06]"
                : "text-white/50 border-white/10 bg-white/[0.03]",
            )}
          >
            {dim.state === "unavailable" ? "n/a" : dim.state}
          </span>
        )}
      </div>

      <div className="flex items-baseline gap-2">
        <span
          className={cn(
            "font-[family-name:var(--font-mono)] text-[28px] leading-none tabular-nums",
            dim.score == null ? "text-white/25" : scoreColor(dim.score),
          )}
        >
          {dim.score == null ? "—" : (dim.score * 100).toFixed(1)}
        </span>
        <span className="text-2xs text-white/30 font-[family-name:var(--font-mono)]">
          {dim.score == null ? "" : "/100"}
        </span>
      </div>

      <div className="h-[3px] rounded-full bg-white/[0.05] overflow-hidden">
        {dim.score != null && (
          <div
            className={cn("h-full rounded-full transition-all", barColor(dim.score))}
            style={{ width: `${(dim.score * 100).toFixed(1)}%` }}
          />
        )}
      </div>

      {dim.reason && (
        <p className="text-2xs text-white/45 leading-relaxed line-clamp-3">
          {dim.reason}
        </p>
      )}
    </div>
  );
}
