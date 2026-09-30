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
  test(`entry stays consistent through slow configuration and reconnect at ${viewport.width}px`, async ({ page }, info) => {
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
    await expect(page.getByRole("status").filter({ hasText: "Connecting…" })).toBeVisible();
    await expect(page.getByText("What should your agent do?", { exact: true })).toHaveCount(0);
    await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toHaveCount(0);
    const before = await heading.boundingBox();
    await page.screenshot({ path: info.outputPath(`entry-loading-${viewport.width}.png`) });
    release();
    await expect(page.getByRole("alert").filter({ hasText: "Couldn’t connect" })).toBeVisible();
    await expect(heading).toBeVisible();
    await expect(build).toBeDisabled();
    await expect(page.getByRole("status").filter({ hasText: "Connecting…" })).toHaveCount(0);
    await page.getByRole("button", { name: "Retry connection" }).click();
    await expect(build).toBeEnabled();
    await expect(improve).toBeEnabled();
    expect(await heading.boundingBox()).toEqual(before);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(writes).toEqual([]);
    expect(errors).toEqual([]);
  });
}

test("offline retry preserves a restored unsent draft and submits nothing", async ({ page }) => {
  const session = { id: "draft-session", revision: 1, event_cursor: 1, anonymous: true, operations: [], document: {
    format_version: 1, evaluation: { id: "draft-session", chat_id: "draft-session", door: "build" }, models: defaultModels,
    messages: [], requirements: [], artifacts: [],
  } };
  await page.addInitScript(() => sessionStorage.setItem("vibe-build-drafts:draft-session", JSON.stringify({ version: 1, guide: "Keep this unsent draft.", trials: {} })));
  const writes: string[] = [];
  let configRequests = 0;
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request(), path = new URL(request.url()).pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (request.method() !== "GET") writes.push(path);
    if (path.endsWith("/config")) {
      if (++configRequests === 1) return route.fulfill({ status: 503, headers, json: { error: { message: "Offline" } } });
      return route.fulfill({ headers, json: { enabled: true, two_door: true, defaults: defaultModels, models: [] } });
    }
    if (path.endsWith("/sessions")) return route.fulfill({ headers, json: [session] });
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: ": connected\n\n" });
    if (path.endsWith("/sessions/draft-session")) return route.fulfill({ headers, json: session });
    return route.fulfill({ headers, json: [] });
  });
  await page.goto("/vibe-evals?session=draft-session");
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await expect(composer).toHaveValue("Keep this unsent draft.");
  await expect(page.getByRole("alert").filter({ hasText: "Your message is still here." })).toBeVisible();
  expect(writes).toEqual([]);
  await page.getByRole("button", { name: "Retry connection" }).click();
  await expect(page.getByRole("alert").filter({ hasText: "Couldn’t connect" })).toHaveCount(0);
  await expect(composer).toHaveValue("Keep this unsent draft.");
  expect(writes).toEqual([]);
});
