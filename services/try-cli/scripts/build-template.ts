/**
 * Builds the `agentclash-trycli` E2B sandbox template in E2B's cloud
 * (Build System 2.0 — no local Docker required).
 *
 * Every CLI offered by a Try CLI demo is pre-installed here so sandboxes boot
 * ready in ~1-2s instead of running a download+install on every visit.
 *
 * Usage:
 *   cd services/try-cli
 *   E2B_API_KEY=e2b_... bun run scripts/build-template.ts
 *
 * The positional alias ("agentclash-trycli") is what the runtime references via
 * Sandbox.create("agentclash-trycli", ...). Re-run after changing tool versions.
 */
import { Template, defaultBuildLogger } from "e2b";

export const TEMPLATE_ALIAS = "agentclash-trycli";

const template = Template()
  .fromNodeImage("22")
  // System tools (aptInstall runs privileged)
  .aptInstall(["ripgrep", "git", "curl", "ca-certificates", "unzip"])
  // Dev tools + AI coding CLIs as global npm binaries. npmInstall runs as root,
  // so these land in /usr/local/bin (on PATH for the sandbox user).
  .npmInstall(
    [
      "bun",
      "@biomejs/biome",
      "@anthropic-ai/claude-code",
      "@openai/codex",
      "opencode-ai",
      "grok-dev",
    ],
    { g: true },
  )
  // uv + ruff (Astral installers). runCmd runs as the unprivileged sandbox
  // user, so these install into ~/.local/bin (/home/user/.local/bin).
  .runCmd("curl -LsSf https://astral.sh/uv/install.sh | sh")
  .runCmd("curl -LsSf https://astral.sh/ruff/install.sh | sh")
  .setEnvs({
    PATH: "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/home/user/.local/bin",
  });

async function main() {
  await Template.build(template, TEMPLATE_ALIAS, {
    cpuCount: 2,
    memoryMB: 2048,
    onBuildLogs: defaultBuildLogger(),
  });
  console.log(`\n✓ Built E2B template alias: ${TEMPLATE_ALIAS}`);
}

main().catch((err) => {
  console.error("Template build failed:", err);
  process.exit(1);
});
