import { spawn } from "node:child_process";
import { cp, mkdtemp, rm, symlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Next rewrites next-env.d.ts/tsconfig during startup. Run the unchanged app
// source from a temporary copy so this test never touches an active .next tree
// or developer configuration. No .env files or credentials are copied.
const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const staging = await mkdtemp(path.join(tmpdir(), "agentclash-vibe-browser-"));
let child;
let stopping = false;
async function stop(signal = "SIGTERM") {
  if (stopping) return;
  stopping = true;
  if (child && child.exitCode === null) {
    child.kill(signal);
    await new Promise(resolve => child.once("exit", resolve));
  }
  await rm(staging, { recursive: true, force: true });
}
process.on("SIGTERM", () => void stop());
process.on("SIGINT", () => void stop("SIGINT"));
try {
  await Promise.all([
    ...["src", "public", "package.json", "tsconfig.json", "next.config.ts", "postcss.config.mjs"].map(
      entry => cp(path.join(web, entry), path.join(staging, entry), { recursive: true }),
    ),
    symlink(path.join(web, "node_modules"), path.join(staging, "node_modules"), "dir"),
  ]);
  child = spawn(process.execPath, [
    path.join(web, "node_modules/next/dist/bin/next"),
    "dev", "--hostname", "127.0.0.1", "--port", process.env.VIBE_BROWSER_WEB_PORT ?? "53518",
  ], { cwd: staging, env: process.env, stdio: "inherit" });
  const code = await new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", resolve);
  });
  await stop();
  process.exitCode = typeof code === "number" ? code : 0;
} catch (error) {
  await stop();
  throw error;
}
