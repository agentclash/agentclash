import { defineConfig } from "@playwright/test";

// Separate ports and a temporary frontend copy leave ordinary dev servers alone.
const port = process.env.VIBE_BROWSER_WEB_PORT ?? "53518";
const apiPort = process.env.VIBE_BROWSER_API_PORT ?? "55441";
const baseURL = `http://127.0.0.1:${port}`;
const apiURL = `http://127.0.0.1:${apiPort}`;

export default defineConfig({
  testDir: "./e2e/vibe-stack",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 240_000,
  expect: { timeout: 30_000 },
  outputDir: "test-results/vibe-stack",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    launchOptions: process.env.VIBE_TEST_CHROMIUM
      ? { executablePath: process.env.VIBE_TEST_CHROMIUM }
      : {},
  },
  webServer: [
    {
      command: "go -C ../backend test -p 1 ./internal/api -run '^TestVibeBrowserStack$' -count=1 -timeout=12m -v",
      url: `${apiURL}/__fixture/ready`,
      reuseExistingServer: false,
      timeout: 180_000,
      gracefulShutdown: { signal: "SIGTERM", timeout: 20_000 },
      env: {
        VIBE_BROWSER_STACK: "1",
        VIBE_BROWSER_API_ADDRESS: `127.0.0.1:${apiPort}`,
        VIBE_BROWSER_WEB_ORIGIN: baseURL,
      },
    },
    {
      command: "node e2e/vibe-stack/start-web.mjs",
      url: `${baseURL}/vibe-evals`,
      reuseExistingServer: false,
      timeout: 180_000,
      gracefulShutdown: { signal: "SIGTERM", timeout: 20_000 },
      env: {
        VIBE_BROWSER_WEB_PORT: port,
        NEXT_PUBLIC_API_URL: apiURL,
        NEXT_TELEMETRY_DISABLED: "1",
        WORKOS_CLIENT_ID: "client_test_vibe_stack",
        WORKOS_API_KEY: "sk_test_not_a_real_key",
        WORKOS_COOKIE_PASSWORD: "vibe-stack-cookie-password-32-characters-long",
        NEXT_PUBLIC_WORKOS_REDIRECT_URI: `${baseURL}/auth/callback`,
        NEXT_PUBLIC_POSTHOG_KEY: "",
        ANALYTICS_REQUIRED: "false",
      },
    },
  ],
});
