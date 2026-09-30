import { expect, test, type Page } from "@playwright/test";
import { defaultModels, type CaseResult, type EvidenceSet, type Operation, type Session } from "../../src/lib/vibe";

const artifactID = "a129a2a3-246a-42f1-9b11-cba998eaf153";
const titles = ["Remember a purchase", "Outside the return window"];
const customer = ["I bought it 10 days ago.", "I bought it 45 days ago, unopened."];

function evidence(id: string, corrected: boolean): EvidenceSet {
  return {
    id, label: corrected ? "Updated replies" : "Pasted conversations", raw: corrected ? "Updated saved transcript" : "Original saved transcript",
    conversations: titles.map((title, index) => ({
      key: `chat-${index + 1}`, title,
      messages: [
        { id: `c${index + 1}-m1`, role: "user" as const, content: customer[index] },
        { id: `c${index + 1}-m2`, role: "assistant" as const, content: corrected ? "It is eligible." : index === 0 ? "When did you buy it?" : "It is outside the 30-day window." },
      ],
    })),
  };
}

const original = evidence("evidence-1", false);
const updated = evidence("evidence-2", true);

function results(set: EvidenceSet, corrected: boolean): CaseResult[] {
  return set.conversations.map((conversation, index) => {
    const verdict = (corrected ? index === 0 : index === 1) ? "PASS" : "FAIL";
    const explanation = corrected
      ? index === 0 ? "Uses the previously supplied purchase age." : "Allows a return after 45 days."
      : index === 0 ? "Asks for a purchase age the customer already supplied." : "Correctly declines the late return.";
    return {
      case_key: conversation.key, title: conversation.title, version: artifactID,
      input: { conversation: conversation.title }, output: conversation.messages[1].content,
      messages: conversation.messages, expected: "Remember purchase details and use the 30-day return window.",
      verdict, checks: [{ key: "rule", verdict, evidence: explanation, message_ids: conversation.messages.map(message => message.id) }],
    };
  });
}

function operation(id: string, set: EvidenceSet, corrected: boolean): Operation {
  return {
    id, kind: corrected ? "retest" : "check", state: "COMPLETED", billing: "RELEASED", models: defaultModels,
    max_cost_nano_usd: 0, actual_cost_nano_usd: 0, baseline_id: corrected ? "recorded-1" : undefined,
    source: { kind: "provided_conversations", label: "Pasted conversations", artifact_id: artifactID,
      evidence_set_id: set.id, comparison: corrected ? "updated_replies" : undefined },
    results: results(set, corrected),
    scorecard: { total: 2, passed: 1, failed: 1, unknown: 0, evaluated: 2, coverage: 1, pass_rate: 0.5 },
  };
}

// Historical recorded results inside a V1 project survive retirement. They use
// the current Results pane, not the removed pre-V1 archive renderer.
const project: Session = {
  id: "recorded-project", revision: 2, event_cursor: 2, anonymous: true,
  document: {
    format_version: 1, test_journey: true,
    evaluation: { id: "recorded-project", chat_id: "recorded-project", door: "test" },
    models: defaultModels, requirements: [], evidence_sets: [original, updated], active_evidence_id: updated.id,
    artifacts: [{ id: artifactID, kind: "conversation_evaluation", title: "Return policy chat", agent_prompt: "Follow the 30-day return policy.", blueprint: {}, accepted: true, source_message_id: "request", conversation_evaluation: { evidence_set_id: original.id, expectations: [{ id: "rule", statement: "Use known purchase age and apply the policy." }] } }],
    messages: [
      { id: "request", role: "user", content: "Check our support agent against the 30-day policy.", operation_id: "prepare" },
      { id: "prepared", role: "assistant", content: "Saved conversations and checks are available below.", operation_id: "prepare" },
    ],
  },
  operations: [operation("recorded-1", original, false), operation("recorded-2", updated, true)],
};

async function serve(page: Page) {
  const writes: string[] = [];
  const caseReads: string[] = [];
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" };
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS" } });
    if (request.method() !== "GET") {
      writes.push(`${request.method()} ${path}`);
      return route.fulfill({ status: 405, headers, json: { error: { message: "Reading history must not mutate the project" } } });
    }
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, two_door: true, defaults: defaultModels, models: [] } });
    if (path.endsWith("/sessions")) return route.fulfill({ headers, json: [project] });
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(project)}\n\n` });
    if (path.endsWith("/case")) {
      const id = path.split("/operations/")[1]?.split("/")[0];
      const key = url.searchParams.get("key");
      caseReads.push(`${id}:${key}`);
      const result = project.operations.find(item => item.id === id)?.results.find(item => item.case_key === key);
      return route.fulfill({ status: result ? 200 : 404, headers, json: result || { error: { message: "Case missing" } } });
    }
    if (path.endsWith("/saved-checks")) return route.fulfill({ headers, json: [] });
    return route.fulfill({ headers, json: project });
  });
  await page.goto("/vibe-evals?session=recorded-project&view=checks");
  await expect(page.getByRole("combobox", { name: "Result history" })).toBeVisible({ timeout: 15_000 });
  return { writes, caseReads };
}

test("V1 recorded replies and historical result selection survive refresh without writes", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  const before = structuredClone(project);
  const { writes, caseReads } = await serve(page);
  const history = page.getByRole("combobox", { name: "Result history" });
  await expect(history).toHaveValue("recorded-2");
  await history.selectOption("recorded-1");
  await expect(page).toHaveURL(/run=recorded-1/);
  await expect(page.getByRole("article", { name: "Evaluation scorecard" })).toContainText("1 passed · 1 failed");
  const row = page.locator(".vibe-result-row").first();
  if (await row.getAttribute("open") === null) await row.locator("summary").click();
  await expect(row.getByText("When did you buy it?", { exact: true }).first()).toBeVisible();
  await history.selectOption("recorded-2");
  if (await row.getAttribute("open") === null) await row.locator("summary").click();
  await expect(row.getByText("It is eligible.", { exact: true }).first()).toBeVisible();
  await history.selectOption("recorded-1");
  await page.reload();
  await expect(history).toHaveValue("recorded-1");
  await expect(page).toHaveURL(/run=recorded-1/);
  await page.getByRole("tab", { name: "Conversation", exact: true }).click();
  await page.reload();
  await expect(page.getByRole("tab", { name: "Conversation", exact: true })).toHaveAttribute("aria-selected", "true");
  await page.getByRole("tab", { name: "Results", exact: true }).click();
  await expect(history).toHaveValue("recorded-1");
  expect(caseReads).toEqual(expect.arrayContaining(["recorded-1:chat-1", "recorded-2:chat-2"]));
  expect(writes).toEqual([]);
  expect(project).toEqual(before);
});
