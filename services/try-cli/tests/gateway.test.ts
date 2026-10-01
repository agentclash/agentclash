import { expect, test } from "bun:test";
import { loadConfig } from "../server/config.ts";
import { createLimits } from "../server/limits.ts";
import { SessionManager } from "../server/sessions.ts";
import { Gateway } from "../server/gateway.ts";
import { demo, eventually, FakeFactory } from "./fakes.ts";

async function fixture(upstream?: (url: string, init: RequestInit) => Promise<Response>) {
  const config = loadConfig({ OPENAI_API_KEY: crypto.randomUUID(), TRY_CLI_GATEWAY_URL: "https://terminal.example.test",
    GW_SESSION_BUDGET_USD: "0.1", GW_MAX_REQUEST_BYTES: "512" });
  const limits = createLimits(config); const factory = new FakeFactory(); const sessions = new SessionManager(config, factory);
  const session = await sessions.create("codex", demo, "192.0.2.1");
  await eventually(() => session.status === "ready");
  const gateway = new Gateway({ config, limits, sessions, fetch: upstream ?? (async () => Response.json({ model: "gpt-5", usage: { input_tokens: 100, output_tokens: 20 } })) });
  const req = (body: unknown = {}, extra: Record<string, string> = {}) => new Request("https://terminal.example.test/gw/openai/v1/responses", {
    method: "POST", headers: { authorization: `Bearer ${session.proxyToken}`, ...extra }, body: JSON.stringify(body),
  });
  return { config, limits, sessions, session, gateway, req,
    async close() { await gateway.close(); await sessions.close(); limits.close(); } };
}

test("gateway strips all proxy/client credentials, clamps output and meters usage", async () => {
  let forwarded!: RequestInit;
  const f = await fixture(async (_url, init) => { forwarded = init; return Response.json({ model: "gpt-5", usage: { input_tokens: 100, output_tokens: 20 } }); });
  const response = await f.gateway.handle(f.req({ max_output_tokens: 999999, max_tokens: 999999, background: true }, {
    "x-agentclash-proxy-secret": "synthetic", cookie: "private=test", "x-agentclash-user": "forged-user",
  }));
  expect(response.status).toBe(200); await response.text();
  const headers = new Headers(forwarded.headers);
  expect(headers.get("authorization")).toBe(`Bearer ${f.config.providerKeys.openai}`);
  expect(headers.has("x-agentclash-proxy-secret")).toBe(false); expect(headers.has("cookie")).toBe(false);
  const body = JSON.parse(String(forwarded.body));
  expect(body.max_output_tokens).toBe(2048); expect(body.max_tokens).toBeUndefined(); expect(body.background).toBe(false);
  expect(f.session.budget.spent).toBeCloseTo(0.0009); expect(f.gateway.pending).toBe(0);
  await f.close();
});

test("concurrent requests reserve session budget before any asynchronous work", async () => {
  let calls = 0;
  const f = await fixture(async () => { calls++; return new Response(new ReadableStream({ start() {} })); });
  const responses = await Promise.all(Array.from({ length: 8 }, () => f.gateway.handle(f.req())));
  expect(responses.filter(r => r.status === 200)).toHaveLength(2);
  expect(responses.filter(r => r.status === 402)).toHaveLength(6); expect(calls).toBe(2);
  expect(f.session.budget.reserved).toBeCloseTo(0.1);
  await f.gateway.close();
  expect(f.session.budget.spent).toBeCloseTo(0.1); expect(f.session.budget.reserved).toBe(0);
  await f.close();
});

test("invalid/expired/BYO tokens and non-inference routes never reach the provider", async () => {
  let calls = 0; const f = await fixture(async () => { calls++; return Response.json({}); });
  f.session.tier = "authenticated";
  expect((await f.gateway.handle(f.req())).status).toBe(401);
  f.session.tier = "anonymous"; f.session.expiresAt = Date.now() - 1;
  expect((await f.gateway.handle(f.req())).status).toBe(401);
  expect((await f.gateway.handle(new Request("https://terminal.example.test/gw/openai/v1/files", { method: "POST" }))).status).toBe(404);
  expect(calls).toBe(0); await f.close();
});

test("body and JSON limits reject before provider calls or durable spending", async () => {
  let calls = 0; const f = await fixture(async () => { calls++; return Response.json({}); });
  expect((await f.gateway.handle(f.req({ prompt: "x".repeat(1000) }))).status).toBe(413);
  expect((await f.gateway.handle(f.req([]))).status).toBe(400);
  expect(calls).toBe(0); expect(f.session.budget.reserved).toBe(0); await f.close();
});

test("unavailable Redis refuses requests; missing usage retains the reservation charge", async () => {
  let calls = 0; const f = await fixture(async () => { calls++; return Response.json({}); });
  const response = await f.gateway.handle(f.req()); await response.text();
  expect(f.session.budget.spent).toBeCloseTo(0.05);
  f.limits.close(); expect((await f.gateway.handle(f.req())).status).toBe(503);
  expect(calls).toBe(1); await f.close();
});

test("shutdown aborts pending fetch and waits for settlement", async () => {
  let aborted = false;
  const f = await fixture(async (_url, init) => new Promise((_resolve, reject) => {
    init.signal!.addEventListener("abort", () => { aborted = true; reject(new Error("Synthetic abort")); });
  }));
  const request = f.gateway.handle(f.req());
  await eventually(() => f.session.budget.reserved > 0);
  // Let the request enter the provider before initiating shutdown.
  await Bun.sleep(10);
  await f.gateway.close(); await request;
  expect(aborted).toBe(true); expect(f.gateway.pending).toBe(0);
  expect(f.session.budget.spent).toBeCloseTo(0.05); await f.close();
});

test("in-flight gateway work cannot outlive the session deadline", async () => {
  let aborted = false;
  const f = await fixture(async (_url, init) => new Promise((_resolve, reject) => {
    init.signal!.addEventListener("abort", () => { aborted = true; reject(new Error("Synthetic expiry")); });
  }));
  f.session.expiresAt = Date.now() + 100;
  expect((await f.gateway.handle(f.req())).status).toBe(503);
  expect(aborted).toBe(true); expect(f.gateway.pending).toBe(0);
  expect(f.session.budget.spent).toBeCloseTo(0.05); await f.close();
});
