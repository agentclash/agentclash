// Failure branches of the entry and both doors (verification.md section 6 H), with the stateful
// fake (no API, no model). Where today's behaviour falls short of the target, the target is a
// test.fail that errors once it is met.
import { expect, test, type Page } from "@playwright/test";
import { description, fakeVibe, instructions, transcript, expectation, type FakeOptions } from "./entry-fake";
import { writeRecord } from "./flow-meter";
import { chooseDoor, composer, doorButton, doorReady, entryNotice, entryReady, improveSource, sendButton, workspacePicker, runTestsButton, improvePrimary, repliesInput, rulesInput, sampleButton, sourceButton, startComposer } from "./entry-model";

test.use({ viewport: { width: 1440, height: 960 }, reducedMotion: "reduce" });
test.describe.configure({ timeout: 90_000 });

const signedIn: FakeOptions = { signedIn: true, config: { availability: { guest: { available: false }, workspace: { available: true } } } };
const noCredits = { status: 402, code: "insufficient_credits", message: "This workspace has no AI credits.", admission: "rejected" as const };
const known = (id: string) => `Known defect: see ~/.claude/plans/vibe-evals-redesign.md section 8.3 ${id}`;
const doorsDisabled = async (page: Page) => { for (const door of ["build", "improve"] as const) await expect(doorButton(page, door)).toBeDisabled(); };
// A delayed automatic retry would come with the next event stream reconnect, so wait for one.
async function afterReconnect(fake: { events: () => number }) {
  const seen = fake.events();
  await expect.poll(() => fake.events(), { timeout: 15_000 }).toBeGreaterThan(seen);
}
const writes = (fake: { posts: { method: string; path: string }[] }) => fake.posts.map(post => `${post.method} ${post.path.replace(/sessions\/[^/]+/, "sessions/:id")}`);

// The local AuthKit fixture signs in; the fake answers /v1/users/me.
async function signIn(page: Page) {
  await page.goto("/auth/login?returnTo=%2Fvibe-evals");
  await page.getByRole("button", { name: "Continue with AgentClash" }).click();
}

// The target for H7: the no-credit error offers a way to the workspace credits next to it.
async function creditsActionNextTo(page: Page, error: string) {
  const near = page.getByRole("alert").filter({ hasText: error }).locator("xpath=..");
  await expect(near.getByRole("button", { name: /credits/i }).or(near.getByRole("link", { name: /credits/i })).first()).toBeVisible({ timeout: 2_000 });
}

for (const viewer of [false, true]) test(`H3 signed in with ${viewer ? "only viewer workspaces" : "no workspace"} shows one message and one action`, async ({ page }) => {
  const fake = await fakeVibe(page, { ...signedIn, me: { user_id: "a8000000-0000-4000-8000-000000000001", organizations: viewer ? [{ id: "org", name: "Returns Co", slug: "returns-co", role: "org_member", workspaces: [{ id: "a8000000-0000-4000-8000-000000000003", name: "Returns team", slug: "returns", role: "workspace_viewer" }] }] : [] } });
  await signIn(page);
  await expect(entryNotice(page).getByRole("status")).toHaveText(/^You need a workspace to start a project\./, { timeout: 20_000 });
  await expect(entryNotice(page).locator("a, button")).toHaveCount(1);
  await expect(entryNotice(page).getByRole("link", { name: "Open your dashboard", exact: true })).toHaveAttribute("href", "/dashboard");
  await expect(workspacePicker(page)).toHaveCount(0);
  await expect(page.locator(".vibe-sidebar-workspace")).toHaveCount(0);
  await doorsDisabled(page);
  expect(fake.posts).toEqual([]);
  expect(fake.errors).toEqual([]);
});

test("H4 workspaces that fail to load offer Try again, which recovers", async ({ page }) => {
  const failures: FakeOptions["failures"] = { "GET /users/me": { status: 503, code: "unavailable", message: "Unavailable" } };
  const fake = await fakeVibe(page, { ...signedIn, failures });
  await signIn(page);
  await expect(entryNotice(page).getByRole("alert")).toHaveText("Your workspaces couldn’t be loaded.", { timeout: 20_000 });
  await expect(entryNotice(page).locator("a, button")).toHaveCount(1);
  await doorsDisabled(page);
  delete failures["GET /users/me"];
  await entryNotice(page).getByRole("button", { name: "Try again", exact: true }).click();
  await expect(workspacePicker(page)).toBeVisible();
  await expect(entryNotice(page).getByRole("alert")).toHaveCount(0);
  await expect(entryNotice(page).getByRole("status")).toHaveCount(0);
  expect(fake.posts).toEqual([]);
  expect(fake.errors).toEqual([]);
});

