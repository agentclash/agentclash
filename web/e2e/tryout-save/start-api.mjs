import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
const staging = await mkdtemp(path.join(tmpdir(), "agentclash-tryout-browser-api-"));
let child;
let stopped = false;
const run = (args, env = {}) => new Promise((resolve, reject) => {
 child = spawn(args[0], args.slice(1), { cwd: "../backend", env: { ...process.env, DATABASE_URL: "", ...env }, stdio: "inherit" });
 child.once("error", reject); child.once("exit", code => resolve(code));
});
for (const signal of ["SIGINT", "SIGTERM"]) process.on(signal, () => { stopped = true; child?.kill(signal); });
try {
 const binary = path.join(staging, "tryout-fixture");
 const build = await run(["go", "test", "-c", "-o", binary, "./internal/api"]);
 if (build !== 0) throw new Error("Could not build tryout browser fixture");
 if (!stopped) {
  const code = await run([binary, "-test.run", "^TestTryoutBrowserFixture$", "-test.timeout", "0"], { TRYOUT_BROWSER_FIXTURE: "1" });
  if (!stopped && code !== 0) process.exitCode = 1;
 }
} finally { await rm(staging, { recursive: true, force: true }); }
