// Instruments for the entry and door checks (see ~/.claude/plans/vibe-evals-redesign/verification.md
// sections 1 and 6): an action counter, a screen census, an axe runner, a dialog watcher and a
// write log. Each flow writes its record as JSON into the test output folder (a CI artifact).
import { readFileSync } from "node:fs";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { expect, test, type Locator, type Page } from "@playwright/test";

const axeSource = readFileSync(path.resolve(__dirname, "../../node_modules/axe-core/axe.min.js"), "utf8");
export const axeTags = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];
// Experimental in axe 4.11 and off by default: WCAG 2.5.3 label in name.
export const labelInName = "label-content-name-mismatch";

export async function writeRecord(name: string, record: unknown) {
  await writeFile(test.info().outputPath(`${name}.json`), JSON.stringify(record, null, 2));
}

export type ControlRow = { zone: "main" | "nav" | "header"; first: boolean; tag: string; name: string; disabled: boolean; primary: boolean; inline: boolean; w: number; h: number };
export type Census = { controls: { all: number; main: number; mainEnabled: number }; primary: string[]; h1: string[]; words: number; overflow: boolean; rows: ControlRow[] };

/** Visible controls, primaries, h1 and words. Counts of controls and words are for the first viewport. */
export function census(page: Page): Promise<Census> {
  return page.evaluate(() => {
    const selector = 'a[href], button, input:not([type="hidden"]), select, textarea, summary, [role="button"], [role="tab"], [role="link"], [role="combobox"], [role="radio"], [role="checkbox"], [role="switch"], [role="menuitem"], [role="option"], [tabindex]:not([tabindex="-1"])';
    const text = (value: string | null | undefined) => (value || "").replace(/\s+/g, " ").trim();
    const shown = (el: Element) => {
      const box = el.getBoundingClientRect();
      return box.width > 0 && box.height > 0 && el.checkVisibility({ checkVisibilityCSS: true }) && !el.closest('[inert], [aria-hidden="true"], nextjs-portal');
    };
    const first = (el: Element) => { const box = el.getBoundingClientRect(); return box.bottom > 0 && box.right > 0 && box.top < innerHeight && box.left < innerWidth; };
    const zone = (el: Element) => el.closest('.vibe-sidebar, [aria-label="Projects sidebar"], [aria-label="Your projects"]') ? "nav" as const : el.closest("header") ? "header" as const : "main" as const;
    // A link inside a sentence is exempt from the 44 px target rule (WCAG 2.5.8 inline exception).
    const inline = (el: Element) => el.tagName === "A" && !!el.parentElement && getComputedStyle(el).display === "inline" && text(el.parentElement.textContent).length > text(el.textContent).length;
    const rows = [...new Set(document.querySelectorAll(selector))].filter(shown).map(el => {
      const box = el.getBoundingClientRect();
      return { zone: zone(el), first: first(el), tag: el.tagName.toLowerCase(), name: text(el.getAttribute("aria-label") || (el as HTMLElement).innerText || el.getAttribute("placeholder")).slice(0, 80),
        disabled: (el as HTMLButtonElement).disabled === true || el.getAttribute("aria-disabled") === "true", primary: el.classList.contains("vibe-button-primary"), inline: inline(el), w: Math.round(box.width), h: Math.round(box.height) };
    });
    const firstRows = rows.filter(row => row.first);
    let words = 0;
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      const parent = node.parentElement, value = text(node.textContent);
      if (!value || !parent || parent.closest("script, style") || !shown(parent) || !first(parent) || zone(parent) !== "main") continue;
      words += value.split(" ").length;
    }
    return {
      controls: { all: firstRows.length, main: firstRows.filter(row => row.zone === "main").length, mainEnabled: firstRows.filter(row => row.zone === "main" && !row.disabled).length },
      primary: rows.filter(row => row.primary).map(row => row.name),
      h1: [...document.querySelectorAll("h1")].filter(shown).map(h => text(h.textContent)),
      words, overflow: document.documentElement.scrollWidth > innerWidth, rows,
    };
  });
}

export type AxeViolation = { id: string; impact: string | null; tags: string[]; nodes: string[] };
/** WCAG 2.0–2.2 A and AA rules, plus the experimental label-in-name rule. */
export async function axe(page: Page): Promise<AxeViolation[]> {
  await page.evaluate(axeSource);
  return page.evaluate(async ({ tags, extra }) => {
    const runner = (window as unknown as { axe: { run: (context: unknown, options: unknown) => Promise<{ violations: { id: string; impact: string | null; tags: string[]; nodes: { target: string[] }[] }[] }> } }).axe;
    const result = await runner.run({ exclude: [["nextjs-portal"]] }, { runOnly: { type: "tag", values: tags }, rules: { [extra]: { enabled: true } }, resultTypes: ["violations"] });
    return result.violations.map(v => ({ id: v.id, impact: v.impact, tags: v.tags, nodes: v.nodes.slice(0, 5).map(n => n.target.join(" ")) }));
  }, { tags: axeTags, extra: labelInName });
}
export const seriousOrCritical = (violations: AxeViolation[]) => violations.filter(v => v.id !== labelInName && (v.impact === "serious" || v.impact === "critical"));

