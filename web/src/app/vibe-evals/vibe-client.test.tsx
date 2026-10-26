import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, VibeError, type Operation, type Session } from "@/lib/vibe";
import { draftMemory } from "@/lib/vibe-build-drafts";
import { VibeClient } from "./vibe-client";

const harness = vi.hoisted(() => ({
  params: new URLSearchParams("session=session-one"),
  authLoading: false,
  user: undefined as { id: string; email?: string } | undefined,
  token: vi.fn(async () => undefined as string | undefined),
  watch: vi.fn(),
  me: vi.fn(),
  analytics: vi.fn(),
}));
vi.mock("@/lib/analytics/posthog-client", () => ({ captureWebEvent: harness.analytics, getPostHogSessionID: () => "fixture-analytics" }));
vi.mock("next/navigation", () => ({ useSearchParams: () => harness.params }));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: harness.token }),
  useAuth: () => ({ loading: harness.authLoading, user: harness.user }),
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
let requests: { path: string; query?: string; method: string; body: Record<string, unknown> }[];
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
  if (name === "Try a message") name = "Try it yourself";
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
      : "Agent";

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
  if (name === "Describe" && container.querySelector('[role="tab"][data-active][id="vibe-tab-build"]')) return;
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
  harness.authLoading = false; harness.user = undefined;
  sessionStorage.clear(); draftMemory.clear();
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  Element.prototype.scrollIntoView = vi.fn();
  vi.stubGlobal("CSS", { escape: (value: string) => value });
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
  requests = []; harness.analytics.mockClear();
  respond = async (path) =>
    path === "/saved-checks" ? json([]) : path === "/config"
      ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) })
      : json(session);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, options: RequestInit = {}) => {
      const path = new URL(url).pathname.replace("/v1/vibe", "");
      requests.push({
        path,
        ...(new URL(url).search ? { query: new URL(url).search } : {}),
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
    ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, local_testing: true, free_only: freeOnly, defaults: defaultModels,
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

it.each(["setup", "agent"].flatMap(mode => [false, true].map(touch => ({ mode, touch }))))(
  "sends the $mode composer with touch=$touch and preserves newlines and IME input",
  async ({ mode, touch }) => {
    vi.stubGlobal("matchMedia", vi.fn((query: string) => ({ matches: query === "(any-pointer: coarse)" && touch, addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn() })));
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
      ...(touch ? [{}] : []),
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
      ...(touch ? { ctrlKey: true } : {}),
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
          ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) })
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
          enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels,
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
      expect(button("Import a test pack").disabled).toBe(true);
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
          ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) })
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
        ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) })
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
      respond = async path => path === "/config" ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }] }) : json(session);
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
      ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) })
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
            admission: "rejected",
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
              admission: "rejected",
            },
          },
          400,
        )
      : path === "/config"
        ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) })
        : json(session);
  await render();
  await type("Help with a new project");
  await click("Send message");
  expect(composer().value).toBe("Help with a new project");
  expect(container.textContent).toContain("What should your agent help with?");
  expect(container.textContent).not.toContain("Retry submission");
  expect(button("Send message").disabled).toBe(false);
  await openSettings();
  expect(button("Import a test pack").disabled).toBe(false);
  await click("Close");
  expect(posts()).toHaveLength(1);
});

it("verifies a new session cookie before changing the URL, streaming or sending the brief", async () => {
  harness.params = new URLSearchParams();
  harness.watch.mockClear();
  const replace = vi.spyOn(window.history, "replaceState");
  respond = async (path, options) => {
    if (path === "/config")
      return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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
  expect(replace.mock.calls.every(call => !String(call[2]).includes("session="))).toBe(true);
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
  expect(button("Try it yourself")).toBeTruthy();
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
  expect(button("Try it yourself")).toBeTruthy();
  expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
});

function signedInWorkspace() {
  harness.user = { id: "fixture-user" };
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
          enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels,
          models: [
            { id: defaultModels.target, name: "Mini" },
            { id: "openai/gpt-4.1", name: "Full" },
          ],
        });
      if (path.endsWith("/messages"))
        return json(
          { error: { code, message: "Execution was not admitted", admission: "rejected" } },
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
    expect(button("Import a test pack").disabled).toBe(false);
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
        enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels,
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
            enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels,
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
            enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels,
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
      return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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
  const previous = respond;
  respond = async (path, options) => path === "/config" ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set([...Object.values(defaultModels), "openai/gpt-4.1"])].map(id => ({ id, name: id })) }) : previous(path, options);
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
  expect([...container.querySelectorAll('[role="tab"]')].map(tab => tab.textContent)).toEqual(["Agent", "Tests", "Results"]);
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
    if (path === "/config") return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [alternative] });
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
    if (path === "/config") return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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
    if (path === "/config") return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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
    if (path === "/config") return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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

