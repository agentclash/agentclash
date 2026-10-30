import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defaultModels, VibeError, type Operation, type Session } from "@/lib/vibe";
import { draftMemory } from "@/lib/vibe-build-drafts";
import { VibeClient } from "./vibe-client";

const harness = vi.hoisted(() => ({
  params: new URLSearchParams("session=session-one"),
  authLoading: false,
  staleRouter: false,
  user: undefined as { id: string; email?: string } | undefined,
  token: vi.fn(async () => undefined as string | undefined),
  watch: vi.fn(),
  me: vi.fn(),
  analytics: vi.fn(),
}));
vi.mock("@/lib/analytics/posthog-client", () => ({ captureWebEvent: harness.analytics, getPostHogSessionID: () => "fixture-analytics" }));
vi.mock("next/navigation", () => ({ useSearchParams: () => harness.staleRouter ? harness.params : new URLSearchParams(window.location.search) }));
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
  CreditsDialog: ({ workspace, open, onOpenChange }: { workspace: string; open: boolean; onOpenChange: (open: boolean) => void }) => open ? (
    <div role="dialog" aria-label="Workspace AI credits">
      <span>Credits for {workspace}</span>
      <button type="button" onClick={() => onOpenChange(false)}>Close credits</button>
    </div>
  ) : null,
}));

const replaceBrowserURL = window.history.replaceState.bind(window.history);
let routerQuery: string | undefined;
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
  // Simulate an external Next route change before mounting. In-app navigation
  // uses the real browser URL, so a stale mock cannot overwrite its selection.
  const query = harness.params.toString();
  if (query !== routerQuery) {
    replaceBrowserURL(null, "", `/vibe-evals?${query}`);
    routerQuery = query;
  }
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
  if (name === "Run these tests") name = "Start run";
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
      b.getAttribute("aria-label") === name || b.textContent?.trim() === name || b.textContent?.trim().startsWith(`${name} · up to $`),
  );
  expect(found, `button: ${name}`).toBeTruthy();
  return found!;
}
async function click(name: string) {
  if (name === "Describe" && container.querySelector('[role="tab"][data-active][id="vibe-tab-build"]')) return;
  // The project Agent tab shows the trial inline, so there is nothing to open.
  if (name === "Try a message" && document.getElementById("vibe-trial-message")) return;
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
    `textarea[aria-label="${label}"]${label === "Message Vibe Evals" ? ", #vibe-start-build" : ""}`,
  ) || [...container.querySelectorAll("textarea")].find(area => [...(area.labels || [])].some(item => item.textContent === label))!;
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
    'textarea[aria-label="Message Vibe Evals"], #vibe-start-build',
  )!;
}
function posts() {
  return requests.filter((r) => r.path.endsWith("/messages"));
}

beforeEach(() => {
  routerQuery = undefined;
  harness.staleRouter = false;
  harness.authLoading = false; harness.user = undefined;
  sessionStorage.clear(); localStorage.clear(); draftMemory.clear();
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
        body: typeof options.body === "string" ? JSON.parse(options.body) : {},
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

it.each([[false,false],[true,false],[false,true]])("saves recorded replies through evidence intake before an explicit pinned quote (lost ack: %s, retired target: %s)", async (lostAck, retiredTarget) => {
  session.document.format_version=1; session.document.evaluation={id:session.id,chat_id:session.id,door:"test"};
  if(retiredTarget)session.document.models={...defaultModels,target:"retired/source-model"};
  const initial=respond;
  let evidencePosts=0;
  respond=async(path,options)=>{
    if(path==="/config") return json({enabled:true,two_door:true,availability:{guest:{available:true},workspace:{available:true}},defaults:defaultModels,models:[...new Set(Object.values(defaultModels))].map(id=>({id,name:id}))});
    if(path.endsWith("/evidence")){
      const body=JSON.parse(String(options.body));
      session={...session,revision:2,event_cursor:2,document:{...session.document,artifacts:[{id:"recorded-version",kind:"conversation_evaluation",title:"App replies",agent_prompt:"",blueprint:null,accepted:false,source_message_id:"",conversation_evaluation:{evidence_set_id:"recorded-source",expectations:body.expectations}}],evidence_sets:[{id:"recorded-source",label:"App replies",raw:body.content,conversations:[{key:"chat-1",title:"Chat 1",messages:[{id:"c1-m1",role:"user",content:"Hello"},{id:"c1-m2",role:"assistant",content:"Exact reply"}]}]}]}};
      evidencePosts++;if(lostAck&&evidencePosts===1)throw new TypeError("Connection closed after commit");
      return json({...session,evidence_receipt:{client_id:body.client_id,evidence_id:"recorded-source",artifact_id:"recorded-version"}});
    }
    if(path.endsWith("/run-quote"))return json({id:"quote-recorded",kind:"run",revision:2,cases:1,calls:1,max_cost_nano_usd:0,expires_at:new Date(Date.now()+600000).toISOString()});
    return initial(path,options);
  };
  await render();
  await click("Test replies from my app");
  expect(requests.filter(r=>r.method==="POST")).toHaveLength(0);
  await type("Customer: Hello\nAgent: Exact reply","Recorded replies");
  await type("Answer without inventing facts.","Expected behavior");
  await click("Review recorded replies");
  if(lostAck){
    await act(async()=>snapshot(session));
    await click("Retry saving these replies");
    const attempts=requests.filter(r=>r.path.endsWith("/evidence"));
    expect(attempts).toHaveLength(2);expect(attempts[1].body).toEqual(attempts[0].body);
  }
  const evidence=requests.find(r=>r.path.endsWith("/evidence"))!;
  expect(evidence.body).toMatchObject({revision:1,content:"Customer: Hello\nAgent: Exact reply",expectations:[{id:"expectation-1",statement:"Answer without inventing facts."}]});
  expect(typeof evidence.body.client_id).toBe("string");
  expect(requests.some(r=>r.path.endsWith("/messages") || r.path.endsWith("/run-quote"))).toBe(false);
  expect(container.textContent).toContain("Exact reply");
  await click("Run recorded tests");
  const cost = document.querySelector('[role="dialog"]')!;
  expect(cost.querySelector('[data-slot="dialog-description"]')?.textContent).toBe("1 test · up to $0.00.");
  expect(cost.textContent).toContain("No new replies are generated and your app is not called.");
  expect(cost.textContent).not.toContain("selected instructions, materials and tests");
  expect(requests.find(r=>r.path.endsWith("/run-quote"))?.body).toMatchObject({artifact_id:"recorded-version",evidence_set_id:"recorded-source",kind:"check"});
  expect(requests.some(r=>r.path.endsWith("/messages"))).toBe(false);
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
      `textarea[aria-label="${label}"]${label === "Message Vibe Evals" ? ", #vibe-start-build" : ""}`,
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
      replaceBrowserURL(_state, _unused, url);
      harness.params = new URL(String(url), "http://localhost").searchParams;
    },
  );
  await render();
  await type("Keep this bike repair receptionist brief");
  expect(container.textContent).toContain(
    "This browser can’t access the saved project",
  );
  expect(container.textContent).not.toContain("Reconnecting to saved progress");
  expect(timers.mock.calls.filter(([, delay]) => delay === 2000)).toHaveLength(
    0,
  );
  expect(harness.watch).toHaveBeenCalledTimes(1);
  expect(button("Send message").disabled).toBe(true);
  await click("Keep message in a new project");
  expect(composer().value).toBe("Keep this bike repair receptionist brief");
  expect(harness.params.has("session")).toBe(false);
  expect(container.textContent).not.toContain(
    "This browser can’t access the saved project",
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
    "Keep message in a new project",
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
    "This browser can’t access the saved project",
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
    await click("Save and iterate");
    const save = requests.find((r) => r.path.endsWith("/save"))!;
    expect(save.body.models).toEqual({
      ...defaultModels,
      assistant: "openai/gpt-4.1",
      target: "openai/gpt-4.1",
    });
    expect(button("Saved").disabled).toBe(true);
    expect(
      container.querySelector(
        'a[href^="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]',
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
  await click("Save and iterate");
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
  "hydrates the $name saved artifact link from a session-only URL, pinning the open save summary while a new artifact arrives",
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
    session.operations=[{id:"original-default-run",kind:"check",state:"COMPLETED",billing:"SETTLED",models:defaultModels,max_cost_nano_usd:0,actual_cost_nano_usd:0,results:[],source:{kind:"prompt",artifact_id:"artifact-one"}}];
    await render();
    const href =
      "/workspaces/workspace-one/challenge-packs/builder/canonical-one";
    expect(container.querySelector(`a[href^="${href}"]`)).not.toBeNull();
    expect(container.textContent).not.toContain("Credits for workspace-one");
    await click("Credits");
    expect(container.textContent).toContain("Credits for workspace-one");
    await click("Close credits");
    await click("Save agent");
    expect(button(confirmed ? "Saved" : "Save and iterate").disabled).toBe(
      confirmed,
    );
    session.document.artifacts.push({
      ...session.document.artifacts[0],
      id: "artifact-two",
      accepted: true,
    });
    session.document.active_artifact_id = "artifact-two";
    session.operations.push({...session.operations[0],id:"later-default-run"});
    session.revision++;
    session.event_cursor = session.revision;
    await act(async () => snapshot(structuredClone(session)));
    expect(container.querySelector(`a[href^="${href}"]`)).toBeNull();
    const returnLink=document.querySelector<HTMLAnchorElement>('[role="dialog"] a[href^="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]')!;
    const returnPath=new URL(returnLink.href).searchParams.get("return_to")!;
    expect(new URL(returnPath,"http://localhost").searchParams.get("agent")).toBe("artifact-one");
    expect(new URL(returnPath,"http://localhost").searchParams.get("run")).toBe("original-default-run");
    expect(button(confirmed ? "Saved" : "Save and iterate").disabled).toBe(confirmed);
    expect(container.textContent).not.toContain(
      "Your agent and tests are saved.",
    );
    expect([...document.querySelectorAll('[role="dialog"] button')].some(b=>b.textContent==="Saved")).toBe(confirmed);
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
    expect(container.textContent).toContain("Your agent and tests are saved.");
    await selectModel(label, "openai/gpt-4.1");
    expect(container.textContent).not.toContain(
      "Your agent and tests are saved.",
    );
    expect(container.textContent).toContain(
      "Your current model choices differ from those recorded when this agent was saved.",
    );
    await click("Save agent");
    await click("Save and iterate");
    const dialog = document.querySelector('[role="dialog"]')!;
    expect(dialog.textContent).toContain(
      "Create a new draft before saving different models.",
    );
    expect(
      dialog.querySelector(
        'a[href^="/workspaces/workspace-one/challenge-packs/builder/canonical-one"]',
      ),
    ).not.toBeNull();
    expect(button("Save and iterate").disabled).toBe(false);
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
    expect(container.textContent).toContain("Your agent and tests are saved.");
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
      const link = scope.querySelector<HTMLAnchorElement>(`a[href^="${href}"]`);
      expect(link, "known canonical draft remains navigable").not.toBeNull();
      expect(new URL(link!.href).pathname).toBe(href);
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
      "Your agent and tests are saved.",
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
    await click("Save and iterate");
    expect(dialog.textContent).toContain(conflict);
    expect(dialog.textContent).toContain(notice);
    expect(button("Save and iterate").disabled).toBe(false);
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
    await click("Use a test message");
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

it("opens the retained trial without a thread selector and preserves explicit New across refresh", async () => {
  scenarioAgent();
  harness.params = new URLSearchParams(
    "session=session-one&view=try&agent=artifact-one",
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
  expect(new URL(location.href).searchParams.get("thread")).toBe("thread-one");
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
  expect(new URL(location.href).searchParams.has("thread")).toBe(true);
  expect(new URL(location.href).searchParams.get("thread")).toBe("");
  harness.params = new URLSearchParams(location.search);
  await act(async () => root.unmount()); root = createRoot(container);
  await render();
  expect(container.querySelector('[role="log"][aria-label="Trial conversation"]')?.textContent).toBe("");
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
  expect(container.textContent).toContain("Reviewing these test changes…");
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
  expect(container.textContent).not.toContain("Reviewing these test changes…");
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
  ).not.toBeNull();
  expect(container.textContent).toContain(
    "your app’s tools and data aren’t connected",
  );
  expect(posts()).toHaveLength(0);
  await type(
    "Use our supplied return policy. Ask for missing details.",
    "Agent instructions",
  );
  await click("Add your agent's instructions");
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
  await click(unavailable ? "Keep these tests" : "Keep for later");
  await act(async () => {
    const dialog = document.querySelector('[role="dialog"]')!;
    const save = [...dialog.querySelectorAll("button")].find(
      (b) => b.textContent === "Save and iterate",
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
  await click("Prepare tests");
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
  // The finding's dispute control is the only one; the per-check copy was a duplicate.
  expect([...container.querySelectorAll("button")].filter(b => b.textContent === "Change expected result")).toHaveLength(1);
  await click("Change expected result");
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
  await click("Save this result");
  await act(async () =>
    [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')]
      .find((b) => b.textContent === "Save and iterate")!
      .click(),
  );
  const write = requests.find((r) => r.path.endsWith("/save-check"));
  expect(write?.body).toMatchObject({
    baseline_operation_id: "recorded-run",
    workspace_id: "workspace-one",
  });
  expect(write?.body).not.toHaveProperty("agent_prompt");
  expect(requests.some((r) => r.path.endsWith("/save"))).toBe(false);
  expect(document.body.textContent).toContain("Open saved result");
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
    expect(container.textContent).toContain("What would you like to evaluate?");
    expect(container.textContent).not.toContain("What should your agent do?");
    expect(container.querySelector('[aria-label="Projects sidebar"]')).toBeNull();
    expect(container.querySelector(".vibe-entry-topbar")).not.toBeNull();
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
  expect(container.textContent).not.toContain("can’t access the saved project");
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
  expect(container.textContent).toContain("Couldn’t refresh your projects");
  result = "forbidden"; await click("Refresh projects");
  expect(container.querySelector('[data-evaluation-id="earlier-project"]')).toBeNull();
  expect(container.textContent).toContain("Couldn’t load your projects");
  result = "success"; await click("Refresh projects");
  expect(container.querySelector('[data-evaluation-id="earlier-project"]')).toBeTruthy();
  harness.user = { id: "other-actor" }; harness.token.mockResolvedValue(undefined);
  await render();
  expect(container.querySelector('[data-evaluation-id="earlier-project"]')).toBeNull();
  expect(container.textContent).toContain("Sign in to load your projects");
});

it.each(["build", "test"] as const)("blocks unavailable %s admission while retaining readable history and exports", async door => {
  session.document.evaluation = { id: session.id, chat_id: "chat", door };
  session.document.messages = [{ id: "saved", role: "user", content: "Previously saved question" }];
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, defaults: defaultModels, models: [], availability: { guest: { available: false, reason_code: "worker_unavailable" }, workspace: { available: true } } }) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render(); await type("Keep this unsent question");
  expect(button("Send message").disabled).toBe(true);
  expect(container.textContent).toContain("Previously saved question");
  expect(container.textContent).toContain("No approved models");
  await click("New project");
  expect(button("Back to your project").disabled).toBe(false);
  await click("Back to your project");
  expect(container.textContent).toContain("Previously saved question");
  await click("Settings");
  expect(button("Export selected version / result").disabled).toBe(false);
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
  expect(button("Start run").disabled).toBe(true);
  await click("Refresh estimate");
  expect(quotes).toBe(2); expect(button("Start run").disabled).toBe(false);
  harness.params = new URLSearchParams(`session=${session.id}&agent=other-version`); await render();
  expect(button("Start run").disabled).toBe(true);
  harness.params = new URLSearchParams(`session=${session.id}&agent=${suite.id}`); await render();
  expect(button("Start run").disabled).toBe(false);
  clientNow += 600001;
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 1100)); });
  expect(button("Start run").disabled).toBe(true);
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

function dialogNames() {
  return [...document.querySelectorAll('[role="dialog"]')].map(dialog =>
    dialog.getAttribute("aria-label") || document.getElementById(dialog.getAttribute("aria-labelledby") || "")?.textContent || "");
}
// Records the most dialogs present together, including one that is still closing.
function watchDialogs() {
  let most = 0;
  const count = () => { most = Math.max(most, document.querySelectorAll('[role="dialog"]').length); };
  const observer = new MutationObserver(count);
  observer.observe(document.body, { childList: true, subtree: true, attributes: true });
  return { most: () => { count(); return most; }, stop: () => observer.disconnect() };
}
async function clickIn(scope: string, name: string) {
  const dialog = document.querySelectorAll<HTMLElement>('[role="dialog"]')[dialogNames().indexOf(scope)];
  const found = [...dialog?.querySelectorAll<HTMLButtonElement>("button") || []].find(b => b.textContent?.trim() === name);
  expect(found, `${scope} button: ${name}`).toBeTruthy();
  await act(async () => found!.click());
}

it("closes Settings before Credits opens, so only one dialog shows at a time", async () => {
  acceptedDraft(); signedInWorkspace();
  Object.assign(session, { anonymous: false, workspace_id: "workspace-one" });
  await render();
  const dialogs = watchDialogs();
  await click("Settings");
  expect(dialogNames()).toEqual(["Settings and details"]);
  await clickIn("Settings and details", "Credits");
  expect(dialogNames()).toEqual(["Workspace AI credits"]);
  expect(container.textContent).toContain("Credits for workspace-one");
  await click("Close credits");
  expect(dialogNames()).toEqual([]);
  await click("Settings");
  const settings = [...document.querySelectorAll<HTMLElement>('[role="dialog"]')].find(dialog => dialog.textContent?.includes("Settings and details"))!;
  const exports = [...settings.querySelectorAll("button")].filter(b => b.textContent?.startsWith("Export"));
  expect(exports.map(b => b.textContent)).toEqual(["Export agent and tests"]);
  // Saving is an action, so it sits with export after the settings, not above them.
  const save = [...settings.querySelectorAll("button")].find(b => b.textContent === "Save agent")!;
  const importPack = [...settings.querySelectorAll("button")].find(b => b.textContent?.includes("Import a test pack"))!;
  expect(save.compareDocumentPosition(importPack) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy();
  await clickIn("Settings and details", "Save agent");
  expect(dialogNames()).toEqual(["Save agent and tests"]);
  expect(dialogs.most()).toBe(1);
  dialogs.stop();
  expect(requests.filter(request => request.method === "POST")).toHaveLength(0);
});

it("closes the Details sheet before Settings opens below 1280px", async () => {
  acceptedDraft();
  session.document.evaluation = { id: session.id, chat_id: "chat", door: "build" };
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id })) }) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  const dialogs = watchDialogs();
  await click("Details");
  expect(dialogNames()).toEqual(["Details"]);
  await clickIn("Details", "Settings");
  expect(dialogNames()).toEqual(["Settings and details"]);
  expect(dialogs.most()).toBe(1);
  dialogs.stop();
});

it("closes the Guide sheet before a run estimate that arrives while it is open below 1280px", async () => {
  const suite = scenarioAgent(); suite.kind = "test_suite"; suite.accepted = true;
  session.document.evaluation = { id: session.id, chat_id: "chat", door: "test" };
  harness.params = new URLSearchParams(`session=${session.id}&view=tests&agent=${suite.id}`);
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  respond = async path => {
    if (path === "/config") return json({ enabled: true, two_door: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id })) });
    if (path.endsWith("/run-quote")) { await held; return json({ id: "quote-1", cases: 3, calls: 6, max_cost_nano_usd: 0, expires_at: new Date(Date.now() + 600000).toISOString() }); }
    return path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  };
  await render();
  const dialogs = watchDialogs();
  await click("Run 3 tests");
  await click("Guide");
  expect(dialogNames()).toEqual(["Guide"]);
  await act(async () => { release(); await new Promise(resolve => setTimeout(resolve, 0)); });
  expect(dialogNames()).toEqual(["Run these tests"]);
  expect(dialogs.most()).toBe(1);
  dialogs.stop();
  // Only the estimate was requested; nothing ran.
  expect(requests.filter(r => r.method === "POST").map(r => r.path)).toEqual([`/sessions/${session.id}/run-quote`]);
});

