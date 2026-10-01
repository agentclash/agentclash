import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, type Artifact, type CaseResult, type Operation, type Session } from "@/lib/vibe";
import { BuildRunResult } from "./build-run-result";
import { BuildNextActions } from "./build-next-actions";

const artifact: Artifact = { id: "v1", title: "Sorter", agent_prompt: "Classify email", blueprint: { cases: [{ key: "email", payload: { question: "Win a prize" } }] }, accepted: true, source_message_id: "brief" };
const evidence: CaseResult = { version: "v1", case_key: "email", input: { question: "Win a prize" }, expected: "Mark as spam", output: "Spam", verdict: "PASS", checks: [{ key: "behavior", verdict: "PASS", evidence: "It classified the sample correctly." }] };
const op: Operation = { id: "run1", kind: "check", state: "COMPLETED", models: defaultModels, billing: "SETTLED", max_cost_nano_usd: 0, actual_cost_nano_usd: 0, source: { kind: "prompt", artifact_id: "v1", label: "Prototype" }, results: [{ ...evidence, input: null, output: "" }], scorecard: { total: 1, passed: 1, failed: 0, unknown: 0, evaluated: 1, coverage: 1, pass_rate: 1 } };
let root: Root, node: HTMLDivElement;
const load = vi.fn(), improve = vi.fn(), trial = vi.fn(), details = vi.fn();
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); node = document.createElement("div"); document.body.append(node); root = createRoot(node); vi.clearAllMocks(); load.mockResolvedValue(evidence); });
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); });
async function render(operation = op, options: { current?: boolean; sample?: string; baseline?: Operation } = {}) {
  const target = { ...artifact, sample: options.sample, sample_basis: options.sample ? { sample_basis: options.sample, rules: [{ id: "rule-1", statement: "Prize promises are spam." }] } : undefined };
  const session: Session = { id: "session", anonymous: true, revision: 1, document: { artifacts: [target], messages: [], requirements: [], models: defaultModels }, operations: [...options.baseline ? [options.baseline] : [], operation] };
  await act(async () => root.render(<><BuildRunResult session={session} operation={operation} artifact={target} current={options.current !== false} first busy={false} loadEvidence={load} onImprove={improve} onDetails={details} />
    {options.current !== false && <BuildNextActions session={session} artifact={target} operation={operation} hasOutput={false} busy={false} primary onTry={trial} onDetails={details} onGuide={vi.fn()} onSave={vi.fn()} loadEvidence={load} />}</>));
}
it("keeps passing evidence closed and opens the full checks only on request", async () => {
  const second = { ...evidence, case_key: "second", input: null, output: "" };
  await render({ ...op, results: [...op.results, second], scorecard: { ...op.scorecard!, total: 2, evaluated: 2, passed: 2 } });
  expect(node.querySelectorAll(".vibe-result-row")).toHaveLength(0);
  expect(node.querySelectorAll(".vibe-build-next-actions")).toHaveLength(1);
  expect(node.querySelectorAll(".vibe-project-enquiry")).toHaveLength(1);
  expect(load).not.toHaveBeenCalled();
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent?.includes("See 2 checks"))!.click());
  expect(details).toHaveBeenCalledTimes(1);
});
async function openSample() {
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent === "Try a sample")!.click());
}
it("shows an actual example and explains it once, with a direct way to try the prototype", async () => {
  await render();
  expect(node.textContent).toContain("This check matched your rules");
  expect(load).not.toHaveBeenCalled();
  await openSample();
  expect(load).toHaveBeenCalledWith("run1", "email");
  expect(node.querySelector(".vibe-result-row summary")?.textContent).toContain("Win a prize");
  expect(node.querySelector(".vibe-result-row")?.hasAttribute("open")).toBe(true);
  const row = node.querySelector(".vibe-result-row") as HTMLDetailsElement;
  await act(async () => { row.open = true; row.dispatchEvent(new Event("toggle")); });
  expect(node.textContent).toContain("We gave it");
  expect(node.textContent).toContain("Win a prize");
  expect(node.textContent).toContain("It replied");
  expect(node.textContent).toContain("Other situations remain untested");
  expect(load).toHaveBeenCalledTimes(1);
  await render(); expect(load).toHaveBeenCalledTimes(1);
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent?.includes("Try it yourself"))!.click());
  expect(trial).toHaveBeenCalledTimes(1); expect(improve).not.toHaveBeenCalled();
});
it("shows exact short excerpts and lets a reader expand long saved text", async () => {
  const longInput = "A very long customer request with many details. ".repeat(12);
  const longReply = "A careful but lengthy reply explaining the decision. ".repeat(12);
  load.mockResolvedValue({ ...evidence, input: longInput, output: longReply });
  await render();
  await openSample();
  expect(node.querySelectorAll(".vibe-evidence-excerpt")).toHaveLength(2);
  expect(node.textContent).toContain("Show full input");
  expect(node.textContent).toContain("Show full reply");
  expect(node.querySelector(".vibe-evidence-excerpt")?.textContent).not.toContain(longInput);
  const full = node.querySelector(".vibe-evidence-full") as HTMLDetailsElement;
  await act(async () => { full.open = true; full.dispatchEvent(new Event("toggle")); });
  expect(full.textContent).toContain(longInput.trim());
});
it("does not load historical evidence or offer mutations merely by rendering an old run", async () => {
  await render(op, { current: false });
  expect(load).not.toHaveBeenCalled();
  expect(node.textContent).not.toContain("Try it yourself");
  expect(node.textContent).not.toContain("Keep and export");
});
it("keeps sample-policy provenance visible", async () => {
  await render(op, { sample: "email" });
  expect(node.textContent).toContain("Sample demonstration");
  expect(node.textContent).toContain("not your business policy");
  expect(node.querySelector(".vibe-scope-rules")?.textContent).toContain("Prize promises are spam.");
});
it("recovers missing evidence without making up a reply or offering a fix", async () => {
  load.mockRejectedValue(new Error("Evidence unavailable")); await render(); await openSample();
  const row = node.querySelector(".vibe-result-row") as HTMLDetailsElement;
  await act(async () => { row.open = true; row.dispatchEvent(new Event("toggle")); });
  expect(node.textContent).toContain("Evidence unavailable");
  expect(node.textContent).toContain("Reload saved evidence");
  expect(node.textContent).not.toContain("What it actually replied");
  expect(node.textContent).not.toContain("Review a fix");
});
it("offers improvement only after a supported failure and does not submit automatically", async () => {
  const failure: CaseResult = { ...evidence, verdict: "FAIL", output: "Not spam", checks: [{ key: "behavior", verdict: "FAIL", evidence: "It let a spam email through.", evidence_version: 1, finding: { kind: "observed", missing: "", quotes: [{ message_id: "output", text: "Not spam" }], covered_message_ids: [] } }] };
  load.mockResolvedValue(failure);
  await render({ ...op, results: [failure], scorecard: { ...op.scorecard!, passed: 0, failed: 1 } });
  expect(load).not.toHaveBeenCalled();
  const row = node.querySelector(".vibe-result-row") as HTMLDetailsElement;
  await act(async () => { row.open=true; row.dispatchEvent(new Event("toggle")); });
  expect(node.querySelector("[role=status]")?.textContent).toBe("It let a spam email through.");
  expect(improve).not.toHaveBeenCalled();
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent?.includes("Review a fix"))!.click());
  expect(improve).toHaveBeenCalledTimes(1);
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent === "Try it yourself")!.click());
  expect(trial).toHaveBeenCalledTimes(1);
});
it("never treats unknowns or a stopped check as a complete success", async () => {
  await render({ ...op, scorecard: { ...op.scorecard!, passed: 0, unknown: 1 } });
  expect(node.textContent).toContain("could not be fully checked");
  expect(node.textContent).toContain("See what happened");
  expect(node.textContent).toContain("Try it yourself");
  await render({ ...op, state: "CANCELLED" });
  expect(node.textContent).toContain("Stopped");
  expect(node.textContent).not.toContain("This check matched your rules");
  expect(node.querySelector(".vibe-build-next-actions")?.hasAttribute("hidden")).toBe(true);
});
it("reports improvements and regressions only against matching tests and grading", async () => {
  const grading: NonNullable<Operation["grading"]> = { version: 1, hash: "same-tests-and-grader", criteria_hash: "criteria", tests_hash: "tests", parser: "v1", normalization: "v1", schema: "v1", prompt_hash: "judge", aggregation: "v1", evaluator: { provider: "fixture", model: "judge", route: "fixture", temperature: 0, max_output: 1000, disable_reasoning: true } };
  const baseline: Operation = { ...op, id: "baseline", grading, results: [
    { ...evidence, case_key: "spam", verdict: "FAIL" }, { ...evidence, case_key: "legitimate", verdict: "PASS" },
  ] };
  const comparison: Operation = { ...op, id: "comparison", baseline_id: "baseline", grading, results: [
    { ...evidence, case_key: "spam", verdict: "PASS" }, { ...evidence, case_key: "legitimate", verdict: "FAIL" },
  ], scorecard: { ...op.scorecard!, total: 2, evaluated: 2, passed: 1, failed: 1 } };
  await render(comparison, { baseline, current: false });
  expect(node.textContent).toContain("1 improved · 1 regressed");
  await render({ ...comparison, grading: { ...grading, version: 2 } }, { baseline: { ...baseline, grading: { ...grading, version: 2 } }, current: false });
  expect(node.textContent).toContain("1 improved · 1 regressed");
  await render({ ...comparison, grading: { ...grading, version: 2 } }, { baseline, current: false });
  expect(node.textContent).toContain("Separate baseline");
  await render({ ...comparison, grading: { ...grading, hash: "changed-tests" } }, { baseline, current: false });
  expect(node.textContent).toContain("Separate baseline");
  expect(node.textContent).not.toContain("1 improved");
  await render({ ...comparison, source: { ...comparison.source!, comparison: "regraded" } }, { baseline, current: false });
  expect(node.textContent).toContain("Only the grades changed");
  expect(node.textContent).not.toContain("1 improved");
});
