// A stateful fake of /v1/vibe/** for the entry screen and both doors. It walks each
// door to its first test result with no backend and no model. Every non-loopback
// request is aborted and recorded. State changes that a worker would make are
// queued per project and applied one per /events request (the client reconnects).
/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Page } from "@playwright/test";
import { defaultModels, type Artifact, type CaseResult, type Operation, type Session } from "../../src/lib/vibe";

export const models = defaultModels;
export const fakeConfig = {
  enabled: true,
  availability: { guest: { available: true }, workspace: { available: true } },
  two_door: true,
  // Production has material Build on (backend/internal/vibe/models.go).
  material_build: true,
  interaction_actions: true,
  defaults: models,
  models: [...new Set(Object.values(models))].map(id => ({ id, name: id, input_nano_per_token: 0, output_nano_per_token: 0 })),
};
export const workspaces = [
  { id: "a8000000-0000-4000-8000-000000000003", name: "Returns team", slug: "returns-team", role: "workspace_admin" },
  { id: "a8000000-0000-4000-8000-000000000006", name: "Support pilots", slug: "support-pilots", role: "workspace_member" },
];

export const description = "Answer customer questions about returns for my online shop. Unopened items within 30 days are eligible. Ask for missing details.";
export const instructions = "Help shoppers with returns. Allow unopened items within 30 days of purchase. Decline later returns politely. Ask for missing details before deciding.";
export const transcript = "Customer: I bought headphones 45 days ago. They are still sealed. Can I return them?\nAgent: Sure, bring them back with the receipt.";
export const expectation = "Decline purchases older than 30 days.";
export const packFile = { name: "returns-pack.yaml", mimeType: "application/yaml", buffer: Buffer.from("name: returns\ncases: []\n") };

const inputs = ["Unopened, bought 10 days ago. Can I return it?", "Unopened, bought 45 days ago. Can I return it?", "I want to return something."];
const expected = ["Explain that it is eligible.", "Decline the late return.", "Ask for purchase age and item condition."];
const outputs = ["Yes. Unopened items bought within 30 days are eligible for a return.", "Yes, you can return it. Just bring the receipt to any store.", "Happy to help. How long ago did you buy it, and is it still unopened?"];
// The first agent answer (M1): any model-written test reply on screen. Recorded replies use other text.
export const agentReply = (page: Page) => page.getByText(new RegExp(outputs.map(text => text.replace(/[.?]/g, "\\$&")).join("|"))).filter({ visible: true }).first();
const blueprint = {
  cases: inputs.map((question, i) => ({ key: `case-${i + 1}`, payload: { question }, expectations: [{ key: "expected_behavior", kind: "text", value: expected[i] }] })),
  judges: [{ key: "behavior", context_from: ["case.expectations.expected_behavior"], assertion: "The response meets this case's expected_behavior." }],
  validators: [{ key: "has_answer" }],
  dimensions: [{}, {}],
};
const caseResults = (version: string): CaseResult[] => inputs.map((input, i) => ({
  case_key: `case-${i + 1}`, title: input, version, input: { question: input }, output: outputs[i], expected: expected[i], verdict: i === 1 ? "FAIL" : "PASS", expected_checks: 1,
  messages: [{ id: `u-${version}-${i}`, role: "user", content: input }, { id: `a-${version}-${i}`, role: "assistant", content: outputs[i] }],
  expectations: [{ id: "behavior", statement: expected[i] }],
  checks: [{ key: "behavior", verdict: i === 1 ? "FAIL" : "PASS", evidence: i === 1 ? "The agent allowed a return 45 days after purchase." : "The reply follows the policy." }],
}));
const seeded = (version: string) => caseResults(version).map((c, i) => i === 0 ? c : { ...c, output: "", verdict: "UNKNOWN" as const, checks: [], error: { code: "not_evaluated", message: "This case has not been evaluated yet." } });
const scorecard = { passed: 2, failed: 1, unknown: 0, total: 3, evaluated: 3, pass_rate: 2 / 3, coverage: 1 };
const running = (id: string, kind: string, extra: Partial<Operation> = {}): Operation => ({ id, kind, state: "RUNNING", billing: "RESERVED", models, max_cost_nano_usd: 120_000_000, actual_cost_nano_usd: 0, results: [], ...extra });
const finish = (op: Operation, extra: Partial<Operation> = {}) => Object.assign(op, { state: "COMPLETED", billing: "SETTLED", progress: undefined, ...extra });

export function project(id: string, door: "build" | "test", workspace?: string): Session {
  return { id, revision: 1, event_cursor: 1, anonymous: !workspace, ...(workspace ? { workspace_id: workspace } : {}), operations: [], updated_at: new Date().toISOString(),
    document: { format_version: 1, models, messages: [], requirements: [], artifacts: [], test_journey: true, evaluation: { id, chat_id: "chat", door } } };
}
const bump = (s: Session) => { s.revision++; s.event_cursor = s.revision; s.updated_at = new Date().toISOString(); };

