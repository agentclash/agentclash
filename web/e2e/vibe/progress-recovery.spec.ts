import { expect, test, type Page } from "@playwright/test";
import type { Session } from "../../src/lib/vibe";

function fixture(): Session {
  const models = { assistant: "fixture/free", target: "fixture/free", evaluator: "fixture/free" };
  return {
    id: "phase-six", revision: 1, event_cursor: 1, anonymous: true,
    document: { test_journey: true, models, requirements: [], artifacts: [], messages: [
      { id: "request", role: "user", content: "Prepare return tests.", operation_id: "request-op" },
    ] },
    operations: [{ id: "request-op", kind: "message", state: "FAILED", billing: "RECONCILING", models,
      max_cost_nano_usd: 100, actual_cost_nano_usd: null, results: [], retryable: true,
      error: { code: "provider_rate_limit", message: "The selected model’s provider is busy. Your request is saved.", retry_available_at: new Date(Date.now() + 10_000).toISOString() },
    }],
  };
}

async function serve(page: Page, state: Session) {
  const control = { disconnected: false, posts: [] as unknown[], snapshots: 0 };
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request(), path = new URL(request.url()).pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS" } });
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, defaults: state.document.models, models: [] } });
    if (request.method() === "POST") {
      control.posts.push(request.postDataJSON());
      state.operations[0].retryable = false;
      state.event_cursor!++;
      return route.fulfill({ headers, json: state.operations[0] });
    }
    state.server_time = new Date().toISOString();
    if (path.endsWith("/events")) {
      if (control.disconnected) return route.abort("failed");
      control.snapshots++;
      return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
    }
    return route.fulfill({ headers, json: state });
  });
  return control;
}

test("provider cooldown survives reload, enables a manual retry and preserves the next message", async ({ page }, info) => {
  const state = fixture(), control = await serve(page, state);
  const errors: string[] = []; page.on("pageerror", e => errors.push(e.message));
  await page.goto(`/vibe-evals?session=${state.id}`);
  const retry = page.getByRole("button", { name: /Try again/ });
  await expect(retry).toBeDisabled();
  await page.reload();
  await expect(retry).toBeDisabled();
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("Keep this separate question.");
  for (const width of [320, 390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await expect(retry).toBeEnabled({ timeout: 15_000 });
  expect(control.posts).toHaveLength(0);
  await expect(composer).toBeFocused();
  await page.screenshot({ path: info.outputPath("retry-ready.png"), fullPage: true });
  await retry.click();
  await expect.poll(() => control.posts.length).toBe(1);
  await expect(composer).toHaveValue("Keep this separate question.");
  expect(errors).toEqual([]);
});

test("stream disconnect preserves dirty tests, the next message and keyboard focus", async ({ page }) => {
  const state = fixture();
  state.operations = [];
  state.document.artifacts = [{ id: "suite", kind: "test_suite", title: "Returns", source_message_id: "request", proposal_message_id: "prepared", agent_prompt: "Follow the policy.", accepted: true,
    blueprint: { judges: [{ context_from: ["case.expectations.expected_behavior"] }], cases: [{ key: "return", payload: { question: "Can I return it?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Ask for the purchase age." }] }] } }];
  state.document.messages.push({ id: "prepared", role: "assistant", content: "1 test is ready.", artifact_id: "suite" });
  const control = await serve(page, state);
  await page.goto(`/vibe-evals?session=${state.id}`);
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("My next question stays here.");
  const review = page.locator("summary").filter({ hasText: /^Review your tests$/ });
  if (await review.isVisible()) await review.click();
  await page.getByRole("button", { name: "Edit tests", exact: true }).click();
  const expected = page.getByRole("textbox", { name: "What should happen", exact: true });
  await expected.fill("Ask only for the missing purchase age.");
  control.disconnected = true;
  await expect(page.getByText("Reconnecting to saved progress…", { exact: true })).toBeVisible({ timeout: 10_000 });
  await expect(expected).toHaveValue("Ask only for the missing purchase age.");
  await expect(expected).toBeFocused();
  const before = control.snapshots;
  control.disconnected = false;
  state.event_cursor!++;
  await expect.poll(() => control.snapshots).toBeGreaterThan(before);
  await expect(page.getByText("Reconnecting to saved progress…", { exact: true })).toHaveCount(0);
  await expect(expected).toHaveValue("Ask only for the missing purchase age.");
  await expect(expected).toBeFocused();
  await expect(composer).toHaveValue("My next question stays here.");
  await expect(page.getByRole("button", { name: "Run 1 test", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Save test changes", exact: true })).toBeEnabled();
  expect(control.posts).toHaveLength(0);
});
