import { expect, test, type Page } from "@playwright/test";
import { defaultModels, type Session } from "../../src/lib/vibe";

function evaluation(id: string): Session {
  return { id, revision: 1, event_cursor: 1, anonymous: true, operations: [],
    document: { models: defaultModels, messages: [], requirements: [], artifacts: [], test_journey: true,
      evaluation: { id, chat_id: "chat", door: "test" } } };
}

async function serve(page: Page, many = false) {
  const first = evaluation("first"), second = evaluation("second");
  second.document.messages = [{ id: "request", role: "user", content: "Check our support agent.", operation_id: "running" }];
  second.operations = [{ id: "running", kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels,
    max_cost_nano_usd: 1, actual_cost_nano_usd: 0, results: [], progress: { phase: "understanding", completed_cases: 0, total_cases: 0 } }];
  const sessions = new Map([[first.id, first], [second.id, second]]);
  if (many) for (let index = 0; index < 30; index++) {
    const extra = evaluation(`older-${index}`);
    extra.document.artifacts.push({ id: `artifact-${index}`, kind: "test_suite", title: `Customer support for international returns and exchanges ${index}`,
      agent_prompt: "", blueprint: {}, accepted: false, source_message_id: "source" });
    sessions.set(extra.id, extra);
  }
  const posts: { path: string; body: Record<string, unknown> }[] = [];
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request(), path = new URL(request.url()).pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers,
      "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS" } });
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, two_door: true, defaults: defaultModels, models: [] } });
    if (path.endsWith("/saved-checks")) return route.fulfill({ headers, json: [] });
    if (request.method() === "POST") {
      posts.push({ path, body: request.postDataJSON() });
      if (path.endsWith("/evaluations")) {
        const created = evaluation(sessions.has("created") ? `created-${posts.length}` : "created");
        created.document.evaluation!.door = request.postDataJSON().door;
        sessions.set(created.id, created);
        return route.fulfill({ headers, json: created });
      }
      return route.fulfill({ status: 400, headers, json: { error: { message: "Unexpected mutation in navigation test" } } });
    }
    if (path.endsWith("/evaluations")) return route.fulfill({ headers, json: [...sessions.values()] });
    const id = path.split("/sessions/")[1]?.split("/")[0];
    const state = sessions.get(id);
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
    return route.fulfill({ headers, json: state || {} });
  });
  await page.goto("/vibe-evals?session=first");
  // The reused development server also waits for AuthKit's initial session check.
  await expect(page.getByRole("heading", { name: "Improve an agent you already have." })).toBeVisible({ timeout: 15_000 });
  return { first, second, sessions, posts, errors };
}

