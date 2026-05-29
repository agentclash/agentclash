/**
 * Metered LLM gateway for the anonymous free trial.
 *
 * The real provider keys live ONLY here (server-side). Each anonymous session
 * is minted a short-lived, spend-capped proxy token that the sandbox CLIs send
 * as their API key while pointing their base URL at `/gw/<provider>`. The
 * gateway validates the token against a live, in-budget session, swaps in the
 * real key, streams the provider response straight back, and meters spend.
 *
 * Safety model (see PR discussion): a leaked proxy token is near-worthless —
 * it only works through this gateway, expires with the session, and is bounded
 * by both a per-session budget AND a durable global daily ceiling (Redis), which
 * is the real backstop against a public no-login endpoint.
 */
import type { SessionManager, TerminalSession } from "./sessions.ts";

interface ProviderConfig {
  base: string;
  /** Inject the real upstream credential. */
  applyAuth: (h: Headers) => void;
  /** Clamp the per-request output ceiling in the JSON body. */
  clampBody: (body: Record<string, unknown>, maxOut: number) => void;
}

const MAX_OUTPUT_TOKENS = Number(process.env.GW_MAX_OUTPUT_TOKENS ?? 2048);
const MAX_REQUEST_BYTES = Number(process.env.GW_MAX_REQUEST_BYTES ?? 256 * 1024);

function providers(): Record<string, ProviderConfig | undefined> {
  const anthropicKey = process.env.ANTHROPIC_API_KEY;
  const openaiKey = process.env.OPENAI_API_KEY;
  const xaiKey = process.env.XAI_API_KEY;
  return {
    anthropic: anthropicKey
      ? {
          base: "https://api.anthropic.com",
          applyAuth: (h) => h.set("x-api-key", anthropicKey),
          clampBody: (b, max) => {
            if (typeof b.max_tokens !== "number" || b.max_tokens > max) b.max_tokens = max;
          },
        }
      : undefined,
    openai: openaiKey
      ? {
          base: "https://api.openai.com",
          applyAuth: (h) => h.set("authorization", `Bearer ${openaiKey}`),
          clampBody: (b, max) => {
            if (typeof b.max_output_tokens === "number" && b.max_output_tokens > max)
              b.max_output_tokens = max;
            if (typeof b.max_tokens === "number" && b.max_tokens > max) b.max_tokens = max;
          },
        }
      : undefined,
    xai: xaiKey
      ? {
          base: "https://api.x.ai",
          applyAuth: (h) => h.set("authorization", `Bearer ${xaiKey}`),
          clampBody: (b, max) => {
            if (typeof b.max_tokens === "number" && b.max_tokens > max) b.max_tokens = max;
          },
        }
      : undefined,
  };
}

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

export interface GatewayDeps {
  sessions: SessionManager;
  /** Durable daily spend ledger (Redis-backed in prod, in-memory in dev). */
  daily: DailyLedger;
}

export interface DailyLedger {
  /** Current spend in USD for today. */
  get(): Promise<number>;
  /** Add usd to today's total; returns the new total. */
  add(usd: number): Promise<number>;
}

const DAILY_CEILING_USD = Number(process.env.GW_DAILY_CEILING_USD ?? 5);

function json(data: unknown, status: number): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "content-type": "application/json" },
  });
}

/**
 * Handle a `/gw/<provider>/...` request. Returns the streamed provider response
 * on success, or a JSON error (401/402/429/502) otherwise.
 */
export async function handleGatewayRequest(
  req: Request,
  url: URL,
  deps: GatewayDeps,
): Promise<Response> {
  const m = url.pathname.match(/^\/gw\/(anthropic|openai|xai)(\/.*)?$/);
  if (!m) return json({ error: "unknown gateway route" }, 404);
  const providerName = m[1]!;
  const provider = providers()[providerName];
  if (!provider) return json({ error: `${providerName} not configured` }, 503);

  // The CLI sends the proxy token where the real key would go.
  const token =
    req.headers.get("x-api-key") ??
    req.headers.get("authorization")?.replace(/^Bearer\s+/i, "") ??
    "";
  const session = deps.sessions.validateGatewayToken(token);
  if (!session) return json({ error: "invalid or expired trial token" }, 401);

  // Per-session budget.
  if (session.gatewaySpentUsd >= session.gatewayBudgetUsd) {
    return json(
      { error: "Free trial budget reached. Sign in with your own account to continue." },
      402,
    );
  }
  // Durable global daily ceiling — the real backstop.
  if ((await deps.daily.get()) >= DAILY_CEILING_USD) {
    return json({ error: "Daily free-trial capacity reached. Try again tomorrow or sign in." }, 429);
  }

  // Read + clamp the request body (these bodies are small; only the response streams).
  let bodyInit: BodyInit | undefined;
  if (req.method !== "GET" && req.method !== "HEAD") {
    const raw = await req.text();
    if (raw.length > MAX_REQUEST_BYTES) return json({ error: "request too large" }, 413);
    if (raw) {
      try {
        const parsed = JSON.parse(raw) as Record<string, unknown>;
        provider.clampBody(parsed, MAX_OUTPUT_TOKENS);
        bodyInit = JSON.stringify(parsed);
      } catch {
        bodyInit = raw; // non-JSON (rare) — forward verbatim
      }
    }
  }

  const headers = new Headers(req.headers);
  headers.delete("host");
  headers.delete("authorization");
  headers.delete("x-api-key");
  // Force identity encoding so the metering tee can read plaintext usage from
  // the response (a gzipped body would parse to no usage → no spend recorded).
  headers.set("accept-encoding", "identity");
  provider.applyAuth(headers);
  if (bodyInit !== undefined) headers.set("content-length", String(Buffer.byteLength(bodyInit as string)));

  const target = `${provider.base}${m[2] ?? ""}${url.search}`;
  let upstream: Response;
  try {
    upstream = await fetch(target, {
      method: req.method,
      headers,
      body: bodyInit,
      // @ts-expect-error bun supports half-duplex streaming
      duplex: "half",
    });
  } catch (err) {
    console.error(`[gw] upstream error ${providerName}:`, err);
    return json({ error: "upstream request failed" }, 502);
  }

  if (!upstream.body) {
    return new Response(null, { status: upstream.status, headers: upstream.headers });
  }

  // Tee: one branch streams to the CLI, the other is drained to meter usage.
  const [toClient, toMeter] = upstream.body.tee();
  void meter(toMeter, providerName, session, deps).catch(() => {});

  return new Response(toClient, {
    status: upstream.status,
    headers: upstream.headers,
  });
}

async function meter(
  stream: ReadableStream<Uint8Array>,
  providerName: string,
  session: TerminalSession,
  deps: GatewayDeps,
): Promise<void> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let text = "";
  // Cap accumulation so a runaway stream can't balloon memory.
  const CAP = 8 * 1024 * 1024;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (text.length < CAP) text += decoder.decode(value, { stream: true });
  }
  const usage = parseUsage(text);
  if (!usage) return;
  const usd = priceUsd(usage.model || defaultModel(providerName), usage.inTok, usage.outTok);
  if (usd <= 0) return;
  deps.sessions.addGatewaySpend(session.id, usd);
  await deps.daily.add(usd);
}

function defaultModel(provider: string): string {
  if (provider === "anthropic") return "sonnet";
  if (provider === "xai") return "grok";
  return "gpt-4o";
}
