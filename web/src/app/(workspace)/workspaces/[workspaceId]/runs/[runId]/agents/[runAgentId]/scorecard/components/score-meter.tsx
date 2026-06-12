"use client";

import { cn } from "@/lib/utils";

type DimensionSegment = {
  key: string;
  score?: number;
  weight?: number;
};

/**
 * Distinctive circular score meter for the hero.
 *
 * Draws two concentric rings:
 *  - Outer ring: segmented, one arc per dimension. Each segment's length maps to
 *    the dimension's weight (or equal share when weights are unknown). Each
 *    segment's fill opacity maps to its score — so a glance reveals both the
 *    *size* and *health* of each contributor.
 *  - Inner ring: the overall score, thin, glowing.
 * The center shows the overall percentage in mono.
 *
 * Falls back to a single ring when there are no dimensions.
 */
export function ScoreMeter({
  overall,
  dimensions,
  size = 160,
}: {
  overall?: number;
  dimensions?: DimensionSegment[];
  size?: number;
}) {
  const outerR = (size - 14) / 2;
  const innerR = outerR - 14;
  const cx = size / 2;
  const cy = size / 2;

  // Gap (radians) between segments — kept small so the ring still reads as a ring.
  const gap = 0.04;

  const totalWeight = (dimensions ?? []).reduce(
    (acc, d) => acc + (d.weight ?? 1),
    0,
  );
  const segments = (dimensions ?? []).filter((d) => d.score != null);
  const segCount = segments.length;

  // Compute each segment's arc span in radians, then cumulative start angles.
  // Start at 12 o'clock, go clockwise. Uses reduce to build immutably — the
  // react-hooks/immutability rule flags let-reassignment in render bodies.
  const totalGap = gap * segCount;
  const availableArc = 2 * Math.PI - totalGap;
  const arcs = segments.reduce<
    Array<{ seg: DimensionSegment; start: number; end: number }>
  >((acc, seg) => {
    const prev = acc[acc.length - 1];
    const start = prev ? prev.end + gap : -Math.PI / 2;
    const share =
      segCount > 0 ? (seg.weight ?? 1) / (totalWeight || segCount) : 0;
    const span = availableArc * share;
    acc.push({ seg, start, end: start + span });
    return acc;
  }, []);

  const color = (score?: number) => {
    if (score == null) return "rgba(255,255,255,0.12)";
    if (score >= 0.8) return "rgb(52, 211, 153)";
    if (score >= 0.5) return "rgb(251, 191, 36)";
    return "rgb(248, 113, 113)";
  };

  const innerColor = color(overall);

  const overallPct = overall == null ? "—" : Math.round(overall * 100);

  return (
    <div
      className="relative inline-flex items-center justify-center"
      style={{ width: size, height: size }}
    >
      <svg width={size} height={size}>
        {/* Outer ring background */}
        <circle
          cx={cx}
          cy={cy}
          r={outerR}
          fill="none"
          stroke="rgba(255,255,255,0.06)"
          strokeWidth={8}
        />
        {/* Outer ring segments */}
        {arcs.map(({ seg, start, end }) => (
          <path
            key={seg.key}
            d={arcPath(cx, cy, outerR, start, end)}
            fill="none"
            stroke={color(seg.score)}
            strokeWidth={8}
            strokeLinecap="butt"
            opacity={0.35 + 0.65 * (seg.score ?? 0)}
          />
        ))}
        {/* Inner ring track */}
        <circle
          cx={cx}
          cy={cy}
          r={innerR}
          fill="none"
          stroke="rgba(255,255,255,0.05)"
          strokeWidth={2}
        />
        {/* Inner ring overall */}
        {overall != null && (
          <circle
            cx={cx}
            cy={cy}
            r={innerR}
            fill="none"
            stroke={innerColor}
            strokeWidth={2}
            strokeDasharray={2 * Math.PI * innerR}
            strokeDashoffset={2 * Math.PI * innerR * (1 - overall)}
            strokeLinecap="round"
            transform={`rotate(-90 ${cx} ${cy})`}
            style={{ filter: `drop-shadow(0 0 6px ${innerColor})` }}
          />
        )}
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span
          className={cn(
            "font-[family-name:var(--font-mono)] text-[32px] leading-none font-medium tabular-nums",
          )}
          style={{ color: innerColor }}
        >
          {overallPct}
        </span>
        <span className="mt-0.5 text-2xs uppercase tracking-[0.18em] text-white/40">
          Overall
        </span>
      </div>
    </div>
  );
}

function arcPath(
  cx: number,
  cy: number,
  r: number,
  startAngle: number,
  endAngle: number,
): string {
  const sx = cx + r * Math.cos(startAngle);
  const sy = cy + r * Math.sin(startAngle);
  const ex = cx + r * Math.cos(endAngle);
  const ey = cy + r * Math.sin(endAngle);
  const largeArc = endAngle - startAngle > Math.PI ? 1 : 0;
  return `M ${sx} ${sy} A ${r} ${r} 0 ${largeArc} 1 ${ex} ${ey}`;
}