it("keeps the Guide open by default from 1280px after the first version and remembers a close across a reload", async () => {
  vi.stubGlobal("matchMedia", vi.fn((query: string) => ({ matches: query === "(min-width: 1280px)", addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  acceptedDraft();
  // The Guide opens by default only with a conversation in it.
  session.document.messages = [{ id: "message-one", role: "assistant", content: "I made a first version.", artifact_id: "artifact-one" }];
  session.document.evaluation = { id: session.id, chat_id: "chat", door: "build" };
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id })) }) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  const guide = () => container.querySelector('[aria-label="Guide utility"]');
  expect(guide()).not.toBeNull();
  expect(button("Guide").getAttribute("aria-pressed")).toBe("true");
  expect(dialogNames()).toEqual([]);
  await click("Close Guide");
  expect(guide()).toBeNull();
  await act(async () => root.unmount());
  root = createRoot(container);
  await render();
  expect(container.querySelector('[aria-label="Workspace views"]')).not.toBeNull();
  expect(guide()).toBeNull();
  expect(button("Guide").getAttribute("aria-pressed")).toBe("false");
  await click("Guide");
  expect(guide()).not.toBeNull();
  expect(posts()).toHaveLength(0);
});

it("puts the workspace picker on the entry screen and focuses it from the sidebar instead of opening Settings", async () => {
  signedInWorkspace(); harness.params = new URLSearchParams();
  harness.me.mockResolvedValue({ organizations: [{ role: "org_admin", workspaces: [{ id: "workspace-one", name: "Workspace One" }, { id: "workspace-two", name: "Workspace Two" }] }] });
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, availability: { guest: { available: false }, workspace: { available: true } }, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }] }) : json([]);
  await render();
  const pickers = document.querySelectorAll<HTMLSelectElement>('select[aria-label="Workspace for new projects"]');
  expect(pickers).toHaveLength(1);
  const picker = pickers[0];
  expect(picker.closest(".vibe-welcome")).not.toBeNull();
  expect(picker.closest(".vibe-sidebar")).toBeNull();
  expect(document.getElementById(picker.getAttribute("aria-describedby")!)?.textContent).toBe("Choose a workspace to use its credits.");
  expect([...document.querySelectorAll("button")].some(b => b.textContent === "Review settings")).toBe(false);
  const sidebar = container.querySelector(".vibe-sidebar-workspace")!;
  expect(sidebar.textContent).toContain("Not chosen");
  await click("Choose workspace");
  expect(document.activeElement).toBe(picker);
  expect(dialogNames()).toEqual([]);
  await act(async () => { picker.value = "workspace-two"; picker.dispatchEvent(new Event("change", { bubbles: true })); });
  expect(picker.hasAttribute("aria-describedby")).toBe(false);
  expect(sidebar.textContent).toContain("Workspace Two");
  expect(button("Change workspace")).toBeTruthy();
  expect([...container.querySelectorAll<HTMLButtonElement>("button")].filter(b => /Build an agent|Improve an existing agent/.test(b.textContent || "")).every(b => !b.disabled)).toBe(true);
  expect(requests.filter(request => request.method === "POST")).toHaveLength(0);
});

it.each([
  { access: "ready", text: "You need a workspace to start a project.", action: "Open your dashboard" },
  { access: "error", text: "Your workspaces couldn’t be loaded.", action: "Try again" },
  { access: "signed_out", text: "Sign in again to choose a workspace.", action: "Sign in again" },
])("replaces Review settings with a workspace action when workspace access is $access", async ({ access, text, action }) => {
  signedInWorkspace(); harness.params = new URLSearchParams();
  if (access === "ready") harness.me.mockResolvedValue({ organizations: [] });
  else harness.me.mockRejectedValue(Object.assign(new Error("Unavailable"), { status: access === "signed_out" ? 401 : 503 }));
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, availability: { guest: { available: false }, workspace: { available: true } }, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }] }) : json([]);
  await render();
  expect(document.querySelector('select[aria-label="Workspace for new projects"]')).toBeNull();
  expect(container.querySelector(".vibe-entry-availability")?.textContent).toContain(text);
  expect([...container.querySelectorAll(".vibe-entry-availability :is(a, button)")].map(item => item.textContent)).toEqual([action]);
});

it("automatically chooses the only workspace without a picker or inference", async () => {
  signedInWorkspace(); harness.params = new URLSearchParams();
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, availability: { guest: { available: false }, workspace: { available: true } }, defaults: defaultModels, models: [{ id: defaultModels.target, name: "Fixture" }] }) : json([]);
  await render();
  expect(container.querySelector('select[aria-label="Workspace for new projects"]')).toBeNull();
  expect(container.querySelector(".vibe-sidebar-workspace")?.textContent).toContain("Workspace One");
  expect([...container.querySelectorAll<HTMLButtonElement>(".vibe-entry-doors button")].every(button => !button.disabled)).toBe(true);
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
  expect(history.value).toBe("two");
  expect(container.querySelector('[role="log"]')?.textContent).toContain("Saved reply two");
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
  await render(); await click("New project");
  const choice = container.querySelector<HTMLSelectElement>('select[aria-label="Workspace for new projects"]')!;
  expect([...choice.options].some(o => o.value === "workspace-b")).toBe(true);
  await act(async () => { choice.value = "workspace-b"; choice.dispatchEvent(new Event("change", { bubbles: true })); });
  const door = [...container.querySelectorAll<HTMLButtonElement>("button")].find(b => b.textContent?.includes("Build an agent"))!;
  expect(door.disabled).toBe(false);
  await act(async () => door.click());
  expect(requests.filter(r => r.path === "/sessions" && r.method === "POST")).toHaveLength(0);
  expect(params().get("start")).toBe("build");
  expect(params().get("workspace")).toBe("workspace-b");
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

it("keeps untested version Results empty until explicit history pins the tested version", async () => {
  const v1 = {id:"v1",kind:"test_suite" as const,title:"Old policy",agent_prompt:"Old instructions",accepted:true,source_message_id:"request",blueprint:{cases:[]},sample:"email"};
  const v2 = {...v1,id:"v2",title:"New policy",agent_prompt:"New instructions",parent_id:"v1"};
  session.document.evaluation={id:session.id,chat_id:session.id,door:"build"};
  session.document.artifacts=[v1,v2]; session.document.active_artifact_id="v2";
  session.operations=[{id:"old-run",kind:"check",state:"COMPLETED",billing:"SETTLED",models:defaultModels,source:{kind:"prompt",artifact_id:"v1",label:"Old policy"},results:[],max_cost_nano_usd:0,actual_cost_nano_usd:0,scorecard:{total:1,passed:1,failed:0,unknown:0,evaluated:1,pass_rate:1,coverage:1}}];
  harness.params=new URLSearchParams(`session=${session.id}&view=agent&agent=v2`);
  const original=respond; respond=async(path,options)=>path==="/config"?json({enabled:true,two_door:true,defaults:defaultModels,models:[...new Set(Object.values(defaultModels))].map(id=>({id,name:id})),availability:{guest:{available:true},workspace:{available:true}}}):original(path,options);
  await render(); await click("Results");
  expect(new URL(location.href).searchParams.get("run")).toBeNull();
  expect(container.querySelector(".vibe-project-header")?.textContent).toContain("v2 · New policy");
  expect(container.querySelector(".vibe-project-main")?.textContent).toContain("No results for this version");
  expect(container.querySelector('[aria-label="Keep and extend your results"]')).toBeNull();
  expect(container.querySelector('[aria-label="Next actions for this version"]')).toBeNull();
  const history=container.querySelector<HTMLSelectElement>('[aria-label="Result history"]');
  expect(history).not.toBeNull();
  expect(session.operations).toHaveLength(1);
  await act(async()=>{history!.value="old-run";history!.dispatchEvent(new Event("change",{bubbles:true}));});
  expect(new URL(location.href).searchParams.get("run")).toBe("old-run");
  expect(new URL(location.href).searchParams.get("agent")).toBe("v1");
  expect(container.querySelector(".vibe-project-header")?.textContent).toContain("v1 · Old policy");
  expect(container.querySelector('[aria-label="Next actions for this version"]')?.getAttribute("data-artifact-id")).toBe("v1");
  expect(container.querySelector('[aria-label="Result history"]')).toBeNull();
  const followup=container.querySelector('[aria-label="Next actions for this version"]')!;
  await act(async()=>[...followup.querySelectorAll("summary")].find(summary=>summary.textContent==="More ways to test")!.click());
  await act(async()=>[...followup.querySelectorAll<HTMLButtonElement>("button")].find(button=>button.textContent==="Add harder tests")!.click());
  await click("See cost and continue");
  expect(requests.find(request=>request.path.endsWith("/build-quote"))?.body.artifact_id).toBe("v1");
  expect(posts()).toHaveLength(0);
});


