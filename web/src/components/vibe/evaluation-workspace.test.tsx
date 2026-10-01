import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  defaultModels,
  type Operation,
  type Session,
} from "@/lib/vibe";
import {
  EvaluationWorkspace,
  exampleAnswer,
  type EvaluationWorkspaceProps,
} from "./evaluation-workspace";

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
    onSelectRun: vi.fn(async (run: string) => { await render({ requestedRunID: run }); }),
    onEdit: vi.fn(async () => true),
    onDirty: vi.fn(),
    onSave: vi.fn(),
    onImport: vi.fn(),
    onSettings: vi.fn(),
    loadEvidence: vi.fn(),
    onAction: vi.fn(),
    onDispute: vi.fn(),
    notice: null,
    preview: null,
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

it("clears copied instruction feedback when the selected historical version changes", async () => {
  const copy = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText: copy },
  });
  const base = {
    id: "v1",
    title: "Returns",
    agent_prompt: "Allow opened returns within 30 days.",
    blueprint: {},
    accepted: true,
    source_message_id: "brief",
  };
  const first = {
    ...base,
    id: "v2",
    parent_id: base.id,
    agent_prompt: "Allow unopened returns within 30 days.",
  };
  const second = {
    ...first,
    id: "v3",
    agent_prompt: "Allow unopened returns within 45 days.",
  };
  const state = session([{ id: "brief", role: "user", content: "Returns agent" }]);
  state.document.artifacts = [base, first, second];
  await render({ session: state, artifact: first });
  await act(async () => button("Copy updated instructions").click());
  expect(copy).toHaveBeenCalledWith(first.agent_prompt);
  expect(container.textContent).toContain("Copied instructions");
  await render({ artifact: second });
  expect(container.textContent).not.toContain("Copied instructions");
  await act(async () => button("Copy updated instructions").click());
  expect(copy).toHaveBeenLastCalledWith(second.agent_prompt);
});

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

it("offers instructions and packs without the retired conversation-upload workflow", async () => {
  const data = session([]);
  data.document.evaluation = { id: data.id, chat_id: data.id, door: "test" };
  data.document.format_version = 1;
  await render({ session: data, twoDoor: true });
  expect(container.textContent).not.toContain("Paste a real conversation");
  await act(async () => button("Paste agent instructions").click());
  expect(container.querySelector('textarea[aria-label="Conversations to check"]')).toBeNull();
  const input = container.querySelector<HTMLTextAreaElement>('#vibe-instructions-source')!;
  await type(input, "Only unopened items qualify.");
  await act(async () => button("Prepare examples").click());
  expect(props.onMessage).toHaveBeenCalledWith(expect.any(String), undefined, "Only unopened items qualify.");
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

it.each(["waiting", "clarifying"])("offers a sample only for a real clarification, not idle banter: %s", async phase => {
 const current = session([{id: "joke", role: "user", content: "let's drink vodka yay"}]);
 current.document.evaluation = {id: current.id, chat_id: current.id, door: "build"};
 current.document.build = {cycle_id: "cycle", phase, clarifications_used: phase === "clarifying" ? 1 : 0};
 await act(async () => root.render(<EvaluationWorkspace {...props} session={current} twoDoor onSample={vi.fn()} />));
 expect(container.textContent?.includes("Use a sample policy")).toBe(phase === "clarifying");
});

it("shows a saved taskless demo offer without starting it", async () => {
 const current = session([{id: "offer-reply", role: "assistant", content: "You can try a sample email assistant."}]);
 current.document.evaluation = {id: current.id, chat_id: current.id, door: "build"};
 current.document.build = {cycle_id: "cycle", phase: "waiting", clarifications_used: 1};
 current.document.conversation_state = {version: 1, brief: {scope_id: "scope", revision: 1, facts: []}, guidance: {}, pending_demo: {id: "offer", scope_id: "scope", origin_message_id: "offer-reply", sample: "email"}};
 const choose = vi.fn();
 await act(async () => root.render(<EvaluationWorkspace {...props} session={current} twoDoor onDemo={choose} />));
 expect(choose).not.toHaveBeenCalled();
 expect(container.textContent).toContain("You can describe your own task instead.");
 await act(async () => button("Try a sample email assistant").click());
 expect(choose).toHaveBeenCalledExactlyOnceWith("offer");
});

it("does not promise a prototype for an unclassified message or casual chat", async () => {
  const current = session([{ id: "greeting", role: "user", content: "hhui", operation_id: "turn" }]);
  current.document.evaluation = { id: current.id, chat_id: current.id, door: "build" };
  current.document.build = { cycle_id: "cycle", phase: "preparing", clarifications_used: 0 };
  const turn: Operation = { ...operation("turn"), kind: "message", state: "QUEUED", source: undefined, scorecard: undefined };
  current.operations = [turn];
  const show = () => act(async () => root.render(<EvaluationWorkspace {...props} session={current} twoDoor busy />));
  for (const state of ["QUEUED", "RUNNING", "FINALIZING"] as const) {
    turn.state = state;
    await show();
    expect(container.textContent).not.toMatch(/first version|three made-up|Creating your prototype|Saving your results|Waiting to start/);
    expect(button("Stop")).toBeTruthy();
  }
  turn.state = "RUNNING";
  turn.conversation_decision = { intent: "chat", source_message_id: "greeting" };
  await show();
  expect(container.textContent).toContain("Replying…");
  expect(container.querySelector(".vibe-build-intro")).toBeNull();
  turn.state = "COMPLETED";
  current.document.build.phase = "waiting";
  current.document.messages.push({ id: "reply", role: "assistant", content: "Hi! What would you like help with?", operation_id: "turn" });
  await show();
  expect(container.textContent).toContain("Hi! What would you like help with?");
  expect(container.querySelector(".vibe-build-intro")).toBeNull();
});

it("announces prototype creation only after a confirmed build decision", async () => {
  const current = session([{ id: "task", role: "user", content: "Sort customer emails.", operation_id: "turn" }]);
  current.document.evaluation = { id: current.id, chat_id: current.id, door: "build" };
  current.document.build = { cycle_id: "cycle", phase: "preparing", clarifications_used: 0 };
  current.operations = [{ ...operation("turn"), kind: "message", state: "RUNNING", source: undefined, scorecard: undefined,
    conversation_decision: { intent: "prepare_tests", source_message_id: "task" } }];
  await act(async () => root.render(<EvaluationWorkspace {...props} session={current} twoDoor busy />));
  expect(container.querySelector(".vibe-build-intro")?.textContent).toContain("I’m making a first version");
  expect(container.textContent).toContain("Creating your prototype…");
});