test("H5 an expired sign-in offers Sign in again back to Vibe Evals", async ({ page }) => {
  const fake = await fakeVibe(page, { ...signedIn, failures: { "GET /users/me": { status: 401, code: "unauthenticated", message: "Sign in again." } } });
  await signIn(page);
  await expect(entryNotice(page).getByRole("alert")).toHaveText("Your sign-in expired. Sign in again to choose a workspace.", { timeout: 20_000 });
  await expect(entryNotice(page).locator("a, button")).toHaveCount(1);
  await expect(entryNotice(page).getByRole("link", { name: "Sign in again", exact: true })).toHaveAttribute("href", "/auth/login?returnTo=%2Fvibe-evals");
  await doorsDisabled(page);
  expect(fake.posts).toEqual([]);
  expect(fake.errors).toEqual([]);
});

test.describe("H7 no credits", () => {
  test("on the first Build send: the error shows, the draft stays and nothing retries", async ({ page }) => {
    const fake = await fakeVibe(page, { ...signedIn, failures: { "POST /messages:message": noCredits } });
    await signIn(page);
    await workspacePicker(page).selectOption({ label: "Returns team" });
    await chooseDoor(page, "build");
    await doorReady(page, "build");
    await composer(page).fill(description);
    await expect(sendButton(page)).toBeEnabled({ timeout: 20_000 });
    await sendButton(page).click();
    await expect(page.getByRole("alert").filter({ hasText: noCredits.message })).toBeVisible();
    await expect(sendButton(page)).toBeEnabled();
    await expect(composer(page)).toHaveValue(description);
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await page.reload();
    await expect(composer(page)).toHaveValue(description);
    await expect(sendButton(page)).toBeEnabled();
    await writeRecord("h7-build", { alerts: await page.getByRole("alert").allInnerTexts(), writes: writes(fake) });
    expect(writes(fake).filter(write => !write.endsWith("/build-quote"))).toEqual(["POST /sessions", "POST /sessions/:id/messages", "DELETE /sessions/:id"]);
    expect(fake.errors).toEqual([]);
  });

  test("on the first Build send: a Credits action sits next to the error", async ({ page }) => {
    test.fail(true, known("H7"));
    await fakeVibe(page, { ...signedIn, failures: { "POST /messages:message": noCredits } });
    await signIn(page);
    await workspacePicker(page).selectOption({ label: "Returns team" });
    await chooseDoor(page, "build");
    await composer(page).fill(description);
    await expect(sendButton(page)).toBeEnabled({ timeout: 20_000 });
    await sendButton(page).click();
    await creditsActionNextTo(page, noCredits.message);
  });

  async function startRun(page: Page) {
    await signIn(page);
    await workspacePicker(page).selectOption({ label: "Returns team" });
    await chooseDoor(page, "improve");
    await improveSource(page, "instructions");
    await startComposer(page, "improve").fill(instructions);
    await improvePrimary(page, "instructions").click();
    await page.getByRole("tab", { name: "Tests", exact: true }).click();
    await runTestsButton(page).click();
    await page.getByRole("dialog").getByRole("button", { name: "Start run", exact: true }).click();
  }

  test("on the Improve Start run: the error shows, the cost dialog closes and nothing retries", async ({ page }) => {
    const fake = await fakeVibe(page, { ...signedIn, failures: { "POST /messages:check": noCredits } });
    await startRun(page);
    await expect(page.getByRole("alert").filter({ hasText: noCredits.message })).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(runTestsButton(page)).toBeDisabled();
    await creditsActionNextTo(page, noCredits.message);
    await afterReconnect(fake);
    await writeRecord("h7-run", { alerts: await page.getByRole("alert").allInnerTexts(), writes: writes(fake) });
    expect(writes(fake).filter(write => !write.endsWith("/run-quote"))).toEqual(["POST /sessions", "POST /sessions/:id/messages", "POST /sessions/:id/messages"]);
    const quotes = fake.posts.filter(post => post.path.endsWith("/run-quote"));
    expect(quotes).toHaveLength(1);
    for (const quote of quotes) expect(quote.body).toMatchObject({ kind: "check", artifact_id: "v1", approve_artifact: true });
    expect(fake.errors).toEqual([]);
  });

  test("on the Improve Start run: a Credits action sits next to the error", async ({ page }) => {
    await fakeVibe(page, { ...signedIn, failures: { "POST /messages:check": noCredits } });
    await startRun(page);
    await creditsActionNextTo(page, noCredits.message);
  });
});

