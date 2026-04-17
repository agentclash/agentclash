import { describe, expect, it } from "vitest";
import type { ReplayStep } from "@/lib/api/types";
import { findHighlightIndex } from "./replay-highlight";

function step(
  startedSequence: number,
  completedSequence?: number,
): ReplayStep {
  return {
    type: "agent_step",
    status: "completed",
    headline: "step",
    source: "native_engine",
    started_sequence: startedSequence,
    completed_sequence: completedSequence,
    occurred_at: "2026-04-18T00:00:00Z",
    event_count: 1,
    event_types: [],
  } as ReplayStep;
}

describe("findHighlightIndex", () => {
  it("returns an exact match on started_sequence", () => {
    const steps = [step(1, 3), step(4, 6), step(7, 9)];
    expect(findHighlightIndex(steps, 4)).toBe(1);
  });

  it("matches when the target falls inside a step's sequence range", () => {
    const steps = [step(1, 3), step(4, 10)];
    expect(findHighlightIndex(steps, 7)).toBe(1);
  });

  it("falls back to the nearest earlier step when no range contains the target", () => {
    const steps = [step(1), step(4), step(7)];
    expect(findHighlightIndex(steps, 6)).toBe(1);
  });

  it("returns -1 when the target precedes every step", () => {
    expect(findHighlightIndex([step(10), step(20)], 5)).toBe(-1);
  });

  it("returns -1 when target is not finite", () => {
    expect(findHighlightIndex([step(1)], Number.NaN)).toBe(-1);
  });
});
