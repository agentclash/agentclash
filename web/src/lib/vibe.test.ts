import { afterEach, expect, it, vi } from "vitest";
import { resolveVibeBaseURL, vibeFetch, watchVibe } from "./vibe";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

it.each([
  ["http://localhost:55440", "127.0.0.1", "http://127.0.0.1:55440"],
  ["http://127.0.0.1:55440/", "localhost", "http://localhost:55440"],
  ["http://localhost:55440/proxy/", "127.0.0.1", "http://127.0.0.1:55440/proxy"],
  [undefined, "127.0.0.1", "http://127.0.0.1:8080"],
  [undefined, "www.agentclash.dev", "https://api.agentclash.dev"],
  ["https://api.agentclash.dev", "127.0.0.1", "https://api.agentclash.dev"],
  ["http://localhost:55440", "preview.example.com", "http://localhost:55440"],
  ["http://localhost.evil.example:55440", "127.0.0.1", "http://localhost.evil.example:55440"],
  ["http://127.0.0.1:55440", undefined, "http://127.0.0.1:55440"],
])("resolves %s from %s without crossing local cookie sites", (configured, hostname, expected) => {
  expect(resolveVibeBaseURL(configured, hostname)).toBe(expected);
});

it("uses the same credentialed loopback origin for requests and event streams", async () => {
  vi.stubEnv("NEXT_PUBLIC_API_URL", "http://localhost:55440");
  vi.stubGlobal("window", { location: { hostname: "127.0.0.1" } });
  const fetch = vi.fn()
    .mockResolvedValueOnce(new Response(JSON.stringify({ id: "session" })))
    .mockResolvedValueOnce(new Response('event: snapshot\ndata: {"id":"session"}\n\n'));
  vi.stubGlobal("fetch", fetch);
  await vibeFetch("/sessions/session");
  const snapshot = vi.fn();
  await watchVibe("session", undefined, new AbortController().signal, snapshot);
  expect(fetch.mock.calls.map(([url, options]) => [url, options.credentials])).toEqual([
    ["http://127.0.0.1:55440/v1/vibe/sessions/session", "include"],
    ["http://127.0.0.1:55440/v1/vibe/sessions/session/events", "include"],
  ]);
  expect(snapshot).toHaveBeenCalledWith({ id: "session" });
});

it("preserves session-auth failures so the UI can stop reconnecting", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "forbidden", message: "Private cookie missing" } }), { status: 403 })));
  await expect(watchVibe("session", undefined, new AbortController().signal, vi.fn())).rejects.toMatchObject({ code: "forbidden", status: 403 });
});
