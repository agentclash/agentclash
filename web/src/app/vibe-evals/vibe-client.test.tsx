import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, VibeError, type Operation, type Session } from "@/lib/vibe";
import { quickCheckClientID } from "@/lib/vibe-quick-check";
import { VibeClient } from "./vibe-client";

const harness = vi.hoisted(() => ({
  params: new URLSearchParams("session=session-one"),
  authLoading: false,
  token: vi.fn(async () => undefined as string | undefined),
  watch: vi.fn(),
  me: vi.fn(),
}));
vi.mock("next/navigation", () => ({ useSearchParams: () => harness.params }));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: harness.token }),
  useAuth: () => ({ loading: harness.authLoading, user: { id: "fixture-user" } }),
}));
vi.mock("@/lib/vibe", async (original) => ({
  ...(await original<typeof import("@/lib/vibe")>()),
  watchVibe: harness.watch,
}));
vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ get: harness.me }),
}));
vi.mock("@/components/vibe/credits-dialog", () => ({
  CreditsDialog: ({ workspace }: { workspace: string }) => (
    <span>Credits for {workspace}</span>
  ),
}));

let container: HTMLDivElement;
let root: Root;
let session: Session;
let snapshot: (value: Session) => void;
let requests: { path: string; method: string; body: Record<string, unknown> }[];
let respond: (path: string, options: RequestInit) => Promise<Response>;
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}
async function render() {
  await act(async () => root.render(<VibeClient />));
}
function button(name: string) {
  if (
    name === "Apply changes" &&
    [...container.querySelectorAll("button")].some(
      (b) => b.textContent === "Save expectations",
    )
  )
    name = "Save expectations";
  if (name === "Run these tests")
    return [...container.querySelectorAll<HTMLButtonElement>("button")].find(
      (b) => /^Run .*examples$/.test(b.textContent || ""),
    )!;
  if (name === "Describe")
    name = [...container.querySelectorAll("button")].some((b) =>
      b.textContent?.includes("Back to Vibe Evals"),
    )
      ? "← Back to Vibe Evals"
      : "Conversation";

  if (name === "Send message")
    return container.querySelector<HTMLButtonElement>(
      "form button[type=submit]",
    )!;
  const found = [
    ...document.querySelectorAll<HTMLButtonElement>("button"),
  ].find(
    (b) =>
      b.getAttribute("aria-label") === name || b.textContent?.trim() === name,
  );
  expect(found, `button: ${name}`).toBeTruthy();
  return found!;
}
async function click(name: string) {
  if (name === "Describe" && composer()) return;
  await act(async () => button(name).click());
}
async function openChecks() {
  const back = [
    ...container.querySelectorAll<HTMLButtonElement>("button"),
  ].find((b) => b.textContent?.includes("Back to Vibe Evals"));
  if (back) await act(async () => back.click());
  const results = [
    ...container.querySelectorAll<HTMLButtonElement>("button"),
  ].find((b) => b.textContent === "Results");
  if (results) await act(async () => results.click());
}
async function type(value: string, label = "Message Vibe Evals") {
  const input = container.querySelector<HTMLTextAreaElement>(
    `textarea[aria-label="${label}"]`,
  )!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLTextAreaElement.prototype,
      "value",
    )!.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
function composer() {
  return container.querySelector<HTMLTextAreaElement>(
    'textarea[aria-label="Message Vibe Evals"]',
  )!;
}
function posts() {
  return requests.filter((r) => r.path.endsWith("/messages"));
}

beforeEach(() => {
  harness.authLoading = false;
  sessionStorage.clear();
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  Element.prototype.scrollIntoView = vi.fn();
  harness.params = new URLSearchParams("session=session-one");
  harness.token.mockResolvedValue(undefined);
  harness.watch.mockImplementation(
    (_id, _token, signal: AbortSignal, onSnapshot) => {
      snapshot = onSnapshot;
      return new Promise<void>((resolve) =>
        signal.addEventListener("abort", () => resolve()),
      );
    },
  );
  session = {
    id: "session-one",
    revision: 1,
    event_cursor: 1,
    anonymous: true,
    document: {
      models: defaultModels,
      messages: [],
      requirements: [],
      artifacts: [],
    },
    operations: [],
  };
  requests = [];
  respond = async (path) =>
    path === "/saved-checks" ? json([]) : path === "/config"
      ? json({ enabled: true, defaults: defaultModels, models: [] })
      : json(session);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, options: RequestInit = {}) => {
      const path = new URL(url).pathname.replace("/v1/vibe", "");
      requests.push({
        path,
        method: options.method || "GET",
        body: options.body ? JSON.parse(String(options.body)) : {},
      });
      return respond(path, options);
    }),
  );
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

