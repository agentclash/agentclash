"use client";

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

/**
 * Small, state-aware dot + hover tooltip explaining what each state means.
 *
 * Single source of truth for the state vocabulary. Previously the UI leaned
 * on five distinct state/verdict strings (`available`, `unavailable`, `error`,
 * `pass`, `fail`, `n/a`) without ever telling the user what they meant.
 */

type StateKind =
  | "pass"
  | "fail"
  | "error"
  | "unavailable"
  | "available"
  | "pending";

const COPY: Record<StateKind, { label: string; body: string }> = {
  pass: {
    label: "Pass",
    body: "The validator ran and met its expectation.",
  },
  fail: {
    label: "Fail",
    body: "The validator ran but did not meet its expectation.",
  },
  error: {
    label: "Error",
    body: "The validator or collector threw while running — the signal is unavailable, not a fail.",
  },
  unavailable: {
    label: "Unavailable",
    body: "No signal was produced — either the required evidence was missing or the collector was skipped.",
  },
  available: {
    label: "Available",
    body: "The metric or collector produced a value.",
  },
  pending: {
    label: "Pending",
    body: "Scoring hasn't finished yet.",
  },
};

const DOT_CLASS: Record<StateKind, string> = {
  pass: "bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.45)]",
  fail: "bg-red-400 shadow-[0_0_6px_rgba(248,113,113,0.45)]",
  error: "bg-amber-400 shadow-[0_0_6px_rgba(251,191,36,0.45)]",
  unavailable: "bg-white/25",
  available: "bg-emerald-400/60",
  pending: "bg-white/30 animate-pulse",
};

export function StateDot({ state, size = 6 }: { state: StateKind; size?: number }) {
  const copy = COPY[state];
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span
            className={`inline-block rounded-full ${DOT_CLASS[state]}`}
            style={{ width: size, height: size }}
            aria-label={copy.label}
          />
        }
      />
      <TooltipContent
        side="top"
        className="max-w-[240px] bg-[#0b0b0b] border border-white/10 text-white/85 px-3 py-2"
      >
        <div className="font-medium text-2xs">{copy.label}</div>
        <div className="text-2xs text-white/55 mt-0.5">{copy.body}</div>
      </TooltipContent>
    </Tooltip>
  );
}

export function normalizeState(verdict: string, state: string): StateKind {
  if (state === "error") return "error";
  if (state === "unavailable") return "unavailable";
  if (verdict === "pass") return "pass";
  if (verdict === "fail") return "fail";
  if (state === "available") return "available";
  return "unavailable";
}
