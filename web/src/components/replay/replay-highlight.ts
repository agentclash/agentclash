import type { ReplayStep } from "@/lib/api/types";

// findHighlightIndex maps a deep-link target sequence to a step index.
//
// The replay builder uses a stacked model where wrapper steps like `run` and
// `scoring` open first and then extend their completed_sequence over every
// nested event — so a naive "first range that contains target" pick would
// always return the outermost wrapper. We instead prefer:
//
//   1. Exact match on started_sequence.
//   2. Among steps whose [started, completed] range contains the target, the
//      one with the narrowest range (innermost step). Ties broken by latest
//      started_sequence so deeper nesting still wins.
//   3. When nothing contains the target, the nearest earlier step — but only
//      if the target is within the currently-loaded window. Otherwise return
//      -1 and let the UI signal that the target is out of range, rather than
//      highlighting an unrelated early card.
export function findHighlightIndex(
  steps: ReplayStep[],
  target: number,
): number {
  if (!Number.isFinite(target) || steps.length === 0) return -1;

  let best = -1;
  let bestSpan = Number.POSITIVE_INFINITY;
  let fallback = -1;
  let maxSequence = Number.NEGATIVE_INFINITY;

  for (let i = 0; i < steps.length; i++) {
    const step = steps[i];
    if (step.started_sequence === target) return i;

    const endSeq = step.completed_sequence ?? step.started_sequence;
    if (endSeq > maxSequence) maxSequence = endSeq;

    const contains =
      step.started_sequence <= target && target <= endSeq;
    if (contains) {
      const span = endSeq - step.started_sequence;
      if (span < bestSpan) {
        bestSpan = span;
        best = i;
      } else if (span === bestSpan && step.started_sequence > steps[best].started_sequence) {
        // Same-span tie: prefer the later start — that's the deeper nested
        // step in the stacked wrapper model.
        best = i;
      }
    }

    if (step.started_sequence <= target) fallback = i;
  }

  if (best >= 0) return best;
  // Only fall back when the target is inside the loaded window. Beyond it,
  // we have no way to tell which later-loaded step owns the sequence, so
  // highlighting the last-seen early step would be actively misleading.
  if (target > maxSequence) return -1;
  return fallback;
}
