import { Sandbox } from "e2b";
import type { Demo } from "@try-cli/core";
import type { ServerWebSocket } from "bun";

export type SessionTier = "anonymous" | "authenticated";

export interface TerminalSession {
  id: string;
  slug: string;
  demo: Demo;
  ip: string;
  tier: SessionTier;
  /** High-entropy credential the sandbox CLIs send to the gateway (NOT the
   *  session id, which travels in loggable URLs/WS params). */
  proxyToken: string;
  /** Free-trial spend cap (USD) and running total, enforced by the gateway. */
  gatewayBudgetUsd: number;
  gatewaySpentUsd: number;
  /** Estimated spend for in-flight gateway requests, so concurrent requests
   *  can't all pass the budget check before any of them finishes metering. */
  gatewayReservedUsd: number;
  /** Whether this session's CLI is wired to the metered gateway (anon trial). */
  trialWired: boolean;
  /** Extra env injected into the PTY (gateway base URLs + proxy token). */
  ptyEnv: Record<string, string>;
  sandbox: Sandbox | null;
  ptyPid: number | null;
  ws: ServerWebSocket<unknown> | null;
  expiresAt: number;
  status: "starting" | "ready" | "expired" | "error";
  error?: string;
  mock: boolean;
}

// Per-tier session length (minutes). Anonymous trials are short and run on our
// keys; signed-in users get longer and bring their own credentials.
const ANON_MAX_MINUTES = Number(process.env.GW_ANON_MINUTES ?? 7);
const AUTH_MAX_MINUTES = Number(process.env.GW_AUTH_MINUTES ?? 20);
const ANON_BUDGET_USD = Number(process.env.GW_SESSION_BUDGET_USD ?? 0.3);
const SANDBOX_WORKDIR = "/home/user/project";
// Global cap on concurrent live sandboxes, to stay under E2B's per-team
// concurrency/rate limits (cost isn't the concern — rate limiting is).
const MAX_CONCURRENT_SANDBOXES = Number(process.env.TRY_CLI_MAX_CONCURRENT_SANDBOXES ?? 40);

// Demos whose free trial routes through the gateway, and the provider they use.
// Demos not listed are bring-your-own-credentials only (no anon trial).
const TRIAL_PROVIDER: Record<
  string,
  "anthropic" | "openai" | "xai" | "openrouter" | undefined
> = {
  "claude-code": "anthropic",
  codex: "openai",
  grok: "xai",
  // Each runs its OWN dedicated CLI pointed at OpenRouter through the gateway.
  "kimi-k2": "openrouter", // kimi-cli
  "qwen3-coder": "openrouter", // @qwen-code/qwen-code
};

const PROVIDER_ENV_KEY: Record<string, string> = {
  anthropic: "ANTHROPIC_API_KEY",
  openai: "OPENAI_API_KEY",
  xai: "XAI_API_KEY",
  openrouter: "OPENROUTER_API_KEY",
};

// OpenRouter model id per demo slug.
const OPENROUTER_MODEL: Record<string, string> = {
  "kimi-k2": "moonshotai/kimi-k2",
  "qwen3-coder": "qwen/qwen3-coder",
};

export class SessionManager {
  private sessions = new Map<string, TerminalSession>();
  // Reverse index: gateway proxy token -> session, for O(1) gateway auth.
  private byProxyToken = new Map<string, TerminalSession>();
  // Live (not yet destroyed) session count per IP. Incremented on create,
  // decremented on destroy, so a user who finishes/resets a session frees the slot.
  private liveSessionsPerIp = new Map<string, number>();
  private readonly maxSessionsPerIp = 3;
  // Count of live sandboxes we've reserved capacity for (E2B sessions).
  private activeSandboxes = 0;
  private readonly useE2B: boolean;

  constructor() {
    this.useE2B = Boolean(process.env.E2B_API_KEY);
    if (!this.useE2B) {
      console.warn("[try-cli] E2B_API_KEY not set — running in mock terminal mode");
    }
    setInterval(() => this.cleanup(), 30_000);
  }

  private hasCapacity(ip: string): boolean {
    return (this.liveSessionsPerIp.get(ip) ?? 0) < this.maxSessionsPerIp;
  }

  get(id: string): TerminalSession | undefined {
    return this.sessions.get(id);
  }

