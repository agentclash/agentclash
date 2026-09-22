import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  defaultModels,
  type EvidenceSet,
  type Operation,
  type Session,
} from "@/lib/vibe";
import {
  EvaluationWorkspace,
  exampleAnswer,
  type EvaluationWorkspaceProps,
} from "./evaluation-workspace";
import { EvidenceIntake } from "./evidence-intake";

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("./scorecard", () => ({
  VibeScorecard: ({
    operation,
    onNewReplies,
  }: {
    operation: Operation;
    onNewReplies?: () => void;
  }) => (
    <article data-result={operation.id}>
      Finding for {operation.id}
      <button onClick={onNewReplies}>Check the new answer</button>
    </article>
  ),
}));

let container: HTMLDivElement;
let root: Root;
let props: EvaluationWorkspaceProps;

function session(messages: Session["document"]["messages"] = []): Session {
  return {
    id: "session-one",
    revision: 1,
    anonymous: true,
    document: {
      messages,
      requirements: [],
      artifacts: [],
      models: defaultModels,
    },
    operations: [],
  };
}

function operation(id: string): Operation {
  return {
    id,
    kind: "check",
    state: "COMPLETED",
    billing: "SETTLED",
    models: defaultModels,
    max_cost_nano_usd: 0,
    actual_cost_nano_usd: 0,
    source: {
      kind: "provided_conversations",
      label: "Your answer",
      artifact_id: "artifact-one",
      evidence_set_id: "evidence-one",
    },
    results: [],
    scorecard: {
      passed: 0,
      failed: 1,
      unknown: 0,
      total: 1,
      evaluated: 1,
      pass_rate: 0,
      coverage: 1,
    },
  };
}

function evidence(id: string, unknown = false): EvidenceSet {
  return {
    id,
    label: "App answer",
    raw: "User: Keep the total under 5000.\nAssistant: The total is 8000.",
    conversations: [
      {
        key: "trip",
        title: "Trip request",
        messages: [
          {
            id: "question",
            role: "user",
            content: "Keep the total under 5000.",
          },
          {
            id: "answer",
            role: unknown ? "unknown" : "assistant",
            content: "The total is 8000.",
          },
        ],
      },
    ],
  };
}

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({
      matches: true,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
  props = {
    session: null,
    view: "build",
    busy: false,
    dirty: false,
    content: "",
    onContent: vi.fn(),
    onSend: vi.fn(),
    onMessage: vi.fn(),
    onNavigate: vi.fn(),
    onRun: vi.fn(),
    onAttach: vi.fn(async () => true),
    onEdit: vi.fn(async () => true),
    onDirty: vi.fn(),
    onSave: vi.fn(),
    onImport: vi.fn(),
    onSettings: vi.fn(),
    onInstructions: vi.fn(),
    loadEvidence: vi.fn(),
    onAction: vi.fn(),
    onDispute: vi.fn(),
    notice: null,
    preview: null,
    instructions: null,
    savedChecks: [],
  };
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});

async function render(changes: Partial<EvaluationWorkspaceProps> = {}) {
  props = { ...props, ...changes };
  await act(async () => root.render(<EvaluationWorkspace {...props} />));
}

function button(text: string) {
  const found = [
    ...container.querySelectorAll<HTMLButtonElement>("button"),
  ].find(
    (item) =>
      item.textContent?.trim() === text ||
      item.getAttribute("aria-label") === text,
  );
  expect(found, `Expected button: ${text}`).toBeTruthy();
  return found!;
}

async function type(input: HTMLTextAreaElement, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLTextAreaElement.prototype,
      "value",
    )!.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

it("keeps a failed request beside its turn after later chat and a prepared suite", async () => {
  const failed = {
    ...operation("failed-preparation"),
    kind: "message",
    state: "FAILED",
    retryable: true,
    error: { code: "invalid_response", message: "The generated tests could not be validated." },
  };
  const state = session([
    { id: "failed-turn", role: "user", content: "Prepare tests from these rules.", operation_id: failed.id },
    { id: "new-request", role: "user", content: "Prepare this other set of tests." },
    { id: "suite-reply", role: "assistant", content: "The other tests are ready.", artifact_id: "suite" },
    { id: "chat", role: "assistant", content: "Enjoy your coffee." },
  ]);
  state.document.test_journey = true;
  const artifact = {
    id: "suite", kind: "test_suite" as const, title: "Tests", agent_prompt: "Follow the rules.",
    accepted: false, source_message_id: "new-request", proposal_message_id: "suite-reply",
    blueprint: { cases: [{ key: "one", payload: { question: "Question?" } }] },
  };
  state.document.artifacts = [artifact];
  state.operations = [failed, { ...operation("chat"), kind: "message" }];
  const retry = vi.fn();
  await render({ session: state, artifact, testJourney: true, onRetry: retry });
  const turn = container.querySelector('[data-message-id="failed-turn"]')!;
  expect(turn.closest("details")).toBeNull();
  expect(turn.textContent).toContain(failed.error.message);
  await act(async () => button("Retry").click());
  expect(retry).toHaveBeenCalledWith(failed.id);
  await render({ session: structuredClone(state) });
  expect(container.querySelector('[data-message-id="failed-turn"]')?.textContent).toContain(failed.error.message);
});

