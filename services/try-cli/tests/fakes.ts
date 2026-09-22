import type { Demo } from "@try-cli/core";
import type { SandboxFactory, TerminalSandbox } from "../server/sandbox.ts";
import type { TerminalSocket } from "../server/sessions.ts";

export function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((ok, fail) => { resolve = ok; reject = fail; });
  return { promise, resolve, reject };
}
export async function eventually(check: () => boolean, timeoutMs = 2000) {
  const end = Date.now() + timeoutMs;
  while (!check()) {
    if (Date.now() > end) throw new Error("Condition timed out");
    await Bun.sleep(5);
  }
}
export const demo: Demo = {
  slug: "codex", name: "Test terminal", tagline: "Isolated test", sessionMinutes: 1,
  commands: [], welcome: "Welcome to the test terminal", image: "test", install: [],
};
export class FakeSandbox implements TerminalSandbox {
  kills = 0;
  failKill = false;
  killWait: Promise<void> | undefined;
  failInstall = false;
  inputs: string[] = [];
  filesWritten: string[] = [];
  environments: Record<string, string>[] = [];
  creates = 0;
  disconnects = 0;
  onData: ((data: Uint8Array) => void) | undefined;
  ptyWait: Promise<void> | undefined;
  commands = { run: async (_command: string) => ({ exitCode: this.failInstall ? 1 : 0 }) };
  files = { write: async (_path: string, content: string) => { this.filesWritten.push(content); } };
  pty = {
    create: async (opts: { envs: Record<string, string>; onData(data: Uint8Array): void }) => {
      this.creates++; this.environments.push(opts.envs); this.onData = opts.onData;
      await this.ptyWait;
      return { pid: 7, disconnect: async () => { this.disconnects++; } };
    },
    sendInput: async (_pid: number, data: Uint8Array) => {
      const input = new TextDecoder().decode(data);
      this.inputs.push(input); this.onData?.(data);
    },
  };
  async kill() {
    this.kills++;
    await this.killWait;
    if (this.failKill) throw new Error("Synthetic cleanup failure");
  }
}
export class FakeFactory implements SandboxFactory {
  created: FakeSandbox[] = [];
  timeouts: number[] = [];
  next: Promise<FakeSandbox> | undefined;
  available = true;
  async health() { if (!this.available) throw new Error("Synthetic dependency outage"); }
  async create(_template: string | undefined, timeoutMs: number) {
    this.timeouts.push(timeoutMs);
    const sandbox = this.next ? await this.next : new FakeSandbox();
    this.next = undefined; this.created.push(sandbox);
    return sandbox;
  }
}
export class FakeSocket implements TerminalSocket {
  readyState = 1;
  output: string[] = [];
  closeCode: number | undefined;
  send(data: string | Uint8Array) { this.output.push(typeof data === "string" ? data : new TextDecoder().decode(data)); }
  close(code?: number) { this.readyState = 3; this.closeCode = code; }
}
