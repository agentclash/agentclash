import { Sandbox, AuthenticationError, RateLimitError, TemplateError, InvalidArgumentError, SandboxNotFoundError } from "e2b";
import type { Config } from "./config.ts";

export interface PtyHandle { pid: number; disconnect(): Promise<void> }
export interface TerminalSandbox {
  commands: { run(command: string, signal?: AbortSignal): Promise<{ exitCode: number }> };
  files: { write(path: string, content: string): Promise<unknown> };
  pty: {
    create(opts: { cwd: string; envs: Record<string, string>; onData(data: Uint8Array): void }): Promise<PtyHandle>;
    sendInput(pid: number, data: Uint8Array): Promise<void>;
  };
  kill(): Promise<void>;
}
export interface SandboxFactory {
  create(template: string | undefined, timeoutMs: number, sessionId: string): Promise<TerminalSandbox>;
  health(): Promise<void>;
}
export class AllocationRejected extends Error {}

export function e2bFactory(config: Config): SandboxFactory | undefined {
  if (!config.e2bApiKey) return;
  const request = { apiKey: config.e2bApiKey, requestTimeoutMs: 10000 };
  return {
    async health() {
      await Sandbox.list({ ...request, requestTimeoutMs: 3000, limit: 1 }).nextItems();
    },
    async create(template, timeoutMs, sessionId) {
      let sandbox: Sandbox;
      try {
        const opts = { ...request, timeoutMs, metadata: { service: "try-cli", session: sessionId } };
        sandbox = template ? await Sandbox.create(template, opts) : await Sandbox.create(opts);
      } catch (err) {
        if (err instanceof AuthenticationError || err instanceof RateLimitError || err instanceof TemplateError || err instanceof InvalidArgumentError) {
          throw new AllocationRejected("Sandbox allocation rejected");
        }
        throw new Error("Sandbox allocation uncertain");
      }
      return {
        commands: { run: (command, signal) => sandbox.commands.run(command, { timeoutMs: 30000, requestTimeoutMs: 10000, signal }) },
        files: { write: (path, content) => sandbox.files.write(path, content, { requestTimeoutMs: 10000 }) },
        pty: {
          create: opts => sandbox.pty.create({ ...opts, cols: 80, rows: 24, timeoutMs: 0, requestTimeoutMs: 10000 }),
          sendInput: (pid, data) => sandbox.pty.sendInput(pid, data, { requestTimeoutMs: 10000 }),
        },
        async kill() {
          try { await sandbox.kill({ requestTimeoutMs: 10000 }); }
          catch (err) { if (!(err instanceof SandboxNotFoundError)) throw err; }
        },
      };
    },
  };
}
