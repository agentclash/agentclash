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
  build.document.artifacts = [{ id: "prototype", kind: "test_suite", title: "Customer email sorter", agent_prompt: "Suggest a label for each email.", accepted: true, source_message_id: "request", proposal_message_id: "prepared", scope_note: "Suggests labels using supplied text.", blueprint: { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: [{ key: "spam", payload: { question: "You won a prize." }, expectations: [{ key: "expected_behavior", kind: "text", value: "Label as Spam." }] }] } }];
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
    extra.title=`Customer support for international returns and exchanges ${index}`;
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
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, two_door: true, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) } });
    if (path.endsWith("/saved-checks")) return route.fulfill({ headers, json: [] });
    if(request.method()==="PATCH"&&path.endsWith("/metadata")){
      const id=path.split("/sessions/")[1].split("/")[0],state=sessions.get(id)!;
      const body=request.postDataJSON();posts.push({path,body});
      if(body.revision!==state.revision)return route.fulfill({status:409,headers,json:{error:{message:"Reload the project"}}});
      if(body.title!==undefined)state.title=body.title.trim();
      if(body.archived!==undefined)state.archived_at=body.archived?new Date().toISOString():undefined;
      state.revision++;state.event_cursor=state.revision;state.updated_at=new Date().toISOString();
      return route.fulfill({headers,json:state});
    }
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
    if (path.endsWith("/sessions")) return route.fulfill({ headers, json: [...sessions.values()].filter(s=>{const mode=new URL(request.url()).searchParams.get("archived");return mode==="all"||(mode==="only"?!!s.archived_at:!s.archived_at)}) });
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

