// The "Ink & Clay" visual foundation (plan PR 2; design-direction.md sections 4 and 7): the ink
// primary, clay and sage status with a glyph and a word, the display serif roles, 3:1 control
// boundaries, elevated surfaces, the working-state motion and the browser theme colour. Every
// `/v1/vibe/**` request is fulfilled by a fake, so no API or model is reached.
import { expect, test, type Locator, type Page } from "@playwright/test";
import type { Session } from "../../src/lib/vibe";
import { description, fakeVibe, instructions, models, project } from "./entry-fake";
import { chooseDoor, composer, doorButton, doorReady, entryHeading, entryReady, sendButton, sourceButton } from "./entry-model";

type Scheme = "light" | "dark";
const schemes = ["light", "dark"] as const;
// Token values from project-workspace.css, as computed colours.
const ink = { light: "rgb(28, 26, 22)", dark: "rgb(237, 233, 225)" };
const onInk = { light: "rgb(250, 248, 245)", dark: "rgb(18, 17, 15)" };
const canvas = { light: "rgb(250, 248, 245)", dark: "rgb(18, 17, 15)" };
const elevated = { light: "rgb(255, 255, 255)", dark: "rgb(33, 31, 27)" };
const clay = { light: "rgb(168, 67, 42)", dark: "rgb(240, 149, 122)" };
const sage = { light: "rgb(46, 107, 64)", dark: "rgb(140, 203, 154)" };
const clayMore = { light: "rgb(142, 53, 32)", dark: "rgb(255, 179, 158)" };
const sageMore = { light: "rgb(31, 85, 48)", dark: "rgb(168, 221, 179)" };
const themeColor = { light: "#faf8f5", dark: "#12110f" };

test.use({ viewport: { width: 1440, height: 960 }, reducedMotion: "reduce" });
test.describe.configure({ timeout: 90_000 });

async function settle(page: Page, scheme: Scheme, media: Parameters<Page["emulateMedia"]>[0] = {}) {
  await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce", contrast: "no-preference", ...media });
  await expect(page.locator("main.vibe-workspace")).toHaveAttribute("data-appearance", scheme);
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
}

// WCAG 1.4.11 for a control: its border against the colour outside it and against its own fill,
// whichever is lower. Alpha is composited down to the first opaque ancestor, as in navigation.spec.ts.
function boundary(control: Locator) {
  return control.evaluate(element => {
    const paint = document.createElement("canvas").getContext("2d")!;
    const rgba = (color: string) => { paint.clearRect(0, 0, 1, 1); paint.fillStyle = color; paint.fillRect(0, 0, 1, 1); return [...paint.getImageData(0, 0, 1, 1).data].map((v, i) => i === 3 ? v / 255 : v); };
    const blend = (fg: number[], bg: number[]) => fg.slice(0, 3).map((v, i) => v * fg[3] + bg[i] * (1 - fg[3]));
    const behind = (start: Element | null) => { const layers: number[][] = []; for (let el = start; el; el = el.parentElement) { const c = rgba(getComputedStyle(el).backgroundColor); layers.push(c); if (c[3] === 1) break; } return layers.reverse().reduce((bg, c) => blend(c, bg), [255, 255, 255]); };
    const luminance = (c: number[]) => c.map(v => { const n = v / 255; return n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4; }).reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0);
    const ratio = (a: number[], b: number[]) => { const x = luminance(a), y = luminance(b); return (Math.max(x, y) + .05) / (Math.min(x, y) + .05); };
    const outside = behind(element.parentElement), inside = behind(element);
    const border = blend(rgba(getComputedStyle(element).borderTopColor), inside);
    return Math.min(ratio(border, outside), ratio(border, inside));
  });
}
const css = (locator: Locator, property: string) => locator.evaluate((el, name) => getComputedStyle(el).getPropertyValue(name), property);

async function toResults(page: Page) {
  const fake = await fakeVibe(page);
  await page.goto("/vibe-evals");
  await entryReady(page, 30_000);
  await chooseDoor(page, "build");
  await doorReady(page, "build");
  await composer(page).fill(description);
  await expect(sendButton(page)).toBeEnabled({ timeout: 20_000 });
  await sendButton(page).click();
  await expect(page.locator(".vibe-project-header")).toContainText("Run 1: Completed", { timeout: 30_000 });
  await page.getByRole("tab", { name: "Results", exact: true }).click();
  await expect(page.getByRole("article", { name: "Run results" })).toBeVisible();
  return fake;
}

