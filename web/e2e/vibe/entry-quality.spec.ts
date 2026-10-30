// Screen quality of the entry and the start of both doors (verification.md section 6, B, D, E, F, G).
// Each viewport walks the states once in beforeAll and measures both appearances; each check is
// then one small test. A known defect is held at its measured baseline, so both a regression and a fix fail.
import { expect, test, type Page } from "@playwright/test";
import { description, fakeVibe } from "./entry-fake";
import { axe, census, focused, labelInName, seriousOrCritical, tabsTo, writeRecord, type AxeViolation, type Census } from "./flow-meter";
import { chooseDoor, composer, doorButton, doorReady, entryReady, improveSource, repliesInput, sendButton, startComposer, startCopy } from "./entry-model";

const states = ["S0", "S1", "S1t", "S2", "S2i", "S2r"] as const;
const schemes = ["light", "dark"] as const;
type State = (typeof states)[number];
type Scheme = (typeof schemes)[number];
type Measure = { census: Record<Scheme, Census>; axe: Record<Scheme, AxeViolation[]>; animations: number; focus?: { tag: string; name: string }; tabsToDoor?: number };

const viewports = [
  { label: "1440 mouse", viewport: { width: 1440, height: 960 }, touch: false },
  { label: "390 touch", viewport: { width: 390, height: 844 }, touch: true },
];
const known = (id: string) => `Known defect ${id} (see ~/.claude/plans/vibe-evals-redesign.md section 8.2): it must stay at today's baseline. The PR that fixes it deletes its baseline entry here.`;
// Today's defects (d2248411), held at their measured baseline. A regression fails, and so does a fix,
// which forces the fixing PR to delete the entry and meet the target instead.
// PR 1 fixed B3 and E2. PR 4 fixes B2 on starts, D2 and both F1 baselines.
const baseline: Record<"B2" | "B3" | "E2" | "F1", Partial<Record<State, unknown>>> = {
  B2: {},
  B3: {},
  E2: {},
  // Measured at 390 touch, the only viewport that runs F1.
  F1: {},
};
const held = (id: keyof typeof baseline, state: State) => baseline[id][state] !== undefined;
function check(id: keyof typeof baseline, state: State, actual: unknown, target: () => void) {
  if (held(id, state)) expect(actual, known(id)).toEqual(baseline[id][state]);
  else target();
}

