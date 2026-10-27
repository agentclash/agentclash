import { expect, test } from "bun:test";
import { loadConfig } from "../server/config.ts";
import { createLimits } from "../server/limits.ts";
import { SessionManager } from "../server/sessions.ts";
import { Gateway } from "../server/gateway.ts";
import { startService } from "../server/app.ts";
import { demo, FakeFactory } from "./fakes.ts";

test("readiness and all new admission fail when configured dependencies are unavailable", async () => {
  const config = loadConfig({}); const limits = createLimits(config); const factory = new FakeFactory();
  factory.available = false;
  const sessions = new SessionManager(config, factory); const gateway = new Gateway({ config, limits, sessions });
  const app = startService({ config, limits, sessions, gateway, registry: { list: () => [demo], get: () => demo },
    sandboxHealth: () => factory.health() }, { hostname: "127.0.0.1", port: 0 });
  const base = `http://127.0.0.1:${app.server.port}`;
  const create = () => fetch(base + "/api/sessions", { method: "POST", body: '{"slug":"codex"}' });
  try {
    expect((await fetch(base + "/health")).status).toBe(200);
    expect((await fetch(base + "/health/ready")).status).toBe(503);
    expect((await create()).status).toBe(503);
    expect(factory.created).toHaveLength(0);
    factory.available = true;
    expect((await fetch(base + "/health/ready")).status).toBe(200);
    limits.close();
    expect((await fetch(base + "/health/ready")).status).toBe(503);
    expect((await create()).status).toBe(503);
    expect(factory.created).toHaveLength(0);
  } finally { await app.close(); }
});
