import type { SessionManager, TerminalSession } from "./sessions.ts";
import type { Config } from "./config.ts";
import type { BudgetHold, Limits } from "./limits.ts";
import { readBody, json, BodyError } from "./http.ts";

/** Rough $/Mtok by model family — used only to bound spend, not to bill. */
function priceUsd(model: string, inTok: number, outTok: number): number {
  const m = model.toLowerCase();
  let inP = 3,
    outP = 15; // conservative default
  if (m.includes("haiku")) [inP, outP] = [1, 5];
  else if (m.includes("sonnet")) [inP, outP] = [3, 15];
  else if (m.includes("opus")) [inP, outP] = [15, 75];
  else if (m.includes("gpt-4o-mini") || m.includes("mini")) [inP, outP] = [0.15, 0.6];
  else if (m.includes("gpt-4o") || m.includes("gpt-4.1")) [inP, outP] = [2.5, 10];
  else if (m.includes("gpt-5") || m.includes("o3") || m.includes("o1")) [inP, outP] = [5, 20];
  else if (m.includes("grok")) [inP, outP] = [2, 10];
  return (inTok * inP + outTok * outP) / 1_000_000;
}

/** Parse usage from a provider response (SSE stream or JSON) — best effort. */
function parseUsage(text: string): { model: string; inTok: number; outTok: number } | null {
  // Anthropic streaming: message_start has input usage, message_delta has output.
  let model = "";
  let inTok = 0;
  let outTok = 0;
  let found = false;
  // Try JSON (non-streaming) first.
  try {
    const j = JSON.parse(text);
    const u = j.usage ?? j.response?.usage;
    if (u) {
      model = j.model ?? j.response?.model ?? "";
      inTok = u.input_tokens ?? u.prompt_tokens ?? 0;
      outTok = u.output_tokens ?? u.completion_tokens ?? 0;
      return { model, inTok, outTok };
    }
  } catch {
    /* not JSON — scan SSE */
  }
  for (const line of text.split("\n")) {
    const t = line.trim();
    if (!t.startsWith("data:")) continue;
    const payload = t.slice(5).trim();
    if (payload === "[DONE]") continue;
    try {
      const ev = JSON.parse(payload);
      const u = ev.usage ?? ev.message?.usage ?? ev.response?.usage;
      if (ev.model) model = ev.model;
      if (ev.message?.model) model = ev.message.model;
      if (ev.response?.model) model = ev.response.model;
      if (u) {
        if (typeof u.input_tokens === "number") inTok = Math.max(inTok, u.input_tokens);
        if (typeof u.output_tokens === "number") outTok = Math.max(outTok, u.output_tokens);
        if (typeof u.prompt_tokens === "number") inTok = Math.max(inTok, u.prompt_tokens);
        if (typeof u.completion_tokens === "number") outTok = Math.max(outTok, u.completion_tokens);
        found = true;
      }
    } catch {
      /* ignore partial */
    }
  }
  return found ? { model, inTok, outTok } : null;
}

export interface GatewayOptions {
  config: Config;
  sessions: SessionManager;
  limits: Limits;
  // Dependency injection for isolated tests; the production entrypoint uses fetch.
  fetch?: (url: string, init: RequestInit) => Promise<Response>;
}
const BASES: Record<string, string> = {
  openai: "https://api.openai.com", anthropic: "https://api.anthropic.com",
  xai: "https://api.x.ai", openrouter: "https://openrouter.ai",
};
const PATHS: Record<string, RegExp> = {
  openai: /^\/v1\/(responses|chat\/completions)$/,
  anthropic: /^\/v1\/messages$/,
  xai: /^\/v1\/(chat\/completions|responses)$/,
  openrouter: /^\/api\/v1\/chat\/completions$/,
};

interface ActiveRequest {
  abort: AbortController;
  done: Promise<void>;
  finish(): void;
}

export class Gateway {
  private active = new Set<ActiveRequest>();
  private closing = false;
  private meteringFailed = false;
  constructor(private deps: GatewayOptions) {}
  get healthy() { return !this.closing && !this.meteringFailed; }
  get pending() { return this.active.size; }

