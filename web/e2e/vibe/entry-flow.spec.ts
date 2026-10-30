// Flow economy of each door path, from the entry to the first test result (M2), with a
// stateful fake (no API, no model). Budgets equal today's baseline (d2248411): they stop
// regressions. Each test writes flow-<path>.json (actions, milestones, decisions, writes,
// dialogs) to its output folder. See ~/.claude/plans/vibe-evals-redesign/verification.md, A, C and D.
import { expect, test, type Page } from "@playwright/test";
import { agentReply, description, expectation, fakeVibe, instructions, packFile, transcript, type Fake } from "./entry-fake";
import { flowMeter, type FlowMeter } from "./flow-meter";
import { composer, doorButton, doorReady, entryReady, sendButton, sourceButton } from "./entry-model";

test.use({ viewport: { width: 1440, height: 960 }, reducedMotion: "reduce" });
test.describe.configure({ timeout: 120_000 });

type Budget = { clicks: number; texts: number; files: number; decisions: number };
// Enabled main-zone controls in the first viewport at each decision point, measured today (1440, light).
const controlBudget: Record<string, number> = { entry: 3, "build start": 7, "improve start": 8, "imported tests": 12, cost: 4 };
const tab = (page: Page, name: "Agent" | "Tests" | "Results") => page.getByRole("tab", { name, exact: true });
const runResults = (page: Page) => page.getByRole("article", { name: "Run results" });
const runDone = (page: Page) => expect(page.locator(".vibe-project-header")).toContainText("Run 1: Completed", { timeout: 30_000 });
const startRun = (page: Page) => page.getByRole("dialog").getByRole("button", { name: "Start run", exact: true });

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
  const record = await flow.save({ selectedResultUnavailable: await page.getByRole("alert").filter({ hasText: "selected result is unavailable" }).count() });
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
  // A5: nothing model-bound before the explicit consent, and one project per door choice.
  expect(record.writesBeforeConsent.filter(w => w.path.endsWith("/messages"))).toEqual([]);
  expect(record.writesBeforeConsent.filter(w => w.path === "/sessions").length).toBeLessThanOrEqual(1);
  expect(fake.unknown).toEqual([]);
  expect(fake.errors).toEqual([]);
}

test("Build reaches its first test result in 3 clicks and 1 text", async ({ page }) => {
  const { fake, flow } = await open(page, "build");
  await flow.decision("entry", [doorButton(page, "build"), doorButton(page, "improve")]);
  await flow.click(doorButton(page, "build"), "Build door");
  await doorReady(page, "build");
  await flow.fill(composer(page), description, "task");
  await expect(sendButton(page)).toBeEnabled({ timeout: 20_000 });
  // The forward options of the Build start, as in verification.md section 2: describe, example, attach, switch door.
  await flow.decision("build start", [sendButton(page), page.getByRole("button", { name: "See an example" }), page.getByRole("button", { name: "Add a file or text" }), page.getByRole("button", { name: "Already have an agent? Improve it instead." })]);
  flow.consent();
  await flow.click(sendButton(page), "Build and run 3 tests");
  await runDone(page);
  await flow.click(tab(page, "Results"), "Results tab");
  await expect(runResults(page)).toBeVisible();
  await flow.mark("M2");
  await firstAnswer(page, flow);
  await finish(page, fake, flow, { clicks: 3, texts: 1, files: 0, decisions: 2 });
});

test("Improve with saved instructions reaches its first test result in 7 clicks and 1 text", async ({ page }) => {
  const { fake, flow } = await open(page, "improve-instructions");
  await flow.decision("entry", [doorButton(page, "build"), doorButton(page, "improve")]);
  await flow.click(doorButton(page, "improve"), "Improve door");
  await doorReady(page, "improve");
  // The forward options of the Improve start, as in verification.md section 2: three sources and describe.
  await flow.decision("improve start", [sourceButton(page, "import"), sourceButton(page, "recorded"), sourceButton(page, "instructions"), composer(page)]);
  await flow.click(sourceButton(page, "instructions"), "instructions source");
  await flow.fill(page.getByRole("textbox", { name: "Agent instructions to test" }), instructions, "instructions");
  flow.consent();
  await flow.click(page.getByRole("button", { name: "Prepare tests" }), "Prepare tests");
  await expect(page.getByText("These are example messages I’ll send to your agent.").first()).toBeVisible({ timeout: 30_000 });
  await flow.click(tab(page, "Tests"), "Tests tab");
  await flow.click(page.getByRole("button", { name: "Run 3 tests", exact: true }), "Run 3 tests");
  await flow.decision("cost", [startRun(page)]);
  await flow.click(startRun(page), "Start run");
  await runDone(page);
  await flow.click(tab(page, "Results"), "Results tab");
  await expect(runResults(page)).toBeVisible();
  await flow.mark("M2");
  await firstAnswer(page, flow);
  await finish(page, fake, flow, { clicks: 7, texts: 1, files: 0, decisions: 2 });
});