async function openCase(page: Page, index = 0) {
  const row = page.locator(".vibe-result-row").nth(index);
  if (!(await row.evaluate(element => (element as HTMLDetailsElement).open))) await row.locator("summary").click();
  return row;
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
  const openGuide = async (id: string) => {
    await expect(page.locator(`[data-evaluation-id="${id}"]`)).toHaveAttribute("aria-current", "page");
    const button = page.getByRole("button", { name: "Guide", exact: true });
    if (await button.getAttribute("aria-pressed") !== "true") await button.click();
  };
  await expect(page.getByRole("button", { name: "Guide", exact: true })).toHaveAttribute("aria-pressed", "false");
  await openGuide("first");
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("A's unsent guide");
  await page.locator('[data-evaluation-id="second"]').click();
  await openGuide("second");
  await expect(composer).toHaveValue("B's separate guide draft");
  await page.getByRole("button", { name: "Try it yourself", exact: true }).click();
  const trial = page.getByRole("textbox", { name: "Message your agent", exact: true });
  await expect(trial).toHaveValue("B's unsent trial");
  await trial.fill("B's edited unsent trial");
  await page.locator('[data-evaluation-id="first"]').click();
  await openGuide("first");
  await expect(composer).toHaveValue("A's unsent guide");
  await page.locator('[data-evaluation-id="second"]').click();
  await openGuide("second");
  await expect(trial).toHaveValue("B's edited unsent trial");
  await page.reload();
  await expect(trial).toHaveValue("B's edited unsent trial");
  await page.getByRole("tab", { name: "Agent", exact: true }).click();
  await openGuide("second");
  await expect(composer).toHaveValue("B's separate guide draft");
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("sidebar creation and cancellation preserve the draft and never start a run", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const control = await serve(page);
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("Keep this unsent description.");
  await page.getByRole("button", { name: "New evaluation", exact: true }).click();
  await expect(page.getByRole("heading", { name: "What would you like to evaluate?" })).toBeVisible();
  await page.getByRole("button", { name: "Back to your evaluation" }).click();
  await expect(composer).toHaveValue("Keep this unsent description.");
  await expect(composer).toBeFocused();
  await page.getByRole("button", { name: "New evaluation", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(composer).toBeFocused();
  expect(control.posts).toHaveLength(0);
  await page.getByRole("button", { name: "New evaluation", exact: true }).click();
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

test("drawer supports keyboard dismissal, focus return and source selection on small screens", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  const control = await serve(page);
  const trigger = page.getByRole("button", { name: "Open agents", exact: true });
  await trigger.click();
  const drawer = page.getByRole("dialog", { name: "Your agents" });
  await expect(drawer).toBeVisible();
  await expect(drawer).toHaveCSS("opacity", "1");
  await page.keyboard.press("Escape");
  await expect(drawer).toBeHidden();
  await expect(trigger).toBeFocused();
  await trigger.click();
  await drawer.getByRole("button", { name: "New evaluation", exact: true }).click();
  await expect(drawer).toBeHidden();
  await expect(page.getByLabel("New evaluation choices", { exact: true })).toBeFocused();
  await page.getByRole("button", { name: "Back to your evaluation" }).click();
  await page.getByRole("button", { name: "Try my saved instructions", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Improve an agent you already have." })).toBeHidden();
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("entry, context label and composer fit mobile, desktop and zoom-sized layouts", async ({ page }) => {
  const control = await serve(page);
  for (const width of [320, 390, 640, 768, 1280, 1440]) {
    await page.setViewportSize({ width, height: width === 640 ? 450 : 900 });
    await expect(page.getByRole("heading", {name:"Improve an agent you already have.",exact:true})).toBeVisible();
    await expect(page.getByRole("button", { name: "Try my saved instructions", exact: true })).toBeVisible();
    const dock = page.getByTestId("vibe-composer-dock");
    await expect(dock).toBeVisible();
    await expect.poll(async () => {
      const box = await dock.boundingBox();
      return box ? box.y + box.height - await page.evaluate(() => innerHeight) : Infinity;
    }).toBeLessThanOrEqual(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const input = page.getByRole("textbox", { name: "Message Vibe Evals" });
    await input.fill("A long description ".repeat(120));
    await expect(page.getByRole("button", { name: "Send message", exact: true })).toBeVisible();
    expect((await input.boundingBox())!.height).toBeLessThanOrEqual(240);
    await input.fill("");
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
  const help = page.getByLabel("What is a test pack?", { exact: true });
  await page.mouse.move(0, 0);
  await help.hover();
  const tooltip = page.locator('[data-slot="tooltip-content"][data-open]');
  await expect(tooltip).toBeVisible();
  await expect(tooltip).toContainText("A test pack (also called a challenge pack) contains situations to try");
  await help.focus();
  await expect(tooltip).toContainText("Import one only if you already have it");
  await page.getByRole("button", { name: "Actually, I want to build an agent.", exact: true }).click();
  await expect(page.getByLabel("New evaluation choices", { exact: true })).toBeVisible();
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
  await expect(page.getByLabel("New evaluation choices")).toBeVisible();
  await expect(page.getByRole("alert").filter({ hasText: "Couldn’t import this file" })).toHaveCount(0);
  expect(control.posts.map(post => post.path)).toEqual(["/v1/vibe/sessions/first/import"]);
  expect(control.errors).toEqual([]);
});

test("dismissing the extra-run quote restores focus without submitting a paid run", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 600 });
  const control = await serve(page);
  control.sessions.set("build", buildEvaluation());
  await page.goto("/vibe-evals?session=build");
  await page.getByRole("tab", { name: "Results", exact: true }).click();
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
  await expect(actions.getByRole("button", { name: "See cost and continue" })).toBeFocused();
  expect(control.posts.map(post => post.path)).toEqual(["/v1/vibe/sessions/build/build-quote"]);
  expect(control.errors).toEqual([]);
});

test("long evaluation titles scroll inside the sidebar and saved work stays accessible", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const control = await serve(page, true);
  const list = page.getByRole("navigation", { name: "Your agents" });
  await expect(list.locator("[data-evaluation-id]")).toHaveCount(32);
  expect(await list.evaluate(element => element.scrollHeight > element.clientHeight)).toBe(true);
  const title = list.locator(".vibe-sidebar-item-copy > span").last();
  expect(await title.evaluate(element => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await list.locator("button").last().scrollIntoViewIfNeeded();
  await expect(page.getByRole("button", { name: "Saved work", exact: true })).toBeInViewport();
  await page.getByRole("button", { name: "Saved work", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "Your saved work" })).toBeVisible();
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await expect(page.locator('[data-evaluation-id="first"]')).toHaveAttribute("aria-current", "page");
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});


test("canonical Tests preserves an unsaved case through results, project switching and reload, with Back restoring the view", async ({ page }) => {
  const control = await serve(page, false, true);
  await page.getByRole("tab", { name: "Tests", exact: true }).click();
  await expect(page).toHaveURL(/view=tests/);
  await page.getByRole("button", { name: "Edit tests", exact: true }).click();
  const expectation = page.getByRole("textbox", { name: "What should happen", exact: true });
  await expectation.fill("Keep this unsaved expectation.");
  await page.getByRole("tab", { name: "Results", exact: true }).click();
  await expect(page).toHaveURL(/view=results/);
  await page.goBack(); await expect(page).toHaveURL(/view=tests/);
  await expect(expectation).toHaveValue("Keep this unsaved expectation.");
  await page.locator('[data-evaluation-id="second"]').click();
  await page.locator('[data-evaluation-id="first"]').click();
  await expect(page).toHaveURL(/view=tests/);
  await expect(expectation).toHaveValue("Keep this unsaved expectation.");
  await page.reload(); await expect(expectation).toHaveValue("Keep this unsaved expectation.");
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByRole("heading", { name: "1 test is ready", exact: true })).toBeFocused();
  expect(control.posts).toHaveLength(0); expect(control.errors).toEqual([]);
});

test("same-artifact case Back and Forward restores the visible expectation without inference", async ({ page }) => {
  const control = await serve(page, false, true);
  const artifact = control.first.document.artifacts[0];
  const blueprint = artifact.blueprint as { cases: unknown[] };
  blueprint.cases.push({ key: "invoice", payload: { question: "Invoice attached." }, expectations: [{ key: "expected_behavior", kind: "text", value: "Label as Billing." }] });
  await page.reload();
  await page.getByRole("tab", { name: "Tests", exact: true }).click();
  await (await openCase(page, 0)).getByRole("button", {name:"Edit this expectation",exact:true}).click();
  const expectation = page.locator('[aria-label="Your tests"] textarea');
  await expect(expectation).toHaveValue("Label as Spam.");
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await (await openCase(page, 1)).getByRole("button", {name:"Edit this expectation",exact:true}).click();
  await expect(expectation).toHaveValue("Label as Billing.");
  await page.goBack(); await expect(expectation).toHaveCount(0);
  await page.goBack(); await expect(expectation).toHaveValue("Label as Spam.");
  await page.goForward(); await expect(expectation).toHaveCount(0);
  await page.goForward(); await expect(expectation).toHaveValue("Label as Billing.");
  expect(control.posts).toHaveLength(0); expect(control.errors).toEqual([]);
});

test("text roles stay consistent across viewport sizes and portaled dialogs", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  const control = await serve(page, false, true);
  const check = control.first.operations.find(operation => operation.kind === "check")!;
  await page.route(/\/v1\/vibe\/operations\/check\/case\?/, route => route.fulfill({
    headers: { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" },
    json: check.results[0],
  }));
  await page.route(/\/v1\/vibe\/credits\?/, route => route.fulfill({
    headers: { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" },
    json: { under_review: false, balance_nano_usd: 0, held_nano_usd: 0, available_nano_usd: 0, products: [] },
  }));
  await page.goto("/vibe-evals?session=first&workspace=a8000000-0000-4000-8000-000000000003");
  await page.getByRole("tab", { name: "Results", exact: true }).click();
  await page.getByText("Test results · 1", { exact: true }).click();
  await openCase(page);
  const reply = page.locator(".vibe-result-row .vibe-markdown").first();
  await expect.soft(reply).toHaveCSS("font-size", "16px");
  await expect.soft(reply).toHaveCSS("line-height", "24px");
  await page.getByRole("tab", { name: "Agent", exact: true }).click();
  for (const width of [390, 1440, 2560]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(page.locator(".vibe-project-identity h1")).toHaveCSS("font-size", "20px");
    await expect(page.getByRole("heading", { name: "Agent instructions", exact: true })).toHaveCSS("font-size", "16px");
    await expect(page.getByRole("heading", { name: "Agent instructions", exact: true })).toHaveCSS("line-height", "22px");
    await expect(page.locator(".vibe-instructions-text")).toHaveCSS("font-size", "16px");
    await expect(page.getByRole("textbox", { name: "Message your agent", exact: true })).toHaveCSS("font-size", "16px");
    await expect(page.getByRole("tab", { name: "Agent", exact: true })).toHaveCSS("font-size", "14px");
    if (width < 1024) await page.getByRole("button", { name: "Open agents", exact: true }).click();
    const navigation = width < 1024 ? page.getByRole("dialog", { name: "Your agents", exact: true }) : page.getByRole("complementary", { name: "Agent sidebar", exact: true });
    await expect(navigation.locator('[data-evaluation-id="first"] .vibe-project-full-title')).toHaveCSS("font-size", "14px");
    await expect(navigation.locator('[data-evaluation-id="first"] small')).toHaveCSS("font-size", "12px");
    const actions = navigation.locator(".vibe-project-actions");
    if (!(await actions.evaluate(element => (element as HTMLDetailsElement).open))) await actions.locator("summary").click();
    await actions.getByRole("button", { name: "Delete this project", exact: true }).click();
    const deletion = page.getByRole("dialog", { name: /^Delete / });
    await expect.soft(deletion.locator('[data-slot="dialog-title"]')).toHaveCSS("font-size", "20px");
    await deletion.getByRole("button", { name: "Cancel", exact: true }).click();
    await page.getByRole("button", { name: "Details", exact: true }).click();
    const details = width < 1280 ? page.getByRole("dialog", { name: "Details", exact: true }) : page.getByRole("complementary", { name: "Details utility", exact: true });
    await expect(details.getByLabel("Appearance", { exact: true })).toHaveCSS("font-size", "16px");
    await details.getByRole("button", { name: "Settings", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Settings and details", exact: true });
    await expect.soft(dialog).toHaveCSS("font-family", /Geist/);
    await expect(dialog.getByRole("heading", { name: "Settings and details", exact: true })).toHaveCSS("font-size", "20px");
    await expect(dialog.locator('[data-slot="dialog-description"]')).toHaveCSS("font-size", "14px");
    await expect(dialog.getByRole("button", { name: "Import a test pack", exact: true })).toHaveCSS("font-size", "14px");
    await dialog.getByRole("button", { name: "Credits", exact: true }).click();
    const credits = page.getByRole("dialog", { name: "Workspace AI credits", exact: true });
    await expect.soft(credits.locator('[data-slot="dialog-title"]')).toHaveCSS("font-size", "20px");
    await credits.getByRole("button", { name: "Close", exact: true }).click();
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    await page.getByRole("button", { name: "Close Details", exact: true }).click();
  }
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("Guide keeps its composer in view through short-window resizing without losing a draft", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  const control = await serve(page, false, true);
  await page.getByRole("tab", { name: "Tests", exact: true }).click();
  await openCase(page);
  await page.getByRole("button", { name: "Ask Guide about this test", exact: true }).click();
  const input = page.getByRole("textbox", { name: "Message Vibe Evals", exact: true });
  const draft = "Keep this unsent question while I resize the window.";
  await input.fill(draft);
  for (const size of [
    { width: 320, height: 568 }, { width: 844, height: 390 },
    { width: 390, height: 300 }, { width: 1280, height: 300 }, { width: 1280, height: 600 },
    { width: 1920, height: 1080 },
  ]) {
    await page.setViewportSize(size);
    await expect(input).toHaveValue(draft);
    await expect(page.getByLabel("Guide reference", { exact: true })).toContainText("You won a prize.");
    for (const control of [input, page.getByRole("button", { name: "Send message", exact: true })]) {
      await expect.poll(async () => {
        const box = await control.boundingBox();
        return !!box && box.y >= 0 && box.y + box.height <= size.height &&
          box.x >= 0 && box.x + box.width <= size.width;
      }, { message: `Guide input and Send fit ${size.width}×${size.height} before scrolling` }).toBe(true);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.getByText("Data use", { exact: true }).click();
  await expect(page.getByText(/Guest uploads expire after seven days/)).toBeVisible();
  await page.getByText("Data use", { exact: true }).click();
  expect(control.posts).toHaveLength(0);
  expect(control.errors).toEqual([]);
});

test("workspace geometry, scoped portal appearance and reading focus adapt at every specified boundary", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  const control = await serve(page, false, true);
  for (const width of [390, 600, 601, 1023, 1024, 1279, 1280, 1440, 1920, 2560]) {
    await page.setViewportSize({ width, height: 900 });
    // Resize preserves the utility value; close the sheet to inspect main content.
    const close = page.getByRole("button", { name: "Close Guide", exact: true });
    if (width < 1280 && await close.isVisible()) await close.click();
    await page.getByRole("button", { name: "Details", exact: true }).click();
    const appearance = page.getByLabel("Appearance", { exact: true });
    await appearance.selectOption("light");
    const geometry = await page.evaluate(() => {
      const sidebar = document.querySelector<HTMLElement>(".vibe-sidebar")!;
      const utility = document.querySelector<HTMLElement>('[data-slot="sheet-content"], .vibe-utility')!;
      const popup = getComputedStyle(utility);
      const backdrop = document.querySelector<HTMLElement>('[data-slot="sheet-overlay"]');
      return { width: innerWidth, sidebar: getComputedStyle(sidebar).display === "none" ? 0 : sidebar.getBoundingClientRect().width, utility: utility.getBoundingClientRect().width, height: utility.getBoundingClientRect().height, background: popup.backgroundColor, color: popup.color, transition: popup.transitionDuration, backdropMotion: backdrop ? getComputedStyle(backdrop).transitionDuration : null, overflow: document.documentElement.scrollWidth > innerWidth, globalTheme: document.documentElement.classList.contains("dark") };
    });
    expect(geometry.sidebar).toBe(width >= 1024 ? 252 : 0);
    if (width >= 1280) {
      expect(geometry.utility).toBeGreaterThanOrEqual(320);
      expect(geometry.utility).toBeLessThanOrEqual(480);
      expect(geometry.utility).toBeLessThan((width - geometry.sidebar) / 2);
    } else expect(geometry.utility).toBe(width <= 600 ? width : 480);
    if (width <= 600) expect(geometry.height).toBe(900);
    expect(geometry.background).toBe("rgb(255, 255, 255)"); expect(geometry.color).toBe("rgb(37, 36, 31)");
    expect(geometry.transition).toBe("0s"); if (width < 1280) expect(geometry.backdropMotion).toBe("0s");
    expect(geometry.overflow).toBe(false); expect(geometry.globalTheme).toBe(true);
    await appearance.selectOption("dark");
    const utility = page.locator(width >= 1280 ? ".vibe-utility" : '[data-slot="sheet-content"]');
    await expect(utility).toHaveCSS("background-color", "rgb(25, 25, 22)");
    await appearance.selectOption("system");
    await expect(utility).toHaveCSS("background-color", "rgb(255, 255, 255)");
    await page.getByRole("button", { name: "Close Details", exact: true }).click();
    await expect(page.getByRole("button", { name: "Details", exact: true })).toBeFocused();
  }
  await page.getByRole("button", { name: "Guide", exact: true }).click();
  await page.getByRole("button", { name: "New response", exact: true }).click();
  await expect(page.getByLabel("Guide response", { exact: true })).toBeFocused();
  await page.getByRole("tab", { name: "Tests", exact: true }).click();
  await openCase(page);
  await page.getByRole("button", { name: "Ask Guide about this test", exact: true }).click();
  await expect(page.getByLabel("Guide reference", { exact: true })).toContainText("You won a prize.");
  await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toBeFocused();
  expect(control.posts).toHaveLength(0); expect(control.errors).toEqual([]);
});


test("remaining 320 and 768 layouts retain contrast, scaled text and utility focus", async ({ page }) => {
  await page.setViewportSize({width:320,height:900});
  await page.emulateMedia({colorScheme:"dark",reducedMotion:"reduce"});
  const control=await serve(page,false,true);
  for(const width of [320,768]) {
    await page.setViewportSize({width,height:900});
    const results=(await page.getByRole("tab",{name:"Results",exact:true}).boundingBox())!;
    const guide=(await page.getByRole("button",{name:"Guide",exact:true}).boundingBox())!;
    const details=(await page.getByRole("button",{name:"Details",exact:true}).boundingBox())!;
    expect(results.x+results.width).toBeLessThanOrEqual(guide.x);
    expect(guide.x+guide.width).toBeLessThanOrEqual(details.x);
    for(const box of [results,guide,details]) {
      expect(box.width).toBeGreaterThanOrEqual(44);
      expect(box.height).toBeGreaterThanOrEqual(44);
    }
    await page.getByRole("button",{name:"Details",exact:true}).click();
    const popup=page.locator('[data-slot="sheet-content"]');
    const appearance=page.getByLabel("Appearance",{exact:true});
    await appearance.selectOption("system");
    await expect(popup).toHaveCSS("background-color","rgb(25, 25, 22)");
    await expect(popup).toHaveCSS("color","rgb(238, 236, 229)");
    await appearance.selectOption("light");
    await expect(popup).toHaveCSS("background-color","rgb(255, 255, 255)");
    const geometry=await popup.evaluate(el=>({width:el.getBoundingClientRect().width,height:el.getBoundingClientRect().height,background:getComputedStyle(el).backgroundColor,color:getComputedStyle(el).color,transition:getComputedStyle(el).transitionDuration,overflow:document.documentElement.scrollWidth>innerWidth}));
    expect(geometry.width).toBe(width<=600?width:480); expect(geometry.height).toBe(900); expect(geometry.overflow).toBe(false); expect(geometry.transition).toBe("0s");
    await page.keyboard.press("Escape");
    await expect(page.getByRole("button",{name:"Details",exact:true})).toBeFocused();
  }
  // Text-only enlargement doubles each element's computed font size; this is
  // deterministic Linux coverage, not a claim about Safari or physical zoom.
  await page.setViewportSize({width:768,height:900});
  await page.evaluate(()=> { const elements=[...document.querySelectorAll<HTMLElement>(".vibe-workspace *")]; const sizes=elements.map(el=>parseFloat(getComputedStyle(el).fontSize)); elements.forEach((el,i)=>{el.style.fontSize=`${sizes[i]*2}px`;}); });
  for (const width of [320, 768]) {
    await page.setViewportSize({ width, height: 900 });
    const results = (await page.getByRole("tab", { name: "Results", exact: true }).boundingBox())!;
    const guide = (await page.getByRole("button", { name: "Guide", exact: true }).boundingBox())!;
    const overlaps = results.x < guide.x + guide.width && results.x + results.width > guide.x &&
      results.y < guide.y + guide.height && results.y + results.height > guide.y;
    expect(overlaps, `Results and Guide remain separate with 200% text at ${width}px`).toBe(false);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  }
  await page.emulateMedia({forcedColors:"active"});
  await page.getByRole("button",{name:"Details",exact:true}).click();
  const forced=await page.locator('[data-slot="sheet-content"]').evaluate(el=>({forced:matchMedia("(forced-colors: active)").matches,background:getComputedStyle(el).backgroundColor,color:getComputedStyle(el).color}));
  expect(forced.forced).toBe(true); expect(forced.background).not.toBe(forced.color);
  await page.keyboard.press("Escape"); await expect(page.getByRole("button",{name:"Details",exact:true})).toBeFocused();
  expect(control.posts).toHaveLength(0); expect(control.errors).toEqual([]);
});


test("an explicit pre-run Guide reference submits only its identity and the user's question", async ({page}) => {
 const control=await serve(page,false,true);
 await page.getByRole("tab",{name:"Tests",exact:true}).click();
 await openCase(page);
 await page.getByRole("button",{name:"Ask Guide about this test",exact:true}).click();
 expect(control.posts).toHaveLength(0);
 await page.getByRole("textbox",{name:"Message Vibe Evals"}).fill("Explain this example.");
 await page.getByRole("button",{name:"Send message",exact:true}).click();
 await expect.poll(()=>control.posts.length).toBe(1);
 expect(control.posts[0].body).toMatchObject({kind:"message",content:"Explain this example.",artifact_id:"prototype",viewed_case_key:"spam"});
 expect(control.posts[0].body).not.toHaveProperty("baseline_id");
 expect(JSON.stringify(control.posts[0].body)).not.toContain("Label as Spam.");
 // The fake boundary rejects inference; request construction is the owner here.
 await expect(page.getByRole("alert").filter({hasText:"Unexpected mutation in navigation test"}).first()).toBeVisible();
 expect(control.errors).toEqual([]);
});

test("Settings descendants retain readable scoped appearance including warning, control and focus states", async ({page}) => {
  await page.setViewportSize({width:1440,height:900});
  await page.emulateMedia({colorScheme:"light",reducedMotion:"reduce"});
  const control=await serve(page,false,true);
  await page.route("**/v1/vibe/config",route=>route.fulfill({headers:{"Access-Control-Allow-Origin":new URL(page.url()).origin,"Access-Control-Allow-Credentials":"true"},json:{enabled:true,two_door:true,local_testing:true,defaults:defaultModels,models:[...new Set([...Object.values(defaultModels),"fixture/alternative"])].map(id=>({id,name:id})),availability:{guest:{available:true},workspace:{available:true}}}}));
  control.first.operations[1].state="FAILED";
  control.first.operations[1].error={code:"provider_unavailable",message:"Fixture provider is unavailable."};
  await page.reload(); await page.getByRole("tab",{name:"Results",exact:true}).click();
  const globalBefore=await page.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue("--builder-fg-muted"));
  await page.getByRole("button",{name:"Details",exact:true}).click();
  for(const [appearance,system] of [["light","dark"],["dark","light"],["system","light"],["system","dark"]] as const) {
    await page.emulateMedia({colorScheme:system,reducedMotion:"reduce"});
    await page.getByLabel("Appearance",{exact:true}).selectOption(appearance);
    await page.getByRole("complementary",{name:"Details utility"}).getByRole("button",{name:"Settings",exact:true}).click();
    const dialog=page.getByRole("dialog",{name:"Settings and details"});
    const model=dialog.getByLabel("Assistant model",{exact:true});
    await model.focus(); await page.keyboard.press("Tab"); await page.keyboard.press("Shift+Tab");
    const action=dialog.getByRole("button",{name:"Import a test pack",exact:true}); await action.hover();
    const measurement=await dialog.evaluate(popup=> {
      const canvas=document.createElement("canvas"), context=canvas.getContext("2d")!;
      const rgba=(color:string)=>{context.clearRect(0,0,1,1);context.fillStyle=color;context.fillRect(0,0,1,1);return [...context.getImageData(0,0,1,1).data].map((v,i)=>i===3?v/255:v);};
      const blend=(fg:number[],bg:number[])=>fg.slice(0,3).map((v,i)=>v*fg[3]+bg[i]*(1-fg[3]));
      const background=(element:Element)=>{const layers:number[][]=[];for(let el:Element|null=element;el;el=el.parentElement){const c=rgba(getComputedStyle(el).backgroundColor);layers.push(c);if(c[3]===1)break;}return layers.reverse().reduce((bg,c)=>blend(c,bg),[255,255,255]);};
      const luminance=(c:number[])=>c.map(v=>{const n=v/255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4;}).reduce((sum,v,i)=>sum+v*[.2126,.7152,.0722][i],0);
      const ratio=(a:number[],b:number[])=>{const x=luminance(a),y=luminance(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05);};
      const sample=(element:Element)=>{const style=getComputedStyle(element),bg=background(element),fg=blend(rgba(style.color),bg);return {text:element.textContent?.trim().slice(0,80),color:style.color,background:bg,contrast:ratio(fg,bg)};};
      const select=popup.querySelector<HTMLSelectElement>('[aria-label="Assistant model"]')!, css=getComputedStyle(select), bg=background(select);
      const help=[...popup.querySelectorAll("p")].filter(el=>/Plans the check|Generates replies|Grades replies|Model changes apply/.test(el.textContent||""));
      const labels=[...popup.querySelectorAll("label")].filter(el=>el.querySelector('select[aria-label$=" model"]'));
      const warning=[...document.querySelectorAll('[role="alert"]')].find(el=>el.textContent?.includes("Fixture provider is unavailable."))!;
      const ringColor=[...css.boxShadow.matchAll(/(?:rgba?|oklch|color)\([^)]*\)/g)].map(match=>match[0]).filter(color=>rgba(color)[3]>0).at(-1);
      const action=[...popup.querySelectorAll("button")].find(button=>button.textContent?.includes("Import a test pack"))!;
      return {action:sample(action),actionBorderContrast:ratio(blend(rgba(getComputedStyle(action).borderTopColor),background(action)),background(action)),labels:labels.map(sample),help:help.map(sample),control:sample(select),warning:sample(warning),border:css.borderTopColor,borderContrast:ratio(blend(rgba(css.borderTopColor),bg),bg),focus:{visible:select.matches(":focus-visible"),shadow:css.boxShadow,contrast:ringColor?ratio(blend(rgba(ringColor),bg),bg):0}};
    });
    expect(measurement.labels.length).toBe(3); expect(measurement.help.length).toBe(4);
    for(const row of [...measurement.labels,...measurement.help,measurement.control,measurement.warning,measurement.action]) expect(row.contrast,`${appearance}/${system}: ${row.text}`).toBeGreaterThanOrEqual(4.5);
    expect(measurement.borderContrast).toBeGreaterThanOrEqual(3); expect(measurement.actionBorderContrast).toBeGreaterThanOrEqual(3);
    expect(measurement.focus.visible).toBe(true); expect(measurement.focus.contrast).toBeGreaterThanOrEqual(3);
    await expect(action).toBeVisible();
    await dialog.getByRole("button",{name:"Close",exact:true}).click();
  }
  expect(await page.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue("--builder-fg-muted"))).toBe(globalBefore);
  expect(control.posts).toHaveLength(0);expect(control.errors).toEqual([]);
});

test("library search rename archive and sidebar preference preserve full touch and keyboard titles without running",async({page})=>{
 const control=await serve(page,true);
 const search = page.getByRole("searchbox",{name:"Search loaded projects"});
 const assertSearchAffordance = async () => {
   const control = await search.evaluate(input => {
     const style=getComputedStyle(input), rgb=(value:string)=>value.match(/[\d.]+/g)!.slice(0,3).map(Number);
     const luminance=(value:string)=>rgb(value).map(v=>{const n=v/255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4;}).reduce((total,v,i)=>total+v*[.2126,.7152,.0722][i],0);
     const border=luminance(style.borderTopColor), background=luminance(style.backgroundColor);
     return {height:input.getBoundingClientRect().height,border:parseFloat(style.borderTopWidth),contrast:(Math.max(border,background)+.05)/(Math.min(border,background)+.05)};
   });
   expect(control.height).toBeGreaterThanOrEqual(44); expect(control.border).toBeGreaterThanOrEqual(1); expect(control.contrast).toBeGreaterThanOrEqual(3);
 };
 await assertSearchAffordance();
 await search.fill("exchanges 29");
 await expect(page.locator("[data-evaluation-id]")).toHaveCount(1);
 await page.locator('[data-evaluation-id="older-29"]').click();
 await page.getByText("Project actions",{exact:true}).click();
 await page.getByRole("button",{name:"Rename project",exact:true}).click();
 const title="International customer support – returns, exchanges, warranty exceptions and multilingual delivery questions for the autumn project";
 await page.getByRole("textbox",{name:"Project title",exact:true}).fill(title);
 await page.getByRole("textbox",{name:"Project title",exact:true}).press("Enter");
 await expect(page.getByRole("button",{name:"Rename project",exact:true})).toBeVisible();
 await page.getByRole("searchbox",{name:"Search loaded projects"}).fill("autumn project");
 const row=page.locator('[data-evaluation-id="older-29"]');await expect(row).toContainText(title);
 await row.focus();expect(await row.locator('.vibe-project-full-title').evaluate(node=>node.scrollHeight<=node.clientHeight+1)).toBe(true);
 await page.getByRole("button",{name:"Archive project",exact:true}).click();await expect(row).toHaveCount(0);
 await page.getByLabel("Project archive filter").selectOption("only");await expect(row).toHaveCount(1);
 await page.getByRole("button",{name:"Restore project",exact:true}).click();await expect(row).toHaveCount(0);
 await page.getByLabel("Project archive filter").selectOption("exclude");await expect(row).toHaveCount(1);
 await page.getByRole("button",{name:"Collapse sidebar",exact:true}).click();await page.reload();await expect(page.getByRole("button",{name:"Expand sidebar",exact:true})).toBeVisible();
 await page.setViewportSize({width:390,height:844});if(await page.getByRole("button",{name:"Close Guide",exact:true}).isVisible())await page.getByRole("button",{name:"Close Guide",exact:true}).click();await page.getByRole("button",{name:"Open agents",exact:true}).click();
 const mobile=page.getByRole("dialog");await expect(mobile.locator('[data-evaluation-id="older-29"]')).toContainText(title);
 await assertSearchAffordance();
 expect(await mobile.locator('[data-evaluation-id="older-29"] .vibe-project-full-title').evaluate(node=>node.scrollHeight<=node.clientHeight+1)).toBe(true);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 expect(control.posts.every(p=>p.path.endsWith('/metadata'))).toBe(true);expect(control.errors).toEqual([]);
});
