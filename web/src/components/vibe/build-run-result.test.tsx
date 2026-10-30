import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";
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
async function render(operation = op, options: { current?: boolean; sample?: string; baseline?: Operation; first?: boolean } = {}) {
  const target = { ...artifact, sample: options.sample, sample_basis: options.sample ? { sample_basis: options.sample, rules: [{ id: "rule-1", statement: "Prize promises are spam." }] } : undefined };
  const session: Session = { id: "session", anonymous: true, revision: 1, document: { artifacts: [target], messages: [], requirements: [], models: defaultModels }, operations: [...options.baseline ? [options.baseline] : [], operation] };
  await act(async () => root.render(<><BuildRunResult session={session} operation={operation} artifact={target} current={options.current !== false} first={options.first !== false} busy={false} loadEvidence={load} onImprove={improve} onTry={trial} onDetails={details} />
    {options.current !== false && <BuildNextActions session={session} artifact={target} operation={operation} hasOutput={false} busy={false} primary={false} onTry={trial} onDetails={details} onGuide={vi.fn()} onSave={vi.fn()} loadEvidence={load} />}</>));
}
it("opens the leading passing evidence and opens full checks only on request", async () => {
  const second = { ...evidence, case_key: "second", input: null, output: "" };
  await render({ ...op, results: [...op.results, second], scorecard: { ...op.scorecard!, total: 2, evaluated: 2, passed: 2 } });
  expect(node.querySelectorAll(".vibe-result-row")).toHaveLength(2);
  expect(node.querySelector(".vibe-other-results > summary")?.textContent).toBe("Other results · 1");
  expect(node.querySelector(".vibe-other-results .vibe-verdict-word")?.textContent).toBe("Passed");
  expect(node.querySelector(".vibe-other-results svg.vibe-verdict-pass")).not.toBeNull();
  expect(node.querySelector(".vibe-other-results .vibe-result-row")?.hasAttribute("open")).toBe(false);
  expect(node.querySelectorAll(".vibe-build-next-actions")).toHaveLength(1);
  expect(node.querySelectorAll(".vibe-project-enquiry")).toHaveLength(0);
  expect(load).toHaveBeenCalledWith("run1", "email");
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent?.includes("See 2 results"))!.click());
  expect(details).toHaveBeenCalledTimes(1);
});
it("shows an actual example and explains it once, with a direct way to try the prototype", async () => {
  await render();
  expect(node.querySelector(".vibe-results-outcome")?.textContent).toBe("Your agent passed all 1 test");
  expect(load).toHaveBeenCalledWith("run1", "email");
  expect(node.querySelector(".vibe-result-row summary")?.textContent).toContain("Win a prize");
  expect(node.querySelector(".vibe-result-row")?.hasAttribute("open")).toBe(true);
  const row = node.querySelector(".vibe-result-row") as HTMLDetailsElement;
  await act(async () => { row.open = true; row.dispatchEvent(new Event("toggle")); });
  expect(node.textContent).toContain("We gave it");
  expect(node.textContent).toContain("Win a prize");
  expect(node.textContent).toContain("It replied");
  expect(node.textContent).toContain("Other inputs remain untested");
  expect(load).toHaveBeenCalledTimes(1);
  await render(); expect(load).toHaveBeenCalledTimes(1);
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent?.includes("Try your agent"))!.click());
  expect(trial).toHaveBeenCalledTimes(1); expect(improve).not.toHaveBeenCalled();
});
it("shows exact short excerpts and lets a reader expand long saved text", async () => {
  const longInput = "A very long customer request with many details. ".repeat(12);
  const longReply = "A careful but lengthy reply explaining the decision. ".repeat(12);
  load.mockResolvedValue({ ...evidence, input: longInput, output: longReply });
  await render();
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
  expect(node.textContent).not.toContain("Try your agent");
  expect(node.textContent).not.toContain("Keep and export");
});
it("keeps sample-policy provenance visible", async () => {
  await render(op, { sample: "email" });
  expect(node.textContent).toContain("Sample demonstration");
  expect(node.textContent).toContain("not your business policy");
  expect(node.querySelector(".vibe-scope-rules")?.textContent).toContain("Prize promises are spam.");
});
it("recovers missing evidence without making up a reply or offering a fix", async () => {
  load.mockRejectedValue(new Error("Evidence unavailable")); await render();
  const row = node.querySelector(".vibe-result-row") as HTMLDetailsElement;
  await act(async () => { row.open = true; row.dispatchEvent(new Event("toggle")); });
  expect(node.textContent).toContain("Evidence unavailable");
  expect(node.textContent).toContain("Reload saved evidence");
  expect(node.textContent).not.toContain("What it actually replied");
  expect(node.textContent).not.toContain("Suggest a fix");
});
it("offers one explicit fix for a failed result without submitting automatically", async () => {
  const failure: CaseResult = { ...evidence, verdict: "FAIL", output: "Not spam", checks: [{ key: "behavior", verdict: "FAIL", evidence: "It let a spam email through.", evidence_version: 1, finding: { kind: "observed", missing: "", quotes: [{ message_id: "output", text: "Not spam" }], covered_message_ids: [] } }] };
  load.mockResolvedValue(failure);
  await render({ ...op, results: [failure], scorecard: { ...op.scorecard!, passed: 0, failed: 1 } });
  expect(load).toHaveBeenCalledWith("run1", "email");
  const row = node.querySelector(".vibe-result-row") as HTMLDetailsElement;
  await act(async () => { row.open=true; row.dispatchEvent(new Event("toggle")); });
  expect(node.querySelector("[role=status]")?.textContent).toBe("Your agent passed 0 of 1 test");
  expect(node.textContent).toContain("It let a spam email through.");
  expect(improve).not.toHaveBeenCalled();
  expect(node.querySelectorAll(".vibe-button-primary")).toHaveLength(1);
  expect(node.querySelector(".vibe-button-primary svg")).not.toBeNull();
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent?.includes("Suggest a fix"))!.click());
  expect(improve).toHaveBeenCalledTimes(1);
  await act(async () => Array.from(node.querySelectorAll("button")).find(b => b.textContent === "Try your agent")!.click());
  expect(trial).toHaveBeenCalledTimes(1);
});
it("never treats unknowns or a stopped check as a complete success", async () => {
  await render({ ...op, scorecard: { ...op.scorecard!, passed: 0, unknown: 1 } });
  expect(node.textContent).toContain("could not be fully graded");
  expect(node.textContent).toContain("See what happened");
  expect(node.textContent).toContain("Try your agent");
  await render({ ...op, state: "CANCELLED" });
  expect(node.textContent).toContain("Stopped");
  expect(node.textContent).not.toContain("Your agent passed all 1 test");
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

it("opens leading evidence for a later current run too", async () => {
  await render(op, { first: false }); expect(node.querySelector(".vibe-result-row")?.hasAttribute("open")).toBe(true); expect(load).toHaveBeenCalledWith("run1", "email");
});

it("keeps the expected reply inside the expanded test before what needs attention", async () => {
  const failure = { ...evidence, verdict: "FAIL" as const, checks: [{ key: "behavior", verdict: "FAIL" as const, evidence: "It let spam through." }] };
  load.mockResolvedValue(failure);
  await render({ ...op, results: [failure], scorecard: { ...op.scorecard!, passed: 0, failed: 1 } });
  const row = node.querySelector(".vibe-result-row")!;
  expect(row.textContent).toContain("It should");
  const text = row.textContent!;
  const positions = ["We gave it", "It replied", "It should", "What needs attention"].map(label => text.indexOf(label));
  expect(positions.every(position => position >= 0)).toBe(true);
  expect(positions).toEqual([...positions].sort((a, b) => a - b));
  expect(node.querySelectorAll(".vibe-build-examples .vibe-evidence-label")).toHaveLength(3);
});

async function withBuildStyles(check: () => void) {
  const style = document.createElement("style"); style.textContent = readFileSync("src/components/vibe/build-output.css", "utf8"); document.head.append(style);
  try { await render(); check(); } finally { style.remove(); }
}

it("keeps evidence in a single stretching column at every width", async () => {
  await withBuildStyles(() => {
    const card = getComputedStyle(node.querySelector(".vibe-build-examples")!);
    expect(card.display).toBe("grid");
    expect(card.gridTemplateColumns).toBe("minmax(0, 1fr)");
  });
});

it("stretches the test row and its divider across the evidence card", async () => {
  await withBuildStyles(() => {
    expect(getComputedStyle(node.querySelector(".vibe-build-examples > .vibe-result-row")!).width).toBe("100%");
  });
});

it("uses exactly one scorecard outcome h2 for the result headline", async () => {
  await render();
  const headings = node.querySelectorAll(".vibe-build-result h2");
  expect(headings).toHaveLength(1);
  expect(headings[0].classList.contains("vibe-results-outcome")).toBe(true);
  expect(headings[0].textContent).toBe("Your agent passed all 1 test");
});

it("leads with the outcome and keeps scope below evidence without repeating the agent identity", async () => {
  await render();
  const card = node.querySelector(".vibe-build-result")!;
  expect(card.firstElementChild?.querySelector("h2")?.textContent).toBe("Your agent passed all 1 test");
  expect(card.querySelector(".vibe-build-task, .vibe-build-eyebrow")).toBeNull();
  const scope = Array.from(card.querySelectorAll("p")).find(p => p.textContent?.startsWith("Runs here using"))!;
  expect(scope).toBeDefined();
  expect(card.querySelector(".vibe-build-examples")!.compareDocumentPosition(scope) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});

it.each([0, 2, 3])("U12/U13 Build uses the shared outcome and fix label (%s passed)", async passed => {
  const results = Array.from({ length: 3 }, (_, index) => ({ ...evidence, case_key: `case-${index}`, verdict: index < passed ? "PASS" as const : "FAIL" as const }));
  await render({ ...op, results, scorecard: { ...op.scorecard!, total: 3, evaluated: 3, passed, failed: 3 - passed } });
  expect(node.querySelector(".vibe-results-outcome")?.textContent).toBe(passed === 3 ? "Your agent passed all 3 tests" : `Your agent passed ${passed} of 3 tests`);
  if (passed < 3) expect([...node.querySelectorAll("button")].find(button => button.textContent === "Suggest a fix")?.querySelector("svg")).toBeTruthy();
});
