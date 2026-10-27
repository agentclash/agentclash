import { expect, test } from "bun:test";
import { createTryCliProxy } from "../../../web/src/lib/try-cli/proxy.ts";

test("actual frontend helper supplies server identity and strips incoming impersonation headers", async () => {
  const secret = crypto.randomUUID();
  for (const user of [undefined, "server-user"]) {
    let forwarded!: RequestInit; let target = "";
    const proxy = createTryCliProxy({ production: true, vercel: true, secret, serviceUrl: "https://terminal.example.test",
      userId: async () => user, fetch: async (url, init) => { target = url; forwarded = init; return Response.json({ ok: true }); } });
    const response = await proxy(new Request("https://frontend.example.test/api/try/sessions?action=reset", {
      method: "POST", body: "{}", headers: { "x-real-ip": "192.0.2.1", "x-forwarded-for": "198.51.100.1",
        "x-agentclash-user": "attacker", "x-agentclash-proxy-secret": "attacker", cookie: "private=test" },
    }), ["sessions"]);
    expect(response.status).toBe(200);
    const headers = new Headers(forwarded.headers);
    expect(headers.get("x-agentclash-client-ip")).toBe("192.0.2.1");
    expect(headers.get("x-agentclash-proxy-secret")).toBe(secret);
    expect(headers.get("x-agentclash-user")).toBe(user ?? null);
    expect(headers.has("x-forwarded-for")).toBe(false); expect(headers.has("cookie")).toBe(false);
    expect(target).toBe("https://terminal.example.test/api/sessions?action=reset");
  }
});

test("frontend production proxy fails on missing platform/config and rejects path escape", async () => {
  const request = () => new Request("http://localhost/api/try", { headers: { "x-real-ip": "192.0.2.1" } });
  for (const vercel of [false, true]) {
    const proxy = createTryCliProxy({ production: true, vercel, userId: async () => undefined });
    expect((await proxy(request(), ["sessions"])).status).toBe(503);
  }
  const proxy = createTryCliProxy({ production: false, vercel: false, userId: async () => undefined,
    fetch: async () => { throw new Error("Must not fetch"); } });
  expect((await proxy(request(), ["..", "gw"])).status).toBe(503);
});