test("sidebar creation and cancellation preserve the draft and never start a run", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const control = await serve(page);
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("Keep this unsent description.");
  await page.getByRole("button", { name: "New evaluation", exact: true }).click();
  await expect(page.getByRole("heading", { name: "What would you like AI to handle?" })).toBeVisible();
  await page.getByRole("button", { name: "Back to your evaluation" }).click();
  await expect(composer).toHaveValue("Keep this unsent description.");
  await expect(composer).toBeFocused();
  await page.getByRole("button", { name: "New evaluation", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(composer).toBeFocused();
  expect(control.posts).toHaveLength(0);
  await page.getByRole("button", { name: "New evaluation", exact: true }).click();
  await page.getByRole("button", { name: /Improve an existing agent/ }).click();
  await expect(page.locator('[data-evaluation-id="created"]')).toHaveAttribute("aria-current", "page");
  await page.locator('[data-evaluation-id="first"]').click();
  await expect(composer).toHaveValue("Keep this unsent description.");
  expect(control.posts).toEqual([{ path: "/v1/vibe/sessions/chat/evaluations", body: { client_id: expect.any(String), door: "test" } }]);
  expect(control.errors).toEqual([]);
});

test("navigation preserves active context while another evaluation completes", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const control = await serve(page);
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("First draft");
  await page.locator('[data-evaluation-id="second"]').click();
  await expect(page.getByRole("status").filter({ hasText: "Understanding your request" })).toBeVisible();
  await page.getByRole("button", { name: "New evaluation", exact: true }).click();
  await expect(page.getByRole("button", { name: /Improve an existing agent/ })).toBeEnabled();
  await page.getByRole("button", { name: "Back to your evaluation" }).click();
  await composer.fill("Second draft");
  await page.locator('[data-evaluation-id="first"]').click();
  await expect(composer).toHaveValue("First draft");
  control.second.operations[0].state = "COMPLETED";
  control.second.document.messages.push({ id: "reply", role: "assistant", content: "Which support policy should I use?", operation_id: "running" });
  control.second.event_cursor!++;
  await expect(page.locator('[data-evaluation-id="first"]')).toHaveAttribute("aria-current", "page");
  await expect(page.getByText("Which support policy should I use?", { exact: true })).toHaveCount(0);
  await page.locator('[data-evaluation-id="second"]').click();
  await expect(page.getByText("Which support policy should I use?", { exact: true })).toBeVisible();
  await expect(composer).toHaveValue("Second draft");
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("drawer supports keyboard dismissal, focus return and source selection on small screens", async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  const control = await serve(page);
  const trigger = page.getByRole("button", { name: "Open evaluations", exact: true });
  await trigger.click();
  const drawer = page.getByRole("dialog", { name: "Your evaluations" });
  await expect(drawer).toBeVisible();
  await expect(drawer).toHaveCSS("opacity", "1");
  await page.screenshot({ path: info.outputPath("sidebar-mobile.png") });
  await page.keyboard.press("Escape");
  await expect(drawer).toBeHidden();
  await expect(trigger).toBeFocused();
  await trigger.click();
  await drawer.getByRole("button", { name: "New evaluation", exact: true }).click();
  await expect(drawer).toBeHidden();
  await expect(page.getByLabel("New evaluation choices", { exact: true })).toBeFocused();
  await page.getByRole("button", { name: "Back to your evaluation" }).click();
  await page.getByRole("button", { name: "Paste a real conversation", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Improve an agent you already have." })).toBeHidden();
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("entry, context label and composer fit mobile, desktop and zoom-sized layouts", async ({ page }, info) => {
  const control = await serve(page);
  for (const width of [320, 390, 640, 768, 1280, 1440]) {
    await page.setViewportSize({ width, height: width === 640 ? 450 : 900 });
    await expect(page.getByLabel("Active evaluation", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Paste agent instructions", exact: true })).toBeVisible();
    const dock = page.getByTestId("vibe-composer-dock");
    await expect(dock).toBeVisible();
    const dockBox = await dock.boundingBox();
    expect(dockBox!.y + dockBox!.height).toBeLessThanOrEqual((await page.evaluate(() => innerHeight)) + 1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const input = page.getByRole("textbox", { name: "Message Vibe Evals" });
    await input.fill("A long description ".repeat(120));
    await expect(page.getByRole("button", { name: "Send message", exact: true })).toBeVisible();
    expect((await input.boundingBox())!.height).toBeLessThanOrEqual(240);
    await input.fill("");
    await page.screenshot({ path: info.outputPath(`entry-${width}.png`), fullPage: true });
  }
  await page.getByRole("button", { name: "Collapse sidebar", exact: true }).click();
  await expect(page.getByRole("button", { name: "Expand sidebar", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Expand sidebar", exact: true })).toBeFocused();
  await expect(page.getByRole("button", { name: "New evaluation", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Expand sidebar", exact: true }).click();
  await expect(page.getByRole("button", { name: "New evaluation", exact: true })).toBeVisible();
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("plain-language source choices explain test packs and let an empty choice change doors", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const control = await serve(page);
  await page.getByRole("button", { name: "New evaluation", exact: true }).click();
  await page.getByRole("button", { name: /Improve an existing agent/ }).click();
  await expect(page.getByRole("heading", { name: "Improve an agent you already have." })).toBeVisible();
  await page.getByLabel("What is a test pack?", { exact: true }).hover();
  await expect(page.getByText(/A test pack is a saved set of situations to try/)).toBeVisible();
  await page.getByLabel("What is a test pack?", { exact: true }).focus();
  await expect(page.getByText(/A test pack is a saved set of situations to try/)).toContainText('Import one only if you already have it');
  await page.getByRole("button", { name: "Actually, I want to build an agent.", exact: true }).click();
  await expect(page.getByLabel("New evaluation choices", { exact: true })).toBeVisible();
  await expect(page.locator('[data-evaluation-id="created"]')).toHaveCount(0);
  await page.getByRole("button", { name: /Build an agent/ }).click();
  await expect(page.getByRole("heading", { name: "What would you like help with?" })).toBeVisible();
  expect(control.posts.map(post => post.path)).toEqual([
    "/v1/vibe/sessions/chat/evaluations",
    "/v1/vibe/sessions/chat/evaluations",
  ]);
  expect(control.errors).toEqual([]);
});

test("long evaluation titles scroll inside the sidebar and saved work stays accessible", async ({ page }, info) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const control = await serve(page, true);
  const list = page.getByRole("navigation", { name: "Evaluations in this chat" });
  await expect(list.locator("button")).toHaveCount(32);
  expect(await list.evaluate(element => element.scrollHeight > element.clientHeight)).toBe(true);
  const title = list.locator(".vibe-sidebar-item-copy > span").last();
  expect(await title.evaluate(element => element.scrollWidth > element.clientWidth)).toBe(true);
  await list.locator("button").last().scrollIntoViewIfNeeded();
  await expect(page.getByRole("button", { name: "Saved work", exact: true })).toBeInViewport();
  await page.screenshot({ path: info.outputPath("sidebar-history.png") });
  await page.getByRole("button", { name: "Saved work", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "Your saved work" })).toBeVisible();
  await page.getByText("Earlier evaluations in this chat", { exact: true }).click();
  await expect(page.locator('[data-evaluation-id="first"]')).toHaveAttribute("aria-current", "page");
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});
