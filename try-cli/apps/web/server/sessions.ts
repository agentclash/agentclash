import { Sandbox } from "e2b";
import type { Demo } from "@try-cli/core";
import type { ServerWebSocket } from "bun";

export interface TerminalSession {
  id: string;
  slug: string;
  demo: Demo;
  sandbox: Sandbox | null;
  ptyPid: number | null;
  ws: ServerWebSocket<unknown> | null;
  expiresAt: number;
  status: "starting" | "ready" | "expired" | "error";
  error?: string;
  mock: boolean;
}

const SESSION_TTL_MS = 10 * 60 * 1000;

export class SessionManager {
  private sessions = new Map<string, TerminalSession>();
  private ipCounts = new Map<string, { count: number; resetAt: number }>();
  private readonly maxSessionsPerIp = 3;
  private readonly useE2B: boolean;

  constructor() {
    this.useE2B = Boolean(process.env.E2B_API_KEY);
    if (!this.useE2B) {
      console.warn("[try-cli] E2B_API_KEY not set — running in mock terminal mode");
    }
    setInterval(() => this.cleanup(), 30_000);
  }

  checkRateLimit(ip: string): boolean {
    const now = Date.now();
    const entry = this.ipCounts.get(ip);
    if (!entry || now > entry.resetAt) {
      this.ipCounts.set(ip, { count: 1, resetAt: now + 60 * 60 * 1000 });
      return true;
    }
    if (entry.count >= this.maxSessionsPerIp) return false;
    entry.count++;
    return true;
  }

  get(id: string): TerminalSession | undefined {
    return this.sessions.get(id);
  }

  async create(slug: string, demo: Demo, ip: string): Promise<TerminalSession> {
    if (!this.checkRateLimit(ip)) {
      throw new Error("Rate limit exceeded. Try again later.");
    }

    const id = crypto.randomUUID();
    const session: TerminalSession = {
      id,
      slug,
      demo,
      sandbox: null,
      ptyPid: null,
      ws: null,
      expiresAt: Date.now() + (demo.sessionMinutes ?? 10) * 60 * 1000,
      status: "starting",
      mock: !this.useE2B,
    };
    this.sessions.set(id, session);

    if (this.useE2B) {
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

  private async bootstrapE2B(session: TerminalSession) {
    const { demo } = session;
    const timeoutMs = (demo.sessionMinutes ?? 10) * 60 * 1000;

    const sandbox = await Sandbox.create({
      timeoutMs,
      ...(demo.template ? { template: demo.template } : {}),
    });
    session.sandbox = sandbox;

    for (const cmd of demo.install ?? []) {
      const result = await sandbox.commands.run(cmd, { timeoutMs: 120_000 });
      if (result.exitCode !== 0) {
        throw new Error(`Install failed: ${cmd}\n${result.stderr}`);
      }
    }

    session.status = "ready";

    if (session.ws) {
      await this.attachPty(session, session.ws as ServerWebSocket<{ sessionId: string }>);
    }
  }

  async attachPty(session: TerminalSession, ws: ServerWebSocket<{ sessionId: string }>) {
    session.ws = ws;

    if (session.mock) {
      this.attachMockPty(session, ws);
      return;
    }

    if (!session.sandbox || session.status !== "ready") return;

    const { demo, sandbox } = session;
    const welcomeScript = demo.welcome
      ? `clear; echo ${JSON.stringify(demo.welcome)}; echo`
      : "clear";

    const handle = await sandbox.pty.create({
      cols: 80,
      rows: 24,
      timeoutMs: 0,
      onData: (data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(data);
        }
      },
      envs: {
        TERM: "xterm-256color",
        PS1: "\\[\\033[01;32m\\]\\u@try-cli\\[\\033[00m\\]:\\[\\033[01;34m\\]\\w\\[\\033[00m\\]$ ",
      },
    });

    session.ptyPid = handle.pid;

    if (welcomeScript) {
      await sandbox.pty.sendInput(handle.pid, new TextEncoder().encode(`${welcomeScript}\n`));
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
    await this.destroy(session.id);
    return this.create(session.slug, session.demo, "reset");
  }

  async destroy(id: string) {
    const session = this.sessions.get(id);
    if (!session) return;

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
    this.sessions.delete(id);
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
    for (const [id, session] of this.sessions) {
      if (now > session.expiresAt) {
        session.status = "expired";
        this.destroy(id);
      }
    }
  }
}

export const sessions = new SessionManager();