it.each([false, true])("paid local testing permits evaluator choice without sending a request: %s", async (freeOnly) => {
  const alternative = "deepseek/deepseek-v4.1-flash";
  respond = async (path) => path === "/config"
    ? json({ enabled: true, local_testing: true, free_only: freeOnly, defaults: defaultModels,
      models: [{ id: defaultModels.assistant, name: "Original" }, { id: alternative, name: "DeepSeek V4.1 Flash" }] })
    : json(session);
  await render();
  await selectModel("Assistant", alternative);
  await openSettings();
  const evaluator = document.querySelector<HTMLSelectElement>('select[aria-label="Evaluator model"]')!;
  expect(evaluator.disabled).toBe(freeOnly);
  expect(document.body.textContent).toContain("Retry with another model");
  if (!freeOnly) {
    await click("Use assistant model for all roles");
    for (const label of ["Assistant", "Agent", "Evaluator"]) {
      expect(document.querySelector<HTMLSelectElement>(`select[aria-label="${label} model"]`)!.value).toBe(alternative);
    }
  } else {
    expect(document.body.textContent).not.toContain("Use assistant model for all roles");
  }
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it.each(["setup", "agent"])(
  "sends the %s composer with Enter and preserves Shift+Enter and IME input",
  async (mode) => {
    if (mode === "agent") scenarioAgent();
    await render();
    if (mode === "agent") await click("Try a message");
    const label =
      mode === "agent" ? "Message your agent" : "Message Vibe Evals";
    await type("I bought an unopened item 10 days ago.", label);
    const input = container.querySelector<HTMLTextAreaElement>(
      `textarea[aria-label="${label}"]`,
    )!;
    for (const options of [
      { shiftKey: true },
      { isComposing: true },
      { keyCode: 229 },
    ]) {
      const event = new KeyboardEvent("keydown", {
        key: "Enter",
        bubbles: true,
        cancelable: true,
        ...options,
      });
      await act(async () => {
        input.dispatchEvent(event);
      });
      expect(event.defaultPrevented).toBe(false);
      expect(posts()).toHaveLength(0);
    }
    const event = new KeyboardEvent("keydown", {
      key: "Enter",
      bubbles: true,
      cancelable: true,
    });
    await act(async () => {
      input.dispatchEvent(event);
    });
    expect(event.defaultPrevented).toBe(true);
    expect(posts()).toHaveLength(1);
    expect(posts()[0].body.content).toBe(
      "I bought an unopened item 10 days ago.",
    );
    expect(posts()[0].body.kind).toBe(
      mode === "agent" ? "playground" : "message",
    );
  },
);

it.each([
  "network",
  "server",
  "unknown-client",
  "mismatched-rejection",
  "unstructured-client",
  "idempotency-conflict",
])(
  "retries the immutable submission after a %s acknowledgement failure and newer SSE without auto-dispatch",
  async (failure) => {
    let accepted: string | undefined;
    let executions = 0;
    respond = async (path, options) => {
      if (!path.endsWith("/messages"))
        return path === "/config"
          ? json({ defaults: defaultModels, models: [] })
          : json(session);
      if (!accepted) {
        accepted = String(options.body);
        executions++;
        session = {
          ...session,
          revision: 2,
          event_cursor: 2,
          document: {
            ...session.document,
            messages: [
              { id: "persisted", role: "user", content: "Original request" },
            ],
          },
        };
        if (failure === "server")
          return json(
            { error: { code: "internal", message: "Response interrupted" } },
            503,
          );
        if (failure === "unknown-client")
          return json(
            {
              error: {
                code: "unknown_admission_outcome",
                message: "Response interrupted",
              },
            },
            400,
          );
        if (failure === "mismatched-rejection")
          return json(
            {
              error: {
                code: "pricing_unavailable",
                message: "Unexpected error status",
              },
            },
            400,
          );
        if (failure === "unstructured-client") return json({}, 400);
        if (failure === "idempotency-conflict")
          return json(
            {
              error: {
                code: "idempotency_conflict",
                message: "An operation already uses this ID",
              },
            },
            409,
          );
        throw new TypeError("Network acknowledgement lost");
      }
      return String(options.body) === accepted
        ? json({ id: "original-operation" }, 202)
        : json(
            {
              error: {
                code: "idempotency_conflict",
                message: "Different submission",
              },
            },
            409,
          );
    };
    await render();
    await type("Original request");
    await click("Send message");
    await act(async () => snapshot(structuredClone(session)));
    expect(posts()).toHaveLength(1);
    await type("My next request");
    await click("Retry submission");
    expect(posts()).toHaveLength(2);
    expect(posts()[1].body).toEqual(posts()[0].body);
    expect(executions).toBe(1);
    expect(composer().value).toBe("My next request");
    expect(container.textContent).not.toContain("Different submission");
  },
);

it.each([
  { name: "rate", failures: [[429, "rate_limit"]] },
  { name: "auth", failures: [[403, "forbidden"]] },
  { name: "profile", failures: [[503, "pricing_unavailable"]] },
  {
    name: "rate/auth/profile",
    failures: [
      [429, "rate_limit"],
      [403, "forbidden"],
      [503, "pricing_unavailable"],
    ],
  },
] as const)(
  "retains earlier uncertainty through $name rejection until explicit idempotent success",
  async ({ failures }) => {
    acceptedDraft();
    const ids = vi.spyOn(crypto, "randomUUID");
    const bodies: string[] = [];
    const admissions = new Map<string, string>();
    const selected = {
      ...defaultModels,
      assistant: "openai/gpt-4o-mini",
      target: "openai/gpt-4.1",
    };
    respond = async (path, options) => {
      if (path === "/config")
        return json({
          defaults: defaultModels,
          models: [
            defaultModels.target,
            selected.assistant,
            selected.target,
          ].map((id) => ({ id, name: id })),
        });
      if (!path.endsWith("/messages")) return json(session);
      const body = String(options.body);
      bodies.push(body);
      const rejection = failures[bodies.length - 2];
      if (rejection) {
        const [status, code] = rejection;
        return json({ error: { code, message: `Blocked by ${code}` } }, status);
      }
      const sub = JSON.parse(body);
      const existing = admissions.get(sub.client_id);
      if (existing)
        return existing === body
          ? json({ id: "original-operation" }, 202)
          : json(
              {
                error: {
                  code: "idempotency_conflict",
                  message: "Changed original payload",
                },
              },
              409,
            );
      admissions.set(sub.client_id, body);
      session = {
        ...session,
        revision: 2,
        event_cursor: 2,
        document: {
          ...session.document,
          models: selected,
          messages: [{ id: sub.client_id, role: "user", content: sub.content }],
        },
        operations: [
          {
            id: "original-operation",
            kind: "message",
            state: "COMPLETED",
            billing: "SETTLED",
            models: selected,
            max_cost_nano_usd: 0,
            actual_cost_nano_usd: 0,
            results: [],
          },
        ],
      };
      throw new TypeError("Network acknowledgement lost");
    };
    await render();
    await selectModel("Assistant", selected.assistant);
    await selectModel("Agent", selected.target);
    await type("Original request");
    await click("Send message");
    expect(posts()[0].body).toEqual({
      client_id: expect.any(String),
      evaluation_first: true,
      quick_check: true,
      revision: 1,
      kind: "message",
      content: "Original request",
      models: selected,
      artifact_id: "artifact-one",
    });
    // A later snapshot must not replace any of the original request's fields.
    session = {
      ...session,
      revision: 3,
      event_cursor: 3,
      document: {
        ...session.document,
        models: defaultModels,
        active_artifact_id: "artifact-two",
        artifacts: [
          ...session.document.artifacts,
          { ...session.document.artifacts[0], id: "artifact-two" },
        ],
      },
    };
    await act(async () => snapshot(structuredClone(session)));
    await type("My next request");
    expect(posts()).toHaveLength(1);
    for (const [index, [, code]] of failures.entries()) {
      await click("Retry submission");
      expect(container.textContent).toContain(`Blocked by ${code}`);
      expect(button("Send message").disabled).toBe(true);
      await openSettings();
      expect(button("Import an evaluation").disabled).toBe(true);
      expect(button("Save agent").disabled).toBe(true);
      for (const label of ["Assistant", "Agent"])
        expect(
          document.querySelector<HTMLSelectElement>(
            `select[aria-label="${label} model"]`,
          )!.disabled,
        ).toBe(true);
      await click("Close");
      expect(button("Retry submission").disabled).toBe(false);
      await act(async () =>
        composer().dispatchEvent(
          new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
        ),
      );
      await act(async () => snapshot(structuredClone(session)));
      expect(posts()).toHaveLength(index + 2);
      expect(composer().value).toBe("My next request");
    }
    await click("Retry submission");
    expect(posts()).toHaveLength(failures.length + 2);
    expect(bodies.every((body) => body === bodies[0])).toBe(true);
    expect(
      posts().every((post) => post.path === "/sessions/session-one/messages"),
    ).toBe(true);
    expect(ids).toHaveBeenCalledTimes(1);
    expect(admissions.size).toBe(1);
    expect(session.operations).toHaveLength(1);
    expect(composer().value).toBe("My next request");
    expect(button("Send message").disabled).toBe(false);
    expect(button("Save agent").disabled).toBe(false);
    expect(container.textContent).not.toContain("Retry submission");
    expect(container.textContent).not.toContain("Changed original payload");
  },
);

it.each(["A different next message", "Original request"])(
  "preserves text typed during a pending POST, including retyping %s",
  async (next) => {
    const response = deferred<Response>();
    respond = async (path) =>
      path.endsWith("/messages")
        ? response.promise
        : path === "/config"
          ? json({ defaults: defaultModels, models: [] })
          : json(session);
    await render();
    await type("Original request");
    await click("Send message");
    expect(posts()).toHaveLength(1);
    await type("");
    await type(next);
    await act(async () =>
      response.resolve(json({ id: "accepted-operation" }, 202)),
    );
    expect(composer().value).toBe(next);
  },
);

it.each(["pending", "failed"])(
  "never creates a replacement session while the URL session GET is %s",
  async (outcome) => {
    const response = deferred<Response>();
    respond = async (path) =>
      path === "/config"
        ? json({ defaults: defaultModels, models: [] })
        : response.promise;
    await render();
    if (outcome === "failed")
      await act(async () =>
        response.reject(new Error("Cannot load this conversation")),
      );
    await type("Do not create another conversation");
    // Exercise keyboard submission as well as the button gate.
    await act(async () =>
      composer().dispatchEvent(
        new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
      ),
    );
    expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
    expect(button("Send message").disabled).toBe(true);
    if (outcome === "pending") {
      await act(async () => response.resolve(json(session)));
      expect(button("Send message").disabled).toBe(false);
    } else {
      expect(container.textContent).toContain("Cannot load this conversation");
      respond = async () => json(session);
      await click("Retry connection");
      expect(button("Send message").disabled).toBe(false);
      expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
    }
  },
);

it("shows the submitted message before admission and preserves a newer draft after rejection", async () => {
  const response = deferred<Response>();
  let retry = false;
  respond = async (path) =>
    path === "/config"
      ? json({ defaults: defaultModels, models: [] })
      : path.endsWith("/messages")
        ? retry
          ? json({ id: "accepted" }, 202)
          : response.promise
        : json(session);
  await render();
  await type("Please check this first answer");
  await click("Send message");
  expect(composer().value).toBe("");
  expect(container.textContent).toContain("Please check this first answer");
  expect(container.textContent).toContain("Sending your message");
  await type("My next thought");
  await act(async () =>
    response.resolve(
      json(
        {
          error: {
            code: "invalid_message",
            message: "Try sending that again.",
          },
        },
        400,
      ),
    ),
  );
  expect(composer().value).toBe("My next thought");
  retry = true;
  await click("Retry unsent message");
  expect(posts()[1].body.content).toBe("Please check this first answer");
  expect(composer().value).toBe("My next thought");
});

it("continues a server-authorized pasted-chat check once and keeps result navigation tied to its operation", async () => {
  const artifactID = "42316676-f6e5-4af9-8912-ce6936d42c3c";
  const response = deferred<Response>();
  respond = async (path) =>
    path === "/config"
      ? json({ defaults: defaultModels, models: [] })
      : path.endsWith("/messages")
        ? response.promise
        : json(session);
  await render();
  session = {
    ...session,
    revision: 2,
    event_cursor: 2,
    document: {
      ...session.document,
      messages: [
        {
          id: "pasted-message",
          role: "user",
          content: "Customer: Hello\nAgent: Hello",
        },
      ],
      active_evidence_id: "pasted-evidence",
      evidence_sets: [
        {
          id: "pasted-evidence",
          label: "Chat",
          raw: "Customer: Hello\nAgent: Hello",
          conversations: [
            {
              key: "chat-1",
              title: "Chat",
              messages: [
                { id: "c1-m1", role: "user", content: "Hello" },
                { id: "c1-m2", role: "assistant", content: "Hello" },
              ],
            },
          ],
        },
      ],
      artifacts: [
        {
          id: artifactID,
          title: "Greeting",
          kind: "conversation_evaluation",
          quick_check: true,
          source_message_id: "pasted-message",
          accepted: false,
          agent_prompt: "",
          blueprint: null,
          conversation_evaluation: {
            evidence_set_id: "pasted-evidence",
            expectations: [
              { id: "rule-1", statement: "Respond to the greeting." },
            ],
          },
        },
      ],
    },
  };
  await act(async () => snapshot(structuredClone(session)));
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body).toMatchObject({
    kind: "check",
    artifact_id: artifactID,
    evidence_set_id: "pasted-evidence",
    approve_artifact: true,
    client_id: quickCheckClientID(artifactID),
  });
  await act(async () => snapshot(structuredClone(session)));
  expect(posts()).toHaveLength(1);
  session.document.artifacts[0].accepted = true;
  session.operations.push({
    id: "quick-run",
    kind: "check",
    state: "RUNNING",
    billing: "RESERVED",
    models: defaultModels,
    max_cost_nano_usd: 0,
    actual_cost_nano_usd: null,
    results: [],
    source: {
      kind: "provided_conversations",
      artifact_id: artifactID,
      evidence_set_id: "pasted-evidence",
      label: "Chat",
    },
  });
  session.revision++;
  session.event_cursor = session.revision;
  await act(async () => response.resolve(json(session.operations[0], 202)));
  expect(window.location.search).toContain("view=checks");
  await act(async () => snapshot(structuredClone(session)));
  expect(posts()).toHaveLength(1);
});

function acceptedDraft() {
  session.document.artifacts = [
    {
      id: "artifact-one",
      title: "Support agent",
      agent_prompt: "Original policy",
      blueprint: { cases: [{ key: "refund" }] },
      accepted: true,
      source_message_id: "message-one",
    },
  ];
  session.document.active_artifact_id = "artifact-one";
}

it("keeps the composer editable after a definite intake rejection", async () => {
  respond = async (path) =>
    path.endsWith("/messages")
      ? json(
          {
            error: {
              code: "invalid_message",
              message: "What should your agent help with?",
            },
          },
          400,
        )
      : path === "/config"
        ? json({ defaults: defaultModels, models: [] })
        : json(session);
  await render();
  await type("Help with a new project");
  await click("Send message");
  expect(composer().value).toBe("Help with a new project");
  expect(container.textContent).toContain("What should your agent help with?");
  expect(container.textContent).not.toContain("Retry submission");
  expect(button("Send message").disabled).toBe(false);
  await openSettings();
  expect(button("Import an evaluation").disabled).toBe(false);
  await click("Close");
  expect(posts()).toHaveLength(1);
});

it("verifies a new session cookie before changing the URL, streaming or sending the brief", async () => {
  harness.params = new URLSearchParams();
  harness.watch.mockClear();
  const replace = vi.spyOn(window.history, "replaceState");
  respond = async (path, options) => {
    if (path === "/config")
      return json({ defaults: defaultModels, models: [] });
    if (path === "/sessions" && options.method === "POST")
      return json(session, 201);
    return json(
      {
        error: {
          code: "forbidden",
          message: "Start a new conversation to create a private trial.",
        },
      },
      403,
    );
  };
  await render();
  const brief = "Build a text receptionist for a bike repair shop.";
  await type(brief);
  await click("Send message");
  expect(container.textContent).toContain(
    "The browser could not keep its private session",
  );
  expect(composer().value).toBe(brief);
  expect(posts()).toHaveLength(0);
  expect(harness.watch).not.toHaveBeenCalled();
  expect(replace).not.toHaveBeenCalled();
  expect(button("Send message").disabled).toBe(false);
  expect(container.textContent).not.toContain("Reconnecting to saved progress");
});

it("stops forbidden event retries and lets an empty inaccessible session keep its unsent brief", async () => {
  harness.watch
    .mockReset()
    .mockRejectedValue(new VibeError("forbidden", "Missing cookie", 403));
  const timers = vi.spyOn(globalThis, "setTimeout");
  vi.spyOn(window.history, "replaceState").mockImplementation(
    (_state, _unused, url) => {
      harness.params = new URL(String(url), "http://localhost").searchParams;
    },
  );
  await render();
  await type("Keep this bike repair receptionist brief");
  expect(container.textContent).toContain(
    "This browser can’t access the saved session",
  );
  expect(container.textContent).not.toContain("Reconnecting to saved progress");
  expect(timers.mock.calls.filter(([, delay]) => delay === 2000)).toHaveLength(
    0,
  );
  expect(harness.watch).toHaveBeenCalledTimes(1);
  expect(button("Send message").disabled).toBe(true);
  await click("Keep message in a new conversation");
  expect(composer().value).toBe("Keep this bike repair receptionist brief");
  expect(harness.params.has("session")).toBe(false);
  expect(container.textContent).not.toContain(
    "This browser can’t access the saved session",
  );
  expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
  expect(button("Send message").disabled).toBe(false);
});

it("keeps an inaccessible accepted draft available while offering an explicit connection retry", async () => {
  acceptedDraft();
  harness.watch
    .mockReset()
    .mockRejectedValue(
      new VibeError("not_found", "Conversation unavailable", 404),
    );
  await render();
  expect(
    container.querySelector('section[aria-label="Proposed check"]'),
  ).not.toBeNull();
  expect(button("Retry connection").disabled).toBe(false);
  expect(container.textContent).not.toContain(
    "Keep message in a new conversation",
  );
  expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
  harness.watch.mockImplementation(
    (_id, _token, signal: AbortSignal, onSnapshot) => {
      onSnapshot(structuredClone(session));
      return new Promise<void>((resolve) =>
        signal.addEventListener("abort", () => resolve()),
      );
    },
  );
  await click("Retry connection");
  expect(container.textContent).not.toContain(
    "This browser can’t access the saved session",
  );
  expect(
    container.querySelector('section[aria-label="Proposed check"]'),
  ).not.toBeNull();
  expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
});

it("separates late-loading preview rules without marking the draft dirty and preserves them on edit", async () => {
  acceptedDraft();
  const rules = "Preview capabilities: text only, no connected actions.\n\n";
  session.document.artifacts[0].agent_prompt =
    rules + "Answer supplied refund questions.";
  const configResponse = deferred<Response>();
  respond = async (path) =>
    path === "/config" ? configResponse.promise : json(session);
  await render();
  const instructions = () =>
    container.querySelector<HTMLTextAreaElement>(
      'textarea[aria-label="Agent instructions"]',
    )!;
  expect(instructions().value).toBe(session.document.artifacts[0].agent_prompt);
  await act(async () =>
    configResponse.resolve(
      json({
        defaults: defaultModels,
        models: [],
        capabilities: [
          {
            id: "text_preview",
            label: "Text preview",
            available: true,
            description: "Supplied text only",
            instructions: rules,
          },
        ],
      }),
    ),
  );
  expect(instructions().value).toBe("Answer supplied refund questions.");
  const disclosure = [...container.querySelectorAll("details")].find(
    (d) => d.querySelector("summary")?.textContent === "How this preview works",
  )!;
  expect(disclosure.open).toBe(false);
  expect(disclosure.textContent).toContain(rules.trim());
  expect(button("Run these tests").disabled).toBe(false);
  expect(container.textContent).not.toContain("Apply changes");
  await type(
    "Ask for the supplied refund policy when it is missing.",
    "Agent instructions",
  );
  expect(button("Run these tests").disabled).toBe(true);
  await click("Apply changes");
  expect(
    requests.filter((r) => r.method === "PATCH").at(-1)?.body.agent_prompt,
  ).toBe(rules + "Ask for the supplied refund policy when it is missing.");
  expect(posts()).toHaveLength(0);
});

it("allows trying an unaccepted agent and editing the legacy shared rule without confirming requirements", async () => {
  acceptedDraft();
  const a = session.document.artifacts[0];
  a.accepted = false;
  a.blueprint = {
    cases: [{ payload: { question: "No relevant sources" } }],
    judges: [
      { key: "behavior", assertion: "Allow no qualifying recommendation" },
    ],
    validators: [{ key: "has_answer" }],
    dimensions: [{}, {}],
  };
  await render();
  expect(container.textContent).not.toContain("Accept this draft");
  await click("Try a message");
  await type("Find relevant sources", "Message your agent");
  await click("Send to agent");
  expect(posts()[0].body).toMatchObject({
    kind: "playground",
    artifact_id: a.id,
    preview_thread_id: expect.any(String),
  });
  expect(posts()[0].body.approve_artifact).toBeUndefined();
  expect(a.accepted).toBe(false);
  await openChecks();
  await click("Edit expectations");
  await type(
    "State uncertainty when evidence is absent",
    "Shared expected behavior",
  );
  expect(button("Save agent").disabled).toBe(true);
  await click("Apply changes");
  expect(
    requests.filter((r) => r.method === "PATCH").at(-1)?.body,
  ).toMatchObject({
    evaluation: {
      examples: ["No relevant sources"],
      success_criteria: "State uncertainty when evidence is absent",
    },
  });
  expect(posts()).toHaveLength(1);
});

it("groups a proposed replacement with its confirmed predecessor and keeps history", async () => {
  session.document.messages = [
    { id: "m2", role: "user", content: "Explain that booking is unavailable." },
  ];
  session.document.requirements = [
    {
      id: "old",
      statement: "Confirm the booking",
      status: "accepted",
      source_message_id: "m1",
    },
    {
      id: "new",
      statement: "Explain that booking is unavailable",
      status: "proposed",
      source_message_id: "m2",
      supersedes_id: "old",
      change: "replace",
    },
    {
      id: "retired",
      statement: "Old duplicate",
      status: "superseded",
      source_message_id: "m0",
    },
  ];
  await render();
  expect(container.textContent).toContain("Previous: Confirm the booking");
  expect(container.textContent).toContain("Requirement history");
  expect(
    [...container.querySelectorAll("button")].filter(
      (b) => b.textContent === "Confirm",
    ),
  ).toHaveLength(1);
  await click("Dismiss");
  expect(
    requests.filter((r) => r.method === "PATCH").at(-1)?.body,
  ).toMatchObject({ requirement_id: "new", status: "rejected" });
});

it("applies edited instructions as a new version and approves them atomically when checks run", async () => {
  acceptedDraft();
  const original = structuredClone(session.document.artifacts[0]);
  respond = async (path, options) => {
    if (path === "/config")
      return json({ defaults: defaultModels, models: [] });
    if (options.method === "PATCH") {
      const body = JSON.parse(String(options.body));
      if (body.revision !== session.revision)
        return json(
          { error: { code: "revision_conflict", message: "Stale edit" } },
          409,
        );
      session.document.artifacts.push({
        ...original,
        id: "artifact-two",
        parent_id: original.id,
        agent_prompt: body.agent_prompt,
        accepted: false,
      });
      session.event_cursor = ++session.revision;
    }
    return json(session);
  };
  await render();
  await click("Agent instructions");
  await type("Edited policy", "Agent instructions");
  for (const name of ["Run these tests", "Try a message", "Save agent"])
    expect(button(name).disabled).toBe(true);
  expect(posts()).toHaveLength(0);
  await click("Apply changes");
  expect(session.document.artifacts[0]).toEqual(original);
  expect(session.document.artifacts[1].accepted).toBe(false);
  expect(button("Try a message").disabled).toBe(false);
  await click("Run these tests");
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body).toMatchObject({
    artifact_id: "artifact-two",
    approve_artifact: true,
    kind: "check",
  });
  expect(requests.filter((r) => r.method === "PATCH")).toHaveLength(1);
});