test("H8 guest capacity reached explains why before the disabled doors and keeps the sample readable", async ({ page }) => {
  const fake = await fakeVibe(page, { config: { availability: { guest: { available: false, reason_code: "trial_capacity_reached" }, workspace: { available: true } } } });
  await page.goto("/vibe-evals");
  const reason = entryNotice(page).getByRole("status");
  await expect(reason).toHaveText("Guest inference capacity is currently reached. Your saved work remains available.", { timeout: 20_000 });
  await doorsDisabled(page);
  expect(await reason.evaluate((node, door) => !!(node.compareDocumentPosition(door!) & Node.DOCUMENT_POSITION_FOLLOWING), await doorButton(page, "build").elementHandle())).toBe(true);
  await sampleButton(page).click();
  await expect(page.getByRole("heading", { name: "Sample: a return policy assistant" })).toBeVisible();
  expect(fake.posts).toEqual([]);
  expect(fake.errors).toEqual([]);
});

test("H9 a failed first commit keeps the Build draft and leaves no project", async ({ page }) => {
  const fake = await fakeVibe(page, { failures: { "POST /sessions": { status: 503, code: "unavailable", message: "Vibe could not complete this request. Try again shortly." } } });
  await page.goto("/vibe-evals");
  await entryReady(page, 30_000);
  await chooseDoor(page, "build");
  expect(fake.posts).toEqual([]);
  await composer(page).fill(description);
  await sendButton(page).click();
  await expect(page.getByRole("alert").filter({ hasText: "Vibe could not complete this request. Try again shortly." })).toBeVisible();
  await expect(sendButton(page)).toBeEnabled();
  await expect(composer(page)).toHaveValue(description);
  await expect(page.locator(".vibe-project-header")).toHaveCount(0);
  expect(new URL(page.url()).searchParams.get("session")).toBeNull();
  expect(writes(fake)).toEqual(["POST /sessions"]);
  expect(fake.errors).toEqual([]);
});

test("H10 a model error in the first Build run stays visible, and one retry runs while the next draft stays", async ({ page }) => {
  const fake = await fakeVibe(page, { buildFails: true });
  await page.goto("/vibe-evals");
  await entryReady(page, 30_000);
  await chooseDoor(page, "build");
  await doorReady(page, "build");
  await composer(page).fill(description);
  await expect(sendButton(page)).toBeEnabled({ timeout: 20_000 });
  await sendButton(page).click();
  await expect(page.getByRole("alert").filter({ hasText: "The selected model’s provider failed. Your request is saved." })).toBeVisible({ timeout: 20_000 });
  const retry = page.getByRole("button", { name: "Retry", exact: true });
  await expect(retry).toBeEnabled();
  await composer(page).fill("Also check opened items.");
  await retry.click();
  await expect(page.locator(".vibe-project-header")).toContainText("Run 1: Completed", { timeout: 30_000 });
  await expect(page.getByRole("tab", { name: "Results", exact: true })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("article", { name: "Run results", exact: true })).toBeVisible();
  await expect(composer(page)).toHaveValue("Also check opened items.");
  expect(writes(fake).filter(write => !write.endsWith("/build-quote"))).toEqual(["POST /sessions", "POST /sessions/:id/messages", expect.stringMatching(/^POST \/sessions\/:id\/operations\/.+\/retry$/)]);
  expect(fake.unknown).toEqual([]);
  expect(fake.errors).toEqual([]);
});