it.each([[false,false],[false,true],[true,false]])("collects new recorded replies against a completed baseline before quoting the exact new artifact (test journey: %s, lost acknowledgment: %s)",async(testJourney,lostAck)=>{
 const oldArtifact={id:"old-recorded",kind:"conversation_evaluation",title:"Original replies",agent_prompt:"",blueprint:null,accepted:true,source_message_id:"",conversation_evaluation:{evidence_set_id:"old-source",expectations:[{id:"original-rule",statement:"Use the supplied facts."}]}};
 const oldCase={case_key:"chat-1",version:oldArtifact.id,input:null,output:"Old reply",verdict:"FAIL" as const,checks:[{key:"original-rule",verdict:"FAIL" as const,evidence:"Unsupported claim"}]};
 const baseline:Operation={id:"old-result",kind:"check",state:"COMPLETED",billing:"SETTLED",models:defaultModels,max_cost_nano_usd:0,actual_cost_nano_usd:0,results:[oldCase],source:{kind:"provided_conversations",artifact_id:oldArtifact.id,evidence_set_id:"old-source",label:"Original replies"},scorecard:{passed:0,failed:1,unknown:0,total:1,evaluated:1,pass_rate:0,coverage:1}};
 session.document={...session.document,format_version:1,test_journey:testJourney,evaluation:{id:session.id,chat_id:session.id,door:"test"},artifacts:[oldArtifact],evidence_sets:[{id:"old-source",label:"Original replies",raw:"Customer: Question\nAgent: Old reply",conversations:[{key:"chat-1",title:"Chat 1",messages:[{id:"m1",role:"user",content:"Question"},{id:"m2",role:"assistant",content:"Old reply"}]}]}]};session.operations=[baseline];
 const initial=respond;let intakeAttempts=0;
 respond=async(path,options)=>{
  if(path==="/config")return json({enabled:true,two_door:true,availability:{guest:{available:true},workspace:{available:true}},defaults:defaultModels,models:[...new Set(Object.values(defaultModels))].map(id=>({id,name:id}))});
  if(path.endsWith("/grade-reviews"))return json([]);
  if(path.includes("/case"))return json(oldCase);
  if(path.endsWith("/evidence")){
   const body=JSON.parse(String(options.body));
   session={...session,revision:session.revision+1,event_cursor:session.event_cursor+1,document:{...session.document,artifacts:[oldArtifact,{...oldArtifact,id:"new-recorded",accepted:false,conversation_evaluation:{evidence_set_id:"new-source",expectations:body.expectations}}],evidence_sets:[...session.document.evidence_sets!,{id:"new-source",label:"Updated replies",raw:body.content,conversations:[{key:"chat-1",title:"Chat 1",messages:[{id:"m1",role:"user",content:"Question"},{id:"m2",role:"assistant",content:"Corrected reply"}]}]}]}};
   intakeAttempts++;if(lostAck && intakeAttempts===1)throw new TypeError("Lost updated intake acknowledgment");
   return json({...session,evidence_receipt:{client_id:body.client_id,evidence_id:"new-source",artifact_id:"new-recorded"}});
  }
  if(path.endsWith("/messages"))return json({...baseline,id:"updated-result",baseline_id:baseline.id,source:{...baseline.source,artifact_id:"new-recorded",evidence_set_id:"new-source"}});
  if(path.endsWith("/run-quote"))return json({id:"new-source-quote",kind:"run",revision:session.revision,cases:1,calls:1,max_cost_nano_usd:0,expires_at:new Date(Date.now()+600000).toISOString()});
  return initial(path,options);
 };
 await render();await click("Results");await click("Test new replies");
 expect(requests.some(r=>r.path.endsWith("/messages"))).toBe(false);
 expect(container.querySelector<HTMLTextAreaElement>('[aria-label="Recorded replies"]')!.value).toBe("");
 await type("Customer: Question\nAgent: Corrected reply","Recorded replies");await type("Use the supplied facts.","Expected behavior");await click("Review recorded replies");
 if(lostAck){await act(async()=>snapshot(session));await click("Retry saving these replies");const attempts=requests.filter(r=>r.path.endsWith("/evidence"));expect(attempts[1].body).toEqual(attempts[0].body);}
 expect(requests.find(r=>r.path.endsWith("/evidence"))?.body.expectations).toEqual([{id:"original-rule",statement:"Use the supplied facts."}]);
 expect(requests.some(r=>r.path.endsWith("/messages"))).toBe(false);
 expect(session.document.artifacts[0]).toEqual(oldArtifact);
 await act(async()=>root.render(<div/>));await render();await click("Tests");
 expect(requests.some(r=>r.path.endsWith("/run-quote"))).toBe(false);
 await click("Run recorded tests");
 expect(requests.findLast(r=>r.path.endsWith("/run-quote"))?.body).toMatchObject({kind:"retest",baseline_id:baseline.id,artifact_id:"new-recorded",evidence_set_id:"new-source"});
 expect(requests.some(r=>r.path.endsWith("/messages"))).toBe(false);
 await click("Refresh estimate");
 const quotes=requests.filter(r=>r.path.endsWith("/run-quote")&&r.body.artifact_id==="new-recorded");expect(quotes).toHaveLength(2);
 expect(quotes.at(-1)!.body).toMatchObject({kind:"retest",baseline_id:baseline.id,artifact_id:"new-recorded",evidence_set_id:"new-source"});
 await click("Start run");
 expect(requests.find(r=>r.path.endsWith("/messages"))?.body).toMatchObject({kind:"retest",baseline_id:baseline.id,artifact_id:"new-recorded",evidence_set_id:"new-source",run_quote_id:"new-source-quote"});
});

it("saves the frozen dialog version after a newer version arrives without substituting its tests",async()=>{
 acceptedDraft();signedInWorkspace();
 const original=structuredClone(session.document.artifacts[0]);
 respond=async(path)=>path.endsWith('/save')?json({draft_id:'frozen-draft',workspace_id:'workspace-one'}):json(session);
 await render();await click('Save agent');
 session.document.artifacts.push({...original,id:'later-version',title:'Different later tests'});session.document.active_artifact_id='later-version';session.revision++;session.event_cursor=session.revision;
 await act(async()=>snapshot(structuredClone(session)));
 const dialog=document.querySelector('[role="dialog"]')!;expect(dialog.textContent).toContain(original.id.slice(0,8));expect(dialog.textContent).not.toContain('Different later tests');
 await click('Save and iterate');
 const write=requests.find(r=>r.path.endsWith('/save'));expect(write?.body.artifact_id).toBe(original.id);expect(write?.body.models).toEqual(defaultModels);expect(posts()).toHaveLength(0);
});

function sdkProject() {
 const id="a1000000-0000-4000-8000-000000000001";
 const artifact={id:"a1000000-0000-4000-8000-000000000002",kind:"test_suite" as const,title:"Pinned SDK tests",agent_prompt:"",accepted:false,source_message_id:"",blueprint:{cases:[]}};
 const newer={...artifact,id:"a1000000-0000-4000-8000-000000000003",title:"Newer tests"};
 session.anonymous=false;session.workspace_id="workspace-sdk";
 session.document.evaluation={id:session.id,chat_id:session.id,door:"test"};session.document.artifacts=[artifact,newer];
 harness.user={id:"sdk-actor"};harness.token.mockResolvedValue("sdk-token");
 harness.me.mockResolvedValue({organizations:[{role:"org_admin",workspaces:[{id:session.workspace_id,name:"SDK workspace",role:"workspace_admin"}]}]});
 const report={id,external_report_id:"historical-sdk",source_kind:"agent_result",content_sha256:"hash",size_bytes:40,cases:[],binding:{session_id:session.id,artifact_id:artifact.id,result_ref:{kind:"sdk_report",id}},evidence_metadata:{origin:"imported",capture_scope:{},fingerprints:{}}};
 const before=respond;
 respond=async(path,options)=>path==="/config"?json({enabled:false,two_door:true,defaults:defaultModels,models:[],availability:{guest:{available:false},workspace:{available:false}}}):path.endsWith("/reports")?json([report]):path.endsWith("/reports/"+id)?json(report):path.endsWith("/case-mapping")||path.endsWith("/grade-reviews")?json([]):before(path,options);
 return {id,artifact,newer,report};
}
it("resolves an SDK URL before pinning its version and keeps historical Results usable without inference or operations",async()=>{
 const {id,artifact,newer,report}=sdkProject();
 harness.params=new URLSearchParams(`session=${session.id}&view=results&agent=${newer.id}&run=sdk_report:${id}`);
 const gate=deferred<Response>();const before=respond;respond=(path,options)=>path.endsWith("/reports/"+id)?gate.promise:before(path,options);
 await render();
 expect(new URL(location.href).searchParams.get("run")).toBe(`sdk_report:${id}`);
 expect(container.textContent).toContain("Loading selected result");
 await act(async()=>gate.resolve(json(report)));
 expect(new URL(location.href).searchParams.get("agent")).toBe(artifact.id);
 expect(container.querySelector('[aria-label="Imported report detail"]')).not.toBeNull();
 expect(container.querySelector('[aria-label="SDK original JSON"]')).not.toBeNull();
 expect(button("Estimate hosted regrade").disabled).toBe(true);
 expect(requests.filter(request=>request.path.endsWith("/reports/"+id))).toHaveLength(1);
 expect(posts()).toHaveLength(0);
});
it("retains an unavailable SDK selection for retry then clears a confirmed foreign report with an explanation",async()=>{
 const {id}=sdkProject();harness.params=new URLSearchParams(`session=${session.id}&view=results&run=sdk_report:${id}`);
 const before=respond;let count=0;respond=(path,options)=>path.endsWith("/reports/"+id)?Promise.resolve(++count===1?json({error:{code:"unavailable",message:"Temporary source failure"}},503):json({error:{code:"not_found",message:"Report unavailable"}},404)):before(path,options);
 await render();expect(new URL(location.href).searchParams.get("run")).toBe(`sdk_report:${id}`);
 await click("Retry selected result");expect(new URL(location.href).searchParams.get("run")).toBeNull();
 expect(container.textContent).toContain("selected result is unavailable");expect(posts()).toHaveLength(0);
});

it("does not revive a prior actor's SDK lookup after an A to B to A transition",async()=>{
 const {id,report}=sdkProject();harness.params=new URLSearchParams(`session=${session.id}&view=results&run=sdk_report:${id}`);
 const old=deferred<Response>(),fresh=deferred<Response>();const before=respond;let reads=0;
 respond=(path,options)=>path.endsWith("/reports/"+id)?(++reads===1?old.promise:fresh.promise.then(response=>response.clone())):before(path,options);
 await render();harness.user={id:"other-sdk-actor"};await render();harness.user={id:"sdk-actor"};await render();
 await act(async()=>old.resolve(json(report)));expect(container.querySelector('[aria-label="Imported report detail"]')).toBeNull();expect(container.textContent).toContain("Loading selected result");
 await act(async()=>fresh.resolve(json(report)));expect(container.querySelector('[aria-label="Imported report detail"]')).not.toBeNull();expect(posts()).toHaveLength(0);
});

it("explains a malformed SDK selector without loading or executing it",async()=>{
 sdkProject();harness.params=new URLSearchParams(`session=${session.id}&view=results&run=sdk_report:invalid`);await render();
 expect(new URL(location.href).searchParams.get("run")).toBeNull();expect(container.textContent).toContain("selected result is unavailable");expect(requests.some(r=>r.path.endsWith("/reports/invalid"))).toBe(false);expect(posts()).toHaveLength(0);
});

it("loads canonical source-qualified Results once, pins its artifact and stops terminal polling",async()=>{
 vi.useFakeTimers();
 try {
 const {id,artifact,report}=sdkProject();const result={binding:{...report.binding,result_ref:{kind:"run",id}},evidence_metadata:{...report.evidence_metadata,result_ref:{kind:"run",id}},name:"Approved application",status:"completed",created_at:"2026-10-26T00:00:00Z",participants:[],evidence_hash:"canonical-hash",omissions:["Raw canonical event bodies omitted."]};
 harness.params=new URLSearchParams(`session=${session.id}&view=results&agent=${artifact.id}&run=run:${id}`);const before=respond;respond=async(path,options)=>path.endsWith("/canonical-results")?json([result]):path.includes("/canonical-results/run/")?json(result):before(path,options);
 await render();expect(container.querySelector('[aria-label="Application result detail"]')).not.toBeNull();expect(container.querySelector(".vibe-project-header")?.textContent).toContain(" · Application run");expect(requests.filter(request=>request.path.includes("/canonical-results/run/"))).toHaveLength(1);expect(new URL(location.href).searchParams.get("run")).toBe(`run:${id}`);expect(posts()).toHaveLength(0);
 await act(async()=>vi.advanceTimersByTime(9000));
 expect(requests.filter(request=>request.path.includes("/canonical-results/run/"))).toHaveLength(1);
 expect(requests.filter(request=>request.path.endsWith("/canonical-results"))).toHaveLength(1);
 } finally { vi.useRealTimers(); }
});
it("retains canonical source uncertainty and retries an authorized lookup without dispatch",async()=>{
 const {id,artifact,report}=sdkProject();const result={binding:{...report.binding,result_ref:{kind:"run",id}},evidence_metadata:{},name:"Application",status:"completed",created_at:"",participants:[],evidence_hash:"hash",omissions:[]};harness.params=new URLSearchParams(`session=${session.id}&view=results&agent=${artifact.id}&run=run:${id}`);const before=respond;let calls=0;respond=async(path,options)=>path.endsWith("/canonical-results")?json([result]):path.includes("/canonical-results/run/")?(++calls===1?json({error:{code:"unavailable",message:"Canonical temporarily unavailable"}},503):json(result)):before(path,options);await render();expect(new URL(location.href).searchParams.get("run")).toBe(`run:${id}`);await click("Retry selected result");expect(container.querySelector('[aria-label="Application result detail"]')).not.toBeNull();expect(posts()).toHaveLength(0);
});


it("explains unavailable evaluation entry before its disabled choices while keeping the sample readable", async () => {
  harness.params = new URLSearchParams();
  respond = async path => path === "/config" ? json({ enabled: true, two_door: true, defaults: defaultModels, models: [], availability: { guest: { available: false }, workspace: { available: false } } }) : json([]);
  await render();
  const reason = [...container.querySelectorAll('[role="status"]')].find(node => node.textContent === "No approved models are available.")!;
  const choices = container.querySelector(".vibe-entry-doors")!;
  expect(reason).toBeTruthy();
  expect(reason.compareDocumentPosition(choices) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect([...choices.querySelectorAll("button")].every(button => button.disabled)).toBe(true);
  expect(button("Read a sample test")).toBeTruthy();
  expect(requests.filter(request => request.method === "POST")).toHaveLength(0);
});

// Entry and door invariants (~/.claude/plans/vibe-evals-redesign.md section 8.2). it.fails marks a
// known defect; vitest errors once it is fixed, so the marker must then be removed.
async function enterDoor(door: "build" | "test") {
  harness.params = new URLSearchParams();
  respond = async (path, options) => {
    if (path === "/config") return json({ enabled: true, two_door: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) });
    if (path === "/sessions" && options.method === "POST") {
      const id = JSON.parse(String(options.body)).id;
      session = { ...session, id, document: { ...session.document, format_version: 1, test_journey: true, evaluation: { id, chat_id: id, door } } };
      return json(session, 201);
    }
    if (path.endsWith("/build-quote")) return json({ id: "quote", max_calls: 9, cases: 3, max_cost_nano_usd: 250000000, request: { content: JSON.parse(String(options.body)).content, models: defaultModels }, expires_at: new Date(Date.now() + 600000).toISOString() });
    if (path.endsWith("/messages")) return json({ id: "admitted", kind: "message", state: "QUEUED", models: defaultModels, results: [] }, 202);
    return path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  };
  await render();
  const choice = [...container.querySelectorAll<HTMLButtonElement>(".vibe-entry-doors button")].find(b => b.textContent?.includes(door === "build" ? "Build an agent" : "Improve an existing agent"))!;
  choice.focus();
  await act(async () => { choice.click(); await new Promise(resolve => setTimeout(resolve, 0)); });
  expect(container.textContent).toContain(door === "build" ? "What task should the agent handle?" : "How do you want to test your agent?");
}
const visibleH1 = () => [...container.querySelectorAll("h1")].filter(h1 => !h1.closest("[hidden]"));

// Plain tests of the setup, so an it.fails below cannot pass because the door setup broke.
for (const door of ["build", "test"] as const) it(`enters the ${door} door with the shared setup`, () => enterDoor(door));

for (const door of ["build", "test"] as const) {
  // B3: the uncommitted start question is the only h1.
  it(`shows exactly one h1 on the ${door} start (B3)`, async () => {
    await enterDoor(door);
    expect(visibleH1()).toHaveLength(1);
    expect(visibleH1()[0].textContent).toBe(door === "build" ? "What task should the agent handle?" : "How do you want to test your agent?");
    expect(container.querySelector(".vibe-project-header")).toBeNull();
  });
  // D3: Build focuses its task input; Improve focuses its start heading.
  it(`moves focus into the ${door} start after the door click, not to the body (D3)`, async () => {
    await enterDoor(door);
    expect(document.activeElement).not.toBe(document.body);
    expect(document.activeElement).toBe(door === "build" ? composer() : visibleH1()[0]);
  });
}

it("shows one primary action on the Build start (B2)", async () => {
  await enterDoor("build");
  expect(container.querySelectorAll(".vibe-button-primary")).toHaveLength(1);
});

it("creates no project before the first committed input (A5)", async () => {
  await enterDoor("build");
  await type("Keep this uncommitted task.");
  expect(container.querySelectorAll(".vibe-start-page button:not(:disabled), .vibe-start-page textarea")).toHaveLength(7);
  expect(requests.filter(r => r.method === "POST" && r.path === "/sessions")).toHaveLength(0);
});

