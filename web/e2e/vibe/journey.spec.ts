import { expect, test, type Page } from "@playwright/test";
import type { Operation, Session } from "../../src/lib/vibe";

const models = {
  assistant: "openai/gpt-4.1-mini",
  target: "openai/gpt-4.1-mini",
  evaluator: "openai/gpt-4.1-mini",
};

test("new conversations use the server's free model defaults", async ({ page }) => {
  const id = "liquid/lfm-2.5-2.6b:free";
  await page.route("**/v1/vibe/config", (route) => route.fulfill({
    contentType: "application/json",
    headers: {
      "Access-Control-Allow-Origin": "http://127.0.0.1:53517",
      "Access-Control-Allow-Credentials": "true",
    },
    body: JSON.stringify({ enabled: true, free_only: true,
      defaults: { assistant: id, target: id, evaluator: id },
      models: [{ id, name: "Liquid (free)", input_nano_per_token: 0, output_nano_per_token: 0 }] }),
  }));
  await page.goto("/vibe-evals");
  await expect(page.getByRole("combobox", { name: "Assistant model" })).toHaveValue(id);
});
async function mockVibe(page: Page, options: { paused?: boolean; casual?: boolean; loseAcknowledgement?: boolean } = {}) {
  const submissions = new Map<string, { body: string; operation: unknown }>();
  const messageRequests: Record<string, unknown>[] = [];
  let lostAcknowledgement = false;
  let messages = 0;
  let checks = 0;
  const session: Session = {
    id: "",
    revision: 0,
    anonymous: true,
    document: { messages: [], requirements: [], artifacts: [], models },
    operations: [],
  };
  let stale = false;
  const initial = structuredClone(session);
  const snapshot = () => ({
    ...session,
    event_cursor: session.event_cursor ?? session.revision,
    operations: session.operations.map((o) => ({
      ...o,
      results: o.results.map((r) => ({
        ...r,
        input: null,
        output: "",
        checks: r.checks.map((c) => ({
          key: c.key,
          verdict: c.verdict,
          evidence: "",
        })),
      })),
    })),
  });
  await page.route("**/v1/vibe/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const send = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(body),
        headers: {
          "Access-Control-Allow-Origin": "http://127.0.0.1:53517",
          "Access-Control-Allow-Credentials": "true",
        },
      });
    if (request.method() === "OPTIONS")
      return route.fulfill({
        status: 204,
        headers: {
          "Access-Control-Allow-Origin": "http://127.0.0.1:53517",
          "Access-Control-Allow-Credentials": "true",
          "Access-Control-Allow-Headers":
            "Content-Type, Authorization, If-Match",
          "Access-Control-Allow-Methods": "GET, POST, PATCH, OPTIONS",
        },
      });
    if (path.endsWith("/config"))
      return send({
        enabled: true,
        defaults: models,
        models: [
          { id: models.assistant, name: "GPT-4.1 Mini" },
          { id: "openai/gpt-4.1", name: "GPT-4.1" },
        ],
      });
    if (path.endsWith("/sessions") && request.method() === "POST") {
      session.id = request.postDataJSON().id;
      return send(session, 201);
    }
    if (path.endsWith("/events"))
      return route.fulfill({
        contentType: "text/event-stream",
        body: `id: 1\nevent: snapshot\ndata: ${JSON.stringify(stale ? { ...initial, id: session.id, event_cursor: 0 } : snapshot())}\n\n`,
        headers: {
          "Access-Control-Allow-Origin": "http://127.0.0.1:53517",
          "Access-Control-Allow-Credentials": "true",
        },
      });
    if (path.endsWith("/case")) {
      const operation = session.operations.find(
        (o) => o.id === path.split("/").at(-2),
      );
      return send(
        operation?.results.find(
          (r) => r.case_key === new URL(request.url()).searchParams.get("key"),
        ),
      );
    }
    if (path.endsWith("/stop")) {
      const operation = session.operations.find(
        (o) => o.id === path.split("/").at(-2),
      );
      if (operation) {
        operation.state = "CANCELLED";
        operation.billing = "RECONCILING";
        session.revision++;
      }
      return send({});
    }
    if (path.endsWith("/messages")) {
      const body = request.postDataJSON();
      messageRequests.push(body);
      const prior = submissions.get(body.client_id);
      if (prior) return JSON.stringify(body) === prior.body
        ? send(prior.operation, 202)
        : send({ error: { code: "idempotency_conflict", message: "This submission ID has different content." } }, 409);
      if (body.revision !== session.revision) return send({ error: { code: "revision_conflict", message: "Reload the latest conversation." } }, 409);
      const acknowledge = (operation: unknown) => {
        submissions.set(body.client_id, { body: JSON.stringify(body), operation });
        if (options.loseAcknowledgement && !lostAcknowledgement) {
          lostAcknowledgement = true;
          return route.abort("failed");
        }
        return send(operation, 202);
      };
      session.document.models = body.models;
      session.revision++;
      if (body.kind === "message") {
        messages++;
        session.document.messages.push(
          { id: body.client_id, role: "user", content: body.content },
          {
            id: `assistant-${messages}`,
            role: "assistant",
            content: options.casual ? "What would you like to explore about support agents?" :
              "I’ve drafted a support agent and three examples. Review the policy before checking it.\n\n<script>window.__vibePwned=true</script>\n![unsafe image](https://attacker.invalid/track.png)",
          },
        );
        const draft = options.casual ? null : {
          id: `draft-${messages}`,
          title: "Refund assistant",
          agent_prompt:
            "Help customers with refunds within 30 days. Escalate unclear cases.",
          blueprint: {
            name: "Refund check",
            cases: [{ key: "eligible" }, { key: "late" }, { key: "unclear" }],
          },
          accepted: false,
          source_message_id: body.client_id,
        };
        if (draft) session.document.artifacts.push(draft);
        if (messages === 1)
          session.document.requirements.push({
            id: "policy-one",
            statement: "Refunds are allowed within 30 days.",
            status: "proposed",
            source_message_id: body.client_id,
          });
        return acknowledge({ id: `message-operation-${messages}`, state: "COMPLETED" });
      }
      checks++;
      const operation: Operation = {
        id: `check-${checks}`,
        kind: body.kind,
        state: options.paused ? "RUNNING" : "PARTIAL",
        billing: options.paused ? "RESERVED" : "SETTLED",
        models: body.models,
        max_cost_nano_usd: 50000000,
        actual_cost_nano_usd: 10000000,
        results: [
          {
            case_key: "eligible",
            version: "draft-one",
            input: { question: "Refund after 10 days?" },
            output: "You are eligible for a refund.",
            verdict: "PASS" as const,
            checks: [
              {
                key: "policy",
                verdict: "PASS" as const,
                evidence: "Within the policy window.",
              },
            ],
          },
          {
            case_key: "late",
            version: "draft-one",
            input: { question: "Refund after 50 days?" },
            output: "",
            verdict: "UNKNOWN" as const,
            checks: [],
            error: { message: "Target timed out." },
          },
          {
            case_key: "unclear",
            version: "draft-one",
            input: { question: "What about a damaged item?" },
            output: "",
            verdict: "UNKNOWN" as const,
            checks: [],
            error: { message: "Evaluator unavailable." },
          },
        ],
        scorecard: {
          passed: 1,
          failed: 0,
          unknown: 2,
          total: 3,
          evaluated: 1,
          pass_rate: 1,
          coverage: 1 / 3,
        },
      };
      if (body.kind === "retest") {
        operation.baseline_id = body.baseline_id;
        operation.state = "COMPLETED";
        operation.scorecard = {
          passed: 3,
          failed: 0,
          unknown: 0,
          total: 3,
          evaluated: 3,
          pass_rate: 1,
          coverage: 1,
        };
        operation.results = operation.results.map((r) => ({
          ...r,
          verdict: "PASS",
          error: undefined,
          output: "Policy followed.",
          checks: [
            { key: "policy", verdict: "PASS", evidence: "Policy confirmed." },
          ],
        }));
      }
      session.operations.push(operation);
      return acknowledge(operation);
    }
    if (request.method() === "PATCH") {
      const body = request.postDataJSON();
      if (body.revision !== session.revision) return send({ error: { code: "revision_conflict", message: "Reload before editing." } }, 409);
      if (body.artifact_id) {
        const artifact = session.document.artifacts.find((a) => a.id === body.artifact_id);
        if (!artifact) return send({ error: { code: "not_found", message: "Draft not found." } }, 404);
        if (typeof body.agent_prompt === "string") {
          session.document.artifacts.push({ ...artifact, id: `edited-${session.revision}`, parent_id: artifact.id, agent_prompt: body.agent_prompt, accepted: false });
        } else {
          artifact.accepted = true;
          session.document.active_artifact_id = artifact.id;
        }
      }
      if (body.requirement_id) {
        const requirement = session.document.requirements.find((r) => r.id === body.requirement_id);
        if (!requirement) return send({ error: { code: "not_found", message: "Requirement not found." } }, 404);
        if (body.status === "superseded") {
          requirement.status = "superseded";
          session.document.requirements.push({ ...requirement, id: `requirement-${session.revision}`, statement: body.statement, status: "accepted" });
        } else requirement.status = body.status;
      }
      session.revision++;
      return send(snapshot());
    }
    return send(snapshot());
  });
  return {
    messageCount: () => messages,
    messageRequests: () => messageRequests,
    checkCount: () => checks,
    snapshot: () => session,
    stale: () => {
      stale = true;
    },
  };
}

