// The "Ink & Clay" visual foundation (plan PR 2; design-direction.md sections 4 and 7): the ink
// primary, clay and sage status with a glyph and a word, the display serif roles, 3:1 control
// boundaries, elevated surfaces, the working-state motion and the browser theme colour. Every
// `/v1/vibe/**` request is fulfilled by a fake, so no API or model is reached.
import { expect, test, type Locator, type Page } from "@playwright/test";
import type { Session } from "../../src/lib/vibe";
import { description, fakeVibe, instructions, models, project } from "./entry-fake";
import { chooseDoor, composer, doorButton, doorReady, entryHeading, entryReady, sendButton, sourceButton, doorHeading, startScreen, startComposer, buildOptions, startCopy, otherResults, resultsMore } from "./entry-model";

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
function boundary(control: Locator, indicator: "border" | "outline" = "border") {
  return control.evaluate((element, indicator) => {
    const paint = document.createElement("canvas").getContext("2d")!;
    const rgba = (color: string) => { paint.clearRect(0, 0, 1, 1); paint.fillStyle = color; paint.fillRect(0, 0, 1, 1); return [...paint.getImageData(0, 0, 1, 1).data].map((v, i) => i === 3 ? v / 255 : v); };
    const blend = (fg: number[], bg: number[]) => fg.slice(0, 3).map((v, i) => v * fg[3] + bg[i] * (1 - fg[3]));
    const behind = (start: Element | null) => { const layers: number[][] = []; for (let el = start; el; el = el.parentElement) { const c = rgba(getComputedStyle(el).backgroundColor); layers.push(c); if (c[3] === 1) break; } return layers.reverse().reduce((bg, c) => blend(c, bg), [255, 255, 255]); };
    const luminance = (c: number[]) => c.map(v => { const n = v / 255; return n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4; }).reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0);
    const ratio = (a: number[], b: number[]) => { const x = luminance(a), y = luminance(b); return (Math.max(x, y) + .05) / (Math.min(x, y) + .05); };
    const outside = behind(element.parentElement), inside = behind(element);
    const style = getComputedStyle(element);
    const border = blend(rgba(indicator === "outline" ? style.outlineColor : style.borderTopColor), indicator === "outline" ? outside : inside);
    return Math.min(ratio(border, outside), ratio(border, inside));
  }, indicator);
}
const css = (locator: Locator, property: string) => locator.evaluate((el, name) => getComputedStyle(el).getPropertyValue(name), property);

async function composerFocus(page: Page, box: Locator, textarea: Locator, context: string) {
  await textarea.blur();
  await expect(box, context).toHaveCSS("outline-style", "none");
  await textarea.focus();
  await page.keyboard.press("Shift+Tab");
  await page.keyboard.press("Tab");
  await expect(textarea, context).toBeFocused();
  await expect.poll(() => textarea.evaluate(el => el.matches(":focus-visible")), { message: context }).toBe(true);
  await expect(textarea, context).toHaveCSS("outline-style", "none");
  await expect(box, context).toHaveCSS("outline-style", "solid");
  await expect(box, context).toHaveCSS("outline-width", "2px");
  await expect(box, context).toHaveCSS("outline-offset", "2px");
  expect(await css(box, "outline-color"), context).not.toBe(await css(box, "border-top-color"));
  expect(await boundary(box, "outline"), `${context} focus ring`).toBeGreaterThanOrEqual(3);
  await textarea.blur();
  await expect(box, context).toHaveCSS("outline-style", "none");
}

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
  await expect(page.getByRole("tab", { name: "Results", exact: true })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("article", { name: "Run results" })).toBeVisible();
  const outcome = page.locator(".vibe-project-main h1.vibe-results-outcome");
  await expect(outcome).toBeFocused();
  expect(await outcome.evaluate(element => document.activeElement === element)).toBe(true);
  return fake;
}

