import { expect, test, type Page } from "@playwright/test";
import { defaultModels, type Session } from "../../src/lib/vibe";

function evaluation(id: string): Session {
  return { id, revision: 1, event_cursor: 1, anonymous: true, operations: [],
    document: { format_version: 1, models: defaultModels, messages: [], requirements: [], artifacts: [], test_journey: true,
      evaluation: { id, chat_id: "chat", door: "test" } } };
}

function buildEvaluation(): Session {
  const build = evaluation("build");
  build.document.evaluation!.door = "build";
  build.document.artifacts = [{ id: "prototype", kind: "test_suite", title: "Customer email sorter", agent_prompt: "Suggest a label for each email.", accepted: true, source_message_id: "request", proposal_message_id: "prepared", scope_note: "Suggests labels using supplied text.", blueprint: { cases: [{ key: "spam", payload: { question: "You won a prize." }, expectations: [{ key: "expected_behavior", kind: "text", value: "Label as Spam." }] }] } }];
  build.document.active_artifact_id = "prototype";
  build.document.messages = [{ id: "request", role: "user", content: "Sort customer emails.", operation_id: "prepare" }, { id: "prepared", role: "assistant", content: "", operation_id: "prepare", artifact_id: "prototype" }];
  build.document.build = { cycle_id: "cycle", phase: "results", clarifications_used: 0, artifact_id: "prototype", check_id: "check" };
  build.rule_coverage = { prototype: [{ rule_id: "boundary", statement: "Unknown senders receive an Unsure label", case_keys: [] }] };
  build.operations = [
    { id: "prepare", kind: "message", state: "COMPLETED", billing: "SETTLED", models: defaultModels, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] },
    { id: "check", kind: "check", state: "COMPLETED", billing: "SETTLED", models: defaultModels, source: { kind: "prompt", artifact_id: "prototype", label: "Customer email sorter" }, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [{ case_key: "spam", title: "A prize email", version: "prototype", input: { question: "You won a prize." }, expected: "Label as Spam.", output: "Suggested label: Spam", verdict: "PASS", checks: [{ key: "behavior", verdict: "PASS", evidence: "Correct label." }] }], scorecard: { total: 1, passed: 1, failed: 0, unknown: 0, evaluated: 1, coverage: 1, pass_rate: 1 } },
  ];
  return build;
}

async function serve(page: Page, many = false, prototypes = false) {
  const first = prototypes ? { ...buildEvaluation(), id: "first" } : evaluation("first");
  const second = prototypes ? { ...buildEvaluation(), id: "second" } : evaluation("second");
  if (prototypes) {
    first.document.evaluation!.id = first.id;
    second.document.evaluation!.id = second.id;
  } else {
  second.document.messages = [{ id: "request", role: "user", content: "Check our support agent.", operation_id: "running" }];
  second.operations = [{ id: "running", kind: "message", state: "RUNNING", billing: "RESERVED", models: defaultModels,
    max_cost_nano_usd: 1, actual_cost_nano_usd: 0, results: [], progress: { phase: "understanding", completed_cases: 0, total_cases: 0 } }];
  }
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
      if (path.endsWith("/build-quote")) {
        posts.push({ path, body: request.postDataJSON() });
        return route.fulfill({ headers, json: { id: "quote", max_cost_nano_usd: 250000000, cases: 2, max_calls: 4, calls: 4, expires_at: new Date(Date.now() + 600000).toISOString() } });
      }
      if (path.endsWith("/sessions")) {
        const body = request.postDataJSON();
        posts.push({ path, body });
        const created = evaluation(body.id);
        created.document.evaluation!.door = body.door;
        sessions.set(created.id, created);
        return route.fulfill({ headers, json: created });
      }
      if (path.endsWith("/import")) {
        posts.push({ path, body: { file: request.postData() || "" } });
        return route.fulfill({ status: 503, headers, json: { error: { code: "request_failed", message: "Fixture parser failed" } } });
      }
      posts.push({ path, body: request.postDataJSON() });
      return route.fulfill({ status: 400, headers, json: { error: { message: "Unexpected mutation in navigation test" } } });
    }
    if (path.endsWith("/sessions")) return route.fulfill({ headers, json: [...sessions.values()] });
    const id = path.split("/sessions/")[1]?.split("/")[0];
    const state = sessions.get(id);
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
    return route.fulfill({ headers, json: state || {} });
  });
  await page.goto("/vibe-evals?session=first");
  // The reused development server also waits for AuthKit's initial session check.
  if (prototypes) await expect(page.getByRole("button", { name: "Try it yourself", exact: true })).toBeVisible({ timeout: 15_000 });
  else await expect(page.getByRole("heading", { name: "Improve an agent you already have." })).toBeVisible({ timeout: 15_000 });
  return { first, second, sessions, posts, errors };
}