it("preserves unapplied instructions when the editor closes and requires an explicit discard", async () => {
  acceptedDraft();
  await render();
  await click("Agent instructions");
  await type("Unsaved policy", "Agent instructions");
  await click("Agent instructions");
  await click("Agent instructions");
  expect(
    container.querySelector<HTMLTextAreaElement>(
      'textarea[aria-label="Agent instructions"]',
    )!.value,
  ).toBe("Unsaved policy");
  expect(button("Run these tests").disabled).toBe(true);
  await click("Discard edits");
  expect(
    container.querySelector<HTMLTextAreaElement>(
      'textarea[aria-label="Agent instructions"]',
    )!.value,
  ).toBe("Original policy");
  await click("Run these tests");
  expect(posts()).toHaveLength(1);
});

it.each([
  ["Confirm", "accepted"],
  ["Dismiss", "rejected"],
] as const)(
  "lets a conversational proposal be %s with no draft and no scorecard",
  async (action, status) => {
    respond = async (path, options) => {
      if (path === "/config")
        return json({ defaults: defaultModels, models: [] });
      if (path.endsWith("/messages")) {
        session.document.messages = [
          {
            id: "reply",
            role: "assistant",
            content: "Would you like to explore support use cases?",
          },
        ];
        session.document.requirements = [
          {
            id: "proposal",
            statement: "Explore support use cases",
            status: "proposed",
            source_message_id: "reply",
          },
        ];
        session.revision++;
        session.event_cursor = session.revision;
      }
      if (options.method === "PATCH") {
        const body = JSON.parse(String(options.body));
        if (body.revision !== session.revision)
          return json(
            { error: { code: "revision_conflict", message: "Stale proposal" } },
            409,
          );
        expect(body.requirement_id).toBe("proposal");
        session.document.requirements[0].status = body.status;
        session.revision++;
        session.event_cursor = session.revision;
      }
      return json(session);
    };
    await render();
    await type("Just exploring AI");
    await click("Send message");
    expect(posts()).toHaveLength(1);
    expect(container.textContent).toContain(
      "Would you like to explore support use cases?",
    );
    expect(
      container.querySelector('article[aria-label="Evaluation scorecard"]'),
    ).toBeNull();
    expect(
      container.querySelector('section[aria-label="Proposed check"]'),
    ).toBeNull();
    expect(container.textContent).toContain(
      "Proposed · needs your confirmation",
    );
    await click(action);
    expect(session.document.requirements[0].status).toBe(status);
    expect(container.textContent).not.toContain(
      "Proposed · needs your confirmation",
    );
  },
);

