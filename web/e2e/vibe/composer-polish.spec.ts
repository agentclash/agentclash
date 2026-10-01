import { expect, test } from "@playwright/test";
import type { Session } from "../../src/lib/vibe";

test.use({ video: { mode: "on", size: { width: 1440, height: 960 } } });

test("responsive input, draft preservation and truthful shining status", async ({ page }, info) => {
  test.setTimeout(60_000);
  const models = { assistant: "fixture/free", target: "fixture/free", evaluator: "fixture/free" };
  const state: Session = {
    id: "polish-fixture", revision: 1, anonymous: true,
    document: { test_journey: true, models, requirements: [],
      messages: [{ id: "one", role: "user", content: "My agent handles returns." }, { id: "two", role: "assistant", content: "I’ll check eligible returns and missing information.", artifact_id: "tests" }],
      artifacts: [{
        id: "tests", kind: "test_suite", title: "Returns tests", summary: "Return eligibility", agent_prompt: "Follow the return policy.", accepted: false, source_message_id: "one", proposal_message_id: "two",
        blueprint: { cases: [{ key: "first", payload: { question: "Can I return this?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Ask for age and condition." }] }] },
      }],
    },
    operations: [{ id: "thinking", state: "RUNNING", kind: "message", models, billing: "RESERVED", max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [], conversation_decision: { intent: "chat", source_message_id: "three" } }],
  };
  const errors: string[] = [];
  page.on("pageerror", e => errors.push(e.message));
  await page.route("**/v1/vibe/**", async route => {
    const path = new URL(route.request().url()).pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, free_only: true, defaults: models, models: [{ id: models.target, name: "Fixture (free)" }] } });
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
    return route.fulfill({ headers, json: state });
  });
  await page.setViewportSize({ width: 1440, height: 960 });
  await page.goto("/vibe-evals?session=polish-fixture");
  const input = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await expect(page.locator('[role="status"]').filter({ hasText: "Thinking…" })).toBeVisible();
  await expect(page.getByText("Preparing your tests…", { exact: true })).not.toBeVisible();
  const shine = page.locator(".vibe-status-shine");
  await expect(shine).toBeVisible();
  const position = await shine.evaluate(el => getComputedStyle(el).backgroundPosition);
  await expect.poll(() => shine.evaluate(el => getComputedStyle(el).backgroundPosition)).not.toBe(position);
  // An intentional motion-review capture, not a synchronization wait.
  await page.waitForTimeout(30_000);
  await page.getByRole("button", { name: "Pause status animation" }).click();
  await expect(shine).toHaveCount(0);
  await page.getByRole("button", { name: "Resume status animation" }).click();
  for (const width of [320, 390, 768, 1280, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await input.fill("Please keep the purchase age and condition rules. ".repeat(40));
    await expect.poll(() => input.evaluate(el => el.clientHeight)).toBeLessThanOrEqual(160);
    const box = await input.boundingBox(), send = await page.getByRole("button", { name: "Send message" }).boundingBox();
    expect(box!.x + box!.width).toBeLessThanOrEqual(send!.x);
    expect(send!.width).toBeGreaterThanOrEqual(44);
    expect(send!.height).toBeGreaterThanOrEqual(44);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await input.fill("Keep the original 30-day rule.");
    await expect.poll(() => input.evaluate(el => el.clientHeight)).toBeLessThan(110);
    await page.screenshot({ path: info.outputPath(`composer-${width}.png`), fullPage: true });
  }
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect(shine).not.toBeVisible();
  await page.emulateMedia({ reducedMotion: "no-preference", forcedColors: "active" });
  await expect(shine).not.toBeVisible();
  await page.emulateMedia({ forcedColors: "none" });
  // Browser reflow at a 1280px screen / 400% zoom is a 320 CSS-pixel viewport.
  // Actual Safari zoom and physical IME remain workstation checks.
  await page.setViewportSize({ width: 320, height: 800 });
  await page.addStyleTag({ content: ".vibe-workspace { font-size: 200%; } .vibe-composer textarea { font-size: 32px; }" });
  await input.fill("A longer message with enlarged text that wraps naturally.");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(errors).toEqual([]);
});
