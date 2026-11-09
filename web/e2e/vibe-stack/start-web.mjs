import { recordOwned } from "./ownership.mjs";
import { startAuthFixture } from "./auth-fixture.mjs";
import { spawn } from "node:child_process";
import { cp, mkdir, rm, symlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { fileURLToPath } from "node:url";

// Next rewrites next-env.d.ts/tsconfig during startup. Run the unchanged app
// source from a temporary copy so this test never touches an active .next tree
// or developer configuration. No .env files or credentials are copied.
const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const staging = path.join(tmpdir(), `agentclash-vibe-browser-${randomUUID()}`);
await recordOwned("frontend_directory",staging);
await mkdir(staging);
await recordOwned("frontend_directory", staging, {phase:"created"});
let child;
let closeAuth;
let stopping = false;
let shutdown;
let startupDone;
const started = new Promise(resolve => { startupDone = resolve; });
function stop(signal = "SIGTERM") {
  stopping = true;
  return shutdown ??= (async () => {
    // Startup must settle before cleanup; otherwise it could spawn after stop.
    await started;
    if (child && child.exitCode === null && child.signalCode === null) {
      const exited = new Promise(resolve => child.once("exit", resolve));
      child.kill(signal);
      const timer = setTimeout(() => child.kill("SIGKILL"), 5000);
      try { await exited; } finally { clearTimeout(timer); }
    }
    if (closeAuth) await closeAuth();
    await rm(staging, { recursive: true, force: true });
    await recordOwned("frontend_directory", staging, {phase:"absent"});
  })();
}
const stopAfterSignal = signal => { void stop(signal).catch(error => { console.error(error); process.exitCode = 1; }); };
process.on("SIGTERM", () => stopAfterSignal("SIGTERM"));
process.on("SIGINT", () => stopAfterSignal("SIGINT"));
try {
  await recordOwned("loopback_listener", `127.0.0.1:${process.env.WORKOS_API_PORT || "55442"}`);
  closeAuth = await startAuthFixture(Number(process.env.WORKOS_API_PORT || "55442"), `http://127.0.0.1:${process.env.VIBE_BROWSER_WEB_PORT ?? "53518"}`);
  if (stopping) throw new Error("Frontend startup interrupted");
  await Promise.all([
    ...["src", "public", "content", "package.json", "tsconfig.json", "next.config.ts", "postcss.config.mjs"].map(
      entry => cp(path.join(web, entry), path.join(staging, entry), { recursive: true }),
    ),
    symlink(path.join(web, "node_modules"), path.join(staging, "node_modules"), "dir"),
  ]);
  if (stopping) throw new Error("Frontend startup interrupted");
  await recordOwned("frontend_process", `next:127.0.0.1:${process.env.VIBE_BROWSER_WEB_PORT || "53518"}:${staging}`);
  if (stopping) throw new Error("Frontend startup interrupted");
  child = spawn(process.execPath, [
    path.join(web, "node_modules/next/dist/bin/next"),
    "dev", "--hostname", "127.0.0.1", "--port", process.env.VIBE_BROWSER_WEB_PORT ?? "53518",
  ], { cwd: staging, env: process.env, stdio: "inherit" });
  const exited = new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", resolve);
  });
  startupDone();
  await recordOwned("frontend_process", staging, {phase:"started",child_pid:child.pid});
  const code = await exited;
  await recordOwned("frontend_process", staging, {phase:"exited",child_pid:child.pid,exit_code:code,signal:child.signalCode});
  await stop();
  process.exitCode = typeof code === "number" ? code : 0;
} catch (error) {
  startupDone();
  await stop();
  throw error;
}