test("describe, review, independently select agent, check, inspect unknowns and reconnect", async ({
  page,
}) => {
  const mock = await mockVibe(page);
  await page.goto("/vibe-evals");
  await expect(
    page.getByRole("heading", { name: "What are you working on?" }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill(
      "We need a customer support agent. Refunds are allowed within 30 days; escalate unclear cases.",
    );
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Refund assistant" }),
  ).toBeVisible();
  expect(mock.messageCount()).toBe(1);
  await expect(
    page.getByText("Proposed · needs your confirmation"),
  ).toBeVisible();
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await page.getByRole("button", { name: "Accept this draft" }).click();
  await page
    .getByRole("combobox", { name: "Agent model", exact: true })
    .selectOption("openai/gpt-4.1");
  await expect(
    page.getByRole("combobox", { name: "Assistant model" }),
  ).toHaveValue(models.assistant);
  await page.getByRole("button", { name: "Check this agent" }).click();
  await expect(
    page.getByRole("article", { name: "Evaluation scorecard" }),
  ).toContainText("1 of 3 cases evaluated");
  expect(mock.snapshot().operations[0].models.target).toBe("openai/gpt-4.1");
  expect(mock.snapshot().operations[0].models.evaluator).toBe(models.evaluator);
  await page.getByText("late", { exact: true }).click();
  await expect(page.getByText("Target timed out.")).toBeVisible();
  const url = page.url();
  await page.reload();
  await expect(
    page.getByRole("article", { name: "Evaluation scorecard" }),
  ).toBeVisible();
  expect(page.url()).toBe(url);
  expect(mock.checkCount()).toBe(1);
  expect(
    await page.evaluate(
      () => (window as unknown as { __vibePwned?: boolean }).__vibePwned,
    ),
  ).toBeUndefined();
  await expect(page.locator('img[src*="attacker.invalid"]')).toHaveCount(0);
  if (process.env.VIBE_SCREENSHOT_PATH) {
    await page.screenshot({ path: process.env.VIBE_SCREENSHOT_PATH });
  }
});

test("simple chat sends a real message, offers proposals without a draft and never invents a scorecard", async ({ page }) => {
  const mock = await mockVibe(page, { casual: true });
  await page.goto("/vibe-evals");
  await page.getByRole("button", { name: "I’m figuring out what AI could do for us" }).click();
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(page.getByText("What would you like to explore about support agents?")).toBeVisible();
  await expect(page.getByRole("article", { name: "Evaluation scorecard" })).toHaveCount(0);
  await expect(page.getByRole("complementary", { name: "Agent draft" })).toHaveCount(0);
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(page.getByText("Confirmed by you")).toBeVisible();
  expect(mock.messageCount()).toBe(1);
  expect(mock.snapshot().document.artifacts).toHaveLength(0);
  expect(mock.checkCount()).toBe(0);
});

async function createAcceptedDraft(page: Page) {
  await page.goto("/vibe-evals");
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill("Build a support agent with a 30 day refund policy.");
  await page
    .getByRole("button", { name: "Send message", exact: true })
    .click({ clickCount: 2 });
  await page.getByRole("button", { name: "Accept this draft" }).click();
  await page
    .getByRole("button", { name: "Check this agent", exact: true })
    .click();
}

test("improve and retest keep a fixed comparison, duplicate clicks send once", async ({
  page,
}) => {
  const mock = await mockVibe(page);
  await createAcceptedDraft(page);
  expect(mock.messageCount()).toBe(1);
  await page
    .getByRole("button", { name: "Improve the instructions", exact: true })
    .click();
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await page.getByRole("button", { name: "Accept this draft" }).click();
  await page
    .getByRole("button", { name: "Retest the same examples", exact: true })
    .click();
  await expect(page.getByText("Same examples and evaluator:")).toContainText(
    "1 → 3 passed",
  );
  expect(mock.checkCount()).toBe(2);
  expect(mock.snapshot().operations[1].baseline_id).toBe("check-1");
  mock.stale();
  await page.waitForResponse((r) => r.url().endsWith("/events"));
  await expect(
    page.getByRole("article", { name: "Evaluation scorecard" }),
  ).toHaveCount(2);
});

test("refresh during a check never reruns; Stop preserves reconciling billing", async ({
  page,
}) => {
  const mock = await mockVibe(page, { paused: true });
  await createAcceptedDraft(page);
  await expect(
    page.getByRole("button", { name: "Stop", exact: true }),
  ).toBeVisible();
  await page.reload();
  await page.getByRole("button", { name: "Stop", exact: true }).click();
  await expect(
    page.getByText("Execution: cancelled", { exact: false }),
  ).toContainText("Billing: reconciling");
  expect(mock.checkCount()).toBe(1);
  await expect(
    page.getByRole("button", { name: "Stop", exact: true }),
  ).toHaveCount(0);
});

test("lost acknowledgement retries exactly after SSE advances and preserves the next message", async ({ page }) => {
  const mock = await mockVibe(page, { loseAcknowledgement: true });
  await page.goto("/vibe-evals");
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("Build a refund agent.");
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Refund assistant" })).toBeVisible();
  expect(mock.messageRequests()).toHaveLength(1);
  await composer.fill("Here is my next question");
  await page.getByRole("button", { name: "Retry submission" }).click();
  await expect(page.getByRole("button", { name: "Retry submission" })).toHaveCount(0);
  expect(mock.messageRequests()).toHaveLength(2);
  expect(mock.messageRequests()[1]).toEqual(mock.messageRequests()[0]);
  expect(mock.messageCount()).toBe(1);
  await expect(composer).toHaveValue("Here is my next question");
});

test("dirty accepted instructions cannot spend a check before save-as-draft and acceptance", async ({ page }) => {
  const mock = await mockVibe(page);
  await page.goto("/vibe-evals");
  await page.getByRole("textbox", { name: "Message Vibe Evals" }).fill("Build a refund agent.");
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await page.getByRole("button", { name: "Accept this draft" }).click();
  await page.getByRole("textbox", { name: "Agent instructions", exact: true }).fill("Edited policy: escalate all exceptions.");
  await expect(page.getByRole("button", { name: "Check this agent" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Keep it" })).toBeDisabled();
  expect(mock.checkCount()).toBe(0);
  await page.getByRole("button", { name: "Save as a new draft" }).click();
  await expect(page.getByRole("button", { name: "Check this agent" })).toBeDisabled();
  await page.getByRole("button", { name: "Accept this draft" }).click();
  await page.getByRole("button", { name: "Check this agent" }).click();
  await expect(page.getByRole("article", { name: "Evaluation scorecard" })).toBeVisible();
  expect(mock.checkCount()).toBe(1);
  expect(mock.snapshot().document.artifacts).toHaveLength(2);
  expect(mock.messageRequests().at(-1)?.artifact_id).toBe(mock.snapshot().document.artifacts.at(-1)?.id);
});

test("expanded evidence refreshes when a persisted UNKNOWN becomes PASS", async ({ page }) => {
  const mock = await mockVibe(page, { paused: true });
  await createAcceptedDraft(page);
  await page.getByText("late", { exact: true }).click();
  await expect(page.getByText("Target timed out.")).toBeVisible();
  const operation = mock.snapshot().operations[0];
  operation.results[1] = { ...operation.results[1], verdict: "PASS", output: "Late requests need escalation.", error: undefined, checks: [{ key: "policy", verdict: "PASS", evidence: "Escalation verified." }] };
  operation.scorecard = { ...operation.scorecard!, passed: 2, unknown: 1, evaluated: 2, coverage: 2 / 3 };
  mock.snapshot().revision++;
  await expect(page.getByText("Late requests need escalation.", { exact: true })).toBeVisible();
  await expect(page.getByText("Escalation verified.", { exact: true })).toBeVisible();
  await expect(page.getByText("Target timed out.")).toHaveCount(0);
  expect(mock.checkCount()).toBe(1);
});

for (const state of ["RUNNING", "CANCELLED"]) {
  test(`expanded evidence refreshes output-only recovery from ${state}`, async ({ page }) => {
    const mock = await mockVibe(page, { paused: true });
    await createAcceptedDraft(page);
    const session = mock.snapshot();
    const operation = session.operations[0];
    operation.state = state;
    session.event_cursor = session.revision + 1;
    await page.getByText("late", { exact: true }).click();
    await expect(page.getByText("Target timed out.")).toBeVisible();
    const revision = session.revision;
    const caseBefore = structuredClone(operation.results[1]);
    operation.results[1].output = "Recovered journaled response without an evaluator verdict.";
    operation.state = state === "RUNNING" ? "PARTIAL" : state;
    session.event_cursor++;
    // Neither document revision nor redacted case metadata changes on recovery.
    expect(session.revision).toBe(revision);
    expect({ ...operation.results[1], output: caseBefore.output }).toEqual(caseBefore);
    await expect(page.getByText(operation.results[1].output, { exact: true })).toBeVisible();
    const row = page.locator("details").filter({ has: page.locator("summary", { hasText: "late" }) });
    await expect(row).toHaveAttribute("open", "");
    await expect(row.locator("summary")).toContainText("UNKNOWN");
    expect(mock.checkCount()).toBe(1);
    expect(mock.messageCount()).toBe(1);
  });
}
