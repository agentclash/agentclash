import { expect, test } from "@playwright/test";
import type {
  CaseResult,
  EvidenceSet,
  Operation,
  Session,
} from "../../src/lib/vibe";

const models = {
  assistant: "test/assistant",
  target: "unused/target",
  evaluator: "test/evaluator",
};
const artifactID = "a129a2a3-246a-42f1-9b11-cba998eaf153";
const brief =
  "Our support agent should follow a 30-day return policy and remember details across follow-ups.";
const expectations = [
  {
    id: "rule",
    statement: "Use the details already supplied and apply the 30-day policy.",
  },
];
const original = `Customer: I bought it 10 days ago.
Agent: Is it unopened?
Customer: Yes.
Agent: When did you buy it?
---
Customer: I bought it 45 days ago, unopened.
Agent: It is outside the 30-day window.`;
const updated = original
  .replace("When did you buy it?", "It is eligible.")
  .replace("It is outside the 30-day window.", "It is eligible.");
const firstMessage = `${brief}\n\n${original}`;

function savedEvidence(raw: string, isUpdate = false): EvidenceSet {
  return {
    id: isUpdate ? "evidence-2" : "evidence-1",
    label: "Pasted conversations",
    raw,
    conversations: [
      {
        key: "chat-1",
        title: "Remember a purchase",
        messages: [
          { id: "c1-m1", role: "user", content: "I bought it 10 days ago." },
          { id: "c1-m2", role: "assistant", content: "Is it unopened?" },
          { id: "c1-m3", role: "user", content: "Yes." },
          {
            id: "c1-m4",
            role: "assistant",
            content: isUpdate ? "It is eligible." : "When did you buy it?",
          },
        ],
      },
      {
        key: "chat-2",
        title: "Outside the return window",
        messages: [
          {
            id: "c2-m1",
            role: "user",
            content: "I bought it 45 days ago, unopened.",
          },
          {
            id: "c2-m2",
            role: "assistant",
            content: isUpdate
              ? "It is eligible."
              : "It is outside the 30-day window.",
          },
        ],
      },
    ],
  };
}

