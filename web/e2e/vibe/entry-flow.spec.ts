// Flow economy of each door path, from the entry to the first test result (M2), with a
// stateful fake (no API, no model). Budgets equal the measured counts: they stop
// regressions. Each test writes flow-<path>.json (actions, milestones, decisions, writes,
// dialogs) to its output folder. See ~/.claude/plans/vibe-evals-redesign/verification.md, A, C and D.
// PR 1: Tests and Results open by themselves when the tests or the result are ready (A7), so
// no path clicks a tab to see its first result.
import { expect, test, type Page } from "@playwright/test";
import { agentReply, description, expectation, fakeVibe, instructions, packFile, transcript, type Fake } from "./entry-fake";
import { flowMeter, type FlowMeter } from "./flow-meter";
import { composer, doorButton, doorReady, entryReady, improvePrimary, repliesInput, rulesInput, sendButton, sourceButton, startComposer, switchDoorButton } from "./entry-model";

test.use({ viewport: { width: 1440, height: 960 }, reducedMotion: "reduce" });
test.describe.configure({ timeout: 120_000 });

type Budget = { clicks: number; texts: number; files: number; decisions: number };
// Enabled main-zone controls in the first viewport at each decision point, measured (1440, light).
// "imported tests" is 13: the import writes Guide's two messages, as the backend does, so the Guide
// column opens with them and its controls count in the main zone (12 before PR 1, with an empty Guide).
const controlBudget: Record<string, number> = { entry: 3, "build start": 4, "improve start": 8, "imported tests": 13, cost: 4 };
const tab = (page: Page, name: "Agent" | "Tests" | "Results") => page.getByRole("tab", { name, exact: true });
const runResults = (page: Page) => page.getByRole("article", { name: "Run results" });
const runDone = (page: Page) => expect(page.locator(".vibe-project-header")).toContainText("Run 1: Completed", { timeout: 30_000 });
const startRun = (page: Page) => page.getByRole("dialog").getByRole("button", { name: "Start run", exact: true });
// The result comes to the user: Results opens by itself when the run ends.
async function resultShown(page: Page) {
  await expect(tab(page, "Results")).toHaveAttribute("aria-selected", "true");
  await expect(runResults(page)).toBeVisible();
}

async function open(page: Page, name: string, options: { keyboardOnly?: boolean } = {}) {
  const fake = await fakeVibe(page);
  const flow = await flowMeter(page, name, options);
  flow.watch("M1", agentReply(page));
  await page.goto("/vibe-evals");
  await entryReady(page, 30_000);
  return { fake, flow };
}

// The agent reply in the failed test is the first agent answer (M1). Open the test only when it is closed.
async function firstAnswer(page: Page, flow: FlowMeter) {
  const failed = runResults(page).locator("details").filter({ has: page.locator("summary", { hasText: "Issue found" }) }).first();
  if (!(await failed.evaluate(el => (el as HTMLDetailsElement).open))) await flow.click(failed.locator("summary").first(), "open failed test");
  await expect(agentReply(page)).toBeVisible();
  await flow.mark("M1");
}