test.describe("H11 Back from an Improve source panel keeps the drafts", () => {
  test("saved instructions", async ({ page }) => {
    const fake = await fakeVibe(page);
    await page.goto("/vibe-evals");
    await entryReady(page, 30_000);
    await chooseDoor(page, "improve");
    await doorReady(page, "improve");
    await startComposer(page, "improve").fill("My unsent description.");
    await improveSource(page, "instructions");
    const box = startComposer(page, "improve");
    await box.fill(instructions);
    await page.getByRole("button", { name: "Back", exact: true }).click();
    await chooseDoor(page, "improve");
    await doorReady(page, "improve");
    await expect(composer(page)).toHaveValue(instructions);
    await improveSource(page, "instructions");
    await expect(box).toHaveValue(instructions);
    expect(writes(fake)).toEqual([]);
    expect(fake.errors).toEqual([]);
  });

  test("recorded replies", async ({ page }) => {
    const fake = await fakeVibe(page);
    await page.goto("/vibe-evals");
    await entryReady(page, 30_000);
    await chooseDoor(page, "improve");
    await doorReady(page, "improve");
    await startComposer(page, "improve").fill("My unsent description.");
    await improveSource(page, "recorded");
    await repliesInput(page).fill(transcript);
    await rulesInput(page).fill(expectation);
    await page.getByRole("button", { name: "Back", exact: true }).click();
    await chooseDoor(page, "improve");
    await doorReady(page, "improve");
    await sourceButton(page, "instructions").click();
    await expect(composer(page)).toHaveValue("My unsent description.");
    await improveSource(page, "recorded");
    await expect(repliesInput(page)).toHaveValue(transcript);
    await expect(rulesInput(page)).toHaveValue(expectation);
    expect(writes(fake)).toEqual([]);
    expect(fake.errors).toEqual([]);
  });
});

for (const door of ["build", "improve"] as const) test(`a committed ${door} first message survives a lost acknowledgement without a second operation`, async ({ page }) => {
  const fake = await fakeVibe(page, { loseFirstMessageAck: true });
  await page.goto("/vibe-evals"); await entryReady(page); await chooseDoor(page, door); await doorReady(page, door);
  await startComposer(page, door).fill(door === "build" ? description : instructions);
  await (door === "build" ? sendButton(page) : improvePrimary(page, "instructions")).click();
  await expect(page.locator(".vibe-project-header")).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry submission", exact: true })).toHaveCount(0);
  if (door === "build") await expect(page.locator(".vibe-project-header")).toContainText("Run 1: Completed", { timeout: 30_000 });
  else await expect(page.getByRole("tab", { name: "Tests", exact: true })).toHaveAttribute("aria-selected", "true");
  expect(fake.posts.filter(post => post.method === "DELETE")).toEqual([]);
  expect(fake.posts.filter(post => post.path === "/sessions")).toHaveLength(1);
  expect(fake.posts.filter(post => post.path.endsWith("/messages") && post.body.kind === "message")).toHaveLength(1);
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});


for (const kind of ["malformed", "oversized"] as const) test(`fix8 Settings ${kind} import keeps the signed-in Improve start without a project`, async ({ page }) => {
  const failure = { status: 400, code: "invalid_pack", message: "This test pack is not valid JSON.", admission: "rejected" as const };
  const fake = await fakeVibe(page, { ...signedIn, failures: { "POST /import": failure } });
  await signIn(page);
  await workspacePicker(page).selectOption({ label: "Returns team" });
  await chooseDoor(page, "improve");
  await doorReady(page, "improve");
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Import a test pack", exact: true }).click();
  await page.getByLabel("Test pack file", { exact: true }).setInputFiles({ name: "tests.json", mimeType: "application/json", buffer: Buffer.from(kind === "malformed" ? "{broken" : "x".repeat(1024 * 1024 + 1)) });
  await expect(page.locator('.vibe-improve-start [role="alert"]')).toHaveText(kind === "malformed" ? failure.message : "This file exceeds the import size limit.");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.locator(".vibe-project-header")).toHaveCount(0);
  expect(new URL(page.url()).searchParams.get("session")).toBeNull();
  expect(fake.sessions.size).toBe(0);
  expect(writes(fake)).toEqual(kind === "malformed" ? ["POST /sessions", "POST /sessions/:id/import", "DELETE /sessions/:id"] : []);
  expect(fake.errors).toEqual([]);
  expect(fake.unknown).toEqual([]);
});


