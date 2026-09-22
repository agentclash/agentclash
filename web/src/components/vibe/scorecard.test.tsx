import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CaseResult, Operation } from "@/lib/vibe";
import { VibeScorecard } from "./scorecard";

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
  expect(container.querySelector("h1")!.textContent).toBe(
    "1 of 2 tests passed",
  );
  expect(container.textContent).not.toContain("Check the new answer");
  expect(container.textContent).not.toContain("bring back its new replies");
  await click("Help me fix this");
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
  expect(container.querySelector("h1")!.textContent).toBe("One thing to fix");
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
  expect(button("See evidence").getAttribute("aria-expanded")).toBe("false");
  const fullEvidence = document.getElementById(
    button("See evidence").getAttribute("aria-controls")!,
  )!;
  expect(fullEvidence.hidden).toBe(true);
  expect(fullEvidence.textContent).toContain("I bought it 10 days ago.");
  expect(fullEvidence.textContent).toContain("Ask only for missing details.");
  expect(fullEvidence.textContent).toContain("Full conversation · 4 messages");
  expect(fullEvidence.textContent).toContain("All checks · 1");
  expect(
    button("Copy fix prompt").closest('[aria-label="Leading finding"]'),
  ).toBe(leading);
  expect(button("That’s not what I meant").closest("[hidden]")).toBeNull();
  expect(load.mock.calls).toEqual([["memory"]]);
  for (const label of ["Other results", "What was checked"]) {
    const details = [...container.querySelectorAll("details")].find((node) =>
      node.firstElementChild?.textContent?.startsWith(label),
    );
    expect(details?.open).toBe(false);
  }
  await click("See evidence");
  expect(button("Hide evidence").getAttribute("aria-expanded")).toBe("true");
  expect(fullEvidence.hidden).toBe(false);
  await click("That’s not what I meant");
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
  expect(quote.textContent).toContain("App reply · excerpt");
  expect(quote.textContent).not.toContain("The exact final sentence.");
  await click("See evidence");
  const fullEvidence = document.getElementById(
    button("Hide evidence").getAttribute("aria-controls")!,
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
  await click("See evidence");
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
  expect(button("Check the new answer").className).toContain(
    "vibe-button-primary",
  );
  await click("Check the new answer");
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
  await render(operation([failure, regressed]), operation([failure, passing]));
  expect(container.querySelector("h1")!.textContent).toContain("1 new issue");
  expect(
    container.querySelector('[aria-label="Leading finding"] summary')!
      .textContent,
  ).toContain(passing.title);
  expect(
    container.querySelector('[aria-label="Individual results"]')!.textContent,
  ).toContain(failure.title);
  expect(container.textContent).toContain(
    "0 previously failing chats now pass · 1 new failure",
  );
});

it("does not turn regrading unchanged replies into an improvement claim", async () => {
  const op = operation([passing]);
  op.source!.comparison = "rechecked";
  await render(op, operation([{ ...passing, verdict: "FAIL" }]));
  expect(container.querySelector("h1")!.textContent).toBe(
    "Same chats rechecked",
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
      "Missing evidence is not counted as a pass",
    );
    expect(
      container
        .querySelector('[aria-label="Leading finding"] details')!
        .hasAttribute("open"),
    ).toBe(true);
    expect(container.textContent).not.toContain("Copy fix prompt");
    if (state === "CANCELLED")
      expect(container.querySelector("h1")!.textContent).toContain(
        "Check stopped",
      );
  },
);

it("keeps an all-pass outcome scoped to its tested examples and offers no repair prompt", async () => {
  await render(operation([passing]));
  expect(container.querySelector("h1")!.textContent).toBe(
    "No issues found in this chat",
  );
  expect(container.textContent).toContain(
    "These findings cover only this chat",
  );
  expect(container.textContent).not.toContain("Copy fix prompt");
  expect(button("Check the new answer").className).toContain(
    "vibe-button-primary",
  );
});

it("offers saving as the primary action after a successful comparison without starting it automatically", async () => {
  await render(
    operation([passing]),
    operation([{ ...passing, verdict: "FAIL" }]),
    true,
  );
  expect(button("Save this check").className).toContain("vibe-button-primary");
  expect(button("Save this check").closest("details")).toBeNull();
  expect(button("Check the new answer").className).toContain(
    "vibe-button-secondary",
  );
  expect(save).not.toHaveBeenCalled();
  await click("Save this check");
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
    expect(button("Save this check").className).not.toContain(
      "vibe-button-primary",
    );
    expect(button("Save this check").closest("details")).not.toBeNull();
    expect(save).not.toHaveBeenCalled();
  },
);
