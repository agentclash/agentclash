import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CaseResult, Operation } from "@/lib/vibe";
import { CaseEvidence } from "./case-evidence";
import { VibeScorecard } from "./scorecard";

let container: HTMLDivElement;
let root: Root;
const result = (verdict: CaseResult["verdict"]): CaseResult => ({
  case_key: "exact-case",
  version: "artifact-one",
  verdict,
  input: null,
  output: "",
  checks: [{ key: "policy", verdict, evidence: "" }],
});
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});
async function open() {
  await act(async () => {
    const details = container.querySelector("details")!;
    details.open = true;
    details.dispatchEvent(new Event("toggle"));
  });
}

it.each(["verdict", "check metadata"])(
  "refreshes expanded exact-case evidence when persisted %s changes without polling loops",
  async (change) => {
    const initial = result(change === "verdict" ? "UNKNOWN" : "FAIL");
    const updated =
      change === "verdict"
        ? result("PASS")
        : {
            ...result("FAIL"),
            checks: [{ key: "policy", verdict: "PASS" as const, evidence: "" }],
          };
    const load = vi
      .fn()
      .mockResolvedValueOnce({
        ...initial,
        input: { question: "Refund?" },
        error: { message: "Still waiting on evaluator" },
      })
      .mockResolvedValue({
        ...updated,
        input: { question: "Refund?" },
        output: "Newly persisted answer",
        checks: [
          {
            key: "policy",
            verdict: "PASS",
            evidence: "Verified policy evidence",
          },
        ],
      });
    await act(async () =>
      root.render(<CaseEvidence summary={initial} load={load} />),
    );
    await open();
    expect(container.textContent).toContain("Still waiting on evaluator");
    await act(async () =>
      root.render(<CaseEvidence summary={updated} load={(key) => load(key)} />),
    );
    expect(container.textContent).toContain("Newly persisted answer");
    expect(container.textContent).toContain("Verified policy evidence");
    expect(container.textContent).not.toContain("Still waiting on evaluator");
    expect(container.querySelector("details")!.open).toBe(true);
    expect(load.mock.calls).toEqual([["exact-case"], ["exact-case"]]);
    for (let i = 0; i < 3; i++)
      await act(async () =>
        root.render(
          <CaseEvidence
            summary={structuredClone(updated)}
            load={(key) => load(key)}
          />,
        ),
      );
    expect(load).toHaveBeenCalledTimes(2);
  },
);

it.each(["RUNNING", "CANCELLED"])(
  "refreshes output-only recovery from %s without changing verdicts or starting work",
  async (state) => {
    const summary = result("UNKNOWN");
    const operation: Operation = {
      id: "check-one",
      kind: "check",
      state,
      billing: "RECONCILING",
      models: {
        assistant: "fixture/assistant",
        target: "fixture/target",
        evaluator: "fixture/evaluator",
      },
      max_cost_nano_usd: 0,
      actual_cost_nano_usd: null,
      results: [summary],
      scorecard: {
        passed: 0,
        failed: 0,
        unknown: 1,
        total: 1,
        evaluated: 0,
        pass_rate: null,
        coverage: 0,
      },
    };
    const recovered = {
      ...summary,
      input: { prompt: "Say hello" },
      output: "Journaled answer recovered after the worker stopped",
    };
    const load = vi
      .fn()
      .mockResolvedValueOnce(summary)
      .mockResolvedValue(recovered);
    const improve = vi.fn();
    const retest = vi.fn();
    const render = async (op: Operation, cursor: number) =>
      act(async () =>
        root.render(
          <VibeScorecard
            operation={op}
            eventCursor={cursor}
            loadEvidence={(key) => load(key)}
            onImprove={improve}
            onRetest={retest}
            busy={false}
          />,
        ),
      );
    await render(operation, 10);
    if (state === "RUNNING") {
      expect(load).not.toHaveBeenCalled();
      await open();
    } else {
      // Terminal results now expand one finding immediately, including unknown
      // outcomes after cancellation. This only reads the saved case.
      expect(container.querySelector("details")!.open).toBe(true);
    }
    expect(load).toHaveBeenCalledTimes(1);
    expect(container.textContent).not.toContain(recovered.output);

    const finished = {
      ...operation,
      state: state === "RUNNING" ? "PARTIAL" : state,
    };
    // Recovery can persist output after cancellation, without even a state change.
    // The session's persisted event cursor advances; the redacted case does not.
    await render(finished, 11);
    expect(finished.results).toEqual(operation.results);
    expect(container.textContent).toContain(recovered.output);
    expect(container.querySelector("details")!.open).toBe(true);
    expect(container.querySelector("summary")!.textContent).toContain(
      "Could not determine",
    );
    expect(load.mock.calls).toEqual([["exact-case"], ["exact-case"]]);
    for (let i = 0; i < 3; i++) await render(structuredClone(finished), 11);
    expect(load).toHaveBeenCalledTimes(2);
    expect(improve).not.toHaveBeenCalled();
    expect(retest).not.toHaveBeenCalled();
  },
);


