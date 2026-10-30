// Failure branches of the entry and both doors (verification.md section 6 H), with the stateful
// fake (no API, no model). Where today's behaviour falls short of the target, the target is a
// test.fail that errors once it is met.
import { expect, test, type Page } from "@playwright/test";
import { description, fakeVibe, instructions, transcript, expectation, type FakeOptions } from "./entry-fake";
import { writeRecord } from "./flow-meter";
import { chooseDoor, composer, doorButton, doorReady, entryNotice, entryReady, improveSource, sendButton, workspacePicker , runTestsButton } from "./entry-model";

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

test("H3 signed in with no workspace shows one message and one action", async ({ page }) => {
  const fake = await fakeVibe(page, { ...signedIn, me: { user_id: "a8000000-0000-4000-8000-000000000001", organizations: [] } });
  await signIn(page);
  await expect(entryNotice(page).getByRole("status")).toHaveText(/^You need a workspace to start a project\./, { timeout: 20_000 });
  await expect(entryNotice(page).locator("a, button")).toHaveCount(1);
  await expect(entryNotice(page).getByRole("link", { name: "Open your dashboard", exact: true })).toHaveAttribute("href", "/dashboard");
  await expect(workspacePicker(page)).toHaveCount(0);
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
    await afterReconnect(fake);
    await writeRecord("h7-build", { alerts: await page.getByRole("alert").allInnerTexts(), writes: writes(fake) });
    expect(writes(fake).filter(write => !write.endsWith("/build-quote"))).toEqual(["POST /sessions", "POST /sessions/:id/messages"]);
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
    await page.getByRole("textbox", { name: "Agent instructions to test" }).fill(instructions);
    await page.getByRole("button", { name: "Prepare tests" }).click();
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
  await page.locator("summary").filter({ hasText: "Read a sample test" }).click();
  await expect(page.getByRole("heading", { name: "Sample: a return policy assistant" })).toBeVisible();
  expect(fake.posts).toEqual([]);
  expect(fake.errors).toEqual([]);
});

test("H9 a failed project creation at the door shows the error and keeps both doors usable", async ({ page }) => {
  const fake = await fakeVibe(page, { failures: { "POST /sessions": { status: 503, code: "unavailable", message: "Vibe could not complete this request. Try again shortly." } } });
  await page.goto("/vibe-evals");
  await entryReady(page, 30_000);
  await chooseDoor(page, "build");
  await expect(page.getByRole("alert").filter({ hasText: "Vibe could not complete this request. Try again shortly." })).toBeVisible();
  for (const door of ["build", "improve"] as const) await expect(doorButton(page, door)).toBeEnabled();
  await expect(page.getByText("No projects yet.", { exact: true })).toBeVisible();
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
    await composer(page).fill("My unsent description.");
    await improveSource(page, "instructions");
    const box = page.getByRole("textbox", { name: "Agent instructions to test" });
    await box.fill(instructions);
    await page.getByRole("button", { name: "Back to conversation", exact: true }).click();
    await doorReady(page, "improve");
    await expect(composer(page)).toHaveValue("My unsent description.");
    await improveSource(page, "instructions");
    await expect(box).toHaveValue(instructions);
    expect(writes(fake)).toEqual(["POST /sessions"]);
    expect(fake.errors).toEqual([]);
  });

  test("recorded replies", async ({ page }) => {
    const fake = await fakeVibe(page);
    await page.goto("/vibe-evals");
    await entryReady(page, 30_000);
    await chooseDoor(page, "improve");
    await doorReady(page, "improve");
    await composer(page).fill("My unsent description.");
    await improveSource(page, "recorded");
    await page.getByLabel("Recorded replies", { exact: true }).fill(transcript);
    await page.getByLabel("Expected behavior", { exact: true }).fill(expectation);
    await page.getByRole("form", { name: "Recorded reply intake" }).getByRole("button", { name: "Back", exact: true }).click();
    await doorReady(page, "improve");
    await expect(composer(page)).toHaveValue("My unsent description.");
    await improveSource(page, "recorded");
    await expect(page.getByLabel("Recorded replies", { exact: true })).toHaveValue(transcript);
    await expect(page.getByLabel("Expected behavior", { exact: true })).toHaveValue(expectation);
    expect(writes(fake)).toEqual(["POST /sessions"]);
    expect(fake.errors).toEqual([]);
  });
});