async function finish(page: Page, fake: Fake, flow: FlowMeter, budget: Budget) {
  const unavailable = await page.getByRole("alert").filter({ hasText: "selected result is unavailable" }).count();
  const record = await flow.save({ selectedResultUnavailable: unavailable });
  const m2 = record.milestones.M2;
  expect(m2, "M2 reached").toBeTruthy();
  expect(m2.clicks, "clicks to the first test result").toBeLessThanOrEqual(budget.clicks);
  expect(m2.texts, "text entries to the first test result").toBeLessThanOrEqual(budget.texts);
  expect(m2.files, "file choices to the first test result").toBeLessThanOrEqual(budget.files);
  expect(record.decisions.filter(d => d.options >= 2).length, "decision points").toBeLessThanOrEqual(budget.decisions);
  for (const d of record.decisions) {
    expect(controlBudget[d.at], `a control budget for "${d.at}"`).toBeDefined();
    expect(d.mainEnabled, `enabled main controls at "${d.at}"`).toBeLessThanOrEqual(controlBudget[d.at]);
  }
  expect(record.mostDialogs, "dialogs open at the same time").toBeLessThanOrEqual(1);
  // A8 (fixed in PR 1): a run with no error shows no "selected result is unavailable" alert.
  expect(unavailable, "A8: unavailable-result alerts").toBe(0);
  // A5: inference waits for consent; each path creates one project on its committed input.
  expect(record.writesBeforeConsent.filter(w => w.path.endsWith("/messages"))).toEqual([]);
  expect(record.writesBeforeConsent.filter(w => w.path === "/sessions").length).toBeLessThanOrEqual(1);
  expect(fake.posts.filter(post => post.path === "/sessions")).toHaveLength(1);
  expect(fake.unknown).toEqual([]);
  expect(fake.errors).toEqual([]);
}

test("Build reaches its first test result in 2 clicks and 1 text", async ({ page }) => {
  const { fake, flow } = await open(page, "build");
  await flow.decision("entry", [doorButton(page, "build"), doorButton(page, "improve")]);
  await flow.click(doorButton(page, "build"), "Build door");
  await doorReady(page, "build");
  expect(fake.posts).toEqual([]);
  await flow.fill(composer(page), description, "task");
  await expect(sendButton(page)).toBeEnabled({ timeout: 20_000 });
  // The minimal Build start offers submission or the other door; Back and the input make four controls.
  await flow.decision("build start", [sendButton(page), switchDoorButton(page, "build")]);
  expect(fake.posts).toEqual([]);
  flow.consent();
  await flow.click(sendButton(page), "Build and run 3 tests");
  await runDone(page);
  await resultShown(page);
  await flow.mark("M2");
  await firstAnswer(page, flow);
  await finish(page, fake, flow, { clicks: 2, texts: 1, files: 0, decisions: 2 });
});

// The saved-instructions path to the first result. The tests and the result open by themselves.
async function instructionsToResult(page: Page, flow: FlowMeter, fake: Fake) {
  await flow.decision("entry", [doorButton(page, "build"), doorButton(page, "improve")]);
  await flow.click(doorButton(page, "improve"), "Improve door");
  await doorReady(page, "improve");
  expect(fake.posts).toEqual([]);
  // The forward options of the Improve start, as in verification.md section 2: three sources and describe.
  await flow.decision("improve start", [sourceButton(page, "import"), sourceButton(page, "recorded"), sourceButton(page, "instructions"), composer(page)]);
  await flow.click(sourceButton(page, "instructions"), "instructions source");
  await flow.fill(startComposer(page, "improve"), instructions, "instructions");
  expect(fake.posts).toEqual([]);
  flow.consent();
  await flow.click(improvePrimary(page, "instructions"), "Prepare tests");
  await expect(page.getByText("These are example messages I’ll send to your agent.").first()).toBeVisible({ timeout: 30_000 });
  await expect(tab(page, "Tests")).toHaveAttribute("aria-selected", "true");
  await flow.click(page.getByRole("button", { name: "Run 3 tests", exact: true }), "Run 3 tests");
  await flow.decision("cost", [startRun(page)]);
  await flow.click(startRun(page), "Start run");
  await runDone(page);
  await resultShown(page);
  await flow.mark("M2");
}

test("Improve with saved instructions reaches its first test result in 5 clicks and 1 text", async ({ page }) => {
  const { fake, flow } = await open(page, "improve-instructions");
  await instructionsToResult(page, flow, fake);
  await firstAnswer(page, flow);
  await finish(page, fake, flow, { clicks: 5, texts: 1, files: 0, decisions: 2 });
});

