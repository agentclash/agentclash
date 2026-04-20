#!/usr/bin/env node
// Thin Node.js shim that resolves the platform-specific AgentClash binary
// shipped as an optional dependency and execs it with the caller's args.
// No downloads happen at install or run time — each platform package carries
// its binary, and npm's `os`/`cpu` filters ensure only the right one lands
// in node_modules for the host.
"use strict";

const { spawnSync } = require("node:child_process");

const platform = process.platform;
const arch = process.arch;
const ext = platform === "win32" ? ".exe" : "";
const pkg = `@agentclash/cli-${platform}-${arch}`;

let binPath;
try {
  binPath = require.resolve(`${pkg}/bin/agentclash${ext}`);
} catch (err) {
  console.error(
    `agentclash: no prebuilt binary for ${platform}-${arch} (looked for ${pkg}).`
  );
  console.error(
    "Install manually from https://github.com/agentclash/agentclash/releases " +
      "or open an issue if your platform should be supported."
  );
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(`agentclash: failed to spawn binary: ${result.error.message}`);
  process.exit(1);
}
if (typeof result.status === "number") {
  process.exit(result.status);
}
if (result.signal) {
  process.kill(process.pid, result.signal);
  return;
}
process.exit(1);