export type Failure = { status: number; code: string; message: string; admission?: "rejected" };
// Keys: "POST /sessions", "POST /messages:message", "POST /messages:check", "POST /import", "GET /users/me", "GET /config".
// buildFails: the first Build turn ends with a retryable provider error (a retry then succeeds).
export type FakeOptions = { config?: Record<string, unknown>; signedIn?: boolean; me?: unknown; failures?: Record<string, Failure | undefined>; buildFails?: boolean };

export async function fakeVibe(page: Page, options: FakeOptions = {}) {
  // Hide the Next.js dev-tools badge; it is a dev-server artifact, not product UI.
  await page.addInitScript(() => {
    const hide = () => { const style = document.createElement("style"); style.textContent = "nextjs-portal{display:none!important}"; document.head.appendChild(style); };
    if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", hide); else hide();
  });
  const sessions = new Map<string, Session>();
  const queues = new Map<string, (() => void)[]>();
  const posts: { method: string; path: string; body: any }[] = [];
  const unknown: string[] = [], errors: string[] = [], external: string[] = [];
  const failures = options.failures ?? {};
  const config = { ...fakeConfig, ...options.config };
  const later = (s: Session, ...steps: (() => void)[]) => queues.set(s.id, [...(queues.get(s.id) ?? []), ...steps.map(step => () => { step(); bump(s); })]);
  const retries = new Map<string, () => void>();
  // The first Build turn makes a version, then runs its three tests.
  const buildRun = (s: Session, op: Operation, art: Artifact) => {
    if (!s.operations.includes(op)) s.operations.push(op);
    const run = running("run-1", "check", { source: { kind: "prompt", label: art.title, artifact_id: art.id }, results: seeded(art.id), progress: { phase: "grading", completed_cases: 1, total_cases: 3, phase_elapsed_ms: 4_000 } } as any);
    later(s, () => {
      finish(op); s.document.artifacts.push(art); s.document.active_artifact_id = art.id;
      s.document.messages.push({ id: art.proposal_message_id!, role: "assistant", content: "I made a first version of your returns assistant and three example situations. I’m running them now.", operation_id: op.id, artifact_id: art.id });
      s.operations.push(run); s.document.build = { ...s.document.build!, phase: "checking", artifact_id: art.id, check_id: run.id };
    }, () => { finish(run, { actual_cost_nano_usd: 41_000_000, results: caseResults(art.id), scorecard }); s.document.build = { ...s.document.build!, phase: "results" }; });
  };
  page.on("pageerror", e => errors.push(e.message));
  page.on("request", r => {
    const u = new URL(r.url());
    if (u.protocol.startsWith("http") && !["127.0.0.1", "localhost"].includes(u.hostname)) external.push(`${r.method()} ${r.url()}`);
  });
  await page.route(u => u.protocol.startsWith("http") && !["127.0.0.1", "localhost"].includes(u.hostname), route => route.abort());
  const cors = () => ({ "Access-Control-Allow-Origin": new URL(page.url()).origin, "Access-Control-Allow-Credentials": "true" });
  const preflight = { "Access-Control-Allow-Headers": "Content-Type,Authorization,If-Match,X-File-Name", "Access-Control-Allow-Methods": "GET,POST,PATCH,DELETE,OPTIONS" };
  const fail = (f: Failure) => ({ status: f.status, json: { error: { code: f.code, message: f.message, ...(f.admission ? { admission: f.admission } : {}) } } });
  if (options.signedIn) await page.route("**/v1/users/me", route => {
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers: { ...cors(), ...preflight } });
    const f = failures["GET /users/me"];
    if (f) return route.fulfill({ headers: cors(), ...fail(f) });
    return route.fulfill({ headers: cors(), json: options.me ?? { user_id: "a8000000-0000-4000-8000-000000000001", email: "browser@example.invalid", organizations: [{ id: "org", name: "Returns Co", slug: "returns-co", role: "org_admin", workspaces }] } });
  });
  await page.route("**/v1/vibe/**", async route => {
    const request = route.request(), url = new URL(request.url()), p = url.pathname.replace(/^\/v1\/vibe/, ""), method = request.method();
    const headers = cors();
    const json = (body: unknown, status = 200) => route.fulfill({ status, headers, json: body });
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers: { ...headers, ...preflight } });
    if (p === "/config") { const f = failures["GET /config"]; return f ? route.fulfill({ headers, ...fail(f) }) : json(config); }
    if (p.endsWith("/saved-checks") || p.endsWith("/grade-reviews") || p.endsWith("/inputs") || p.endsWith("/canonical-results") || p.endsWith("/reports")) return json([]);
    if (p.includes("/credits")) return json({ under_review: false, balance_nano_usd: 0, held_nano_usd: 0, available_nano_usd: 0, products: [] });
    const id = p.split("/sessions/")[1]?.split("/")[0];
    const state = id ? sessions.get(id) : undefined;
    if (p.endsWith("/case")) {
      const key = url.searchParams.get("key");
      const hit = [...sessions.values()].flatMap(item => item.operations).flatMap(op => p.includes(`/operations/${op.id}/`) ? op.results : []).find(r => r.case_key === key);
      return hit ? json(hit) : json({ error: { code: "not_found", message: "missing case" } }, 404);
    }
    if (method === "GET") {
      if (p === "/sessions") return json([...sessions.values()]);
      if (state && p.endsWith("/events")) {
        queues.get(state.id)?.shift()?.();
        return route.fulfill({ headers, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(state)}\n\n` });
      }
      if (state && p === `/sessions/${id}`) return json(state);
      unknown.push(`GET ${p}`);
      return json({ error: { code: "not_found", message: "Project is unavailable." } }, 404);
    }
    let body: any = {};
    try { body = request.postDataJSON() ?? {}; } catch { body = { raw: request.postData() }; }
    posts.push({ method, path: p, body });
    const name = p === "/sessions" ? "/sessions" : p.endsWith("/messages") ? `/messages:${body.kind}` : `/${p.split("/").at(-1)}`;
    const f = failures[`${method} ${name}`] ?? failures[`${method} ${name.split(":")[0]}`];
    if (f) return route.fulfill({ headers, ...fail(f) });
    if (method === "POST" && p === "/sessions") {
      const created = project(body.id, body.door, body.workspace_id);
      sessions.set(created.id, created);
      return json(created, 201);
    }
    if (!state) { unknown.push(`${method} ${p}`); return json({ error: { code: "not_found", message: "Project is unavailable." } }, 404); }
    const s = state;
    if (p.endsWith("/build-quote")) return json({ id: `build-quote-${posts.length}`, max_calls: 9, cases: 3, max_cost_nano_usd: 250_000_000, request: { content: body.content, models: body.models, ...(body.inputs ? { inputs: body.inputs } : {}), ...(body.adopt_rules ? { adopt_rules: body.adopt_rules } : {}) }, expires_at: new Date(Date.now() + 600_000).toISOString() });
    if (p.endsWith("/run-quote")) return json({ id: `run-quote-${posts.length}`, max_cost_nano_usd: 120_000_000, cases: 3, calls: 6, expires_at: new Date(Date.now() + 600_000).toISOString() });
    if (p.endsWith("/messages") && body.kind === "message") {
      const op = running(body.client_id, "message", { max_cost_nano_usd: 1, conversation_decision: { intent: "prepare_tests", source_message_id: `m-${body.client_id}` }, progress: { phase: "preparing", completed_cases: 0, total_cases: 3, phase_elapsed_ms: 2_000 } } as any);
      s.operations.push(op);
      s.document.messages.push({ id: `m-${body.client_id}`, role: "user", content: body.content, operation_id: op.id });
      const art: Artifact = { id: `v${s.document.artifacts.length + 1}`, kind: "test_suite", title: "Shop returns assistant", summary: "Eligible returns, late purchases and missing details.", agent_prompt: body.instructions || instructions, accepted: true, source_message_id: `m-${op.id}`, proposal_message_id: `prepared-${op.id}`, scope_note: "Answers return questions using the policy you supplied.", blueprint };
      if (s.document.evaluation?.door === "build") {
        s.title = "Online shop returns helper";
        s.document.build = { cycle_id: body.cycle_id || "cycle", phase: "preparing", clarifications_used: 0 };
        if (options.buildFails) {
          later(s, () => finish(op, { state: "FAILED", billing: "RELEASED", retryable: true, error: { code: "provider_error", message: "The selected model’s provider failed. Your request is saved." } } as any));
          retries.set(op.id, () => { finish(op, { retryable: false } as any); buildRun(s, running(`retry-${op.id}`, "message", { retry_of_operation_id: op.id, max_cost_nano_usd: 1 } as any), art); });
          bump(s);
          return json(op, 202);
        }
        buildRun(s, op, art);
      } else {
        s.title = "Support agent checks";
        later(s, () => {
          finish(op); s.document.artifacts.push(art); s.document.active_artifact_id = art.id;
          s.document.messages.push({ id: art.proposal_message_id!, role: "assistant", content: "These are example messages I’ll send to your agent. Open a test to see what a good reply should do.", operation_id: op.id, artifact_id: art.id });
        });
      }
      bump(s);
      return json(op, 202);
    }
    if (p.endsWith("/messages") && (body.kind === "check" || body.kind === "retest")) {
      const artifactID = body.artifact_id || s.document.active_artifact_id || s.document.artifacts.at(-1)!.id;
      const art = s.document.artifacts.find(a => a.id === artifactID);
      if (art && body.approve_artifact) art.accepted = true;
      const recorded = !!art?.conversation_evaluation;
      const op = running(body.client_id, body.kind, { baseline_id: body.baseline_id, source: recorded ? { kind: "provided_conversations", label: "App replies", artifact_id: artifactID, evidence_set_id: art!.conversation_evaluation!.evidence_set_id } : { kind: "prompt", label: art?.title || "Agent", artifact_id: artifactID }, results: recorded ? [] : seeded(artifactID), progress: { phase: "grading", completed_cases: recorded ? 0 : 1, total_cases: recorded ? 1 : 3, phase_elapsed_ms: 3_000 } } as any);
      s.operations.push(op);
      later(s, () => finish(op, recorded ? {
        actual_cost_nano_usd: 12_000_000, scorecard: { passed: 0, failed: 1, unknown: 0, total: 1, evaluated: 1, pass_rate: 0, coverage: 1 },
        results: [{ case_key: "conversation-1", title: "Conversation 1", version: artifactID, input: null, output: "", verdict: "FAIL", messages: s.document.evidence_sets![0].conversations[0].messages, expectations: [{ id: "expectation-1", statement: expectation }],
          checks: [{ key: "expectation-1", verdict: "FAIL", evidence: "The agent accepted a sealed item bought 45 days ago.", message_ids: ["c1-a"] }] }],
      } : { actual_cost_nano_usd: 41_000_000, results: caseResults(artifactID), scorecard }));
      bump(s);
      return json(op, 202);
    }
    if (p.endsWith("/retry") && retries.has(p.split("/operations/")[1]?.split("/")[0])) {
      const opID = p.split("/operations/")[1].split("/")[0];
      retries.get(opID)!(); retries.delete(opID);
      bump(s);
      return json(s.operations.at(-1), 202);
    }
    if (p.endsWith("/evidence")) {
      const conversations = [{ key: "conversation-1", title: "Conversation 1", messages: [{ id: "c1-u", role: "user" as const, content: "I bought headphones 45 days ago. They are still sealed. Can I return them?" }, { id: "c1-a", role: "assistant" as const, content: "Sure, bring them back with the receipt." }] }];
      s.document.evidence_sets = [{ id: "ev-1", label: body.label || "App replies", raw: body.content, conversations }];
      s.document.active_evidence_id = "ev-1";
      const art: Artifact = { id: "rec-v1", kind: "conversation_evaluation", title: "App replies", agent_prompt: "", accepted: true, source_message_id: body.client_id, blueprint: {}, conversation_evaluation: { evidence_set_id: "ev-1", expectations: body.expectations } } as any;
      s.document.artifacts.push(art); s.document.active_artifact_id = art.id;
      s.evidence_receipt = { client_id: body.client_id, evidence_id: "ev-1", artifact_id: art.id };
      s.title = "App replies";
      bump(s);
      return json(s);
    }
    if (p.endsWith("/import")) {
      const art: Artifact = { id: "pack-v1", kind: "test_suite", title: packFile.name, summary: "Imported test pack.", agent_prompt: "", accepted: false, source_message_id: "import", blueprint,
        original: { id: "orig-1", artifact_id: "pack-v1", filename: packFile.name, media_type: packFile.mimeType, content_sha256: "0".repeat(64), size_bytes: packFile.buffer.length } } as any;
      s.document.artifacts.push(art); s.document.active_artifact_id = art.id; s.title = packFile.name;
      bump(s);
      return json(s);
    }
    if (method === "PATCH" && p === `/sessions/${s.id}` && typeof body.agent_prompt === "string") {
      const parent = s.document.artifacts.find(a => a.id === body.artifact_id) ?? s.document.artifacts.at(-1)!;
      const next: Artifact = { ...parent, id: `${parent.id}-agent`, parent_id: parent.id, agent_prompt: body.agent_prompt, proposal_message_id: undefined };
      s.document.artifacts.push(next); s.document.active_artifact_id = next.id;
      bump(s);
      return json(s);
    }
    unknown.push(`${method} ${p}`);
    return json({ error: { code: "invalid_state", message: "The entry fake has no handler for this request." } }, 400);
  });
  return { sessions, posts, unknown, errors, external, config, current: () => [...sessions.values()].at(-1)! };
}
export type Fake = Awaited<ReturnType<typeof fakeVibe>>;
