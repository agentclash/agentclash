import { expect, test, type Page } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import type { Artifact, Operation, Session } from "../../src/lib/vibe";

const api = `http://127.0.0.1:${process.env.VIBE_BROWSER_API_PORT ?? "55441"}`;
const reviewVersion = process.env.VIBE_BROWSER_REVIEW_VERSION ?? "suite-review-v3";
type Evidence = {
  session: Session;
  hashes: Record<string, { blueprint: string; grading: string }>;
  calls: { role: string; request?: { id: string; text: string } }[];
  attempt_count: number;
  unsettled_attempts: number;
  delivered_operations: number;
  temporal_namespace: string;
};

async function evidence(page: Page, session: string): Promise<Evidence> {
  const response = await page.request.get(`${api}/__fixture/evidence?session=${session}`);
  expect(response.ok(), await response.text()).toBeTruthy();
  return response.json();
}

async function finished(page: Page, session: string, previousCount: number): Promise<Operation> {
  let operation: Operation | undefined;
  await expect.poll(async () => {
    const state = await evidence(page, session);
    operation = state.session.operations.at(-1);
    return state.session.operations.length > previousCount &&
      !!operation && ["COMPLETED", "FAILED", "PARTIAL", "CANCELLED"].includes(operation.state);
  }, { message: "real Temporal operation reaches a persisted terminal state", timeout: 60_000 }).toBe(true);
  return operation!;
}

async function openTests(page: Page) {
  const conversation = page.getByRole("tab", { name: "Conversation", exact: true });
  if (await conversation.count()) await conversation.click();
  const summary = page.locator("summary").filter({ hasText: /^Review (your tests|the suggested fix)$/ }).last();
  if (await summary.count() && !(await summary.evaluate(node => (node.parentElement as HTMLDetailsElement).open))) await summary.click();
}

test.afterEach(async ({ page }, info) => {
  const session = new URL(page.url()).searchParams.get("session");
  if (!session) return;
  const response = await page.request.get(`${api}/__fixture/evidence?session=${session}`);
  const file = info.outputPath("persisted-evidence.json");
  await writeFile(file, response.ok() ? JSON.stringify(await response.json(), null, 2) : await response.text());
  await info.attach("persisted-evidence", { path: file, contentType: "application/json" });
});