async function settle(page: Page, scheme: Scheme) {
  await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
  await expect(page.locator("main.vibe-workspace")).toHaveAttribute("data-appearance", scheme);
  await page.evaluate(async () => { await document.fonts.ready; await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
}

// Focus after a screen change. Give a deferred focus move up to 2 s before reading it.
async function focusAfterChange(page: Page) {
  await expect.poll(async () => (await focused(page)).tag, { timeout: 2_000 }).not.toBe("body").catch(() => undefined);
  const active = page.locator(":focus");
  await expect(active).toHaveAccessibleName(/\S/);
  const focus = await focused(page);
  return { ...focus, name: await active.getAttribute("aria-label") || await active.evaluate(element => (element as HTMLTextAreaElement).labels?.[0]?.textContent || element.textContent || "") };
}

async function measure(page: Page, extra: Partial<Measure> = {}): Promise<Measure> {
  await settle(page, "light");
  const animations = await page.evaluate(() => document.getAnimations().filter(animation => animation.playState === "running").length);
  const result = { census: {}, axe: {}, animations, ...extra } as Measure;
  for (const scheme of schemes) {
    await settle(page, scheme);
    result.census[scheme] = await census(page);
    result.axe[scheme] = await axe(page);
  }
  await settle(page, "light");
  return result;
}

for (const { label, viewport, touch } of viewports) test.describe(label, () => {
  const results = {} as Record<State, Measure>;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(180_000);
    const context = await browser.newContext({ baseURL: test.info().project.use.baseURL, viewport, hasTouch: touch, isMobile: touch, colorScheme: "light", reducedMotion: "reduce" });
    const start = async () => {
      const page = await context.newPage();
      const fake = await fakeVibe(page);
      await page.goto("/vibe-evals");
      await entryReady(page, 30_000);
      return { page, fake };
    };
    const build = await start();
    const tabsToDoor = await tabsTo(build.page, doorButton(build.page, "build"));
    expect(tabsToDoor, "Tab reaches the Build door").toBeGreaterThan(0);
    results.S0 = await measure(build.page, { tabsToDoor });
    await chooseDoor(build.page, "build");
    await doorReady(build.page, "build");
    results.S1 = await measure(build.page, { focus: await focusAfterChange(build.page) });
    await composer(build.page).fill(description);
    await expect(sendButton(build.page)).toBeEnabled({ timeout: 20_000 });
    await expect(build.page.getByText("Free trial", { exact: true })).toBeVisible();
    expect(build.fake.posts).toEqual([]);
    results.S1t = await measure(build.page);
    // The Improve door has its own draft.
    const improve = await start();
    await chooseDoor(improve.page, "improve");
    await doorReady(improve.page, "improve");
    results.S2 = await measure(improve.page, { focus: await focusAfterChange(improve.page) });
    await improveSource(improve.page, "instructions");
    await expect(startComposer(improve.page, "improve")).toBeVisible();
    results.S2i = await measure(improve.page);
    await improveSource(improve.page, "recorded");
    await expect(repliesInput(improve.page)).toBeVisible();
    await expect(repliesInput(improve.page)).toHaveAttribute("placeholder", /Customer:.*\nAgent:/);
    results.S2r = await measure(improve.page);
    for (const { fake } of [build, improve]) { expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]); }
    expect(Object.keys(results)).toEqual([...states]);
    await writeRecord(`quality-${label.replace(" ", "-")}`, results);
    await context.close();
  });

  test("Build preview wraps whole numbered steps and keeps a short door name", async ({ page }) => {
    await page.setViewportSize(viewport);
    const fake = await fakeVibe(page);
    await page.goto("/vibe-evals"); await entryReady(page);
    const door = doorButton(page, "build");
    await expect(door).toHaveAccessibleName("Build an agent");
    await expect(door).toHaveAccessibleDescription("Turn a task into an agent and three tests. Next: Describe the task");
    const steps = door.locator("ol > li");
    await expect(steps).toHaveText(["01Describe the task", "02We draft the agent", "03Three tests run"]);
    const layout = await steps.evaluateAll(items => items.map(item => {
      const box = item.getBoundingClientRect(), number = item.querySelector("b")!, label = document.createRange();
      label.selectNodeContents(item.lastChild!);
      return { x: box.x, y: box.y, labelLines: label.getClientRects().length, whiteSpace: getComputedStyle(item).whiteSpace, numerals: getComputedStyle(number).fontVariantNumeric };
    }));
    for (const item of layout) {
      expect(item.whiteSpace).toBe("nowrap"); expect(item.labelLines).toBe(1);
      expect(item.numerals).toContain("tabular-nums");
    }
    if (touch) {
      expect(layout.map(item => item.x)).toEqual([layout[0].x, layout[0].x, layout[0].x]);
      expect(layout[1].y).toBeGreaterThan(layout[0].y); expect(layout[2].y).toBeGreaterThan(layout[1].y);
    } else expect(await door.locator("ol").evaluate(list => getComputedStyle(list).flexWrap)).toBe("wrap");
    expect(fake.posts).toEqual([]); expect(fake.errors).toEqual([]);
  });

  test("Build input uses its visible label as its accessible name", async ({ page }) => {
    await page.setViewportSize(viewport);
    const fake = await fakeVibe(page);
    await page.goto("/vibe-evals"); await entryReady(page); await chooseDoor(page, "build"); await doorReady(page, "build");
    await expect(startComposer(page, "build")).toHaveAccessibleName(startCopy.build.heading);
    expect(fake.posts).toEqual([]); expect(fake.errors).toEqual([]);
  });

  // The structural checks below use the light appearance: this test proves dark gives the same structure.
  test("light and dark give the same structure", () => {
    for (const state of states) expect(results[state].census.dark, state).toEqual(results[state].census.light);
  });

  for (const state of states) {
    test(`B2 ${state}: ${held("B2", state) ? "known defect held at baseline" : state === "S0" ? "no primary, the two doors are the choice" : "exactly one primary"}`, () => {
      const primary = results[state].census.light.primary;
      // Rule: the entry (S0) offers its two doors as the choice, so it has no primary; every other state has one.
      check("B2", state, primary, () => expect(primary).toHaveLength(state === "S0" ? 0 : 1));
    });
    test(`B3 ${state}: ${held("B3", state) ? "known defect held at baseline" : "exactly one visible h1"}`, () => {
      const h1 = results[state].census.light.h1.length;
      check("B3", state, h1, () => expect(h1).toBe(1));
    });
    for (const scheme of schemes) {
      test(`E1 ${state} ${scheme}: axe finds no serious or critical WCAG 2.2 AA violation`, () => {
        expect(seriousOrCritical(results[state].axe[scheme])).toEqual([]);
      });
      test(`E2 ${state} ${scheme}: ${held("E2", state) ? "known defect held at baseline" : "visible labels are part of accessible names"}`, () => {
        const nodes = results[state].axe[scheme].filter(violation => violation.id === labelInName).flatMap(violation => violation.nodes);
        check("E2", state, nodes, () => expect(nodes).toEqual([]));
      });
    }
    if (touch) test(`F1 ${state}: ${held("F1", state) ? "known defect held at baseline" : "every enabled control is at least 44 by 44 px on a touch screen"}`, () => {
      const small = results[state].census.light.rows.filter(row => !row.disabled && !row.inline && (row.w < 44 || row.h < 44)).map(row => `${row.name} ${row.w}x${row.h}`);
      check("F1", state, small, () => expect(small).toEqual([]));
    });
    test(`F2 ${state}: no horizontal scroll`, () => {
      expect(results[state].census.light.overflow).toBe(false);
    });
    test(`G2 ${state}: no running animation with reduced motion`, () => {
      expect(results[state].animations).toBe(0);
    });
  }

  test("D2 S0: at most 3 Tab presses reach the first door", () => {
    // The empty entry starts its keyboard path at the doors.
    expect(results.S0.tabsToDoor).toBeLessThanOrEqual(3);
  });
  // D3: the uncommitted start heading takes focus after each door click.
  for (const state of ["S1", "S2"] as const) test(`D3 ${state}: focus moves into the door, not to the page body`, () => {
    expect(results[state].focus).toEqual({ tag: state === "S1" ? "textarea" : "h1", name: expect.stringMatching(/\S/) });
  });
});