  async create(
    slug: string,
    demo: Demo,
    ip: string,
    tier: SessionTier = "anonymous",
  ): Promise<TerminalSession> {
    if (!this.hasCapacity(ip)) {
      throw new Error("Rate limit exceeded. Try again later.");
    }
    if (this.useE2B && this.activeSandboxes >= MAX_CONCURRENT_SANDBOXES) {
      throw new Error("Try CLI is at capacity right now — please try again in a moment.");
    }

    const id = crypto.randomUUID();
    const maxMinutes = tier === "authenticated" ? AUTH_MAX_MINUTES : ANON_MAX_MINUTES;
    const minutes = Math.min(demo.sessionMinutes ?? 10, maxMinutes);
    const session: TerminalSession = {
      id,
      slug,
      demo,
      ip,
      tier,
      proxyToken: `tct_${crypto.randomUUID().replace(/-/g, "")}${crypto.randomUUID().replace(/-/g, "")}`,
      gatewayBudgetUsd: ANON_BUDGET_USD,
      gatewaySpentUsd: 0,
      gatewayReservedUsd: 0,
      trialWired: false,
      ptyEnv: {},
      sandbox: null,
      ptyPid: null,
      ws: null,
      expiresAt: Date.now() + minutes * 60 * 1000,
      status: "starting",
      mock: !this.useE2B,
    };
    this.sessions.set(id, session);
    this.byProxyToken.set(session.proxyToken, session);
    this.liveSessionsPerIp.set(ip, (this.liveSessionsPerIp.get(ip) ?? 0) + 1);

    if (this.useE2B) {
      this.activeSandboxes++; // reserve a concurrency slot (released in destroy)
      this.bootstrapE2B(session).catch((err) => {
        session.status = "error";
        session.error = err instanceof Error ? err.message : String(err);
        console.error(`[session ${id}] bootstrap failed:`, err);
      });
    } else {
      session.status = "ready";
    }

    return session;
  }

  // e2b v2 takes the template alias as a positional arg — passing it inside the
  // options object is silently ignored. Retries on transient E2B rate limits.
  private async createSandboxWithRetry(template: string | undefined, timeoutMs: number) {
    let lastErr: unknown;
    for (let attempt = 0; attempt < 4; attempt++) {
      try {
        return template
          ? await Sandbox.create(template, { timeoutMs })
          : await Sandbox.create({ timeoutMs });
      } catch (err) {
        lastErr = err;
        const msg = err instanceof Error ? err.message : String(err);
        const rateLimited = /rate.?limit|429|too many requests/i.test(msg);
        if (!rateLimited || attempt === 3) throw err;
        await new Promise((r) => setTimeout(r, 500 * 2 ** attempt));
      }
    }
    throw lastErr;
  }

  private async bootstrapE2B(session: TerminalSession) {
    const { demo } = session;
    const timeoutMs = (demo.sessionMinutes ?? 10) * 60 * 1000;

    const sandbox = await this.createSandboxWithRetry(demo.template, timeoutMs);
    session.sandbox = sandbox;

    for (const cmd of demo.install ?? []) {
      const result = await sandbox.commands.run(cmd, { timeoutMs: 120_000 });
      if (result.exitCode !== 0) {
        throw new Error(`Install failed: ${cmd}\n${result.stderr}`);
      }
    }

    // Start the user in a project dir, not $HOME — some agent CLIs (e.g. Qwen
    // Code) refuse to run in the home directory.
    try {
      await session.sandbox.commands.run(`mkdir -p ${SANDBOX_WORKDIR}`);
    } catch {
      /* non-fatal */
    }

    await this.wireGatewayTrial(session);
    session.status = "ready";

    if (session.ws) {
      await this.attachPty(session, session.ws as ServerWebSocket<{ sessionId: string }>);
    }
  }