test("the entry and both door starts: the display serif question, 3:1 control boundaries, a flat composer and dock", async ({ page }) => {
  const fake = await fakeVibe(page);
  await page.goto("/vibe-evals");
  await entryReady(page, 30_000);
  for (const scheme of schemes) for (const contrast of ["no-preference", "more"] as const) {
    await settle(page, scheme, { contrast });
    const title = entryHeading(page);
    await expect(title, scheme).toHaveCSS("font-family", /Instrument[ _]Serif/);
    await expect(title, scheme).toHaveCSS("font-weight", "400");
    // Rule: the display serif is never below 20px; the entry question is the largest role (56px at 1440).
    expect(parseFloat(await css(title, "font-size")), scheme).toBeGreaterThanOrEqual(36);
    for (const door of ["build", "improve"] as const) expect(await boundary(doorButton(page, door)), `${scheme} ${door} door`).toBeGreaterThanOrEqual(3);
  }
  await chooseDoor(page, "build");
  await doorReady(page, "build");
  await composer(page).blur();
  const box = page.locator(".vibe-composer").filter({ has: composer(page) });
  const dock = page.locator(".vibe-composer-dock").filter({ has: composer(page) });
  for (const scheme of schemes) for (const contrast of ["no-preference", "more"] as const) {
    await settle(page, scheme, { contrast });
    expect(await boundary(box), `${scheme} composer`).toBeGreaterThanOrEqual(3);
    // Elevation 1: a hairline or a border, no shadow; the dock has no fade and no upward shadow.
    await expect(box, scheme).toHaveCSS("box-shadow", "none");
    await expect(dock, scheme).toHaveCSS("box-shadow", "none");
    await expect(dock, scheme).toHaveCSS("background-image", "none");
    await expect(dock, scheme).toHaveCSS("background-color", canvas[scheme]);
    const secondary = page.locator(".vibe-button-secondary").filter({ visible: true });
    expect(await secondary.count(), scheme).toBeGreaterThan(0);
    for (const button of await secondary.all()) expect(await boundary(button), `${scheme} ${await button.textContent()}`).toBeGreaterThanOrEqual(3);
  }
  await page.goto("/vibe-evals");
  await entryReady(page, 30_000);
  await chooseDoor(page, "improve");
  await doorReady(page, "improve");
  for (const scheme of schemes) for (const contrast of ["no-preference", "more"] as const) {
    await settle(page, scheme, { contrast });
    for (const source of ["instructions", "recorded", "import"] as const) expect(await boundary(sourceButton(page, source)), `${scheme} ${source} source`).toBeGreaterThanOrEqual(3);
  }
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});

test("results: an ink primary, clay and sage status with a glyph and a word, and the serif title and outcome", async ({ page }) => {
  const fake = await toResults(page);
  const results = page.getByRole("article", { name: "Run results" });
  await results.getByText(/^Other results ·/).click();
  const issue = results.locator("summary").filter({ hasText: "Issue found" }).first();
  const passed = results.locator("summary").filter({ hasText: "Passed" }).first();
  await expect(passed).toBeVisible();
  const title = page.locator(".vibe-project-identity h1");
  const outcome = page.locator(".vibe-results-outcome");
  for (const scheme of schemes) {
    await settle(page, scheme);
    const primary = page.locator(".vibe-button-primary").filter({ visible: true });
    await expect(primary, scheme).toHaveCount(1);
    await expect(primary, scheme).toHaveCSS("background-color", ink[scheme]);
    await expect(primary, scheme).toHaveCSS("color", onInk[scheme]);
    // Each status colour sits on a glyph and on a word, never alone.
    await expect(issue.locator("svg").first(), scheme).toHaveCSS("color", clay[scheme]);
    await expect(issue.getByText("Issue found", { exact: true }), scheme).toHaveCSS("color", clay[scheme]);
    await expect(passed.locator("svg").first(), scheme).toHaveCSS("color", sage[scheme]);
    await expect(passed.getByText("Passed", { exact: true }), scheme).toHaveCSS("color", sage[scheme]);
    await expect(title, scheme).toHaveCSS("font-family", /Instrument[ _]Serif/);
    await expect(title, scheme).toHaveCSS("font-size", "28px");
    await expect(outcome, scheme).toHaveCSS("font-family", /Instrument[ _]Serif/);
    expect(parseFloat(await css(outcome, "font-size")), scheme).toBeGreaterThanOrEqual(32);
    expect(await boundary(page.getByRole("combobox", { name: "Filter tests" })), `${scheme} filter select`).toBeGreaterThanOrEqual(3);
    // Increased contrast deepens clay and sage and raises the control boundary.
    await settle(page, scheme, { contrast: "more" });
    await expect(issue.getByText("Issue found", { exact: true }), `${scheme} more contrast`).toHaveCSS("color", clayMore[scheme]);
    await expect(passed.getByText("Passed", { exact: true }), `${scheme} more contrast`).toHaveCSS("color", sageMore[scheme]);
    expect(await boundary(page.getByRole("combobox", { name: "Filter tests" })), `${scheme} more contrast filter select`).toBeGreaterThanOrEqual(4.5);
  }
  await page.emulateMedia({ forcedColors: "active" });
  await expect(issue.getByText("Issue found", { exact: true })).toBeVisible();
  await expect(passed.getByText("Passed", { exact: true })).toBeVisible();
  await expect(issue.locator("svg").first()).toHaveCSS("color", await css(issue.getByText("Issue found", { exact: true }), "color"));
  await expect(passed.locator("svg").first()).toHaveCSS("color", await css(passed.getByText("Passed", { exact: true }), "color"));
  await page.emulateMedia({ forcedColors: "none" });
  // Phones keep Geist 600 20 for the project name: the serif is for room, not for every width.
  await page.setViewportSize({ width: 390, height: 844 });
  await settle(page, "light");
  await expect(title).toHaveCSS("font-family", /Geist/);
  await expect(title).toHaveCSS("font-size", "20px");
  await expect(title).toHaveCSS("font-weight", "600");
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});

