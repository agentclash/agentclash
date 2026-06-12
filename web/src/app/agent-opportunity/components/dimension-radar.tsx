"use client";

import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import type { OpportunityMetrics } from "../report-metrics";

type Axis = {
  key: keyof OpportunityMetrics;
  label: string;
};

const AXES: Axis[] = [
  { key: "workflowFit", label: "Workflow" },
  { key: "roiSignal", label: "ROI" },
  { key: "evalReadiness", label: "Eval" },
  { key: "riskProfile", label: "Risk" },
];

const CX = 185;
const CY = 140;
const RADIUS = 86;
const RINGS = [25, 50, 75, 100];

const ACCENT = "#fff";
const FILL = "rgba(255,255,255,0.06)";

function pointAt(axisIndex: number, value: number): [number, number] {
  const angle = (Math.PI / 2) * axisIndex - Math.PI / 2;
  const r = (RADIUS * Math.max(0, Math.min(100, value))) / 100;
  return [CX + r * Math.cos(angle), CY + r * Math.sin(angle)];
}

function ringPath(value: number): string {
  return AXES.map((_, index) => pointAt(index, value).join(",")).join(" ");
}

const LABEL_ANCHORS: {
  anchor: "start" | "middle" | "end";
  dx: number;
  dy: number;
}[] = [
  { anchor: "middle", dx: 0, dy: -16 },
  { anchor: "start", dx: 12, dy: 0 },
  { anchor: "middle", dx: 0, dy: 22 },
  { anchor: "end", dx: -12, dy: 0 },
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

  const values = AXES.map((axis) => metrics[axis.key]);
  const polygon = values
    .map((value, index) => pointAt(index, value).join(","))
    .join(" ");

  return (
    <svg
      viewBox="0 0 370 286"
      role="img"
      aria-label={`Dimension profile: ${AXES.map(
        (axis, index) => `${axis.label} ${values[index]}`,
      ).join(", ")}`}
      className={cn("block w-full", className)}
    >
      {/* Grid rings */}
      {RINGS.map((ring) => (
        <polygon
          key={ring}
          points={ringPath(ring)}
          fill="none"
          stroke="rgba(255,255,255,0.08)"
          strokeWidth={1}
        />
      ))}

      {/* Axis lines */}
      {AXES.map((axis, index) => {
        const [x, y] = pointAt(index, 100);
        return (
          <line
            key={axis.key}
            x1={CX}
            y1={CY}
            x2={x}
            y2={y}
            stroke="rgba(255,255,255,0.08)"
            strokeWidth={1}
          />
        );
      })}

      {/* Data polygon */}
      <g
        className="motion-reduce:transition-none"
        style={{
          transformOrigin: `${CX}px ${CY}px`,
          transform: mounted ? "scale(1)" : "scale(0.85)",
          opacity: mounted ? 1 : 0,
          transition: "transform 500ms ease-out, opacity 400ms ease",
        }}
      >
        <polygon
          points={polygon}
          fill={FILL}
          stroke={ACCENT}
          strokeWidth={1.5}
          strokeLinejoin="round"
        />
        {values.map((value, index) => {
          const [x, y] = pointAt(index, value);
          return (
            <g key={AXES[index].key}>
              <circle cx={x} cy={y} r={4} fill="#080808" stroke={ACCENT} strokeWidth={1.5} />
              <circle cx={x} cy={y} r={1.5} fill={ACCENT} />
            </g>
          );
        })}
      </g>

      {/* Labels */}
      {AXES.map((axis, index) => {
        const [x, y] = pointAt(index, 100);
        const { anchor, dx, dy } = LABEL_ANCHORS[index];
        return (
          <g key={axis.key} textAnchor={anchor}>
            <text
              x={x + dx}
              y={y + dy}
              className="fill-white/90 font-mono text-[13px] font-medium tabular-nums"
            >
              {values[index]}
            </text>
            <text
              x={x + dx}
              y={y + dy + 13}
              className="fill-white/40 font-mono text-[9px] uppercase tracking-[0.14em]"
            >
              {axis.label}
            </text>
          </g>
        );
      })}
    </svg>
  );
}
