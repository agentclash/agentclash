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

  it("prefers the innermost wrapper when nested steps overlap", () => {
    // Replay builder stacks wrappers: a wide `run` wraps a narrower
    // `agent_step`, which wraps a `tool_call`. The target sits inside all
    // three; we want the innermost tool_call, not the outer run wrapper.
    const steps = [
      step(1, 20), // run
      step(3, 15), // agent_step
      step(7, 9), // tool_call
    ];
    expect(findHighlightIndex(steps, 8)).toBe(2);
  });

  it("falls back to the nearest earlier step when no range contains the target but it is within the loaded window", () => {
    const steps = [step(1), step(4), step(7, 8)];
    expect(findHighlightIndex(steps, 6)).toBe(1);
  });

  it("returns -1 when the target precedes every step", () => {
    expect(findHighlightIndex([step(10), step(20)], 5)).toBe(-1);
  });

  it("returns -1 when the target is beyond the loaded window", () => {
    // Replay pagination loads a 50-step window at a time. A deep link to
    // sequence 300 against a window ending at 50 should NOT highlight the
    // last loaded step — that would be a misleading fallback.
    const steps = [step(1, 10), step(11, 25), step(26, 50)];
    expect(findHighlightIndex(steps, 300)).toBe(-1);
  });

  it("returns -1 when target is not finite", () => {
    expect(findHighlightIndex([step(1)], Number.NaN)).toBe(-1);
  });
});
