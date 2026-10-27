import { loadConfig } from "./config.ts";
import { createLimits } from "./limits.ts";
import { SessionManager } from "./sessions.ts";
import { e2bFactory } from "./sandbox.ts";
import { Gateway } from "./gateway.ts";
import { registry } from "./registry.ts";
import { startService } from "./app.ts";
import { installSignals } from "./signals.ts";

try {
  const config = loadConfig();
  const limits = createLimits(config);
  const factory = e2bFactory(config);
  const sessions = new SessionManager(config, factory);
  const gateway = new Gateway({ config, limits, sessions });
  const app = startService({ config, limits, sessions, gateway, registry,
    sandboxHealth: () => factory?.health() ?? Promise.resolve() });
  const lifecycle = installSignals(app);
  try {
    await app.ready();
    console.log(`[try-cli] ready (${config.e2bApiKey ? "E2B" : "development mock"})`);
  } catch {
    if (!lifecycle.stopping) {
      console.error("[try-cli] startup dependency check failed");
      await app.close().catch(() => {});
      process.exit(1);
    }
  }
} catch {
  console.error("[try-cli] invalid production configuration or startup failure");
  process.exit(1);
}
