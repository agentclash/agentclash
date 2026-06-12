"use client";

import { cn } from "@/lib/utils";

function scoreColor(score: number) {
  if (score >= 75) return "rgb(52, 211, 153)";
  if (score >= 45) return "rgb(251, 191, 36)";
  return "rgb(248, 113, 113)";
}

export function FitScoreRing({
  score,
  size = 148,
  label = "Agent fit",
}: {
  score: number;
  size?: number;
  label?: string;
}) {
  const radius = (size - 16) / 2;
  const cx = size / 2;
  const cy = size / 2;
  const circumference = 2 * Math.PI * radius;
  const progress = Math.max(0, Math.min(100, score)) / 100;
  const stroke = scoreColor(score);

  return (
    <div
      className="relative inline-flex items-center justify-center"
      style={{ width: size, height: size }}
    >
      <svg width={size} height={size} aria-hidden>
        <circle
          cx={cx}
          cy={cy}
          r={radius}
          fill="none"
          stroke="rgba(255,255,255,0.06)"
          strokeWidth={10}
        />
        <circle
          cx={cx}
          cy={cy}
          r={radius}
          fill="none"
          stroke={stroke}
          strokeWidth={10}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={circumference * (1 - progress)}
          transform={`rotate(-90 ${cx} ${cy})`}
          style={{ filter: `drop-shadow(0 0 8px ${stroke})` }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span
          className={cn(
            "font-[family-name:var(--font-mono)] text-[34px] leading-none font-medium tabular-nums",
          )}
          style={{ color: stroke }}
        >
          {score}
        </span>
        <span className="mt-1 text-[9px] uppercase tracking-[0.18em] text-white/40">
          {label}
        </span>
      </div>
    </div>
  );
}
