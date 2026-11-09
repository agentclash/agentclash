import { afterEach, expect, it, vi } from "vitest";
import { createApiClient } from "../client";
import { createAnonymousAgentTryout, getPublicAgentTryout, getPublicAgentTryoutEvents, submitAgentTryoutTurn } from "../agent-tryouts";
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
it("sends creator cookies only on tryout calls", async () => {
 vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.example.test");
 const fetch = vi.fn(async () => new Response("{}", { headers: { "Content-Type": "application/json" } }));
 vi.stubGlobal("fetch", fetch);
 const api = createApiClient();
 await createAnonymousAgentTryout(api, { template_slug: "meeting-minutes", input: { notes: "fixture" } });
 await getPublicAgentTryout(api, "result");
 await getPublicAgentTryoutEvents(api, "result");
 await submitAgentTryoutTurn(api, "result", { message: "fixture" });
 for (const [, init] of fetch.mock.calls as unknown as [string, RequestInit][]) expect(init.credentials).toBe("include");
 await api.get("/v1/tool-library");
 expect((fetch.mock.calls.at(-1) as unknown as [string, RequestInit])[1].credentials).toBeUndefined();
});