it("opens the complete exact explanation beside its concise excerpt",async()=>{
 const explanation="The reply appears inconsistent with the policy. ".repeat(8)+"However, the quoted exception explicitly allows this customer's request.";
 const evidence={...result("FAIL"),checks:[{key:"policy",verdict:"FAIL" as const,evidence:explanation}]};
 await act(async()=>root.render(<CaseEvidence summary={evidence} concise load={async()=>evidence}/>));
 await open();
 const summary=[...container.querySelectorAll("summary")].find(node=>node.textContent==="Read complete explanation");
 expect(summary).toBeTruthy();
 const disclosure=summary!.parentElement as HTMLDetailsElement;
 await act(async()=>{disclosure.open=true;disclosure.dispatchEvent(new Event("toggle"))});
 expect(disclosure.textContent).toContain(explanation);
 expect(disclosure.parentElement?.textContent).toContain("excerpt");
});

it("shows a grounded finding and its expectation once, and offers changes only for expectations", async () => {
  const reply = "Yes, you can return it. Just bring the receipt.";
  const finding = { key: "behavior", verdict: "FAIL" as const, evidence: "The agent allowed a late return.", evidence_version: 1, finding: { kind: "observed" as const, quotes: [{ message_id: "output", text: "Yes, you can return it." }], missing: "", covered_message_ids: [] } };
  const evidence: CaseResult = { ...result("FAIL"), output: reply, expected: "Decline the late return.", checks: [finding] };
  const dispute = vi.fn();
  await act(async () => root.render(<CaseEvidence summary={evidence} load={async () => evidence} onDispute={dispute} />));
  await open();
  const count = (text: string) => container.textContent!.split(text).length - 1;
  expect(count(finding.evidence)).toBe(1);
  expect(count("Decline the late return.")).toBe(1);
  expect(container.textContent).not.toContain("Grading details");
  const labels = () => [...container.querySelectorAll("button")].map(b => b.textContent);
  expect(labels().filter(label => label === "Change expected result")).toHaveLength(1);
  const withAnswer = { ...evidence, expectations: [{ id: "receipt", statement: "Ask for the receipt." }], checks: [{ key: "has_answer", verdict: "FAIL" as const, evidence: "No answer check." }, { key: "receipt", verdict: "PASS" as const, evidence: "It asked for the receipt." }, finding] };
  await act(async () => root.render(<CaseEvidence summary={withAnswer} load={async () => withAnswer} onDispute={dispute} />));
  expect(container.textContent).toContain("Grading details · 3");
  // "Reply received" is a grading step, not a rule, so only the finding and the user's rule can change.
  expect(labels().filter(label => label === "Change expected result")).toHaveLength(2);
  expect([...container.querySelectorAll("button")].map(b => b.getAttribute("aria-label"))).toContain("Change expected result: Ask for the receipt.");
});