  /**
   * For an anonymous trial of a supported AI CLI, point the CLI at the metered
   * gateway with the session's proxy token. The real provider key never enters
   * the sandbox. No-op for authenticated (BYO) sessions, non-AI demos, providers
   * whose key isn't configured on the service, or when no gateway URL is set.
   */
  private async wireGatewayTrial(session: TerminalSession) {
    if (session.tier !== "anonymous") return;

    // opencode runs on opencode's own hosted "Zen" models with the operator's
    // Zen key injected directly (its own account, not routed through our gateway).
    if (session.slug === "opencode") {
      const zen = process.env.OPENCODE_ZEN_API_KEY;
      if (!zen) return;
      session.ptyEnv = { OPENCODE_API_KEY: zen };
      session.trialWired = true;
      return;
    }

    const provider = TRIAL_PROVIDER[session.slug];
    if (!provider) return;
    if (!process.env[PROVIDER_ENV_KEY[provider]]) return;
    const gw = process.env.TRY_CLI_GATEWAY_URL;
    if (!gw || !session.sandbox) return;
    const base = gw.replace(/\/$/, "");
    const token = session.proxyToken;
    const env: Record<string, string> = {};

    if (provider === "anthropic") {
      // ANTHROPIC_AUTH_TOKEN (Bearer) is the gateway path and skips the
      // first-use API-key approval prompt that ANTHROPIC_API_KEY triggers.
      env.ANTHROPIC_BASE_URL = `${base}/gw/anthropic`;
      env.ANTHROPIC_AUTH_TOKEN = token;
    } else if (provider === "openai") {
      env.OPENAI_API_KEY = token;
      const model = process.env.GW_CODEX_MODEL ?? "gpt-5-codex";
      const cfg =
        `model = "${model}"\n` +
        `model_provider = "trycli"\n` +
        `[model_providers.trycli]\n` +
        `name = "trycli"\n` +
        `base_url = "${base}/gw/openai/v1"\n` +
        `env_key = "OPENAI_API_KEY"\n` +
        `wire_api = "responses"\n`;
      try {
        await session.sandbox.files.write("/home/user/.codex/config.toml", cfg);
      } catch (err) {
        console.error(`[session ${session.id}] codex config write failed:`, err);
      }
    } else if (provider === "xai") {
      env.GROK_BASE_URL = `${base}/gw/xai/v1`;
      env.XAI_API_KEY = token;
      env.GROK_API_KEY = token;
    } else if (provider === "openrouter") {
      const model = OPENROUTER_MODEL[session.slug];
      if (!model) return;
      const orBase = `${base}/gw/openrouter/api/v1`;

      if (session.slug === "qwen3-coder") {
        // Qwen Code reads OPENAI_* env vars first-class.
        env.OPENAI_API_KEY = token;
        env.OPENAI_BASE_URL = orBase;
        env.OPENAI_MODEL = model;
      } else if (session.slug === "kimi-k2") {
        // Kimi CLI: an `openai_legacy` provider whose key comes from OPENAI_API_KEY.
        env.OPENAI_API_KEY = token;
        // Kimi CLI schema: provider block + a top-level [models.<id>] entry that
        // default_model references by id (not the raw model name).
        // Top-level keys MUST precede any [section] in TOML, or they get parsed
        // into the preceding table — so default_provider/default_model come first.
        const cfg =
          `default_provider = "trycli"\n` +
          `default_model = "trycli-default"\n\n` +
          `[providers.trycli]\n` +
          `type = "openai_legacy"\n` +
          `base_url = "${orBase}"\n` +
          `api_key = "${token}"\n\n` +
          `[models.trycli-default]\n` +
          `provider = "trycli"\n` +
          `model = "${model}"\n` +
          `max_context_size = 131072\n`;
        try {
          await session.sandbox.commands.run("mkdir -p /home/user/.kimi");
          await session.sandbox.files.write("/home/user/.kimi/config.toml", cfg);
        } catch (err) {
          console.error(`[session ${session.id}] kimi config write failed:`, err);
        }
      }
    }

    session.ptyEnv = env;
    session.trialWired = true;
  }

  /** Resolve + authorize a gateway proxy token. Returns the live anon session
   *  that is still within its time window, or undefined. */
  validateGatewayToken(token: string): TerminalSession | undefined {
    if (!token) return undefined;
    const s = this.byProxyToken.get(token);
    if (!s || s.tier !== "anonymous" || !s.trialWired) return undefined;
    if (s.status === "expired" || Date.now() > s.expiresAt) return undefined;
    return s;
  }

  addGatewaySpend(id: string, usd: number) {
    const s = this.sessions.get(id);
    if (s) s.gatewaySpentUsd += usd;
  }

  async attachPty(session: TerminalSession, ws: ServerWebSocket<{ sessionId: string }>) {
    session.ws = ws;

    if (session.mock) {
      this.attachMockPty(session, ws);
      return;
    }

    if (!session.sandbox || session.status !== "ready") return;

    const { demo, sandbox } = session;

    const handle = await sandbox.pty.create({
      cols: 80,
      rows: 24,
      cwd: SANDBOX_WORKDIR,
      timeoutMs: 0,
      onData: (data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(data);
        }
      },
      envs: {
        TERM: "xterm-256color",
        PS1: "\\[\\033[01;32m\\]\\u@try-cli\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]$ ",
        ...session.ptyEnv,
      },
    });

    session.ptyPid = handle.pid;

