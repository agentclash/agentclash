import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CaseResult, Operation } from "@/lib/vibe";
import { ResultRecoveryControls } from "./result-recovery";
import { persistDrafts, emptyDrafts, editorDraftKey, readEditorDraft, discardDrafts } from "@/lib/vibe-build-drafts";
import { GradeReviews } from "./grade-reviews";
import { ResultComparison } from "./result-comparison";
import { compareResultEvidence } from "@/lib/vibe-result-evidence";
import { VibeScorecard } from "./scorecard";
import { CaseEvidence } from "./case-evidence";

let container: HTMLDivElement;
let root: Root;
const clipboardDescriptor = Object.getOwnPropertyDescriptor(
  navigator,
  "clipboard",
);
const writeText = vi.fn();
const nextAnswer = vi.fn();
const retest = vi.fn();
const improve = vi.fn();
const dispute = vi.fn();
const save = vi.fn();
const failure: CaseResult = {
  case_key: "memory",
  title: "The purchase age was asked twice",
  version: "rules-one",
  input: null,
  output: "",
  verdict: "FAIL",
  expectations: [
    { id: "remember", statement: "Ask only for missing details." },
  ],
  messages: [
    { id: "u1", role: "user", content: "I bought it 10 days ago." },
    { id: "a1", role: "assistant", content: "Is it unopened?" },
    { id: "u2", role: "user", content: "It is unopened." },
    {
      id: "a2",
      role: "assistant",
      content: "How many days ago did you buy it?",
    },
  ],
  checks: [
    {
      key: "remember",
      verdict: "FAIL",
      evidence: "The agent asked for the purchase age already supplied.",
      message_ids: ["u1", "a2"],
    },
  ],
};
const passing: CaseResult = {
  ...failure,
  case_key: "boundary",
  title: "The return window was respected",
  verdict: "PASS",
  messages: [
    {
      id: "u1",
      role: "user",
      content: "I bought this unopened item 45 days ago.",
    },
    {
      id: "a1",
      role: "assistant",
      content: "It is outside the 30-day return window.",
    },
  ],
  checks: [
    {
      key: "window",
      verdict: "PASS",
      evidence: "The reply correctly refused an older purchase.",
      message_ids: ["u1", "a1"],
    },
  ],
  expectations: [
    { id: "window", statement: "Purchases older than 30 days are ineligible." },
  ],
};
function operation(results: CaseResult[], state = "COMPLETED"): Operation {
  return {
    id: "run-one",
    kind: "check",
    state,
    billing: "SETTLED",
    source: {
      kind: "provided_conversations",
      label: "Provided chats",
      artifact_id: "rules-one",
      evidence_set_id: "source-one",
    },
    models: {
      assistant: "assistant/model",
      target: "target/model",
      evaluator: "evaluator/model",
    },
    max_cost_nano_usd: 0,
    actual_cost_nano_usd: 0,
    results,
    scorecard: {
      total: results.length,
      passed: results.filter((result) => result.verdict === "PASS").length,
      failed: results.filter((result) => result.verdict === "FAIL").length,
      unknown: results.filter((result) => result.verdict === "UNKNOWN").length,
      evaluated: results.filter((result) => result.verdict !== "UNKNOWN")
        .length,
      pass_rate: null,
      coverage: 0,
    },
  };
}
function redacted(op: Operation): Operation {
  return {
    ...op,
    results: op.results.map((result) => ({
      ...result,
      messages: undefined,
      input: null,
      output: "",
      checks: result.checks.map((check) => ({
        ...check,
        evidence: "",
        message_ids: undefined,
      })),
    })),
  };
}
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  writeText.mockReset().mockResolvedValue(undefined);
  nextAnswer.mockReset();
  retest.mockReset();
  improve.mockReset();
  dispute.mockReset();
  save.mockReset();
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
  if (clipboardDescriptor)
    Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
  else Reflect.deleteProperty(navigator, "clipboard");
});
async function render(
  op: Operation,
  baseline?: Operation,
  withSave = false,
  testJourney = false,
) {
  const load = vi.fn(
    async (key: string) =>
      op.results.find((result) => result.case_key === key)!,
  );
  await act(async () =>
    root.render(
      <VibeScorecard
        testJourney={testJourney}
        operation={redacted(op)}
        baseline={baseline}
        loadEvidence={load}
        onImprove={improve}
        onRetest={retest}
        onNewReplies={nextAnswer}
        onDispute={dispute}
        onSave={withSave ? save : undefined}
        busy={false}
      />,
    ),
  );
  return load;
}

