import { expect, test } from "bun:test";
import { join } from "node:path";
import { openSync, closeSync, readFileSync, rmSync } from "node:fs";

const directory = process.env.TRY_CLI_TEST_PRIVATE_DIR;
const integration = directory ? test : test.skip;
integration("real process signals drain admission, clean sessions and report timeout as failure", async () => {
  for (const mode of ["normal", "hang"]) {
    rmSync(join(directory!, "process-cleanup.json"), { force: true });
    const logPath = join(directory!, `process-${mode}.log`);
    const log = openSync(logPath, "w", 0o600);
    const child = Bun.spawn([process.execPath, "--no-env-file", "--preserve-symlinks", "run", "tests/process-fixture.ts"], { cwd: import.meta.dir + "/../..",
      env: { PATH: process.env.PATH!, HOME: process.env.HOME!, TRY_CLI_TEST_PRIVATE_DIR: directory!, TRY_CLI_PROCESS_TEST_MODE: mode },
      stdout: log, stderr: log });
    try {
      let port: number | undefined;
      for (let i = 0; i < 80; i++) {
        const line = readFileSync(logPath, "utf8").split("\n").find(s => s.startsWith('{"port":'));
        if (line) { port = JSON.parse(line).port; break; }
        await Bun.sleep(25);
      }
      expect(port).toBeNumber();
      const base = `http://127.0.0.1:${port}`;
      const curl = Bun.spawn(["curl", "--silent", "--fail", base + "/health/ready"], { stdout: "ignore", stderr: "ignore" });
      expect(await curl.exited).toBe(0);
      const create = () => fetch(base + "/api/sessions", { method: "POST", body: '{"slug":"codex"}' });
      const response = await create(); expect(response.status).toBe(200);
      process.kill(child.pid, "SIGUSR2"); await Bun.sleep(40);
      expect((await fetch(base + "/health")).status).toBe(200);
      expect((await fetch(base + "/health/ready")).status).toBe(503);
      expect((await create()).status).toBe(503);
      const started = Date.now(); process.kill(child.pid, "SIGTERM");
      const exit = await Promise.race([child.exited, Bun.sleep(3000).then(() => -1)]);
      expect(exit).toBe(mode === "normal" ? 0 : 1);
      expect(Date.now() - started).toBeLessThan(2500);
      if (mode === "normal") expect(JSON.parse(readFileSync(join(directory!, "process-cleanup.json"), "utf8"))).toEqual({ sessions: 0, killed: true });
    } finally {
      if (child.exitCode === null) child.kill("SIGKILL");
      await child.exited; closeSync(log);
    }
  }
});

integration("production entrypoint refuses missing dependencies without mock fallback", async () => {
  const child = Bun.spawn([process.execPath, "--no-env-file", "--preserve-symlinks", "run", "server/index.ts"], { cwd: import.meta.dir + "/../..",
    env: { PATH: process.env.PATH!, HOME: process.env.HOME!, NODE_ENV: "production" }, stdout: "pipe", stderr: "pipe" });
  expect(await child.exited).toBe(1);
  const output = await new Response(child.stderr).text();
  expect(output).toContain("invalid production configuration");
  expect(output).not.toContain("mock");
});