it("never shows two dialogs at once from the entry to the first Build send", async () => {
  const dialogs = watchDialogs();
  await enterDoor("build");
  await type("Answer shop return questions within 30 days.");
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 450)); });
  expect(button("Send message").disabled).toBe(false);
  await click("Send message");
  expect(posts()).toHaveLength(1);
  expect(dialogs.most()).toBeLessThanOrEqual(1);
  dialogs.stop();
});

// PR 1 flow fixes (~/.claude/plans/vibe-evals-redesign.md sections 2 and 7, row 1).
const twoDoorConfig = () => json({ enabled: true, two_door: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id })) });
const runID = "a8000000-0000-4000-8000-0000000000a8";
const params = () => new URL(location.href).searchParams;
const frame = () => act(async () => { await new Promise(resolve => setTimeout(resolve, 50)); });
const shown = (selector: string) => [...container.querySelectorAll<HTMLElement>(selector)].filter(node => !node.closest("[hidden]"));
// A two-door project with three ready tests (artifact-one).
function readyTests(door: "build" | "test" = "test") {
  const suite = scenarioAgent(); suite.kind = "test_suite"; suite.accepted = true;
  session.document = { ...session.document, format_version: 1, test_journey: true, evaluation: { id: session.id, chat_id: session.id, door } };
  return suite;
}

it.each(["guest", "member", "viewer", "no workspace", "loading"])("shows SDK import only for a signed-in workspace writer (%s)", async actor => {
  readyTests();
  if (actor !== "guest") signedInWorkspace();
  if (actor !== "guest" && actor !== "no workspace") session.workspace_id = "workspace-one";
  if (actor === "viewer") harness.me.mockResolvedValue({ organizations: [{ role: "org_member", workspaces: [{ id: "workspace-one", name: "Workspace One", role: "workspace_viewer" }] }] });
  if (actor === "loading") harness.me.mockReturnValue(new Promise(() => {}));
  session.operations = [run(runID, "artifact-one")];
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  respond = async path => path === "/config" ? twoDoorConfig() : path.endsWith("/case") ? json(session.operations[0].results[0]) : path.endsWith("/reports") || path.endsWith("/canonical-results") || path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  expect(container.querySelector('[aria-label="Run results"]')).not.toBeNull();
  const summaries = [...container.querySelectorAll("summary")].map(s => s.textContent);
  expect(summaries.filter(s => s === "Import SDK report")).toHaveLength(actor === "member" ? 1 : 0);
  expect(summaries).not.toContain("SDK report history");
  expect(summaries).not.toContain("Application run history");
  expect(container.querySelector('[aria-label="Observed result evidence"]')).toBeNull();
  if (actor === "guest") expect(requests.filter(r => r.path.endsWith("/reports") || r.path.endsWith("/canonical-results"))).toHaveLength(0);
});
function run(id: string, artifact: string, extra: Partial<Operation> = {}): Operation {
  return { id, kind: "check", state: "COMPLETED", billing: "SETTLED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, source: { kind: "prompt", label: "Tests", artifact_id: artifact },
    results: [{ case_key: "case-1", version: artifact, input: { question: "A complete task" }, output: "A wrong reply", verdict: "FAIL", checks: [{ key: "behavior", verdict: "FAIL", evidence: "It ignored the rule." }] }],
    scorecard: { passed: 0, failed: 1, unknown: 0, total: 1, evaluated: 1, pass_rate: 0, coverage: 1 }, ...extra };
}
// The server admits the run; the session snapshot gets it only later (A8).
function admitRunLater() {
  respond = async path => {
    if (path === "/config") return twoDoorConfig();
    if (path.endsWith("/run-quote")) return json({ id: "quote", cases: 3, calls: 6, max_cost_nano_usd: 0, expires_at: new Date(Date.now() + 600000).toISOString() });
    if (path.endsWith("/messages")) return json(run(runID, "artifact-one", { state: "RUNNING", scorecard: undefined, results: [] }), 202);
    return path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  };
}
const publish = (operations: Operation[], extra: Partial<Session["document"]> = {}) => act(async () => {
  session = { ...session, revision: session.revision + 1, event_cursor: session.event_cursor + 1, operations, document: { ...session.document, ...extra } };
  snapshot(session);
});

it("opens Results with focus on its heading when a run started on Tests ends, with no unavailable alert while the snapshot catches up (A8)", async () => {
  readyTests();
  harness.params = new URLSearchParams(`session=${session.id}&view=tests`);
  admitRunLater();
  await render();
  await click("Run 3 tests"); await click("Start run");
  expect(params().get("run")).toBe(runID);
  expect(container.textContent).not.toContain("selected result is unavailable");
  const entries = history.length;
  await publish([run(runID, "artifact-one", { state: "RUNNING", scorecard: undefined, results: [] })]);
  expect(params().get("view")).toBe("tests");
  await publish([run(runID, "artifact-one")]); await frame();
  expect(params().get("view")).toBe("results");
  expect(params().get("run")).toBe(runID);
  expect(history.length).toBe(entries);
  expect(document.activeElement?.tagName).toBe("H2");
  expect(document.activeElement?.closest('[aria-label="Run results"]')).not.toBeNull();
  expect(container.textContent).not.toContain("selected result is unavailable");
  expect(posts()).toHaveLength(1);
});

it("keeps a user who left the run's view where they are and offers See the result", async () => {
  readyTests();
  harness.params = new URLSearchParams(`session=${session.id}&view=tests`);
  admitRunLater();
  await render();
  await click("Run 3 tests"); await click("Start run");
  await publish([run(runID, "artifact-one", { state: "RUNNING", scorecard: undefined, results: [] })]);
  await click("Agent");
  await publish([run(runID, "artifact-one")]); await frame();
  expect(params().get("view")).toBe("agent");
  expect(container.querySelector(".vibe-arrival")?.textContent).toContain("Your run finished.");
  await click("See the result"); await frame();
  expect(params().get("view")).toBe("results");
  expect(params().get("run")).toBe(runID);
  expect(container.querySelector(".vibe-arrival")).toBeNull();
  expect(document.activeElement?.closest('[aria-label="Run results"]')).not.toBeNull();
});

it.each(["link", "history"])("announces a new run while Results shows an older run and clears arrival through %s", async selection => {
  readyTests();
  const oldID = "a8000000-0000-4000-8000-000000000001";
  const old = run(oldID, "artifact-one");
  session.operations = [old];
  harness.params = new URLSearchParams(`session=${session.id}&view=tests`);
  admitRunLater();
  await render();
  await click("Run again"); await click("Start run");
  await publish([old, run(runID, "artifact-one", { state: "RUNNING", scorecard: undefined, results: [] })]);
  await click("Results");
  const selectRun = (id: string) => act(async () => {
    const history = container.querySelector<HTMLSelectElement>('[aria-label="Result history"]')!;
    history.value = id; history.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await selectRun(oldID);
  await publish([old, run(runID, "artifact-one")]); await frame();
  expect(params().get("view")).toBe("results");
  expect(params().get("run")).toBe(oldID);
  expect(announced()).toContain("Your run finished.");
  expect(container.querySelector(".vibe-arrival")?.textContent).toContain("Your run finished.");
  expect(resultLinks()).toHaveLength(1);
  await publish(session.operations);
  expect(resultLinks()).toHaveLength(1);
  if (selection === "link") await click("See the result");
  else await selectRun(runID);
  expect(params().get("run")).toBe(runID);
  expect(container.querySelector(".vibe-arrival")).toBeNull();
  expect(announced()).not.toContain("Your run finished.");
  await selectRun(oldID);
  expect(resultLinks()).toHaveLength(0);
});

it("does not move a project behind the new-project chooser when its run ends, and offers the result on return", async () => {
  readyTests();
  harness.params = new URLSearchParams(`session=${session.id}&view=tests`);
  admitRunLater();
  await render();
  await click("Run 3 tests"); await click("Start run");
  await publish([run(runID, "artifact-one", { state: "RUNNING", scorecard: undefined, results: [] })]);
  await click("New project");
  await publish([run(runID, "artifact-one")]); await frame();
  // The chooser stays, and the project behind it stays on Tests.
  expect(container.querySelector('[aria-label="New project choices"]')).not.toBeNull();
  expect(params().get("view")).toBe("tests");
  await click("Back to your project"); await frame();
  expect(params().get("view")).toBe("tests");
  expect(announced()).toContain("Your run finished.");
  expect(resultLinks()).toHaveLength(1);
  await click("See the result"); await frame();
  expect(params().get("view")).toBe("results");
  expect(params().get("run")).toBe(runID);
  expect(announced()).not.toContain("Your run finished.");
});

// The persistent status region announces the finished work; the visible link has no role of its own.
const announced = () => [...container.querySelectorAll('.sr-only[role="status"]')].map(node => node.textContent).join(" ");
const resultLinks = () => [...container.querySelectorAll("button")].filter(b => b.textContent === "See the result");

it("offers See the result on Tests while the user edits the tests, and announces the finished run", async () => {
  readyTests();
  harness.params = new URLSearchParams(`session=${session.id}&view=tests`);
  admitRunLater();
  await render();
  await click("Run 3 tests"); await click("Start run");
  await publish([run(runID, "artifact-one", { state: "RUNNING", scorecard: undefined, results: [] })]);
  await click("Agent");
  expect(announced()).not.toContain("Your run finished.");
  await publish([run(runID, "artifact-one")]); await frame();
  expect(params().get("view")).toBe("agent");
  expect(announced()).toContain("Your run finished.");
  expect(container.querySelector(".vibe-arrival")?.getAttribute("role")).toBeNull();
  await click("Tests");
  // The tests' own "See the result" leads; the arrival link does not repeat it.
  expect(resultLinks()).toHaveLength(1);
  expect(container.querySelector(".vibe-arrival")).toBeNull();
  await click("Edit tests");
  // The editor hides the tests' own link, so the arrival link shows.
  expect(resultLinks()).toHaveLength(1);
  expect(container.querySelector(".vibe-arrival")?.textContent).toContain("Your run finished.");
  await click("See the result"); await frame();
  expect(params().get("view")).toBe("results");
  expect(params().get("run")).toBe(runID);
  expect(announced()).not.toContain("Your run finished.");
});

// Guide's fix (artifact-two) is rerun from the Agent view against Run 1.
async function rerunFixOnAgent() {
  readyTests("build");
  const v1 = session.document.artifacts[0];
  session.document.artifacts = [v1, { ...v1, id: "artifact-two", parent_id: v1.id, accepted: false, agent_prompt: "Original policy. Never promise a refund." }];
  session.operations = [run(runID, "artifact-one")];
  harness.params = new URLSearchParams(`session=${session.id}&view=agent&agent=artifact-two`);
  const retestID = "a8000000-0000-4000-8000-0000000000c3";
  const retest = (extra: Partial<Operation> = {}) => run(retestID, "artifact-two", { kind: "retest", baseline_id: runID, ...extra });
  respond = async path => {
    if (path === "/config") return twoDoorConfig();
    if (path.endsWith("/run-quote")) return json({ id: "quote", cases: 3, calls: 6, max_cost_nano_usd: 0, expires_at: new Date(Date.now() + 600000).toISOString() });
    if (path.endsWith("/messages")) return json(retest({ state: "RUNNING", scorecard: undefined, results: [] }), 202);
    return path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  };
  await render();
  button("Apply change and rerun").focus();
  await click("Apply change and rerun");
  await click("Start run"); await frame();
  await publish([...session.operations, retest({ state: "RUNNING", scorecard: undefined, results: [] })]);
  return { finish: () => publish([run(runID, "artifact-one"), retest()]) };
}

it("opens Results after a rerun though the cost dialog left focus in the empty trial composer", async () => {
  const rerun = await rerunFixOnAgent();
  // The disabled rerun button cannot take focus back, so the dialog gives it to the trial composer.
  expect(document.activeElement?.id).toBe("vibe-trial-message");
  expect((document.activeElement as HTMLTextAreaElement).value).toBe("");
  await rerun.finish(); await frame();
  expect(params().get("view")).toBe("results");
  expect(document.activeElement?.closest('[aria-label="Run results"]')).not.toBeNull();
  expect(container.querySelector(".vibe-arrival")).toBeNull();
});

it("keeps a user with unsaved agent instructions on Agent when the rerun ends, and offers See the result", async () => {
  const rerun = await rerunFixOnAgent();
  await click("Change agent instructions"); await frame();
  const editor = container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Agent instructions"]')!;
  editor.focus();
  await type("Half-typed new instructions", "Agent instructions");
  // The change is unsaved; the user has left the field.
  act(() => editor.blur());
  expect(document.activeElement).toBe(document.body);
  await rerun.finish(); await frame();
  expect(params().get("view")).toBe("agent");
  expect(editor.isConnected).toBe(true);
  expect(editor.value).toBe("Half-typed new instructions");
  expect(container.querySelector(".vibe-arrival")?.textContent).toContain("Your run finished.");
  expect(announced()).toContain("Your run finished.");
});

// The first Build run: Guide makes artifact-one and runs its tests in the cycle of the Build estimate.
async function firstBuildRunning() {
  await enterDoor("build");
  await type("Answer shop return questions within 30 days.");
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 450)); });
  await click("Send message");
  expect(posts()).toHaveLength(1);
  const cycle = String(posts()[0].body.cycle_id);
  const message: Operation = { id: "admitted", kind: "message", state: "COMPLETED", billing: "SETTLED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] };
  const v1 = { ...scenarioAgent(), kind: "test_suite", accepted: true };
  const build = (phase: string) => ({ artifacts: [v1], build: { cycle_id: cycle, phase, clarifications_used: 0, artifact_id: v1.id, check_id: runID } });
  await publish([message, run(runID, v1.id, { state: "RUNNING", scorecard: undefined, results: [] })], build("checking"));
  expect(params().get("view")).not.toBe("results");
  return { finish: () => publish([message, run(runID, v1.id)], build("results")) };
}

it("keeps a user who writes to the agent during the first Build run where they are, with focus in the composer", async () => {
  const build = await firstBuildRunning();
  const trial = document.getElementById("vibe-trial-message") as HTMLTextAreaElement;
  expect(trial?.closest(".vibe-project-main")).not.toBeNull();
  trial.focus();
  await type("Can I return opened headphones?", "Message your agent");
  await build.finish(); await frame();
  expect(params().get("view")).not.toBe("results");
  expect(document.activeElement).toBe(trial);
  expect(trial.value).toBe("Can I return opened headphones?");
  expect(container.querySelector(".vibe-arrival")?.textContent).toContain("Your run finished.");
  await click("See the result"); await frame();
  expect(params().get("view")).toBe("results");
  expect(container.querySelector('[aria-label="Run results"]')).not.toBeNull();
});

it("opens Results with focus on its heading when the first Build run ends and the user is not typing", async () => {
  const build = await firstBuildRunning();
  // Focus on a button of the view does not hold the user there.
  const change = button("Change agent instructions");
  act(() => change.focus());
  expect(document.activeElement).toBe(change);
  await build.finish(); await frame();
  expect(params().get("view")).toBe("results");
  expect(document.activeElement?.tagName).toBe("H2");
  expect(document.activeElement?.closest('[aria-label="Run results"]')).not.toBeNull();
  expect(container.querySelector(".vibe-arrival")).toBeNull();
});

it("does not announce an arrival when Results already shows the first Build run", async () => {
  const build = await firstBuildRunning();
  await click("Results");
  expect(container.querySelector('[aria-label="Result history"]')).toBeNull();
  expect(container.querySelector(".vibe-project-header")?.textContent).toContain("Run 1: Running");
  expect(params().get("run")).toBeNull();
  await build.finish(); await frame();
  expect(container.querySelector('[aria-label="Agent test results"]')?.getAttribute("data-run-id")).toBe(runID);
  expect(params().get("view")).toBe("results");
  expect(container.querySelector(".vibe-arrival")).toBeNull();
  expect(announced()).not.toContain("Your run finished.");
});

