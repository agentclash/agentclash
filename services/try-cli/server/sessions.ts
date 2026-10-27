import type { Demo } from "@try-cli/core";
import type { Config } from "./config.ts";
import type { SandboxFactory, TerminalSandbox, PtyHandle } from "./sandbox.ts";
import { AllocationRejected } from "./sandbox.ts";

export type SessionTier = "anonymous" | "authenticated";
export interface TerminalSocket {
  readyState: number;
  send(data: string | Uint8Array): unknown;
  close(code?: number, reason?: string): void;
}
export interface TrialBudget { limit: number; spent: number; reserved: number }
export interface TerminalSession {
  id: string; slug: string; demo: Demo; ip: string; tier: SessionTier;
  proxyToken: string; budget: TrialBudget; trialWired: boolean;
  ptyEnv: Record<string, string>; sandbox: TerminalSandbox | null;
  ptyPid: number | null; ws: TerminalSocket | null; expiresAt: number;
  status: "starting" | "ready" | "expired" | "error";
  error?: string; mock: boolean;
}
interface Entry {
  session: TerminalSession;
  boot: Promise<void>;
  closing: boolean;
  abort: AbortController;
  destroy?: Promise<void>;
  ptyStart?: Promise<void>;
  handle?: PtyHandle;
  uncertainAllocation: boolean;
  cleanupFailed: boolean;
  resetStarted: boolean;
}
export class AdmissionError extends Error {}
const SANDBOX_WORKDIR = "/home/user/project";
const TRIAL_PROVIDER = {
  codex: "openai", grok: "xai", "kimi-k2": "openrouter", "qwen3-coder": "openrouter",
} as Record<string, "anthropic" | "openai" | "xai" | "openrouter" | undefined>;
const OPENROUTER_MODEL: Record<string, string> = {
  "kimi-k2": "moonshotai/kimi-k2", "qwen3-coder": "qwen/qwen3-coder",
};

export class SessionManager {
  private entries = new Map<string, Entry>();
  private byProxyToken = new Map<string, TerminalSession>();
  private draining = false;
  private timer: ReturnType<typeof setInterval>;