it("folds older chat while keeping its content and the current tests accessible", async () => {
  const state = session([
    { id: "request", role: "user", content: "Prepare return tests." },
    { id: "prepared", role: "assistant", content: "The return tests are ready.", artifact_id: "suite" },
    ...Array.from({ length: 6 }, (_, index) => ({ id: `chat-${index}`, role: index % 2 ? "assistant" : "user", content: `Later chat ${index}` })),
  ]);
  const artifact = { id: "suite", kind: "test_suite" as const, title: "Returns", agent_prompt: "Follow the policy.",
    accepted: false, source_message_id: "request", proposal_message_id: "prepared",
    blueprint: { cases: [{ key: "return", payload: { question: "Can I return this?" } }] } };
  state.document.artifacts = [artifact];
  await render({ session: state, artifact, testJourney: true });
  const earlier = container.querySelector('[data-message-id="request"]')!.closest("details")!;
  expect(earlier.open).toBe(false);
  await act(async () => earlier.querySelector("summary")!.click());
  expect(earlier.open).toBe(true);
  expect(earlier.textContent).toContain("Prepare return tests.");
  const tests = container.querySelector('[aria-label="Your tests"]')!;
  expect(earlier.contains(tests)).toBe(false);
  expect(container.querySelectorAll('[aria-label="Your tests"]')).toHaveLength(1);
  await act(async () => tests.closest("details")!.querySelector("summary")!.click());
  expect(tests.closest("details")!.open).toBe(true);
  expect(props.onRun).not.toHaveBeenCalled();
});

it("offers a clear manual retry when the provider rate limits a saved request", async () => {
  const failed = {
    ...operation("rate-limited"),
    kind: "message",
    state: "FAILED",
    billing: "RECONCILING",
    retryable: true,
    error: { code: "provider_rate_limit", message: "The selected model's provider is busy. Your request is saved." },
  };
  const state = session([{ id: "rate-limited-turn", role: "user", content: "Prepare tests for this support agent.", operation_id: failed.id }]);
  state.document.test_journey = true;
  state.operations = [failed];
  const retry = vi.fn();
  await render({ session: state, testJourney: true, onRetry: retry });
  expect(container.textContent).toContain("Your request is saved.");
  await act(async () => button("Try again").click());
  expect(retry).toHaveBeenCalledWith(failed.id);
});

it("shows a committed result instead of a stale failure and removes its retry", async () => {
  const completed = {
    ...operation("committed"), kind: "message", state: "FAILED", retryable: true,
    error: { code: "worker_interrupted", message: "The worker stopped." },
    completion_receipt: { action: "prepare_tests", source_message_id: "request", artifact_id: "suite", case_count: 5, changed_case_count: 5 },
  };
  const state = session([{ id: "request", role: "user", content: "Prepare five tests.", operation_id: completed.id }]);
  state.operations = [completed];
  await render({ session: state, onRetry: vi.fn() });
  expect(container.textContent).toContain("5 tests are ready.");
  expect(container.textContent).not.toContain("The worker stopped.");
  expect([...container.querySelectorAll("button")].some(b => b.textContent === "Retry")).toBe(false);
});

it("shows an example without changing the draft; explicit use fills it without sending", async () => {
  await render();
  expect(container.textContent).toContain("Check the AI in your app.");
  expect(container.querySelectorAll("textarea")).toHaveLength(1);
  expect(container.textContent).not.toContain("I have example chats");
  expect(container.textContent).not.toContain("I have the instructions");
  await act(async () => button("See an example").click());
  expect(props.onContent).not.toHaveBeenCalled();
  expect(container.textContent).toContain("Example only");
  await act(async () => button("Use this description").click());
  expect(props.onContent).toHaveBeenCalledWith(exampleAnswer);
  expect(exampleAnswer).toContain("User:");
  expect(exampleAnswer).toContain("Assistant:");
  expect(exampleAnswer).toContain("total budget");
  expect(props.onSend).not.toHaveBeenCalled();
  expect(document.activeElement).toBe(container.querySelector("textarea"));
});

