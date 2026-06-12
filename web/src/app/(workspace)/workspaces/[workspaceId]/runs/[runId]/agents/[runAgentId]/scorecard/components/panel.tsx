"use client";

import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

/**
 * Custom panel surface for the scorecard UI.
 *
 * Intentionally not a shadcn Card — AgentClash's scorecard leans into a tighter,
 * instrument-panel aesthetic: pure-black ground, hairline 1px white-alpha borders,
 * subtle surface tint, sharper corners than the generic shadcn card.
 */
export function Panel({
  children,
  className,
  tone = "default",
}: {
  children: ReactNode;
  className?: string;
  tone?: "default" | "warn" | "danger" | "dim";
}) {
  const toneClass = {
    default: "border-white/[0.08] bg-white/[0.015]",
    warn: "border-amber-500/25 bg-amber-500/[0.04]",
    danger: "border-red-500/25 bg-red-500/[0.04]",
    dim: "border-white/[0.05] bg-transparent",
  }[tone];

  return (
    <div className={cn("border rounded-md", toneClass, className)}>
      {children}
    </div>
  );
}

export function PanelHeader({
  title,
  icon,
  trailing,
  className,
}: {
  title: string;
  icon?: ReactNode;
  trailing?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex items-center gap-2.5 px-4 h-11 border-b border-white/[0.06]",
        className,
      )}
    >
      {icon && <span className="text-white/40">{icon}</span>}
      <h2 className="text-2xs leading-none text-white/75 uppercase tracking-[0.22em] font-medium">
        {title}
      </h2>
      {trailing && <div className="ml-auto">{trailing}</div>}
    </div>
  );
}
