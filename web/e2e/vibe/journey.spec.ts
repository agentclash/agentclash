import { expect, test, type Locator, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import type { Operation, Session } from "../../src/lib/vibe";

const models = {
  assistant: "openai/gpt-4.1-mini",
  target: "openai/gpt-4.1-mini",
  evaluator: "openai/gpt-4.1-mini",
};
const previewRules =
  "Preview capabilities: this is a text-only simulation with no connected tools.\n\n";

async function expectCenteredNavigation(page: Page) {
  await expect
    .poll(async () => {
      const header = await page.locator("header").boundingBox();
      const navigation = await page
        .getByRole("tablist", { name: "Workspace views" })
        .boundingBox();
      if (!header || !navigation) return Infinity;
      return Math.abs(
        navigation.x + navigation.width / 2 - (header.x + header.width / 2),
      );
    })
    .toBeLessThan(2);
}

async function openDetails(summary: Locator) {
  await expect(summary).toBeVisible();
  if (
    !(await summary.evaluate(
      (node) => (node.parentElement as HTMLDetailsElement).open,
    ))
  ) {
    await summary.click();
  }
}

async function openScorecardSection(page: Page, name: string | RegExp) {
  await openDetails(
    page
      .getByRole("article", { name: "Evaluation scorecard" })
      .locator("summary")
      .filter({ hasText: name }),
  );
}

test("new conversations use the server's free model defaults", async ({
  page,
}) => {
  const id = "liquid/lfm-2.5-2.6b:free";
  await page.route("**/v1/vibe/config", (route) =>
    route.fulfill({
      contentType: "application/json",
      headers: {
        "Access-Control-Allow-Origin": new URL(page.url()).origin,
        "Access-Control-Allow-Credentials": "true",
      },
      body: JSON.stringify({
        enabled: true,
        free_only: true,
        defaults: { assistant: id, target: id, evaluator: id },
        models: [
          {
            id,
            name: "Liquid (free)",
            input_nano_per_token: 0,
            output_nano_per_token: 0,
          },
        ],
      }),
    }),
  );
  await page.goto("/vibe-evals");
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("Liquid (free)");
  await expect(
    page.getByRole("combobox", { name: "Assistant model" }),
  ).toHaveCount(0);
});
async function mockVibe(
  page: Page,
  options: {
    paused?: boolean;
    casual?: boolean;
    loseAcknowledgement?: boolean;
    previewRules?: boolean;
  } = {},
) {
  const submissions = new Map<string, { body: string; operation: unknown }>();
  const messageRequests: Record<string, unknown>[] = [];
  let lostAcknowledgement = false;
  let messages = 0;
  let checks = 0;
  let trials = 0;
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
          "Access-Control-Allow-Origin": new URL(page.url()).origin,
          "Access-Control-Allow-Credentials": "true",
        },
      });
    if (request.method() === "OPTIONS")
      return route.fulfill({
        status: 204,
        headers: {
          "Access-Control-Allow-Origin": new URL(page.url()).origin,
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
        capabilities: options.previewRules
          ? [
              {
                id: "text_preview",
                label: "Text preview",
                available: true,
                description: "Supplied text only",
                instructions: previewRules,
              },
            ]
          : [],
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
          "Access-Control-Allow-Origin": new URL(page.url()).origin,
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
      if (prior)
        return JSON.stringify(body) === prior.body
          ? send(prior.operation, 202)
          : send(
              {
                error: {
                  code: "idempotency_conflict",
                  message: "This submission ID has different content.",
                },
              },
              409,
            );
      if (body.revision !== session.revision)
        return send(
          {
            error: {
              code: "revision_conflict",
              message: "Reload the latest conversation.",
            },
          },
          409,
        );
      const acknowledge = (operation: unknown) => {
        submissions.set(body.client_id, {
          body: JSON.stringify(body),
          operation,
        });
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
            content: options.casual
              ? "What would you like to explore about support agents?"
              : "I’ve drafted a support agent and three examples. Review the policy before checking it.\n\n<script>window.__vibePwned=true</script>\n![unsafe image](https://attacker.invalid/track.png)",
          },
        );
        const draft = options.casual
          ? null
          : {
              id: `draft-${messages}`,
              title: "Refund assistant",
              agent_prompt:
                (options.previewRules ? previewRules : "") +
                "Help customers with refunds within 30 days. Escalate unclear cases.",
              blueprint: {
                name: "Refund check",
                cases: [
                  {
                    key: "case-1",
                    payload: { question: "Refund after 10 days?" },
                    expectations: [
                      {
                        key: "expected_behavior",
                        kind: "text",
                        value: "Explain that it is eligible.",
                      },
                    ],
                  },
                  {
                    key: "case-2",
                    payload: { question: "Refund after 50 days?" },
                    expectations: [
                      {
                        key: "expected_behavior",
                        kind: "text",
                        value: "Explain that it is too late.",
                      },
                    ],
                  },
                  {
                    key: "case-3",
                    payload: { question: "Can I get a refund?" },
                    expectations: [
                      {
                        key: "expected_behavior",
                        kind: "text",
                        value: "Ask for the missing purchase details.",
                      },
                    ],
                  },
                ],
                judges: [
                  {
                    key: "behavior",
                    mode: "assertion",
                    rubric: "",
                    context_from: ["case.expectations.expected_behavior"],
                    assertion:
                      "The response meets this case's expected_behavior and the following shared rules:\n\nUse only supplied facts.",
                  },
                ],
                validators: [
                  {
                    key: "has_answer",
                    type: "regex_match",
                    target: "final_output",
                    expected_from: "literal:.+",
                  },
                ],
                dimensions: [
                  {
                    key: "output_present",
                    source: "validators",
                    validators: ["has_answer"],
                  },
                  {
                    key: "behavior_correctness",
                    source: "llm_judge",
                    judge_key: "behavior",
                  },
                ],
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
        return acknowledge({
          id: `message-operation-${messages}`,
          state: "COMPLETED",
        });
      }
      if (body.kind === "playground") {
        trials++;
        const id = `trial-${trials}`;
        session.document.messages.push(
          {
            id: body.client_id,
            role: "user",
            content: body.content,
            origin: "playground",
            artifact_id: body.artifact_id,
            operation_id: id,
            preview_thread_id: body.preview_thread_id,
          },
          {
            id: `${id}-reply`,
            role: "assistant",
            content:
              trials === 1
                ? "Is it unopened?"
                : "An unopened item bought 10 days ago is eligible.",
            origin: "playground",
            artifact_id: body.artifact_id,
            operation_id: id,
            preview_thread_id: body.preview_thread_id,
          },
        );
        const operation: Operation = {
          id,
          kind: "playground",
          state: "COMPLETED",
          billing: "RELEASED",
          models: body.models,
          max_cost_nano_usd: 0,
          actual_cost_nano_usd: 0,
          results: [],
        };
        session.operations.push(operation);
        return acknowledge(operation);
      }
      const reviewed = session.document.artifacts.find(
        (a) => a.id === body.artifact_id,
      );
      if (!reviewed || !body.approve_artifact)
        return send(
          {
            error: {
              code: "artifact_required",
              message: "Review and run these checks.",
            },
          },
          400,
        );
      reviewed.accepted = true;
      session.document.active_artifact_id = reviewed.id;
      checks++;
      const operation: Operation = {
        id: `check-${checks}`,
        grading: { version: 1, hash: "matching-fixture-contract" } as Operation["grading"],
        kind: body.kind,
        state: options.paused ? "RUNNING" : "PARTIAL",
        billing: options.paused ? "RESERVED" : "SETTLED",
        models: body.models,
        max_cost_nano_usd: 50000000,
        actual_cost_nano_usd: 10000000,
        results: [
          {
            case_key: "case-1",
            version: body.artifact_id,
            input: { question: "Refund after 10 days?" },
            expected: "Explain that it is eligible.",
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
            case_key: "case-2",
            version: body.artifact_id,
            input: { question: "Refund after 50 days?" },
            expected: "Explain that it is too late.",
            output: "",
            verdict: "UNKNOWN" as const,
            checks: [],
            error: { message: "Target timed out." },
          },
          {
            case_key: "case-3",
            version: body.artifact_id,
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
      if (body.revision !== session.revision)
        return send(
          {
            error: {
              code: "revision_conflict",
              message: "Reload before editing.",
            },
          },
          409,
        );
      if (body.artifact_id) {
        const artifact = session.document.artifacts.find(
          (a) => a.id === body.artifact_id,
        );
        if (!artifact)
          return send(
            { error: { code: "not_found", message: "Draft not found." } },
            404,
          );
        if (typeof body.agent_prompt === "string") {
          session.document.artifacts.push({
            ...artifact,
            id: `edited-${session.revision}`,
            parent_id: artifact.id,
            agent_prompt: body.agent_prompt,
            accepted: false,
          });
        } else {
          artifact.accepted = true;
          session.document.active_artifact_id = artifact.id;
        }
      }
      if (body.requirement_id) {
        const requirement = session.document.requirements.find(
          (r) => r.id === body.requirement_id,
        );
        if (!requirement)
          return send(
            { error: { code: "not_found", message: "Requirement not found." } },
            404,
          );
        if (body.status === "superseded") {
          requirement.status = "superseded";
          session.document.requirements.push({
            ...requirement,
            id: `requirement-${session.revision}`,
            statement: body.statement,
            status: "accepted",
          });
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
    page.getByRole("heading", { name: "What should your agent do?" }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill(
      "We need a customer support agent. Refunds are allowed within 30 days; escalate unclear cases.",
    );
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Here’s what I’ll check" }),
  ).toBeVisible();
  expect(mock.messageCount()).toBe(1);
  await page
    .getByRole("button", { name: "Agent instructions", exact: true })
    .click();
  await page
    .getByText("Assumptions and confirmed requirements", { exact: true })
    .first()
    .click();
  await expect(
    page.getByText("Proposed · needs your confirmation"),
  ).toBeVisible();
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page
    .getByRole("combobox", { name: "Agent model", exact: true })
    .selectOption("openai/gpt-4.1");
  await expect(
    page.getByRole("combobox", { name: "Assistant model" }),
  ).toHaveValue(models.assistant);
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await page.getByRole("button", { name: "Run 3 examples" }).click();
  await expect(
    page.getByRole("article", { name: "Evaluation scorecard" }),
  ).toContainText("1 passed · 0 need attention · 2 unresolved");
  expect(mock.snapshot().operations[0].models.target).toBe("openai/gpt-4.1");
  expect(mock.snapshot().operations[0].models.evaluator).toBe(models.evaluator);
  await openScorecardSection(page, /^Other results/);
  await openScorecardSection(page, /^Situation 2/);
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

test("simple chat sends a real message, offers proposals without a draft and never invents a scorecard", async ({
  page,
}) => {
  const mock = await mockVibe(page, { casual: true });
  await page.goto("/vibe-evals");
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill("Help me explore how AI could support our customer support team.");
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(
    page.getByText("What would you like to explore about support agents?"),
  ).toBeVisible();
  await expect(
    page.getByRole("article", { name: "Evaluation scorecard" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: "Your agent", exact: true }),
  ).toHaveCount(0);
  await page.getByText("Requirements and assumptions", { exact: true }).click();
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect
    .poll(() => mock.snapshot().document.requirements[0].status)
    .toBe("accepted");
  await openDetails(
    page.locator("summary").filter({ hasText: "Requirements and assumptions" }),
  );
  await expect(page.getByText("Confirmed by you")).toBeVisible();
  expect(mock.messageCount()).toBe(1);
  expect(mock.snapshot().document.artifacts).toHaveLength(0);
  expect(mock.checkCount()).toBe(0);
});

for (const viewport of [
  { width: 1440, height: 900 },
  { width: 390, height: 844 },
]) {
  test(`one clear first action and centered results navigation at ${viewport.width}px`, async ({
    page,
  }) => {
    await page.setViewportSize(viewport);
    const mock = await mockVibe(page);
    await page.goto("/vibe-evals");
    await expect(page.getByRole("tablist")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Send message", exact: true }),
    ).toBeDisabled();
    await page.getByRole("button", { name: "See an example" }).click();
    await page.getByRole("button", { name: "Use this description" }).click();
    expect(mock.messageCount()).toBe(0);
    await page.getByRole("button", { name: "Send message", exact: true }).click();
    await page.getByRole("button", { name: "Run 3 examples" }).click();
    await expect(page.getByRole("tab", { name: "Results" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expectCenteredNavigation(page);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: test.info().outputPath("responsive-results.png"),
    });
  });
}

test("draft editor separates preview rules while edits and exports keep the effective instructions", async ({
  page,
}) => {
  const mock = await mockVibe(page, { previewRules: true });
  await page.goto("/vibe-evals");
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill("Build a support agent with a 30 day refund policy.");
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await page
    .getByRole("button", { name: "Agent instructions", exact: true })
    .click();
  const instructions = page.getByRole("textbox", {
    name: "Agent instructions",
    exact: true,
  });
  await expect(instructions).toHaveValue(
    "Help customers with refunds within 30 days. Escalate unclear cases.",
  );
  await expect(
    page.getByText(previewRules.trim(), { exact: true }),
  ).toBeHidden();
  await page.screenshot({ path: test.info().outputPath("draft-editor.png") });
  await page.getByText("How this preview works", { exact: true }).click();
  await expect(
    page.getByText(previewRules.trim(), { exact: true }),
  ).toBeVisible();
  await page.getByText("How this preview works", { exact: true }).click();
  await instructions.fill(
    "Use supplied refund facts. Ask when the policy is missing.",
  );
  await page
    .getByRole("button", { name: "Apply changes", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Try a message", exact: true }),
  ).toBeEnabled();
  await expect(instructions).toHaveValue(
    "Use supplied refund facts. Ask when the policy is missing.",
  );
  const effective =
    previewRules + "Use supplied refund facts. Ask when the policy is missing.";
  expect(mock.snapshot().document.artifacts.at(-1)?.agent_prompt).toBe(
    effective,
  );
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  const downloadReady = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "Export agent and checks", exact: true })
    .click();
  const download = await downloadReady;
  const exported = JSON.parse(await readFile((await download.path())!, "utf8"));
  expect(exported.agent_prompt).toBe(effective);
  expect(mock.messageCount()).toBe(1);
  expect(mock.checkCount()).toBe(0);
});

async function createAndCheck(page: Page) {
  await page.goto("/vibe-evals");
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill("Build a support agent with a 30 day refund policy.");
  await page
    .getByRole("button", { name: "Send message", exact: true })
    .click({ clickCount: 2 });
  await page
    .getByRole("button", { name: "Run 3 examples", exact: true })
    .click();
}

test("Try keeps followups separate from Build and starts fresh on reset", async ({
  page,
}) => {
  const mock = await mockVibe(page);
  await page.goto("/vibe-evals");
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill("Build a return-policy assistant.");
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill("Unsent changes to my agent");
  await page
    .getByRole("button", { name: "Try a message", exact: true })
    .click();
  const composer = page.getByRole("textbox", { name: "Message your agent" });
  await composer.fill("I bought it 10 days ago.");
  await page
    .getByRole("button", { name: "Send to agent", exact: true })
    .click();
  await expect(
    page.getByRole("log", { name: "Trial conversation", exact: true }),
  ).toContainText("Is it unopened?");
  await composer.fill("It is unopened.");
  await page
    .getByRole("button", { name: "← Back to Vibe Evals", exact: true })
    .click();
  await expect(
    page.getByRole("textbox", { name: "Message Vibe Evals" }),
  ).toHaveValue("Unsent changes to my agent");
  await expect(
    page.getByRole("log", { name: "Conversation with Vibe Evals" }),
  ).not.toContainText("Is it unopened?");
  await page
    .getByRole("button", { name: "Try a message", exact: true })
    .click();
  await expect(composer).toHaveValue("It is unopened.");
  await page
    .getByRole("button", { name: "Send to agent", exact: true })
    .click();
  const trials = mock.messageRequests().filter((r) => r.kind === "playground");
  expect(trials).toHaveLength(2);
  expect(trials[0].preview_thread_id).toBe(trials[1].preview_thread_id);
  expect(trials[0].approve_artifact).toBeUndefined();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Try a message" }),
  ).toBeVisible();
  await expect(
    page.getByRole("log", { name: "Trial conversation", exact: true }),
  ).toContainText("An unopened item bought 10 days ago is eligible.");
  await page
    .getByRole("button", { name: "New conversation", exact: true })
    .click();
  await expect(
    page.getByRole("log", { name: "Trial conversation", exact: true }),
  ).toBeEmpty();
  expect(mock.checkCount()).toBe(0);
  expect(mock.snapshot().document.artifacts[0].accepted).toBe(false);
});

test("improve and retest keep a fixed comparison, duplicate clicks send once", async ({
  page,
}) => {
  const mock = await mockVibe(page);
  await createAndCheck(page);
  expect(mock.messageCount()).toBe(1);
  mock.snapshot().operations[0].results[0].verdict = "FAIL";
  mock.snapshot().operations[0].scorecard!.failed = 1;
  mock.snapshot().operations[0].scorecard!.passed = 0;
  mock.snapshot().revision++;
  await openScorecardSection(page, "What was checked");
  await page
    .getByRole("button", { name: "Suggest a change", exact: true })
    .click();
  await expect.poll(() => mock.messageCount()).toBe(2);
  await page.getByRole("tab", { name: "Results", exact: true }).click();
  await openScorecardSection(page, "What was checked");
  expect(
    mock
      .messageRequests()
      .filter((r) => r.kind === "message")
      .at(-1)?.baseline_id,
  ).toBe("check-1");
  await page
    .getByRole("button", { name: "Run the same examples again", exact: true })
    .click();
  await expect(
    page.getByRole("article", { name: "Evaluation scorecard" }),
  ).toContainText("1 previously failing example now passes");
  expect(mock.checkCount()).toBe(2);
  expect(mock.snapshot().operations[1].baseline_id).toBe("check-1");
  mock.stale();
  await page.waitForResponse((r) => r.url().endsWith("/events"));
  await expect(
    page.getByRole("article", { name: "Evaluation scorecard" }),
  ).toHaveCount(1);
});

test("refresh during a check never reruns; Stop preserves reconciling billing", async ({
  page,
}) => {
  const mock = await mockVibe(page, { paused: true });
  await createAndCheck(page);
  await expect(
    page.getByRole("button", { name: "Stop", exact: true }),
  ).toBeVisible();
  await page.reload();
  await page.getByRole("button", { name: "Stop", exact: true }).click();
  await openScorecardSection(page, "What was checked");
  await expect(
    page.getByText("Provider spend is still being reconciled.", {
      exact: true,
    }),
  ).toBeVisible();
  expect(mock.checkCount()).toBe(1);
  await expect(
    page.getByRole("button", { name: "Stop", exact: true }),
  ).toHaveCount(0);
});

test("lost acknowledgement retries exactly after SSE advances and preserves the next message", async ({
  page,
}) => {
  const mock = await mockVibe(page, { loseAcknowledgement: true });
  await page.goto("/vibe-evals");
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill("Build a refund agent.");
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Here’s what I’ll check" }),
  ).toBeVisible();
  expect(mock.messageRequests()).toHaveLength(1);
  await composer.fill("Here is my next question");
  await page.getByRole("button", { name: "Retry submission" }).click();
  await expect(
    page.getByRole("button", { name: "Retry submission" }),
  ).toHaveCount(0);
  expect(mock.messageRequests()).toHaveLength(2);
  expect(mock.messageRequests()[1]).toEqual(mock.messageRequests()[0]);
  expect(mock.messageCount()).toBe(1);
  await expect(composer).toHaveValue("Here is my next question");
});

test("unapplied instructions cannot run checks; applying creates a ready-to-try version", async ({
  page,
}) => {
  const mock = await mockVibe(page);
  await page.goto("/vibe-evals");
  await page
    .getByRole("textbox", { name: "Message Vibe Evals" })
    .fill("Build a refund agent.");
  await page.getByRole("button", { name: "Send message", exact: true }).click();
  await page
    .getByRole("button", { name: "Agent instructions", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "Agent instructions", exact: true })
    .fill("Edited policy: escalate all exceptions.");
  await expect(
    page.getByRole("button", { name: "Run 3 examples" }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Save agent", exact: true }),
  ).toBeDisabled();
  expect(mock.checkCount()).toBe(0);
  await page.getByRole("button", { name: "Apply changes" }).click();
  await page.getByRole("button", { name: "Run 3 examples" }).click();
  await expect(
    page.getByRole("article", { name: "Evaluation scorecard" }),
  ).toBeVisible();
  expect(mock.checkCount()).toBe(1);
  expect(mock.snapshot().document.artifacts).toHaveLength(2);
  expect(mock.messageRequests().at(-1)?.artifact_id).toBe(
    mock.snapshot().document.artifacts.at(-1)?.id,
  );
});

test("expanded evidence refreshes when a persisted UNKNOWN becomes PASS", async ({
  page,
}) => {
  const mock = await mockVibe(page, { paused: true });
  await createAndCheck(page);
  await openScorecardSection(page, /^Situation 2/);
  await expect(page.getByText("Target timed out.")).toBeVisible();
  const operation = mock.snapshot().operations[0];
  operation.results[1] = {
    ...operation.results[1],
    verdict: "PASS",
    output: "Late requests need escalation.",
    error: undefined,
    checks: [
      { key: "policy", verdict: "PASS", evidence: "Escalation verified." },
    ],
  };
  operation.scorecard = {
    ...operation.scorecard!,
    passed: 2,
    unknown: 1,
    evaluated: 2,
    coverage: 2 / 3,
  };
  mock.snapshot().revision++;
  await expect(
    page.getByText("Late requests need escalation.", { exact: true }).first(),
  ).toBeVisible();
  await expect(
    page.getByText("Escalation verified.", { exact: true }).first(),
  ).toBeVisible();
  await expect(page.getByText("Target timed out.")).toHaveCount(0);
  expect(mock.checkCount()).toBe(1);
});

for (const state of ["RUNNING", "CANCELLED"]) {
  test(`expanded evidence refreshes output-only recovery from ${state}`, async ({
    page,
  }) => {
    const mock = await mockVibe(page, { paused: true });
    await createAndCheck(page);
    const session = mock.snapshot();
    const operation = session.operations[0];
    operation.state = state;
    session.event_cursor = session.revision + 1;
    if (state === "CANCELLED") {
      await openScorecardSection(page, /^Other results/);
    }
    await openScorecardSection(page, /^Situation 2/);
    await expect(page.getByText("Target timed out.")).toBeVisible();
    const revision = session.revision;
    const caseBefore = structuredClone(operation.results[1]);
    operation.results[1].output =
      "Recovered journaled response without an evaluator verdict.";
    operation.state = state === "RUNNING" ? "PARTIAL" : state;
    session.event_cursor++;
    // Neither document revision nor redacted case metadata changes on recovery.
    expect(session.revision).toBe(revision);
    expect({ ...operation.results[1], output: caseBefore.output }).toEqual(
      caseBefore,
    );
    await expect(
      page.getByText(operation.results[1].output, { exact: true }).first(),
    ).toBeVisible();
    const row = page
      .getByRole("article", { name: "Evaluation scorecard" })
      .locator("summary")
      .filter({ hasText: "Situation 2" })
      .locator("..");
    await expect(row).toHaveAttribute("open", "");
    await expect(row.locator(":scope > summary")).toContainText("Could not determine");
    expect(mock.checkCount()).toBe(1);
    expect(mock.messageCount()).toBe(1);
  });
}
