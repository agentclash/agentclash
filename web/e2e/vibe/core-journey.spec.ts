import { expect, test } from "@playwright/test";
import type { Artifact, Operation, Session } from "../../src/lib/vibe";

test.use({ video: { mode: "on", size: { width: 1440, height: 960 } } });

// Browser contract fixture: real provider and persistence are checked separately.
const models = {
  assistant: "fixture/free",
  target: "fixture/free",
  evaluator: "fixture/free",
};
const brief =
  "My agent handles returns: unopened items within 30 days. Ask for missing details.";
const instructions = "Allow unopened returns. Ask for missing details.";
const inputs = [
  "Unopened, bought 10 days ago. Can I return it?",
  "Unopened, bought 45 days ago. Can I return it?",
  "I want to return something.",
];
const expected = [
  "Explain that it is eligible.",
  "Decline the late return.",
  "Ask for purchase age and item condition.",
];

for (const mobile of [false, true]) {
  test(`description to tests, fix and unchanged rerun (${mobile ? "phone, reduced motion" : "desktop"})`, async ({
    page,
  }, info) => {
    await page.setViewportSize(
      mobile ? { width: 390, height: 844 } : { width: 1440, height: 960 },
    );
    if (mobile) await page.emulateMedia({ reducedMotion: "reduce" });
    const state: Session = {
      id: "",
      revision: 0,
      anonymous: true,
      document: { models, messages: [], requirements: [], artifacts: [] },
      operations: [],
    };
    const requests: Record<string, unknown>[] = [];
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
    let releasePreparation!: () => void;
    const preparation = new Promise<void>((resolve) => {
      releasePreparation = resolve;
    });
    const blueprint = {
      cases: inputs.map((question, i) => ({
        key: `case-${i + 1}`,
        payload: { question },
        expectations: [
          { key: "expected_behavior", kind: "text", value: expected[i] },
        ],
      })),
      judges: [
        {
          key: "behavior",
          context_from: ["case.expectations.expected_behavior"],
          assertion:
            "The response meets this case's expected_behavior and the following shared rules:\n\nUnopened items within 30 days.",
        },
      ],
      validators: [{ key: "has_answer" }],
      dimensions: [{}, {}],
    };
    const snapshot = () => ({ ...state, event_cursor: state.revision });
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
            "Access-Control-Allow-Headers":
              "Content-Type,Authorization,If-Match",
            "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS",
          },
        });
      if (path.endsWith("/config"))
        return send({
          enabled: true,
          free_only: true,
          defaults: models,
          models: [{ id: models.target, name: "Fixture (free)" }],
        });
      if (path.endsWith("/events"))
        return route.fulfill({
          headers,
          contentType: "text/event-stream",
          body: `event: snapshot\ndata: ${JSON.stringify(snapshot())}\n\n`,
        });
      if (path.endsWith("/case"))
        return send(
          state.operations
            .find((o) => path.includes(o.id))
            ?.results.find(
              (c) => c.case_key === new URL(req.url()).searchParams.get("key"),
            ),
        );
      if (req.method() === "POST" && path.endsWith("/sessions")) {
        state.id = req.postDataJSON().id;
        return send(snapshot(), 201);
      }
      if (req.method() === "PATCH") {
        const body = req.postDataJSON();
        expect(body.agent_prompt).toBe(instructions);
        state.document.artifacts.push({
          ...state.document.artifacts.at(-1)!,
          id: "bound",
          parent_id: "tests",
          agent_prompt: body.agent_prompt,
        });
        state.document.artifacts.at(-1)!.proposal_message_id = "reply-bound";
        state.document.messages.push(
          { id: "edit-bound", role: "user", content: "Updated agent instructions.", origin: "edit" },
          { id: "reply-bound", role: "assistant", content: "Ready to test these changes.", origin: "edit", artifact_id: "bound" },
        );
        state.revision++;
        return send(snapshot());
      }
      if (req.method() === "POST" && path.endsWith("/messages")) {
        const body = req.postDataJSON();
        requests.push(body);
        expect(body.test_journey).toBe(true);
        expect(body.quick_check).toBeUndefined();
        state.document.test_journey = true;
        if (body.kind === "message") {
          if (body.content === "let's drink vodka") {
            expect(body.viewed_run_id).toBe("run-1");
            const count = state.document.artifacts.length;
            state.document.messages.push(
              { id: body.client_id, role: "user", content: body.content },
              { id: "casual-reply", role: "assistant", content: "Taking a break? Your tests will be here when you’re ready." },
            );
            state.revision++;
            expect(state.document.artifacts).toHaveLength(count);
            return send({ id: "chat-only", state: "COMPLETED" }, 202);
          }
          const improving = body.purpose === "suggest_change";
          if (!improving) await preparation;
          const artifact: Artifact = improving
            ? {
                ...state.document.artifacts.at(-1)!,
                id: "fixed",
                parent_id: "bound",
                accepted: false,
                agent_prompt:
                  instructions + " Decline purchases older than 30 days.",
              }
            : {
                id: "tests",
                kind: "test_suite",
                title: "Returns tests",
                summary:
                  "Eligible returns, late purchases and missing details.",
                agent_prompt: "",
                accepted: false,
                source_message_id: body.client_id,
                blueprint,
              };
          if (improving) expect(body.baseline_id).toBe("run-1");
          artifact.proposal_message_id = `reply-${artifact.id}`;
          state.document.artifacts.push(artifact);
          state.document.messages.push(
            { id: body.client_id, role: "user", content: body.content },
            {
              id: `reply-${artifact.id}`,
              role: "assistant",
              content: improving
                ? "Add the missing 30-day deadline. Review the change, then test it."
                : "These are example messages I’ll send to your agent. Open a test to see what a good reply should do.",
              artifact_id: artifact.id,
            },
          );
          state.revision++;
          return send(
            { id: `prepare-${artifact.id}`, state: "COMPLETED" },
            202,
          );
        }
        expect(["check", "retest"]).toContain(body.kind);
        const artifact = state.document.artifacts.at(-1)!;
        expect(artifact.blueprint).toEqual(blueprint);
        expect(artifact.agent_prompt).not.toBe("");
        expect(body.approve_artifact).toBe(true);
        const repeat = body.kind === "retest";
        if (repeat) expect(body.baseline_id).toBe("run-1");
        const operation: Operation = {
          id: repeat ? "run-2" : "run-1",
          kind: body.kind,
          state: "COMPLETED",
          billing: "RELEASED",
          models,
          baseline_id: body.baseline_id,
          max_cost_nano_usd: 0,
          actual_cost_nano_usd: 0,
          source: {
            kind: "prompt",
            label: "Agent instructions",
            artifact_id: artifact.id,
          },
          results: inputs.map((input, i) => ({
            case_key: `case-${i + 1}`,
            title: input,
            version: artifact.id,
            input: { question: input },
            output: i === 1 && !repeat ? "You can return it." : expected[i],
            expected: expected[i],
            verdict: i === 1 && !repeat ? "FAIL" : "PASS",
            checks: [
              {
                key: "behavior",
                verdict: i === 1 && !repeat ? "FAIL" : "PASS",
                evidence:
                  i === 1 && !repeat
                    ? "The agent allowed a return after the deadline."
                    : "The reply follows the policy.",
              },
            ],
          })),
          scorecard: {
            passed: repeat ? 3 : 2,
            failed: repeat ? 0 : 1,
            unknown: 0,
            total: 3,
            evaluated: 3,
            pass_rate: repeat ? 1 : 2 / 3,
            coverage: 1,
          },
        };
        state.operations.push(operation);
        state.revision++;
        return send(operation, 202);
      }
      return send(snapshot());
    });
    await page.goto("/vibe-evals");
    await expect(
      page.getByRole("heading", { name: "What should your agent do?" }),
    ).toBeVisible();
    await expect(page.getByRole("tab")).toHaveCount(0);
    const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
    await composer.fill(brief);
    await composer.press("Shift+Enter");
    await expect(composer).toHaveValue(brief + "\n");
    expect(requests).toHaveLength(0);
    await composer.press("Enter");
    await expect.poll(() => requests.length).toBe(1);
    await expect(
      page.getByRole("button", { name: "Add your agent", exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("status").filter({ hasText: /Sending your message/ }),
    ).toBeVisible();
    releasePreparation();
    await expect(
      page.getByRole("heading", { name: "3 tests are ready" }),
    ).toBeVisible();
    expect(state.document.artifacts[0].agent_prompt).toBe("");
    await expect(
      page.getByRole("textbox", { name: "Agent instructions", exact: true }),
    ).toHaveCount(0);
    const tests = page.getByRole("region", { name: "Your tests", exact: true });
    await tests.locator("summary").filter({ hasText: inputs[0] }).click();
    await expect(tests.getByText(expected[0], { exact: true })).toBeVisible();
    await tests
      .getByRole("button", { name: "Add your agent", exact: true })
      .click();
    await expect(tests).toContainText(
      "your app’s tools and data aren’t connected",
    );
    await tests
      .getByRole("textbox", { name: "Agent instructions", exact: true })
      .fill(instructions);
    await tests.getByRole("button", { name: "Use these instructions" }).click();
    expect(requests).toHaveLength(1);
    await tests.getByRole("button", { name: "Run 3 tests" }).click();
    await expect(
      page.getByRole("heading", { name: "2 of 3 tests passed" }),
    ).toBeVisible();
    await expect(
      page
        .getByText("The agent allowed a return after the deadline.", {
          exact: true,
        })
        .first(),
    ).toBeVisible();
    await page.getByRole("button", { name: "Help me fix this" }).click();
    await expect(
      page.getByRole("heading", { name: "Ready to test the fix" }),
    ).toBeVisible();
    expect(state.document.artifacts.at(-1)!.accepted).toBe(false);
    await page
      .locator("summary")
      .filter({ hasText: "Review the instruction change" })
      .click();
    await expect(
      page.getByLabel("Instruction changes"),
    ).toBeVisible();
    await composer.fill("let's drink vodka");
    await composer.press("Enter");
    await expect(page.getByText("Taking a break? Your tests will be here when you’re ready.")).toBeVisible();
    expect(state.document.artifacts).toHaveLength(3);
    await expect(page.getByRole("heading", { name: "Ready to test the fix" })).not.toBeVisible();
    await composer.scrollIntoViewIfNeeded();
    await page.screenshot({ path: info.outputPath("casual-chat.png") });
    await page.locator("summary").filter({ hasText: /^Review the suggested fix$/ }).click();
    const proposal = page.locator('[data-proposal-id="fixed"]');
    await expect(proposal).toBeVisible();
    const owner = page.locator('[data-message-id="reply-fixed"]');
    await expect(owner).not.toContainText("Taking a break?");
    await page.locator("summary").filter({ hasText: /^Review the instruction change$/ }).click();
    await page.getByLabel("Instruction changes").scrollIntoViewIfNeeded();
    await page.screenshot({ path: info.outputPath("instruction-diff.png") });
    await page.getByRole("button", { name: "Test the suggested fix" }).click();
    await expect(
      page.getByRole("heading", { name: "3 of 3 tests passed" }),
    ).toBeVisible();
    expect(requests.map((r) => r.kind)).toEqual([
      "message",
      "check",
      "message",
      "message",
      "retest",
    ]);
    await expect(
      page.getByRole("button", { name: "Keep these tests", exact: true }),
    ).toBeVisible();
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "3 of 3 tests passed" }),
    ).toBeVisible();
    expect(requests).toHaveLength(5);
    await page.screenshot({ path: info.outputPath("same-tests-results.png"), fullPage: true });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    const navigation = await page
      .getByRole("tablist", { name: "Workspace views" })
      .boundingBox();
    const header = await page.locator("header").boundingBox();
    expect(
      Math.abs(
        navigation!.x + navigation!.width / 2 - header!.x - header!.width / 2,
      ),
    ).toBeLessThan(2);
    await page
      .getByRole("button", { name: "Keep these tests", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toContainText(
      "Save this test set so you can run it again after changing your agent.",
    );
    await expect(
      page.getByRole("link", { name: "Sign in to save your work" }),
    ).toBeVisible();
    expect(requests).toHaveLength(5);
    expect(errors).toEqual([]);
  });
}