/** Records the most dialogs present at the same time, including one that is still closing. Call before page.goto. */
export async function watchDialogs(page: Page) {
  await page.addInitScript(() => {
    const scope = window as unknown as { __vibeMostDialogs: number };
    scope.__vibeMostDialogs = 0;
    const count = () => { scope.__vibeMostDialogs = Math.max(scope.__vibeMostDialogs, document.querySelectorAll('[role="dialog"], [role="alertdialog"]').length); };
    new MutationObserver(count).observe(document, { childList: true, subtree: true, attributes: true });
  });
}
export const mostDialogs = (page: Page) => page.evaluate(() => (window as unknown as { __vibeMostDialogs?: number }).__vibeMostDialogs ?? 0);

/** Presses Tab until the target has focus. Returns the number of presses (0 when it already has focus, D3), or -1. */
export async function tabsTo(page: Page, target: Locator, max = 40) {
  for (let presses = 0; presses <= max; presses++) {
    if (presses) await page.keyboard.press("Tab");
    if (await target.evaluate(el => el === document.activeElement).catch(() => false)) return presses;
  }
  return -1;
}

export const focused = (page: Page) => page.evaluate(() => {
  const el = document.activeElement as HTMLElement | null;
  return { tag: el?.tagName.toLowerCase() ?? "", name: (el?.getAttribute("aria-label") || el?.innerText || "").replace(/\s+/g, " ").trim().slice(0, 60) };
});

type Write = { method: string; path: string; kind?: string; afterActions: number };
type Counts = { clicks: number; texts: number; files: number; submits: number; tabs: number; actions: number };

/** Counts the user's actions on one door path, and the writes, decisions, milestones and dialogs on the way. Call before page.goto. */
export async function flowMeter(page: Page, name: string, { keyboardOnly = false } = {}) {
  await watchDialogs(page);
  const counts = { clicks: 0, texts: 0, files: 0, submits: 0, tabs: 0 };
  const snapshot = (): Counts => ({ ...counts, actions: counts.clicks + counts.texts + counts.files + counts.submits });
  const steps: string[] = [], decisions: { at: string; options: number; mainEnabled: number }[] = [], writes: Write[] = [];
  const milestones: Record<string, Counts> = {};
  const probes = new Map<string, Locator>();
  let consent: Counts | undefined;
  page.on("request", request => {
    const url = new URL(request.url());
    if (["GET", "OPTIONS", "HEAD"].includes(request.method()) || !url.pathname.startsWith("/v1/vibe/")) return;
    let kind: string | undefined;
    try { kind = request.postDataJSON()?.kind; } catch { kind = undefined; }
    writes.push({ method: request.method(), path: url.pathname.replace(/^\/v1\/vibe/, ""), ...(kind ? { kind } : {}), afterActions: snapshot().actions });
  });
  // A milestone counts the actions before the first action that follows its appearance.
  const check = async () => {
    for (const [label, locator] of probes) if (!milestones[label] && await locator.filter({ visible: true }).count()) milestones[label] = snapshot();
  };
  const act = async (kind: keyof typeof counts, label: string, run: () => Promise<unknown>) => {
    await check();
    if (keyboardOnly && kind === "clicks") throw new Error(`Pointer action "${label}" in a keyboard-only flow`);
    // Count first, so a write that this action causes is logged after it.
    counts[kind]++;
    steps.push(`${kind}:${label}`);
    await run();
  };
  return {
    click: (locator: Locator, label: string) => act("clicks", label, () => locator.click()),
    fill: (locator: Locator, value: string, label: string) => act("texts", label, () => keyboardOnly ? page.keyboard.insertText(value) : locator.fill(value)),
    /** A click on the trigger, then a file choice in the chooser. */
    chooseFile: async (trigger: Locator, file: { name: string; mimeType: string; buffer: Buffer }, label: string) => {
      const chooser = page.waitForEvent("filechooser");
      if (keyboardOnly) await act("submits", label, () => page.keyboard.press("Enter")); else await act("clicks", label, () => trigger.click());
      await act("files", file.name, async () => (await chooser).setFiles(file));
    },
    press: (key: string, label: string) => act("submits", label, () => page.keyboard.press(key)),
    tabTo: async (target: Locator, label: string) => {
      const presses = await tabsTo(page, target);
      expect(presses, `Tab presses to ${label}`).toBeGreaterThanOrEqual(0);
      counts.tabs += presses;
      steps.push(`tabs:${label}:${presses}`);
      return presses;
    },
    /** Records a decision point: the forward options the spec names, and the census of enabled main-zone controls in the first viewport. */
    decision: async (at: string, options: Locator[]) => {
      for (const option of options) await expect(option).toBeEnabled();
      decisions.push({ at, options: options.length, mainEnabled: (await census(page)).controls.mainEnabled });
    },
    /** The next action is the explicit consent to a model-bound write. */
    consent: () => { consent = snapshot(); },
    watch: (label: string, locator: Locator) => { probes.set(label, locator); },
    mark: async (label: string) => { await check(); milestones[label] ??= snapshot(); return milestones[label]; },
    async save(extra: Record<string, unknown> = {}) {
      await check();
      const record = { name, keyboardOnly, totals: snapshot(), milestones, decisions, steps, writes,
        writesBeforeConsent: consent ? writes.filter(w => w.afterActions <= consent!.actions) : writes, mostDialogs: await mostDialogs(page), ...extra };
      await writeRecord(`flow-${name}`, record);
      return record;
    },
  };
}
export type FlowMeter = Awaited<ReturnType<typeof flowMeter>>;