it.each([false, true])("delivers the first Build result after preparation fails and retries in the same cycle (typing: %s)", async typing => {
  await enterDoor("build");
  await type("Answer shop return questions within 30 days.");
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 450)); });
  await click("Send message");
  const cycle = String(posts()[0].body.cycle_id);
  const failed: Operation = { id: "admitted", kind: "message", state: "FAILED", billing: "RELEASED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [], retryable: true, error: { code: "provider_error", message: "Provider failed. Your request is saved." } };
  await publish([failed], { messages: [{ id: String(posts()[0].body.client_id), role: "user", content: String(posts()[0].body.content), operation_id: failed.id }], build: { cycle_id: cycle, phase: "error", clarifications_used: 0, error: failed.error } });
  expect(container.textContent).toContain("Provider failed. Your request is saved.");
  const retry = { ...failed, id: "retry-build", retry_of_operation_id: failed.id, state: "RUNNING", retryable: false, error: undefined };
  const before = respond;
  respond = async (path, options) => path.endsWith("/retry") ? json(retry, 202) : before(path, options);
  await click("Retry");
  await publish([failed, retry], { build: { cycle_id: cycle, phase: "preparing", clarifications_used: 0 } });
  const v1 = { ...scenarioAgent(), kind: "test_suite", accepted: true };
  const build = (phase: string) => ({ artifacts: [v1], build: { cycle_id: cycle, phase, clarifications_used: 0, artifact_id: v1.id, check_id: runID } });
  await publish([failed, { ...retry, state: "COMPLETED" }, run(runID, v1.id, { state: "RUNNING", scorecard: undefined, results: [] })], build("checking"));
  const trial = document.getElementById("vibe-trial-message") as HTMLTextAreaElement;
  if (typing) {
    act(() => trial.focus());
    await type("Keep this draft while retry finishes", "Message your agent");
  } else act(() => button("Change agent instructions").focus());
  await publish([failed, { ...retry, state: "COMPLETED" }, run(runID, v1.id)], build("results")); await frame();
  if (typing) {
    expect(params().get("view")).toBe("agent");
    expect(document.activeElement).toBe(trial);
    expect(trial.value).toBe("Keep this draft while retry finishes");
    expect(announced()).toContain("Your run finished.");
    await click("See the result");
  }
  expect(params().get("view")).toBe("results");
  expect(params().get("run")).toBe(runID);
  expect(container.querySelector('[aria-label="Run results"]')).not.toBeNull();
  if (!typing) expect(document.activeElement?.closest('[aria-label="Run results"]')).not.toBeNull();
  expect(posts()).toHaveLength(1);
  expect(requests.filter(request => request.path.endsWith("/retry"))).toHaveLength(1);
});

it("still reports a run that is missing from the project as unavailable (A8)", async () => {
  readyTests();
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  respond = async path => path === "/config" ? twoDoorConfig() : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  expect(params().get("run")).toBeNull();
  expect(container.textContent).toContain("The selected result is unavailable in this project.");
});

it("offers See the result as the primary on Tests after these tests ran, and Run again as secondary", async () => {
  readyTests();
  harness.params = new URLSearchParams(`session=${session.id}&view=tests`);
  respond = async path => path === "/config" ? twoDoorConfig() : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  expect(button("Run 3 tests").className).toContain("vibe-button-primary");
  expect([...container.querySelectorAll("button")].some(b => b.textContent === "See the result")).toBe(false);
  await publish([run(runID, "artifact-one")]);
  expect(button("See the result").className).toContain("vibe-button-primary");
  expect(button("Run again").className).toContain("vibe-button-secondary");
  expect([...container.querySelectorAll("button")].some(b => b.textContent === "Run 3 tests")).toBe(false);
  await click("See the result");
  expect(params().get("view")).toBe("results");
  expect(params().get("run")).toBe(runID);
});

it.each([["build", "Apply change and rerun"], ["test", "Test this change"]] as const)("selects Guide's fix on the %s Agent view with the diff open and the rerun next to it, and runs nothing", async (door, rerun) => {
  readyTests(door);
  session.operations = [run(runID, "artifact-one")];
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  const fixID = "a8000000-0000-4000-8000-0000000000f1";
  respond = async path => {
    if (path === "/config") return twoDoorConfig();
    if (path.endsWith("/run-quote")) return json({ id: "quote", cases: 3, calls: 6, max_cost_nano_usd: 0, expires_at: new Date(Date.now() + 600000).toISOString() });
    if (path.endsWith("/messages")) return json({ id: fixID, kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }, 202);
    return path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  };
  await render();
  await click(door === "build" ? "Fix this" : "Suggest a fix");
  expect(posts()[0].body).toMatchObject({ kind: "message", purpose: "suggest_change", baseline_id: runID });
  expect(params().get("view")).toBe("agent");
  const v1 = session.document.artifacts[0];
  await publish([...session.operations, { id: fixID, kind: "message", state: "COMPLETED", billing: "SETTLED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }], {
    artifacts: [v1, { ...v1, id: "artifact-two", parent_id: v1.id, accepted: false, agent_prompt: "Original policy. Never promise a refund." }],
    messages: [{ id: "fix-reply", role: "assistant", content: "Here is a focused change.", operation_id: fixID, artifact_id: "artifact-two" }],
  });
  expect(params().get("agent")).toBe("artifact-two");
  expect(params().get("run")).toBeNull();
  expect(params().get("view")).toBe("agent");
  expect([...container.querySelectorAll("button")].some(b => b.textContent === "Review latest version")).toBe(false);
  const change = container.querySelector<HTMLDetailsElement>('[aria-label="Suggested change"] details')!;
  expect(change.open).toBe(true);
  expect(change.textContent).toContain("Never promise a refund.");
  expect(posts()).toHaveLength(1);
  await click(rerun);
  expect(requests.findLast(r => r.path.endsWith("/run-quote"))?.body).toMatchObject({ kind: "retest", baseline_id: runID, artifact_id: "artifact-two" });
  expect(dialogNames()).toEqual(["Run the same tests again"]);
  expect(posts()).toHaveLength(1);
});

it.each(["reload", "later snapshot"])("selects the core journey's message-linked fix without an operation in the snapshot (%s)", async delivery => {
  readyTests();
  const v1 = { ...session.document.artifacts[0], source_message_id: "original-brief" };
  const baseline = run(runID, v1.id);
  session.document.artifacts = [v1];
  session.operations = [baseline];
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  const proposal = { ...v1, id: "fixed", parent_id: v1.id, accepted: false, proposal_message_id: "reply-fixed", agent_prompt: "Original policy. Never promise a refund." };
  let messages: Session["document"]["messages"] = [];
  const initial = respond;
  respond = async (path, options) => {
    if (path.endsWith("/messages")) {
      const body = JSON.parse(String(options.body));
      messages = [
        { id: body.client_id, role: "user", content: body.content },
        { id: "reply-fixed", role: "assistant", content: "Review the change, then test it.", artifact_id: proposal.id },
      ];
      if (delivery === "reload") session = { ...session, revision: 2, event_cursor: 2, document: { ...session.document, artifacts: [v1, proposal], messages } };
      return json({ id: "prepare-fixed", state: "COMPLETED" }, 202);
    }
    return initial(path, options);
  };
  await render(); await click("Suggest a fix");
  if (delivery === "later snapshot") await publish([baseline], { artifacts: [v1, proposal], messages });
  expect(posts()[0].body).toMatchObject({ kind: "message", purpose: "suggest_change", baseline_id: runID });
  expect(session.operations.map(o => o.id)).toEqual([runID]);
  expect(proposal.source_message_id).toBe("original-brief");
  expect(params().get("agent")).toBe("fixed");
  expect(params().get("run")).toBeNull();
  expect(params().get("view")).toBe("agent");
  expect([...container.querySelectorAll("h2")].some(h => h.textContent === "Ready to test the fix")).toBe(true);
  expect(container.querySelector('[data-proposal-id="fixed"] [aria-label="Instruction changes"]')?.textContent).toContain("Never promise a refund.");
  expect(button("Test this change")).toBeTruthy();
  expect(posts()).toHaveLength(1);
});

it.each(["next user", "foreign operation"])("does not select a legacy proposal from another request (%s)", async mode => {
  readyTests();
  const baseline = run(runID, "artifact-one");
  session.operations = [baseline];
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  const initial = respond;
  respond = async (path, options) => path.endsWith("/messages") ? json({ id: "prepare-fixed", state: "COMPLETED" }, 202) : initial(path, options);
  await render(); await click("Suggest a fix");
  const selected = params().get("agent");
  const v1 = session.document.artifacts[0];
  await publish([baseline], {
    artifacts: [v1, { ...v1, id: "unrelated", parent_id: v1.id, proposal_message_id: "reply-unrelated", agent_prompt: "An unrelated edit." }],
    messages: [
      { id: String(posts()[0].body.client_id), role: "user", content: "Suggest a focused fix." },
      { id: "no-fix", role: "assistant", content: "I have no change to suggest." },
      ...(mode === "next user" ? [{ id: "another-request", role: "user" as const, content: "Edit my instructions." }] : []),
      { id: "reply-unrelated", role: "assistant", content: "Here is another edit.", artifact_id: "unrelated", ...(mode === "foreign operation" ? { operation_id: "another-operation" } : {}) },
    ],
  });
  expect(params().get("agent")).toBe(selected);
  expect(params().get("run")).toBe(runID);
  expect(button("Review latest version")).toBeTruthy();
  expect(posts()).toHaveLength(1);
});

it.each(["RUNNING", "FAILED", "CANCELLED", "COMPLETED"])("does not select another Guide operation's child as the %s fix", async state => {
  readyTests("build");
  session.operations = [run(runID, "artifact-one")];
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  const fix: Operation = { id: "fix-request", kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] };
  respond = async path => path === "/config" ? twoDoorConfig() : path.endsWith("/messages") ? json(fix, 202) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render(); await click("Fix this");
  const selected = params().get("agent");
  const v1 = session.document.artifacts[0];
  const operations = [...session.operations, { ...fix, state }];
  await publish(operations);
  const edit: Operation = { ...fix, id: "unrelated-edit", state: "COMPLETED", completion_receipt: { action: "edit_tests", source_message_id: "edit-request", artifact_id: "edited-tests", case_count: 3, changed_case_count: 1 } };
  await publish([...operations, edit], {
    artifacts: [v1, { ...v1, id: "edited-tests", parent_id: v1.id, source_message_id: "edit-request", agent_prompt: "Instructions from another request." }],
    messages: [{ id: "edit-reply", role: "assistant", content: "Updated your tests.", operation_id: edit.id, artifact_id: "edited-tests" }],
  });
  expect(params().get("view")).toBe("agent");
  expect(params().get("agent")).toBe(selected);
  expect(button("Review latest version")).toBeTruthy();
  expect(posts()).toHaveLength(1);
});

it.each(["nested", "explicit", "lost acknowledgment"])("selects the fix from its retry after the original fails (%s)", async mode => {
  const explicit = mode !== "nested";
  readyTests("build");
  const baseline = run(runID, "artifact-one");
  session.operations = [baseline];
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  const fix: Operation = { id: "fix-request", kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] };
  const retry = { ...fix, id: "fix-retry", retry_of_operation_id: fix.id };
  const nested = { ...retry, id: "fix-retry-again", retry_of_operation_id: retry.id };
  respond = async path => {
    if (path.endsWith("/retry")) {
      if (mode === "lost acknowledgment") throw new TypeError("Lost retry acknowledgment");
      return json(retry, 202);
    }
    return path === "/config" ? twoDoorConfig() : path.endsWith("/messages") ? json(fix, 202) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  };
  await render(); await click("Fix this");
  const failed = { ...fix, state: "FAILED", retryable: true, error: { code: "provider_error", message: "Provider failed. Retry your fix." } };
  const messages: Session["document"]["messages"] = [{ id: "fix-message", role: "user", content: "Suggest a focused fix.", operation_id: fix.id }];
  if (explicit) {
    await publish([baseline, failed], { messages });
    await click("Guide"); await click("Retry"); await click("Close Guide");
    await publish([baseline, failed, retry], { messages });
  } else await publish([baseline, failed, { ...retry, state: "FAILED" }, nested], { messages });
  const proposal = { ...session.document.artifacts[0], id: "retried-fix", parent_id: "artifact-one", agent_prompt: "Never promise a refund." };
  const completed = { ...(explicit ? retry : nested), state: "COMPLETED", completion_receipt: { action: "suggest_change", source_message_id: "fix-message", artifact_id: proposal.id, case_count: 3, changed_case_count: 0 } };
  await publish([...session.operations.filter(o => o.id !== completed.id), completed], { artifacts: [...session.document.artifacts, proposal] });
  expect(params().get("view")).toBe("agent");
  expect(params().get("agent")).toBe(proposal.id);
  expect(params().get("run")).toBeNull();
  expect(container.querySelector<HTMLDetailsElement>('[aria-label="Suggested change"] details')?.open).toBe(true);
  expect(posts()).toHaveLength(1);
});

it("leaves the pinned result in place when the user moved to Results before the fix arrived", async () => {
  readyTests("build");
  session.operations = [run(runID, "artifact-one")];
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  const fixID = "a8000000-0000-4000-8000-0000000000f2";
  respond = async path => path === "/config" ? twoDoorConfig() : path.endsWith("/messages") ? json({ id: fixID, kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }, 202) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  await click("Fix this");
  await click("Results");
  const v1 = session.document.artifacts[0];
  await publish(session.operations, { artifacts: [v1, { ...v1, id: "artifact-two", parent_id: v1.id, agent_prompt: "Original policy. Never promise a refund." }], messages: [{ id: "fix-reply", role: "assistant", content: "Here is a focused change.", operation_id: fixID, artifact_id: "artifact-two" }] });
  expect(params().get("view")).toBe("results");
  expect(params().get("run")).toBe(runID);
  expect(params().get("agent")).not.toBe("artifact-two");
  expect(button("Review latest version")).toBeTruthy();
});