it("shows the pending message before session admission and preserves focus and typed-ahead text", async () => {
  await render({ content: "I built a trip planner." });
  const input = container.querySelector("textarea")!;
  input.focus();
  const pendingMessage = {
    id: "message-one",
    content: "I built a trip planner.",
  };
  await render({
    busy: true,
    pendingMessage,
    pendingLabel: "Sending your message…",
    content: "It should respect the budget.",
  });
  expect(container.querySelectorAll(".vibe-message-user")).toHaveLength(1);
  expect(container.querySelector(".vibe-message-user")?.textContent).toContain(
    pendingMessage.content,
  );
  expect(container.querySelectorAll('[role="status"]')).toHaveLength(1);
  expect(container.querySelector('[role="status"]')?.textContent).toContain(
    "Sending your message…",
  );
  expect(button("Send message").disabled).toBe(true);
  expect(container.querySelector("textarea")).toBe(input);
  expect(input.value).toBe("It should respect the budget.");
  expect(document.activeElement).toBe(input);
  await render({ session: session([{ ...pendingMessage, role: "user" }]) });
  expect(container.querySelectorAll(".vibe-message-user")).toHaveLength(1);
});

it("keeps a blocked instruction proposal accessible without claiming work is running", async () => {
  const artifact = {
    id: "artifact-one",
    kind: "agent_draft" as const,
    title: "Trip planner",
    agent_prompt: "Keep the total within the budget.",
    blueprint: {
      evaluation: {
        examples: ["Plan a trip"],
        success_criteria: "Respect the budget",
      },
    },
    accepted: true,
    source_message_id: "message-one",
  };
  await render({
    session: session([
      { id: "message-one", role: "user", content: "Try these instructions" },
    ]),
    artifact,
    busy: true,
  });
  expect(
    container.querySelector('[aria-label="Proposed check"]'),
  ).not.toBeNull();
  expect(container.querySelector('[role="status"]')?.textContent).toBe("");
  expect(button("Agent instructions")).toBeTruthy();
});

it("holds the direct-check proposal behind one truthful activity state", async () => {
  const data = session([
    { id: "question", role: "user", content: exampleAnswer },
    {
      id: "proposal",
      role: "assistant",
      content: "Prepared rules",
      artifact_id: "artifact-one",
    },
  ]);
  const artifact = {
    id: "artifact-one",
    kind: "conversation_evaluation" as const,
    title: "Budget",
    agent_prompt: "",
    blueprint: {},
    accepted: false,
    source_message_id: "question",
    conversation_evaluation: {
      evidence_set_id: "evidence-one",
      expectations: [{ id: "budget", statement: "Respect the total budget" }],
    },
  };
  await render({
    session: data,
    artifact,
    quickChecking: true,
    busy: true,
    pendingLabel: "Checking your answer…",
  });
  expect(container.querySelector('[aria-label="Proposed check"]')).toBeNull();
  expect(container.querySelectorAll('[role="status"]')).toHaveLength(1);
  expect(container.querySelector(".vibe-message-user")?.textContent).toContain(
    "trip-planning app",
  );
  expect(container.querySelector('[role="log"]')?.textContent).not.toContain(
    "Prepared rules",
  );
});

it("uses the requested run identity while permitting deliberate history selection", async () => {
  const data = session();
  data.operations = [operation("old")];
  await render({ session: data, view: "checks", requestedRunID: "new" });
  expect(container.querySelector("[data-result]")).toBeNull();
  expect(container.querySelector('[role="status"]')).not.toBeNull();
  data.operations = [...data.operations, operation("new")];
  await render({ session: { ...data } });
  expect(
    container.querySelector("[data-result]")?.getAttribute("data-result"),
  ).toBe("new");
  await act(async () => {
    const history = container.querySelector<HTMLSelectElement>(
      'select[aria-label="Result history"]',
    )!;
    history.value = "old";
    history.dispatchEvent(new Event("change", { bubbles: true }));
  });
  expect(
    container.querySelector("[data-result]")?.getAttribute("data-result"),
  ).toBe("old");
  await render({ requestedRunID: "next" });
  expect(container.querySelector("[data-result]")).toBeNull();
  await render({
    session: { ...data, operations: [...data.operations, operation("next")] },
  });
  expect(
    container.querySelector("[data-result]")?.getAttribute("data-result"),
  ).toBe("next");
});

it("keeps the previous transcript hidden while a new-answer check is being admitted", async () => {
  const data = session([
    {
      id: "old-question",
      role: "user",
      content: "The old answer before the fix",
    },
    { id: "old-reply", role: "assistant", content: "The earlier finding" },
  ]);
  data.operations = [operation("old")];
  await render({
    session: data,
    view: "build",
    quickChecking: true,
    busy: true,
    pendingLabel: "Starting your check…",
  });
  expect(container.textContent).not.toContain("The old answer before the fix");
  expect(container.textContent).not.toContain("The earlier finding");
  expect(container.querySelector("[data-result]")).toBeNull();
  expect(container.querySelectorAll('[role="status"]')).toHaveLength(1);
  expect(container.querySelector("textarea")).not.toBeNull();
});

