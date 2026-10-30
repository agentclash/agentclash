// One home for the copy and roles of the /vibe-evals entry screen and its two doors.
// A redesign of the entry or the doors changes this file, not every spec.
import { expect, type Locator, type Page } from "@playwright/test";

export type Door = "build" | "improve";
export type ImproveSource = "instructions" | "recorded" | "import";
type FilePayload = { name: string; mimeType: string; buffer: Buffer };

export const startCopy = {
  build: { heading: "What task should the agent handle?", composer: "What task should the agent handle?", primary: "Build and run 3 tests", switchDoor: "Already have an agent? Improve it instead." },
  improve: { heading: "Improve an agent you already have.", composer: "Agent instructions to test", switchDoor: "Actually, I want to build an agent." },
  sources: { instructions: "Try my saved instructions", recorded: "Test replies from my app", import: "Import a test pack" },
  primaries: { instructions: "Prepare tests", recorded: "Review recorded replies", import: /^(Choose a file|Import test pack)$/ },
  replies: "Recorded replies", rules: "Expected behavior",
} as const;
export const entryCopy = {
  heading: "What would you like to evaluate?", choices: "New project choices",
  doors: { build: "Build an agent", improve: "Improve an existing agent" },
  doorHeadings: { build: startCopy.build.heading, improve: startCopy.improve.heading },
  sources: startCopy.sources, composer: startCopy.build.composer, send: startCopy.build.primary,
} as const;

export const startScreen = (page: Page) => page.locator(".vibe-start-page");
export const entryHeading = (page: Page) => page.getByRole("heading", { name: entryCopy.heading });
// The reason the doors are unavailable (connection, workspace, capacity), with its one action.
export const entryNotice = (page: Page) => page.locator(".vibe-entry-availability");
export const workspacePicker = (page: Page) => page.getByRole("combobox", { name: "Workspace for new projects", exact: true });
export const newProjectChoices = (page: Page, exact = true) => page.getByLabel(entryCopy.choices, { exact });
export const doorButton = (page: Page, door: Door) => page.getByRole("button", { name: entryCopy.doors[door], exact: true });
export const doorHeading = (page: Page, door: Door, exact = true) => startScreen(page).getByRole("heading", { level: 1, name: startCopy[door].heading, exact });
export const projectStartHeading = (page: Page, door: Door) => page.getByRole("heading", { level: 2, name: startCopy[door].heading, exact: true });
export const sourceButton = (page: Page, source: ImproveSource) => page.getByRole("button", { name: startCopy.sources[source], exact: true });
export const startComposer = (page: Page, door: Door) => startScreen(page).getByRole("textbox", { name: startCopy[door].composer, exact: true });
// Retain the project Guide composer as well as the start inputs for existing specs.
export const composer = (page: Page) => page.getByRole("textbox", { name: "Message Vibe Evals", exact: true }).or(startComposer(page, "build")).or(startComposer(page, "improve"));
export const buildResult = (page: Page) => page.getByRole("article", { name: "Agent test results", exact: true });
export const sendButton = (page: Page) => startScreen(page).getByRole("button", { name: startCopy.build.primary, exact: true });
export const improvePrimary = (page: Page, source: ImproveSource) => startScreen(page).getByRole("button", { name: startCopy.primaries[source], exact: true });
export const repliesInput = (page: Page) => startScreen(page).getByRole("textbox", { name: startCopy.replies, exact: true });
export const rulesInput = (page: Page) => startScreen(page).getByRole("textbox", { name: startCopy.rules, exact: true });
export const switchDoorButton = (page: Page, door: Door) => startScreen(page).getByRole("button", { name: startCopy[door].switchDoor, exact: true });
export const sampleButton = (page: Page) => page.getByRole("button", { name: "Read a sample test", exact: true });
// A filled primary button (components/vibe/vibe-button.tsx). Add any new primary style here too.
export const primaryButtons = (page: Page): Locator => page.locator(".vibe-button-primary").filter({ visible: true });

/** The entry screen is shown and the first door can be chosen. */
export async function entryReady(page: Page, timeout?: number) {
  await expect(entryHeading(page)).toBeVisible({ timeout });
  await expect(doorButton(page, "build")).toBeEnabled({ timeout });
}

export async function chooseDoor(page: Page, door: Door) {
  await doorButton(page, door).click();
}

/** The start screen of a door is shown. */
export async function doorReady(page: Page, door: Door, timeout?: number) {
  await expect(doorHeading(page, door)).toBeVisible({ timeout });
  await expect(door === "build" ? startComposer(page, door) : doorHeading(page, door)).toBeFocused({ timeout });
  await expect(door === "build" ? sendButton(page) : startScreen(page).getByRole("group", { name: "Agent source" })).toBeVisible({ timeout });
  await expect(page.locator("h1:visible")).toHaveCount(1);
  await expect(page.locator(".vibe-project-header")).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp(`[?&]start=${door === "build" ? "build" : "improve"}(?:&|$)`));
}

/**
 * Shows a project view. Since PR 1, Results (after a run) and Tests (after the first test
 * preparation) can open by themselves. When `finished` names a run ("Run 1"), wait until the
 * page shows that run as ended and the view settles; then click the tab only when it is not selected.
 * Pass `opensResults` for a run that opens Results by itself (the first Build run): the helper waits
 * until Results is selected, then clicks. The real backend ends the run before it marks the Build
 * results ready (backend/internal/vibe/reliable_recovery.go), so the run can show as ended before
 * the move, and a late move would undo the click.
 */
export async function showView(page: Page, name: "Agent" | "Tests" | "Results", finished?: string, { opensResults = false } = {}) {
  if (finished) {
    await expect(page.locator(".vibe-project-header")).toContainText(new RegExp(`${finished}: (Completed|Failed|Stopped|Partly complete|Expired)`), { timeout: 30_000 });
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  }
  if (opensResults) await expect(page.getByRole("tab", { name: "Results", exact: true })).toHaveAttribute("aria-selected", "true", { timeout: 30_000 });
  const tab = page.getByRole("tab", { name, exact: true });
  if (await tab.getAttribute("aria-selected") !== "true") await tab.click();
  await expect(tab).toHaveAttribute("aria-selected", "true");
}

/** Chooses a source on the Improve start screen. For "import", pass the file to choose. */
export async function improveSource(page: Page, source: ImproveSource, file?: FilePayload) {
  const chooser = source === "import" && file ? page.waitForEvent("filechooser") : undefined;
  await sourceButton(page, source).click();
  if (chooser && file) await (await chooser).setFiles(file);
}