  async handle(req: Request): Promise<Response> {
    if (!this.healthy) return json({ error: "gateway_unavailable" }, 503);
    const url = new URL(req.url);
    const match = url.pathname.match(/^\/gw\/(openai|anthropic|xai|openrouter)(\/.*)$/);
    if (!match || req.method !== "POST" || !PATHS[match[1]!]!.test(match[2]!) || url.search) {
      return json({ error: "unsupported_gateway_route" }, 404);
    }
    const provider = match[1]! as keyof Config["providerKeys"];
    const key = this.deps.config.providerKeys[provider];
    if (!key) return json({ error: "provider_unavailable" }, 503);
    const token = req.headers.get("x-api-key") ?? req.headers.get("authorization")?.replace(/^Bearer\s+/i, "") ?? "";
    const session = this.deps.sessions.validateGatewayToken(token);
    if (!session) return json({ error: "invalid_or_expired_trial" }, 401);
    const config = this.deps.config;
    const reserved = config.requestReserve;
    // No await between checking and reserving the per-session budget.
    if (session.budget.spent + session.budget.reserved + reserved > session.budget.limit + 1e-10) {
      return json({ error: "trial_budget_reached" }, 402);
    }
    session.budget.reserved += reserved;
    let finish!: () => void;
    const active: ActiveRequest = { abort: new AbortController(), done: new Promise<void>(r => { finish = r; }), finish: () => finish() };
    this.active.add(active);
    const signal = AbortSignal.any([req.signal, active.abort.signal, AbortSignal.timeout(Math.max(1, Math.min(config.gatewayTimeoutMs, session.expiresAt - Date.now())))]);
    let hold: BudgetHold | null = null;
    let completed = false;
    const complete = async (cost: number) => {
      if (completed) return;
      completed = true;
      try {
        if (hold) {
          session.budget.spent += cost;
          await hold.settle(cost);
        }
      } catch {
        // The persisted hold remains charged. Stop further spending until an
        // operator can reconcile it; never silently drop failed metering.
        this.meteringFailed = true;
        console.error("[try-cli] gateway settlement failed");
      } finally {
        session.budget.reserved = Math.max(0, session.budget.reserved - reserved);
        this.active.delete(active); active.finish();
      }
    };
    try {
      const body = JSON.parse(await readBody(req, config.maxRequestBytes, signal));
      if (!body || typeof body !== "object" || Array.isArray(body)) throw new BodyError(400);
      const field = match[2]!.endsWith("/responses") ? "max_output_tokens" :
        "max_completion_tokens" in body ? "max_completion_tokens" : "max_tokens";
      body[field] = typeof body[field] === "number" && Number.isInteger(body[field]) && body[field] > 0
        ? Math.min(body[field], config.maxOutputTokens) : config.maxOutputTokens;
      // Do not leave a second output field able to override the clamped one.
      for (const other of ["max_tokens", "max_completion_tokens", "max_output_tokens"]) if (other !== field) delete body[other];
      if (match[2]!.endsWith("/responses")) { body.background = false; body.store = false; }
      if (body.stream && match[2]!.endsWith("/chat/completions")) body.stream_options = { include_usage: true };
      hold = await this.deps.limits.reserve(reserved, config.dailyCeiling);
      if (!hold) { await complete(0); return json({ error: "daily_trial_capacity_reached" }, 429); }
      signal.throwIfAborted();
      const headers = new Headers({ "content-type": "application/json", "accept-encoding": "identity" });
      if (provider === "anthropic") {
        headers.set("x-api-key", key);
        headers.set("anthropic-version", req.headers.get("anthropic-version") ?? "2023-06-01");
      } else headers.set("authorization", `Bearer ${key}`);
      const upstream = await (this.deps.fetch ?? fetch)(`${BASES[provider]}${match[2]}`, {
        method: "POST", headers, body: JSON.stringify(body), signal, redirect: "error",
      });
      if (!upstream.body) { await complete(reserved); return new Response(null, { status: upstream.status }); }
      const reader = upstream.body.getReader();
      const decoder = new TextDecoder();
      let captured = "";
      let capturedBytes = 0;
      let truncated = false;
      const abortStream = () => { void reader.cancel().catch(() => {}).then(() => complete(reserved)); };
      signal.addEventListener("abort", abortStream, { once: true });
      if (signal.aborted) abortStream();
      const stream = new ReadableStream<Uint8Array>({
        async pull(controller) {
          try {
            signal.throwIfAborted();
            const { done, value } = await reader.read();
            signal.throwIfAborted();
            if (done) {
              signal.removeEventListener("abort", abortStream);
              captured += decoder.decode();
              const usage = truncated ? null : parseUsage(captured);
              const price = usage ? priceUsd(usage.model, usage.inTok, usage.outTok) : reserved;
              await complete(Number.isFinite(price) && price > 0 ? price : reserved);
              controller.close();
            } else {
              capturedBytes += value.byteLength;
              if (capturedBytes <= 8 * 1024 * 1024) captured += decoder.decode(value, { stream: true });
              else truncated = true;
              controller.enqueue(value);
            }
          } catch {
            signal.removeEventListener("abort", abortStream);
            await reader.cancel().catch(() => {});
            await complete(reserved);
            controller.error(new Error("Gateway stream interrupted"));
          }
        },
        async cancel() {
          signal.removeEventListener("abort", abortStream);
          active.abort.abort();
          await reader.cancel().catch(() => {});
          await complete(reserved);
        },
      });
      return new Response(stream, { status: upstream.status, headers: {
        "content-type": upstream.headers.get("content-type") ?? "application/json",
        "cache-control": "no-store", "x-accel-buffering": "no",
      } });
    } catch (err) {
      await complete(reserved);
      if (err instanceof BodyError) return json({ error: err.message }, err.status);
      if (err instanceof SyntaxError) return json({ error: "invalid_json" }, 400);
      return json({ error: "gateway_dependency_unavailable" }, 503);
    }
  }
  async close() {
    this.closing = true;
    const pending = [...this.active];
    for (const active of pending) active.abort.abort();
    await Promise.all(pending.map(a => a.done));
    if (this.meteringFailed) throw new Error("Gateway settlement incomplete");
  }
}
