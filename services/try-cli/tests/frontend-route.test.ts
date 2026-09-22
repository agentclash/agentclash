import { expect, mock, test } from "bun:test";

// WorkOS session lookup is external; exercise the actual Next route with a
// synthetic server session and a synthetic network transport.
mock.module("@workos-inc/authkit-nextjs", () => ({ withAuth: async () => ({ user: { id: "route-user" } }) }));

test("actual frontend route uses WorkOS-derived user and matching proxy helper", async () => {
  const names = ["TRY_CLI_API_URL", "TRY_CLI_PROXY_SECRET", "NODE_ENV", "VERCEL"];
  const prior = names.map(name => process.env[name]);
  const originalFetch = globalThis.fetch;
  const secret = crypto.randomUUID();
  process.env.TRY_CLI_API_URL = "https://terminal.example.test";
  process.env.TRY_CLI_PROXY_SECRET = secret; process.env.NODE_ENV = "production"; process.env.VERCEL = "1";
  let headers!: Headers;
  globalThis.fetch = Object.assign(async (_input: unknown, init?: RequestInit) => {
    headers = new Headers(init?.headers); return Response.json({ ok: true });
  }, { preconnect: originalFetch.preconnect }) as typeof fetch;
  try {
    const route = await import("../../../web/src/app/api/try/[[...path]]/route.ts");
    const response = await route.POST(new Request("https://frontend.example.test/api/try/sessions", {
      method: "POST", body: "{}", headers: { "x-real-ip": "192.0.2.1", "x-agentclash-user": "forged" },
    }), { params: Promise.resolve({ path: ["sessions"] }) });
    expect(response.status).toBe(200);
    expect(headers.get("x-agentclash-user")).toBe("route-user");
    expect(headers.get("x-agentclash-proxy-secret")).toBe(secret);
  } finally {
    globalThis.fetch = originalFetch;
    names.forEach((name, i) => { if (prior[i] === undefined) delete process.env[name]; else process.env[name] = prior[i]; });
  }
});