    // Render the welcome text directly to the terminal output instead of running
    // it through the shell — passing `demo.welcome` into a shell command would let
    // a `$(...)`/backtick payload in a demo config execute inside the sandbox.
    const enc = new TextEncoder();
    if (session.trialWired && ws.readyState === WebSocket.OPEN) {
      ws.send(
        enc.encode(
          `\x1b[32m✓ Free trial active\x1b[0m — running on AgentClash credentials, no key needed. ` +
            `Just run the command. Sign in to continue with your own account.\r\n\r\n`,
        ),
      );
    }
    if (demo.welcome && ws.readyState === WebSocket.OPEN) {
      const text = demo.welcome.replace(/\r?\n/g, "\r\n");
      ws.send(enc.encode(`${text}\r\n`));
    }
  }

  private mockBuffers = new Map<string, string>();

  private attachMockPty(session: TerminalSession, ws: ServerWebSocket<{ sessionId: string }>) {
    const encoder = new TextEncoder();
    const welcome = session.demo.welcome ?? `${session.demo.name} demo (mock mode — set E2B_API_KEY for real sandbox)\r\n`;
    const lines = welcome.split("\n").map((l) => `${l}\r\n`).join("");
    const prompt = `\r\n\x1b[32muser@try-cli\x1b[0m:\x1b[34m~\x1b[0m$ `;

    ws.send(encoder.encode(`\x1b[2J\x1b[H${lines}${prompt}`));
    this.mockBuffers.set(session.id, "");
  }

  handleMockInput(session: TerminalSession, ws: ServerWebSocket<{ sessionId: string }>, data: Uint8Array) {
    const encoder = new TextEncoder();
    const prompt = `\r\n\x1b[32muser@try-cli\x1b[0m:\x1b[34m~\x1b[0m$ `;
    let buffer = this.mockBuffers.get(session.id) ?? "";

    for (const char of new TextDecoder().decode(data)) {
      if (char === "\r") {
        ws.send(encoder.encode("\r\n"));
        const cmd = buffer.trim();
        buffer = "";
        this.mockBuffers.set(session.id, buffer);

        if (!cmd) {
          ws.send(encoder.encode(prompt));
          continue;
        }

        const responses: Record<string, string> = {
          help: "Mock terminal — set E2B_API_KEY for a real Linux sandbox.\r\nCommands: help, clear",
          clear: "\x1b[2J\x1b[H",
        };
        const lower = cmd.toLowerCase();
        if (lower === "clear") {
          ws.send(encoder.encode(responses.clear + prompt));
          continue;
        }
        const output = responses[lower] ?? `\x1b[33m[mock]\x1b[0m Would run: ${cmd}\r\n`;
        ws.send(encoder.encode(output + prompt));
      } else if (char === "\u007F") {
        if (buffer.length > 0) {
          buffer = buffer.slice(0, -1);
          this.mockBuffers.set(session.id, buffer);
          ws.send(encoder.encode("\b \b"));
        }
      } else if (char >= " " || char === "\t") {
        buffer += char;
        this.mockBuffers.set(session.id, buffer);
        ws.send(encoder.encode(char));
      }
    }
  }

  async reset(session: TerminalSession): Promise<TerminalSession> {
    const originalExpiry = session.expiresAt;
    const tier = session.tier;
    await this.destroy(session.id);
    // Reuse the original client IP so a reset (destroy + create) nets to the same
    // live-session count for that IP rather than charging a shared bucket.
    const next = await this.create(session.slug, session.demo, session.ip, tier);
    // A reset must not extend an anonymous trial past its original deadline —
    // carry over the original expiry so the free window stays hard-capped.
    if (tier === "anonymous") next.expiresAt = originalExpiry;
    return next;
  }

  async destroy(id: string) {
    const session = this.sessions.get(id);
    if (!session) return;

    // Remove from the maps first so concurrent/double destroys can't double-count.
    this.sessions.delete(id);
    this.byProxyToken.delete(session.proxyToken);
    if (!session.mock) this.activeSandboxes = Math.max(0, this.activeSandboxes - 1);
    const live = (this.liveSessionsPerIp.get(session.ip) ?? 0) - 1;
    if (live > 0) {
      this.liveSessionsPerIp.set(session.ip, live);
    } else {
      this.liveSessionsPerIp.delete(session.ip);
    }

    if (session.ptyPid && session.sandbox) {
      try {
        await session.sandbox.pty.kill(session.ptyPid);
      } catch {
        /* ignore */
      }
    }
    if (session.sandbox) {
      try {
        await session.sandbox.kill();
      } catch {
        /* ignore */
      }
    }
  }

  sendInput(session: TerminalSession, ws: ServerWebSocket<{ sessionId: string }>, data: Uint8Array) {
    if (session.mock) {
      this.handleMockInput(session, ws, data);
      return;
    }
    if (session.sandbox && session.ptyPid) {
      session.sandbox.pty.sendInput(session.ptyPid, data).catch(console.error);
    }
  }

  resize(session: TerminalSession, cols: number, rows: number) {
    if (session.sandbox && session.ptyPid) {
      session.sandbox.pty.resize(session.ptyPid, { cols, rows }).catch(console.error);
    }
  }

  private cleanup() {
    const now = Date.now();
    // Collect first — destroy() mutates the sessions Map, so we must not delete
    // from it while iterating.
    const expired: string[] = [];
    for (const [id, session] of this.sessions) {
      if (now > session.expiresAt) expired.push(id);
    }
    for (const id of expired) {
      const session = this.sessions.get(id);
      if (session) session.status = "expired";
      void this.destroy(id);
    }
  }
}

export const sessions = new SessionManager();
