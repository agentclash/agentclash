import { expect, test, type Page } from "@playwright/test";
import { defaultModels, type Session } from "../../src/lib/vibe";

async function conversation(page: Page, content: string) {
  const state: Session = {
    id: "build-progress", revision: 1, anonymous: true, event_cursor: 1,
    document: {
      format_version: 1, models: defaultModels, requirements: [], artifacts: [],
      evaluation: { id: "build-progress", chat_id: "build-progress", door: "build" },
      build: { cycle_id: "cycle", phase: "preparing", clarifications_used: 0 },
      messages: [{ id: "request", role: "user", content, operation_id: "turn" }],
    },
    operations: [{ id: "turn", kind: "message", state: "QUEUED", billing: "RESERVED", models: defaultModels,
      max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }],
  };
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.route("**/v1/vibe/**", async route => {
    const path = new URL(route.request().url()).pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, availability: { guest: { available: true }, workspace: { available: true } }, two_door: true, defaults: defaultModels, models: [...new Set(Object.values(defaultModels))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })) } });
    if (path.endsWith("/saved-checks")) return route.fulfill({ headers, json: [] });
    if (path.endsWith("/sessions")) return route.fulfill({ headers, json: [state] });
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
    return route.fulfill({ headers, json: state });
  });
  const show = async () => {
    await page.goto("/vibe-evals?session=build-progress");
    await expect(page.locator('[data-message-id="request"]')).toContainText(content);
  };
  await show();
  return { state, show, errors };
}

for (const [width, content] of [[1440, "hhui"], [390, "let’s drink vodka yay"]] as const) {
  test(`casual chat has neutral progress and no build promise at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 900 });
    const { state, show, errors } = await conversation(page, content);
    const turn = state.operations[0];
    for (const phase of ["QUEUED", "RUNNING", "FINALIZING"] as const) {
      turn.state = phase;
      if (phase === "FINALIZING") turn.conversation_decision = { intent: "chat", source_message_id: "request" };
      await show();
      await expect(page.getByRole("status").filter({ hasText: phase === "FINALIZING" ? "Saving your reply…" : "Thinking…" })).toBeVisible();
      await expect(page.locator(".vibe-build-intro")).toHaveCount(0);
      await expect(page.getByText(/Creating your agent|Waiting to start|Saving your results/)).toHaveCount(0);
      await expect(page.getByRole("button", { name: "Stop", exact: true })).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
    await page.screenshot({ path: info.outputPath("casual-chat-progress.png") });
    turn.state = "COMPLETED";
    state.document.build!.phase = "waiting";
    state.document.messages.push({ id: "reply", role: "assistant", content: "Hi! Tell me what you’d like AI to help with.", operation_id: "turn" });
    await show();
    await expect(page.getByText("Hi! Tell me what you’d like AI to help with.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Stop", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Use a sample policy" })).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test("a real task announces a prototype after the server confirms the action", async ({ page }) => {
  const { state, show, errors } = await conversation(page, "Sort customer emails.");
  await expect(page.locator(".vibe-build-intro")).toHaveCount(0);
  state.operations[0].state = "RUNNING";
  state.operations[0].conversation_decision = { intent: "prepare_tests", source_message_id: "request" };
  await show();
  await expect(page.locator(".vibe-build-intro")).toContainText("I’m making a first version");
  await expect(page.getByRole("status").filter({ hasText: "Understand the task" })).toBeVisible();
  expect(errors).toEqual([]);
});

for (const width of [1440, 390]) test(`first Build shows each server stage and one Stop at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  const { state, show, errors } = await conversation(page, "Sort customer emails using our support rules.");
  const turn = state.operations[0]; turn.state = "RUNNING"; turn.created_at = new Date(Date.now() - 65000).toISOString();
  turn.conversation_decision = { intent: "prepare_tests", source_message_id: "request" };
  for (const [phase, label] of [["understanding", "Understand the task"], ["preparing", "Write the instructions"], ["reviewing", "Write 3 tests"], ["repairing", "Write 3 tests"]]) {
    turn.progress = { phase, completed_cases: 0, total_cases: 0 }; await show();
    const strip = page.locator(".vibe-progress-strip[data-active]");
    await expect(strip.getByRole("status")).toHaveText(label); await expect(strip.getByRole("list", { name: "Build stages" }).getByRole("listitem")).toHaveCount(4);
    const stages = strip.getByRole("list", { name: "Build stages" }).getByRole("listitem");
    const index = phase === "understanding" ? 0 : phase === "preparing" ? 1 : 2;
    await expect(stages.nth(index)).toHaveAttribute("aria-current", "step");
    expect(await stages.nth(index).evaluate(stage => getComputedStyle(stage).fontWeight)).toBe("600");
    expect(await stages.nth(index).evaluate(stage => getComputedStyle(stage).textDecorationLine)).not.toContain("underline");
    for (let done = 0; done < index; done++) { await expect(stages.nth(done)).toContainText("✓"); await expect(stages.nth(done)).toContainText("Done:"); }
    await expect(stages.nth(index + 1)).toHaveAttribute("data-state", "upcoming");
    await expect(strip).toContainText(/Elapsed 1:/); await expect(page.getByRole("button", { name: "Stop", exact: true })).toHaveCount(1);
    await expect(strip).toContainText("Stop keeps finished results.");
  }
  expect(errors).toEqual([]);
});
