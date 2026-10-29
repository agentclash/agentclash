// Screenshot review of the main Vibe Evals screens in light and dark appearance,
// at desktop and phone widths. Every `/v1/vibe/**` request is fulfilled here, so
// no API or model is reached. Run it before and after a UI change and compare:
//   VIBE_SCREENS=1 VIBE_SCREENS_DIR=test-results/screens-before npx playwright test --config playwright.vibe.config.ts appearance-screens
import { mkdir, rm } from "node:fs/promises";
import path from "node:path";
import { expect, test, type Page } from "@playwright/test";
import { defaultModels, type Artifact, type CaseResult, type Operation, type Session } from "../../src/lib/vibe";
import { actionSession } from "./interaction-fixture";

test.skip(process.env.VIBE_SCREENS !== "1", "Screenshot review only; enable VIBE_SCREENS=1.");

// Relative folders resolve from web/, wherever the command runs.
const outDir = path.resolve(__dirname, "../..", process.env.VIBE_SCREENS_DIR ?? "test-results/vibe-screens");
const viewports = [{ width: 1440, height: 960 }, { width: 390, height: 844 }] as const;
const schemes = ["dark", "light"] as const;
type Combo = { width: number; height: number; scheme: (typeof schemes)[number] };

const models = defaultModels;
const config = {
  enabled: true,
  availability: { guest: { available: true }, workspace: { available: true } },
  two_door: true,
  interaction_actions: true,
  defaults: models,
  models: [...new Set(Object.values(models))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })),
};

const instructions = "Help shoppers with returns. Allow unopened items within 30 days of purchase. Decline later returns politely. Ask for missing details before deciding.";
const inputs = ["Unopened, bought 10 days ago. Can I return it?", "Unopened, bought 45 days ago. Can I return it?", "I want to return something."];
const expected = ["Explain that it is eligible.", "Decline the late return.", "Ask for purchase age and item condition."];
const blueprint = {
  cases: inputs.map((question, i) => ({ key: `case-${i + 1}`, payload: { question }, expectations: [{ key: "expected_behavior", kind: "text", value: expected[i] }] })),
  judges: [{ key: "behavior", context_from: ["case.expectations.expected_behavior"], assertion: "The response meets this case's expected_behavior and the following shared rules:\n\nUnopened items within 30 days." }],
  validators: [{ key: "has_answer" }],
  dimensions: [{}, {}],
};
const prototype = (id: string): Artifact => ({
  id, kind: "test_suite", title: "Shop returns assistant", summary: "Eligible returns, late purchases and missing details.",
  agent_prompt: instructions, accepted: true, source_message_id: "request", proposal_message_id: "prepared",
  scope_note: "Answers return questions using the policy you supplied.", blueprint,
});
const caseResults = (version: string): CaseResult[] => inputs.map((input, i) => ({
  case_key: `case-${i + 1}`, title: input, version, input: { question: input },
  output: i === 1 ? "Yes, you can return it. Just bring the receipt to any store." : i === 0 ? "Yes. Unopened items bought within 30 days are eligible for a return." : "Happy to help. How long ago did you buy it, and is it still unopened?",
  expected: expected[i], verdict: i === 1 ? "FAIL" : "PASS", expected_checks: 1,
  messages: [{ id: `u-${i}`, role: "user", content: input }, { id: `a-${i}`, role: "assistant", content: i === 1 ? "Yes, you can return it. Just bring the receipt to any store." : "The reply follows the policy." }],
  expectations: [{ id: "behavior", statement: expected[i] }],
  checks: [{
    key: "behavior", verdict: i === 1 ? "FAIL" : "PASS", evidence: i === 1 ? "The agent allowed a return 45 days after purchase, past the 30-day limit." : "The reply follows the policy.",
    ...(i === 1 ? { evidence_version: 1, finding: { kind: "observed" as const, quotes: [{ message_id: "a-1", text: "Yes, you can return it." }], missing: "", covered_message_ids: [] } } : {}),
  }],
}));

