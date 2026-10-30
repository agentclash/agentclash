// One home for the copy and roles of the /vibe-evals entry screen and its two doors.
// A redesign of the entry or the doors changes this file, not every spec.
import { expect, type Locator, type Page } from "@playwright/test";

export type Door = "build" | "improve";
export type ImproveSource = "instructions" | "recorded" | "import";
type FilePayload = { name: string; mimeType: string; buffer: Buffer };

export const entryCopy = {
  heading: "What would you like to evaluate?",
  choices: "New project choices",
  doors: { build: /Build an agent/, improve: /Improve an existing agent/ },
  doorHeadings: { build: "What would you like help with?", improve: "Improve an agent you already have." },
  sources: { instructions: "Try my saved instructions", recorded: "Test replies from my app", import: "Import a test pack" },
  composer: "Message Vibe Evals",
  send: "Send message",
} as const;

export const entryHeading = (page: Page) => page.getByRole("heading", { name: entryCopy.heading });
// The reason the doors are unavailable (connection, workspace, capacity), with its one action.
export const entryNotice = (page: Page) => page.locator(".vibe-entry-availability");
export const workspacePicker = (page: Page) => page.getByRole("combobox", { name: "Workspace for new projects", exact: true });
export const newProjectChoices = (page: Page, exact = true) => page.getByLabel(entryCopy.choices, { exact });
export const doorButton = (page: Page, door: Door) => page.getByRole("button", { name: entryCopy.doors[door] });
export const doorHeading = (page: Page, door: Door, exact?: boolean) =>
  page.getByRole("heading", { name: entryCopy.doorHeadings[door], ...(exact === undefined ? {} : { exact }) });
export const sourceButton = (page: Page, source: ImproveSource) => page.getByRole("button", { name: entryCopy.sources[source], exact: true });
export const composer = (page: Page) => page.getByRole("textbox", { name: entryCopy.composer });
export const sendButton = (page: Page) => page.getByRole("button", { name: entryCopy.send, exact: true });
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
}

/** Chooses a source on the Improve start screen. For "import", pass the file to choose. */
export async function improveSource(page: Page, source: ImproveSource, file?: FilePayload) {
  const chooser = source === "import" && file ? page.waitForEvent("filechooser") : undefined;
  await sourceButton(page, source).click();
  if (chooser && file) await (await chooser).setFiles(file);
}