it("shows the outcome before filtering result rows without changing whole-run totals", async () => {
 await render(operation([failure,passing]));
 const heading=container.querySelector("h2")!.textContent;
 const select=container.querySelector<HTMLSelectElement>('select[aria-label="Filter tests"]');
 expect(select).not.toBeNull();
 expect(container.querySelector("h2")!.compareDocumentPosition(select!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
 await act(async()=>{select!.value="passing";select!.dispatchEvent(new Event("change",{bubbles:true}));});
 expect(container.querySelector("h2")!.textContent).toBe(heading);
 expect(container.querySelector("h2 + p")!.textContent).toBe("1 passed · 1 failed");
 expect(container.textContent).not.toContain("needs attention.");
 expect(container.textContent).not.toContain("The purchase age was asked twice");
 expect(container.textContent).toContain("The return window was respected");
});
function button(label: string) {
  const found = [
    ...container.querySelectorAll<HTMLButtonElement>("button"),
  ].find((item) => item.textContent?.trim() === label);
  if (!found) throw new Error(`Missing button: ${label}`);
  return found;
}
async function click(label: string) {
  await act(async () => button(label).click());
}

it("makes fixing, rerunning and keeping tests direct actions for the core journey", async () => {
  const op = operation([passing, failure]);
  op.source = {
    kind: "prompt",
    artifact_id: "suite-one",
    label: "Agent instructions",
  };
  await render(op, undefined, true, true);
  expect(container.querySelector("h2")!.textContent).toBe(
    "Your agent passed 1 of 2 tests",
  );
  expect(container.textContent).not.toContain("Test new replies");
  expect(container.textContent).not.toContain("bring back its new replies");
  await click("Suggest a fix");
  expect(improve).toHaveBeenCalledOnce();
  await click("Run tests again");
  expect(retest).toHaveBeenCalledOnce();
  expect(nextAnswer).not.toHaveBeenCalled();
  await click("Keep these tests");
  expect(save).toHaveBeenCalledOnce();
});

it("makes keeping tests the primary action for a passing first run", async () => {
  await render(operation([passing]), undefined, true, true);
  expect(
    button("Keep these tests").classList.contains("vibe-button-primary"),
  ).toBe(true);
  expect(
    button("Run tests again").classList.contains("vibe-button-primary"),
  ).toBe(false);
});

it("leads with one reply and keeps complete evidence behind a single accessible disclosure", async () => {
  const load = await render(operation([passing, failure]));
  const leading = container.querySelector('[aria-label="Leading finding"]')!;
  expect(container.querySelector("h2")!.textContent).toBe("1 test failed");
  expect(leading.querySelector("details")!.open).toBe(true);
  expect(leading.textContent).toContain(failure.checks[0].evidence);
  const visibleQuotes = [...leading.querySelectorAll("blockquote")].filter(
    (quote) => !quote.closest("[hidden]"),
  );
  expect(visibleQuotes).toHaveLength(1);
  expect(visibleQuotes[0].textContent).toContain(
    "How many days ago did you buy it?",
  );
  expect(visibleQuotes[0].textContent).toContain("App reply");
  expect(button("Show full conversation").getAttribute("aria-expanded")).toBe("false");
  const fullEvidence = document.getElementById(
    button("Show full conversation").getAttribute("aria-controls")!,
  )!;
  expect(fullEvidence.hidden).toBe(true);
  expect(fullEvidence.textContent).toContain("I bought it 10 days ago.");
  expect(fullEvidence.textContent).toContain("Ask only for missing details.");
  expect(fullEvidence.textContent).toContain("Full conversation · 4 messages");
  // One check that is the finding: the finding above is its only copy.
  expect(fullEvidence.textContent).not.toContain("All expectations");
  expect(leading.textContent!.split(failure.checks[0].evidence).length - 1).toBe(1);
  // Only the finding offers a change; its expectation is not repeated as a second button.
  expect([...container.querySelectorAll("button")].filter((b) => b.textContent?.trim() === "Change expected result")).toHaveLength(1);
  expect(
    button("Copy fix prompt").closest('[aria-label="Leading finding"]'),
  ).toBe(leading);
  expect(button("Change expected result").closest("[hidden]")).toBeNull();
  expect(load.mock.calls).toEqual([["memory"]]);
  for (const label of ["Other results", "About this run"]) {
    const details = [...container.querySelectorAll("details")].find((node) =>
      node.firstElementChild?.textContent?.startsWith(label),
    );
    expect(details?.open).toBe(false);
  }
  await click("Show full conversation");
  expect(button("Hide full conversation").getAttribute("aria-expanded")).toBe("true");
  expect(fullEvidence.hidden).toBe(false);
  await click("Change expected result");
  expect(dispute).toHaveBeenCalledWith(
    "Ask only for missing details.",
    failure,
  );
});

it("keeps a long reply complete in evidence and the fix prompt while showing a labeled excerpt", async () => {
  const fullReply =
    "This sentence is part of the original reply. ".repeat(10) +
    "The exact final sentence.";
  const long = {
    ...failure,
    messages: failure.messages!.map((message) =>
      message.id === "a2" ? { ...message, content: fullReply } : message,
    ),
  };
  await render(operation([long]));
  const quote = container.querySelector(
    '[aria-label="Leading finding"] blockquote',
  )!;
  expect(quote.textContent).toContain("App reply · preview; full reply below");
  expect(quote.textContent).not.toContain("The exact final sentence.");
  await click("Show full conversation");
  const fullEvidence = document.getElementById(
    button("Hide full conversation").getAttribute("aria-controls")!,
  )!;
  expect(fullEvidence.hidden).toBe(false);
  expect(fullEvidence.textContent).toContain(fullReply);
  await click("Copy fix prompt");
  expect(writeText.mock.calls[0][0]).toContain(fullReply);
});

it("preserves a reader’s collapsed finding until a different run arrives", async () => {
  const op = operation([failure]);
  await render(op);
  await act(async () => {
    const details = container.querySelector<HTMLDetailsElement>(
      '[aria-label="Leading finding"] > details',
    )!;
    details.open = false;
    details.dispatchEvent(new Event("toggle"));
  });
  const sameRunLoad = await render(structuredClone(op));
  expect(
    container.querySelector<HTMLDetailsElement>(
      '[aria-label="Leading finding"] > details',
    )!.open,
  ).toBe(false);
  expect(sameRunLoad).not.toHaveBeenCalled();
  const newRunLoad = await render({ ...op, id: "run-two" });
  expect(
    container.querySelector<HTMLDetailsElement>(
      '[aria-label="Leading finding"] > details',
    )!.open,
  ).toBe(true);
  expect(newRunLoad).toHaveBeenCalledWith("memory");
});

it("includes every cited message in the evidence disclosure", async () => {
  const allCited = {
    ...failure,
    checks: [
      {
        ...failure.checks[0],
        message_ids: failure.messages!.map((message) => message.id),
      },
    ],
  };
  await render(operation([allCited]));
  await click("Show full conversation");
  const supporting = container.querySelector(
    '[aria-label="Supporting messages"]',
  )!;
  expect(supporting.querySelectorAll("blockquote")).toHaveLength(4);
  for (const message of failure.messages!)
    expect(supporting.textContent).toContain(message.content);
});

it("copies an honest prompt and dispatches the new-answer callback only after a separate click", async () => {
  await render(operation([failure, passing]));
  await click("Copy fix prompt");
  expect(writeText).toHaveBeenCalledTimes(1);
  const prompt = writeText.mock.calls[0][0];
  expect(prompt).toContain(failure.messages![0].content);
  expect(prompt).toContain(failure.messages![3].content);
  expect(prompt).toContain("Ask only for missing details.");
  expect(prompt).toContain("does not establish a root cause");
  expect(prompt).toContain("No code or agent instructions have been changed");
  expect(nextAnswer).not.toHaveBeenCalled();
  expect(improve).not.toHaveBeenCalled();
  expect(retest).not.toHaveBeenCalled();
  expect(button("Test new replies").className).toContain(
    "vibe-button-primary",
  );
  await click("Test new replies");
  expect(nextAnswer).toHaveBeenCalledTimes(1);
});

it("provides a selectable prompt when the browser refuses clipboard access", async () => {
  writeText.mockRejectedValue(new Error("Clipboard blocked"));
  await render(operation([failure]));
  await click("Copy fix prompt");
  const fallback = container.querySelector<HTMLTextAreaElement>(
    'textarea[aria-label="Fix prompt to copy"]',
  )!;
  expect(fallback.readOnly).toBe(true);
  expect(fallback.value).toContain(failure.messages![3].content);
  expect(container.textContent).toContain("Select the prompt below");
  expect(nextAnswer).not.toHaveBeenCalled();
});

it("copies every original conversation in order without replacing customer text", async () => {
  const load = await render(operation([passing, failure]));
  await click("Copy original inputs");
  const copied = JSON.parse(writeText.mock.calls[0][0]);
  expect(copied.conversations).toHaveLength(2);
  expect(copied.conversations[0].messages[0].content).toBe(
    passing.messages![0].content,
  );
  expect(copied.conversations[1].messages[0].content).toBe(
    failure.messages![0].content,
  );
  expect(copied.conversations[1].messages[2].content).toBe(
    failure.messages![2].content,
  );
  expect(copied.conversations[1].messages[1].content).toContain(
    "Paste the new agent reply",
  );
  expect(load.mock.calls).toEqual([["memory"], ["boundary"], ["memory"]]);
  expect(retest).not.toHaveBeenCalled();
});

it("prioritizes a new regression over an existing failure without hiding either", async () => {
  const regressed = {
    ...passing,
    verdict: "FAIL" as const,
    checks: [
      {
        ...passing.checks[0],
        verdict: "FAIL" as const,
        evidence: "An older purchase was accepted.",
      },
    ],
  };
  const old = operation([failure, passing]);
  const next = { ...operation([failure, regressed]), id: "next", baseline_id: old.id };
  old.grading = next.grading = { version: 1, hash: "matching" } as Operation["grading"];
  await render(next, old);
  expect(container.querySelector("h2")!.textContent).toContain("1 newly failing test");
  expect(
    container.querySelector('[aria-label="Leading finding"] summary')!
      .textContent,
  ).toContain(passing.title);
  expect(
    container.querySelector('[aria-label="Individual results"]')!.textContent,
  ).toContain(failure.title);
  expect(container.textContent).toContain(
    "0 previously failing tests now pass · 1 new failure",
  );
});

it("does not turn regrading unchanged replies into an improvement claim", async () => {
  const op = operation([passing]);
  op.source!.comparison = "rechecked";
  await render(op, operation([{ ...passing, verdict: "FAIL" }]));
  expect(container.querySelector("h2")!.textContent).toBe(
    "Same replies graded again",
  );
  expect(container.textContent).toContain(
    "A different grade does not show that your agent improved",
  );
  expect(container.textContent).not.toContain("now passes");
});

it.each(["PARTIAL", "CANCELLED"])(
  "keeps %s missing evidence unresolved without a fabricated fix",
  async (state) => {
    const unknown: CaseResult = {
      ...failure,
      verdict: "UNKNOWN",
      messages: undefined,
      output: "",
      checks: [
        {
          key: "remember",
          verdict: "UNKNOWN",
          evidence: "The provider did not finish.",
        },
      ],
      error: {
        code: "provider_unavailable",
        message: "No answer was produced.",
      },
    };
    await render(operation([unknown], state));
    expect(container.textContent).toContain("1 result is unresolved");
    expect(container.textContent).toContain(
      "An unfinished grade is not a failure by your agent",
    );
    expect(
      container
        .querySelector('[aria-label="Leading finding"] details')!
        .hasAttribute("open"),
    ).toBe(true);
    expect(container.textContent).not.toContain("Copy fix prompt");
    if (state === "CANCELLED")
      expect(container.querySelector("h2")!.textContent).toContain(
        "Run stopped",
      );
  },
);

it("keeps an all-pass outcome scoped to its tests and offers no repair prompt", async () => {
  await render(operation([passing]));
  expect(container.querySelector("h2")!.textContent).toBe(
    "No issues found in this test",
  );
  expect(container.textContent).toContain(
    "These findings cover only this test",
  );
  expect(container.textContent).not.toContain("Copy fix prompt");
  expect(button("Test new replies").className).toContain(
    "vibe-button-primary",
  );
});

it("offers saving as the primary action after a successful comparison without starting it automatically", async () => {
  await render(
    operation([passing]),
    operation([{ ...passing, verdict: "FAIL" }]),
    true,
  );
  expect(button("Save this result").className).toContain("vibe-button-primary");
  expect(button("Save this result").closest("details")).toBeNull();
  expect(button("Test new replies").className).toContain(
    "vibe-button-secondary",
  );
  expect(save).not.toHaveBeenCalled();
  await click("Save this result");
  expect(save).toHaveBeenCalledTimes(1);
  expect(nextAnswer).not.toHaveBeenCalled();
});

it.each(["initial", "rechecked", "stopped", "incomplete", "empty", "failed"])(
  "does not promote saving an %s result as a successful comparison",
  async (kind) => {
    const op = operation(
      kind === "empty" ? [] : kind === "failed" ? [failure] : [passing],
    );
    if (kind === "rechecked") op.source!.comparison = "rechecked";
    if (kind === "stopped") op.state = "CANCELLED";
    if (kind === "incomplete") op.scorecard!.incomplete_cases = 1;
    await render(
      op,
      kind === "initial" ? undefined : operation([passing]),
      true,
    );
    expect(button("Save this result").className).not.toContain(
      "vibe-button-primary",
    );
    expect(button("Save this result").closest("details")).not.toBeNull();
    expect(save).not.toHaveBeenCalled();
  },
);


it("keeps before and after evidence independent when the baseline load fails",async()=>{
 const baseline={...operation([failure]),id:"baseline"};const current={...operation([failure]),id:"current"};
 let fail=true;
 const load=vi.fn(async(id:string,key:string)=>{if(id==="baseline"&&fail)throw Error("Baseline temporarily unavailable");return {...failure,case_key:key,output:id==="baseline"?"Original reply":"Changed reply"}});
 await act(async()=>root.render(<ResultComparison current={current} baseline={baseline} load={load}/>));
 expect(container.textContent).toContain("Comparison with changed or unverified settings");
 await click("Compare saved replies");
 expect(container.querySelector('[aria-label="Before"]')?.textContent).toContain("Baseline temporarily unavailable");
 expect(container.querySelector('[aria-label="After"]')?.textContent).toContain("Changed reply");
 fail=false;await click("Reload before");
 expect(container.querySelector('[aria-label="Before"]')?.textContent).toContain("Original reply");
 expect(load.mock.calls.filter(([id])=>id==="current")).toHaveLength(1);
 const capture={scope:"unknown" as const,instrumentation:"unknown" as const};
 const metadata={result_ref:{kind:"vibe_operation" as const,id:"baseline"},origin:"prototype" as const,fingerprints:{model:"m",build:"b",test:"t",judge:"j",config:"c"},capture_scope:{messages:capture,tools:capture,retrieval:capture},application_revision:null};
 baseline.evidence_metadata=metadata;current.evidence_metadata={...metadata,result_ref:{kind:"vibe_operation",id:"current"},fingerprints:{...metadata.fingerprints,build:"updated"}};
 baseline.result_scope=current.result_scope={artifact_id:"rules-one",case_keys:["memory"]};
 expect(compareResultEvidence(current,baseline).controlled).toBe(true);
 current.evidence_metadata.fingerprints.config=null;
 expect(compareResultEvidence(current,baseline).controlled).toBe(false);
 current.evidence_metadata.fingerprints.config="c";current.result_scope={artifact_id:"rules-one",case_keys:["different"]};
 expect(compareResultEvidence(current,baseline).controlled).toBe(false);
});


it("recovers the same grade review receipt after a lost response and labels later evidence",async()=>{
 const op=operation([failure]);const source={...failure,evidence_hash:"e".repeat(64)};
 const before=JSON.stringify(op);const requests:Record<string,unknown>[]=[];let changed=false;let saved:Record<string,unknown>|undefined;
 vi.stubGlobal("fetch",vi.fn(async(_url:string,options:RequestInit={})=>{
  if(options.method==="POST"){
   const body=JSON.parse(String(options.body));requests.push(body);
   saved={...body,id:"review-one",created_at:"2026-10-26T00:00:00Z",evidence_hash:source.evidence_hash,evidence:source};
   if(requests.length===1)throw new TypeError("Reply lost after commit");
   return new Response(JSON.stringify(saved),{status:201,headers:{"Content-Type":"application/json"}});
  }
  return new Response(JSON.stringify(saved?[{...saved,evidence_changed:changed}]:[]),{headers:{"Content-Type":"application/json"}});
 }));
 const load=async()=>source;
 const draw=async(cursor:number)=>act(async()=>root.render(<GradeReviews sessionID="review-project" operation={op} eventCursor={cursor} load={load}/>));
 await draw(1);await click("Review a grade");
 const input=container.querySelector<HTMLTextAreaElement>('[aria-label="Grade review reason"]')!;
 await act(async()=>{Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(input,"The earlier reply satisfies the rule.");input.dispatchEvent(new Event("input",{bubbles:true}))});
 await click("Save grade review");
 expect(requests[0]).toMatchObject({source_evidence_hash:source.evidence_hash,result_ref:{kind:"vibe_operation",id:op.id},artifact_id:failure.version,case_key:failure.case_key});
 await act(async()=>root.render(<div/>));await draw(1);await click("Review a grade");await click("Retry saving this review");
 expect(requests).toHaveLength(2);expect(requests[1]).toEqual(requests[0]);
 changed=true;await draw(2);
 expect(container.textContent).toContain("This review retains the original evidence");
 expect(JSON.stringify(op)).toBe(before);
});


it("offers mixed failed and unassessed checks in the unresolved filter and exact retry selection", async()=>{
 const mixed={...failure,case_key:"mixed",title:"Partially assessed",expected_checks:2,checks:[failure.checks[0],{key:"other",verdict:"UNKNOWN" as const,evidence:"",error:{message:"Judge unavailable"}}]};
 const waiting={...failure,case_key:"waiting",title:"Awaiting assessment",verdict:"UNKNOWN" as const,expected_checks:1,checks:[{key:"pending",verdict:"UNKNOWN" as const,evidence:""}]};
 const op=operation([failure,mixed,passing,waiting]);
 op.scorecard={...op.scorecard!,incomplete_cases:2,checks_expected:5,checks_evaluated:3};
 await render(op);
 expect(container.textContent).toContain("1 passed · 2 failed · 2 unresolved");
 expect(container.querySelector("h2")!.textContent).toBe("2 tests failed");
 expect(container.textContent).toContain("can also be counted as passed or failed");
 const select=container.querySelector<HTMLSelectElement>('[aria-label="Filter tests"]')!;
 await act(async()=>{select.value="failing";select.dispatchEvent(new Event("change",{bubbles:true}))});
 expect(container.textContent).toContain("Partially assessed");
 expect(container.textContent).toContain(failure.title);
 expect(container.textContent).not.toContain("Awaiting assessment");
 expect(container.textContent).toContain("1 passed · 2 failed · 2 unresolved");
 await act(async()=>{select.value="unresolved";select.dispatchEvent(new Event("change",{bubbles:true}))});
 expect(container.textContent).toContain("Partially assessed");
 expect(container.textContent).not.toContain(failure.title);
 expect(container.textContent).toContain("Awaiting assessment");
 expect(container.querySelector("h2")!.textContent).toBe("2 tests failed");
 expect(container.textContent).toContain("1 passed · 2 failed · 2 unresolved");
 const retry=vi.fn();
 await act(async()=>root.render(<ResultRecoveryControls operation={op} busy={false} onReload={()=>{}} onRetry={retry} onFullRun={()=>{}}/>));
 expect(container.querySelectorAll('input[type="checkbox"]')).toHaveLength(2);
 await click("Retry unresolved");expect(retry).toHaveBeenCalledWith(["mixed","waiting"]);
});

it.each([
  ["complete", "COMPLETED", [passing], false],
  ["stopped", "CANCELLED", [passing], true],
  ["unresolved", "COMPLETED", [{ ...failure, verdict: "UNKNOWN" as const, checks: [{ key: "remember", verdict: "UNKNOWN" as const, evidence: "" }] }], true],
] as const)("offers the full rerun and evidence reload only for a %s run that needs recovery", async (_kind, state, results, offered) => {
  const fullRun = vi.fn();
  await act(async () => root.render(<ResultRecoveryControls operation={operation([...results], state)} busy={false} onReload={() => {}} onFullRun={fullRun} />));
  const labels = [...container.querySelectorAll("button")].map(b => b.textContent);
  expect(labels.includes("Run all again")).toBe(offered);
  expect(labels.includes("Reload saved evidence")).toBe(offered);
  if (offered) { await click("Run all again"); expect(fullRun).toHaveBeenCalledOnce(); }
  else expect(container.querySelector('[aria-label="Result recovery"]')).toBeNull();
});

it("keeps one primary action on a failed check and moves Suggest a change into the action row", async () => {
  const op = operation([failure]);
  op.source = { kind: "prompt", artifact_id: "suite-one", label: "Agent instructions" };
  await act(async () => root.render(<VibeScorecard operation={op} loadEvidence={async () => failure} onImprove={improve} onRetest={retest} onSave={save} busy={false} />));
  expect(button("Suggest a change").closest("details")).toBeNull();
  expect([...container.querySelectorAll(".vibe-button-primary")].map(b => b.textContent?.trim())).toEqual(["Suggest a change"]);
  expect(button("Copy fix prompt").className).toContain("vibe-button-secondary");
  // "Run tests again" already reruns the same tests when there are no new replies.
  expect(container.textContent).not.toContain("Run the same tests again");
  await click("Run tests again");
  expect(retest).toHaveBeenCalledOnce();
  await click("Suggest a change");
  expect(improve).toHaveBeenCalledOnce();
  expect(button("Save this result").closest("details")).not.toBeNull();
});

it("keeps Compare an update only while the new-answer button collects new replies", async () => {
  await render(operation([failure]));
  expect(button("Compare an update").closest("details")).not.toBeNull();
  await click("Compare an update");
  expect(retest).toHaveBeenCalledOnce();
  await click("Test new replies");
  expect(nextAnswer).toHaveBeenCalledOnce();
  expect(container.textContent).not.toContain("Start a separate run");
});

it("starts a fresh review only after the stale request is authoritatively rejected without a receipt",async()=>{
 const op=operation([failure]);let source={...failure,evidence_hash:"1".repeat(64)};
 const requests:Record<string,unknown>[]=[];let rejection="transport";
 vi.stubGlobal("fetch",vi.fn(async(_url:string,options:RequestInit={})=>{
  if(options.method!=="POST")return new Response("[]",{headers:{"Content-Type":"application/json"}});
  const body=JSON.parse(String(options.body));requests.push(body);
  if(rejection==="transport")throw new TypeError("Disconnected before commit");
  if(rejection==="auth")return new Response(JSON.stringify({error:{code:"unauthorized",message:"Sign in again"}}),{status:401});
  if(rejection==="conflict")return new Response(JSON.stringify({error:{code:"idempotency_conflict",message:"Request identity conflicts"}}),{status:409});
  if(rejection==="stale")return new Response(JSON.stringify({error:{code:"evidence_changed",message:"Evidence changed. Reload."}}),{status:409});
  return new Response(JSON.stringify({...body,id:"new-review",created_at:"2026-10-26T00:00:00Z",evidence:source}),{status:201});
 }));
 const draw=()=>act(async()=>root.render(<GradeReviews sessionID="uncommitted-project" operation={op} load={async()=>source}/>));
 await draw();await click("Review a grade");
 const input=container.querySelector<HTMLTextAreaElement>('[aria-label="Grade review reason"]')!;
 await act(async()=>{Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(input,"Retain my reason.");input.dispatchEvent(new Event("input",{bubbles:true}))});
 await click("Save grade review");
 expect(container.textContent).not.toContain("Review updated evidence");
 source={...source,evidence_hash:"2".repeat(64),output:"Late source reply"};
 await act(async()=>root.render(<div/>));await draw();await click("Review a grade");
 rejection="auth";await click("Retry saving this review");
 expect(container.textContent).not.toContain("Review updated evidence");
 rejection="conflict";await click("Retry saving this review");
 expect(container.textContent).not.toContain("Correct review reason");
 expect(container.textContent).not.toContain("Review updated evidence");
 rejection="stale";await click("Retry saving this review");
 expect(requests.slice(1)).toEqual([requests[0],requests[0],requests[0]]);
 await click("Review updated evidence");
 expect(container.querySelector<HTMLTextAreaElement>('[aria-label="Grade review reason"]')!.value).toBe("Retain my reason.");
 expect(container.textContent).toContain("Late source reply");
 rejection="success";await click("Save grade review");
 expect(requests[4]).toMatchObject({source_evidence_hash:source.evidence_hash,reason:"Retain my reason.",case_key:failure.case_key});
 expect(requests[4].client_id).not.toBe(requests[0].client_id);
});


it.each(["new ASCII", "new multibyte", "persisted invalid"])("corrects an overlimit grade-review reason without trapping its receipt (%s)",async(mode)=>{
 const project=`byte-limit-${mode}`;const op=operation([failure]);const source={...failure,evidence_hash:"2".repeat(64)};
 const invalid=mode==="new ASCII"?"a".repeat(4097):"🙂".repeat(1025);
 const valid=mode==="new ASCII"?"a".repeat(4096):"🙂".repeat(1024);
 const previous={client_id:"previous-invalid-request",source_evidence_hash:"1".repeat(64),result_ref:{kind:"vibe_operation",id:op.id},artifact_id:failure.version,case_key:failure.case_key,reason:invalid};
 const posts:Record<string,unknown>[]=[];
 vi.stubGlobal("fetch",vi.fn(async(_url:string,options:RequestInit={})=>{
  if(options.method!=="POST")return new Response("[]",{headers:{"Content-Type":"application/json"}});
  const body=JSON.parse(String(options.body));posts.push(body);
  return new Response(JSON.stringify({...body,id:"saved-review",created_at:"2026-10-26T00:00:00Z",evidence:source}),{status:201});
 }));
 if(mode==="persisted invalid"){
  // Restore a request left by the prior UI after the API's pretransaction
  // invalid_request rejection. It cannot have a receipt for this exact body.
  const editors={
   [editorDraftKey(project,op.id,"review-request")]:{artifact:op.id,source:"review-v1",value:JSON.stringify(previous)},
   [editorDraftKey(project,op.id,`review:${failure.case_key}`)]:{artifact:op.id,source:"review-v1",value:invalid},
  };
  persistDrafts(project,{...emptyDrafts(),editors});
 }
 const draw=()=>act(async()=>root.render(<GradeReviews sessionID={project} operation={op} load={async()=>source}/>));
 const typeReason=async(value:string)=>act(async()=>{const input=container.querySelector<HTMLTextAreaElement>('[aria-label="Grade review reason"]')!;Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,"value")!.set!.call(input,value);input.dispatchEvent(new Event("input",{bubbles:true}))});
 await draw();await click("Review a grade");
 if(mode!=="persisted invalid"){
  await typeReason(invalid);
  expect(button("Save grade review").disabled).toBe(true);
  // The submit owner must also reject Enter/programmatic form submission.
  await act(async()=>container.querySelector('form')!.dispatchEvent(new Event("submit",{bubbles:true,cancelable:true})));
  expect(readEditorDraft(project,op.id,"review-request")).toBeUndefined();
 }else{
  expect(container.querySelector<HTMLTextAreaElement>('[aria-label="Grade review reason"]')!.disabled).toBe(true);
  await click("Correct review reason");
  expect(readEditorDraft(project,op.id,"review-request")).toBeUndefined();
 }
 expect(posts).toHaveLength(0);
 expect(container.textContent).toContain("4,096 UTF-8 bytes");
 await act(async()=>root.render(<div/>));await draw();await click("Review a grade");
 const input=container.querySelector<HTMLTextAreaElement>('[aria-label="Grade review reason"]')!;
 expect(input.value).toBe(invalid);expect(input.disabled).toBe(false);
 await typeReason(valid);expect(button("Save grade review").disabled).toBe(false);
 await click("Save grade review");
 expect(posts).toHaveLength(1);expect(posts[0]).toMatchObject({reason:valid,source_evidence_hash:source.evidence_hash,case_key:failure.case_key});
 expect(posts[0].client_id).not.toBe(previous.client_id);
 expect(readEditorDraft(project,op.id,"review-request")).toBeUndefined();
 discardDrafts(project);
});


it("opens Guide for the chosen case even when its evidence cannot load", async () => {
  const ask = vi.fn();
  await act(async () => root.render(<VibeScorecard operation={redacted(operation([failure]))} loadEvidence={async () => { throw Error("Saved evidence offline"); }} onImprove={improve} onRetest={retest} onAskCase={ask} busy={false} />));
  expect(button("Ask Guide about this test").closest(".vibe-evidence-actions")).not.toBeNull();
  await click("Ask Guide about this test");
  expect(ask).toHaveBeenCalledExactlyOnceWith("memory");
  expect(retest).not.toHaveBeenCalled();
  expect(improve).not.toHaveBeenCalled();
});

it("puts the Improve finding and Suggest a fix before a single More disclosure", async () => {
  await render(operation([passing, failure]), undefined, true, true);
  const fix = button("Suggest a fix"); expect(fix).toBeDefined(); expect(fix.closest("details")).toBeNull();
  expect([...container.querySelectorAll(".vibe-button-primary")].map(b => b.textContent?.trim())).toEqual(["Suggest a fix"]);
  expect(button("Run tests again").closest("details")?.querySelector("summary")?.textContent).toBe("More");
  expect(container.textContent).toContain("Grading model:");
  expect(container.querySelector('[aria-label="Leading finding"]')).not.toBeNull();
  await click("Suggest a fix"); expect(improve).toHaveBeenCalledOnce();
});

it.each(["prompt", "provided_conversations"] as const)("shows the shared input, reply, expected behavior and finding order for %s Improve results", async kind => {
  const result = { ...failure, input: kind === "prompt" ? "May I return this?" : { conversation: "Conversation 1" }, output: kind === "prompt" ? "How many days ago did you buy it?" : "" };
  const op = operation([result]); op.source!.kind = kind;
  await render(op, undefined, true, true);
  const leading = container.querySelector('[aria-label="Leading finding"]')!;
  const labels = [...leading.querySelectorAll("p")].filter(p => ["We gave it", "It replied", "It should", "What needs attention"].includes(p.textContent!.trim()));
  expect(labels.map(p => p.textContent!.trim())).toEqual(["We gave it", "It replied", "It should", "What needs attention"]);
  expect(labels[0].parentElement!.textContent).toContain(kind === "prompt" ? "May I return this?" : "I bought it 10 days ago.");
  if (kind === "provided_conversations") {
    expect(labels[0].parentElement!.textContent).toContain("It is unopened.");
    expect(labels[0].parentElement!.textContent).not.toContain('"conversation"');
    expect(labels[0].parentElement!.textContent).not.toContain("Conversation 1");
    expect(labels[0].parentElement!.querySelectorAll('.vibe-recorded-bubble[data-role="user"]')).toHaveLength(2);
  }
  expect(labels[1].parentElement!.textContent).toContain("How many days ago did you buy it?");
  expect(labels[2].parentElement!.textContent).toContain("Ask only for missing details.");
  expect(labels[3].parentElement!.textContent).toContain(failure.checks[0].evidence);
  expect(labels.every(label => !label.closest("[hidden]"))).toBe(true);
  expect(button("Show full conversation").getAttribute("aria-expanded")).toBe("false");
  await click("Show full conversation");
  expect(document.getElementById(button("Hide full conversation").getAttribute("aria-controls")!)!.hidden).toBe(false);
});

it("uses the same expected behavior label and order in the compact Build evidence", async () => {
  await act(async () => root.render(<CaseEvidence summary={failure} load={async () => failure} concise exampleFirst compactPreview defaultOpen />));
  const labels = [...container.querySelectorAll("p")].map(p => p.textContent!.trim()).filter(text => ["We gave it", "It replied", "It should", "What needs attention"].includes(text));
  expect(labels).toEqual(["We gave it", "It replied", "It should", "What needs attention"]);
  expect(container.textContent).toContain("Ask only for missing details.");
});

it("keeps the actual expected rule when a leading Improve grade is disputed", async () => {
  await render(operation([failure]), undefined, true, true);
  const more = [...container.querySelectorAll("details")].find(detail => detail.querySelector(":scope > summary")?.textContent === "More")!;
  await act(async () => more.querySelector<HTMLButtonElement>("summary")!.click());
  const row = container.querySelector<HTMLDetailsElement>('[aria-label="Leading finding"] .vibe-result-row')!;
  await act(async () => { row.open = true; row.dispatchEvent(new Event("toggle")); });
  await click("Change expected result");
  expect(dispute).toHaveBeenCalledWith("Ask only for missing details.", failure);
});

it("shows other results separately from More with a passed glyph and word in the Improve journey", async () => {
  await render(operation([passing, failure]), undefined, true, true);
  const other = container.querySelector(".vibe-other-results")!;
  expect(other?.querySelector("summary")?.textContent).toBe("Other results · 1");
  expect(other.closest("details")?.parentElement?.closest("details")).toBeNull();
  expect(other.querySelector(".vibe-verdict-word")?.textContent).toBe("Passed");
  expect(other.querySelector("svg.vibe-verdict-pass")).not.toBeNull();
  expect(other.querySelector(".vibe-result-row")?.hasAttribute("open")).toBe(false);
  expect(button("Suggest a fix").querySelector("svg")).not.toBeNull();
});

it.each([
  { failed: false, label: "Results · 3", rows: 3 },
  { failed: true, label: "Other results · 2", rows: 2 },
])("labels Improve results according to whether a finding leads ($label)", async ({ failed, label, rows }) => {
  const results = [passing, { ...passing, case_key: "second" }, { ...(failed ? failure : passing), case_key: "third" }];
  await render(operation(results), undefined, true, true);
  const list = container.querySelector(".vibe-other-results")!;
  expect(list.querySelector(":scope > summary")?.textContent).toBe(label);
  expect(list.querySelectorAll(".vibe-result-row")).toHaveLength(rows);
  expect(container.querySelectorAll('[aria-label="Leading finding"]')).toHaveLength(failed ? 1 : 0);
});

it.each([0, 2, 3])("U12/U13 Improve uses the shared outcome and fix label (%s passed)", async passed => {
  const results = Array.from({ length: 3 }, (_, index) => ({ ...(index < passed ? passing : failure), case_key: `case-${index}` }));
  await render(operation(results), undefined, true, true);
  expect(container.querySelector(".vibe-results-outcome")?.textContent).toBe(passed === 3 ? "Your agent passed all 3 tests" : `Your agent passed ${passed} of 3 tests`);
  if (passed < 3) expect(button("Suggest a fix").querySelector("svg")).not.toBeNull();
});


it.each(["prompt", "provided_conversations"] as const)("exposes one stable More hook for %s Improve result actions", async kind => {
  const op = operation([failure, passing]); op.source!.kind = kind;
  await render(op, undefined, true, true);
  const more = container.querySelectorAll("details.vibe-results-more");
  expect(more).toHaveLength(1);
  expect(more[0].querySelector(":scope > summary")?.textContent).toBe("More");
  expect(button("Run tests again").closest("details")).toBe(more[0]);
  expect(button("Suggest a fix").closest("details")).toBeNull();
  expect(more[0].contains(container.querySelector(".vibe-other-results"))).toBe(false);
});

it.each([false, true])("does not display a recorded conversation marker when customer messages are unavailable (assistant replies: %s)", async assistantOnly => {
  const result = { ...failure, title: "", input: { conversation: "Conversation 1" }, messages: assistantOnly ? failure.messages!.filter(message => message.role === "assistant") : undefined };
  await render(operation([result]), undefined, false, true);
  const input = container.querySelector(".vibe-example-evidence > div")!;
  expect(input.textContent).toContain("No customer message was recorded.");
  expect(container.textContent).not.toContain('"conversation"');
});

it.each([[0, "Your agent passed 0 of 1 test"], [1, "Your agent passed all 1 test"]])("uses singular in the Improve headline when %i of one test passed", async (passed, headline) => {
  await render(operation([passed ? passing : failure]), undefined, false, true);
  expect(container.querySelector(".vibe-results-outcome")?.textContent).toBe(headline);
});

it("P5: puts the Improve primary between the leading finding and Other results", async () => {
  await render(operation([failure, passing]), undefined, true, true);
  const finding = container.querySelector('[aria-label="Leading finding"]')!;
  const primary = button("Suggest a fix");
  expect(finding.nextElementSibling).toBe(primary);
  expect(primary.nextElementSibling).toBe(container.querySelector(".vibe-other-results"));
  expect(container.querySelector('[aria-label="Individual results"]')?.contains(container.querySelector(".vibe-other-results"))).toBe(true);
});
it.each([false, true])("P5: Ask Guide shares the bottom evidence action row (example first: %s)", async exampleFirst => {
  const ask = vi.fn();
  await act(async () => root.render(<CaseEvidence summary={failure} load={async () => failure} concise exampleFirst={exampleFirst} defaultOpen onAsk={ask} onDispute={dispute} />));
  const action = button("Show full conversation").parentElement!;
  const guide = button("Ask Guide about this test");
  expect(guide.parentElement).toBe(action);
  expect(action.contains(button("Change expected result"))).toBe(true);
  expect(container.querySelector(".vibe-result-row > div > button")?.textContent).not.toBe("Ask Guide about this test");
  await act(async () => guide.click()); expect(ask).toHaveBeenCalledOnce();
});

it("P5: keeps Ask Guide mounted and focused when delayed evidence arrives", async () => {
  let resolve!: (result: CaseResult) => void;
  const pending = new Promise<CaseResult>(done => { resolve = done; });
  await act(async () => root.render(<CaseEvidence summary={failure} load={() => pending} concise exampleFirst defaultOpen onAsk={vi.fn()} onDispute={dispute} />));
  const guide = button("Ask Guide about this test");
  guide.focus(); expect(document.activeElement).toBe(guide);
  await act(async () => resolve(failure));
  expect(button("Ask Guide about this test")).toBe(guide);
  expect(document.activeElement).toBe(guide);
  expect(guide.parentElement).toBe(button("Show full conversation").parentElement);
});