  constructor(private config: Config, private factory?: SandboxFactory) {
    if (config.production && !factory) throw new Error("Production sandbox required");
    this.timer = setInterval(() => { void this.expire(); }, 1000);
    this.timer.unref();
  }
  get size() { return this.entries.size; }
  get healthy() { return !this.draining && ![...this.entries.values()].some(e => e.cleanupFailed); }
  drain() { this.draining = true; }
  get(id: string) {
    const entry = this.entries.get(id);
    return entry && !entry.closing ? entry.session : undefined;
  }
  assertAdmission(ip: string) {
    if (this.draining) throw new AdmissionError("draining");
    if (!this.healthy) throw new AdmissionError("cleanup_required");
    if (this.entries.size >= this.config.maxSandboxes ||
      [...this.entries.values()].filter(e => e.session.ip === ip).length >= 3) throw new AdmissionError("at_capacity");
  }
  async create(slug: string, demo: Demo, ip: string, tier: SessionTier = "anonymous",
    previous?: { expiresAt: number; budget: TrialBudget }): Promise<TerminalSession> {
    this.assertAdmission(ip);
    const minutes = Math.min(demo.sessionMinutes ?? 10, tier === "anonymous" ? this.config.anonMinutes : this.config.authMinutes);
    const expiresAt = previous?.expiresAt ?? Date.now() + minutes * 60000;
    if (expiresAt <= Date.now()) throw new AdmissionError("session_expired");
    const session: TerminalSession = {
      id: crypto.randomUUID(), slug, demo, ip, tier,
      proxyToken: `tct_${crypto.randomUUID().replaceAll("-", "")}${crypto.randomUUID().replaceAll("-", "")}`,
      budget: previous?.budget ?? { limit: this.config.sessionBudget, spent: 0, reserved: 0 },
      trialWired: false, ptyEnv: {}, sandbox: null, ptyPid: null, ws: null,
      expiresAt, status: this.factory ? "starting" : "ready", mock: !this.factory,
    };
    const entry: Entry = { session, closing: false, abort: new AbortController(), boot: Promise.resolve(),
      uncertainAllocation: false, cleanupFailed: false, resetStarted: false };
    this.entries.set(session.id, entry);
    this.byProxyToken.set(session.proxyToken, session);
    if (this.factory) {
      entry.boot = this.bootstrap(entry).catch(() => {
        session.status = "error"; session.error = "sandbox_start_failed";
      });
      void entry.boot.then(() => {
        if (session.status === "error") return this.destroy(session.id);
      }).catch(() => { console.error("[try-cli] sandbox cleanup failed; capacity retained"); });
    }
    return session;
  }
  private live(entry: Entry) {
    return !entry.closing && Date.now() < entry.session.expiresAt;
  }
  private async bootstrap(entry: Entry) {
    const session = entry.session;
    try {
      // Creation remains tracked until it resolves, including a late allocation
      // after drain/destroy. The provider TTL is always bounded by the trial.
      session.sandbox = await this.factory!.create(session.demo.template,
        Math.max(1, session.expiresAt - Date.now()), session.id);
    } catch (err) {
      entry.uncertainAllocation = !(err instanceof AllocationRejected);
      throw err;
    }
    if (!this.live(entry)) return;
    for (const command of session.demo.install ?? []) {
      const result = await session.sandbox.commands.run(command, entry.abort.signal);
      if (!this.live(entry)) return;
      if (result.exitCode !== 0) throw new Error("Install failed");
    }
    await session.sandbox.commands.run(`mkdir -p ${SANDBOX_WORKDIR}`, entry.abort.signal);
    if (!this.live(entry)) return;
    await this.wireGatewayTrial(session);
    if (!this.live(entry)) return;
    session.status = "ready";
    if (session.ws) await this.attachPty(session, session.ws);
  }
  private async wireGatewayTrial(session: TerminalSession) {
    if (session.tier !== "anonymous") return;

    // opencode and Claude Code are BYO-only. Operator keys must never enter
    // a disposable sandbox, even when a provider offers promotional credits.
    const provider = TRIAL_PROVIDER[session.slug];
    if (!provider) return;
    if (!this.config.providerKeys[provider]) return;
    const gw = this.config.gatewayOrigin;
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
      const model = this.config.codexModel;
      const cfg =
        `model = "${model}"\n` +
        `model_provider = "trycli"\n` +
        `[model_providers.trycli]\n` +
        `name = "trycli"\n` +
        `base_url = "${base}/gw/openai/v1"\n` +
        `env_key = "OPENAI_API_KEY"\n` +
        `wire_api = "responses"\n`;
      await session.sandbox.files.write("/home/user/.codex/config.toml", cfg);
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
        await session.sandbox.commands.run("mkdir -p /home/user/.kimi");
        await session.sandbox.files.write("/home/user/.kimi/config.toml", cfg);
      }
    }

    session.ptyEnv = env;
    session.trialWired = true;
  }

  validateGatewayToken(token: string) {
    const session = this.byProxyToken.get(token);
    if (!session || session.tier !== "anonymous" || !session.trialWired || session.status !== "ready" ||
      Date.now() >= session.expiresAt) return;
    return session;
  }
  async attachPty(session: TerminalSession, ws: TerminalSocket) {
    const entry = this.entries.get(session.id);
    if (!entry || !this.live(entry)) { ws.close(4004, "Session unavailable"); return; }
    if (session.ws && session.ws !== ws) session.ws.close(4001, "Terminal reconnected");
    session.ws = ws;
    if (session.mock) { this.attachMockPty(session, ws); return; }
    if (session.status !== "ready" || !session.sandbox) return;
    // Keep one remote PTY and its output stream for the session lifetime.
    // A new browser socket takes ownership; stale close callbacks cannot clear it.
    if (!entry.ptyStart) {
      entry.ptyStart = (async () => {
        const handle = await session.sandbox!.pty.create({
          cwd: SANDBOX_WORKDIR,
          envs: { TERM: "xterm-256color", ...session.ptyEnv },
          onData: data => { if (this.live(entry) && session.ws?.readyState === 1) session.ws.send(data); },
        });
        entry.handle = handle; session.ptyPid = handle.pid;
      })();
    }
    await entry.ptyStart;
    if (this.live(entry) && session.ws === ws && ws.readyState === 1) {
      if (session.trialWired) ws.send("\r\nFree trial active. Sign in to continue with your own credentials.\r\n");
      if (session.demo.welcome) ws.send(session.demo.welcome.replace(/\r?\n/g, "\r\n") + "\r\n");
    }
  }
  detachPty(session: TerminalSession, ws: TerminalSocket) {
    if (session.ws === ws) session.ws = null;
  }
  private mockBuffers = new Map<string, string>();

  private attachMockPty(session: TerminalSession, ws: TerminalSocket) {
    const encoder = new TextEncoder();
    const welcome = session.demo.welcome ?? `${session.demo.name} demo (mock mode — set E2B_API_KEY for real sandbox)\r\n`;
    const lines = welcome.split("\n").map((l) => `${l}\r\n`).join("");
    const prompt = `\r\n\x1b[32muser@try-cli\x1b[0m:\x1b[34m~\x1b[0m$ `;

    ws.send(encoder.encode(`\x1b[2J\x1b[H${lines}${prompt}`));
    this.mockBuffers.set(session.id, "");
  }

  handleMockInput(session: TerminalSession, ws: TerminalSocket, data: Uint8Array) {
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

  async reset(session: TerminalSession) {
    this.assertAdmissionForReset();
    const entry = this.entries.get(session.id);
    if (!entry || entry.closing || entry.resetStarted) throw new AdmissionError("session_unavailable");
    entry.resetStarted = true;
    await this.destroy(session.id);
    // Share the budget object with in-flight metering from the previous sandbox.
    return this.create(session.slug, session.demo, session.ip, session.tier,
      session.tier === "anonymous" ? { expiresAt: session.expiresAt, budget: session.budget } : undefined);
  }
  private assertAdmissionForReset() {
    if (this.draining) throw new AdmissionError("draining");
  }
  destroy(id: string): Promise<void> {
    const entry = this.entries.get(id);
    if (!entry) return Promise.resolve();
    if (entry.destroy) return entry.destroy;
    entry.closing = true;
    entry.abort.abort();
    const session = entry.session;
    this.byProxyToken.delete(session.proxyToken);
    session.ws?.close(1001, "Session ended"); session.ws = null;
    entry.destroy = (async () => {
      await entry.boot;
      await entry.ptyStart?.catch(() => {});
      // Even disconnect failure must not prevent killing the whole sandbox.
      await entry.handle?.disconnect().catch(() => {});
      if (session.sandbox) await session.sandbox.kill();
      else if (entry.uncertainAllocation) {
        // A timed-out create may have reached E2B. Retain capacity and report a
        // failed cleanup for operator reconciliation. The local clock cannot
        // prove when an unacknowledged remote allocation actually began.
        throw new Error("Sandbox allocation uncertain");
      }
      this.entries.delete(id);
      this.mockBuffers.delete(id);
    })().catch(() => {
      entry.cleanupFailed = true; entry.destroy = undefined;
      throw new Error("Sandbox cleanup failed");
    });
    return entry.destroy;
  }
  sendInput(session: TerminalSession, ws: TerminalSocket, data: Uint8Array) {
    const entry = this.entries.get(session.id);
    if (!entry || !this.live(entry) || session.ws !== ws) return;
    if (session.mock) { this.handleMockInput(session, ws, data); return; }
    if (session.sandbox && session.ptyPid !== null) {
      void session.sandbox.pty.sendInput(session.ptyPid, data).catch(() => ws.close(1011, "Terminal input failed"));
    }
  }
  async expire() {
    await Promise.all([...this.entries.values()].filter(e => Date.now() >= e.session.expiresAt || e.cleanupFailed).map(async e => {
      e.session.status = "expired";
      try { await this.destroy(e.session.id); } catch { /* capacity remains occupied */ }
    }));
  }
  async close() {
    this.drain(); clearInterval(this.timer);
    const results = await Promise.allSettled([...this.entries.keys()].map(id => this.destroy(id)));
    if (results.some(r => r.status === "rejected")) throw new Error("Sandbox cleanup incomplete");
  }
}