function base(id: string, door: "build" | "test", title?: string): Session {
  return {
    id, revision: 1, event_cursor: 1, anonymous: true, operations: [], ...(title ? { title } : {}),
    updated_at: new Date(Date.now() - id.length * 60_000).toISOString(),
    document: { format_version: 1, models, messages: [], requirements: [], artifacts: [], test_journey: true, evaluation: { id, chat_id: "chat", door } },
  };
}

function fixtures() {
  const buildStart = base("build-start", "build");
  const improveStart = base("improve-start", "test");

  // Build chat: an assistant clarifying question with choices.
  const question = structuredClone(actionSession);
  question.id = "build-question"; question.title = "Return policy helper"; question.event_cursor = 1;
  question.document = { ...question.document, format_version: 1, test_journey: true, evaluation: { id: question.id, chat_id: "chat", door: "build" },
    build: { cycle_id: "cycle-q", phase: "clarifying", clarifications_used: 1 } };

  // Same question outside the "clarifying" phase, where the choice buttons render in the main pane.
  const choices = structuredClone(question);
  choices.id = "build-choices"; choices.title = "Return window helper";
  choices.document.evaluation = { id: choices.id, chat_id: "chat", door: "build" };
  choices.document.build = { cycle_id: "cycle-c", phase: "waiting", clarifications_used: 1 };

  // Build chat: assistant reply, then a running turn that is creating the first version.
  const progress = base("build-progress", "build", "Refund desk prototype");
  progress.document.build = { cycle_id: "cycle-p", phase: "preparing", clarifications_used: 1 };
  progress.document.messages = [
    { id: "p-1", role: "user", content: "My agent answers shop return questions.", operation_id: "p-op-1" },
    { id: "p-2", role: "assistant", content: "How long can items be returned: 14 days or 30 days? And do opened items count?", operation_id: "p-op-1" },
    { id: "p-3", role: "user", content: "30 days, unopened only. Ask for missing details.", operation_id: "p-op-2" },
  ];
  progress.operations = [
    { id: "p-op-1", kind: "message", state: "COMPLETED", billing: "SETTLED", models, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] },
    { id: "p-op-2", kind: "message", state: "RUNNING", billing: "RESERVED", models, max_cost_nano_usd: 1, actual_cost_nano_usd: 0, results: [],
      conversation_decision: { intent: "prepare_tests", source_message_id: "p-3" },
      progress: { phase: "preparing", completed_cases: 0, total_cases: 3, phase_elapsed_ms: 12_000 } },
  ];

  // Tests ready, not yet run (Improve door).
  const ready = base("returns-ready", "test", "Support agent checks");
  ready.document.artifacts = [{ ...prototype("ready-v1"), title: "Support agent checks" }];
  ready.document.active_artifact_id = "ready-v1";
  ready.document.messages = [
    { id: "request", role: "user", content: "Check my returns agent against our 30-day policy.", operation_id: "r-prepare" },
    { id: "prepared", role: "assistant", content: "These are example messages I’ll send to your agent. Open a test to see what a good reply should do.", operation_id: "r-prepare", artifact_id: "ready-v1" },
  ];
  ready.operations = [{ id: "r-prepare", kind: "message", state: "COMPLETED", billing: "SETTLED", models, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] }];

  // Completed run with one failure (Build door).
  const results = base("returns-results", "build", "Shop returns assistant");
  results.document.artifacts = [prototype("proto-v1")];
  results.document.active_artifact_id = "proto-v1";
  results.document.build = { cycle_id: "cycle-r", phase: "results", clarifications_used: 1, artifact_id: "proto-v1", check_id: "run-1" };
  results.document.messages = [
    { id: "request", role: "user", content: "Build an agent that answers shop return questions. Unopened items within 30 days.", operation_id: "prepare" },
    { id: "prepared", role: "assistant", content: "I made a first version and three example situations. Two passed. The late-return case failed: the agent accepted a 45-day-old purchase.", operation_id: "prepare", artifact_id: "proto-v1" },
  ];
  results.rule_coverage = { "proto-v1": [{ rule_id: "late", statement: "Opened items are never returnable", case_keys: [] }] };
  const run: Operation = {
    id: "run-1", kind: "check", state: "COMPLETED", billing: "SETTLED", models, max_cost_nano_usd: 120_000_000, actual_cost_nano_usd: 41_000_000,
    source: { kind: "prompt", label: "Shop returns assistant", artifact_id: "proto-v1" }, results: caseResults("proto-v1"),
    scorecard: { passed: 2, failed: 1, unknown: 0, total: 3, evaluated: 3, pass_rate: 2 / 3, coverage: 1 },
  };
  results.operations = [
    { id: "prepare", kind: "message", state: "COMPLETED", billing: "SETTLED", models, max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [] },
    run,
  ];
  return new Map<string, Session>([results, ready, progress, question, choices, improveStart, buildStart].map(s => [s.id, s]));
}