test("dialogs sit on the elevated surface, and the browser bar follows the appearance", async ({ page }) => {
  const fake = await toResults(page);
  await expect(page.locator(".vibe-project-header")).toHaveCSS("backdrop-filter", "blur(12px)");
  const meta = () => page.evaluate(() => [...document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')].find(item => !item.media || matchMedia(item.media).matches)?.content);
  for (const scheme of schemes) {
    await settle(page, scheme);
    await expect.poll(meta, scheme).toBe(themeColor[scheme]);
  }
  await settle(page, "light");
  await page.getByRole("button", { name: "Details", exact: true }).click();
  const details = page.getByRole("complementary", { name: "Details utility" });
  const appearance = details.getByLabel("Appearance", { exact: true });
  // A Settings choice that differs from the system appearance also recolours the browser bar.
  await appearance.selectOption("dark");
  await expect.poll(meta).toBe(themeColor.dark);
  for (const scheme of schemes) {
    await appearance.selectOption(scheme);
    await details.getByRole("button", { name: "Settings", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Settings and details" });
    await expect(dialog, scheme).toHaveCSS("background-color", elevated[scheme]);
    await expect(dialog, scheme).not.toHaveCSS("box-shadow", "none");
    // HIG Dark Mode: the elevated surface is lighter than the canvas behind it.
    const lighter = await dialog.evaluate(el => {
      const value = (color: string) => color.match(/[\d.]+/g)!.slice(0, 3).map(Number).reduce((a, b) => a + b, 0);
      return value(getComputedStyle(el).backgroundColor) - value(getComputedStyle(document.querySelector("main.vibe-workspace")!).backgroundColor);
    });
    if (scheme === "dark") expect(lighter, "dark dialog lighter than the canvas").toBeGreaterThan(0);
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
  }
  await appearance.selectOption("system");
  await expect.poll(meta).toBe(themeColor.light);
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});

// A test project with a run whose total is not known yet: the progress strip shows the indeterminate bar.
function runningProject(): Session {
  const s = project("sweep-fixture", "test");
  s.title = "Return desk checks";
  s.document.artifacts = [{ id: "v1", kind: "test_suite", title: "Return desk checks", summary: "Returns", agent_prompt: instructions, accepted: true, source_message_id: "request", proposal_message_id: "prepared",
    blueprint: { cases: [{ key: "case-1", payload: { question: "Can I return this?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Ask for age and condition." }] }] } }];
  s.document.active_artifact_id = "v1";
  s.document.messages = [{ id: "request", role: "user", content: "Check my returns agent.", operation_id: "prepare" }, { id: "prepared", role: "assistant", content: "Here are the tests.", operation_id: "prepare", artifact_id: "v1" }];
  s.operations = [
    { id: "prepare", kind: "message", state: "COMPLETED", billing: "SETTLED", models, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] },
    { id: "run", kind: "check", state: "RUNNING", billing: "RESERVED", models, max_cost_nano_usd: 1, actual_cost_nano_usd: 0, results: [],
      source: { kind: "prompt", label: "Return desk checks", artifact_id: "v1" }, progress: { phase: "running_agent", completed_cases: 0, total_cases: 0 } },
  ];
  return s;
}

test("the working state: no gradient text, a soft sweep for an unknown total, and nothing running with reduced motion", async ({ page }) => {
  const state = runningProject();
  const errors: string[] = []; page.on("pageerror", e => errors.push(e.message));
  await page.route("**/v1/vibe/**", route => {
    const p = new URL(route.request().url()).pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, "Access-Control-Allow-Headers": "Content-Type,Authorization,If-Match", "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS" } });
    if (p.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, two_door: true, availability: { guest: { available: true }, workspace: { available: true } }, defaults: models, models: [{ id: models.target, name: models.target }] } });
    if (p.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
    if (p.endsWith("/sessions")) return route.fulfill({ headers, json: [state] });
    if (p.endsWith(`/sessions/${state.id}`)) return route.fulfill({ headers, json: state });
    return route.fulfill({ headers, json: [] });
  });
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "no-preference" });
  await page.goto(`/vibe-evals?session=${state.id}`);
  const bar = page.locator('.vibe-progress-bar[data-indeterminate]').first();
  await expect(bar).toBeVisible({ timeout: 30_000 });
  const fill = bar.locator("span");
  // Rule 5: no gradient text. The status is a static line beside three pulsing dots.
  const clippedText = await page.evaluate(() => [...document.querySelectorAll(".vibe-workspace *")].filter(el => /text/.test(getComputedStyle(el).backgroundClip + getComputedStyle(el).getPropertyValue("-webkit-background-clip"))).map(el => `${el.tagName}.${el.getAttribute("class") || ""}`));
  expect(clippedText, "No gradient-clipped text in the workspace").toEqual([]);
  await expect(page.locator(".vibe-progress-strip .vibe-status-dots")).toBeVisible();
  // An unknown total sweeps a soft highlight; it does not march dashes along the whole track.
  await expect.poll(() => fill.evaluate(el => el.getAnimations().filter(a => a.playState === "running").map(a => (a as CSSAnimation).animationName))).toEqual(["vibe-progress-sweep"]);
  expect(await css(fill, "background-image")).not.toContain("repeating");
  // Reduced motion: the bar rests at 35% and nothing in the page runs.
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await expect.poll(() => page.evaluate(() => document.getAnimations().filter(a => a.playState === "running").length)).toBe(0);
  const share = await fill.evaluate(el => el.getBoundingClientRect().width / el.parentElement!.getBoundingClientRect().width);
  expect(share).toBeCloseTo(0.35, 2);
  await expect(page.locator(".vibe-status-dots")).not.toBeVisible();
  const media = await page.context().newCDPSession(page);
  await media.send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: "light" }, { name: "prefers-reduced-motion", value: "reduce" }, { name: "prefers-reduced-transparency", value: "reduce" }] });
  for (const surface of [page.locator(".vibe-project-header"), page.locator(".vibe-progress-strip[data-active]")]) {
    await expect(surface).toHaveCSS("background-color", canvas.light);
    await expect(surface).toHaveCSS("backdrop-filter", "none");
  }
  await media.detach();
  await page.getByRole("button", { name: "Details", exact: true }).click();
  await page.getByRole("complementary", { name: "Details utility" }).getByRole("button", { name: "Settings", exact: true }).click();
  const modelIDs = page.getByRole("dialog", { name: "Settings and details" }).locator(`span[title="${models.target}"]`);
  await expect(modelIDs).toHaveCount(3);
  for (let index = 0; index < 3; index++) {
    await expect(modelIDs.nth(index)).toHaveCSS("font-family", /IBM[ _]Plex[ _]Mono/);
    await expect(modelIDs.nth(index)).toHaveCSS("font-variant-numeric", "tabular-nums");
  }
  expect(errors).toEqual([]);
});

test("the loading fallback paints the Vibe canvas of the system appearance, not the dark site canvas", async ({ page }) => {
  // The route's explicit client loading boundary also renders in next dev, where useSearchParams
  // does not suspend. Hold bundles before navigation to inspect that real server-rendered fallback.
  await page.route(/\/_next\/static\/chunks\/.*\.js/, route => route.abort());
  for (const scheme of schemes) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.goto("/vibe-evals", { waitUntil: "commit" });
    const fallback = page.locator("main.vibe-route-fallback");
    await expect(fallback, scheme).toBeVisible();
    await expect(fallback, scheme).toHaveCSS("background-color", canvas[scheme]);
    expect((await fallback.boundingBox())!.height, scheme).toBeGreaterThanOrEqual(960);
  }
});
