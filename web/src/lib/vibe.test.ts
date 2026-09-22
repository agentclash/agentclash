import { afterEach, expect, it, vi } from "vitest";
import {
  editableEvaluation,
  resolveVibeBaseURL,
  vibeFetch,
  watchVibe,
} from "./vibe";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

it("keeps expectations separate and leaves custom imported coverage read-only", () => {
  const blueprint = {
    cases: [
      {
        key: "case-1",
        payload: { question: "Extract the supplied name" },
        expectations: [
          {
            key: "expected_behavior",
            kind: "text",
            value: "Return the name as JSON",
          },
        ],
      },
    ],
    judges: [
      {
        key: "behavior",
        mode: "assertion",
        assertion:
          "The response meets this case's expected_behavior and the following shared rules:\n\nUse supplied facts",
        context_from: ["case.expectations.expected_behavior"],
      },
    ],
    validators: [{ key: "has_answer" }],
    dimensions: [{}, {}],
  };
  expect(editableEvaluation(blueprint)).toEqual({
    examples: [],
    scenarios: [
      {
        input: "Extract the supplied name",
        expected: "Return the name as JSON",
      },
    ],
    success_criteria: "Use supplied facts",
  });
  expect(
    editableEvaluation({
      ...blueprint,
      judges: [
        {
          ...blueprint.judges[0],
          mode: "rubric",
          rubric: "Additional grading rules",
        },
      ],
    }),
  ).toBeNull();
  expect(
    editableEvaluation({
      ...blueprint,
      cases: [
        {
          ...blueprint.cases[0],
          payload: {
            question: "Extract the supplied name",
            supplied_document: "Must not be discarded",
          },
        },
      ],
    }),
  ).toBeNull();
  expect(
    editableEvaluation({
      ...blueprint,
      validators: [
        { ...blueprint.validators[0], custom_extension: "Keep this" },
      ],
    }),
  ).toBeNull();
  const legacy = {
    ...blueprint,
    cases: [{ payload: { question: "Old question" } }],
    judges: [{ key: "behavior", assertion: "Original shared rule" }],
  };
  expect(editableEvaluation(legacy)).toEqual({
    examples: ["Old question"],
    success_criteria: "Original shared rule",
  });
});

it.each([
  ["http://localhost:55440", "127.0.0.1", "http://127.0.0.1:55440"],
  ["http://127.0.0.1:55440/", "localhost", "http://localhost:55440"],
  [
    "http://localhost:55440/proxy/",
    "127.0.0.1",
    "http://127.0.0.1:55440/proxy",
  ],
  [undefined, "127.0.0.1", "http://127.0.0.1:8080"],
  [undefined, "www.agentclash.dev", "https://api.agentclash.dev"],
  ["https://api.agentclash.dev", "127.0.0.1", "https://api.agentclash.dev"],
  ["http://localhost:55440", "preview.example.com", "http://localhost:55440"],
  [
    "http://localhost.evil.example:55440",
    "127.0.0.1",
    "http://localhost.evil.example:55440",
  ],
  ["http://127.0.0.1:55440", undefined, "http://127.0.0.1:55440"],
])(
  "resolves %s from %s without crossing local cookie sites",
  (configured, hostname, expected) => {
    expect(resolveVibeBaseURL(configured, hostname)).toBe(expected);
  },
);

it("uses the same credentialed loopback origin for requests and event streams", async () => {
  vi.stubEnv("NEXT_PUBLIC_API_URL", "http://localhost:55440");
  vi.stubGlobal("window", { location: { hostname: "127.0.0.1" } });
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(new Response(JSON.stringify({ id: "session" })))
    .mockResolvedValueOnce(
      new Response('event: snapshot\ndata: {"id":"session"}\n\n'),
    );
  vi.stubGlobal("fetch", fetch);
  await vibeFetch("/sessions/session");
  const snapshot = vi.fn();
  await watchVibe("session", undefined, new AbortController().signal, snapshot);
  expect(
    fetch.mock.calls.map(([url, options]) => [url, options.credentials]),
  ).toEqual([
    ["http://127.0.0.1:55440/v1/vibe/sessions/session", "include"],
    ["http://127.0.0.1:55440/v1/vibe/sessions/session/events", "include"],
  ]);
  expect(snapshot).toHaveBeenCalledWith({ id: "session" });
});

it("preserves session-auth failures so the UI can stop reconnecting", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        new Response(
          JSON.stringify({
            error: { code: "forbidden", message: "Private cookie missing" },
          }),
          { status: 403 },
        ),
      ),
  );
  await expect(
    watchVibe("session", undefined, new AbortController().signal, vi.fn()),
  ).rejects.toMatchObject({ code: "forbidden", status: 403 });
});
