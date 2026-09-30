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

// Direct-paste/retest controls belonged to the earlier workflow; current Improve keeps those chats as a read-only archive.
const archived: Session = {
  id: "legacy-chats", revision: 2, event_cursor: 2, anonymous: true,
  document: {
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
      return route.fulfill({ status: 405, headers, json: { error: { message: "Archived fixture is read-only" } } });
    }
    if (path.endsWith("/config")) return route.fulfill({ headers, json: { enabled: true, two_door: true, defaults: defaultModels, models: [] } });
    if (path.endsWith("/sessions")) return route.fulfill({ headers, json: [archived] });
    if (path.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(archived)}\n\n` });
    if (path.endsWith("/case")) {
      const id = path.split("/operations/")[1]?.split("/")[0];
      const key = url.searchParams.get("key");
      caseReads.push(`${id}:${key}`);
      const result = archived.operations.find(item => item.id === id)?.results.find(item => item.case_key === key);
      return route.fulfill({ status: result ? 200 : 404, headers, json: result || { error: { message: "Case missing" } } });
    }
    if (path.endsWith("/saved-checks")) return route.fulfill({ headers, json: [] });
    return route.fulfill({ headers, json: archived });
  });
  await page.goto("/vibe-evals?session=legacy-chats");
  await expect(page.getByRole("heading", { name: "Saved conversation" })).toBeVisible({ timeout: 15_000 });
  return { writes, caseReads };
}

test("legacy saved chats retain both historical scorecards and evidence without writes", async ({ page }) => {
  const before = structuredClone(archived);
  const { writes, caseReads } = await serve(page);
  await expect(page.getByText("This earlier conversation is read-only.", { exact: false })).toBeVisible();
  await expect(page.getByText("Check our support agent against the 30-day policy.")).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Check the new answer" })).toHaveCount(0);
  const scorecards = page.locator("details").filter({ has: page.locator("summary", { hasText: "Saved results · 1 passed · 1 failed" }) });
  await expect(scorecards).toHaveCount(2);
  await scorecards.nth(0).locator(":scope > summary").click();
  await scorecards.nth(1).locator(":scope > summary").click();
  await expect(scorecards.nth(0).getByText("Remember a purchase")).toBeVisible();
  await expect(scorecards.nth(1).getByText("Outside the return window")).toBeVisible();
  await scorecards.nth(0).locator(".vibe-result-row").first().locator("summary").click();
  await scorecards.nth(1).locator(".vibe-result-row").nth(1).locator("summary").click();
  await expect(scorecards.nth(0)).toContainText("Asks for a purchase age the customer already supplied.");
  await expect(scorecards.nth(1)).toContainText("Allows a return after 45 days.");
  await expect(scorecards.nth(0)).toContainText("When did you buy it?");
  await expect(scorecards.nth(1)).toContainText("It is eligible.");
  expect(caseReads).toEqual(expect.arrayContaining(["recorded-1:chat-1", "recorded-2:chat-2"]));
  expect(writes).toEqual([]);
  expect(archived).toEqual(before);
  await page.reload();
  await expect(page.getByRole("heading", { name: "Saved conversation" })).toBeVisible();
  await expect(page.locator("details > summary").filter({ hasText: "Saved results · 1 passed · 1 failed" })).toHaveCount(2);
  expect(writes).toEqual([]);
});