async function serve(page: Page) {
  const sessions = fixtures();
  const unknown: string[] = [], errors: string[] = [];
  page.on("pageerror", e => errors.push(e.message));
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request(), url = new URL(request.url()), p = url.pathname;
    const headers = { "Access-Control-Allow-Origin": new URL(page.url() || "http://127.0.0.1:53517").origin, "Access-Control-Allow-Credentials": "true" };
    const json = (body: unknown, status = 200) => route.fulfill({ status, headers, json: body });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, "Access-Control-Allow-Headers": "Content-Type,Authorization,If-Match", "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS" } });
    if (p.endsWith("/config")) return json(config);
    if (p.endsWith("/saved-checks") || p.endsWith("/grade-reviews")) return json([]);
    if (p.endsWith("/credits")) return json({ under_review: false, balance_nano_usd: 0, held_nano_usd: 0, available_nano_usd: 0, products: [] });
    if (p.endsWith("/case")) {
      const key = url.searchParams.get("key");
      for (const s of sessions.values()) for (const op of s.operations) if (p.includes(`/${op.id}/`)) { const hit = op.results.find(r => r.case_key === key); if (hit) return json(hit); }
      unknown.push(`GET ${p}?key=${key}`);
      return json({ error: { message: "missing case" } }, 404);
    }
    const id = p.split("/sessions/")[1]?.split("/")[0];
    const state = id ? sessions.get(id) : undefined;
    if (request.method() === "POST") {
      const body = request.postDataJSON?.() ?? {};
      if (p.endsWith("/run-quote")) return json({ id: "quote-1", max_cost_nano_usd: 120_000_000, cases: 3, calls: 6, expires_at: new Date(Date.now() + 600_000).toISOString() });
      if (p.endsWith("/build-quote")) return json({ id: "build-quote-1", max_calls: 9, cases: 3, max_cost_nano_usd: 250_000_000, request: { content: body.content, models: body.models, ...(body.inputs ? { inputs: body.inputs } : {}), ...(body.adopt_rules ? { adopt_rules: body.adopt_rules } : {}) }, expires_at: new Date(Date.now() + 600_000).toISOString() });
      if (p.endsWith("/sessions")) { const created = base(body.id, body.door || "build"); sessions.set(created.id, created); return json(created, 201); }
      unknown.push(`POST ${p}`);
      return json({ error: { message: "Audit fixture does not run anything." } }, 400);
    }
    if (p.endsWith("/sessions")) return json([...sessions.values()]);
    if (p.endsWith("/events")) return route.fulfill({ headers, contentType: "text/event-stream", body: state ? `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` : ": connected\n\n" });
    if (state && request.method() === "GET" && p.endsWith(`/sessions/${id}`)) return json(state);
    unknown.push(`${request.method()} ${p}`);
    return json({});
  });
  return { sessions, unknown, errors };
}

async function prepare(page: Page, combo: Combo) {
  await page.setViewportSize({ width: combo.width, height: combo.height });
  await page.emulateMedia({ colorScheme: combo.scheme, reducedMotion: "reduce" });
  // Hide the Next.js dev-tools badge; it is a dev-server artifact, not product UI.
  await page.addInitScript(() => {
    const hide = () => { const style = document.createElement("style"); style.textContent = "nextjs-portal{display:none!important}"; document.head.appendChild(style); };
    if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", hide); else hide();
  });
}

