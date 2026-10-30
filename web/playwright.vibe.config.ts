import { defineConfig } from "@playwright/test";
// Override the ports to run two mocked suites at once (for example in two worktrees).
const webPort = process.env.VIBE_TEST_WEB_PORT ?? "53517";
const authPort = process.env.VIBE_TEST_AUTH_PORT ?? "55442";
const baseURL = process.env.VIBE_TEST_BASE_URL ?? `http://127.0.0.1:${webPort}`;

export default defineConfig({
  testDir: "./e2e/vibe",
  // Keep this suite from clearing the real-stack suite's active recordings.
  outputDir: "test-results/vibe",
  fullyParallel: false,
  workers: 1,
  timeout: 45_000,
  use: {
    baseURL,
    trace: "retain-on-failure",
    launchOptions: process.env.VIBE_TEST_CHROMIUM
      ? { executablePath: process.env.VIBE_TEST_CHROMIUM }
      : {},
  },
  // An explicit URL reuses an already-running server without touching its process.
  webServer: process.env.VIBE_TEST_BASE_URL
    ? undefined
    : {
        command: "node e2e/vibe-stack/start-web.mjs",
        url: `${baseURL}/vibe-evals`,
        reuseExistingServer: false,
        gracefulShutdown: { signal: "SIGTERM", timeout: 20_000 },
        timeout: 120_000,
        env: {
          VIBE_BROWSER_WEB_PORT: webPort,
          WORKOS_API_HOSTNAME: "127.0.0.1",
          WORKOS_API_PORT: authPort,
          WORKOS_API_HTTPS: "false",
          NEXT_TELEMETRY_DISABLED: "1",
          NEXT_PUBLIC_POSTHOG_KEY: "",
          ANALYTICS_REQUIRED: "false",
          NEXT_PUBLIC_API_URL: "http://127.0.0.1:55440",
          WORKOS_CLIENT_ID: "client_test_vibe",
          WORKOS_API_KEY: "sk_test_not_a_real_key",
          WORKOS_COOKIE_PASSWORD:
            "vibe-test-cookie-password-32-characters-long",
          NEXT_PUBLIC_WORKOS_REDIRECT_URI: `http://127.0.0.1:${webPort}/auth/callback`,
        },
      },
});