test("a stored trial in B survives opening A, switching, editing and refreshing", async ({ page }) => {
  await page.addInitScript(model => {
    if (!sessionStorage.getItem("vibe-build-drafts:second"))
      sessionStorage.setItem("vibe-build-drafts:second", JSON.stringify({
        version: 1, guide: "B's separate guide draft",
        trials: { ["second:prototype:new:" + model]: "B's unsent trial" },
      }));
  }, defaultModels.target);
  const control = await serve(page, false, true);
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("A's unsent guide");
  await page.locator('[data-evaluation-id="second"]').click();
  await expect(composer).toHaveValue("B's separate guide draft");
  await page.getByRole("button", { name: "Try it yourself", exact: true }).click();
  const trial = page.getByRole("textbox", { name: "Message your agent", exact: true });
  await expect(trial).toHaveValue("B's unsent trial");
  await trial.fill("B's edited unsent trial");
  await page.locator('[data-evaluation-id="first"]').click();
  await expect(composer).toHaveValue("A's unsent guide");
  await page.locator('[data-evaluation-id="second"]').click();
  await expect(trial).toHaveValue("B's edited unsent trial");
  await page.reload();
  await expect(trial).toHaveValue("B's edited unsent trial");
  await page.getByRole("button", { name: "Back to Vibe Evals" }).click();
  await expect(composer).toHaveValue("B's separate guide draft");
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("sidebar creation and cancellation preserve the draft and never start a run", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const control = await serve(page);
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("Keep this unsent description.");
  await page.getByRole("button", { name: "New agent", exact: true }).click();
  await expect(page.getByRole("heading", { name: "What would you like AI to handle?" })).toBeVisible();
  await page.getByRole("button", { name: "Back to your evaluation" }).click();
  await expect(composer).toHaveValue("Keep this unsent description.");
  await expect(composer).toBeFocused();
  await page.getByRole("button", { name: "New agent", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(composer).toBeFocused();
  expect(control.posts).toHaveLength(0);
  await page.getByRole("button", { name: "New agent", exact: true }).click();
  await page.getByRole("button", { name: /Improve an existing agent/ }).click();
  const created = control.posts[0].body.id as string;
  await expect(page.locator(`[data-evaluation-id="${created}"]`)).toHaveAttribute("aria-current", "page");
  await expect(composer).toHaveValue("Keep this unsent description.");
  await expect(page.locator('[data-evaluation-id="first"]')).toHaveCount(0);
  expect(control.posts).toEqual([{ path: "/v1/vibe/sessions", body: { id: expect.any(String), door: "test" } }]);
  expect(control.errors).toEqual([]);
});

test("navigation preserves active context while another evaluation completes", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const control = await serve(page);
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("First draft");
  await page.locator('[data-evaluation-id="second"]').click();
  await expect(page.locator('[data-evaluation-id="second"]')).toContainText("Running");
  await page.getByRole("button", { name: "New agent", exact: true }).click();
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
  const trigger = page.getByRole("button", { name: "Open agents", exact: true });
  await trigger.click();
  const drawer = page.getByRole("dialog", { name: "Your agents" });
  await expect(drawer).toBeVisible();
  await expect(drawer).toHaveCSS("opacity", "1");
  await page.screenshot({ path: info.outputPath("sidebar-mobile.png") });
  await page.keyboard.press("Escape");
  await expect(drawer).toBeHidden();
  await expect(trigger).toBeFocused();
  await trigger.click();
  await drawer.getByRole("button", { name: "New agent", exact: true }).click();
  await expect(drawer).toBeHidden();
  await expect(page.getByLabel("New agent choices", { exact: true })).toBeFocused();
  await page.getByRole("button", { name: "Back to your evaluation" }).click();
  await page.getByRole("button", { name: "Paste agent instructions", exact: true }).click();
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
  await expect(page.getByRole("button", { name: "New agent", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Expand sidebar", exact: true }).click();
  await expect(page.getByRole("button", { name: "New agent", exact: true })).toBeVisible();
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("plain-language source choices explain test packs and let an empty choice change doors", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const control = await serve(page);
  await page.getByRole("button", { name: "New agent", exact: true }).click();
  await page.getByRole("button", { name: /Improve an existing agent/ }).click();
  await expect(page.getByRole("heading", { name: "Improve an agent you already have." })).toBeVisible();
  const help = page.getByLabel("What is a test pack?", { exact: true });
  await page.mouse.move(0, 0);
  await help.hover();
  const tooltip = page.locator('[data-slot="tooltip-content"][data-open]');
  await expect(tooltip).toBeVisible();
  await expect(tooltip).toContainText("A test pack (also called a challenge pack) contains situations to try");
  await help.focus();
  await expect(tooltip).toContainText("Import one only if you already have it");
  await page.getByRole("button", { name: "Actually, I want to build an agent.", exact: true }).click();
  await expect(page.getByLabel("New agent choices", { exact: true })).toBeVisible();
  expect(control.posts).toHaveLength(1);
  await page.getByRole("button", { name: /Build an agent/ }).click();
  await expect(page.getByRole("heading", { name: "What would you like help with?" })).toBeVisible();
  expect(control.posts.map(post => post.path)).toEqual([
    "/v1/vibe/sessions",
    "/v1/vibe/sessions",
  ]);
  expect(control.errors).toEqual([]);
});

test("a failed import stays in Improve and clears when returning to the chooser", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 740 });
  const control = await serve(page);
  await page.locator('input[type="file"]').setInputFiles({ name: "broken.json", mimeType: "application/json", buffer: Buffer.from("{") });
  await expect(page.getByRole("alert").filter({ hasText: "Couldn’t import this file" })).toBeVisible();
  await page.getByRole("button", { name: "Actually, I want to build an agent." }).click();
  await expect(page.getByLabel("New agent choices")).toBeVisible();
  await expect(page.getByRole("alert").filter({ hasText: "Couldn’t import this file" })).toHaveCount(0);
  expect(control.posts.map(post => post.path)).toEqual(["/v1/vibe/sessions/first/import"]);
  expect(control.errors).toEqual([]);
});

test("dismissing the extra-run quote restores focus without submitting a paid run", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 600 });
  const control = await serve(page);
  control.sessions.set("build", buildEvaluation());
  await page.goto("/vibe-evals?session=build");
  const actions = page.getByRole("region", { name: "Next actions for this version" });
  await expect(actions).toBeVisible();
  await actions.locator("summary").filter({ hasText: "More ways to check" }).click();
  await actions.getByRole("button", { name: "Try tougher situations" }).click();
  await actions.getByRole("button", { name: "See cost and continue" }).click();
  const dialog = page.getByRole("dialog", { name: "Try tougher situations" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Prepare and run this batch" })).toBeEnabled();
  expect(control.posts.map(post => post.path)).toEqual(["/v1/vibe/sessions/build/build-quote"]);
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toBeFocused();
  expect(control.posts.map(post => post.path)).toEqual(["/v1/vibe/sessions/build/build-quote"]);
  expect(control.errors).toEqual([]);
});

test("long evaluation titles scroll inside the sidebar and saved work stays accessible", async ({ page }, info) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const control = await serve(page, true);
  const list = page.getByRole("navigation", { name: "Your agents" });
  await expect(list.locator("button")).toHaveCount(32);
  expect(await list.evaluate(element => element.scrollHeight > element.clientHeight)).toBe(true);
  const title = list.locator(".vibe-sidebar-item-copy > span").last();
  expect(await title.evaluate(element => element.scrollWidth > element.clientWidth)).toBe(true);
  await list.locator("button").last().scrollIntoViewIfNeeded();
  await expect(page.getByRole("button", { name: "Saved work", exact: true })).toBeInViewport();
  await page.screenshot({ path: info.outputPath("sidebar-history.png") });
  await page.getByRole("button", { name: "Saved work", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "Your saved work" })).toBeVisible();
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await expect(page.locator('[data-evaluation-id="first"]')).toHaveAttribute("aria-current", "page");
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});