// A1f: from the first failed result to the compared result of Guide's fix. The fix is selected
// by itself and shows with its rerun on the Agent view; the cost dialog stays.
test("Improve fix loop reaches the compared result in 3 clicks", async ({ page }) => {
  const { fake, flow } = await open(page, "improve-fix");
  await instructionsToResult(page, flow, fake);
  const start = { ...(await flow.mark("M2")) };
  await flow.click(runResults(page).getByRole("button", { name: "Help me fix this", exact: true }), "Help me fix this");
  const rerun = page.getByRole("button", { name: "Improve and rerun", exact: true });
  await expect(rerun).toBeVisible({ timeout: 30_000 });
  await expect(tab(page, "Agent")).toHaveAttribute("aria-selected", "true");
  await expect(page.getByLabel("Instruction changes", { exact: true })).toContainText("Decline every return after 30 days.");
  await expect(page.getByRole("button", { name: "Review latest version", exact: true })).toHaveCount(0);
  const runs = () => fake.posts.filter(post => post.path.endsWith("/messages") && ["check", "retest"].includes(post.body.kind));
  // Nothing runs before the cost dialog: only the first run so far.
  expect(runs()).toHaveLength(1);
  await flow.click(rerun, "Improve and rerun");
  await flow.decision("cost", [startRun(page)]);
  await flow.click(startRun(page), "Start run");
  await expect(page.locator(".vibe-project-header")).toContainText("Run 2: Completed", { timeout: 30_000 });
  await resultShown(page);
  await expect(runResults(page)).toContainText("3 passed · 0 failed");
  const end = await flow.mark("F1");
  const record = await flow.save();
  expect(end.clicks - start.clicks, "A1f: clicks in the fix loop").toBeLessThanOrEqual(3);
  expect(runs().map(post => [post.body.kind, post.body.artifact_id])).toEqual([["check", "v1"], ["retest", expect.stringMatching(/-fix$/)]]);
  expect(record.mostDialogs).toBeLessThanOrEqual(1);
  expect(await page.getByRole("alert").filter({ hasText: "selected result is unavailable" }).count()).toBe(0);
  expect(fake.unknown).toEqual([]);
  expect(fake.errors).toEqual([]);
});

test("Improve with recorded replies reaches its first test result in 5 clicks and 2 texts", async ({ page }) => {
  const { fake, flow } = await open(page, "improve-recorded");
  await flow.decision("entry", [doorButton(page, "build"), doorButton(page, "improve")]);
  await flow.click(doorButton(page, "improve"), "Improve door");
  await doorReady(page, "improve");
  expect(fake.posts).toEqual([]);
  await flow.decision("improve start", [sourceButton(page, "import"), sourceButton(page, "recorded"), sourceButton(page, "instructions"), composer(page)]);
  await flow.click(sourceButton(page, "recorded"), "recorded source");
  await flow.fill(repliesInput(page), transcript, "recorded replies");
  await flow.fill(rulesInput(page), expectation, "expected behavior");
  expect(fake.posts).toEqual([]);
  await flow.click(improvePrimary(page, "recorded"), "Review recorded replies");
  await expect(page.getByRole("region", { name: "Recorded reply tests" })).toBeVisible({ timeout: 30_000 });
  await flow.click(page.getByRole("button", { name: "Run recorded tests", exact: true }), "Run recorded tests");
  await flow.decision("cost", [startRun(page)]);
  flow.consent();
  await flow.click(startRun(page), "Start run");
  await runDone(page);
  await resultShown(page);
  await flow.mark("M2");
  // Recorded replies come from the user's app, so this path has no model-written answer (no M1).
  await finish(page, fake, flow, { clicks: 5, texts: 2, files: 0, decisions: 2 });
});

