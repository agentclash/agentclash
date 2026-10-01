import { expect, test } from "bun:test";
import { loadConfig } from "../server/config.ts";
import { SessionManager } from "../server/sessions.ts";
import { AllocationRejected } from "../server/sandbox.ts";
import { demo, deferred, eventually, FakeFactory, FakeSandbox, FakeSocket } from "./fakes.ts";

const config = () => loadConfig({
  TRY_CLI_MAX_CONCURRENT_SANDBOXES: "1", TRY_CLI_GATEWAY_URL: "https://terminal.example.test",
  OPENAI_API_KEY: crypto.randomUUID(),
});

test("destroy during boot kills a late allocation before releasing capacity", async () => {
  const factory = new FakeFactory(); const pending = deferred<FakeSandbox>(); factory.next = pending.promise;
  const manager = new SessionManager(config(), factory);
  const session = await manager.create("codex", demo, "192.0.2.1");
  const stop = manager.destroy(session.id);
  expect(manager.get(session.id)).toBeUndefined();
  expect(manager.size).toBe(1);
  await expect(manager.create("codex", demo, "192.0.2.2")).rejects.toThrow("at_capacity");
  const sandbox = new FakeSandbox(); pending.resolve(sandbox);
  await stop;
  expect(sandbox.kills).toBe(1); expect(sandbox.creates).toBe(0); expect(manager.size).toBe(0);
  await manager.destroy(session.id); await manager.close();
});

test("failed cleanup retains capacity and fails shutdown until confirmed killed", async () => {
  const factory = new FakeFactory(); const manager = new SessionManager(config(), factory);
  const session = await manager.create("codex", demo, "192.0.2.1");
  await eventually(() => session.status === "ready");
  factory.created[0]!.failKill = true;
  await expect(manager.destroy(session.id)).rejects.toThrow("cleanup failed");
  expect(manager.size).toBe(1); expect(manager.healthy).toBe(false);
  expect(manager.validateGatewayToken(session.proxyToken)).toBeUndefined();
  await expect(manager.close()).rejects.toThrow("incomplete");
  factory.created[0]!.failKill = false;
  await manager.close(); expect(manager.size).toBe(0);
});

test("confirmed rejection releases capacity; uncertain creation requires reconciliation", async () => {
  const factory = new FakeFactory(); const manager = new SessionManager(config(), factory);
  factory.next = Promise.reject(new AllocationRejected("Synthetic rejection"));
  await manager.create("codex", demo, "192.0.2.1");
  await eventually(() => manager.size === 0);
  factory.next = Promise.reject(new Error("Synthetic timeout"));
  const session = await manager.create("codex", demo, "192.0.2.1");
  await eventually(() => !manager.healthy);
  expect(manager.size).toBe(1);
  session.expiresAt = Date.now() - 1;
  await manager.expire(); expect(manager.size).toBe(1);
  await expect(manager.close()).rejects.toThrow("incomplete");
});

test("one PTY survives reconnection; stale socket cannot send input or clear ownership", async () => {
  const factory = new FakeFactory(); const manager = new SessionManager(config(), factory);
  const session = await manager.create("codex", demo, "192.0.2.1");
  await eventually(() => session.status === "ready");
  const first = new FakeSocket(); const next = new FakeSocket();
  await Promise.all([manager.attachPty(session, first), manager.attachPty(session, next)]);
  manager.detachPty(session, first);
  manager.sendInput(session, first, new TextEncoder().encode("bad"));
  manager.sendInput(session, next, new TextEncoder().encode("echo"));
  await eventually(() => next.output.includes("echo"));
  expect(session.ws).toBe(next); expect(first.closeCode).toBe(4001);
  expect(factory.created[0]!.creates).toBe(1); expect(factory.created[0]!.inputs).toEqual(["echo"]);
  await manager.close(); expect(next.closeCode).toBe(1001);
});

test("reset preserves anonymous expiry and budget even during old request metering", async () => {
  const factory = new FakeFactory(); const manager = new SessionManager(config(), factory);
  const session = await manager.create("codex", demo, "192.0.2.1");
  session.expiresAt = Date.now() + 10000; session.budget.spent = 0.1; session.budget.reserved = 0.05;
  const next = await manager.reset(session);
  await eventually(() => next.status === "ready");
  expect(next.expiresAt).toBe(session.expiresAt); expect(next.budget).toBe(session.budget);
  expect(factory.timeouts[1]).toBeLessThanOrEqual(10000);
  session.budget.spent += 0.05; expect(next.budget.spent).toBeCloseTo(0.15);
  next.expiresAt = Date.now() - 1;
  await expect(manager.reset(next)).rejects.toThrow("session_expired");
  manager.drain(); await expect(manager.create("codex", demo, "192.0.2.1")).rejects.toThrow("draining");
  await manager.close();
});

test("sandbox configuration contains only trial token; BYO tiers never receive operator keys", async () => {
  const cfg = config(); const factory = new FakeFactory(); const manager = new SessionManager(cfg, factory);
  for (const [slug, tier] of [["codex", "anonymous"], ["codex", "authenticated"], ["opencode", "anonymous"]] as const) {
    const session = await manager.create(slug, demo, "192.0.2.1", tier);
    await eventually(() => session.status === "ready");
    const sandbox = factory.created.at(-1)!;
    await manager.attachPty(session, new FakeSocket());
    expect(JSON.stringify([sandbox.environments, sandbox.filesWritten])).not.toContain(cfg.providerKeys.openai!);
    expect(session.trialWired).toBe(slug === "codex" && tier === "anonymous");
    if (tier === "authenticated") expect(manager.validateGatewayToken(session.proxyToken)).toBeUndefined();
    await manager.destroy(session.id);
  }
  await manager.close();
});

test("startup install failure is sanitized and cleans the allocated sandbox", async () => {
  const factory = new FakeFactory(); const sandbox = new FakeSandbox(); sandbox.failInstall = true;
  factory.next = Promise.resolve(sandbox);
  const manager = new SessionManager(config(), factory);
  const session = await manager.create("codex", { ...demo, install: ["synthetic-command"] }, "192.0.2.1");
  await eventually(() => manager.size === 0);
  expect(session.error).toBe("sandbox_start_failed"); expect(sandbox.kills).toBe(1);
  await manager.close();
});

test("destroy waits for an in-flight PTY before sandbox cleanup", async () => {
  const factory = new FakeFactory(); const manager = new SessionManager(config(), factory);
  const session = await manager.create("codex", demo, "192.0.2.1");
  await eventually(() => session.status === "ready");
  const wait = deferred<void>(); const sandbox = factory.created[0]!; sandbox.ptyWait = wait.promise;
  const attach = manager.attachPty(session, new FakeSocket());
  const destroy = manager.destroy(session.id);
  expect(manager.size).toBe(1); expect(sandbox.kills).toBe(0);
  wait.resolve(); await attach; await destroy;
  expect(sandbox.kills).toBe(1); expect(sandbox.disconnects).toBe(1); expect(manager.size).toBe(0);
  await manager.close();
});