test("the entry and both door starts: display serif questions, 3:1 control boundaries, flat inputs and no start dock", async ({ page }) => {
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
  const box = page.locator(".vibe-build-start-card");
  await expect(buildOptions(page)).toBeVisible();
  for (const scheme of schemes) for (const contrast of ["no-preference", "more"] as const) {
    await settle(page, scheme, { contrast });
    const title = doorHeading(page, "build"), input = startComposer(page, "build");
    await expect(title, scheme).toHaveCSS("font-family", /Instrument[ _]Serif/);
    await expect(title, scheme).toHaveCSS("font-weight", "400");
    expect(parseFloat(await css(title, "font-size")), scheme).toBeGreaterThanOrEqual(32);
    await expect(box, scheme).toHaveCSS("border-top-width", "0px");
    await expect(box, scheme).toHaveCSS("padding-left", "0px");
    expect(await boundary(input), `${scheme} Build input`).toBeGreaterThanOrEqual(3);
    await expect(box, scheme).toHaveCSS("box-shadow", "none");
    await expect(box, scheme).toHaveCSS("background-image", "none");
    await expect(page.locator(".vibe-composer-dock")).toHaveCount(0);
    await input.focus(); await page.keyboard.press("Shift+Tab"); await page.keyboard.press("Tab");
    await expect(input).toBeFocused();
    await expect.poll(() => input.evaluate(element => element.matches(":focus-visible"))).toBe(true);
    await expect(input, scheme).toHaveCSS("outline-style", "solid");
    await expect(input, scheme).toHaveCSS("outline-width", "2px");
    expect(await css(input, "outline-color"), scheme).not.toBe(await css(input, "border-top-color"));
    expect(await boundary(input, "outline"), `${scheme} Build focus ring`).toBeGreaterThanOrEqual(3);
    await input.blur(); await expect(input).toHaveCSS("outline-style", "none");
    const chips = startScreen(page).getByRole("group", { name: "Example tasks" }).locator(".vibe-example-chip");
    await expect(chips).toHaveCount(2);
    for (const chip of await chips.all()) {
      await expect(chip).toHaveCSS("border-radius", "999px");
      expect((await chip.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    }
    const positions = await chips.evaluateAll(elements => elements.map(element => element.getBoundingClientRect().y));
    expect(positions[0]).toBe(positions[1]);
    await expect(page.locator(".vibe-build-price")).toHaveCSS("font-family", /Geist/);
  }
  await page.goto("/vibe-evals");
  await entryReady(page, 30_000);
  await chooseDoor(page, "improve");
  await doorReady(page, "improve");
  for (const scheme of schemes) for (const contrast of ["no-preference", "more"] as const) {
    await settle(page, scheme, { contrast });
    const title = doorHeading(page, "improve");
    await expect(title, scheme).toHaveCSS("font-family", /Instrument[ _]Serif/);
    await expect(title, scheme).toHaveCSS("font-weight", "400");
    expect(parseFloat(await css(title, "font-size")), scheme).toBeGreaterThanOrEqual(32);
    await expect(page.locator(".vibe-composer-dock")).toHaveCount(0);
    for (const source of ["instructions", "recorded", "import"] as const) {
      await sourceButton(page, source).click();
      for (const radio of await startScreen(page).getByRole("radio").all()) expect(await boundary(radio), `${scheme} ${source} segment`).toBeGreaterThanOrEqual(3);
      const surface = startScreen(page).locator(source === "import" ? ".vibe-improve-drop" : ".vibe-improve-form");
      await expect(surface, scheme).toHaveCSS("box-shadow", "none");
      await expect(surface, scheme).toHaveCSS("background-image", "none");
      if (source === "import") expect(await boundary(surface), `${scheme} pack drop zone`).toBeGreaterThanOrEqual(3);
      for (const control of await startScreen(page).locator("textarea, select").filter({ visible: true }).all()) expect(await boundary(control), `${scheme} ${source} input`).toBeGreaterThanOrEqual(3);
    }
  }
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});

test("the Guide composer has a distinct 3:1 keyboard focus ring in every appearance and contrast", async ({ page }) => {
  const fake = await toResults(page);
  const guide = page.getByRole("button", { name: "Guide", exact: true });
  if (await guide.getAttribute("aria-pressed") !== "true") await guide.click();
  const panel = page.getByRole("complementary", { name: "Guide utility", exact: true });
  const textarea = panel.getByRole("textbox", { name: "Message Vibe Evals" });
  await expect(textarea).toBeVisible();
  for (const scheme of schemes) for (const contrast of ["no-preference", "more"] as const) {
    await settle(page, scheme, { contrast });
    await composerFocus(page, panel.locator(".vibe-composer"), textarea, `${scheme} ${contrast} Guide composer`);
  }
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});

test("results: an ink primary, clay and sage status with a glyph and a word, and the serif title and outcome", async ({ page }) => {
  const fake = await toResults(page);
  const results = page.getByRole("tabpanel", { name: "Results", exact: true });
  await otherResults(page).click();
  const issue = results.locator(".vibe-build-result summary").filter({ hasText: "Issue found" }).first();
  const passed = results.locator(".vibe-build-result summary").filter({ hasText: "Passed" }).first();
  await expect(passed).toBeVisible();
  const title = page.locator(".vibe-project-identity :is(h1, h2)");
  const outcome = results.locator(".vibe-results-outcome").first();
  await expect(page.locator("h1:visible")).toHaveCount(1);
  await expect(outcome).toHaveJSProperty("tagName", "H1");
  await results.getByText("All 3 test results", { exact: true }).click();
  await resultsMore(page).click();
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

test.describe("before hydration", () => {
  test.use({ javaScriptEnabled: false });
  test("a light system paints the server entry or Suspense fallback with the light Vibe canvas", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "light" });
    await page.goto("/vibe-evals");
    // Dev renders the entry; a production build can suspend on useSearchParams.
    const canvasSurface = page.locator("main.vibe-route, main.vibe-route-fallback").filter({ visible: true });
    await expect(canvasSurface).toHaveCount(1);
    await expect(canvasSurface).toBeVisible();
    await expect(canvasSurface).toHaveCSS("background-color", canvas.light);
    expect((await canvasSurface.boundingBox())!.height).toBeGreaterThanOrEqual(960);
  });
  for (const scheme of schemes) test(`the Suspense fallback CSS paints the ${scheme} Vibe canvas`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    await page.goto("/vibe-evals");
    // Dev need not suspend. Probe the fallback CSS alongside the server entry in that case.
    await page.evaluate(() => {
      if (document.querySelector("main.vibe-route-fallback")) return;
      const fallback = document.createElement("main");
      fallback.className = "vibe-route-fallback";
      document.body.append(fallback);
    });
    const fallback = page.locator("main.vibe-route-fallback").filter({ visible: true });
    await expect(fallback).toBeVisible();
    await expect(fallback).toHaveCSS("background-color", canvas[scheme]);
    expect((await fallback.boundingBox())!.height).toBeGreaterThanOrEqual(960);
  });
});