test("Improve with a test pack import reaches its first test result in 7 clicks, 1 file and 1 text", async ({ page }) => {
  const { fake, flow } = await open(page, "improve-import");
  await flow.decision("entry", [doorButton(page, "build"), doorButton(page, "improve")]);
  await flow.click(doorButton(page, "improve"), "Improve door");
  await doorReady(page, "improve");
  expect(fake.posts).toEqual([]);
  await flow.decision("improve start", [sourceButton(page, "import"), sourceButton(page, "recorded"), sourceButton(page, "instructions"), composer(page)]);
  await flow.chooseFile(sourceButton(page, "import"), packFile, "Import a test pack");
  const add = page.getByRole("button", { name: "Add your agent", exact: true });
  await expect(add).toBeVisible({ timeout: 30_000 });
  await flow.decision("imported tests", [add, page.getByRole("button", { name: "Create an agent", exact: true }), page.getByRole("button", { name: "Keep for later", exact: true })]);
  await flow.click(add, "Add your agent");
  await flow.fill(page.getByRole("textbox", { name: "Agent instructions", exact: true }), instructions, "agent instructions");
  await flow.click(page.getByRole("button", { name: "Use these instructions", exact: true }), "Use these instructions");
  await expect(tab(page, "Agent")).toHaveAttribute("aria-selected", "true");
  await flow.click(tab(page, "Tests"), "Tests tab");
  await flow.click(page.getByRole("button", { name: "Run 3 tests", exact: true }), "Run 3 tests");
  await flow.decision("cost", [startRun(page)]);
  flow.consent();
  await flow.click(startRun(page), "Start run");
  await runDone(page);
  await resultShown(page);
  await flow.mark("M2");
  await firstAnswer(page, flow);
  await finish(page, fake, flow, { clicks: 7, texts: 1, files: 1, decisions: 3 });
});

// D1: each door reaches its first submission with the keyboard only (the meter rejects pointer actions).
test.describe("keyboard only", () => {
  const submitted = (fake: Fake, path: string) => expect.poll(() => fake.posts.filter(p => p.path.endsWith(path)).length).toBe(1);

  test("Build sends the task", async ({ page }) => {
    const { fake, flow } = await open(page, "build-keyboard", { keyboardOnly: true });
    await flow.tabTo(doorButton(page, "build"), "Build door");
    await flow.press("Enter", "Build door");
    await doorReady(page, "build");
    expect(fake.posts).toEqual([]);
    await flow.tabTo(composer(page), "composer");
    await flow.fill(composer(page), description, "task");
    await expect(sendButton(page)).toBeEnabled({ timeout: 20_000 });
    expect(fake.posts).toEqual([]);
    flow.consent();
    await flow.press("Enter", "send");
    await submitted(fake, "/messages");
    await flow.save();
  });

  test("Improve sends saved instructions", async ({ page }) => {
    const { fake, flow } = await open(page, "improve-instructions-keyboard", { keyboardOnly: true });
    await flow.tabTo(doorButton(page, "improve"), "Improve door");
    await flow.press("Enter", "Improve door");
    await doorReady(page, "improve");
    expect(fake.posts).toEqual([]);
    await flow.tabTo(sourceButton(page, "instructions"), "instructions source");
    await flow.press("Enter", "instructions source");
    const box = startComposer(page, "improve");
    await flow.tabTo(box, "instructions");
    await flow.fill(box, instructions, "instructions");
    await flow.tabTo(improvePrimary(page, "instructions"), "Prepare tests");
    expect(fake.posts).toEqual([]);
    flow.consent();
    await flow.press("Enter", "Prepare tests");
    await submitted(fake, "/messages");
    await flow.save();
  });

  test("Improve saves recorded replies", async ({ page }) => {
    const { fake, flow } = await open(page, "improve-recorded-keyboard", { keyboardOnly: true });
    await flow.tabTo(doorButton(page, "improve"), "Improve door");
    await flow.press("Enter", "Improve door");
    await doorReady(page, "improve");
    expect(fake.posts).toEqual([]);
    await flow.tabTo(sourceButton(page, "recorded"), "recorded source");
    await flow.press("Enter", "recorded source");
    for (const [input, value, label] of [[repliesInput(page), transcript, "recorded replies"], [rulesInput(page), expectation, "expected behavior"]] as const) {
      await flow.tabTo(input, label);
      await flow.fill(input, value, label);
    }
    await flow.tabTo(improvePrimary(page, "recorded"), "Review recorded replies");
    expect(fake.posts).toEqual([]);
    await flow.press("Enter", "Review recorded replies");
    await submitted(fake, "/evidence");
    await flow.save();
  });

  test("Improve imports a test pack", async ({ page }) => {
    const { fake, flow } = await open(page, "improve-import-keyboard", { keyboardOnly: true });
    await flow.tabTo(doorButton(page, "improve"), "Improve door");
    await flow.press("Enter", "Improve door");
    await doorReady(page, "improve");
    expect(fake.posts).toEqual([]);
    await flow.tabTo(sourceButton(page, "import"), "Import a test pack");
    await flow.chooseFile(sourceButton(page, "import"), packFile, "Import a test pack");
    await submitted(fake, "/import");
    await flow.save();
  });
});

