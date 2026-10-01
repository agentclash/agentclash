import { expect, test } from "@playwright/test";
import type { Artifact, Operation, Session } from "../../src/lib/vibe";

// These fixtures verify browser recovery, not live provider quality or billing.
test("failed test updates survive chat and reload, and retry without replacing the next message", async ({ page }) => {
  const models = { assistant: "fixture/free", target: "fixture/free", evaluator: "fixture/free" };
  const failed: Operation = {
    id: "failed-update", kind: "message", state: "FAILED", billing: "SETTLED", models,
    max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [], retryable: true,
    conversation_decision: { intent: "edit_tests", source_message_id: "update-request" },
    error: { code: "test_policy_conflict", message: "The generated expectation conflicts with your new return policy." },
  };
  const suite: Artifact = {
    id: "original-tests", kind: "test_suite", title: "Returns", summary: "Return eligibility.",
    accepted: true, agent_prompt: "Follow the supplied return policy.", source_message_id: "description", proposal_message_id: "prepared",
    validation: { status: "supported", blueprint_hash: "old-tests", policy_hash: "old-policy", validator_version: "v1" },
    blueprint: { cases: [{ key: "boundary", payload: { question: "Unopened, bought 30 days ago. Can I return it?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Eligible under the 30-day policy." }] }], judges: [{ context_from: ["case.expectations.expected_behavior"] }] },
  };
  const state: Session = {
    id: "recovery-fixture", revision: 3, event_cursor: 3, anonymous: true,
    document: { test_journey: true, models, requirements: [], artifacts: [suite], messages: [
      { id: "description", role: "user", content: "Test unopened returns within 30 days." },
      { id: "prepared", role: "assistant", content: "1 test is ready.", artifact_id: suite.id },
      { id: "update-request", role: "user", content: "Our return window is now 14 days. Update the tests.", operation_id: failed.id },
      { id: "coffee", role: "user", content: "I am getting coffee." },
      { id: "coffee-reply", role: "assistant", content: "Enjoy your coffee." },
    ], pending_policy_changes: [{ operation_id: failed.id, source_message_id: "update-request", artifact_id: suite.id, status: "pending", message: "The policy update has not been applied." }] },
    operations: [failed, { ...failed, id: "chat-operation", state: "COMPLETED", error: undefined, retryable: false }],
  };
  const retries: Record<string, unknown>[] = [];
  const messages: unknown[] = [];
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS" } });
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, defaults: models, models: [] } });
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
    if (path.endsWith("/messages")) {
      messages.push(request.postDataJSON());
      return route.abort();
    }
    if (path.endsWith("/retry")) {
      expect(path).toBe(`/v1/vibe/sessions/${state.id}/operations/${failed.id}/retry`);
      retries.push(request.postDataJSON());
      if (retries.length === 1) {
        state.revision++;
        state.event_cursor = state.revision;
        state.document.messages.push({ id: "later", role: "assistant", content: "The conversation is still available." });
        return route.abort("failed");
      }
      const updated: Artifact = {
        ...suite, id: "updated-tests", parent_id: suite.id, source_message_id: "update-request", proposal_message_id: "updated-reply",
        validation: { ...suite.validation!, blueprint_hash: "new-tests", policy_hash: "new-policy" },
        blueprint: { ...suite.blueprint as object, cases: [{ key: "boundary", payload: { question: "Unopened, bought 30 days ago. Can I return it?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Ineligible under the 14-day policy." }] }] },
      };
      const retry: Operation = {
        ...failed, id: "successful-retry", retry_of_operation_id: failed.id, state: "COMPLETED", error: undefined, retryable: false,
        completion_receipt: { action: "edit_tests", source_message_id: "update-request", artifact_id: updated.id, case_count: 1, changed_case_count: 1 },
      };
      state.operations.push(retry);
      state.document.artifacts.push(updated);
      state.document.pending_policy_changes![0].status = "applied";
      state.document.messages.push({ id: "updated-reply", role: "assistant", content: "Updated 1 test.", operation_id: retry.id, artifact_id: updated.id });
      state.event_cursor = ++state.revision;
      return route.fulfill({ headers, json: retry });
    }
    return route.fulfill({ headers, json: state });
  });
  await page.goto(`/vibe-evals?session=${state.id}`);
  const failedTurn = page.locator('[data-message-id="update-request"]');
  await expect(failedTurn.getByRole("alert")).toHaveText(failed.error!.message);
  await expect(page.getByText("Enjoy your coffee.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry", exact: true })).toHaveCount(1);
  await page.getByText("Review your tests", { exact: true }).click();
  await expect(page.getByRole("heading", { name: "1 test from your previous policy" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Run previous tests" })).toBeEnabled();
  await page.reload();
  await expect(failedTurn.getByRole("alert")).toHaveText(failed.error!.message);
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("A separate question that must stay here.");
  await failedTurn.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(failedTurn.getByText(/acknowledgement wasn’t confirmed/)).toBeVisible();
  await expect(page.getByText("The conversation is still available.")).toBeVisible();
  await failedTurn.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(failedTurn.getByRole("status")).toHaveText("Completed on retry.");
  await expect(page.getByText("Updated 1 test.", { exact: true })).toBeVisible();
  await expect(composer).toHaveValue("A separate question that must stay here.");
  expect(retries).toHaveLength(2);
  expect(retries[1]).toEqual(retries[0]);
  expect(retries[0].revision).toBe(3);
  expect(messages).toHaveLength(0);
  await expect(failedTurn).toHaveCount(1);
  await page.reload();
  await expect(page.getByText("Completed on retry.", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry", exact: true })).toHaveCount(0);
  expect(errors).toEqual([]);
});
