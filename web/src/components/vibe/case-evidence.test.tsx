import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CaseResult, Operation } from "@/lib/vibe";
import { CaseEvidence } from "./case-evidence";
import { VibeScorecard } from "./scorecard";

let container: HTMLDivElement;
let root: Root;
const result = (verdict: CaseResult["verdict"]): CaseResult => ({ case_key: "exact-case", version: "artifact-one", verdict, input: null, output: "", checks: [{ key: "policy", verdict, evidence: "" }] });
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); container = document.createElement("div"); document.body.append(container); root = createRoot(container); });
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.unstubAllGlobals(); });
async function open() { await act(async () => { const details = container.querySelector("details")!; details.open = true; details.dispatchEvent(new Event("toggle")); }); }

it.each(["verdict", "check metadata"])("refreshes expanded exact-case evidence when persisted %s changes without polling loops", async (change) => {
  const initial = result(change === "verdict" ? "UNKNOWN" : "FAIL");
  const updated = change === "verdict" ? result("PASS") : { ...result("FAIL"), checks: [{ key: "policy", verdict: "PASS" as const, evidence: "" }] };
  const load = vi.fn().mockResolvedValueOnce({ ...initial, input: { question: "Refund?" }, error: { message: "Still waiting on evaluator" } }).mockResolvedValue({ ...updated, input: { question: "Refund?" }, output: "Newly persisted answer", checks: [{ key: "policy", verdict: "PASS", evidence: "Verified policy evidence" }] });
  await act(async () => root.render(<CaseEvidence summary={initial} load={load} />));
  await open();
  expect(container.textContent).toContain("Still waiting on evaluator");
  await act(async () => root.render(<CaseEvidence summary={updated} load={(key) => load(key)} />));
  expect(container.textContent).toContain("Newly persisted answer");
  expect(container.textContent).toContain("Verified policy evidence");
  expect(container.textContent).not.toContain("Still waiting on evaluator");
  expect(container.querySelector("details")!.open).toBe(true);
  expect(load.mock.calls).toEqual([["exact-case"], ["exact-case"]]);
  for (let i = 0; i < 3; i++) await act(async () => root.render(<CaseEvidence summary={structuredClone(updated)} load={(key) => load(key)} />));
  expect(load).toHaveBeenCalledTimes(2);
});

it.each(["RUNNING", "CANCELLED"])("refreshes output-only recovery from %s without changing verdicts or starting work", async (state) => {
  const summary = result("UNKNOWN");
  const operation: Operation = {
    id: "check-one", kind: "check", state, billing: "RECONCILING",
    models: { assistant: "fixture/assistant", target: "fixture/target", evaluator: "fixture/evaluator" },
    max_cost_nano_usd: 0, actual_cost_nano_usd: null, results: [summary],
    scorecard: { passed: 0, failed: 0, unknown: 1, total: 1, evaluated: 0, pass_rate: null, coverage: 0 },
  };
  const recovered = { ...summary, input: { prompt: "Say hello" }, output: "Journaled answer recovered after the worker stopped" };
  const load = vi.fn().mockResolvedValueOnce(summary).mockResolvedValue(recovered);
  const improve = vi.fn();
  const retest = vi.fn();
  const render = async (op: Operation, cursor: number) => act(async () => root.render(
    <VibeScorecard operation={op} eventCursor={cursor} loadEvidence={(key) => load(key)} onImprove={improve} onRetest={retest} busy={false} />,
  ));
  await render(operation, 10);
  expect(load).not.toHaveBeenCalled();
  await open();
  expect(load).toHaveBeenCalledTimes(1);
  expect(container.textContent).not.toContain(recovered.output);

  const finished = { ...operation, state: state === "RUNNING" ? "PARTIAL" : state };
  // Recovery can persist output after cancellation, without even a state change.
  // The session's persisted event cursor advances; the redacted case does not.
  await render(finished, 11);
  expect(finished.results).toEqual(operation.results);
  expect(container.textContent).toContain(recovered.output);
  expect(container.querySelector("details")!.open).toBe(true);
  expect(container.querySelector("summary")!.textContent).toContain("UNKNOWN");
  expect(load.mock.calls).toEqual([["exact-case"], ["exact-case"]]);
  for (let i = 0; i < 3; i++) await render(structuredClone(finished), 11);
  expect(load).toHaveBeenCalledTimes(2);
  expect(improve).not.toHaveBeenCalled();
  expect(retest).not.toHaveBeenCalled();
});