it.each(["PASS", "FAIL"] as const)("keeps verified %s evidence visible in every unified preview", async verdict => {
  const quote = verdict === "PASS" ? "I cannot process refunds." : "I processed your refund.";
  const rule = "Never claim to process refunds.";
  for (const recorded of [false, true]) for (const compact of [false, true]) {
    const reply = "Here is the policy. ".repeat(60) + quote;
    const evidence: CaseResult = {
      ...result(verdict), output: recorded ? "" : reply,
      messages: recorded ? [{ id: "u", role: "user", content: "Can I return this?" }, { id: "a", role: "assistant", content: reply }] : undefined,
      expectations: [{ id: "policy", statement: rule }],
      checks: [{ key: "policy", verdict, evidence: "The final sentence supports this grade.", evidence_version: 1, finding: { kind: "observed", quotes: [{ message_id: recorded ? "a" : "output", text: quote }], missing: "", covered_message_ids: [] } }],
    };
    const dispute = vi.fn(), regrade = vi.fn();
    await act(async () => root.render(<CaseEvidence key={`${recorded}:${compact}`} summary={evidence} load={async () => evidence} defaultOpen concise compactPreview={compact} origin={recorded ? "provided_conversations" : "prompt"} onDispute={dispute} onRegrade={regrade} />));
    const region = container.querySelector('section[aria-label="Evidence for this grade"]');
    expect(region).not.toBeNull();
    expect(region!.closest("[hidden]")).toBeNull();
    expect(region!.querySelector("blockquote")?.textContent).toBe(`Quoted from the reply${quote}`);
    expect(container.textContent!.split(rule)).toHaveLength(2);
    expect(container.querySelector(".vibe-example-reply > p")?.textContent).not.toContain(quote);
    expect([...container.querySelectorAll(".vibe-example-evidence > div > p")].map(p => p.textContent)).toEqual(["We gave it", "It replied", "It should"]);
    const buttons = [...container.querySelectorAll("button")];
    await act(async () => buttons.find(b => b.textContent === "Change expected result")!.click());
    expect(dispute).toHaveBeenCalledExactlyOnceWith(rule, evidence);
    expect(regrade).not.toHaveBeenCalled();
    await act(async () => buttons.find(b => b.textContent === "Grade the same replies again")!.click());
    expect(regrade).toHaveBeenCalledOnce();
    expect(dispute).toHaveBeenCalledOnce();
  }
});

it("keeps missing behavior visible without inventing a quote or duplicating the finding in grading details", async () => {
  const missing = "Ask for the receipt.";
  const evidence: CaseResult = { ...result("FAIL"), expected: missing, output: "I can help with returns.", checks: [
    { key: "policy", verdict: "FAIL", evidence: "The receipt was not requested.", evidence_version: 1, finding: { kind: "missing_behavior", quotes: [], missing, covered_message_ids: ["output"] } },
    { key: "has_answer", verdict: "PASS", evidence: "A reply was received." },
  ] };
  await act(async () => root.render(<CaseEvidence summary={evidence} load={async () => evidence} defaultOpen />));
  const regions = container.querySelectorAll('section[aria-label="Evidence for this grade"]');
  expect(regions).toHaveLength(1);
  expect(regions[0].closest("[hidden]")).toBeNull();
  expect(regions[0].textContent).toContain("Missing from the replies");
  expect(regions[0].textContent).toContain(missing);
  expect(regions[0].querySelector("blockquote")).toBeNull();
  expect(regions[0].textContent).toContain("The judge reviewed the complete replies");
});

it.each(["invented", "unverified"])("keeps %s findings out of the grounded evidence region", async kind => {
  const evidence: CaseResult = { ...result("FAIL"), output: "I cannot process refunds.", checks: [{ key: "policy", verdict: "FAIL", evidence: "An explanation was recorded.", evidence_version: kind === "unverified" ? undefined : 1, finding: { kind: "observed", quotes: [{ message_id: "output", text: kind === "invented" ? "I processed a refund." : "I cannot process refunds." }], missing: "", covered_message_ids: [] } }] };
  await act(async () => root.render(<CaseEvidence summary={evidence} load={async () => evidence} defaultOpen />));
  expect(container.querySelector('[aria-label="Evidence for this grade"]')).toBeNull();
  expect(container.textContent).toContain("An explanation was recorded.");
});
