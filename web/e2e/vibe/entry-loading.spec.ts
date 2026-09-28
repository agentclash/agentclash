import { expect, test } from "@playwright/test";
import { defaultModels } from "../../src/lib/vibe";

test.describe("before hydration", () => {
  test.use({ javaScriptEnabled: false });
  test("server HTML already shows the V1 entry", async ({ page }) => {
    await page.goto("/vibe-evals");
    await expect(page.getByRole("heading", { name: "What would you like AI to handle?" })).toBeVisible();
    await expect(page.getByRole("button", { name: /Build an agent/ })).toBeDisabled();
    await expect(page.getByText("What should your agent do?", { exact: true })).toHaveCount(0);
  });
});

for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
  test(`entry stays consistent through slow configuration and reconnect at ${viewport.width}px`, async ({ page }) => {
    await page.setViewportSize(viewport);
    const errors: string[] = [];
    const writes: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    let release!: () => void;
    const delayedConfig = new Promise<void>(resolve => { release = resolve; });
    let configRequests = 0;
    await page.route("**/v1/vibe/**", async route => {
      const request = route.request();
      const headers = {
        "Access-Control-Allow-Origin": new URL(page.url()).origin,
        "Access-Control-Allow-Credentials": "true",
      };
      if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
      if (request.method() !== "GET") writes.push(request.url());
      if (new URL(request.url()).pathname.endsWith("/config")) {
        if (++configRequests === 1) {
          await delayedConfig;
          return route.fulfill({ status: 503, headers, json: { error: { message: "Offline" } } });
        }
        return route.fulfill({ headers, json: { enabled: true, two_door: true, defaults: defaultModels, models: [] } });
      }
      return route.fulfill({ headers, json: [] });
    });
    await page.goto("/vibe-evals");
    const heading = page.getByRole("heading", { name: "What would you like AI to handle?" });
    const build = page.getByRole("button", { name: /Build an agent/ });
    const improve = page.getByRole("button", { name: /Improve an existing agent/ });
    await expect(heading).toBeVisible();
    await expect(build).toBeDisabled();
    await expect(improve).toBeDisabled();
    await expect(page.getByText("What should your agent do?", { exact: true })).toHaveCount(0);
    await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toHaveCount(0);
    const before = await heading.boundingBox();
    await page.screenshot({ path: `/tmp/vibe-entry-loading-${viewport.width}.png` });
    release();
    await expect(page.getByRole("alert").filter({ hasText: "Couldn’t connect" })).toBeVisible();
    await expect(heading).toBeVisible();
    await expect(build).toBeDisabled();
    await page.getByRole("button", { name: "Retry connection" }).click();
    await expect(build).toBeEnabled();
    await expect(improve).toBeEnabled();
    expect(await heading.boundingBox()).toEqual(before);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(writes).toEqual([]);
    expect(errors).toEqual([]);
  });
}
