// Screen quality of the entry and the start of both doors (verification.md section 6, B, D, E, F, G).
// Each viewport walks the states once in beforeAll and measures both appearances; each check is
// then one small test. A known defect is held at its measured baseline, so both a regression and a fix fail.
import { expect, test, type Page } from "@playwright/test";
import { description, fakeVibe } from "./entry-fake";
import { axe, census, focused, labelInName, seriousOrCritical, tabsTo, writeRecord, type AxeViolation, type Census } from "./flow-meter";
import { chooseDoor, composer, doorButton, doorReady, entryReady, improveSource, sendButton } from "./entry-model";

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
const baseline: Record<"B2" | "B3" | "E2" | "F1", Partial<Record<State, unknown>>> = {
  B2: { S1: [], S2: [], S2r: [] },
  B3: { S1: 2, S1t: 2, S2: 2 },
  // The Build send button shows "Build and run 3 tests" but is named "Send message", empty or typed.
  E2: { S1: ['button[type="submit"]'], S1t: [".vibe-button-primary"] },
  // Measured at 390 touch, the only viewport that runs F1.
  F1: { S0: ["Read a sample test 352x20"], S2r: ["Sample transcript format 352x24"] },
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
  return focused(page);
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
    await expect(build.page.getByText(/Estimate: up to/).first()).toBeVisible();
    results.S1t = await measure(build.page);
    // A fresh page, so the Improve start has no draft carried over from Build.
    const improve = await start();
    await chooseDoor(improve.page, "improve");
    await doorReady(improve.page, "improve");
    results.S2 = await measure(improve.page, { focus: await focusAfterChange(improve.page) });
    await improveSource(improve.page, "instructions");
    await expect(improve.page.getByRole("textbox", { name: "Agent instructions to test" })).toBeVisible();
    results.S2i = await measure(improve.page);
    await improve.page.getByRole("button", { name: "Back to conversation", exact: true }).click();
    await improveSource(improve.page, "recorded");
    await expect(improve.page.getByLabel("Recorded replies", { exact: true })).toBeVisible();
    results.S2r = await measure(improve.page);
    for (const { fake } of [build, improve]) { expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]); }
    expect(Object.keys(results)).toEqual([...states]);
    await writeRecord(`quality-${label.replace(" ", "-")}`, results);
    await context.close();
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
    // Known defect D2 at 1440 (7 presses, sidebar first). The next test holds it at that baseline.
    test.fail(!touch, known("D2"));
    expect(results.S0.tabsToDoor).toBeLessThanOrEqual(3);
  });
  if (!touch) test("D2 S0: no more Tab presses than today's baseline (7)", () => {
    expect(results.S0.tabsToDoor).toBeLessThanOrEqual(7);
  });
  // Known defect D3: focus falls to <body> after either door. It cannot get worse than <body>.
  for (const state of ["S1", "S2"] as const) test(`D3 ${state}: focus moves into the door, not to the page body`, () => {
    test.fail(true, known("D3"));
    expect(results[state].focus?.tag).not.toBe("body");
  });
});