test("door navigation keeps both drafts and makes no project before input is committed", async ({ page }) => {
  const { fake } = await open(page, "uncommitted-doors");
  await doorButton(page, "build").click(); await doorReady(page, "build");
  await composer(page).fill(description);
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(doorButton(page, "build")).toBeFocused();
  await expect(page.locator("h1")).toHaveCount(1);
  await expect(page.locator(".vibe-entry-intro")).not.toHaveAttribute("data-fresh");
  await doorButton(page, "improve").click(); await doorReady(page, "improve");
  await startComposer(page, "improve").fill(instructions);
  await switchDoorButton(page, "improve").click();
  await doorReady(page, "build"); await expect(composer(page)).toHaveValue(description);
  await page.reload(); await doorReady(page, "build"); await expect(composer(page)).toHaveValue(description);
  await expect(page.locator("h1")).toHaveCount(1);
  expect(fake.posts).toEqual([]);
  expect(fake.errors).toEqual([]); expect(fake.unknown).toEqual([]);
});


for (const door of ["build", "improve"] as const) test(`${door} start follows browser history and reload without creating a project`, async ({ page }) => {
  const { fake } = await open(page, `${door}-history`);
  await doorButton(page, door).click(); await doorReady(page, door);
  expect(fake.posts).toEqual([]);
  await page.goBack(); await entryReady(page);
  await expect(doorButton(page, door)).toBeFocused();
  await page.goForward(); await doorReady(page, door);
  await page.reload(); await doorReady(page, door);
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await entryReady(page); await expect(doorButton(page, door)).toBeFocused();
  expect(fake.posts).toEqual([]);
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});

for (const door of ["build", "improve"] as const) test(`New project returns from the ${door} start without losing its draft`, async ({ page }) => {
  const fake = await fakeVibe(page);
  await page.goto("/vibe-evals"); await entryReady(page); await doorButton(page, door).click(); await doorReady(page, door);
  const input = startComposer(page, door);
  await input.fill("Keep this uncommitted draft.");
  await expect(page.locator('.vibe-entry-topbar').getByRole("link", { name: "Sign in", exact: true })).toHaveAttribute("href", `/auth/login?returnTo=${encodeURIComponent(`/vibe-evals?start=${door}`)}`);
  await page.getByRole("button", { name: "Open projects", exact: true }).click();
  await page.getByRole("dialog", { name: "Your projects" }).getByRole("button", { name: "New project", exact: true }).click();
  await entryReady(page); await expect(page.getByRole("dialog")).toHaveCount(0);
  await doorButton(page, door).click(); await doorReady(page, door); await expect(input).toHaveValue("Keep this uncommitted draft.");
  expect(fake.posts).toEqual([]); expect(fake.errors).toEqual([]);
});
