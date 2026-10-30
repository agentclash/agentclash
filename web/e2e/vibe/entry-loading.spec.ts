import { expect, test } from "@playwright/test";
import { fakeVibe } from "./entry-fake";
import { defaultModels } from "../../src/lib/vibe";
import { composer, doorButton, doorHeading, entryHeading, workspacePicker } from "./entry-model";

test.describe("before hydration", () => {
  test.use({ javaScriptEnabled: false });
  test("server HTML already shows the V1 entry", async ({ page }) => {
    await page.goto("/vibe-evals");
    await expect(entryHeading(page)).toBeVisible();
    await expect(doorButton(page, "build")).toBeDisabled();
    await expect(page.getByText("What should your agent do?", { exact: true })).toHaveCount(0);
  });
  for (const width of [1440, 390]) test(`start reload has one centred loading status before hydration at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/vibe-evals?start=improve");
    const status = page.getByRole("status");
    await expect(status).toHaveCount(1);
    await expect(status).toHaveText("Loading…");
    await expect(status).toBeVisible();
    await expect(page.getByText("Loading projects…", { exact: true })).toHaveCount(0);
    const region = await page.locator(".vibe-start-region").boundingBox();
    const box = await status.boundingBox();
    expect(region).not.toBeNull(); expect(box).not.toBeNull();
    expect(Math.abs(box!.x + box!.width / 2 - (region!.x + region!.width / 2))).toBeLessThan(2);
    expect(Math.abs(box!.y + box!.height / 2 - (region!.y + region!.height / 2))).toBeLessThan(2);
    expect(box!.y).toBeGreaterThan(56);
    expect(box!.y + box!.height).toBeLessThan(850);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
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
        return route.fulfill({ headers, json: { enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, two_door: true, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) } });
      }
      return route.fulfill({ headers, json: [] });
    });
    await page.goto("/vibe-evals");
    const heading = entryHeading(page);
    const build = doorButton(page, "build");
    const improve = doorButton(page, "improve");
    await expect(heading).toBeVisible();
    await expect(build).toBeDisabled();
    await expect(improve).toBeDisabled();
    await expect(page.getByRole("status").filter({ hasText: "Connecting…" })).toBeVisible();
    await expect(page.getByText("What should your agent do?", { exact: true })).toHaveCount(0);
    await expect(composer(page)).toHaveCount(0);
    const before = await heading.boundingBox();
    await page.screenshot({ path: info.outputPath(`entry-loading-${viewport.width}.png`) });
    release();
    await expect(page.getByRole("alert").filter({ hasText: "Couldn’t connect" })).toBeVisible();
    expect(configRequests).toBe(1);
    await expect(heading).toBeVisible();
    await expect(build).toBeDisabled();
    await expect(page.getByRole("status").filter({ hasText: "Connecting…" })).toHaveCount(0);
    expect(await heading.boundingBox()).toEqual(before);
    await page.getByRole("button", { name: "Retry connection" }).click();
    await expect(build).toBeEnabled();
    await expect(improve).toBeEnabled();
    expect(configRequests).toBe(2);
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
  const quotes: string[] = [];
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  let configRequests = 0;
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request(), path = new URL(request.url()).pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path.endsWith("/build-quote")) {
      quotes.push(path);
      return route.fulfill({ headers, json: { id: "draft-quote", request: request.postDataJSON(), max_calls: 4, max_cost_nano_usd: 250000000, cases: 3, expires_at: new Date(Date.now() + 600000).toISOString() } });
    }
    if (request.method() !== "GET") writes.push(path);
    if (path.endsWith("/config")) {
      if (++configRequests === 1) return route.fulfill({ status: 503, headers, json: { error: { message: "Offline" } } });
      return route.fulfill({ headers, json: { enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, two_door: true, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) } });
    }
    if (path.endsWith("/sessions")) return route.fulfill({ headers, json: [session] });
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: ": connected\n\n" });
    if (path.endsWith("/sessions/draft-session")) return route.fulfill({ headers, json: session });
    return route.fulfill({ headers, json: [] });
  });
  await page.goto("/vibe-evals?session=draft-session");
  const draft = composer(page);
  await expect(draft).toHaveValue("Keep this unsent draft.");
  await expect(page.getByRole("alert").filter({ hasText: "Your message is still here." })).toBeVisible();
  expect(configRequests).toBe(1);
  expect(writes).toEqual([]);
  expect(quotes).toEqual([]);
  await page.getByRole("button", { name: "Retry connection" }).click();
  await expect(page.getByRole("alert").filter({ hasText: "Couldn’t connect" })).toHaveCount(0);
  expect(configRequests).toBe(2);
  await expect(draft).toHaveValue("Keep this unsent draft.");
  // Reconnection may price the unsent draft, but never submits or runs it.
  await expect.poll(() => quotes.length).toBe(1);
  expect(writes).toEqual([]);
  expect(errors).toEqual([]);
});

for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
  test(`signed-in entry shows the workspace picker and the sidebar moves focus to it at ${viewport.width}px`, async ({ page }) => {
    await page.setViewportSize(viewport);
    const errors: string[] = [];
    const writes: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    const cors = () => ({ "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" });
    const preflight = { "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS" };
    await page.route("**/v1/users/me", route => route.request().method() === "OPTIONS"
      ? route.fulfill({ status: 204, headers: { ...cors(), ...preflight } })
      : route.fulfill({ headers: cors(), json: { user_id: "a8000000-0000-4000-8000-000000000001", organizations: [{ id: "org", name: "Org", slug: "org", role: "org_admin", workspaces: [
        { id: "a8000000-0000-4000-8000-000000000003", name: "Returns team", slug: "returns", role: "workspace_admin" },
        { id: "a8000000-0000-4000-8000-000000000006", name: "Support pilots", slug: "support", role: "workspace_member" },
      ] }] } }));
    await page.route("**/v1/vibe/**", route => {
      const request = route.request();
      if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...cors(), ...preflight } });
      if (request.method() !== "GET") writes.push(request.url());
      if (new URL(request.url()).pathname.endsWith("/config")) return route.fulfill({ headers: cors(), json: { enabled: true, availability: { guest: { available: false }, workspace: { available: true } }, two_door: true, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) } });
      return route.fulfill({ headers: cors(), json: [] });
    });
    await page.goto("/auth/login?returnTo=%2Fvibe-evals");
    await page.getByRole("button", { name: "Continue with AgentClash" }).click();
    const picker = workspacePicker(page);
    await expect(picker).toBeVisible({ timeout: 20_000 });
    await expect(picker).toHaveAccessibleDescription("Choose a workspace to use its credits.");
    await expect(doorButton(page, "build")).toBeDisabled();
    await expect(page.getByRole("button", { name: "Review settings" })).toHaveCount(0);
    if (viewport.width < 1024) await page.getByRole("button", { name: "Open projects", exact: true }).click();
    await page.getByRole("button", { name: "Choose workspace", exact: true }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(picker).toBeFocused();
    await picker.selectOption({ label: "Support pilots" });
    await expect(doorButton(page, "build")).toBeEnabled();
    await expect(picker).not.toHaveAttribute("aria-describedby");
    if (viewport.width < 1024) await page.getByRole("button", { name: "Open projects", exact: true }).click();
    await expect(page.locator(".vibe-sidebar-workspace").filter({ visible: true })).toContainText("Support pilots");
    await page.getByRole("button", { name: "Change workspace", exact: true }).click();
    await expect(picker).toBeFocused();
    await expect(page.getByRole("combobox", { name: "Workspace for new projects" })).toHaveCount(1);
    expect(writes).toEqual([]);
    expect(errors).toEqual([]);
  });
}

for (const width of [1440, 390]) test(`Improve reload keeps heading focus without an outline at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  await fakeVibe(page);
  await page.goto("/vibe-evals?start=improve");
  await page.reload();
  const heading = doorHeading(page, "improve");
  await expect(heading).toBeVisible();
  await expect(heading).toBeFocused();
  expect(await heading.evaluate(element => document.activeElement === element)).toBe(true);
  await expect(heading).toHaveCSS("outline-style", "none");
  await expect(heading).toHaveCSS("box-shadow", "none");
  await expect(page.getByText("Loading projects…", { exact: true })).toHaveCount(0);
});