function signedInWorkspace() {
  harness.token.mockResolvedValue("test-access-token");
  harness.me.mockResolvedValue({
    organizations: [
      {
        role: "org_admin",
        workspaces: [
          {
            id: "workspace-one",
            name: "Workspace One",
            role: "workspace_admin",
          },
        ],
      },
    ],
  });
}
async function openSettings() {
  if (!document.querySelector('select[aria-label="Assistant model"]'))
    await click("Settings");
}
async function selectModel(label: string, value: string) {
  await openSettings();
  const input = document.querySelector<HTMLSelectElement>(
    `select[aria-label="${label} model"]`,
  )!;
  await act(async () => {
    input.value = value;
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await click("Close");
}
it.each([
  "pricing_unavailable",
  "hosted_disabled",
  "accounting_unavailable",
  "trial_capacity_reached",
])(
  "allows model correction and saving after an initial explicit %s rejection",
  async (code) => {
    acceptedDraft();
    signedInWorkspace();
    respond = async (path, options) => {
      if (path === "/config")
        return json({
          defaults: defaultModels,
          models: [
            { id: defaultModels.target, name: "Mini" },
            { id: "openai/gpt-4.1", name: "Full" },
          ],
        });
      if (path.endsWith("/messages"))
        return json(
          { error: { code, message: "Execution was not admitted" } },
          503,
        );
      if (path.endsWith("/save")) {
        const body = JSON.parse(String(options.body));
        Object.assign(session, {
          workspace_id: "workspace-one",
          saved_draft_id: "canonical-one",
          saved_artifact_id: body.artifact_id,
          saved_models: body.models,
        });
        session.document.models = body.models;
        session.revision++;
        session.event_cursor = session.revision;
        return json({
          draft_id: "canonical-one",
          workspace_id: "workspace-one",
        });
      }
      return json(session);
    };
    await render();
    await type("Keep this message");
    await click("Send message");
    expect(container.textContent).toContain("Execution was not admitted");
    expect(composer().value).toBe("Keep this message");
    expect(posts()).toHaveLength(1);
    for (const label of ["Assistant", "Agent"]) {
      await openSettings();
      expect(
        document.querySelector<HTMLSelectElement>(
          `select[aria-label="${label} model"]`,
        )!.disabled,
        label,
      ).toBe(false);
      await selectModel(label, "openai/gpt-4.1");
    }
    await openSettings();
    expect(button("Import an evaluation").disabled).toBe(false);
    await click("Close");
    expect(button("Send message").disabled).toBe(false);
    expect(button("Save agent").disabled).toBe(false);
    expect(container.textContent).not.toContain("Retry submission");
    await click("Save agent");
    await click("Save agent and checks");
    const save = requests.find((r) => r.path.endsWith("/save"))!;
    expect(save.body.models).toEqual({
      ...defaultModels,
      assistant: "openai/gpt-4.1",
      target: "openai/gpt-4.1",
    });
    expect(button("Saved").disabled).toBe(true);
    expect(
      container.querySelector(
        'a[href="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]',
      ),
    ).not.toBeNull();
    expect(composer().value).toBe("Keep this message");
    expect(posts()).toHaveLength(1);
  },
);

it("Keep sends the independently selected models without an inference submission", async () => {
  acceptedDraft();
  signedInWorkspace();
  respond = async (path, options) => {
    if (path === "/config")
      return json({
        defaults: defaultModels,
        models: [
          { id: defaultModels.target, name: "Mini" },
          { id: "openai/gpt-4.1", name: "Full" },
        ],
      });
    if (path.endsWith("/save")) {
      const body = JSON.parse(String(options.body));
      Object.assign(session, {
        workspace_id: "workspace-one",
        saved_draft_id: "canonical-one",
        saved_artifact_id: body.artifact_id,
        saved_models: body.models,
      });
      session.document.models = body.models;
      session.revision++;
      session.event_cursor = session.revision;
      return json({ draft_id: "canonical-one", workspace_id: "workspace-one" });
    }
    return json(session);
  };
  await render();
  await selectModel("Agent", "openai/gpt-4.1");
  await click("Save agent");
  await click("Save agent and checks");
  const save = requests.find((r) => r.path.endsWith("/save"))!;
  expect(save.body.models).toEqual({
    ...defaultModels,
    target: "openai/gpt-4.1",
  });
  expect(save.body.artifact_id).toBe("artifact-one");
  expect(button("Saved").disabled).toBe(true);
  expect(posts()).toHaveLength(0);
});

it.each([
  {
    name: "confirmed",
    receipt: { saved_models: defaultModels },
    confirmed: true,
  },
  { name: "legacy null", receipt: { saved_models: null }, confirmed: false },
  { name: "legacy absent", receipt: {}, confirmed: false },
])(
  "hydrates the $name saved artifact link from a session-only URL, clearing it for a new artifact",
  async ({ receipt, confirmed }) => {
    acceptedDraft();
    signedInWorkspace();
    Object.assign(session, {
      anonymous: false,
      workspace_id: "workspace-one",
      saved_draft_id: "canonical-one",
      saved_artifact_id: "artifact-one",
      ...receipt,
    });
    await render();
    const href =
      "/workspaces/workspace-one/challenge-packs/builder/canonical-one";
    expect(container.querySelector(`a[href="${href}"]`)).not.toBeNull();
    expect(container.textContent).toContain("Credits for workspace-one");
    await click("Save agent");
    expect(button(confirmed ? "Saved" : "Save agent and checks").disabled).toBe(
      confirmed,
    );
    session.document.artifacts.push({
      ...session.document.artifacts[0],
      id: "artifact-two",
      accepted: true,
    });
    session.document.active_artifact_id = "artifact-two";
    session.revision++;
    session.event_cursor = session.revision;
    await act(async () => snapshot(structuredClone(session)));
    expect(document.querySelector(`a[href="${href}"]`)).toBeNull();
    expect(button("Save agent and checks").disabled).toBe(false);
    expect(container.textContent).not.toContain(
      "Your agent and checks are saved.",
    );
    expect(
      [...document.querySelectorAll("button")].some(
        (b) => b.textContent === "Saved",
      ),
    ).toBe(false);
    expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
  },
);
it.each([
  ["Assistant", "assistant"],
  ["Agent", "target"],
  ["Evaluator", "evaluator"],
] as const)(
  "clears Saved when the %s model changes, retaining its canonical link after a conflict",
  async (label, role) => {
    acceptedDraft();
    signedInWorkspace();
    Object.assign(session, {
      anonymous: false,
      workspace_id: "workspace-one",
      saved_draft_id: "canonical-one",
      saved_artifact_id: "artifact-one",
      saved_models: defaultModels,
    });
    respond = async (path) =>
      path === "/config"
        ? json({
            defaults: defaultModels,
            models: [
              { id: defaultModels.target, name: "Mini" },
              { id: "openai/gpt-4.1", name: "Full" },
            ],
          })
        : path.endsWith("/save")
          ? json(
              {
                error: {
                  code: "saved_model_conflict",
                  message: "Create a new draft before saving different models.",
                },
              },
              409,
            )
          : json(session);
    await render();
    expect(container.textContent).toContain("Your agent and checks are saved.");
    await selectModel(label, "openai/gpt-4.1");
    expect(container.textContent).not.toContain(
      "Your agent and checks are saved.",
    );
    expect(container.textContent).toContain(
      "Your current model choices differ from those recorded when this agent was saved.",
    );
    await click("Save agent");
    await click("Save agent and checks");
    const dialog = document.querySelector('[role="dialog"]')!;
    expect(dialog.textContent).toContain(
      "Create a new draft before saving different models.",
    );
    expect(
      dialog.querySelector(
        'a[href="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]',
      ),
    ).not.toBeNull();
    expect(button("Save agent and checks").disabled).toBe(false);
    expect(requests.filter((r) => r.path.endsWith("/save"))).toEqual([
      {
        path: "/sessions/session-one/save",
        method: "POST",
        body: {
          revision: 1,
          artifact_id: "artifact-one",
          approve_artifact: true,
          workspace_id: "workspace-one",
          models: { ...defaultModels, [role]: "openai/gpt-4.1" },
        },
      },
    ]);
    await click("Close");
    await selectModel(label, defaultModels[role]);
    expect(container.textContent).toContain("Your agent and checks are saved.");
    await click("Save agent");
    expect(button("Saved").disabled).toBe(true);
    expect(requests.filter((r) => r.path.endsWith("/save"))).toHaveLength(1);
    expect(posts()).toHaveLength(0);
  },
);

it.each([
  { name: "confirmed", receipt: { saved_models: defaultModels } },
  { name: "legacy null", receipt: { saved_models: null } },
  { name: "legacy absent", receipt: {} },
])(
  "keeps dirty instructions gated with a $name canonical association",
  async ({ receipt }) => {
    acceptedDraft();
    signedInWorkspace();
    Object.assign(session, {
      anonymous: false,
      workspace_id: "workspace-one",
      saved_draft_id: "canonical-one",
      saved_artifact_id: "artifact-one",
      ...receipt,
    });
    await render();
    const selector =
      'a[href="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]';
    expect(container.querySelector(selector)).not.toBeNull();
    await click("Agent instructions");
    await type("Unsaved policy", "Agent instructions");
    expect(container.querySelector(selector)).toBeNull();
    expect(container.textContent).not.toContain(
      "Your agent and checks are saved.",
    );
    for (const name of ["Save agent", "Run these tests", "Try a message"]) {
      expect(button(name).disabled).toBe(true);
      await click(name);
    }
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    expect(requests.filter((r) => r.method !== "GET")).toHaveLength(0);
    await click("Discard edits");
    expect(
      container.querySelector<HTMLTextAreaElement>(
        'textarea[aria-label="Agent instructions"]',
      )!.value,
    ).toBe("Original policy");
    expect(container.querySelector(selector)).not.toBeNull();
    expect(button("Save agent").disabled).toBe(false);
  },
);

it.each([
  {
    name: "legacy null",
    receipt: { saved_models: null },
    notice:
      "Saved model choices are unknown. Your current model choices are not confirmed as saved.",
  },
  {
    name: "legacy absent",
    receipt: {},
    notice:
      "Saved model choices are unknown. Your current model choices are not confirmed as saved.",
  },
  {
    name: "mismatched",
    receipt: { saved_models: defaultModels },
    notice:
      "Your current model choices differ from those recorded when this agent was saved.",
  },
])(
  "preserves a known canonical link for $name models, including after a save conflict",
  async ({ receipt, notice }) => {
    acceptedDraft();
    signedInWorkspace();
    // Store.GetSession retains saved_artifact_id for pre-00074 rows even though
    // their model receipt is unknown. The mutable document is not a receipt.
    const selected = { ...defaultModels, target: "openai/gpt-4.1" };
    session.document.models = selected;
    Object.assign(session, {
      anonymous: false,
      workspace_id: "workspace-one",
      saved_draft_id: "canonical-one",
      saved_artifact_id: "artifact-one",
      ...receipt,
    });
    const original = structuredClone(session);
    const conflict =
      "These instructions are already saved with different or unrecorded model choices. Open the saved draft in your workspace, or edit and accept a new draft before saving new choices.";
    respond = async (path) =>
      path === "/config"
        ? json({
            defaults: defaultModels,
            models: [defaultModels.target, selected.target].map((id) => ({
              id,
              name: id,
            })),
          })
        : path.endsWith("/save")
          ? json(
              { error: { code: "saved_model_conflict", message: conflict } },
              409,
            )
          : json(session);
    const href =
      "/workspaces/workspace-one/challenge-packs/builder/canonical-one";
    const mutations = () => requests.filter((r) => r.method !== "GET");
    async function followLink(scope: ParentNode) {
      const link = scope.querySelector<HTMLAnchorElement>(`a[href="${href}"]`);
      expect(link, "known canonical draft remains navigable").not.toBeNull();
      const before = structuredClone(requests);
      // JSDOM cannot navigate. Cancel only the browser default, not UI handlers.
      document.addEventListener("click", (event) => event.preventDefault(), {
        once: true,
      });
      await act(async () => link!.click());
      expect(requests).toEqual(before);
    }
    await render();
    await followLink(container);
    expect(container.textContent).toContain(notice);
    expect(container.textContent).not.toContain(
      "Your agent and checks are saved.",
    );
    expect(mutations()).toHaveLength(0);
    await click("Save agent");
    const dialog = document.querySelector('[role="dialog"]')!;
    expect(dialog.textContent).toContain(notice);
    expect(
      [...dialog.querySelectorAll("button")].some(
        (b) => b.textContent === "Saved",
      ),
    ).toBe(false);
    await followLink(dialog);
    expect(mutations()).toHaveLength(0);
    await click("Save agent and checks");
    expect(dialog.textContent).toContain(conflict);
    expect(dialog.textContent).toContain(notice);
    expect(button("Save agent and checks").disabled).toBe(false);
    expect(
      [...dialog.querySelectorAll("button")].some(
        (b) => b.textContent === "Saved",
      ),
    ).toBe(false);
    await followLink(dialog);
    expect(mutations()).toEqual([
      { path: "/sessions/session-one/claim", method: "POST", body: {} },
      {
        path: "/sessions/session-one/save",
        method: "POST",
        body: {
          revision: 1,
          artifact_id: "artifact-one",
          approve_artifact: true,
          workspace_id: "workspace-one",
          models: selected,
        },
      },
    ]);
    expect(posts()).toHaveLength(0);
    expect(session).toEqual(original);
    await openSettings();
    expect(
      document.querySelector<HTMLSelectElement>(
        'select[aria-label="Agent model"]',
      )!.value,
    ).toBe(selected.target);
  },
);

it("does not guess a saved artifact association for legacy snapshots", async () => {
  acceptedDraft();
  Object.assign(session, {
    workspace_id: "workspace-one",
    saved_draft_id: "canonical-one",
  });
  await render();
  expect(container.querySelector('a[href*="canonical-one"]')).toBeNull();
});

function scenarioAgent(title = "Text agent") {
  acceptedDraft();
  const artifact = session.document.artifacts[0];
  artifact.accepted = false;
  artifact.title = title;
  artifact.summary = "Answers using supplied information.";
  artifact.blueprint = {
    cases: [
      {
        key: "case-1",
        payload: { question: "A complete task" },
        expectations: [
          {
            key: "expected_behavior",
            kind: "text",
            value: "Use only the supplied information.",
          },
        ],
      },
      {
        key: "case-2",
        payload: { question: "A conflicting task" },
        expectations: [
          {
            key: "expected_behavior",
            kind: "text",
            value: "Explain the conflict before proceeding.",
          },
        ],
      },
      {
        key: "case-3",
        payload: { question: "An incomplete task" },
        expectations: [
          {
            key: "expected_behavior",
            kind: "text",
            value: "Ask for the missing information.",
          },
        ],
      },
    ],
    judges: [
      {
        key: "behavior",
        context_from: ["case.expectations.expected_behavior"],
        assertion:
          "The response meets this case's expected_behavior and the following shared rules:\n\nUse supplied facts.",
      },
    ],
    validators: [{ key: "has_answer" }],
    dimensions: [{}, {}],
  };
  return artifact;
}

it.each([
  "Writing assistant",
  "JSON extractor",
  "Research assistant",
  "Code generator",
])(
  "offers neutral trial messages and individual expectations for a %s",
  async (title) => {
    scenarioAgent(title);
    await render();
    expect(container.querySelector("aside")).toBeNull();
    expect(container.textContent).not.toContain("Accept this draft");
    await click("Try a message");
    expect(
      container.querySelector('textarea[aria-label="Message Vibe Evals"]'),
    ).toBeNull();
    expect(
      container.querySelector('textarea[aria-label="Message your agent"]'),
    ).not.toBeNull();
    await click("Use an example message");
    expect(
      container.querySelector<HTMLTextAreaElement>(
        'textarea[aria-label="Message your agent"]',
      )!.value,
    ).toBe("A complete task");
    expect(posts()).toHaveLength(0);
    await openChecks();
    await click("Edit expectations");
    expect(
      container.querySelector<HTMLTextAreaElement>(
        'textarea[aria-label="Example 3 expected"]',
      )!.value,
    ).toBe("Ask for the missing information.");
    await type("Ask for a specific missing field.", "Example 3 expected");
    await click("Apply changes");
    expect(
      requests.filter((r) => r.method === "PATCH").at(-1)?.body,
    ).toMatchObject({
      evaluation: {
        examples: [],
        scenarios: [
          expect.anything(),
          expect.anything(),
          {
            input: "An incomplete task",
            expected: "Ask for a specific missing field.",
          },
        ],
      },
    });
    expect(posts()).toHaveLength(0);
  },
);

it("keeps Build and Try buffers separate across views and background agent revisions", async () => {
  scenarioAgent();
  await render();
  await type("My unsent agent change");
  await click("Try a message");
  await type("My unsent trial message", "Message your agent");
  await openChecks();
  await click("Describe");
  expect(composer().value).toBe("My unsent agent change");
  await click("Try a message");
  expect(
    container.querySelector<HTMLTextAreaElement>(
      'textarea[aria-label="Message your agent"]',
    )!.value,
  ).toBe("My unsent trial message");
  const next = structuredClone(session);
  next.document.artifacts.push({
    ...next.document.artifacts[0],
    id: "newer-agent",
    title: "Different agent",
  });
  next.revision++;
  next.event_cursor = next.revision;
  await act(async () => snapshot(next));
  expect(container.textContent).toContain(
    "Talking to the agent from these instructions",
  );
  expect(
    container.querySelector<HTMLTextAreaElement>(
      'textarea[aria-label="Message your agent"]',
    )!.value,
  ).toBe("My unsent trial message");
  expect(posts()).toHaveLength(0);
});

it("retries a trial with its original thread and preserves text typed during an uncertain reply", async () => {
  scenarioAgent();
  let first = true;
  respond = async (path) => {
    if (path === "/config")
      return json({ defaults: defaultModels, models: [] });
    if (path.endsWith("/messages") && first) {
      first = false;
      throw new TypeError("Lost acknowledgement");
    }
    return json(session);
  };
  await render();
  await click("Try a message");
  await type("First trial", "Message your agent");
  await click("Send to agent");
  await type("My next trial", "Message your agent");
  await click("Retry submission");
  expect(posts()).toHaveLength(2);
  expect(posts()[0].body).toEqual(posts()[1].body);
  expect(posts()[0].body.preview_thread_id).toEqual(expect.any(String));
  expect(
    container.querySelector<HTMLTextAreaElement>(
      'textarea[aria-label="Message your agent"]',
    )!.value,
  ).toBe("My next trial");
});

it("restores a trial's version and model from its URL and excludes Build from its history", async () => {
  scenarioAgent();
  harness.params = new URLSearchParams(
    "session=session-one&view=try&agent=artifact-one&thread=thread-one",
  );
  session.document.messages = [
    {
      id: "build",
      role: "user",
      origin: "message",
      content: "Private build instructions",
    },
    {
      id: "trial-user",
      role: "user",
      origin: "playground",
      content: "Bought 10 days ago",
      artifact_id: "artifact-one",
      operation_id: "trial-op",
      preview_thread_id: "thread-one",
    },
    {
      id: "trial-reply",
      role: "assistant",
      origin: "playground",
      content: "Is it unopened?",
      artifact_id: "artifact-one",
      operation_id: "trial-op",
      preview_thread_id: "thread-one",
    },
  ];
  session.operations = [
    {
      id: "trial-op",
      kind: "playground",
      state: "COMPLETED",
      billing: "RELEASED",
      models: { ...defaultModels, target: "openai/gpt-4.1" },
      max_cost_nano_usd: 0,
      actual_cost_nano_usd: 0,
      results: [],
    },
  ];
  await render();
  const history = container.querySelector(
    '[role="log"][aria-label="Trial conversation"]',
  )!;
  expect(history.textContent).toContain("Is it unopened?");
  expect(history.textContent).not.toContain("Private build instructions");
  await type("It is unopened", "Message your agent");
  await click("Send to agent");
  expect(posts()[0].body).toMatchObject({
    kind: "playground",
    preview_thread_id: "thread-one",
    artifact_id: "artifact-one",
    models: { ...defaultModels, target: "openai/gpt-4.1" },
  });
  await click("New conversation");
  expect(history.textContent).toBe("");
  expect(posts()).toHaveLength(1);
});

function recordedCheck() {
  const evidence = {
    id: "evidence-one",
    label: "Support chat.txt",
    raw: "Customer: I bought it 10 days ago.\nAgent: Is it unopened?\nCustomer: Yes.\nAgent: When did you buy it?",
    conversations: [
      {
        key: "chat-1",
        title: "Return follow-up",
        messages: [
          {
            id: "c1-m1",
            role: "user" as const,
            content: "I bought it 10 days ago.\n",
          },
          {
            id: "c1-m2",
            role: "assistant" as const,
            content: "Is it unopened?\n",
          },
          { id: "c1-m3", role: "user" as const, content: "Yes.\n" },
          {
            id: "c1-m4",
            role: "assistant" as const,
            content: "When did you buy it?",
          },
        ],
      },
    ],
  };
  session.document.evaluation_first = true;
  session.document.evidence_sets = [evidence];
  session.document.active_evidence_id = evidence.id;
  session.document.artifacts = [
    {
      id: "conversation-check",
      kind: "conversation_evaluation",
      title: "Remember purchase details",
      agent_prompt: "",
      blueprint: null,
      accepted: false,
      source_message_id: "brief",
      conversation_evaluation: {
        evidence_set_id: evidence.id,
        expectations: [
          { id: "memory", statement: "Remember details across follow-ups." },
        ],
      },
    },
  ];
  session.document.messages = [
    {
      id: "brief",
      role: "user",
      content: "Our agent must remember purchase details.",
    },
    {
      id: "proposal",
      role: "assistant",
      content: "Review the expectations before checking.",
      artifact_id: "conversation-check",
    },
  ];
  return evidence;
}
function recordedResult() {
  const evidence = recordedCheck();
  const result = {
    case_key: "chat-1",
    title: "Return follow-up",
    version: "conversation-check",
    input: { conversation: "Return follow-up" },
    output: "",
    expected: "Remember details across follow-ups.",
    expectations: [
      { id: "memory", statement: "Remember details across follow-ups." },
    ],
    messages: evidence.conversations[0].messages,
    verdict: "FAIL" as const,
    checks: [
      {
        key: "memory",
        verdict: "FAIL" as const,
        evidence: "Asked for the purchase age already supplied.",
        message_ids: ["c1-m1", "c1-m4"],
      },
    ],
  };
  const operation = {
    id: "recorded-run",
    kind: "check",
    state: "COMPLETED",
    billing: "RELEASED",
    models: defaultModels,
    max_cost_nano_usd: 0,
    actual_cost_nano_usd: 0,
    source: {
      kind: "provided_conversations" as const,
      label: evidence.label,
      artifact_id: "conversation-check",
      evidence_set_id: evidence.id,
    },
    results: [result],
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
  session.operations = [operation];
  harness.params.set("view", "checks");
  return { evidence, result, operation };
}

it("starts with one action and a read-only example before explicit use", async () => {
  await render();
  expect(container.querySelectorAll('[role="tab"]')).toHaveLength(0);
  expect(container.querySelector("h1")?.textContent).toBe(
    "What should your agent do?",
  );
  expect(button("Send message").disabled).toBe(true);
  await click("See an example");
  expect(composer().value).toBe("");
  expect(posts()).toHaveLength(0);
  await click("Use this description");
  expect(composer().value).toContain("My agent helps customers with returns.");
  expect(composer().value).toContain("within 30 days");
  expect(composer().value).not.toContain("Assistant:");
  expect(document.activeElement).toBe(composer());
  expect(posts()).toHaveLength(0);
});

it("sends a description to test preparation without opting into pasted-answer checking", async () => {
  await render();
  await type("My agent helps customers return unopened orders within 30 days.");
  await click("Send message");
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body.test_journey).toBe(true);
  expect(posts()[0].body.quick_check).toBeUndefined();
  expect(posts()[0].body.instructions).toBeUndefined();
  expect(posts()[0].body.kind).toBe("message");
});

function failedPreparation(): Operation {
  session.document.test_journey = true;
  const failed: Operation = {
    id: "failed-preparation", kind: "message", state: "FAILED", billing: "SETTLED",
    models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [],
    retryable: true,
    error: { code: "invalid_response", message: "I couldn’t prepare the tests. Your request is still here." },
  };
  session.document.messages = [
    { id: "original-request", role: "user", content: "Prepare tests from my return policy.", operation_id: failed.id },
    { id: "later-chat", role: "assistant", content: "Enjoy your coffee." },
  ];
  session.operations = [failed];
  return failed;
}

it("switches a failed request's assistant explicitly and recovers a lost acknowledgement with the same choice", async () => {
  const failed = failedPreparation();
  failed.error = { code: "provider_rate_limit", message: "Provider busy.", retry_available_at: new Date(Date.now() + 60000).toISOString() };
  const alternative = { id: "openai/gpt-5.4-mini", name: "GPT-5.4 Mini", input_nano_per_token: 750, output_nano_per_token: 3750 };
  let attempts = 0;
  respond = async path => {
    if (path === "/config") return json({ enabled: true, defaults: defaultModels, models: [alternative] });
    if (path.endsWith("/retry")) {
      if (++attempts === 1) throw new TypeError("Lost acknowledgement");
      return json({ ...failed, id: "retry-switched", models: { ...defaultModels, assistant: alternative.id }, retry_of_operation_id: failed.id, state: "QUEUED", error: undefined, retryable: false });
    }
    return json(session);
  };
  await render();
  await type("Keep my next question.");
  await click("Retry with another model");
  expect(attempts).toBe(0);
  await click(alternative.name);
  expect(attempts).toBe(1);
  expect(container.textContent).not.toContain("Retry with another model");
  const first = requests.find(request => request.path.endsWith("/retry"))!;
  expect(first.body).toEqual({ client_id: expect.any(String), revision: 1, assistant_model: alternative.id });
  session = { ...session, revision: 4, event_cursor: 4 };
  await act(async () => snapshot(structuredClone(session)));
  await click("Try again");
  const retries = requests.filter(request => request.path.endsWith("/retry"));
  expect(retries).toHaveLength(2);
  expect(retries[1].body).toEqual(first.body);
  expect(composer().value).toBe("Keep my next question.");
  expect(posts()).toHaveLength(0);
});

it("retries an acknowledged failure once without resending a message or clearing typed-ahead text", async () => {
  const failed = failedPreparation();
  const acknowledgement = deferred<Response>();
  const retry = { ...failed, id: "retry-operation", retry_of_operation_id: failed.id, state: "QUEUED", error: undefined, retryable: false };
  respond = async (path) => {
    if (path === "/config") return json({ enabled: true, defaults: defaultModels, models: [] });
    if (path.endsWith("/retry")) return acknowledgement.promise;
    return json(session);
  };
  await render();
  expect(container.querySelectorAll('[role="alert"]')).toHaveLength(1);
  await type("A separate question for later.");
  await act(async () => { button("Retry").click(); button("Retry").click(); });
  await type("My newer question stays here.");
  expect(requests.filter(request => request.path.endsWith("/retry"))).toHaveLength(1);
  const sent = requests.find(request => request.path.endsWith("/retry"))!;
  expect(sent.path).toBe(`/sessions/${session.id}/operations/${failed.id}/retry`);
  expect(sent.body).toEqual({ client_id: expect.any(String), revision: 1 });
  session = { ...session, revision: 2, event_cursor: 2, operations: [failed, retry] };
  await act(async () => acknowledgement.resolve(json(retry)));
  expect(composer().value).toBe("My newer question stays here.");
  expect(posts()).toHaveLength(0);
  expect(container.querySelectorAll('[data-message-id="original-request"]')).toHaveLength(1);
  expect(container.textContent).toContain("Retrying this request…");
});

it("retries an uncertain recovery with its original request after the session revision advances", async () => {
  const failed = failedPreparation();
  let attempts = 0;
  respond = async (path) => {
    if (path === "/config") return json({ enabled: true, defaults: defaultModels, models: [] });
    if (path.endsWith("/retry")) {
      if (++attempts === 1) throw new TypeError("Connection closed before acknowledgement.");
      const retry = { ...failed, id: "retry-operation", retry_of_operation_id: failed.id, state: "QUEUED", error: undefined, retryable: false };
      session = { ...session, operations: [failed, retry] };
      return json(retry);
    }
    return json(session);
  };
  await render();
  await type("Keep this separate question.");
  await click("Retry");
  expect(container.textContent).toContain("The retry acknowledgement wasn’t confirmed.");
  expect(container.textContent).not.toContain("Retry submission");
  session = { ...session, revision: 9, event_cursor: 9 };
  await act(async () => snapshot(structuredClone(session)));
  await click("Retry");
  const retries = requests.filter(request => request.path.endsWith("/retry"));
  expect(retries).toHaveLength(2);
  expect(retries[1].body).toEqual(retries[0].body);
  expect(retries[1].body.revision).toBe(1);
  expect(composer().value).toBe("Keep this separate question.");
  expect(posts()).toHaveLength(0);
});

it("recovers a lost retry acknowledgement from a persisted child operation without another POST", async () => {
  const failed = failedPreparation();
  respond = async (path) => {
    if (path === "/config") return json({ enabled: true, defaults: defaultModels, models: [] });
    if (path.endsWith("/retry")) throw new TypeError("Connection closed.");
    return json(session);
  };
  await render();
  await click("Retry");
  const retry: Operation = { ...failed, id: "retry-operation", retry_of_operation_id: failed.id, state: "COMPLETED", retryable: false, error: undefined,
    completion_receipt: { action: "prepare_tests", source_message_id: "original-request", artifact_id: "tests", case_count: 3, changed_case_count: 3 } };
  session = { ...session, revision: 3, event_cursor: 3, operations: [failed, retry] };
  await act(async () => snapshot(structuredClone(session)));
  await type("Next question.");
  expect(button("Send message").disabled).toBe(false);
  expect(container.textContent).toContain("Completed on retry.");
  expect(container.textContent).not.toContain("acknowledgement wasn’t confirmed");
  expect(requests.filter(request => request.path.endsWith("/retry"))).toHaveLength(1);
});

it.each(["completed", "failed"])("keeps an asynchronous test edit draft until its %s outcome", async outcome => {
  const suite = scenarioAgent("Returns tests");
  suite.kind = "test_suite";
  session.document.test_journey = true;
  const editOperation: Operation = {
    id: "manual-edit", kind: "message", state: "QUEUED", billing: "RESERVED", models: defaultModels,
    max_cost_nano_usd: 0, actual_cost_nano_usd: null, results: [],
  };
  respond = async (path, options) => {
    if (path === "/config") return json({ enabled: true, defaults: defaultModels, models: [] });
    if (options.method === "PATCH") {
      session = { ...session, revision: 2, event_cursor: 2, operations: [editOperation],
        document: { ...session.document, messages: [...session.document.messages,
          { id: "manual-edit-source", role: "user", content: "Update these test expectations.", operation_id: editOperation.id }] } };
    }
    return json(session);
  };
  await render();
  await click("Edit tests");
  const expectation = container.querySelectorAll<HTMLTextAreaElement>('[aria-label="Your tests"] textarea')[1];
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(expectation, "Decline returns after the new deadline.");
    expectation.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await click("Save test changes");
  expect(container.textContent).toContain("Checking these test changes…");
  expect(container.querySelectorAll<HTMLTextAreaElement>('[aria-label="Your tests"] textarea')[1].value).toBe("Decline returns after the new deadline.");
  expect(button("Save test changes").disabled).toBe(true);
  if (outcome === "completed") {
    const updated = { ...suite, id: "updated-tests", parent_id: suite.id, source_message_id: "manual-edit-source" };
    session = { ...session, revision: 3, event_cursor: 3, document: { ...session.document, artifacts: [...session.document.artifacts, updated] },
      operations: [{ ...editOperation, state: "COMPLETED", completion_receipt: { action: "edit_tests", source_message_id: "manual-edit-source", artifact_id: updated.id, case_count: 3, changed_case_count: 1 } }] };
  } else {
    session = { ...session, revision: 3, event_cursor: 3, operations: [{ ...editOperation, state: "FAILED", error: { code: "test_policy_conflict", message: "The expectation contradicts the rule." } }] };
  }
  await act(async () => snapshot(structuredClone(session)));
  expect(container.textContent).not.toContain("Checking these test changes…");
  if (outcome === "completed") {
    expect(container.querySelector('[data-proposal-id="updated-tests"]')).toBeTruthy();
    expect(container.querySelector('[aria-label="Your tests"] textarea')).toBeNull();
  } else {
    expect(container.querySelectorAll<HTMLTextAreaElement>('[aria-label="Your tests"] textarea')[1].value).toBe("Decline returns after the new deadline.");
    expect(container.textContent).toContain("The expectation contradicts the rule.");
    expect(button("Save test changes").disabled).toBe(false);
  }
});

it("prepares tests before collecting actual instructions and preserves the suite when binding them", async () => {
  const suite = scenarioAgent("Returns tests");
  suite.kind = "test_suite";
  suite.agent_prompt = "";
  session.document.test_journey = true;
  const blueprint = structuredClone(suite.blueprint);
  respond = async (path, options) => {
    if (path === "/config")
      return json({ enabled: true, defaults: defaultModels, models: [] });
    if (options.method === "PATCH") {
      const body = JSON.parse(String(options.body));
      const bound = {
        ...suite,
        id: "bound-agent",
        parent_id: suite.id,
        agent_prompt: body.agent_prompt,
      };
      session.document.artifacts.push(bound);
      session.revision++;
    }
    return json(session);
  };
  await render();
  expect(container.textContent).toContain("3 tests are ready");
  expect(
    container.querySelector('textarea[aria-label="Agent instructions"]'),
  ).toBeNull();
  await click("Add your agent");
  expect(container.textContent).toContain(
    "your app’s tools and data aren’t connected",
  );
  expect(posts()).toHaveLength(0);
  await type(
    "Use our supplied return policy. Ask for missing details.",
    "Agent instructions",
  );
  await click("Use these instructions");
  expect(posts()).toHaveLength(0);
  expect(session.document.artifacts.at(-1)?.blueprint).toEqual(blueprint);
  await click("Run 3 tests");
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body).toMatchObject({
    kind: "check",
    artifact_id: "bound-agent",
    approve_artifact: true,
    test_journey: true,
  });
});

it("keeps a generated suite in the canonical pack system without creating an agent or running inference", async () => {
  const suite = scenarioAgent("Returns tests");
  suite.kind = "test_suite";
  suite.agent_prompt = "";
  session.document.test_journey = true;
  signedInWorkspace();
  respond = async (path, options) => {
    if (path === "/config")
      return json({ enabled: true, defaults: defaultModels, models: [] });
    if (path === "/saved-checks") return json([]);
    if (path.endsWith("/save")) {
      const body = JSON.parse(String(options.body));
      suite.accepted = true;
      Object.assign(session, {
        workspace_id: "workspace-one",
        saved_draft_id: "canonical-tests",
        saved_artifact_id: body.artifact_id,
        saved_models: body.models,
      });
      return json({
        draft_id: "canonical-tests",
        workspace_id: "workspace-one",
      });
    }
    return json(session);
  };
  await render();
  await click("Keep these tests");
  await act(async () => {
    const dialog = document.querySelector('[role="dialog"]')!;
    const save = [...dialog.querySelectorAll("button")].find(
      (b) => b.textContent === "Keep these tests",
    )!;
    save.click();
  });
  expect(requests.some((r) => r.path.endsWith("/save-check"))).toBe(false);
  expect(requests.filter((r) => r.path.endsWith("/save"))).toHaveLength(1);
  expect(button("Saved").disabled).toBe(true);
  const reopen = [...document.querySelectorAll("a")].find(
    (a) => a.textContent === "Open saved tests",
  )!;
  expect(reopen.getAttribute("href")).toBe(
    `/vibe-evals?session=${session.id}&agent=${suite.id}&workspace=workspace-one`,
  );
  expect(posts()).toHaveLength(0);
});

it("keeps optional instruction testing behind More options without a source-selection gate", async () => {
  session.document.messages = [
    {
      id: "brief",
      role: "user",
      content: "Our support agent explains returns.",
    },
    { id: "ask", role: "assistant", content: "What can we check today?" },
  ];
  session.document.evaluation_first = true;
  await render();
  const options = [...container.querySelectorAll("details")].find(
    (details) =>
      details.querySelector("summary")?.textContent === "More options",
  )!;
  expect(options).toBeTruthy();
  expect(options.open).toBe(false);
  await act(async () => options.querySelector("summary")!.click());
  expect(
    container.querySelector('article[aria-label="Evaluation scorecard"]'),
  ).toBeNull();
  expect(container.querySelectorAll('[role="tab"]')).toHaveLength(0);
  await click("Try instructions instead");
  await act(async () => {
    const field = container.querySelector<HTMLTextAreaElement>(
      "#vibe-instructions-source",
    )!;
    Object.getOwnPropertyDescriptor(
      HTMLTextAreaElement.prototype,
      "value",
    )!.set!.call(field, "Only explain eligibility. Never process refunds.");
    field.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await click("Prepare examples");
  expect(posts()[0].body).toMatchObject({
    evaluation_first: true,
    instructions: "Only explain eligibility. Never process refunds.",
    kind: "message",
  });
});

it("reveals Conversation and Results only when a provided-chat check starts", async () => {
  recordedCheck();
  respond = async (path, options) => {
    if (path === "/config")
      return json({ defaults: defaultModels, models: [] });
    if (path.endsWith("/messages")) {
      const body = JSON.parse(String(options.body));
      recordedResult();
      session.revision++;
      expect(body).toMatchObject({
        kind: "check",
        artifact_id: "conversation-check",
        evidence_set_id: "evidence-one",
        approve_artifact: true,
      });
      return json(session.operations[0], 202);
    }
    return json(session);
  };
  await render();
  expect(container.querySelectorAll('[role="tab"]')).toHaveLength(0);
  await type("An unsent question");
  await click("Check these chats");
  expect(
    [...container.querySelectorAll('[role="tab"]')].map((t) => t.textContent),
  ).toEqual(["Conversation", "Results"]);
  expect(button("Results").getAttribute("aria-selected")).toBe("true");
  expect(composer().value).toBe("An unsent question");
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body.preview_thread_id).toBeUndefined();
});

it("cites exact chat messages and prepares a disputed rule without silently changing it", async () => {
  const { result } = recordedResult();
  respond = async (path) =>
    path.endsWith("/case")
      ? json(result)
      : path === "/config"
        ? json({ defaults: defaultModels, models: [] })
        : json(session);
  await render();
  const row = container.querySelector<HTMLDetailsElement>(".vibe-result-row")!;
  await act(async () => {
    row.open = true;
    row.dispatchEvent(new Event("toggle"));
  });
  await vi.waitFor(() =>
    expect(container.textContent).toContain(
      "Asked for the purchase age already supplied.",
    ),
  );
  expect(container.textContent).toContain("AI · message 4");
  expect(container.textContent).toContain("When did you buy it?");
  await click("That’s not our rule");
  expect(composer().value).toContain("The correct expectation is:");
  expect(posts()).toHaveLength(0);
  await type("That’s not our rule. Only ask for missing details.");
  await click("Send message");
  expect(posts()[0].body).toMatchObject({
    baseline_id: "recorded-run",
    artifact_id: "conversation-check",
  });
});

it("requests a suggested change on demand with the selected evidence baseline", async () => {
  recordedResult();
  await render();
  await click("Suggest a change");
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body).toMatchObject({
    kind: "message",
    baseline_id: "recorded-run",
    artifact_id: "conversation-check",
  });
  expect(posts()[0].body.content).toContain("do not claim to apply it");
  expect(session.document.artifacts[0].agent_prompt).toBe("");
});

it("uploads updated chats before comparing and retains the original evaluator and check", async () => {
  const { evidence, operation } = recordedResult();
  respond = async (path, options) => {
    if (path === "/config")
      return json({ defaults: defaultModels, models: [] });
    if (path.endsWith("/evidence")) {
      const body = JSON.parse(String(options.body));
      session.document.evidence_sets!.push({
        ...evidence,
        id: "updated-chats",
        raw: body.content,
      });
      session.document.active_evidence_id = "updated-chats";
      session.revision++;
    }
    return json(session);
  };
  await render();
  await click("Compare an update");
  const text = container.querySelector<HTMLTextAreaElement>("#vibe-evidence")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLTextAreaElement.prototype,
      "value",
    )!.set!.call(
      text,
      evidence.raw.replace("When did you buy it?", "It is eligible."),
    );
    text.dispatchEvent(new Event("input", { bubbles: true }));
  });
  expect(posts()).toHaveLength(0);
  await click("Check new answer");
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body).toMatchObject({
    kind: "retest",
    baseline_id: operation.id,
    artifact_id: "conversation-check",
    evidence_set_id: "updated-chats",
    models: operation.models,
  });
});

