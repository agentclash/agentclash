import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CaseResult, Operation } from "@/lib/vibe";
import { GroundedFinding, verifiedFinding } from "./grounded-finding";
import { VibeScorecard, comparableGrades } from "./scorecard";

let container: HTMLDivElement;
let root: Root;
const tail = "Actually, I processed your refund.";
function result(): CaseResult {
  return { case_key: "refund", version: "suite", input: { question: "Refund?" }, output: "Some context. ".repeat(100) + tail, expected: "Never claim to process refunds.", verdict: "FAIL", expected_checks: 1, checks: [{ key: "behavior", verdict: "FAIL", evidence: "The last sentence claims an action.", evidence_version: 1, finding: { kind: "observed", quotes: [{ message_id: "output", text: tail }], missing: "", covered_message_ids: [] } }] };
}
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); container = document.createElement("div"); document.body.append(container); root = createRoot(container); });
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.unstubAllGlobals(); });

it("shows the decisive tail quote with the expectation instead of the opening preview", async () => {
  const r = result();
  await act(async () => root.render(<GroundedFinding result={r} check={r.checks[0]} />));
  expect(container.querySelector("blockquote")?.textContent).toContain(tail);
  expect(container.textContent).toContain("Never claim to process refunds.");
  expect(container.textContent).not.toContain("Some context.");
});
it.each(["invented", "user", "old", "unknown", "malformed"])("fails closed for %s evidence", kind => {
  const r = result(), check = r.checks[0];
  if (kind === "invented") check.finding!.quotes[0].text = "No refund was processed.";
  if (kind === "user") r.messages = [{ id: "output", role: "user", content: tail }];
  if (kind === "old") delete check.evidence_version;
  if (kind === "unknown") check.verdict = "UNKNOWN";
  if (kind === "malformed") delete (check.finding as Partial<NonNullable<typeof check.finding>>).missing;
  expect(verifiedFinding(r, check)).toBeNull();
});
it("labels missing behavior as a judge conclusion without inventing a quotation", async () => {
  const r = result(); r.checks[0].finding = { kind: "missing_behavior", quotes: [], missing: "Ask whether the item is unopened.", covered_message_ids: ["output"] };
  await act(async () => root.render(<GroundedFinding result={r} check={r.checks[0]} />));
  expect(container.textContent).toContain("Missing from the replies");
  expect(container.querySelector("blockquote")).toBeNull();
  r.messages = [{ id: "output", role: "assistant", content: "Hello." }, { id: "a2", role: "assistant", content: "Thanks." }];
  expect(verifiedFinding(r, r.checks[0])).toBeNull();
});
function operation(): Operation {
  return { id: "new", baseline_id: "old", kind: "retest", state: "COMPLETED", billing: "SETTLED", max_cost_nano_usd: 0, actual_cost_nano_usd: 0, model_calls: 1, models: { assistant: "author", target: "target", evaluator: "judge" }, results: [result()], scorecard: { total: 1, passed: 0, failed: 1, unknown: 0, evaluated: 1 }, source: { kind: "prompt", artifact_id: "suite", label: "Instructions" }, grading: { version: 1, hash: "hash" } as Operation["grading"] } as Operation;
}
it("only compares matching verified grading contracts on the chosen baseline", () => {
  const next = operation(), old = { ...operation(), id: "old" };
  expect(comparableGrades(next, old)).toBe(true);
  expect(comparableGrades(next, { ...old, grading: undefined })).toBe(false);
  expect(comparableGrades(next, { ...old, id: "unrelated" })).toBe(false);
  expect(comparableGrades({ ...next, grading: { ...next.grading!, hash: "new-parser" } }, old)).toBe(false);
});
it("keeps grade rechecks separate from rule edits and makes no improvement claim", async () => {
  const next = operation(); next.source!.comparison = "regraded";
  const regrade = vi.fn(), dispute = vi.fn(), improve = vi.fn(), retest = vi.fn(), newCheck = vi.fn();
  await act(async () => root.render(<VibeScorecard operation={next} baseline={{ ...operation(), id: "old" }} loadEvidence={async () => result()} onImprove={improve} onRetest={retest} onDispute={dispute} onRegrade={regrade} onNewCheck={newCheck} busy={false} testJourney />));
  expect(container.querySelector("h1")?.textContent).toBe("Saved replies regraded");
  expect(container.textContent).toContain("Your agent was not called again.");
  expect(container.textContent).not.toContain("fixed ·");
  const button = [...container.querySelectorAll("button")].find(b => b.textContent === "Recheck saved grades")!;
  await act(async () => button.click());
  expect(regrade).toHaveBeenCalledOnce();
  const separate = [...container.querySelectorAll("button")].find(b => b.textContent === "Run as a separate check")!;
  await act(async () => separate.click());
  expect(newCheck).toHaveBeenCalledOnce(); expect(dispute).not.toHaveBeenCalled(); expect(retest).not.toHaveBeenCalled();
});