it.each(["completed", "failed", "completed from pinned run"])("keeps an asynchronous test edit draft until its %s outcome", async outcome => {
  const pinned = outcome === "completed from pinned run";
  const suite = scenarioAgent("Returns tests");
  suite.kind = "test_suite";
  session.document.test_journey = true;
  const historical: Operation = { id: "historical-run", kind: "check", state: "COMPLETED", billing: "RELEASED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [], source: { kind: "generated", artifact_id: suite.id } };
  if (pinned) { session.operations = [historical]; harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${historical.id}&agent=${suite.id}`); }
  const editOperation: Operation = {
    id: "manual-edit", kind: "message", state: "QUEUED", billing: "RESERVED", models: defaultModels,
    max_cost_nano_usd: 0, actual_cost_nano_usd: null, results: [],
  };
  respond = async (path, options) => {
    if (path === "/config") return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
    if (options.method === "PATCH") {
      session = { ...session, revision: 2, event_cursor: 2, operations: [...(pinned ? [historical] : []), editOperation],
        document: { ...session.document, messages: [...session.document.messages,
          { id: "manual-edit-source", role: "user", content: "Update these test expectations.", operation_id: editOperation.id }] } };
    }
    return json(session);
  };
  await render();
  if (pinned) await click("Tests");
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
  if (!pinned) await type("My next guide question");
  if (outcome !== "failed") {
    const updated = { ...suite, id: "updated-tests", parent_id: suite.id, source_message_id: "manual-edit-source" };
    session = { ...session, revision: 3, event_cursor: 3, document: { ...session.document, artifacts: [...session.document.artifacts, updated] },
      operations: [...(pinned ? [historical] : []), { ...editOperation, state: "COMPLETED", completion_receipt: { action: "edit_tests", source_message_id: "manual-edit-source", artifact_id: updated.id, case_count: 3, changed_case_count: 1 } }] };
  } else {
    session = { ...session, revision: 3, event_cursor: 3, operations: [{ ...editOperation, state: "FAILED", error: { code: "test_policy_conflict", message: "The expectation contradicts the rule." } }] };
  }
  await act(async () => snapshot(structuredClone(session)));
  expect(container.textContent).not.toContain("Checking these test changes…");
  if (outcome !== "failed") {
    expect(container.querySelector('[data-proposal-id="updated-tests"]')).toBeTruthy();
    expect(container.querySelector('[aria-label="Your tests"] textarea')).toBeNull();
    if (pinned) { expect(new URL(window.location.href).searchParams.get("run")).toBeNull(); expect(new URL(window.location.href).searchParams.get("view")).toBe("tests"); return; }
    harness.params = new URLSearchParams(`session=${session.id}&agent=${suite.id}`); await render();
    expect(container.querySelector('[aria-label="Your tests"] textarea')).toBeNull();
    expect(composer().value).toBe("My next guide question");
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
      return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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

it.each([false, true])("keeps a generated suite without inference even when unavailable=%s and refreshes only its workspace", async unavailable => {
  const suite = scenarioAgent("Returns tests");
  suite.kind = "test_suite";
  suite.agent_prompt = unavailable ? "Follow the saved policy." : "";
  session.document.test_journey = true;
  signedInWorkspace();
  harness.params.set("workspace", "workspace-one");
  respond = async (path, options) => {
    if (path === "/config")
      return json({ enabled: true, availability: { guest: { available: !unavailable }, workspace: { available: !unavailable } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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
  if (unavailable) expect(button("Run 3 tests").disabled).toBe(true);
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
    `/vibe-evals?session=${session.id}&agent=${suite.id}&workspace=workspace-one&view=agent`,
  );
  expect(requests.filter(r => r.path === "/saved-checks").every(r => r.query === "?workspace=workspace-one")).toBe(true);
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
  expect([...container.querySelectorAll('[role="tab"]')].map(tab => tab.textContent)).toEqual(["Agent", "Tests", "Results"]);
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

it("cites exact chat messages and prepares a disputed rule without silently changing it", async () => {
  const { result } = recordedResult();
  respond = async (path) =>
    path.endsWith("/case")
      ? json(result)
      : path === "/config"
        ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) })
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

it("saves a provided-chat check without creating a prompt or agent build", async () => {
  recordedResult();
  harness.user = { id: "fixture-user" };
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
      return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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

it("keeps the V1 entry visible while configuration loads, fails, and retries", async () => {
  const ready = deferred<Response>();
  harness.params = new URLSearchParams();
  let configLoads = 0;
  respond = async (path) => {
    if (path === "/config") {
      if (++configLoads === 1) return ready.promise;
      return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, two_door: true, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
    }
    return json([]);
  };
  const entry = () => {
    expect(container.textContent).toContain("What would you like AI to handle?");
    expect(container.textContent).not.toContain("What should your agent do?");
    expect(container.querySelector('[aria-label="Agent sidebar"]')).not.toBeNull();
    expect(composer()).toBeNull();
    return [...container.querySelectorAll<HTMLButtonElement>("button")]
      .filter(b => /Build an agent|Improve an existing agent/.test(b.textContent || ""));
  };
  await render();
  const doors = entry();
  expect(doors).toHaveLength(2);
  expect(doors.every(b => b.disabled)).toBe(true);
  await act(async () => doors.forEach(b => b.click()));
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
  await act(async () => ready.reject(new TypeError("Offline")));
  expect(entry().every(b => b.disabled)).toBe(true);
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("Couldn’t connect");
  await click("Retry connection");
  expect(entry().every(b => !b.disabled)).toBe(true);
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("keeps a saved conversation's draft unsent until configuration loads and retries without losing it", async () => {
  const ready = deferred<Response>();
  const selected = {
    assistant: "verified/free",
    target: "verified/free",
    evaluator: "verified/free",
  };
  let configLoads = 0;
  session.document.messages = [{ id: "earlier", role: "user", content: "Check my app." }];
  respond = async (path) => {
    if (path === "/config") {
      configLoads++;
      if (configLoads === 1) return ready.promise;
      return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: selected, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
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
  expect(posts()[0].body.models).toEqual(session.document.models);
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

it("keeps last successful project rows after a refresh failure, distinguishes forbidden, and hides rows on actor change", async () => {
  session.document.evaluation = { id: session.id, chat_id: "chat", door: "test" };
  let result: "success" | "failure" | "forbidden" = "success";
  const earlier = { ...structuredClone(session), id: "earlier-project", document: { ...structuredClone(session.document), artifacts: [{ id: "earlier-artifact", title: "Earlier saved project", accepted: true, source_message_id: "m", agent_prompt: "x", blueprint: {} }] } };
  respond = async path => {
    if (path === "/config") return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, two_door: true, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }] });
    if (path === "/sessions") return result === "success" ? json([earlier]) : json({ error: { code: result, message: result } }, result === "forbidden" ? 403 : 503);
    return json(session);
  };
  await render(); expect(container.querySelector('[data-evaluation-id="earlier-project"]')).toBeTruthy();
  result = "failure"; await click("Refresh projects");
  expect(container.querySelector('[data-evaluation-id="earlier-project"]')).toBeTruthy();
  expect(container.textContent).toContain("Couldn’t refresh saved work");
  result = "forbidden"; await click("Refresh projects");
  expect(container.querySelector('[data-evaluation-id="earlier-project"]')).toBeNull();
  expect(container.textContent).toContain("don’t have access");
  result = "success"; await click("Refresh projects");
  expect(container.querySelector('[data-evaluation-id="earlier-project"]')).toBeTruthy();
  harness.user = { id: "other-actor" }; harness.token.mockResolvedValue(undefined);
  await render();
  expect(container.querySelector('[data-evaluation-id="earlier-project"]')).toBeNull();
  expect(container.textContent).toContain("Sign in to load saved work");
});

it.each(["build", "test"] as const)("blocks unavailable %s admission while retaining readable history and exports", async door => {
  session.document.evaluation = { id: session.id, chat_id: "chat", door };
  session.document.messages = [{ id: "saved", role: "user", content: "Previously saved question" }];
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, defaults: defaultModels, models: [], availability: { guest: { available: false, reason_code: "worker_unavailable" }, workspace: { available: true } } }) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render(); await type("Keep this unsent question");
  expect(button("Send message").disabled).toBe(true);
  expect(container.textContent).toContain("Previously saved question");
  expect(container.textContent).toContain("No approved models");
  await click("New agent");
  expect(button("Back to your evaluation").disabled).toBe(false);
  await click("Back to your evaluation");
  expect(container.textContent).toContain("Previously saved question");
  await click("Settings");
  expect(button("Export conversation").disabled).toBe(false);
  expect(posts()).toHaveLength(0);
});

it("uses server time to expire a run quote and refreshes its estimate without dispatching inference", async () => {
  const suite = scenarioAgent(); suite.kind = "test_suite";
  session.document.artifacts.push({ ...suite, id: "other-version", title: "Other version" });
  harness.params = new URLSearchParams(`session=${session.id}&agent=${suite.id}`);
  session.document.evaluation = { id: session.id, chat_id: "chat", door: "test" };
  let clientNow = Date.now(); vi.spyOn(Date, "now").mockImplementation(() => clientNow);
  const now = clientNow + 86400000;
  let quotes = 0;
  respond = async path => {
    if (path === "/config") return json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }], server_time: new Date(now).toISOString() });
    if (path.endsWith("/run-quote")) { quotes++; return json({ id: `quote-${quotes}`, cases: 3, calls: 6, max_cost_nano_usd: 250000000, server_time: new Date(now).toISOString(), expires_at: new Date(now + (quotes === 1 ? -1 : 600000)).toISOString() }); }
    return json(session);
  };
  await render(); await click("Run 3 tests");
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("estimate expired");
  expect(button("Run 3 examples").disabled).toBe(true);
  await click("Refresh estimate");
  expect(quotes).toBe(2); expect(button("Run 3 examples").disabled).toBe(false);
  harness.params = new URLSearchParams(`session=${session.id}&agent=other-version`); await render();
  expect(button("Run 3 examples").disabled).toBe(true);
  harness.params = new URLSearchParams(`session=${session.id}&agent=${suite.id}`); await render();
  expect(button("Run 3 examples").disabled).toBe(false);
  clientNow += 600001;
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 1100)); });
  expect(button("Run 3 examples").disabled).toBe(true);
  expect(posts()).toHaveLength(0);
});

it.each(["guide", "trial"])("enforces the advertised UTF-8 limit in the %s composer before transport and admits the exact multibyte boundary", async recipient => {
  if (recipient === "trial") scenarioAgent();
  respond = async path => path === "/config" ? json({ enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }], anonymous_limits: { max_message_bytes: 8 } }) : path.endsWith("/messages") ? json({ id: "admitted", kind: "message", state: "QUEUED", models: defaultModels, results: [] }) : json(session);
  await render(); if (recipient === "trial") await click("Try a message");
  const label = recipient === "trial" ? "Message your agent" : "Message Vibe Evals";
  const send = recipient === "trial" ? "Send to agent" : "Send message";
  await type("🙂🙂x", label); await click(send);
  expect(posts()).toHaveLength(0); expect(container.querySelector<HTMLTextAreaElement>(`textarea[aria-label="${label}"]`)!.value).toBe("🙂🙂x");
  expect(container.textContent).toContain("8 byte UTF-8 limit");
  await type("🙂🙂", label); await click(send);
  expect(posts()).toHaveLength(1); expect(posts()[0].body.content).toBe("🙂🙂");
});

it("never falls back to guest ownership or displays the previous actor’s snapshot when account authentication fails", async () => {
  session.document.messages = [{ id: "private", role: "user", content: "FIRST ACTOR PRIVATE HISTORY" }];
  await render(); expect(container.textContent).toContain("FIRST ACTOR PRIVATE HISTORY");
  const requestsBefore = requests.length;
  harness.user = { id: "expired-user" }; harness.token.mockRejectedValue(new Error("expired token"));
  await render();
  expect(container.textContent).not.toContain("FIRST ACTOR PRIVATE HISTORY");
  expect(container.textContent).toContain("Sign in again");
  expect(requests.slice(requestsBefore).some(request => request.path === "/sessions/session-one")).toBe(false);
});

it.each(["build", "test"] as const)("records the %s admission attempt before a transport failure without draft content", async door => {
  scenarioAgent();
  session.document.evaluation = { id: session.id, chat_id: "chat", door };
  session.document.messages = [{ id: "earlier", role: "user", content: "Existing work" }];
  const previous = respond;
  respond = async (path, options) => {
    if (path.endsWith("/messages")) {
      expect(harness.analytics).toHaveBeenCalledWith("web.vibe.attempt", { schema_version: 1, door, action: "message", session_id: session.id });
      throw new TypeError("Offline");
    }
    return previous(path, options);
  };
  await render(); await type("PRIVATE DRAFT CONTENT"); await click("Send message");
  expect(harness.analytics).toHaveBeenCalledWith("web.vibe.failure", { schema_version: 1, door, action: "message", session_id: session.id, error_code: "transport_failed" });
  expect(JSON.stringify(harness.analytics.mock.calls)).not.toContain("PRIVATE DRAFT CONTENT");
  expect(posts()).toHaveLength(1);
});

it.each([
  { anonymous: true, workspaceID: undefined, blocked: true },
  { anonymous: false, workspaceID: "workspace-one", blocked: false },
  { anonymous: false, workspaceID: undefined, blocked: true },
])("uses persisted session funding for anonymous=$anonymous workspace=$workspaceID rather than treating sign-in as workspace funding", async ({ anonymous, workspaceID, blocked }) => {
  scenarioAgent(); signedInWorkspace();
  session.anonymous = anonymous; session.workspace_id = workspaceID;
  respond = async path => path === "/config" ? json({ enabled: true, availability: { guest: { available: false, reason_code: "trial_capacity_reached" }, workspace: { available: true } }, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }] }) : json(session);
  await render(); await type("A question for this saved project");
  expect(button("Send message").disabled).toBe(blocked);
  expect(posts()).toHaveLength(0);
});

it("lets a signed-in user explicitly choose a workspace before either new-project door without dispatching inference", async () => {
  signedInWorkspace(); harness.params = new URLSearchParams();
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, availability: { guest: { available: false }, workspace: { available: true } }, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }] }) : json([]);
  await render();
  const choice = container.querySelector<HTMLSelectElement>('select[aria-label="Workspace for new projects"]')!;
  expect(choice.value).toBe("");
  const doors = [...container.querySelectorAll<HTMLButtonElement>("button")].filter(button => /Build an agent|Improve an existing agent/.test(button.textContent || ""));
  expect(doors.every(button => button.disabled)).toBe(true);
  await act(async () => { choice.value = "workspace-one"; choice.dispatchEvent(new Event("change", { bubbles: true })); });
  expect(doors.every(button => !button.disabled)).toBe(true);
  harness.user = { id: "second-workspace-actor" }; await render();
  const nextChoice = container.querySelector<HTMLSelectElement>('select[aria-label="Workspace for new projects"]')!;
  expect(nextChoice.value).toBe("");
  expect(doors.every(button => button.disabled)).toBe(true);
  expect(requests.filter(request => request.method === "POST")).toHaveLength(0);
});


it("keeps trial history selectable during an inference outage without sending requests", async () => {
  scenarioAgent();
  session.document.messages = ["one", "two"].map(thread => ({ id: thread, role: "assistant" as const, origin: "playground" as const, content: `Saved reply ${thread}`, artifact_id: "artifact-one", preview_thread_id: thread }));
  const previous = respond;
  respond = async (path, options) => path === "/config" ? json({ enabled: true, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Unavailable" }], availability: { guest: { available: false }, workspace: { available: false } } }) : previous(path, options);
  await render(); await click("Try a message");
  const history = container.querySelector<HTMLSelectElement>('select[aria-label="Trial conversation"]')!;
  expect(history.disabled).toBe(false);
  await act(async () => { history.value = "one"; history.dispatchEvent(new Event("change", { bubbles: true })); });
  expect(container.querySelector('[role="log"]')?.textContent).toContain("Saved reply one");
  expect(container.querySelector('[role="log"]')?.textContent).not.toContain("Saved reply two");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it.each(["guest", "workspace-a"])("starts a new workspace project independently of the historical %s funding", async source => {
  scenarioAgent(); signedInWorkspace();
  session.document.evaluation = { id: session.id, chat_id: session.id, door: "test" };
  session.anonymous = source === "guest"; session.workspace_id = source === "guest" ? undefined : "workspace-a";
  session.document.models = { ...defaultModels, target: "retired-model" };
  harness.me.mockResolvedValue({ organizations: [{ role: "org_admin", workspaces: [{ id: "workspace-a", name: "A" }, { id: "workspace-b", name: "B" }] }] });
  const previous = respond;
  respond = async (path, options) => {
    if (path === "/config") return json({ enabled: true, two_door: true, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Approved" }], availability: { guest: { available: false, reason_code: "trial_capacity_reached" }, workspace: { available: true } } });
    if (path === "/sessions" && options.method !== "POST") return json([]);
    if (path === "/sessions" && options.method === "POST") return json({});
    if (path.startsWith("/sessions/") && path !== `/sessions/${session.id}`) return json({ ...session, id: path.split("/").at(-1), anonymous: false, workspace_id: "workspace-b", document: { ...session.document, models: defaultModels, artifacts: [], messages: [], evaluation: { id: path.split("/").at(-1), door: "build" } } });
    return previous(path, options);
  };
  await render(); await click("New agent");
  const choice = container.querySelector<HTMLSelectElement>('select[aria-label="Workspace for new projects"]')!;
  expect([...choice.options].some(o => o.value === "workspace-b")).toBe(true);
  await act(async () => { choice.value = "workspace-b"; choice.dispatchEvent(new Event("change", { bubbles: true })); });
  const door = [...container.querySelectorAll<HTMLButtonElement>("button")].find(b => b.textContent?.includes("Build an agent"))!;
  expect(door.disabled).toBe(false);
  await act(async () => door.click());
  expect(requests.find(r => r.path === "/sessions" && r.method === "POST")?.body).toMatchObject({ door: "build", workspace_id: "workspace-b" });
  expect(posts()).toHaveLength(0);
});

it.each(["create", "cookie"])("records exactly one sanitized failure when session %s fails before message transport", async failure => {
  harness.params = new URLSearchParams();
  const previous = respond;
  respond = async (path, options) => {
    if (path === "/sessions" && options.method === "POST") return failure === "create" ? json({ error: { code: "unavailable", message: "Unavailable" } }, 503) : json({});
    if (path.startsWith("/sessions/")) return json({ error: { code: "not_found", message: "Cookie missing" } }, 404);
    return previous(path, options);
  };
  await render(); await type("PRIVATE INITIAL REQUEST"); await click("Send message");
  expect(harness.analytics.mock.calls.filter(([event]) => event === "web.vibe.attempt")).toHaveLength(1);
  expect(harness.analytics.mock.calls.filter(([event]) => event === "web.vibe.failure")).toHaveLength(1);
  expect(JSON.stringify(harness.analytics.mock.calls)).not.toContain("PRIVATE INITIAL REQUEST");
  expect(posts()).toHaveLength(0);
});

it("records automatic build estimate attempts and failures before admission without duplicating them", async () => {
  session.document.evaluation = { id: session.id, chat_id: session.id, door: "build" };
  const previous = respond;
  respond = async (path, options) => path.endsWith("/build-quote") ? json({ error: { code: "unavailable", message: "Estimate unavailable" } }, 503) : previous(path, options);
  await render(); await type("PRIVATE BUILD DESCRIPTION");
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 450)); });
  expect(harness.analytics.mock.calls.filter(([event]) => event === "web.vibe.attempt")).toEqual([["web.vibe.attempt", { schema_version: 1, door: "build", action: "estimate", session_id: session.id }]]);
  expect(harness.analytics.mock.calls.filter(([event]) => event === "web.vibe.failure")).toEqual([["web.vibe.failure", { schema_version: 1, door: "build", action: "estimate", session_id: session.id, error_code: "unavailable" }]]);
  expect(JSON.stringify(harness.analytics.mock.calls)).not.toContain("PRIVATE BUILD DESCRIPTION");
  expect(posts()).toHaveLength(0);
});

it("restores persisted thread models on Back/Forward without resetting a new conversation's chosen model", async () => {
  scenarioAgent();
  const alternate = "openai/gpt-4.1";
  session.document.messages = ["a", "b"].map(thread => ({ id: thread, role: "assistant" as const, origin: "playground" as const, content: `Reply ${thread}`, artifact_id: "artifact-one", operation_id: `op-${thread}`, preview_thread_id: thread }));
  session.operations = ["a", "b"].map(thread => ({ id: `op-${thread}`, kind: "playground", state: "COMPLETED", billing: "RELEASED", models: { ...defaultModels, target: thread === "a" ? defaultModels.target : alternate }, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }));
  harness.params = new URLSearchParams("session=session-one&view=try&agent=artifact-one&thread=a");
  const previous = respond;
  respond = async (path, options) => path === "/config" ? json({ enabled: true, defaults: defaultModels, models: [defaultModels.target, alternate].map(id => ({ id, name: id })), availability: { guest: { available: true }, workspace: { available: true } } }) : previous(path, options);
  await render();
  const restore = async (thread: string) => act(async () => {
    const url = new URL(window.location.href); thread ? url.searchParams.set("thread", thread) : url.searchParams.delete("thread");
    window.history.replaceState({}, "", url); window.dispatchEvent(new PopStateEvent("popstate"));
  });
  await restore("b"); await restore("a"); await restore("b");
  expect(container.querySelector('[role="log"]')?.textContent).toContain("Reply b");
  await type("Continue B", "Message your agent"); await click("Send to agent");
  expect(posts().at(-1)?.body).toMatchObject({ preview_thread_id: "b", models: { target: alternate } });
  await restore("a"); await type("Continue A", "Message your agent"); await click("Send to agent");
  expect(posts().at(-1)?.body).toMatchObject({ preview_thread_id: "a", models: { target: defaultModels.target } });
  await click("New conversation"); await selectModel("Agent", alternate);
  await act(async () => snapshot({ ...structuredClone(session), revision: 2, event_cursor: 2 }));
  await type("New selected model", "Message your agent"); await click("Send to agent");
  expect(posts().at(-1)?.body).toMatchObject({ models: { target: alternate } });
});

it.each(["success", "failure"])("ignores a prior actor's late message %s while preserving a newer same-session submission", async outcome => {
  scenarioAgent(); signedInWorkspace(); session.anonymous = false; session.workspace_id = "workspace-one";
  harness.params = new URLSearchParams("session=session-one&view=try&agent=artifact-one&thread=shared-thread");
  session.document.messages = [{ id: "prior", role: "assistant", origin: "playground", content: "Saved conversation", artifact_id: "artifact-one", preview_thread_id: "shared-thread" }];
  const old = deferred<Response>(); const current = deferred<Response>(); let sends = 0;
  const previous = respond;
  respond = async (path, options) => path.endsWith("/messages") ? (++sends === 1 ? old.promise : current.promise) : previous(path, options);
  await render(); await type("Old actor request", "Message your agent"); await click("Send to agent");
  expect(posts()).toHaveLength(1);
  harness.user = { id: "new-actor" }; await render();
  await type("New actor request", "Message your agent"); await click("Send to agent");
  expect(posts()).toHaveLength(2);
  await type("New actor typed ahead", "Message your agent");
  const url = window.location.href;
  await act(async () => old.resolve(outcome === "success" ? json({ id: "old-admission", models: defaultModels }) : json({ error: { code: "revision_conflict", message: "OLD ACTOR ERROR" } }, 409)));
  expect(window.location.href).toBe(url);
  expect(container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Message your agent"]')?.value).toBe("New actor typed ahead");
  expect(button("Send to agent").disabled).toBe(true);
  expect(container.textContent).not.toContain("OLD ACTOR ERROR");
  await act(async () => current.resolve(json({ id: "new-admission", models: defaultModels })));
  expect(button("Send to agent").disabled).toBe(false);
  expect(container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Message your agent"]')?.value).toBe("New actor typed ahead");
});

it.each(["success", "failure"])("ignores a prior actor's late retry %s without changing the newer retry or model", async outcome => {
  const failed = failedPreparation(); signedInWorkspace(); session.anonymous = false; session.workspace_id = "workspace-one";
  const old = deferred<Response>(); const current = deferred<Response>(); let retries = 0;
  const alternate = "openai/gpt-4.1";
  const previous = respond;
  respond = async (path, options) => path.endsWith("/retry") ? (++retries === 1 ? old.promise : current.promise) : path === "/config" ? json({ enabled: true, defaults: defaultModels, local_testing: true, models: [defaultModels.target, alternate].map(id => ({ id, name: id })), availability: { guest: { available: true }, workspace: { available: true } } }) : previous(path, options);
  await render();
  await click("Retry with another model"); await click(alternate);
  expect(retries).toBe(1);
  harness.user = { id: "new-retry-actor" }; await render();
  await click("Retry"); expect(retries).toBe(2);
  await type("New actor next guide draft"); const url = window.location.href;
  await act(async () => old.resolve(outcome === "success" ? json({ ...failed, id: "old-retry", models: { ...defaultModels, assistant: alternate } }) : json({ error: { code: "revision_conflict", message: "OLD RETRY ERROR" } }, 409)));
  expect(window.location.href).toBe(url); expect(composer().value).toBe("New actor next guide draft");
  expect(container.textContent).not.toContain("OLD RETRY ERROR");
  expect(button("Retry").disabled).toBe(true);
  await act(async () => current.resolve(json({ ...failed, id: "new-retry", models: defaultModels })));
  await openSettings();
  expect(document.querySelector<HTMLSelectElement>('select[aria-label="Assistant model"]')!.value).toBe(defaultModels.assistant);
  expect(composer().value).toBe("New actor next guide draft");
});

async function historyStep(direction: "back" | "forward") {
  await act(async () => {
    const moved = new Promise<void>(resolve => window.addEventListener("popstate", () => resolve(), { once: true }));
    window.history[direction](); await moved;
  });
}
function projectHistory() {
  const other = { ...structuredClone(session), id: "session-b" };
  other.document.artifacts[0].id = "artifact-b";
  window.history.replaceState({}, "", "/vibe-evals?session=session-b");
  window.history.pushState({}, "", "/vibe-evals?session=session-one");
  const previous = respond;
  respond = async (path, options) => path === "/sessions/session-b" ? json(other) : previous(path, options);
}

it("restores an uncertain project's immutable retry and pending content through browser Back/Forward without automatic transport", async () => {
  scenarioAgent(); projectHistory();
  const bodies: string[] = []; const previous = respond;
  respond = async (path, options) => {
    if (path.endsWith("/messages")) {
      bodies.push(String(options.body));
      if (bodies.length === 1) throw new TypeError("Lost A acknowledgement");
      return json({ id: "acknowledged-a" });
    }
    return previous(path, options);
  };
  await render(); await type("Immutable A request"); await click("Send message");
  await type("A typed ahead");
  expect(button("Retry submission").disabled).toBe(false);
  await historyStep("back"); await type("B unsent draft");
  expect(container.textContent).not.toContain("Immutable A request");
  expect(composer().value).toBe("B unsent draft");
  await historyStep("forward");
  expect(container.textContent).toContain("Immutable A request");
  expect(composer().value).toBe("A typed ahead");
  expect(button("Retry submission").disabled).toBe(false);
  expect(bodies).toHaveLength(1);
  await click("Retry submission");
  expect(bodies).toHaveLength(2); expect(bodies[1]).toBe(bodies[0]);
  expect(posts()[1].body.client_id).toBe(posts()[0].body.client_id);
  expect(composer().value).toBe("A typed ahead");
});

it.each(["success", "failure"])("settles a late project A %s only in A while browser-history B owns a pending request", async outcome => {
  scenarioAgent(); projectHistory();
  const old = deferred<Response>(); const current = deferred<Response>(); const previous = respond;
  const bodies: string[] = [];
  respond = async (path, options) => {
    if (path.endsWith("/messages")) {
      bodies.push(String(options.body));
      if (path === "/sessions/session-one/messages" && bodies.length === 1) return old.promise;
      if (path === "/sessions/session-b/messages") return current.promise;
      return json({ id: "recovered-a" });
    }
    return previous(path, options);
  };
  await render(); await type("A submitted request"); await click("Send message"); await type("A next draft");
  await historyStep("back"); await type("B submitted request"); await click("Send message"); await type("B next draft");
  expect(posts()).toHaveLength(2); const url = window.location.href;
  await act(async () => { outcome === "success" ? old.resolve(json({ id: "a-admitted", models: defaultModels })) : old.reject(new TypeError("A lost acknowledgement")); });
  expect(window.location.href).toBe(url); expect(composer().value).toBe("B next draft");
  expect(button("Send message").disabled).toBe(true);
  expect(container.textContent).not.toContain("A lost acknowledgement");
  await historyStep("forward"); expect(composer().value).toBe("A next draft");
  if (outcome === "failure") { expect(button("Retry submission").disabled).toBe(false); expect(container.textContent).toContain("A submitted request"); }
  else { expect(container.textContent).not.toContain("Retry submission"); expect(button("Send message").disabled).toBe(false); }
  expect(posts()).toHaveLength(2);
  await historyStep("back"); expect(button("Send message").disabled).toBe(true); expect(composer().value).toBe("B next draft");
  await act(async () => current.resolve(json({ id: "b-admitted", models: defaultModels })));
  expect(button("Send message").disabled).toBe(false); expect(composer().value).toBe("B next draft");
  if (outcome === "failure") {
    await historyStep("forward"); await click("Retry submission");
    expect(bodies[2]).toBe(bodies[0]); expect(composer().value).toBe("A next draft");
  }
});

it.each([
  { phase: "creation", changeActor: false },
  { phase: "cookie verification", changeActor: true },
])("releases an inactive first-session $phase attempt and restores explicit content recovery without disturbing another project", async ({ phase, changeActor }) => {
  scenarioAgent();
  const other = { ...structuredClone(session), id: "session-b" };
  other.document.artifacts[0].id = "artifact-b";
  harness.params = new URLSearchParams();
  window.history.replaceState({}, "", "/vibe-evals?session=session-b");
  window.history.pushState({}, "", "/vibe-evals");
  const gate = deferred<Response>(); const pendingB = deferred<Response>();
  const created = new Map<string, Session>(); const previous = respond;
  let firstID = ""; let cookieReads = 0;
  respond = async (path, options) => {
    if (path === "/sessions" && options.method === "POST") {
      const id = JSON.parse(String(options.body)).id as string;
      const project: Session = { ...structuredClone(session), id, document: { models: defaultModels, messages: [], requirements: [], artifacts: [] }, operations: [] };
      created.set(id, project);
      if (!firstID) { firstID = id; if (phase === "creation") return gate.promise; }
      return json(project);
    }
    if (path === "/sessions/session-b") return json(other);
    const project = created.get(path.replace("/sessions/", ""));
    if (project) { if (project.id === firstID && phase === "cookie verification" && ++cookieReads === 1) return gate.promise; return json(project); }
    if (path === "/sessions/session-b/messages") return pendingB.promise;
    if (path.endsWith("/messages")) return json({ id: "first-message-admitted", models: defaultModels });
    return previous(path, options);
  };
  await render(); await type("Recover this first project description"); await click("Send message");
  expect(firstID).not.toBe(""); expect(posts()).toHaveLength(0);
  if (changeActor) { harness.user = { id: "other-preflight-actor" }; harness.token.mockResolvedValue("other-token"); }
  await historyStep("back");
  await type("B's own pending request"); await click("Send message"); await type("B typed ahead");
  expect(posts()).toHaveLength(1); const bURL = window.location.href;
  await act(async () => gate.resolve(phase === "creation" ? json(created.get(firstID)) : json({ error: { code: "forbidden", message: "Cookie was not retained" } }, 403)));
  expect(window.location.href).toBe(bURL); expect(composer().value).toBe("B typed ahead");
  expect(button("Send message").disabled).toBe(true);
  expect(container.textContent).not.toContain("Recover this first project description");
  await historyStep("forward");
  if (changeActor) { expect(container.textContent).not.toContain("Recover this first project description"); harness.user = undefined; await render(); }
  expect(container.textContent).toContain("Recover this first project description");
  expect(button("Retry unsent message").disabled).toBe(false);
  expect(posts()).toHaveLength(1);
  await historyStep("back");
  if (changeActor) { harness.user = { id: "other-preflight-actor" }; await render(); }
  expect(button("Send message").disabled).toBe(true); expect(composer().value).toBe("B typed ahead");
  await act(async () => pendingB.resolve(json({ id: "b-admitted", models: defaultModels })));
  expect(button("Send message").disabled).toBe(false); expect(composer().value).toBe("B typed ahead");
  await historyStep("forward");
  if (changeActor) { harness.user = undefined; await render(); }
  expect(posts()).toHaveLength(1);
  await click("Retry unsent message");
  expect(posts()).toHaveLength(2);
  expect(posts()[1].body.content).toBe("Recover this first project description");
  expect(posts()[1].path).not.toBe("/sessions/session-b/messages");
  expect(new URL(window.location.href).searchParams.get("session")).not.toBe(firstID);
  expect(new URL(window.location.href).searchParams.get("session")).toBe(posts()[1].path.split("/")[2]);
});