const name = (state: string, combo: Combo, suffix = "") => `${state}-${combo.width}-${combo.scheme}${suffix}.png`;

async function shoot(page: Page, state: string, combo: Combo, opts: { tall?: boolean } = {}) {
  await mkdir(outDir, { recursive: true });
  await page.waitForTimeout(350);
  await page.screenshot({ path: path.join(outDir, name(state, combo)), fullPage: true });
  if (!opts.tall) return;
  // Remove a tall capture from an earlier run, so a folder never mixes old and new evidence.
  await rm(path.join(outDir, name(state, combo, "-tall")), { force: true });
  // The app is an h-dvh shell with inner scrollers, so a full-page capture equals
  // the viewport. Grow the viewport (same width, same breakpoints) to reveal the rest.
  const extra = await page.evaluate(() => {
    let most = 0;
    for (const el of document.querySelectorAll<HTMLElement>(".vibe-workspace *")) {
      if (el.closest(".vibe-sidebar")) continue;
      const style = getComputedStyle(el);
      if (!/(auto|scroll)/.test(style.overflowY) || el.clientHeight < 200) continue;
      most = Math.max(most, el.scrollHeight - el.clientHeight);
    }
    return most;
  });
  if (extra < 40) return;
  await page.setViewportSize({ width: combo.width, height: Math.min(combo.height + extra + 40, 7000) });
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(outDir, name(state, combo, "-tall")), fullPage: true });
  await page.setViewportSize({ width: combo.width, height: combo.height });
}

async function openSession(page: Page, id: string, view?: string) {
  await page.goto(`/vibe-evals?session=${id}${view ? `&view=${view}` : ""}`);
}

// Signed-in entry: the local AuthKit fixture signs in, and /v1/users/me lists two workspaces.
async function signIn(page: Page) {
  await page.route("**/v1/users/me", route => {
    const headers = { "Access-Control-Allow-Origin": new URL(page.url() || "http://127.0.0.1:53517").origin, "Access-Control-Allow-Credentials": "true" };
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, "Access-Control-Allow-Headers": "Content-Type,Authorization", "Access-Control-Allow-Methods": "GET,OPTIONS" } });
    return route.fulfill({ headers, json: { user_id: "a8000000-0000-4000-8000-000000000001", email: "browser@example.invalid", organizations: [{ id: "org", name: "Returns Co", slug: "returns-co", role: "org_admin", workspaces: [
      { id: "a8000000-0000-4000-8000-000000000003", name: "Returns team", slug: "returns-team", role: "workspace_admin" },
      { id: "a8000000-0000-4000-8000-000000000006", name: "Support pilots", slug: "support-pilots", role: "workspace_member" },
    ] }] } });
  });
  await page.goto("/auth/login?returnTo=%2Fvibe-evals");
  await page.getByRole("button", { name: "Continue with AgentClash" }).click();
}

const combos: Combo[] = viewports.flatMap(v => schemes.map(scheme => ({ ...v, scheme })));

