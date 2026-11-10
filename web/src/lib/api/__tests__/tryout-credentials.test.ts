import { afterEach, expect, it, vi } from "vitest";
import { createApiClient } from "../client";
import { claimAgentTryout, createAnonymousAgentTryout, createWorkspaceAgentTryout, getPublicAgentTryout, getPublicAgentTryoutEvents, getSavedAgentTryout, getWorkspaceAgentTryout, submitAgentTryoutTurn } from "../agent-tryouts";

afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
const input = { template_slug: "meeting-minutes", input: { notes: "fixture" } };
const catalogPath = "/v1/agent-tryout-templates";
function fixture(catalog: unknown, failure?: number | "network") {
 vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.example.test");
 const fetch = vi.fn(async (url: string, _init?: RequestInit) => {
  void _init;
  if (url.endsWith(catalogPath)) return Response.json(catalog);
  if (failure === "network") throw new TypeError("Network failed");
  if (failure) return Response.json({ error: { code: "forbidden", message: "Access denied" } }, { status: failure });
  return Response.json({ id: "result", save_eligible: true });
 });
 vi.stubGlobal("fetch", fetch);
 return { api: createApiClient(), fetch };
}

for (const supported of [false, true]) {
 it(`negotiates ${supported ? "cookie-v1" : "legacy"} transport for every public operation`, async () => {
  const { api, fetch } = fixture({ items: [], ...(supported ? { creator_access_protocol: "cookie-v1" } : {}) });
  const created = await createAnonymousAgentTryout(api, input);
  const read = await getPublicAgentTryout(api, "result");
  await getPublicAgentTryoutEvents(api, "result", { after: 2 });
  await submitAgentTryoutTurn(api, "result", { message: "fixture" });
  expect(created.save_eligible).toBe(supported); expect(read.save_eligible).toBe(supported);
  expect(fetch).toHaveBeenCalledTimes(8);
  for (const [url, init] of fetch.mock.calls) {
   expect(init?.cache).toBe("no-store");
   expect(init?.credentials).toBe(url.endsWith(catalogPath) || !supported ? "omit" : "include");
  }
  expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).toEqual({ ...input, ...(supported ? { creator_access_protocol: "cookie-v1" } : {}) });
  expect(fetch.mock.calls[5][0]).toContain("after=2");
 });
}

it.each([null, {}, { items: null }, { items: [null] }, { items: [{}] }, { items: [] , creator_access_protocol: null }, { items: [], creator_access_protocol: "cookie-v2" }, { items: [], creator_access_protocol: "" }])("fails closed on malformed or unknown catalog %j", async catalog => {
 const { api, fetch } = fixture(catalog);
 await expect(createAnonymousAgentTryout(api, input)).rejects.toThrow(/Reload|update/i);
 expect(fetch).toHaveBeenCalledTimes(1);
});

it("discovers fresh support after a legacy catalog and across independent clients/hosts", async () => {
 const { api, fetch } = fixture({ items: [] });
 await createAnonymousAgentTryout(api, input);
 fetch.mockImplementation(async (url: string) => Response.json(url.endsWith(catalogPath) ? { items: [], creator_access_protocol: "cookie-v1" } : {}));
 await createAnonymousAgentTryout(api, input);
 expect(fetch.mock.calls[3][1]?.credentials).toBe("include");
 vi.stubEnv("NEXT_PUBLIC_API_URL", "https://other.example.test");
 await getPublicAgentTryout(createApiClient(), "result");
 expect(fetch.mock.calls[4][0]).toBe(`https://other.example.test${catalogPath}`);
});

it.each([403, "network"] as const)("never retries credentialed failures (%s)", async failure => {
 const { api, fetch } = fixture({ items: [], creator_access_protocol: "cookie-v1" }, failure);
 for (const operation of [() => createAnonymousAgentTryout(api, input), () => getPublicAgentTryout(api, "result"), () => getPublicAgentTryoutEvents(api, "result"), () => submitAgentTryoutTurn(api, "result", { end: true }), () => claimAgentTryout(api, "result", "workspace")]) {
  fetch.mockClear(); await expect(operation()).rejects.toThrow(); expect(fetch).toHaveBeenCalledTimes(2);
  expect(fetch.mock.calls[1][1]?.credentials).toBe("include");
 }
});

it("bounds discovery with an abort signal and never writes when discovery fails", async () => {
 const { api, fetch } = fixture({ items: [] });
 fetch.mockRejectedValue(new TypeError("discovery unavailable"));
 await expect(createAnonymousAgentTryout(api, input)).rejects.toThrow();
 expect(fetch).toHaveBeenCalledTimes(1); expect(fetch.mock.calls[0][1]?.signal).toBeInstanceOf(AbortSignal);
});

it.each([{ items: [] }, { items: [], creator_access_protocol: "future" }])("refuses claim before reaching legacy/unknown handlers", async catalog => {
 const { api, fetch } = fixture(catalog);
 await expect(claimAgentTryout(api, "result", "workspace")).rejects.toThrow(/save|Reload|update/i);
 expect(fetch).toHaveBeenCalledTimes(1);
});

it("keeps workspace, saved and unrelated calls on their existing token contract", async () => {
 const { fetch } = fixture({ items: [] }); const api = createApiClient("token");
 await createWorkspaceAgentTryout(api, "workspace", input);
 await getWorkspaceAgentTryout(api, "workspace", "result");
 await getSavedAgentTryout(api, "result"); await api.get("/v1/tool-library");
 expect(fetch).toHaveBeenCalledTimes(4);
 for (const [, init] of fetch.mock.calls) { expect(init?.credentials).toBeUndefined(); expect(init?.headers).toMatchObject({ Authorization: "Bearer token" }); }
});

it("does not retry a stale legacy create rejected during backend upgrade", async () => {
 const { api, fetch } = fixture({ items: [] }, 409);
 await expect(createAnonymousAgentTryout(api, input)).rejects.toThrow();
 expect(fetch).toHaveBeenCalledTimes(2);
 expect(fetch.mock.calls[1][1]?.credentials).toBe("omit");
 expect(JSON.parse(fetch.mock.calls[1][1]?.body as string)).not.toHaveProperty("creator_access_protocol");
});