it("sends on Enter while preserving Shift+Enter and IME composition", async () => {
  await render({ content: "Check this answer" });
  const input = container.querySelector("textarea")!;
  await act(async () => {
    input.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Enter",
        shiftKey: true,
        bubbles: true,
      }),
    );
    input.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Enter",
        isComposing: true,
        bubbles: true,
      }),
    );
    input.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Enter",
        keyCode: 229,
        bubbles: true,
      }),
    );
  });
  expect(props.onSend).not.toHaveBeenCalled();
  await act(async () =>
    input.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
    ),
  );
  expect(props.onSend).toHaveBeenCalledTimes(1);
});

it("preserves a reader's position and offers the new response without scrolling them", async () => {
  const data = session([
    { id: "first", role: "user", content: "I built a trip planner" },
  ]);
  await render({ session: data });
  const region = container.querySelector<HTMLElement>("#vibe-scroll-region")!;
  Object.defineProperties(region, {
    scrollHeight: { value: 1000 },
    clientHeight: { value: 400 },
  });
  const end = container.querySelector(".vibe-column")!.lastElementChild!;
  const scroll = vi.fn();
  Object.defineProperty(end, "scrollIntoView", {
    configurable: true,
    value: scroll,
  });
  await act(async () => {
    region.scrollTop = 100;
    region.dispatchEvent(new Event("scroll", { bubbles: true }));
  });
  await render({
    session: {
      ...data,
      document: {
        ...data.document,
        messages: [
          ...data.document.messages,
          {
            id: "reply",
            role: "assistant",
            content: "Paste a trip request and the plan it gave you.",
          },
        ],
      },
    },
  });
  expect(scroll).not.toHaveBeenCalled();
  await act(async () => button("New response").click());
  expect(scroll).toHaveBeenCalledWith({ behavior: "auto", block: "nearest" });
});

it("starts a comparison once, only after fresh evidence arrives with persisted speakers", async () => {
  const onAttach = vi.fn(async () => true);
  const onPrepare = vi.fn();
  let current = evidence("old");
  const draw = () =>
    root.render(
      <EvidenceIntake
        evidence={current}
        busy={false}
        comparing
        autoPrepare
        onAttach={onAttach}
        onPrepare={onPrepare}
        onCancel={vi.fn()}
      />,
    );
  await act(async () => draw());
  expect(onPrepare).not.toHaveBeenCalled();
  await act(async () => button("Edit source").click());
  await type(
    container.querySelector("textarea")!,
    "User: Keep the total under 5000.\nAssistant: The new total is 4500.",
  );
  await act(async () => button("Check new answer").click());
  expect(onPrepare).not.toHaveBeenCalled();
  current = evidence("new", true);
  await act(async () => draw());
  expect(onPrepare).not.toHaveBeenCalled();
  await act(async () => {
    const speaker = container.querySelector<HTMLSelectElement>(
      'select[aria-label="Speaker for answer"]',
    )!;
    speaker.value = "assistant";
    speaker.dispatchEvent(new Event("change", { bubbles: true }));
  });
  expect(onPrepare).not.toHaveBeenCalled();
  await act(async () => button("Save speakers and check").click());
  expect(onPrepare).not.toHaveBeenCalled();
  current = evidence("corrected");
  await act(async () => draw());
  expect(onPrepare).toHaveBeenCalledExactlyOnceWith("corrected");
  current = evidence("unrelated-later-update");
  await act(async () => draw());
  expect(onPrepare).toHaveBeenCalledTimes(1);
});

it("does not start work after a failed evidence submission", async () => {
  const onPrepare = vi.fn();
  const onAttach = vi.fn(async () => false);
  const draw = (source?: EvidenceSet) =>
    root.render(
      <EvidenceIntake
        evidence={source}
        busy={false}
        comparing
        autoPrepare
        onAttach={onAttach}
        onPrepare={onPrepare}
        onCancel={vi.fn()}
      />,
    );
  await act(async () => draw());
  await type(
    container.querySelector("textarea")!,
    "User: Question\nAssistant: Answer",
  );
  await act(async () => button("Check new answer").click());
  await act(async () => draw(evidence("late-snapshot")));
  expect(onPrepare).not.toHaveBeenCalled();
  expect(container.querySelector("textarea")?.value).toContain(
    "User: Question",
  );
});
