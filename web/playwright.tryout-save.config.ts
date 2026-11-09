import { defineConfig } from "@playwright/test";
const port = process.env.TRYOUT_BROWSER_WEB_PORT ?? "53818";
const apiPort = process.env.TRYOUT_BROWSER_API_PORT ?? "55741";
const authPort = process.env.TRYOUT_BROWSER_AUTH_PORT ?? "55742";
const baseURL = `http://127.0.0.1:${port}`;
const apiURL = `http://127.0.0.1:${apiPort}`;
export default defineConfig({
 testDir: "./e2e/tryout-save", workers: 1, fullyParallel: false, retries: 0,
 timeout: 120_000, expect: { timeout: 30_000 }, outputDir: "test-results/tryout-save",
 use: { baseURL, actionTimeout: 30_000, navigationTimeout: 30_000, trace: "off", screenshot: "only-on-failure", launchOptions: process.env.VIBE_TEST_CHROMIUM ? { executablePath: process.env.VIBE_TEST_CHROMIUM } : {} },
 webServer: [
  { command: "node e2e/tryout-save/start-api.mjs", url: `${apiURL}/__fixture/ready`, reuseExistingServer: false, timeout: 180_000,
    gracefulShutdown: { signal: "SIGTERM", timeout: 20_000 }, env: { TRYOUT_BROWSER_API_ADDRESS: `127.0.0.1:${apiPort}`, TRYOUT_BROWSER_WEB_ORIGIN: baseURL } },
  { command: "node e2e/vibe-stack/start-web.mjs", url: `${baseURL}/tryouts`, reuseExistingServer: false, timeout: 180_000,
    gracefulShutdown: { signal: "SIGTERM", timeout: 20_000 }, env: {
     VIBE_BROWSER_WEB_PORT: port, API_URL: apiURL, NEXT_PUBLIC_API_URL: apiURL, NEXT_TELEMETRY_DISABLED: "1",
     WORKOS_API_HOSTNAME: "127.0.0.1", WORKOS_API_PORT: authPort, WORKOS_API_HTTPS: "false",
     WORKOS_CLIENT_ID: "client_test_tryout_save", WORKOS_API_KEY: "sk_test_not_a_real_key",
     WORKOS_COOKIE_PASSWORD: "tryout-save-cookie-password-32-characters-long",
     NEXT_PUBLIC_WORKOS_REDIRECT_URI: `${baseURL}/auth/callback`, NEXT_PUBLIC_POSTHOG_KEY: "", ANALYTICS_REQUIRED: "false",
  } },
 ],
});