for (const combo of combos) {
  const label = `${combo.width}-${combo.scheme}`;
  test.describe(`screens ${label}`, () => {
    test.describe.configure({ timeout: 90_000 });

    test(`a entry ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await page.goto("/vibe-evals");
      await expect(page.getByRole("heading", { name: "What would you like to evaluate?" })).toBeVisible({ timeout: 20_000 });
      await expect(page.getByRole("button", { name: /Build an agent/ })).toBeEnabled({ timeout: 20_000 });
      await shoot(page, "a-entry", combo);
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`b build starter ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "build-start");
      await expect(page.getByRole("heading", { name: "What would you like help with?" })).toBeVisible({ timeout: 20_000 });
      await shoot(page, "b-build-starter", combo);
      const composer = page.getByRole("textbox", { name: "Message Vibe Evals" });
      await composer.fill("My agent answers shop return questions. Unopened items can be returned within 30 days. It should ask for missing details.");
      await expect(page.getByText(/Build and try 3 examples/).first()).toBeVisible();
      await page.waitForTimeout(900);
      await shoot(page, "b-build-starter-typed", combo);
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`c improve starter ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "improve-start");
      await expect(page.getByRole("heading", { name: "Improve an agent you already have." })).toBeVisible({ timeout: 20_000 });
      await shoot(page, "c-improve-starter", combo);
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`d build chat ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "build-question");
      await expect(page.getByText("How long can items be returned: 14 days or 30 days?").first()).toBeVisible({ timeout: 20_000 });
      await shoot(page, "d-build-chat-question", combo, { tall: true });
      // The header badge says "Review needed"; open Guide to see where the choices live.
      await page.getByRole("button", { name: "Guide", exact: true }).click();
      await page.waitForTimeout(300);
      await shoot(page, "d-build-chat-question-guide", combo);
      await openSession(page, "build-choices");
      await expect(page.getByText("How long can items be returned: 14 days or 30 days?").first()).toBeVisible({ timeout: 20_000 });
      const options = page.getByRole("region", { name: "Conversation choices" });
      if (await options.count()) await options.first().scrollIntoViewIfNeeded();
      await shoot(page, "d-build-chat-choices", combo);
      await openSession(page, "build-progress");
      await expect(page.locator('[data-message-id="p-3"]')).toBeVisible({ timeout: 20_000 });
      await shoot(page, "d-build-chat-progress", combo);
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`e agent tab ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "returns-results");
      await expect(page.getByRole("tablist", { name: "Workspace views" })).toBeVisible({ timeout: 20_000 });
      await expect(page.getByRole("tab", { name: "Agent", exact: true })).toHaveAttribute("aria-selected", "true");
      await expect(page.getByRole("button", { name: "Try it yourself", exact: true })).toBeVisible();
      await shoot(page, "e-agent-tab", combo, { tall: true });
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`f tests tab and g cost dialog ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "returns-ready");
      await expect(page.getByRole("tablist", { name: "Workspace views" })).toBeVisible({ timeout: 20_000 });
      await page.getByRole("tab", { name: "Tests", exact: true }).click();
      await expect(page.getByRole("heading", { name: "3 tests are ready" })).toBeVisible();
      await shoot(page, "f-tests-ready", combo, { tall: true });
      const tests = page.getByRole("region", { name: "Your tests", exact: true });
      const firstCase = tests.locator("summary").filter({ hasText: inputs[1] }).first();
      await expect(firstCase).toBeVisible();
      if (!(await firstCase.evaluate(node => (node.parentElement as HTMLDetailsElement).open))) await firstCase.click();
      await shoot(page, "f-tests-case-open", combo);
      await tests.getByRole("button", { name: "Run 3 tests" }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog.getByRole("button", { name: "Run 3 examples" })).toBeVisible();
      await shoot(page, "g-cost-dialog", combo);
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`h results ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "returns-results");
      await expect(page.getByRole("tablist", { name: "Workspace views" })).toBeVisible({ timeout: 20_000 });
      await page.getByRole("tab", { name: "Results", exact: true }).click();
      await expect(page.getByRole("article", { name: "Evaluation scorecard" })).toBeVisible();
      await shoot(page, "h-results", combo, { tall: true });
      // Expand the failed case row and its evidence.
      const failed = page.locator(".vibe-result-row").filter({ hasText: inputs[1] }).first();
      if (await failed.count()) {
        if (!(await failed.evaluate(el => (el as HTMLDetailsElement).open))) await failed.locator("summary").first().click();
        const evidence = failed.getByRole("button", { name: "See evidence" });
        if (await evidence.count()) await evidence.first().click();
        await failed.scrollIntoViewIfNeeded();
      }
      await page.waitForTimeout(600);
      await shoot(page, "h-results-failed-case", combo, { tall: true });
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`i guide panel ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "returns-results");
      await expect(page.getByRole("tablist", { name: "Workspace views" })).toBeVisible({ timeout: 20_000 });
      await page.getByRole("button", { name: "Guide", exact: true }).click();
      await expect(page.getByRole("textbox", { name: "Message Vibe Evals" })).toBeVisible();
      await shoot(page, "i-guide-open", combo);
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`j settings dialog ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "returns-results");
      await expect(page.getByRole("tablist", { name: "Workspace views" })).toBeVisible({ timeout: 20_000 });
      await page.getByRole("button", { name: "Details", exact: true }).click();
      const details = combo.width >= 1280 ? page.getByRole("complementary", { name: "Details utility" }) : page.getByRole("dialog", { name: "Details", exact: true });
      await expect(details).toBeVisible();
      await shoot(page, "j-details-panel", combo);
      await details.getByRole("button", { name: "Settings", exact: true }).click();
      await expect(page.getByRole("dialog", { name: "Settings and details" })).toBeVisible();
      await shoot(page, "j-settings-dialog", combo, { tall: true });
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`k sidebar ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await openSession(page, "returns-results");
      await expect(page.getByRole("tablist", { name: "Workspace views" })).toBeVisible({ timeout: 20_000 });
      if (combo.width < 1024) {
        await page.getByRole("button", { name: "Open agents", exact: true }).click();
        await expect(page.getByRole("dialog", { name: "Your agents" })).toBeVisible();
      } else {
        await expect(page.getByRole("complementary", { name: "Agent sidebar", exact: true })).toBeVisible();
      }
      const nav = combo.width < 1024 ? page.getByRole("dialog", { name: "Your agents" }) : page.getByRole("complementary", { name: "Agent sidebar", exact: true });
      await expect(nav.locator("[data-evaluation-id]").first()).toBeVisible();
      await shoot(page, "k-sidebar", combo);
      expect(control.unknown).toEqual([]); expect(control.errors).toEqual([]);
    });

    test(`m signed-in entry with workspace picker ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      await signIn(page);
      const picker = page.getByRole("combobox", { name: "Workspace for new projects", exact: true });
      await expect(picker).toBeVisible({ timeout: 20_000 });
      await expect(page.getByText("Choose a workspace to use its credits.", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Review settings" })).toHaveCount(0);
      await shoot(page, "m-entry-workspace", combo);
      await picker.selectOption({ label: "Returns team" });
      await expect(page.getByRole("button", { name: /Build an agent/ })).toBeEnabled();
      await shoot(page, "m-entry-workspace-chosen", combo);
      expect(control.errors).toEqual([]);
    });

    test(`l delete and credits dialogs ${label}`, async ({ page }) => {
      await prepare(page, combo);
      const control = await serve(page);
      // A workspace in the URL shows the Credits control in Settings.
      await page.goto("/vibe-evals?session=returns-results&workspace=a8000000-0000-4000-8000-000000000003");
      await expect(page.getByRole("tablist", { name: "Workspace views" })).toBeVisible({ timeout: 20_000 });
      if (combo.width < 1024) await page.getByRole("button", { name: "Open agents", exact: true }).click();
      const nav = combo.width < 1024 ? page.getByRole("dialog", { name: "Your agents" }) : page.getByRole("complementary", { name: "Agent sidebar", exact: true });
      const actions = nav.locator(".vibe-project-actions");
      if (!(await actions.evaluate(el => (el as HTMLDetailsElement).open))) await actions.locator("summary").click();
      await actions.getByRole("button", { name: "Delete this project", exact: true }).click();
      const deletion = page.getByRole("dialog", { name: /^Delete / });
      await expect(deletion).toBeVisible();
      await shoot(page, "l-delete-dialog", combo);
      await deletion.getByRole("button", { name: "Cancel", exact: true }).click();
      if (combo.width < 1024 && await nav.isVisible()) await page.keyboard.press("Escape");
      await page.getByRole("button", { name: "Details", exact: true }).click();
      const details = combo.width >= 1280 ? page.getByRole("complementary", { name: "Details utility" }) : page.getByRole("dialog", { name: "Details", exact: true });
      await details.getByRole("button", { name: "Settings", exact: true }).click();
      await page.getByRole("dialog", { name: "Settings and details" }).getByRole("button", { name: "Credits", exact: true }).click();
      await expect(page.getByRole("dialog", { name: "Workspace AI credits" })).toBeVisible();
      await shoot(page, "l-credits-dialog", combo);
      expect(control.errors).toEqual([]);
    });
  });
}