// The trial draft belongs to the version and thread on screen: a fix that arrives while the user
// writes to the agent must not swap the version under the composer.
it.each([["build", true], ["test", true], ["build", false], ["test", false]] as const)("keeps the %s version a user writes to the agent about while Guide prepares the fix (focus in the composer: %s), and offers Review latest version", async (door, focused) => {
  readyTests(door);
  session.operations = [run(runID, "artifact-one")];
  harness.params = new URLSearchParams(`session=${session.id}&view=results&run=${runID}`);
  const fixID = "a8000000-0000-4000-8000-0000000000f3";
  respond = async path => path === "/config" ? twoDoorConfig() : path.endsWith("/messages") ? json({ id: fixID, kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }, 202) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  await click(door === "build" ? "Fix this" : "Suggest a fix");
  expect(params().get("view")).toBe("agent");
  const trial = document.getElementById("vibe-trial-message") as HTMLTextAreaElement;
  act(() => trial.focus());
  await type("Can I return opened headphones?", "Message your agent");
  if (!focused) act(() => trial.blur());
  const selection = { agent: params().get("agent"), thread: params().get("thread") };
  const v1 = session.document.artifacts[0];
  await publish([...session.operations, { id: fixID, kind: "message", state: "COMPLETED", billing: "SETTLED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }], {
    artifacts: [v1, { ...v1, id: "artifact-two", parent_id: v1.id, accepted: false, agent_prompt: "Original policy. Never promise a refund." }],
    messages: [{ id: "fix-reply", role: "assistant", content: "Here is a focused change.", operation_id: fixID, artifact_id: "artifact-two" }],
  }); await frame();
  expect(params().get("view")).toBe("agent");
  expect(params().get("agent")).toBe(selection.agent);
  expect(params().get("thread")).toBe(selection.thread);
  const composer = document.getElementById("vibe-trial-message") as HTMLTextAreaElement;
  expect(composer).toBe(trial);
  expect(composer.value).toBe("Can I return opened headphones?");
  if (focused) expect(document.activeElement).toBe(composer);
  // The user goes to the fix when ready; their draft stays with the version they wrote it for.
  await click("Review latest version");
  expect(params().get("agent")).toBe("artifact-two");
  expect(container.querySelector<HTMLDetailsElement>('[aria-label="Suggested change"] details')?.open).toBe(true);
  expect((document.getElementById("vibe-trial-message") as HTMLTextAreaElement).value).toBe("");
  expect(posts()).toHaveLength(1);
});

it("names the Build send button by its visible text (E2)", async () => {
  await enterDoor("build");
  const send = container.querySelector<HTMLButtonElement>("form button[type=submit]")!;
  expect(send.hasAttribute("aria-label")).toBe(false);
  expect(send.textContent).toBe("Build and run 3 tests");
});

it("names the Improve primary by its visible text (E2)", async () => {
  await enterDoor("test");
  expect(button("Write 3 tests").getAttribute("aria-label")).toBeNull();
});

it("keeps the first draft and shows busy only after committed input", async () => {
  await enterDoor("build");
  const previous = respond;
  const held = deferred<Response>();
  respond = async (path, options) => path === "/sessions" && options.method === "POST" ? held.promise : previous(path, options);
  await type("Answer return questions.");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
  await click("Build and run 3 tests");
  expect(button("Build and run 3 tests").disabled).toBe(true);
  expect(button("Build and run 3 tests").getAttribute("aria-busy")).toBe("true");
  expect(composer().value).toBe("Answer return questions.");
  expect(params().get("start")).toBe("build");
  await act(async () => held.resolve(json({ error: { code: "unavailable", message: "Start failed" } }, 503)));
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("Start failed");
  expect(composer().value).toBe("Answer return questions.");
  expect(container.querySelector(".vibe-project-header")).toBeNull();
});

it("keeps the Build estimate out of the chooser and shows one h1 there", async () => {
  await enterDoor("build");
  expect(shown('[aria-label="Build estimate"]')).toHaveLength(0);
  await click("Back"); await frame();
  expect(shown('[aria-label="Build estimate"]')).toHaveLength(0);
  expect(visibleH1().map(h1 => h1.textContent)).toEqual(["What would you like to evaluate?"]);
  expect(document.activeElement?.id).toBe("vibe-door-build");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("shows a failed run estimate once", async () => {
  readyTests();
  harness.params = new URLSearchParams(`session=${session.id}&view=tests`);
  respond = async path => path === "/config" ? twoDoorConfig() : path.endsWith("/run-quote") ? json({ error: { code: "unavailable", message: "Estimates are paused." } }, 503) : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  await click("Run 3 tests");
  expect(container.textContent?.split("Estimates are paused.").length).toBe(2);
  expect(container.textContent).toContain("Couldn't get the price. Estimates are paused.");
  expect(dialogNames()).toEqual([]);
  expect(posts()).toHaveLength(0);
});

it("keeps an empty Guide closed from 1280px and opens it once it has a conversation", async () => {
  vi.stubGlobal("matchMedia", vi.fn((query: string) => ({ matches: query === "(min-width: 1280px)", addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  readyTests();
  harness.params = new URLSearchParams(`session=${session.id}&view=tests`);
  respond = async path => path === "/config" ? twoDoorConfig() : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  const guide = () => container.querySelector('[aria-label="Guide utility"]');
  expect(guide()).toBeNull();
  expect(button("Guide").getAttribute("aria-pressed")).toBe("false");
  await publish(session.operations, { messages: [{ id: "reply", role: "assistant", content: "These tests cover your rules.", artifact_id: "artifact-one" }] });
  expect(guide()).not.toBeNull();
  expect(posts()).toHaveLength(0);
});

it("preserves both Improve sources through switching, Back and refresh", async () => {
  await enterDoor("test");
  await type("Keep these instructions.", "Agent instructions");
  await click("Real replies");
  await type("Customer: Hi\nAgent: Hello", "Conversations");
  await type("Be polite", "What a good reply must do");
  await click("Instructions");
  expect(container.querySelector<HTMLTextAreaElement>("#vibe-instructions-source")?.value).toBe("Keep these instructions.");
  await click("Back"); await frame();
  expect(document.activeElement?.id).toBe("vibe-door-test");
  await act(async () => buttonByDoor("test").click());
  await click("Real replies");
  expect(container.querySelector<HTMLTextAreaElement>('#vibe-start-conversations')?.value).toBe("Customer: Hi\nAgent: Hello");
  expect(container.querySelector<HTMLTextAreaElement>('#vibe-start-rules')?.value).toBe("Be polite");
  await act(async () => root.unmount()); root = createRoot(container); await render();
  expect(container.querySelector<HTMLTextAreaElement>('#vibe-start-rules')?.value).toBe("Be polite");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("returns focus to the project title from the chooser when the project has no composer (D3)", async () => {
  readyTests();
  respond = async path => path === "/config" ? twoDoorConfig() : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  expect(document.getElementById("vibe-message")).toBeNull();
  await click("New project");
  await click("Back to your project"); await frame();
  expect(document.activeElement).toBe(container.querySelector(".vibe-project-header h1"));
});

it("keeps an Improve credit rejection next to Run and opens Credits without another run", async () => {
  readyTests("test"); signedInWorkspace(); session.workspace_id = "workspace-one";
  harness.params = new URLSearchParams(`session=${session.id}&workspace=workspace-one&view=tests`);
  respond = async path => path === "/config" ? twoDoorConfig()
    : path.startsWith("/credits?") ? json({ products: [], available_nano_usd: 0, held_nano_usd: 0 })
    : path.endsWith("/run-quote") ? json({ id: "quote", cases: 3, calls: 6, max_cost_nano_usd: 0, expires_at: new Date(Date.now() + 600000).toISOString() })
    : path.endsWith("/messages") ? json({ error: { code: "insufficient_credits", message: "This workspace has no AI credits.", admission: "rejected" } }, 402)
    : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render(); await click("Run 3 tests"); await click("Start run");
  const near = container.querySelector('.vibe-tests-actions [role="alert"]');
  expect(near?.textContent).toContain("This workspace has no AI credits.");
  expect(near?.querySelector("button")?.textContent).toBe("Credits");
  expect(button("Run 3 tests").disabled).toBe(true);
  expect(container.textContent?.split("This workspace has no AI credits.")).toHaveLength(2);
  await act(async () => near!.querySelector<HTMLButtonElement>("button")!.click());
  expect(dialogNames()).toEqual(["Workspace AI credits"]);
  expect(posts()).toHaveLength(1);
});

it("ends a compared Improve run with one Copy primary and the exact new instructions", async () => {
  const clipboard = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: clipboard } });
  const original = readyTests("test");
  const updated = { ...original, id: "compared-version", parent_id: original.id, agent_prompt: "Exact improved policy." };
  const baseline = run(runID, original.id, { grading: { version: 1, hash: "fixed-grading" } });
  const current = run("compared-result", updated.id, { kind: "retest", baseline_id: baseline.id, grading: baseline.grading,
    results: [{ ...baseline.results[0], version: updated.id, verdict: "PASS", output: "Correct reply", checks: [{ key: "behavior", verdict: "PASS", evidence: "Followed the rule." }] }],
    scorecard: { passed: 1, failed: 0, unknown: 0, total: 1, evaluated: 1, pass_rate: 1, coverage: 1 } });
  session.document.artifacts.push(updated); session.operations = [baseline, current];
  harness.params = new URLSearchParams(`session=${session.id}&agent=${updated.id}&view=results&run=${current.id}`);
  respond = async path => path === "/config" ? twoDoorConfig()
    : path.includes("/case") ? json(path.includes(baseline.id) ? baseline.results[0] : current.results[0])
    : path.endsWith("/run-quote") ? json({ id: "quote", cases: 1, calls: 2, max_cost_nano_usd: 0, expires_at: new Date(Date.now() + 600000).toISOString() })
    : path.endsWith("/grade-reviews") || path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render();
  expect(container.textContent).toContain("1 fixed · 0 new failures. Same tests and grading model.");
  expect(container.textContent).toContain("Paste them into your app. We don't change your live app.");
  expect([...container.querySelectorAll('.vibe-project-main .vibe-button-primary')].map(button => button.textContent?.trim())).toEqual(["Copy the new instructions"]);
  await click("Copy the new instructions"); expect(clipboard).toHaveBeenCalledWith(updated.agent_prompt);
  expect(posts()).toHaveLength(0);
});

it("runs an imported pack after inline instructions without requesting a price before Run", async () => {
  const suite = readyTests("test"); suite.agent_prompt = "";
  harness.params = new URLSearchParams(`session=${session.id}&view=tests&agent=${suite.id}`);
  const blueprint = structuredClone(suite.blueprint);
  respond = async (path, options) => {
    if (path === "/config") return twoDoorConfig();
    if (path.endsWith("/run-quote")) return json({ id: "pack-quote", cases: 3, calls: 6, max_cost_nano_usd: 120000000, expires_at: new Date(Date.now() + 600000).toISOString() });
    if (options.method === "PATCH") {
      const body = JSON.parse(String(options.body));
      const bound = { ...suite, id: "bound-import", parent_id: suite.id, agent_prompt: body.agent_prompt };
      session = { ...session, revision: session.revision + 1, document: { ...session.document, artifacts: [...session.document.artifacts, bound] } };
      return json(session);
    }
    if (path.endsWith("/messages")) return json({ id: "pack-run", kind: "check", state: "QUEUED", models: defaultModels, results: [] }, 202);
    return path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  };
  await render();
  expect(button("Add your agent's instructions").disabled).toBe(true);
  await type("Use our actual return policy.", "Agent instructions");
  expect(button("Add your agent's instructions").disabled).toBe(false);
  await click("Add your agent's instructions");
  expect(session.document.artifacts.at(-1)?.blueprint).toEqual(blueprint);
  expect(button("Run 3 tests").disabled).toBe(false);
  expect(button("Run 3 tests").textContent?.trim()).toBe("Run 3 tests");
  expect(requests.filter(r => r.path.endsWith("/run-quote"))).toHaveLength(0);
  await click("Run 3 tests");
  expect(dialogNames()).toEqual(["Run these tests"]);
  expect(document.querySelector('[data-slot="dialog-description"]')?.textContent).toBe("3 tests · up to $0.12.");
  expect(requests.filter(r => r.path.endsWith("/run-quote"))).toHaveLength(1);
  expect(posts()).toHaveLength(0);
  await click("Start run");
  expect(posts()).toHaveLength(1);
  expect(posts()[0].body).toMatchObject({ kind: "check", artifact_id: "bound-import", run_quote_id: "pack-quote", approve_artifact: true });
});

it("keeps a quote credit rejection beside Run and retries only after the user's click", async () => {
  readyTests("test"); signedInWorkspace(); session.workspace_id = "workspace-one";
  harness.params = new URLSearchParams(`session=${session.id}&workspace=workspace-one&view=tests`);
  let estimates = 0;
  respond = async path => path === "/config" ? twoDoorConfig()
    : path.endsWith("/run-quote") ? ++estimates === 1
      ? json({ error: { code: "insufficient_credits", message: "No credits available." } }, 402)
      : json({ id: "funded-quote", cases: 3, calls: 6, max_cost_nano_usd: 0, expires_at: new Date(Date.now() + 600000).toISOString() })
    : path === "/sessions" || path === "/saved-checks" ? json([]) : json(session);
  await render(); expect(estimates).toBe(0);
  await click("Run 3 tests");
  const near = container.querySelector('.vibe-tests-actions [role="alert"]');
  expect(near?.textContent).toContain("No credits available.");
  expect(container.textContent?.split("No credits available.")).toHaveLength(2);
  expect(button("Run 3 tests").disabled).toBe(true);
  await act(async () => near!.querySelector<HTMLButtonElement>("button")!.click());
  expect(dialogNames()).toEqual(["Workspace AI credits"]);
  await click("Close credits");
  expect(estimates).toBe(1); expect(posts()).toHaveLength(0);
  await act(async () => [...near!.querySelectorAll("button")].find(b => b.textContent === "Try again")!.click());
  expect(estimates).toBe(2); expect(dialogNames()).toEqual(["Run these tests"]);
  expect(button("Start run").disabled).toBe(false); expect(posts()).toHaveLength(0);
});

it.each([1, 2])("sends first Build clarification choices through messages with the active cycle (selections: %s)", async count => {
  session.document.evaluation = { id: session.id, chat_id: session.id, door: "build" };
  session.document.build = { cycle_id: "active-build-cycle", phase: "clarifying", clarifications_used: 1 };
  session.document.messages = [{ id: "question", role: "assistant", content: "Which return rules apply?" }];
  session.document.conversation_state = { version: 1, actions_version: 1, through_message_id: "question", brief: { scope_id: "scope", revision: 1, facts: [] }, guidance: {}, pending_question: { id: "q", revision: 1, scope_id: "scope", origin_message_id: "question", purpose: "clarify_rule", status: "active", text: "Which return rules apply?", options: [{ id: "30", label: "30 days" }, { id: "sealed", label: "Unopened items" }], max_selections: count, proposal_id: null, proposal_revision: null } };
  const before = respond;
  respond = async (path, options) => {
    if (path === "/config") return json({ enabled: true, two_door: true, interaction_actions: true, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id })), availability: { guest: { available: true }, workspace: { available: true } } });
    if (path.endsWith("/messages")) return json({ id: "answer-admitted", kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }, 202);
    return before(path, options);
  };
  await render();
  if (count === 1) await click("30 days");
  else {
    await act(async () => container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach(input => input.click()));
    await click("Use these answers");
  }
  const writes = requests.filter(request => request.method === "POST");
  expect(writes).toHaveLength(1);
  expect(writes[0].path).toBe(`/sessions/${session.id}/messages`);
  expect(writes[0].body).toMatchObject({ kind: "message", content: count === 1 ? "30 days" : "30 days\nUnopened items", cycle_id: "active-build-cycle", revision: 1, models: defaultModels });
  expect(writes[0].body).not.toHaveProperty("interaction");
  expect(writes[0].body).not.toHaveProperty("payload");
});


it.each(["test", "build"] as const)("keeps clarification choices on actions for %s after the initial Build", async door => {
  session.document.evaluation = { id: session.id, chat_id: session.id, door };
  if (door === "build") {
    session.document.artifacts = [{ id: "existing-agent", kind: "test_suite", title: "Returns", agent_prompt: "Follow the policy.", blueprint: {}, accepted: true, source_message_id: "task" }];
    session.document.build = { cycle_id: "later-cycle", phase: "clarifying", clarifications_used: 1, artifact_id: "existing-agent" };
  }
  session.document.messages = [{ id: "question", role: "assistant", content: "30 days?" }];
  session.document.conversation_state = { version: 1, actions_version: 1, through_message_id: "question", brief: { scope_id: "scope", revision: 1, facts: [] }, guidance: {}, pending_question: { id: "q", revision: 1, scope_id: "scope", origin_message_id: "question", purpose: "clarify_rule", status: "active", text: "30 days?", options: [{ id: "30", label: "30 days" }], max_selections: 1, proposal_id: null, proposal_revision: null } };
  const before = respond;
  respond = async (path, options) => path === "/config" ? json({ enabled: true, interaction_actions: true, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id })), availability: { guest: { available: true }, workspace: { available: true } } }) : before(path, options);
  await render(); await click("30 days");
  const writes = requests.filter(request => request.method === "POST");
  expect(writes).toHaveLength(1);
  expect(writes[0].path).toBe(`/sessions/${session.id}/actions`);
  expect(writes[0].body).toMatchObject({ version: 1, kind: "action", payload: { kind: "answer_question", option_ids: ["30"], target_id: "q" } });
});

