import { defineConfig } from "@playwright/test";
const baseURL = process.env.VIBE_TEST_BASE_URL ?? "http://127.0.0.1:53517";

export default defineConfig({
  testDir: "./e2e/vibe",
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
        command: "npm run dev -- --hostname 127.0.0.1 --port 53517",
        url: `${baseURL}/vibe-evals`,
        reuseExistingServer: true,
        timeout: 120_000,
        env: {
          NEXT_PUBLIC_API_URL: "http://127.0.0.1:55440",
          WORKOS_CLIENT_ID: "client_test_vibe",
          WORKOS_API_KEY: "sk_test_not_a_real_key",
          WORKOS_COOKIE_PASSWORD:
            "vibe-test-cookie-password-32-characters-long",
          NEXT_PUBLIC_WORKOS_REDIRECT_URI:
            "http://127.0.0.1:53517/auth/callback",
        },
      },
});