it("saves a provided-chat check without creating a prompt or agent build", async () => {
  recordedResult();
  harness.token.mockResolvedValue("token");
  harness.me.mockResolvedValue({
    organizations: [
      {
        role: "org_admin",
        workspaces: [
          { id: "workspace-one", name: "Quality", role: "workspace_admin" },
        ],
      },
    ],
  });
  respond = async (path) => {
    if (path === "/config")
      return json({ defaults: defaultModels, models: [] });
    if (path === "/saved-checks") return json([]);
    if (path.endsWith("/save-check"))
      return json({
        id: "saved-check",
        title: "Remember purchase details",
        session_id: session.id,
        artifact_id: "conversation-check",
        baseline_operation_id: "recorded-run",
        workspace_id: "workspace-one",
        source: session.operations[0].source,
      });
    return json(session);
  };
  await render();
  await click("Save this check");
  await act(async () =>
    [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')]
      .find((b) => b.textContent === "Save this check")!
      .click(),
  );
  const write = requests.find((r) => r.path.endsWith("/save-check"));
  expect(write?.body).toMatchObject({
    baseline_operation_id: "recorded-run",
    workspace_id: "workspace-one",
  });
  expect(write?.body).not.toHaveProperty("agent_prompt");
  expect(requests.some((r) => r.path.endsWith("/save"))).toBe(false);
  expect(document.body.textContent).toContain("Open saved check");
});

it("keeps a message unsent until the backend configuration loads and retries without losing it", async () => {
  const ready = deferred<Response>();
  const selected = {
    assistant: "verified/free",
    target: "verified/free",
    evaluator: "verified/free",
  };
  let configLoads = 0;
  harness.params = new URLSearchParams();
  respond = async (path) => {
    if (path === "/config") {
      configLoads++;
      if (configLoads === 1) return ready.promise;
      return json({ defaults: selected, models: [] });
    }
    return json(session);
  };
  await render();
  await type("Please check my app's reply");
  expect(button("Send message").disabled).toBe(true);
  await act(async () => ready.reject(new TypeError("Offline")));
  expect(composer().value).toBe("Please check my app's reply");
  expect(posts()).toHaveLength(0);
  await click("Retry connection");
  expect(button("Send message").disabled).toBe(false);
  await click("Send message");
  expect(posts()[0].body.models).toEqual(selected);
});

it("returns from sign-in to the exact older selection without automatic saving or inference", async () => {
  const original = scenarioAgent("Original tests"); original.kind = "test_suite"; session.document.test_journey = true;
  session.document.artifacts.push({ ...original, id: "newer-version", title: "Newer tests" });
  signedInWorkspace();
  harness.params = new URLSearchParams(`session=session-one&agent=${original.id}&keep=1&workspace=workspace-one`);
  sessionStorage.setItem("vibe-keep:session-one", JSON.stringify({ version: 1, artifact: original.id, content: "An unsent question", models: defaultModels }));
  await render();
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("Keep these tests");
  expect(composer().value).toBe("An unsent question");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
  expect(container.textContent).not.toContain("Newer tests");
});

it("lets the user cancel sign-in without losing the selected tests", async () => {
  const suite = scenarioAgent(); suite.kind = "test_suite"; session.document.test_journey = true;
  await render(); await click("Keep these tests");
  const link = [...document.querySelectorAll("a")].find(a => a.textContent === "Sign in to save your work")!;
  const back = new URL(link.href).searchParams.get("returnTo")!;
  expect(new URL(back, "http://localhost").searchParams.get("agent")).toBe(suite.id);
  expect(back).toContain("keep=1");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
  await click("Close"); expect(composer()).toBeTruthy();
});

it.each(["no access", "expired", "network"])("handles %s while loading save workspaces", async failure => {
  const suite = scenarioAgent(); suite.kind = "test_suite"; session.document.test_journey = true;
  signedInWorkspace();
  if (failure === "no access") harness.me.mockResolvedValue({ organizations: [] });
  else harness.me.mockRejectedValue(Object.assign(new Error("failed"), { status: failure === "expired" ? 401 : 503 }));
  await render(); await click("Keep these tests");
  const dialog = document.querySelector('[role="dialog"]')!;
  expect(dialog.querySelector("select")).toBeNull();
  expect(dialog.textContent).toContain(failure === "no access" ? "workspace you can save to" : failure === "expired" ? "Sign in again" : "Try again");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("keeps a preparation brief without a pack, agent or model call", async () => {
  const brief = scenarioAgent("PDF conversion plan"); brief.kind = "test_plan";
  brief.test_plan = { title: brief.title, objective: "Keep tables", scenarios: [], evidence_needed: ["A converted file"], next_steps: [], local_test_code: "" };
  session.document.test_journey = true; signedInWorkspace();
  respond = async path => path === "/config" ? json({ enabled: true, defaults: defaultModels, models: [] }) : path === "/saved-checks" ? json([]) : path.endsWith("/save-brief") ? json({ kind: "brief", id: "brief-receipt", session_id: session.id, artifact_id: brief.id, workspace_id: "workspace-one", baseline_operation_id: "", title: brief.title }) : json(session);
  await render(); await click("Keep this brief");
  await act(async () => [...document.querySelector('[role="dialog"]')!.querySelectorAll("button")].find(b => b.textContent === "Keep this brief")!.click());
  expect(requests.filter(r => r.path.endsWith("/save-brief"))).toHaveLength(1);
  expect(requests.some(r => r.path.endsWith("/save") || r.path.endsWith("/messages"))).toBe(false);
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("Open saved brief");
});

it("does not substitute the newest tests for a missing saved version", async () => {
  scenarioAgent("Newer tests"); harness.params = new URLSearchParams("session=session-one&agent=missing-version&keep=1");
  await render();
  expect(document.querySelector('[role="dialog"]')).toBeNull();
  expect(container.textContent).toContain("selection is unavailable");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("claims the anonymous conversation when authentication finishes before its event stream connects", async () => {
  const suite=scenarioAgent(); suite.kind="test_suite";session.document.test_journey=true;
  signedInWorkspace();
  harness.watch.mockRejectedValueOnce(new VibeError("not_found","Conversation is unavailable.",404));
  const previous=respond;
  respond=async(path,options)=>{
    if(path.endsWith("/claim")){session={...session,anonymous:false,revision:session.revision+1};return json(session);}
    return previous(path,options);
  };
  await render();
  await act(async()=>{await new Promise(resolve=>setTimeout(resolve,10));});
  expect(requests.filter(r=>r.path.endsWith("/claim"))).toHaveLength(1);
  expect(posts()).toHaveLength(0);
  expect(container.textContent).not.toContain("can’t access the saved session");
});

it("waits for auth loading before requesting a private saved session", async () => {
  signedInWorkspace();harness.authLoading=true;
  await render();
  expect(requests.some(r=>r.path.startsWith("/sessions/"))).toBe(false);
  harness.authLoading=false;
  await render();
  expect(requests.some(r=>r.path==="/sessions/session-one")).toBe(true);
});
