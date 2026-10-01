import { expect, test } from "bun:test";
import { writeFileSync, openSync, closeSync } from "node:fs";
import { join } from "node:path";
import { loadConfig } from "../../server/config.ts";
import { createLimits } from "../../server/limits.ts";
import { SessionManager } from "../../server/sessions.ts";
import { Gateway } from "../../server/gateway.ts";
import { startService } from "../../server/app.ts";
import { createTryCliProxy } from "../../../../web/src/lib/try-cli/proxy.ts";
import { demo, eventually, FakeFactory } from "../fakes.ts";

const evidence = process.env.TRY_CLI_TEST_PRIVATE_DIR;
const integration = evidence ? test : test.skip;

integration("real Caddy HTTP, browser WebSocket, idle survival, gateway callback and drain", async () => {
  const redisUrl = process.env.TRY_CLI_TEST_REDIS_URL!;
  if (!["localhost", "127.0.0.1"].includes(new URL(redisUrl).hostname)) throw new Error("Loopback Redis required");
  const config = loadConfig({ REDIS_URL: redisUrl, REDIS_TLS_CA_FILE: process.env.TRY_CLI_TEST_CA_FILE,
    TRY_CLI_PROXY_SECRET: crypto.randomUUID(), TRY_CLI_TRUSTED_PROXY_CIDRS: "127.0.0.1/32",
    TRY_CLI_CORS_ORIGINS: "https://frontend.example.test", TRY_CLI_GATEWAY_URL: "https://terminal.example.test",
    OPENAI_API_KEY: crypto.randomUUID(), TRY_CLI_SHUTDOWN_MS: "2000" });
  const limits = createLimits(config); const factory = new FakeFactory(); const sessions = new SessionManager(config, factory);
  let providerCalls = 0;
  const gateway = new Gateway({ config, limits, sessions, fetch: async () => { providerCalls++; return Response.json({ model: "gpt-5", usage: { input_tokens: 10, output_tokens: 10 } }); } });
  const fixtureDemo = { ...demo, sessionMinutes: 3 };
  const app = startService({ config, limits, sessions, gateway, sandboxHealth: () => factory.health(),
    registry: { list: () => [fixtureDemo], get: slug => slug === demo.slug ? fixtureDemo : undefined } }, { hostname: "127.0.0.1", port: 0 });
  const probe = Bun.serve({ hostname: "127.0.0.1", port: 0, fetch: () => new Response() });
  const port = probe.port; await probe.stop(true);
  const base = `http://127.0.0.1:${port}`;
  const caddyfile = join(evidence!, "Caddyfile");
  writeFileSync(caddyfile, `{
 admin off
 auto_https off
 persist_config off
}
http://127.0.0.1:${port} {
 bind 127.0.0.1
 reverse_proxy 127.0.0.1:${app.server.port} {
  header_up X-Trycli-Client-IP {http.request.client_ip}
  stream_close_delay 5m
 }
}
`, { mode: 0o600 });
  const log = openSync(join(evidence!, "caddy.log"), "a", 0o600);
  const caddyEnv = { PATH: process.env.PATH!, HOME: process.env.HOME!, XDG_DATA_HOME: join(evidence!, "caddy-data"), XDG_CONFIG_HOME: join(evidence!, "caddy-config") };
  let caddy: ReturnType<typeof Bun.spawn> | undefined;
  const sockets: WebSocket[] = [];
  try {
    const validate = Bun.spawn(["caddy", "validate", "--config", caddyfile, "--adapter", "caddyfile"], { env: caddyEnv, stdout: log, stderr: log });
    expect(await validate.exited).toBe(0);
    caddy = Bun.spawn(["caddy", "run", "--config", caddyfile, "--adapter", "caddyfile"], { env: caddyEnv, stdout: log, stderr: log });
    await app.ready();
    let reachable = false;
    for (let i = 0; i < 40; i++) {
      try { reachable = (await fetch(base + "/health")).ok; } catch { /* waiting for listener */ }
      if (reachable) break; await Bun.sleep(50);
    }
    expect(reachable).toBe(true);
    expect((await fetch(base + "/health/ready")).status).toBe(200);
    const createThroughFrontend = async (user?: string) => {
      const proxy = createTryCliProxy({ production: false, vercel: true, serviceUrl: base, secret: config.proxySecret,
        userId: async () => user });
      return proxy(new Request("https://frontend.example.test/api/try/sessions", { method: "POST", body: JSON.stringify({ slug: "codex" }),
        headers: { "content-type": "application/json", "x-real-ip": "192.0.2.50", "x-agentclash-user": "forged" } }), ["sessions"]);
    };
    const anonResponse = await createThroughFrontend(); expect(anonResponse.status).toBe(200);
    const anonymous = await anonResponse.json() as { id: string; tier: string };
    expect(anonymous.tier).toBe("anonymous");
    expect((await createThroughFrontend()).status).toBe(403);
    const authResponse = await createThroughFrontend("server-user"); expect(authResponse.status).toBe(200);
    expect((await authResponse.json() as { tier: string }).tier).toBe("authenticated");
    const direct = (xff: string) => fetch(base + "/api/sessions", { method: "POST", body: JSON.stringify({ slug: "codex" }),
      headers: { "x-forwarded-for": xff, "x-trycli-client-ip": xff, "x-agentclash-user": "forged" } });
    const directResponse = await direct("198.51.100.1"); expect(directResponse.status).toBe(200);
    expect((await directResponse.json() as { tier: string }).tier).toBe("anonymous");
    expect((await direct("198.51.100.2")).status).toBe(403);
    const session = sessions.get(anonymous.id)!;
    expect(session.ip).toBe("192.0.2.50");
    await eventually(() => session.status === "ready");
    expect((await fetch(base + `/ws?sessionId=${anonymous.id}`, { headers: { origin: "https://attacker.example.test" } })).status).toBe(403);
    const wsUrl = base.replace("http:", "ws:") + `/ws?sessionId=${anonymous.id}`;
    const messages: string[] = [];
    const connect = async () => {
      // Bun supports client headers; TypeScript's DOM constructor omits this overload.
      const Client = WebSocket as typeof WebSocket & { new(url: string, options: import("bun").WebSocketOptions): WebSocket };
      const ws = new Client(wsUrl, { headers: { origin: "https://frontend.example.test" } });
      sockets.push(ws);
      ws.addEventListener("message", async event => {
        messages.push(typeof event.data === "string" ? event.data : event.data instanceof Blob ? await event.data.text() : new TextDecoder().decode(event.data));
      });
      await eventually(() => ws.readyState === WebSocket.OPEN);
      return ws;
    };
    const ws = await connect(); ws.send("first-input"); await eventually(() => messages.includes("first-input"));
    ws.close(); await eventually(() => ws.readyState === WebSocket.CLOSED);
    const reconnected = await connect(); reconnected.send("second-input"); await eventually(() => messages.includes("second-input"));
    expect(factory.created[0]!.creates).toBe(1);
    const callGateway = () => fetch(base + "/gw/openai/v1/responses", { method: "POST", body: "{}", headers: { authorization: `Bearer ${session.proxyToken}` } });
    const inference = await callGateway(); expect(inference.status).toBe(200); await inference.text(); expect(providerCalls).toBe(1);
    // Longer than the service's 60s WS idle timeout; Bun ping/pong must keep it alive.
    console.log("[integration] checking 65-second idle WebSocket through Caddy");
    await Bun.sleep(65000);
    expect(reconnected.readyState).toBe(WebSocket.OPEN);
    reconnected.send("after-idle"); await eventually(() => messages.includes("after-idle"));
    app.drain(); expect((await fetch(base + "/health")).status).toBe(200);
    expect((await fetch(base + "/health/ready")).status).toBe(503);
    expect((await createThroughFrontend("server-user")).status).toBe(503);
    expect((await fetch(base + `/api/sessions/${session.id}?action=reset`, { method: "POST" })).status).toBe(503);
    const duringDrain = await callGateway(); expect(duringDrain.status).toBe(200); await duringDrain.text();
    reconnected.send("after-drain"); await eventually(() => messages.includes("after-drain"));
    await app.close(); await eventually(() => reconnected.readyState === WebSocket.CLOSED);
    expect(sessions.size).toBe(0); expect(factory.created.every(s => s.kills === 1)).toBe(true);
  } finally {
    for (const ws of sockets) ws.close();
    await app.close().catch(() => {});
    if (caddy) { if (caddy.exitCode === null) caddy.kill(); await caddy.exited; }
    closeSync(log);
  }
}, 105000);