for (const limit of [65536, 32768]) test(`fix9 instructions reserve the test request within the ${limit} byte limit`, async ({ page }) => {
  const fake = await fakeVibe(page, { ...signedIn, config: { ...signedIn.config, signed_in_limits: { max_message_bytes: limit } } });
  await signIn(page);
  await workspacePicker(page).selectOption({ label: "Returns team" });
  await chooseDoor(page, "improve"); await doorReady(page, "improve");
  const room = limit - 71;
  const largest = "é".repeat(Math.floor(room / 2)) + "x";
  await startComposer(page, "improve").fill(largest + "x");
  await improvePrimary(page, "instructions").click();
  await expect(page.locator('.vibe-improve-start [role="alert"]')).toHaveText(`Agent instructions can use up to ${room.toLocaleString()} UTF-8 bytes. The test request uses 71 of the ${limit.toLocaleString()} byte limit.`);
  expect(writes(fake)).toEqual([]);
  await expect(page.locator(".vibe-project-header")).toHaveCount(0);
  await startComposer(page, "improve").fill(largest);
  await improvePrimary(page, "instructions").click();
  await expect(page.getByRole("tab", { name: "Tests", exact: true })).toHaveAttribute("aria-selected", "true");
  const messages = fake.posts.filter(post => post.path.endsWith("/messages"));
  expect(messages).toHaveLength(1);
  expect(Buffer.byteLength(messages[0].body.content, "utf8")).toBe(limit);
  expect(fake.sessions.size).toBe(1);
  expect(fake.errors).toEqual([]); expect(fake.unknown).toEqual([]);
});

test("fix9 large pending reply drafts survive reloads and resolve one at a time", async ({ page }) => {
  const fake = await fakeVibe(page, signedIn);
  await signIn(page);
  await workspacePicker(page).selectOption({ label: "Returns team" });
  await chooseDoor(page, "improve"); await doorReady(page, "improve");
  await improveSource(page, "recorded");
  await repliesInput(page).fill("Existing account replies");
  await rulesInput(page).fill("Keep the account rule");
  for (let index = 0; index < 5; index++) {
    await page.evaluate(index => {
      const project = "start:guest:test";
      const value = JSON.stringify({ mode: "replies", conversations: `${index}:` + "x".repeat(240 * 1024), rules: `Guest rule ${index}` });
      sessionStorage.setItem(`vibe-build-drafts:${project}`, JSON.stringify({ version: 2, composers: {}, editors: { [`${project}:start:editor:inputs`]: { artifact: "start", source: "", value } } }));
    }, index);
    await page.reload(); await doorReady(page, "improve");
    await expect(repliesInput(page)).toHaveValue("Existing account replies");
    await expect(page.getByRole("button", { name: "Use it", exact: true })).toBeVisible();
  }
  await page.reload(); await doorReady(page, "improve");
  let current = "Existing account replies", rule = "Keep the account rule";
  for (let index = 0; index < 5; index++) {
    await page.getByRole("button", { name: index % 2 ? "Discard" : "Use it", exact: true }).click();
    if (!(index % 2)) { current = `${index}:` + "x".repeat(240 * 1024); rule = `Guest rule ${index}`; }
    await expect(repliesInput(page)).toHaveValue(current);
    await expect(rulesInput(page)).toHaveValue(rule);
    await page.reload(); await doorReady(page, "improve");
    await expect(repliesInput(page)).toHaveValue(current);
    await expect(page.getByRole("button", { name: "Use it", exact: true })).toHaveCount(index < 4 ? 1 : 0);
  }
  expect(writes(fake)).toEqual([]);
  expect(fake.errors).toEqual([]); expect(fake.unknown).toEqual([]);
});


for (const mode of ["recorded", "import"] as const) test(`fix11 reload restores the ${mode} segment without hydration errors`, async ({ page }) => {
  const fake = await fakeVibe(page);
  await page.goto("/vibe-evals?start=improve"); await doorReady(page, "improve");
  await improveSource(page, mode);
  if (mode === "recorded") {
    await repliesInput(page).fill("Customer: Can I return it?\nAgent: Yes, within 30 days.");
    await rulesInput(page).fill("Allow returns within 30 days.");
  }
  await page.reload(); await doorReady(page, "improve");
  await expect(sourceButton(page, mode)).toHaveAttribute("aria-checked", "true");
  if (mode === "recorded") {
    await expect(repliesInput(page)).toHaveValue("Customer: Can I return it?\nAgent: Yes, within 30 days.");
    await expect(rulesInput(page)).toHaveValue("Allow returns within 30 days.");
  } else await expect(improvePrimary(page, mode)).toBeVisible();
  expect(writes(fake)).toEqual([]);
  expect(fake.errors).toEqual([]); expect(fake.unknown).toEqual([]);
});