function buttonByDoor(door: "build" | "test") { return container.querySelector<HTMLButtonElement>(`#vibe-door-${door}`)!; }

it("PR4 creates a titled Build only on submit with a quote-bound first message and grows the frame", async () => {
  await enterDoor("build");
  await type("Answer return questions. Keep this second sentence out of the title.");
  await click("Build and run 3 tests");
  expect(requests.find(r => r.path === "/sessions" && r.method === "POST")?.body).toMatchObject({ door: "build", title: "Answer return questions." });
  expect(posts()[0].body).toMatchObject({ cycle_id: "quote", kind: "message", evaluation_first: true, test_journey: true });
  expect(params().get("start")).toBeNull(); expect(params().get("session")).toBe(session.id);
  expect(container.querySelector(".vibe-project-header h1")).not.toBeNull();
  expect(container.querySelector('[aria-label="Workspace views"]')).toBeNull();
  expect(container.querySelector('[aria-label="Details"]')).toBeNull();
  expect(container.querySelector('[aria-label="Guide"]')).toBeNull();
  await publish(session.operations, { active_artifact_id: "first-artifact", artifacts: [{ id: "first-artifact", kind: "test_suite", title: "First tests", agent_prompt: "Answer questions", blueprint: { input_sets: [] }, accepted: false, source_message_id: "" }] });
  expect(container.querySelector('[aria-label="Workspace views"]')).not.toBeNull();
  expect(button("Details")).toBeTruthy(); expect(button("Guide")).toBeTruthy();
});

it("PR4 submits instructions and model with a title limited to 60 characters", async () => {
  await enterDoor("test");
  const text = "a".repeat(85);
  await type(text, "Agent instructions");
  await click("Write 3 tests");
  expect(requests.find(r => r.path === "/sessions" && r.method === "POST")?.body).toMatchObject({ door: "test", title: "a".repeat(60) });
  expect(posts()[0].body).toMatchObject({ instructions: text, content: `Write three tests for these instructions.\n\nAgent instructions to test:\n${text}`, models: defaultModels });
  expect(requests.some(r => r.path.endsWith("/build-quote"))).toBe(false);
});

it("PR4 cleans up a failed first message and retains the door draft across a switch", async () => {
  await enterDoor("build");
  const previous = respond;
  respond = async (path, options) => path.endsWith("/messages") ? json({ error: { code: "unavailable", message: "Try again later", admission: "rejected" } }, 503) : previous(path, options);
  await type("Keep this Build draft."); await click("Build and run 3 tests");
  expect(requests.some(r => r.method === "DELETE" && r.path === `/sessions/${session.id}`)).toBe(true);
  expect(params().get("session")).toBeNull();
  expect(container.querySelector('[role="alert"]')?.textContent).toBe("Try again later");
  expect(composer().value).toBe("Keep this Build draft.");
  await click("Already have an agent? Improve it instead.");
  await type("Independent instructions", "Agent instructions");
  await click("Build an agent instead");
  expect(composer().value).toBe("Keep this Build draft.");
});

it.each([undefined, false, true])("PR4 gates a used guest trial before typing (used=%s)", async used => {
  await enterDoor("build");
  const previous = respond;
  respond = async (path, options) => path === "/config" ? json({ ...(await twoDoorConfig().json()), ...(used === undefined ? {} : { guest_trial: { initial_build_used: used, build_in_progress: false } }) }) : previous(path, options);
  await act(async () => root.unmount()); root = createRoot(container); await render();
  if (used) { expect(composer().value).toBe(""); expect(button("Build and run 3 tests").disabled).toBe(true); expect(container.textContent).toContain("Your free build is used."); expect(container.querySelector('a[href*="auth/login"]')?.textContent).toBeTruthy(); }
  else { expect(composer()).not.toBeNull(); expect(container.textContent).toContain(used === false ? "Free trial" : "Price available when you start"); }
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("PR4 saves recorded replies as the first input with explicit expectations", async () => {
  await enterDoor("test");
  const previous = respond;
  respond = async (path, options) => {
    if (!path.endsWith("/evidence")) return previous(path, options);
    const body = JSON.parse(String(options.body));
    session = { ...session, document: { ...session.document, artifacts: [{ id: "reply-tests", kind: "conversation_evaluation", title: "Replies", agent_prompt: "", blueprint: {}, accepted: true, source_message_id: "", conversation_evaluation: { evidence_set_id: "replies", expectations: body.expectations } }] }, evidence_receipt: { client_id: body.client_id, artifact_id: "reply-tests", evidence_id: "replies" } };
    return json(session);
  };
  await click("Real replies");
  await type("Customer: Hi\nAgent: Hello", "Conversations"); await type("Be polite\n\nDo not invent facts", "What a good reply must do");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
  await click("Save and check");
  expect(requests.find(r => r.path === "/sessions" && r.method === "POST")?.body).toMatchObject({ door: "test", title: "Recorded replies" });
  expect(requests.find(r => r.path.endsWith("/evidence"))?.body).toMatchObject({ content: "Customer: Hi\nAgent: Hello", expectations: [{ id: "rule-1", statement: "Be polite" }, { id: "rule-2", statement: "Do not invent facts" }] });
  expect(posts()).toHaveLength(0); expect(params().get("view")).toBe("tests"); expect(params().get("agent")).toBe("reply-tests");
});

it("PR4 imports a pack on selection and derives its title from the filename", async () => {
  await enterDoor("test");
  const previous = respond;
  respond = async (path, options) => { if (!path.endsWith("/import")) return previous(path, options); session = { ...session, document: { ...session.document, artifacts: [{ id: "pack-tests", kind: "test_suite", title: "Pack", agent_prompt: "", accepted: false, source_message_id: "", blueprint: { input_sets: [] } }] } }; return json(session); };
  await click("Test pack");
  const file = new File(['{"cases":[]}'], "Return checks.yaml", { type: "application/yaml" });
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  await act(async () => { Object.defineProperty(input, "files", { value: [file], configurable: true }); input.dispatchEvent(new Event("change", { bubbles: true })); });
  expect(requests.find(r => r.path === "/sessions" && r.method === "POST")?.body).toMatchObject({ door: "test", title: "Return checks" });
  expect(requests.some(r => r.path.endsWith("/import") && r.method === "POST")).toBe(true);
  expect(posts()).toHaveLength(0); expect(params().get("view")).toBe("tests"); expect(params().get("agent")).toBe("pack-tests");
});

it("PR4 shows the configured first-build ceiling for a signed-in user", async () => {
  await enterDoor("build"); signedInWorkspace();
  const previous = respond;
  respond = async (path, options) => path === "/config" ? json({ ...(await twoDoorConfig().json()), first_build_ceiling_nano_usd: 500000000 }) : previous(path, options);
  await render(); expect(container.textContent).toContain("Up to $0.50");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("PR4 opens the sample in one sheet without creating a project", async () => {
  harness.params = new URLSearchParams(); respond = async path => path === "/config" ? twoDoorConfig() : json([]);
  await render(); await click("Read a sample test");
  expect(dialogNames()).toEqual(["Sample: a return policy assistant"]);
  expect(document.body.textContent).toContain("opened items are ineligible");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("PR4 keeps Projects available from the compact top bar on entry and start", async () => {
  await enterDoor("build");
  await click("Open projects");
  expect(dialogNames()).toEqual(["Your projects"]);
  expect(document.body.textContent).toContain("No projects yet.");
  await click("Saved work");
  expect(dialogNames()).toEqual(["Your saved work"]);
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("PR4 hides a failed first input after reload when server cleanup is unavailable", async () => {
  await enterDoor("build");
  const previous = respond;
  respond = async (path, options) => path.endsWith("/messages") || options.method === "DELETE" ? json({ error: { code: "unavailable", message: "Try again", admission: "rejected" } }, 503) : path === "/sessions" && options.method !== "POST" ? json([session]) : previous(path, options);
  await type("Keep this failed brief."); await click("Build and run 3 tests");
  expect(container.querySelector(".vibe-project-header")).toBeNull();
  await act(async () => root.unmount()); root = createRoot(container); await render();
  expect(container.querySelector(".vibe-sidebar")).toBeNull();
  expect(container.querySelector(".vibe-project-header")).toBeNull();
  expect(composer().value).toBe("Keep this failed brief.");
});

it("PR4 admits only one project when the first submit is repeated", async () => {
  await enterDoor("build"); await type("Answer return questions.");
  const previous = respond, held = deferred<Response>();
  respond = async (path, options) => path === "/sessions" && options.method === "POST" ? held.promise : previous(path, options);
  const form = container.querySelector("form")!;
  await act(async () => { form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })); form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })); });
  expect(requests.filter(r => r.path === "/sessions" && r.method === "POST")).toHaveLength(1);
  await act(async () => held.resolve(json({ error: { code: "unavailable", message: "Try again" } }, 503)));
});

it("PR4 explains a busy guest trial before its first submit", async () => {
  await enterDoor("build");
  const previous = respond;
  respond = async (path, options) => path === "/config" ? json({ ...(await twoDoorConfig().json()), guest_trial: { initial_build_used: false, build_in_progress: true } }) : previous(path, options);
  await act(async () => root.unmount()); root = createRoot(container); await render();
  await type("Keep this task.");
  expect(button("Build and run 3 tests").disabled).toBe(true);
  expect(container.textContent).toContain("Another build is using your free trial.");
  expect(requests.filter(r => r.method === "POST")).toHaveLength(0);
});

it("PR4 uses the selected approved model for the first instructions", async () => {
  await enterDoor("test");
  const previous = respond;
  respond = async (path, options) => path === "/config" ? json({ ...(await twoDoorConfig().json()), models: [...new Set(Object.values(defaultModels)), "approved/other"].map(id => ({ id, name: id })) }) : previous(path, options);
  await act(async () => root.unmount()); root = createRoot(container); await render();
  await type("Answer return questions", "Agent instructions");
  const model = container.querySelector<HTMLSelectElement>(".vibe-improve-form select")!;
  await act(async () => { model.value = "approved/other"; model.dispatchEvent(new Event("change", { bubbles: true })); });
  await click("Write 3 tests");
  expect(posts()[0].body.models).toEqual({ ...defaultModels, target: "approved/other" });
});


it.each(["build", "test"] as const)("PR4 navigates the %s start with a stale Next snapshot, reloads and restores Back focus", async door => {
  harness.staleRouter = true;
  await enterDoor(door);
  const heading = door === "build" ? "What task should the agent handle?" : "How do you want to test your agent?";
  expect(harness.params.get("start")).toBeNull();
  expect(params().get("start")).toBe(door === "build" ? "build" : "improve");
  expect(visibleH1().map(h => h.textContent)).toEqual([heading]);
  expect(document.activeElement).toBe(door === "build" ? composer() : visibleH1()[0]);
  expect(container.querySelector(".vibe-project-header")).toBeNull();
  await act(async () => { await new Promise<void>(resolve => { window.addEventListener("popstate", () => resolve(), { once: true }); window.history.back(); }); });
  await frame();
  expect(document.activeElement).toBe(buttonByDoor(door));
  await act(async () => { await new Promise<void>(resolve => { window.addEventListener("popstate", () => resolve(), { once: true }); window.history.forward(); }); });
  expect(visibleH1().map(h => h.textContent)).toEqual([heading]);
  expect(document.activeElement).toBe(door === "build" ? composer() : visibleH1()[0]);
  await act(async () => root.unmount());
  root = createRoot(container);
  await render();
  expect(visibleH1().map(h => h.textContent)).toEqual([heading]);
  expect(document.activeElement).toBe(door === "build" ? composer() : visibleH1()[0]);
  await click("Back"); await frame();
  expect(visibleH1().map(h => h.textContent)).toEqual(["What would you like to evaluate?"]);
  expect(document.activeElement).toBe(buttonByDoor(door));
  expect(requests.filter(r => r.method === "POST" && r.path === "/sessions")).toEqual([]);
});

it("PR4 renders three whole Build preview steps with a short accessible door name", async () => {
  harness.params = new URLSearchParams();
  const previous = respond; respond = async (path, options) => path === "/config" ? twoDoorConfig() : previous(path, options);
  await render();
  const door = buttonByDoor("build");
  expect(door.getAttribute("aria-labelledby")).toBe("vibe-door-build-title");
  expect(document.getElementById(door.getAttribute("aria-labelledby")!)?.textContent).toBe("Build an agent");
  const steps = [...door.querySelectorAll("ol > li")];
  expect(steps.map(step => step.textContent)).toEqual(["01Describe the task", "02We draft the agent", "03Three tests run"]);
  expect(steps.map(step => step.querySelector("b")?.textContent)).toEqual(["01", "02", "03"]);
});


it("PR4 round2 waits for auth before config and keeps doors blocked through slow config and retry", async () => {
  harness.params = new URLSearchParams(); harness.authLoading = true;
  const held = deferred<Response>(); let reads = 0;
  respond = async path => path === "/config" ? (++reads === 1 ? held.promise : twoDoorConfig()) : json([]);
  await render();
  expect(reads).toBe(0);
  expect(buttonByDoor("build").disabled).toBe(true);
  expect(buttonByDoor("test").disabled).toBe(true);
  harness.authLoading = false; await render();
  expect(reads).toBe(1);
  expect(buttonByDoor("build").disabled).toBe(true);
  await act(async () => held.resolve(json({ error: { message: "Offline" } }, 503)));
  expect(container.textContent).toContain("Couldn’t connect. Try again.");
  expect(buttonByDoor("build").disabled).toBe(true);
  expect(buttonByDoor("test").disabled).toBe(true);
  await click("Retry connection");
  expect(reads).toBe(2);
  expect(buttonByDoor("build").disabled).toBe(false);
  expect(buttonByDoor("test").disabled).toBe(false);
  expect(requests.filter(r => r.method !== "GET")).toEqual([]);
});

it("PR4 round2 preserves the restored offline draft until explicit connection retry", async () => {
  harness.authLoading = true;
  session.document.evaluation = { id: session.id, chat_id: session.id, door: "build" };
  sessionStorage.setItem(`vibe-build-drafts:${session.id}`, JSON.stringify({ version: 1, guide: "Keep this unsent draft.", trials: {} }));
  let reads = 0;
  respond = async path => path === "/config" ? (++reads === 1 ? json({ error: { message: "Offline" } }, 503) : twoDoorConfig()) : path === "/sessions" || path === "/saved-checks" ? json([]) : path.endsWith("/build-quote") ? json({ id: "draft-quote", request: { content: "Keep this unsent draft.", models: defaultModels }, expires_at: new Date(Date.now() + 600000).toISOString(), cases: 3, max_calls: 4, max_cost_nano_usd: 0 }) : json(session);
  await render(); harness.authLoading = false; await render();
  expect(reads).toBe(1);
  expect(composer().value).toBe("Keep this unsent draft.");
  expect([...container.querySelectorAll('[role="alert"]')].some(node => node.textContent?.includes("Your message is still here."))).toBe(true);
  expect(requests.filter(r => r.method !== "GET")).toEqual([]);
  await click("Retry connection");
  expect(reads).toBe(2);
  expect(container.textContent).not.toContain("Couldn’t connect");
  expect(composer().value).toBe("Keep this unsent draft.");
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 450)); });
  expect(requests.filter(r => r.method !== "GET").map(r => r.path)).toEqual([`/sessions/${session.id}/build-quote`]);
});

