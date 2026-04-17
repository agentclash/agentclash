import type { ReplayStep } from "@/lib/api/types";

// findHighlightIndex returns the first step index whose started/completed
// sequence range contains the target. Falls back to the step with the closest
// started_sequence <= target so the inspector-sheet "View in replay" link
// still lands somewhere sensible when the exact event isn't covered by a step
// window (e.g. grader verification events emitted outside the agent's
// stepping loop).
export function findHighlightIndex(
  steps: ReplayStep[],
  target: number,
): number {
  if (!Number.isFinite(target)) return -1;
  let fallback = -1;
  for (let i = 0; i < steps.length; i++) {
    const step = steps[i];
    if (step.started_sequence === target) return i;
    if (
      step.started_sequence < target &&
      step.completed_sequence != null &&
      step.completed_sequence >= target
    ) {
      return i;
    }
    if (step.started_sequence <= target) fallback = i;
  }
  return fallback;
}
