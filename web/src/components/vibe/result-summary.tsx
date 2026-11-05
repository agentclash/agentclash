import { dollars, type Operation } from "@/lib/vibe";
import { resultSlotStatus } from "@/lib/vibe-result-evidence";

/** A decorative segment for each test; the adjacent sentence carries the counts. */
export function ResultSummary({ operation }: { operation: Operation }) {
  const score = operation.scorecard;
  if (!score) return null;
  const { slots, counts } = resultSlotStatus(operation.results, score.total);
  return <div className="vibe-result-summary">
    <div className="vibe-result-bar" aria-hidden="true">
      {slots.map((status, index) => <span key={operation.results[index]?.case_key || index} className="vibe-result-segment" data-status={status} />)}
    </div>
    <p className="vibe-result-counts">{counts.pass} passed · {counts.fail} failed{counts.unresolved > 0 ? ` · ${counts.unresolved} unresolved` : ""}{operation.actual_cost_nano_usd != null ? ` · ${dollars(operation.actual_cost_nano_usd)}` : ""}</p>
  </div>;
}