it("PR4 round2 restores the chosen workspace and failed Build draft after auth resolves on reload", async () => {
  await enterDoor("build");
  signedInWorkspace();
  harness.me.mockResolvedValue({ organizations: [{ role: "org_admin", workspaces: [{ id: "workspace-one", name: "One" }, { id: "workspace-two", name: "Two" }] }] });
  const previous = respond;
  respond = async (path, options) => path.endsWith("/messages") ? json({ error: { code: "insufficient_credits", message: "This workspace has no AI credits.", admission: "rejected" } }, 402) : previous(path, options);
  await render(); await click("Back");
  const picker = container.querySelector<HTMLSelectElement>('[aria-label="Workspace for new projects"]')!;
  await act(async () => { picker.value = "workspace-two"; picker.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => buttonByDoor("build").click());
  await type("Keep this rejected Build."); await click("Build and run 3 tests");
  expect(container.textContent).toContain("This workspace has no AI credits.");
  expect(button("Build and run 3 tests").disabled).toBe(false);
  expect(params().get("workspace")).toBe("workspace-two");
  const writes = requests.filter(r => r.method !== "GET").length;
  await act(async () => root.unmount()); root = createRoot(container);
  harness.user = undefined; harness.authLoading = true; await render();
  signedInWorkspace();
  harness.me.mockResolvedValue({ organizations: [{ role: "org_admin", workspaces: [{ id: "workspace-one", name: "One" }, { id: "workspace-two", name: "Two" }] }] });
  harness.authLoading = false; await render();
  expect(composer().value).toBe("Keep this rejected Build.");
  expect(button("Build and run 3 tests").disabled).toBe(false);
  expect(requests.filter(r => r.method !== "GET")).toHaveLength(writes);
});


it("PR4 round2 blocks both doors when refreshing previously loaded config fails", async () => {
  harness.params = new URLSearchParams();
  respond = async path => path === "/config" ? twoDoorConfig() : json([]);
  await render(); expect(buttonByDoor("build").disabled).toBe(false);
  signedInWorkspace();
  respond = async path => path === "/config" ? json({ error: { message: "Offline" } }, 503) : json([]);
  await render();
  expect(container.textContent).toContain("Couldn’t connect. Try again.");
  expect(buttonByDoor("build").disabled).toBe(true);
  expect(buttonByDoor("test").disabled).toBe(true);
  expect(requests.filter(r => r.method !== "GET")).toEqual([]);
});

it.each(["build", "test"] as const)("PR4 round4 recovers a committed %s first message after its acknowledgement is lost", async door => {
  await enterDoor(door);
  const previous = respond;
  respond = async (path, options) => {
    if (path.endsWith("/messages")) {
      const body = JSON.parse(String(options.body));
      const operation = { id: "first-committed", kind: "message", state: "QUEUED", models: body.models, results: [] } as Operation;
      session = { ...session, revision: 2, event_cursor: 2, operations: [operation], document: { ...session.document, messages: [{ id: body.client_id, role: "user", content: body.content, operation_id: operation.id }] } };
      throw new TypeError("Connection closed after commit");
    }
    return previous(path, options);
  };
  await type("Keep one paid operation.", door === "build" ? "Message Vibe Evals" : "Agent instructions");
  await click(door === "build" ? "Build and run 3 tests" : "Write 3 tests");
  expect(requests.filter(r => r.method === "DELETE")).toEqual([]);
  expect(params().get("session")).toBe(session.id);
  expect(container.querySelector(".vibe-project-header")).not.toBeNull();
  expect(container.textContent).toContain("Keep one paid operation.");
  expect(container.textContent).not.toContain("Retry submission");
  expect(posts()).toHaveLength(1);
});

it("PR4 round4 keeps an uncertain first request visible and replays the exact body after a later rejection", async () => {
  await enterDoor("build");
  const previous = respond;
  let attempts = 0, lost = false;
  respond = async (path, options) => {
    if (path.endsWith("/messages")) {
      attempts++;
      if (attempts === 1) {
        lost = true;
        const body = JSON.parse(String(options.body));
        session = { ...session, revision: 2, event_cursor: 2, operations: [{ id: "first-committed", kind: "message", state: "QUEUED", models: body.models, results: [] } as Operation], document: { ...session.document, messages: [{ id: body.client_id, role: "user", content: body.content, operation_id: "first-committed" }] } };
        throw new TypeError("Connection closed after commit");
      }
      if (attempts === 2) return json({ error: { code: "unavailable", message: "Retry unavailable", admission: "rejected" } }, 503);
      lost = false;
      return json({ id: "first-committed", kind: "message", state: "QUEUED", models: defaultModels, results: [] }, 202);
    }
    if (lost && path === `/sessions/${session.id}` && !options.method) throw new TypeError("Receipt read offline");
    return previous(path, options);
  };
  await type("Keep this request identity."); await click("Build and run 3 tests");
  expect(requests.filter(r => r.method === "DELETE")).toEqual([]);
  expect(params().get("session")).toBe(session.id);
  expect(container.querySelector(".vibe-project-header")).not.toBeNull();
  await click("Retry submission");
  expect(container.textContent).toContain("Retry submission");
  await click("Retry submission");
  expect(posts()).toHaveLength(3);
  expect(posts()[1]).toEqual(posts()[0]); expect(posts()[2]).toEqual(posts()[0]);
  expect(requests.filter(r => r.path === "/sessions" && r.method === "POST")).toHaveLength(1);
  expect(requests.filter(r => r.method === "DELETE")).toEqual([]);
  expect(container.textContent).not.toContain("Retry submission");
});

it.each(["build", "test"] as const)("PR4 round4 preserves the guest %s draft through sign-in and reload", async door => {
  await enterDoor(door);
  await type("Guest text survives sign-in.", door === "build" ? "Message Vibe Evals" : "Agent instructions");
  signedInWorkspace(); await render();
  expect(door === "build" ? composer().value : container.querySelector<HTMLTextAreaElement>("#vibe-instructions-source")?.value).toBe("Guest text survives sign-in.");
  await act(async () => root.unmount()); draftMemory.clear(); root = createRoot(container); await render();
  expect(door === "build" ? composer().value : container.querySelector<HTMLTextAreaElement>("#vibe-instructions-source")?.value).toBe("Guest text survives sign-in.");
  expect(requests.filter(r => r.method !== "GET")).toEqual([]);
});

it.each(["build", "test"] as const)("PR4 round4 signs in back to the selected %s start", async door => {
  await enterDoor(door);
  const link = container.querySelector<HTMLAnchorElement>('.vibe-entry-topbar a[href*="auth/login"]')!;
  expect(new URL(link.href).searchParams.get("returnTo")).toBe(`/vibe-evals?start=${door === "build" ? "build" : "improve"}`);
});

it("PR4 round4 uses Settings models for Build quotes and retries", async () => {
  await enterDoor("build"); signedInWorkspace();
  const previous = respond;
  let quotes = 0;
  respond = async (path, options) => {
    if (path === "/config") return json({ ...(await twoDoorConfig().json()), models: [...new Set(Object.values(defaultModels)), "approved/other"].map(id => ({ id, name: id })) });
    if (path.endsWith("/build-quote") && ++quotes === 1) return json({ error: { code: "cost_limit", message: "Choose cheaper models." } }, 400);
    return previous(path, options);
  };
  await render(); await type("Use my chosen model.");
  await click("Build and run 3 tests");
  expect(container.textContent).toContain("Choose cheaper models.");
  await selectModel("Assistant", "approved/other");
  await click("Build and run 3 tests");
  const quote = requests.filter(r => r.path.endsWith("/build-quote")).at(-1)!;
  expect(quote.body.models).toEqual({ ...defaultModels, assistant: "approved/other" });
  expect(posts()[0].body.models).toEqual({ ...defaultModels, assistant: "approved/other" });
});

it.each([["build", "drawer"], ["test", "drawer"], ["build", "sidebar"], ["test", "sidebar"]] as const)("PR4 round4 opens New project from the uncommitted %s start via %s", async (door, surface) => {
  await enterDoor(door);
  await type("Keep the uncommitted input.", door === "build" ? "Message Vibe Evals" : "Agent instructions");
  if (surface === "sidebar") { signedInWorkspace(); await render(); }
  else await click("Open projects");
  await click("New project");
  expect(visibleH1().map(h => h.textContent)).toEqual(["What would you like to evaluate?"]);
  expect(params().get("start")).toBeNull();
  await act(async () => buttonByDoor(door).click());
  expect(door === "build" ? composer().value : container.querySelector<HTMLTextAreaElement>("#vibe-instructions-source")?.value).toBe("Keep the uncommitted input.");
  expect(requests.filter(r => r.method !== "GET")).toEqual([]);
});

it("PR4 round4 names the Build input with its visible label", async () => {
  await enterDoor("build");
  const input = container.querySelector<HTMLTextAreaElement>("#vibe-start-build")!;
  const label = document.getElementById(input.getAttribute("aria-labelledby")!)!;
  expect(input.hasAttribute("aria-label")).toBe(false);
  expect(label.textContent).toBe("What task should the agent handle?");
  expect(visibleH1()).toContain(label);
});

it("PR4 round4 reconciles an uncertain first request from a later saved snapshot without resending", async () => {
  await enterDoor("build");
  const previous = respond;
  respond = async (path, options) => {
    if (path.endsWith("/messages")) throw new TypeError("Lost first response");
    return previous(path, options);
  };
  await type("Save one first request."); await click("Build and run 3 tests");
  expect(requests.filter(r => r.method === "DELETE")).toEqual([]);
  expect(button("Retry submission").disabled).toBe(false);
  const body = posts()[0].body;
  const operation = { id: "saved-first", kind: "message", state: "QUEUED", models: defaultModels, results: [] } as Operation;
  await publish([operation], { messages: [{ id: String(body.client_id), role: "user", content: String(body.content), operation_id: operation.id }] });
  expect(container.textContent).not.toContain("Retry submission");
  expect(container.textContent).not.toContain("Lost first response");
  expect(posts()).toHaveLength(1);
  expect(requests.filter(r => r.path === "/sessions" && r.method === "POST")).toHaveLength(1);
  await click("New project"); await act(async () => buttonByDoor("build").click());
  expect(composer().value).toBe("");
});


it("lets a signed-in user explicitly choose a workspace before either new-project door without dispatching inference", async () => {
  signedInWorkspace(); harness.params = new URLSearchParams();
  harness.me.mockResolvedValue({ organizations: [{ role: "org_admin", workspaces: [{ id: "workspace-one", name: "One" }, { id: "workspace-two", name: "Two" }] }] });
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

it("keeps the name Send message for the icon-only composer (E2)", async () => {
  readyTests();
  const previous = respond;
  respond = async (path, options) => path === "/config" ? twoDoorConfig() : previous(path, options);
  await render(); await click("Guide");
  expect(document.getElementById("vibe-message")?.closest("form")?.querySelector("button[type=submit]")?.getAttribute("aria-label")).toBe("Send message");
});

it("moves focus into each Improve source panel and back to its source button (D3)", async () => {
  session.document.evaluation = { id: session.id, chat_id: session.id, door: "test" };
  const previous = respond;
  respond = async (path, options) => path === "/config" ? twoDoorConfig() : previous(path, options);
  await render();
  await click("Try my saved instructions");
  expect(document.activeElement?.id).toBe("vibe-instructions-source");
  await click("Back to conversation");
  expect(document.activeElement).toBe(button("Try my saved instructions"));
  await click("Test replies from my app");
  expect(document.activeElement?.getAttribute("aria-label")).toBe("Recorded replies");
  await click("Back");
  expect(document.activeElement).toBe(button("Test replies from my app"));
});

it("shows busy only while the first Improve input makes the project", async () => {
  await enterDoor("test");
  const held = deferred<Response>(), previous = respond;
  respond = async (path, options) => path === "/sessions" && options.method === "POST" ? held.promise : previous(path, options);
  await type("Answer questions about returns.", "Agent instructions");
  expect(button("Write 3 tests").getAttribute("aria-busy")).toBeNull();
  expect(requests.filter(r => r.method === "POST")).toEqual([]);
  await click("Write 3 tests");
  expect(button("Write 3 tests").disabled).toBe(true);
  expect(button("Write 3 tests").getAttribute("aria-busy")).toBe("true");
  expect(container.textContent).not.toContain("Saving your changes…");
  expect(container.querySelector<HTMLTextAreaElement>("#vibe-instructions-source")?.value).toBe("Answer questions about returns.");
  await act(async () => held.resolve(json({ error: { code: "unavailable", message: "Start failed" } }, 503)));
  expect(button("Write 3 tests").getAttribute("aria-busy")).toBeNull();
  expect(button("Write 3 tests").disabled).toBe(false);
  expect(container.textContent).toContain("How do you want to test your agent?");
});

it.each(["instructions", "replies"] as const)("keeps the full Improve %s start and all drafts after a rejected first input", async mode => {
  await enterDoor("test");
  expect(document.activeElement).toBe(visibleH1()[0]);
  await type("Keep my exact agent policy.", "Agent instructions");
  await click("Real replies");
  await type("Customer: Hi\nAgent: Hello", "Conversations");
  await type("Be polite", "What a good reply must do");
  if (mode === "instructions") await click("Instructions");
  expect(requests.filter(r => r.method !== "GET")).toEqual([]);
  const held = deferred<Response>(), previous = respond;
  respond = async (path, options) => path.endsWith(mode === "instructions" ? "/messages" : "/evidence") ? held.promise : previous(path, options);
  const primary = mode === "instructions" ? "Write 3 tests" : "Save and check";
  await click(primary);
  expect(button(primary).getAttribute("aria-busy")).toBe("true");
  expect(button(primary).disabled).toBe(true);
  await act(async () => held.resolve(json({ error: { code: "unavailable", message: "First input rejected", admission: "rejected" } }, 503)));
  expect(container.querySelector('.vibe-improve-start [role="alert"]')?.textContent).toBe("First input rejected");
  expect(button(primary).disabled).toBe(false);
  expect(params().get("session")).toBeNull();
  expect(container.querySelector(".vibe-project-header")).toBeNull();
  expect(requests.filter(r => r.method === "DELETE")).toHaveLength(1);
  await click("Instructions");
  expect(container.querySelector<HTMLTextAreaElement>("#vibe-instructions-source")?.value).toBe("Keep my exact agent policy.");
  await click("Real replies");
  expect(container.querySelector<HTMLTextAreaElement>("#vibe-start-conversations")?.value).toBe("Customer: Hi\nAgent: Hello");
  expect(container.querySelector<HTMLTextAreaElement>("#vibe-start-rules")?.value).toBe("Be polite");
  await act(async () => container.querySelector(".vibe-improve-start")!.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }))); await frame();
  expect(document.activeElement).toBe(buttonByDoor("test"));
  await act(async () => buttonByDoor("test").click());
  expect(document.activeElement).toBe(visibleH1()[0]);
  expect(container.querySelector<HTMLTextAreaElement>("#vibe-start-rules")?.value).toBe("Be polite");
  expect(requests.filter(r => r.path.endsWith("/run-quote"))).toEqual([]);
  expect(requests.filter(r => r.path === "/sessions" && r.method === "POST")).toHaveLength(1);
});
