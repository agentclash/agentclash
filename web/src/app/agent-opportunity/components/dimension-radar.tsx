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

const CX = 170;
const CY = 138;
const RADIUS = 84;
const RINGS = [25, 50, 75, 100];

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
  { anchor: "start", dx: 14, dy: 0 },
  { anchor: "middle", dx: 0, dy: 24 },
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
      viewBox="0 0 340 276"
      role="img"
      aria-label={`Dimension profile: ${AXES.map(
        (axis, index) => `${axis.label} ${values[index]} of 100`,
      ).join(", ")}`}
      className={cn("block w-full", className)}
    >
      {RINGS.map((ring) => (
        <polygon
          key={ring}
          points={ringPath(ring)}
          fill="none"
          stroke="rgba(255,255,255,0.07)"
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
            stroke="rgba(255,255,255,0.1)"
            strokeWidth={1}
          />
        );
      })}

      <g
        className="motion-reduce:transition-none"
        style={{
          transformOrigin: `${CX}px ${CY}px`,
          transform: mounted ? "scale(1)" : "scale(0.35)",
          opacity: mounted ? 1 : 0,
          transition:
            "transform 700ms cubic-bezier(0.22, 1, 0.36, 1), opacity 500ms ease",
        }}
      >
        <polygon
          points={polygon}
          fill="rgba(255,255,255,0.07)"
          stroke="rgba(255,255,255,0.75)"
          strokeWidth={1.5}
          strokeLinejoin="round"
        />
        {values.map((value, index) => {
          const [x, y] = pointAt(index, value);
          return (
            <circle key={AXES[index].key} cx={x} cy={y} r={2.5} fill="#fff" />
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
              className="fill-white font-mono text-[13px] tabular-nums"
            >
              {values[index]}
            </text>
            <text
              x={x + dx}
              y={y + dy + 12}
              className="fill-white/35 font-mono text-[8.5px] uppercase"
              style={{ letterSpacing: "0.14em" }}
            >
              {axis.label}
            </text>
          </g>
        );
      })}
    </svg>
  );
}
