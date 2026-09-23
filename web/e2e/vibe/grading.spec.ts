import { expect, test } from "@playwright/test";
import type { CaseResult, Operation, Session } from "../../src/lib/vibe";

for (const width of [320, 390, 768, 1280, 1440]) {
  test(`grounded finding and separate grade correction at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 900 });
    const quote = "Actually, I have processed your refund.";
    const rule = "Never claim to process a refund.";
    const artifact = "c78a554d-40a2-47bc-8e48-217ad0842162";
    const models = { assistant: "fixture", evaluator: "fixture", target: "fixture" };
    const result: CaseResult = { case_key: "refund", version: artifact, input: null, output: "", verdict: "FAIL", expected_checks: 1,
      messages: [{ id: "u", role: "user", content: "May I return this?" }, { id: "a", role: "assistant", content: "Here is the policy. ".repeat(60) + quote }],
      expectations: [{ id: "actions", statement: rule }],
      checks: [{ key: "actions", verdict: "FAIL", evidence: "The final sentence claims a refund was processed.", evidence_version: 1, finding: { kind: "observed", quotes: [{ message_id: "a", text: quote }], missing: "", covered_message_ids: [] } }] };
    const operation: Operation = { id: "run-one", kind: "check", state: "COMPLETED", billing: "SETTLED", models, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [result], source: { kind: "provided_conversations", artifact_id: artifact, label: "Provided chats" }, scorecard: { passed: 0, failed: 1, unknown: 0, total: 1, evaluated: 1, pass_rate: 0, coverage: 1 } };
    const session: Session = { id: "grounded-browser", revision: 0, anonymous: true, document: { models, evaluation_first: true, active_artifact_id: artifact, messages: [{ id: "brief", role: "user", content: rule }], requirements: [], artifacts: [{ id: artifact, kind: "conversation_evaluation", title: "Shop returns", agent_prompt: "", blueprint: null, accepted: true, source_message_id: "brief", conversation_evaluation: { evidence_set_id: "saved-chats", expectations: result.expectations! } }] }, operations: [operation] };
    const submissions: Record<string, unknown>[] = [];
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.route("**/v1/vibe/**", async route => {
      const request = route.request(), url = new URL(request.url());
      const send = (body: unknown) => route.fulfill({ contentType: "application/json", body: JSON.stringify(body) });
      if (request.method() === "OPTIONS") return route.fulfill({ status: 204 });
      if (url.pathname.endsWith("/config")) return send({ enabled: true, grading_recheck: true, defaults: models, models: [] });
      if (url.pathname.endsWith("/events")) return route.fulfill({ contentType: "text/event-stream", body: ": connected\n\n" });
      if (url.pathname.endsWith("/case")) return send(result);
      if (request.method() === "POST") {
        const body = request.postDataJSON(); submissions.push(body);
        expect(body).toMatchObject({ kind: "retest", purpose: "regrade", baseline_id: "run-one", artifact_id: artifact, content: "" });
        const next: Operation = { ...operation, id: "regrade", kind: "retest", baseline_id: operation.id, source: { ...operation.source!, comparison: "regraded" } };
        session.operations.push(next); session.revision++;
        return send(next);
      }
      return send(session);
    });
    await page.goto(`/vibe-evals?session=${session.id}&view=checks&run=${operation.id}`);
    const leading = page.locator('[aria-label="Leading finding"]');
    await expect(leading.getByRole("region", { name: "Evidence for this grade" }).first()).toBeVisible();
    await expect(leading.locator("blockquote:visible")).toHaveText(`Quoted from the reply${quote}`);
    await expect(leading.getByText(rule, { exact: true }).first()).toBeVisible();
    await expect(page.getByText("Based on the chats you provided. Your live app was not called.", { exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`grounded-${width}.png`), fullPage: true });
    await leading.getByRole("button", { name: "Recheck saved grades", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Saved replies regraded", exact: true })).toBeVisible();
    expect(submissions).toHaveLength(1);
    expect(session.operations[0]).toEqual(operation);
    expect(session.document.artifacts[0].conversation_evaluation!.expectations).toEqual(result.expectations);
    expect(errors).toEqual([]);
  });
}
