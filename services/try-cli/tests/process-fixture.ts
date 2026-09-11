// Test-only entrypoint. Never copied into the production image or selected by an env flag there.
import { loadConfig } from "../server/config.ts";
import { createLimits } from "../server/limits.ts";
import { SessionManager } from "../server/sessions.ts";
import { Gateway } from "../server/gateway.ts";
import { startService } from "../server/app.ts";
import { installSignals } from "../server/signals.ts";
import { demo, FakeFactory } from "./fakes.ts";
import { writeFileSync } from "node:fs";

const directory = process.env.TRY_CLI_TEST_PRIVATE_DIR;
if (!directory || !process.env.TRY_CLI_PROCESS_TEST_MODE) throw new Error("Isolated fixture environment required");
const config = loadConfig({ TRY_CLI_SHUTDOWN_MS: "250", TRY_CLI_CORS_ORIGINS: "http://localhost:3000" });
const factory = new FakeFactory(); const limits = createLimits(config); const sessions = new SessionManager(config, factory);
const gateway = new Gateway({ config, limits, sessions, fetch: async () => { throw new Error("No upstream expected"); } });
const app = startService({ config, limits, sessions, gateway, sandboxHealth: () => factory.health(),
  registry: { list: () => [demo], get: () => demo } }, { hostname: "127.0.0.1", port: 0 });
installSignals({ drain: app.drain, async close() {
  if (process.env.TRY_CLI_PROCESS_TEST_MODE === "hang") {
    for (const sandbox of factory.created) sandbox.killWait = new Promise(() => {});
  }
  await app.close();
  writeFileSync(directory + "/process-cleanup.json", JSON.stringify({ sessions: sessions.size, killed: factory.created.every(s => s.kills === 1) }), { mode: 0o600 });
} });
await app.ready();
console.log(JSON.stringify({ port: app.server.port }));