test("real API, PostgreSQL and Temporal preserve the suite across failures, retry and an instruction fix", async ({ page }, info) => {
  // Deliberately no page.route: every application request reaches the fixture API.
  const pageErrors: string[] = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  const original = await readFile(path.resolve(process.cwd(), "../backend/internal/vibe/testdata/reliability/returns-original-request.txt"), "utf8");
  await page.goto("/vibe-evals");
  await expect(page.getByRole("heading", { name: "What should your agent do?" })).toBeVisible();
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  await composer.fill(original);
  await composer.press("Enter");
  await expect(page.getByRole("heading", { name: "3 tests are ready" })).toBeVisible();
  const sessionID = new URL(page.url()).searchParams.get("session");
  expect(sessionID).toBeTruthy();
  let state = await evidence(page, sessionID!);
  expect(state.session.document.messages.find(message => message.role === "user")?.content).toBe(original);
  const preparation = state.session.operations[0];
  expect(preparation.state).toBe("COMPLETED");
  expect(preparation.completion_receipt).toMatchObject({ action: "prepare_tests", case_count: 3 });
  const prepared = state.session.document.artifacts.at(-1)!;
  expect(prepared.validation?.status).toBe("supported");
  expect(prepared.validation?.validator_version).toBe(reviewVersion);
  expect(prepared.agent_prompt).toBe("");
  expect(state.calls.filter(call => call.request).every(call => call.request!.text === original)).toBe(true);
  expect(state.calls.map(call => call.role)).toEqual(["vibe_route_v11", "vibe_prepare_tests_v11", `vibe_${reviewVersion.replaceAll("-", "_")}`]);
  await page.reload();
  await openTests(page);
  const tests = page.getByRole("region", { name: "Your tests", exact: true }).last();
  await tests.getByRole("button", { name: "Add your agent", exact: true }).click();
  await tests.getByRole("textbox", { name: "Agent instructions", exact: true }).fill("Opened items are eligible. Unopened items within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund.");
  await tests.getByRole("button", { name: "Use these instructions" }).click();
  await tests.getByRole("button", { name: "Run 3 tests", exact: true }).click();
  await expect(page.getByRole("heading", { name: "2 of 3 tests passed" })).toBeVisible();
  state = await evidence(page, sessionID!);
  const baseline = state.session.operations.at(-1)!;
  expect(baseline.state).toBe("COMPLETED");
  const baselineArtifact = state.session.document.artifacts.at(-1)!;
  const originalHashes = state.hashes[baselineArtifact.id];
  expect(originalHashes).toEqual(state.hashes[prepared.id]);

  await page.getByRole("button", { name: "Help me fix this", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Ready to test the fix" })).toBeVisible();
  state = await evidence(page, sessionID!);
  const fix = state.session.document.artifacts.at(-1)!;
  expect(state.hashes[fix.id]).toEqual(originalHashes);
  expect(fix.agent_prompt).toBe(baselineArtifact.agent_prompt.replace("Opened items are eligible.", "Only unopened items are eligible."));
  await page.getByRole("button", { name: "Test the suggested fix", exact: true }).click();
  await expect(page.getByRole("heading", { name: "3 of 3 tests passed" })).toBeVisible();
  state = await evidence(page, sessionID!);
  const rerun = state.session.operations.at(-1)!;
  expect(rerun.baseline_id).toBe(baseline.id);
  expect(rerun.models.evaluator).toBe(baseline.models.evaluator);
  expect(state.hashes[fix.id]).toEqual(originalHashes);
  await page.screenshot({ path: info.outputPath("same-tests-fix.png") });
  expect(rerun.grading?.hash).toBe(baseline.grading?.hash);
  expect(rerun.target_config?.instructions_hash).not.toBe(baseline.target_config?.instructions_hash);

  // A grade dispute reuses replies. It cannot call the target or edit the suite.
  const beforeRegrade = state;
  const passedResults = page.locator("summary").filter({ hasText: /^Test results/ });
  await passedResults.click();
  await page.locator('[aria-label="Individual results"] summary').first().click();
  await expect(page.getByRole("region", { name: "Evidence for this grade" }).first()).toBeVisible();
  await page.getByRole("button", { name: "Recheck saved grades", exact: true }).first().click();
  const regraded = await finished(page, sessionID!, beforeRegrade.session.operations.length);
  expect(regraded.source?.comparison).toBe("regraded");
  expect(regraded.baseline_id).toBe(rerun.id);
  await expect(page.getByRole("heading", { name: "Saved replies regraded", exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name: "Saved replies regraded", exact: true })).toBeVisible();
  state = await evidence(page, sessionID!);
  expect(state.calls.slice(beforeRegrade.calls.length).map(call => call.role)).toEqual(["judge", "judge", "judge"]);
  expect(state.session.document.artifacts).toEqual(beforeRegrade.session.document.artifacts);
  expect(state.session.operations.find(op => op.id === baseline.id)?.scorecard).toEqual(baseline.scorecard);
  await page.screenshot({ path: info.outputPath("saved-grades-rechecked.png") });


  // A direct editor request must receive the same real policy review as chat.
  await openTests(page);
  await tests.getByRole("button", { name: "Edit tests", exact: true }).click();
  const contradictoryExpected = tests.getByRole("textbox", { name: "What should happen", exact: true }).nth(1);
  await contradictoryExpected.fill("Confirm opened items are eligible.");
  const beforeManual = state.session.operations.length;
  await tests.getByRole("button", { name: "Save test changes", exact: true }).click();
  const rejected = await finished(page, sessionID!, beforeManual);
  expect(rejected.state).toBe("FAILED");
  expect(rejected.error?.code).toBe("test_policy_conflict");
  await expect(contradictoryExpected).toHaveValue("Confirm opened items are eligible.");
  state = await evidence(page, sessionID!);
  expect(state.session.document.artifacts.at(-1)!.id).toBe(fix.id);
  expect(state.hashes[fix.id]).toEqual(originalHashes);
  await tests.getByRole("button", { name: "Cancel", exact: true }).click();

  // Fail both bounded author attempts, then retry the admitted operation itself.
  const control = await page.request.post(`${api}/__fixture/control`, { data: { fail_edit_calls: 2 } });
  expect(control.ok()).toBeTruthy();
  const addRequest = 'Add a test using this exact message: "Can you recommend a vodka cocktail?" Expected: politely bring the customer back to shop returns.';
  const beforeEdit = state.session.operations.length;
  await composer.fill(addRequest);
  await composer.press("Enter");
  const failed = await finished(page, sessionID!, beforeEdit);
  expect(failed.state).toBe("FAILED");
  expect(failed.retryable).toBe(true);
  state = await evidence(page, sessionID!);
  expect(state.hashes[fix.id]).toEqual(originalHashes);
  expect(state.session.document.artifacts.at(-1)!.id).toBe(fix.id);
  const source = state.session.document.messages.find(message => message.operation_id === failed.id && message.role === "user")!;
  const failedTurn = page.locator(`[data-message-id="${source.id}"]`);
  await expect(failedTurn.getByRole("alert")).toBeVisible();
  await composer.fill("Thanks, I am getting coffee.");
  await composer.press("Enter");
  await finished(page, sessionID!, state.session.operations.length);
  await page.reload();
  await expect(failedTurn.getByRole("alert")).toBeVisible();
  await expect(failedTurn.getByRole("button", { name: "Retry", exact: true })).toHaveCount(1);
  const draft = "A separate question that must stay in the composer.";
  await composer.fill(draft);
  state = await evidence(page, sessionID!);
  await failedTurn.getByRole("button", { name: "Retry", exact: true }).click();
  const retried = await finished(page, sessionID!, state.session.operations.length);
  expect(retried.state, JSON.stringify(retried.error)).toBe("COMPLETED");
  expect(retried.retry_of_operation_id).toBe(failed.id);
  expect(retried.completion_receipt).toMatchObject({ action: "edit_tests", source_message_id: source.id, case_count: 4, changed_case_count: 1 });
  await expect(failedTurn.getByRole("status")).toHaveText("Completed on retry.");
  await expect(composer).toHaveValue(draft);
  await expect(failedTurn).toHaveCount(1);
  state = await evidence(page, sessionID!);
  expect(state.session.document.messages.filter(message => message.role === "user" && message.content === addRequest)).toHaveLength(1);
  expect(state.hashes[fix.id]).toEqual(originalHashes);
  const updated: Artifact = state.session.document.artifacts.at(-1)!;
  expect(updated.validation?.status).toBe("supported");
  expect((updated.blueprint as { cases: unknown[] }).cases).toHaveLength(4);
  expect((updated.blueprint as { cases: { payload: { question: string } }[] }).cases.at(-1)?.payload.question).toBe("Can you recommend a vodka cocktail?");
  expect(state.unsettled_attempts).toBe(0);
  expect(state.attempt_count).toBe(state.calls.length);
  expect(state.delivered_operations).toBe(state.session.operations.length);
  expect(state.temporal_namespace).toMatch(/^vibe-browser-/);
  expect(pageErrors).toEqual([]);
  await page.screenshot({ path: info.outputPath("recovered-operation.png") });
  await page.reload();
  await expect(failedTurn.getByRole("status")).toHaveText("Completed on retry.");
});

test("provider cooldown crosses the real worker and API and retry creates one new operation", async ({ page }, info) => {
  const errors: string[] = []; page.on("pageerror", error => errors.push(error.message));
  await page.request.post(`${api}/__fixture/control`, { data: { rate_limit_calls: 1 } });
  await page.goto("/vibe-evals");
  const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
  const original = await readFile(path.resolve(process.cwd(), "../backend/internal/vibe/testdata/reliability/returns-original-request.txt"), "utf8");
  await composer.fill(original);
  await composer.press("Enter");
  await expect(page.getByRole("button", { name: /Try again in/ })).toBeDisabled();
  const sessionID = new URL(page.url()).searchParams.get("session")!;
  let state = await evidence(page, sessionID);
  const failed = state.session.operations.at(-1)!;
  expect(failed.error?.code).toBe("provider_rate_limit");
  expect(failed.error?.retry_available_at).toBeTruthy();
  expect(failed.billing).toBe("RECONCILING");
  expect(failed.diagnostics?.unresolved_billing_since).toBeTruthy();
  const calls = state.calls.length;
  const early = await page.request.post(`${api}/v1/vibe/sessions/${sessionID}/operations/${failed.id}/retry`, {
    headers: { Origin: new URL(page.url()).origin },
    data: { client_id: await page.evaluate(() => crypto.randomUUID()), revision: state.session.revision },
  });
  expect(early.status()).toBe(429);
  expect(early.headers()["retry-after"]).toBeTruthy();
  expect((await early.json()).error.code).toBe("retry_cooldown");
  await page.reload();
  await expect(page.getByRole("button", { name: /Try again in/ })).toBeDisabled();
  await composer.fill("Keep my next message.");
  const retry = page.getByRole("button", { name: "Try again", exact: true });
  await expect(retry).toBeEnabled({ timeout: 30_000 });
  state = await evidence(page, sessionID);
  expect(state.calls.length).toBe(calls); // Countdown never dispatches.
  expect(state.session.operations).toHaveLength(1);
  await retry.click();
  const completed = await finished(page, sessionID, 1);
  expect(completed.state, JSON.stringify(completed.error)).toBe("COMPLETED");
  await expect(page.getByRole("heading", { name: "3 tests are ready" })).toBeVisible();
  await expect(composer).toHaveValue("Keep my next message.");
  state = await evidence(page, sessionID);
  expect(state.session.operations).toHaveLength(2);
  expect(state.session.operations[0].billing).toBe("RECONCILING");
  expect(state.session.operations[1].diagnostics?.retry_outcome).toBe("completed");
  expect(state.session.diagnostics?.first_useful_result_ms).toBeGreaterThanOrEqual(0);
  expect(state.session.document.messages.filter(message => message.role === "user")).toHaveLength(1);
  expect(errors).toEqual([]);
  await page.screenshot({ path: info.outputPath("rate-limit-recovered.png") });
});

test("real development-auth return keeps exact unrun tests, recovers a lost save response and reopens for a run", async ({ page }, info) => {
  const errors: string[]=[]; page.on("pageerror",e=>errors.push(e.message));
  const original=await readFile(path.resolve(process.cwd(),"../backend/internal/vibe/testdata/reliability/returns-original-request.txt"),"utf8");
  await page.goto("/vibe-evals");
  await page.getByRole("textbox",{name:"Message Vibe Evals"}).fill(original);
  await page.getByRole("textbox",{name:"Message Vibe Evals"}).press("Enter");
  await expect(page.getByRole("heading",{name:"3 tests are ready"})).toBeVisible();
  const id=new URL(page.url()).searchParams.get("session")!;
  const before=await evidence(page,id); const artifact=before.session.document.artifacts.at(-1)!;
  await page.getByRole("button",{name:"I don’t have an agent yet",exact:true}).click();
  await page.getByRole("button",{name:"Keep these tests",exact:true}).click();
  const link=page.getByRole("link",{name:"Sign in to save your work"});
  expect(new URL((await link.getAttribute("href"))!,page.url()).searchParams.get("returnTo")).toContain(`agent=${artifact.id}`);
  await link.click();
  await expect(page.getByRole("button",{name:"Continue with AgentClash"})).toBeVisible();
  // Cancel once: the original anonymous cookie and preparation remain valid.
  await page.goBack();
  await expect(page.getByRole("heading",{name:"3 tests are ready"})).toBeVisible();
  await page.getByRole("button",{name:"I don’t have an agent yet",exact:true}).click();
  await page.getByRole("button",{name:"Keep these tests",exact:true}).click();
  await page.getByRole("link",{name:"Sign in to save your work"}).click();
  await page.getByRole("button",{name:"Continue with AgentClash"}).click();
  await expect(page).toHaveURL(/vibe-evals\?.*keep=1/);
  await expect(page.getByRole("dialog").getByLabel("Save workspace")).toHaveValue("a8000000-0000-4000-8000-000000000003");
  await expect(page.getByRole("dialog").getByRole("button",{name:"Keep these tests",exact:true})).toBeEnabled();
  const returned=await evidence(page,id);
  expect(returned.calls.length).toBe(before.calls.length);
  expect(returned.session.operations).toHaveLength(before.session.operations.length);
  expect(returned.hashes[artifact.id]).toEqual(before.hashes[artifact.id]);
  const savedResponse=page.waitForResponse(r=>r.url().endsWith(`/sessions/${id}/save`) && r.request().method()==="POST");
  await page.getByRole("dialog").getByRole("button",{name:"Keep these tests",exact:true}).click();
  const response=await savedResponse;expect(response.ok(),await response.text()).toBeTruthy();
  const receipt=await response.json();
  await expect(page.getByRole("dialog").getByRole("button",{name:"Saved",exact:true})).toBeDisabled();
  // Replay the exact body/revision, as if the successful response were lost.
  const headers=response.request().headers();
  const duplicate=await page.request.post(response.url(),{data:response.request().postDataJSON(),headers:{Authorization:headers.authorization}});
  expect(duplicate.ok(),await duplicate.text()).toBeTruthy();
  expect(await duplicate.json()).toEqual(receipt);
  const kept=await evidence(page,id);
  expect(kept.session.workspace_id).toBe("a8000000-0000-4000-8000-000000000003");
  expect(kept.session.anonymous).toBe(false);
  expect(kept.calls.length).toBe(before.calls.length);
  expect(kept.session.document.artifacts.at(-1)?.id).toBe(artifact.id);
  await page.screenshot({path:info.outputPath("kept-after-real-auth-return.png")});
  const full=page.getByRole("link",{name:"Open in full pack builder"});
  expect(await full.getAttribute("href")).toContain(receipt.draft_id);
  await page.getByRole("link",{name:"Open saved tests",exact:true}).click();
  await expect(page.getByRole("heading",{name:"3 tests are ready"})).toBeVisible();
  expect(new URL(page.url()).searchParams.get("agent")).toBe(artifact.id);
  const tests=page.getByRole("region",{name:"Your tests",exact:true}).last();
  await tests.getByRole("button",{name:"Add your agent",exact:true}).click();
  await tests.getByRole("textbox",{name:"Agent instructions",exact:true}).fill("Only unopened items within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund.");
  await tests.getByRole("button",{name:"Use these instructions"}).click();
  await tests.getByRole("button",{name:"Run 3 tests",exact:true}).click();
  await expect(page.getByRole("heading",{name:"3 of 3 tests passed"})).toBeVisible();
  const ran=await evidence(page,id);
  expect(ran.hashes[ran.session.document.artifacts.at(-1)!.id]).toEqual(before.hashes[artifact.id]);
  expect(ran.session.operations.filter(o=>o.kind==="check")).toHaveLength(1);
  expect(errors).toEqual([]);
});