test("direct paste checks saved chats, copies a grounded fix and exposes a regression in new answers", async ({
  page,
}) => {
  const state: Session = {
    id: "conversation-browser",
    revision: 0,
    anonymous: true,
    document: {
      models,
      messages: [],
      requirements: [],
      artifacts: [],
      evaluation_first: true,
      evidence_sets: [],
    },
    operations: [],
  };
  const requests: Record<string, unknown>[] = [];
  let uploads = 0;
  const edits: unknown[] = [];
  const copied: string[] = [];
  await page.exposeFunction("recordCopiedText", (text: string) => {
    copied.push(text);
  });
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: (text: string) =>
          (window as unknown as {
            recordCopiedText: (value: string) => Promise<void>;
          }).recordCopiedText(text),
      },
    });
  });
  const snapshot = () => ({
    ...state,
    event_cursor: state.revision,
    operations: state.operations.map((o) => ({
      ...o,
      results: o.results.map((r) => ({
        ...r,
        messages: undefined,
        checks: r.checks.map((c) => ({
          ...c,
          evidence: "",
          message_ids: undefined,
        })),
      })),
    })),
  });
  await page.route("**/v1/vibe/**", async (route) => {
    const req = route.request(),
      path = new URL(req.url()).pathname;
    const headers = {
      "Access-Control-Allow-Origin": new URL(page.url()).origin,
      "Access-Control-Allow-Credentials": "true",
    };
    const send = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        headers,
        contentType: "application/json",
        body: JSON.stringify(body),
      });
    if (req.method() === "OPTIONS")
      return route.fulfill({
        status: 204,
        headers: {
          ...headers,
          "Access-Control-Allow-Headers": "Content-Type,Authorization,If-Match",
          "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS",
        },
      });
    if (path.endsWith("/config"))
      return send({
        enabled: true,
        free_only: true,
        defaults: models,
        models: [],
      });
    if (path.endsWith("/events"))
      return route.fulfill({
        headers,
        contentType: "text/event-stream",
        body: `event: snapshot\ndata: ${JSON.stringify(snapshot())}\n\n`,
      });
    if (path.endsWith("/case")) {
      const operation = state.operations.find((o) => path.includes(o.id));
      return send(
        operation?.results.find(
          (c) => c.case_key === new URL(req.url()).searchParams.get("key"),
        ),
      );
    }
    if (req.method() === "POST" && path.endsWith("/sessions")) {
      state.id = req.postDataJSON().id;
      return send(snapshot(), 201);
    }
    if (req.method() === "POST" && path.endsWith("/evidence")) {
      const body = req.postDataJSON();
      uploads++;
      expect(uploads).toBe(1);
      expect(body.content).toBe(updated);
      const evidence = savedEvidence(body.content, true);
      expect(evidence.conversations.map((chat) =>
        chat.messages.filter((message) => message.role === "user"),
      )).toEqual(state.document.evidence_sets![0].conversations.map((chat) =>
        chat.messages.filter((message) => message.role === "user"),
      ));
      state.document.evidence_sets!.push(evidence);
      state.document.active_evidence_id = evidence.id;
      state.revision++;
      return send(snapshot());
    }
    if (req.method() === "POST" && path.endsWith("/messages")) {
      const body = req.postDataJSON();
      requests.push(body);
      state.revision++;
      if (body.kind === "message") {
        expect(body.content).toBe(firstMessage);
        expect(body.quick_check).toBe(true);
        expect(state.document.artifacts).toHaveLength(0);
        state.document.messages.push({
          id: body.client_id,
          role: "user",
          content: body.content,
        });
        // The authoring response saves the pasted evidence and explicitly grants
        // quick_check. The browser must continue with one separate check request.
        const evidence = savedEvidence(body.content);
        state.document.evidence_sets!.push(evidence);
        state.document.active_evidence_id = evidence.id;
        state.document.artifacts.push({
          id: artifactID,
          kind: "conversation_evaluation",
          title: "Support quality",
          agent_prompt: "",
          blueprint: null,
          accepted: false,
          quick_check: true,
          source_message_id: body.client_id,
          conversation_evaluation: {
            evidence_set_id: evidence.id,
            expectations: structuredClone(expectations),
          },
        });
        state.document.messages.push({
          id: "prepared",
          artifact_id: artifactID,
          role: "assistant",
          content:
            "I’ll check these replies against your return policy and whether the agent remembers details in follow-ups.",
        });
        return send({ id: "message", state: "COMPLETED" }, 202);
      }
      expect(["check", "retest"]).toContain(body.kind);
      expect(body.artifact_id).toBe(artifactID);
      expect(body.approve_artifact).toBe(true);
      const evidence = state.document.evidence_sets!.find(
        (e) => e.id === body.evidence_set_id,
      )!;
      const repeat = body.kind === "retest";
      if (repeat) {
        expect(body.baseline_id).toBe("recorded-1");
        expect(body.models.evaluator).toBe(models.evaluator);
        expect(body.evidence_set_id).toBe("evidence-2");
      } else {
        expect(body.baseline_id).toBeUndefined();
        expect(body.evidence_set_id).toBe("evidence-1");
      }
      const artifact = state.document.artifacts[0];
      expect(artifact.conversation_evaluation!.expectations).toEqual(expectations);
      artifact.accepted = true;
      const results: CaseResult[] = evidence.conversations.map((c, i) => ({
        case_key: c.key,
        title: c.title,
        version: artifactID,
        input: { conversation: c.title },
        output: "",
        messages: c.messages,
        expected: "Remember purchase details and use the return window.",
        expectations: structuredClone(expectations),
        verdict: (repeat ? i === 1 : i === 0) ? "FAIL" : "PASS",
        checks: [
          {
            key: "rule",
            verdict: (repeat ? i === 1 : i === 0) ? "FAIL" : "PASS",
            evidence:
              i === 0
                ? repeat
                  ? "Uses the previously supplied purchase age."
                  : "Asks for a purchase age the customer already supplied."
                : repeat
                  ? "Allows a return after 45 days."
                  : "Correctly declines the late return.",
            message_ids: i === 0 ? ["c1-m1", "c1-m4"] : ["c2-m1", "c2-m2"],
          },
        ],
      }));
      const operation: Operation = {
        id: `recorded-${state.operations.length + 1}`,
        kind: body.kind,
        state: "COMPLETED",
        billing: "RELEASED",
        models,
        max_cost_nano_usd: 0,
        actual_cost_nano_usd: 0,
        baseline_id: body.baseline_id,
        source: {
          kind: "provided_conversations",
          label: evidence.label,
          artifact_id: artifactID,
          evidence_set_id: evidence.id,
          comparison: repeat ? "updated_replies" : undefined,
        },
        results,
        scorecard: {
          passed: 1,
          failed: 1,
          unknown: 0,
          total: 2,
          evaluated: 2,
          pass_rate: 0.5,
          coverage: 1,
        },
      };
      state.operations.push(operation);
      return send(operation, 202);
    }
    if (req.method() === "PATCH") edits.push(req.postDataJSON());
    return send(snapshot());
  });
  // This contract remains available when reopening an existing recorded-chat session.
  await page.goto(`/vibe-evals?session=${state.id}`);
  await expect(page.getByRole("heading", { name: "Check the AI in your app." })).toBeVisible();
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill(brief);
  await composer.press("Shift+Enter");
  await expect(composer).toHaveValue(brief + "\n");
  expect(requests).toHaveLength(0);
  await composer.fill(firstMessage);
  await composer.press("Enter");
  await expect(
    page.getByRole("heading", { name: "One thing to fix" }),
  ).toBeVisible();
  expect(requests.map((request) => request.kind)).toEqual(["message", "check"]);
  expect(uploads).toBe(0);
  const originalEvidence = structuredClone(state.document.evidence_sets![0]);
  const originalResults = structuredClone(state.operations[0].results);
  const scorecard = page.getByRole("article", { name: "Evaluation scorecard" });
  const leading = scorecard.locator('[aria-label="Leading finding"]');
  await expect(leading.locator(":scope > details")).toHaveAttribute("open", "");
  await expect(leading.locator("summary").first()).toContainText("Remember a purchase");
  await expect(
    leading.getByText("Asks for a purchase age the customer already supplied.", {
      exact: true,
    }).first(),
  ).toBeVisible();
  await expect(leading.locator("blockquote:visible")).toHaveCount(1);
  await expect(
    leading.locator("blockquote").getByText("When did you buy it?", { exact: true }).first(),
  ).toBeVisible();
  await expect(leading.getByRole("button", { name: "See evidence", exact: true })).toHaveAttribute("aria-expanded", "false");
  await expect(leading.getByText("Expected behavior", { exact: true })).toBeHidden();
  await expect(leading.getByRole("button", { name: "That’s not what I meant" })).toBeVisible();
  for (const summary of ["Other results · 1", "What was checked"]) {
    await expect(scorecard.locator("details").filter({
      has: page.locator("summary", { hasText: summary }),
    }).last()).not.toHaveAttribute("open");
  }
  await leading.getByRole("button", { name: "Copy fix prompt", exact: true }).click();
  await expect(leading.getByRole("status")).toContainText("Copied. Paste it into your coding tool");
  expect(copied).toHaveLength(1);
  expect(copied[0]).toContain("Asks for a purchase age the customer already supplied.");
  expect(copied[0]).toContain("I bought it 10 days ago.");
  expect(copied[0]).toContain("Is it unopened?");
  expect(copied[0]).toContain("When did you buy it?");
  expect(copied[0]).toContain(expectations[0].statement);
  expect(copied[0]).toContain("it does not establish a root cause");
  expect(copied[0]).toContain("No code or agent instructions have been changed");
  expect(copied[0]).toContain("Vibe Evals did not call the live app");
  expect(requests).toHaveLength(2);
  await scorecard.getByRole("button", { name: "Copy original inputs" }).click();
  await expect(scorecard.getByRole("status").filter({ hasText: "Original inputs copied" })).toBeVisible();
  expect(JSON.parse(copied[1])).toEqual({
    conversations: originalEvidence.conversations.map((chat) => ({
      messages: chat.messages.map(({ role, content }) => ({
        role,
        content: role === "assistant" ? "[Paste the new agent reply here]" : content,
      })),
    })),
  });
  await leading.getByRole("button", { name: "See evidence", exact: true }).click();
  await expect(leading.getByText("Full conversation · 4 messages", { exact: true })).toBeVisible();
  await expect(leading.locator("blockquote").getByText("I bought it 10 days ago.", { exact: true }).first()).toBeVisible();
  await expect(
    leading.getByText("Is it unopened?", { exact: true }),
  ).toBeVisible();
  await expect(leading.getByText("All checks · 1", { exact: true })).toBeVisible();
  await leading.getByRole("button", { name: "Hide evidence", exact: true }).click();
  await leading.getByRole("button", { name: "That’s not what I meant" }).click();
  await expect(
    page.getByRole("textbox", { name: "Message Vibe Evals" }),
  ).toHaveValue(/The correct expectation is:/);
  expect(requests).toHaveLength(2);
  expect(edits).toHaveLength(0);
  await page.getByRole("textbox", { name: "Message Vibe Evals" }).fill("");
  await page.getByRole("tab", { name: "Results", exact: true }).click();
  await scorecard.getByRole("button", { name: "Check the new answer", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Check the new answer", exact: true })).toBeVisible();
  await page
    .getByRole("textbox", { name: "Conversations to check" })
    .fill(updated);
  await page.getByRole("button", { name: "Check new answer", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "1 new issue in this update" }),
  ).toBeVisible();
  await expect(scorecard).toContainText(
    "1 previously failing chat now passes · 1 new failure",
  );
  await expect(scorecard).toContainText("Same expectations and evaluator. Same customer messages.");
  await expect(leading.locator("summary").first()).toContainText("Outside the return window");
  await expect(
    leading.getByText("Allows a return after 45 days.", { exact: true }).first(),
  ).toBeVisible();
  await expect(leading.locator("blockquote").getByText("It is eligible.", { exact: true }).first()).toBeVisible();
  await leading.getByRole("button", { name: "See evidence", exact: true }).click();
  await expect(leading.locator("blockquote").getByText("I bought it 45 days ago, unopened.", { exact: true }).first()).toBeVisible();
  expect(requests.map((request) => request.kind)).toEqual(["message", "check", "retest"]);
  expect(uploads).toBe(1);
  expect(state.document.evidence_sets![0]).toEqual(originalEvidence);
  expect(state.operations[0].results).toEqual(originalResults);
  expect(state.document.artifacts[0].conversation_evaluation!.expectations).toEqual(expectations);
  expect(state.document.artifacts[0].agent_prompt).toBe("");
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "1 new issue in this update" }),
  ).toBeVisible();
  expect(state.operations).toHaveLength(2);
  expect(requests.map((request) => request.kind)).toEqual(["message", "check", "retest"]);
  expect(edits).toHaveLength(0);
});