test("Improve with recorded replies reaches its first test result in 6 clicks and 2 texts", async ({ page }) => {
  const { fake, flow } = await open(page, "improve-recorded");
  await flow.decision("entry", [doorButton(page, "build"), doorButton(page, "improve")]);
  await flow.click(doorButton(page, "improve"), "Improve door");
  await doorReady(page, "improve");
  await flow.decision("improve start", [sourceButton(page, "import"), sourceButton(page, "recorded"), sourceButton(page, "instructions"), composer(page)]);
  await flow.click(sourceButton(page, "recorded"), "recorded source");
  await flow.fill(page.getByLabel("Recorded replies", { exact: true }), transcript, "recorded replies");
  await flow.fill(page.getByLabel("Expected behavior", { exact: true }), expectation, "expected behavior");
  await flow.click(page.getByRole("button", { name: "Review recorded replies", exact: true }), "Review recorded replies");
  await expect(page.getByRole("region", { name: "Recorded reply tests" })).toBeVisible({ timeout: 30_000 });
  await flow.click(page.getByRole("button", { name: "Run recorded tests", exact: true }), "Run recorded tests");
  await flow.decision("cost", [startRun(page)]);
  flow.consent();
  await flow.click(startRun(page), "Start run");
  await runDone(page);
  await flow.click(tab(page, "Results"), "Results tab");
  await expect(runResults(page)).toBeVisible();
  await flow.mark("M2");
  // Recorded replies come from the user's app, so this path has no model-written answer (no M1).
  await finish(page, fake, flow, { clicks: 6, texts: 2, files: 0, decisions: 2 });
});

test("Improve with a test pack import reaches its first test result in 8 clicks, 1 file and 1 text", async ({ page }) => {
  const { fake, flow } = await open(page, "improve-import");
  await flow.decision("entry", [doorButton(page, "build"), doorButton(page, "improve")]);
  await flow.click(doorButton(page, "improve"), "Improve door");
  await doorReady(page, "improve");
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
  await flow.click(tab(page, "Results"), "Results tab");
  await expect(runResults(page)).toBeVisible();
  await flow.mark("M2");
  await firstAnswer(page, flow);
  await finish(page, fake, flow, { clicks: 8, texts: 1, files: 1, decisions: 3 });
});

// D1: each door reaches its first submission with the keyboard only (the meter rejects pointer actions).
test.describe("keyboard only", () => {
  const submitted = (fake: Fake, path: string) => expect.poll(() => fake.posts.filter(p => p.path.endsWith(path)).length).toBe(1);

  test("Build sends the task", async ({ page }) => {
    const { fake, flow } = await open(page, "build-keyboard", { keyboardOnly: true });
    await flow.tabTo(doorButton(page, "build"), "Build door");
    await flow.press("Enter", "Build door");
    await doorReady(page, "build");
    await flow.tabTo(composer(page), "composer");
    await flow.fill(composer(page), description, "task");
    await expect(sendButton(page)).toBeEnabled({ timeout: 20_000 });
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
    await flow.tabTo(sourceButton(page, "instructions"), "instructions source");
    await flow.press("Enter", "instructions source");
    const box = page.getByRole("textbox", { name: "Agent instructions to test" });
    await flow.tabTo(box, "instructions");
    await flow.fill(box, instructions, "instructions");
    await flow.tabTo(page.getByRole("button", { name: "Prepare tests" }), "Prepare tests");
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
    await flow.tabTo(sourceButton(page, "recorded"), "recorded source");
    await flow.press("Enter", "recorded source");
    for (const [label, value] of [["Recorded replies", transcript], ["Expected behavior", expectation]]) {
      await flow.tabTo(page.getByLabel(label, { exact: true }), label);
      await flow.fill(page.getByLabel(label, { exact: true }), value, label);
    }
    await flow.tabTo(page.getByRole("button", { name: "Review recorded replies", exact: true }), "Review recorded replies");
    await flow.press("Enter", "Review recorded replies");
    await submitted(fake, "/evidence");
    await flow.save();
  });

  test("Improve imports a test pack", async ({ page }) => {
    const { fake, flow } = await open(page, "improve-import-keyboard", { keyboardOnly: true });
    await flow.tabTo(doorButton(page, "improve"), "Improve door");
    await flow.press("Enter", "Improve door");
    await doorReady(page, "improve");
    await flow.tabTo(sourceButton(page, "import"), "Import a test pack");
    await flow.chooseFile(sourceButton(page, "import"), packFile, "Import a test pack");
    await submitted(fake, "/import");
    await flow.save();
  });
});
