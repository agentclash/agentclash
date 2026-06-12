"use client";

import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import type { OpportunityMetrics } from "../report-metrics";

type Axis = {
  key: keyof OpportunityMetrics;
  label: string;
};

// Order is positional: top, right, bottom, left.
const AXES: Axis[] = [
  { key: "workflowFit", label: "Workflow fit" },
  { key: "roiSignal", label: "ROI signal" },
  { key: "evalReadiness", label: "Eval ready" },
  { key: "riskProfile", label: "Risk safety" },
];

const CX = 185;
const CY = 140;
const RADIUS = 86;
const RINGS = [25, 50, 75, 100];

const CYAN = "#22d3ee";
const GRADIENT_ID = "radar-fill-" + Math.random().toString(36).slice(2);

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
  { anchor: "middle", dx: 0, dy: -18 },
  { anchor: "start", dx: 14, dy: 0 },
  { anchor: "middle", dx: 0, dy: 26 },
  { anchor: "end", dx: -14, dy: 0 },
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
        (axis, index) => `${axis.label} ${values[index]} of 100`,
      ).join(", ")}`}
      className={cn("block w-full", className)}
    >
      <defs>
        <radialGradient id={GRADIENT_ID} cx="50%" cy="50%" r="65%">
          <stop offset="0%" stopColor={CYAN} stopOpacity="0.32" />
          <stop offset="100%" stopColor={CYAN} stopOpacity="0.05" />
        </radialGradient>
      </defs>

      {RINGS.map((ring) => (
        <polygon
          key={ring}
          points={ringPath(ring)}
          fill="none"
          stroke="rgba(255,255,255,0.1)"
          strokeWidth={1}
        />
      ))}
      {AXES.map((axis, index) => {
        const [x, y] = pointAt(index, 100);
        return (
          <line
            key={axis.key}
            x1={CX}
            y1={CY}
            x2={x}
            y2={y}
            stroke="rgba(255,255,255,0.13)"
            strokeWidth={1}
          />
        );
      })}

      <g
        className="motion-reduce:transition-none"
        style={{
          transformOrigin: `${CX}px ${CY}px`,
          transform: mounted ? "scale(1)" : "scale(0.3)",
          opacity: mounted ? 1 : 0,
          transition:
            "transform 800ms cubic-bezier(0.22, 1, 0.36, 1), opacity 600ms ease",
        }}
      >
        <polygon
          points={polygon}
          fill={`url(#${GRADIENT_ID})`}
          stroke={CYAN}
          strokeWidth={2}
          strokeLinejoin="round"
          style={{ filter: `drop-shadow(0 0 8px ${CYAN}80)` }}
        />
        {values.map((value, index) => {
          const [x, y] = pointAt(index, value);
          return (
            <g key={AXES[index].key}>
              <circle cx={x} cy={y} r={5} fill={CYAN} opacity={0.25} />
              <circle cx={x} cy={y} r={3} fill={CYAN} />
              <circle cx={x} cy={y} r={1.4} fill="#fff" />
            </g>
          );
        })}
      </g>

      {AXES.map((axis, index) => {
        const [x, y] = pointAt(index, 100);
        const { anchor, dx, dy } = LABEL_ANCHORS[index];
        return (
          <g key={axis.key} textAnchor={anchor}>
            <text
              x={x + dx}
              y={y + dy}
              className="fill-white font-mono text-[15px] font-medium tabular-nums"
            >
              {values[index]}
            </text>
            <text
              x={x + dx}
              y={y + dy + 14}
              className="fill-white/60 font-mono text-[10px] uppercase"
              style={{ letterSpacing: "0.12em" }}
            >
              {axis.label}
            </text>
          </g>
        );
      })}
    </svg>
  );
}