test("review polish: shared start hierarchy, chip rows and vertical steps, aligned quiet controls and styled disclosures at 1440 and 390", async ({ page }) => {
  const fake = await fakeVibe(page);
  for (const width of [1440, 390]) for (const scheme of schemes) {
    await page.setViewportSize({ width, height: 960 });
    await page.goto("/vibe-evals");
    await entryReady(page, 30_000);
    await settle(page, scheme);
    const steps = page.locator(".vibe-door-steps > li");
    const rects = await steps.evaluateAll(elements => elements.map(element => { const rect = element.getBoundingClientRect(); return { x: rect.x, y: rect.y }; }));
    expect(rects).toHaveLength(3);
    expect(new Set(rects.map(rect => rect.x)).size).toBe(1);
    expect(rects[0].y).toBeLessThan(rects[1].y); expect(rects[1].y).toBeLessThan(rects[2].y);
    for (const door of ["build", "improve"] as const) {
      await chooseDoor(page, door); await doorReady(page, door);
      const dataUse = startScreen(page).getByRole("link", { name: "Data use and retention (new tab)", exact: true });
      await page.mouse.move(0, 0);
      await expect(dataUse).toBeVisible();
      await expect(dataUse).toHaveAccessibleName("Data use and retention (new tab)");
      await expect(dataUse).toHaveAttribute("href", "/docs/vibe-evals/data-use");
      await expect(dataUse).toHaveCSS("text-decoration-line", "underline");
      const title = doorHeading(page, door);
      await expect(title).toHaveClass("vibe-start-title");
      await expect(title).toHaveCSS("font-family", /Instrument[ _]Serif/);
      await expect(title).toHaveCSS("font-size", width === 390 ? "32px" : "42.88px");
      if (door === "improve") {
        const options = startScreen(page).locator(".vibe-improve-other > summary");
        const disclosure = options.locator("..");
        await expect(options).toHaveCSS("list-style-type", "none");
        const before = () => options.evaluate(element => getComputedStyle(element, "::before").transform);
        const closed = await before();
        await options.click();
        await expect(disclosure).toHaveJSProperty("open", true);
        await expect.poll(before).not.toBe(closed);
        await expect.poll(() => options.evaluate(element => getComputedStyle(element, "::before").transitionDuration)).toBe("0s");
      } else {
        await expect(startScreen(page).locator("details")).toHaveCount(0);
        await expect(buildOptions(page)).toBeVisible();
        const primary = sendButton(page), input = startComposer(page, "build");
        const task = (await input.boundingBox())!, action = (await primary.boundingBox())!;
        expect(Math.abs(action.x - task.x)).toBeLessThanOrEqual(1);
        expect(action.y).toBeGreaterThan((await startScreen(page).locator(".vibe-build-example-chips").boundingBox())!.y);
        await expect(primary).toHaveAccessibleDescription("Describe the task in a sentence or two.");
        await expect(startScreen(page).getByRole("button", { name: startCopy.build.switchDoor, exact: true })).toBeVisible();
      }
      const quiet = startScreen(page).locator(".vibe-button-quiet:not(.vibe-example-chip)").filter({ visible: true });
      for (const button of await quiet.all()) {
        await page.mouse.move(0, 0);
        await expect(button).toHaveCSS("padding-left", "0px");
        await expect(button).toHaveCSS("text-decoration-line", "none");
        expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
        await button.hover();
        await expect(button).toHaveCSS("text-decoration-line", "underline");
      }
      if (door === "build") {
        const chips = startScreen(page).locator(".vibe-example-chip");
        const ys = await chips.evaluateAll(elements => elements.map(element => element.getBoundingClientRect().y));
        expect(ys[0]).toBe(ys[1]);
        expect(await chips.evaluateAll(elements => elements.every(element => element.getBoundingClientRect().right <= innerWidth))).toBe(true);
      } else {
        const labels = startScreen(page).locator("label:not(.sr-only)");
        for (const label of await labels.all()) { await expect(label).toHaveCSS("font-weight", "600"); await expect(label).toHaveCSS("font-size", "14px"); }
        expect(await startScreen(page).locator("option").allTextContents()).not.toEqual(expect.arrayContaining([models.target]));
      }
      await startScreen(page).getByRole("button", { name: "Back", exact: true }).click();
      await entryReady(page);
    }
  }
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});

test("the Build result keeps 32px before the trial and one visible data-use notice as Guide opens and closes", async ({ page }) => {
  const fake = await toResults(page);
  const trial = page.getByRole("region", { name: "Try your agent", exact: true });
  const result = page.locator('.vibe-project-main article[aria-label="Run results"]').first();
  const distance = async () => (await trial.boundingBox())!.y - ((await result.boundingBox())!.y + (await result.boundingBox())!.height);
  await expect.poll(distance).toBeGreaterThanOrEqual(32);
  const guide = page.getByRole("button", { name: "Guide", exact: true });
  if (await guide.getAttribute("aria-pressed") !== "true") await guide.click();
  await expect(page.locator(".vibe-data-use-notice").filter({ visible: true })).toHaveCount(1);
  await expect(page.locator(".vibe-guide-data-use")).toBeVisible();
  await page.getByRole("button", { name: "Close Guide", exact: true }).click();
  await expect(page.locator(".vibe-data-use-notice").filter({ visible: true })).toHaveCount(1);
  await expect(trial.locator(".vibe-data-use-notice")).toBeVisible();
  expect(fake.unknown).toEqual([]); expect(fake.errors).toEqual([]);
});
