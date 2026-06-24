#!/usr/bin/env node
import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const repo = process.cwd();
const tmp = mkdtempSync(path.join(tmpdir(), "agentclash-skill-harness-"));
const detectRepo = path.join(tmp, "detect-repo");
mkdirSync(path.join(detectRepo, "web/content/agent-skills/detect-skill"), { recursive: true });
execFileSync("git", ["init"], { cwd: detectRepo, stdio: "ignore" });
execFileSync("git", ["config", "user.email", "test@example.com"], { cwd: detectRepo });
execFileSync("git", ["config", "user.name", "Test"], { cwd: detectRepo });
writeFileSync(path.join(detectRepo, "README.md"), "root\n");
execFileSync("git", ["add", "."], { cwd: detectRepo });
execFileSync("git", ["commit", "-m", "initial"], { cwd: detectRepo, stdio: "ignore" });
const baseSha = execFileSync("git", ["rev-parse", "HEAD"], { cwd: detectRepo, encoding: "utf8" }).trim();
writeFileSync(
  path.join(detectRepo, "web/content/agent-skills/detect-skill/SKILL.md"),
  `---
name: agentclash-detect-skill
description: Use when testing detection.
metadata:
  agentclash.role: testing
  agentclash.version: "1"
  agentclash.requires_cli: "false"
---

# Detect Skill
`,
);
execFileSync("git", ["add", "."], { cwd: detectRepo });
execFileSync("git", ["commit", "-m", "add skill"], { cwd: detectRepo, stdio: "ignore" });
const headSha = execFileSync("git", ["rev-parse", "HEAD"], { cwd: detectRepo, encoding: "utf8" }).trim();
const detected = JSON.parse(
  execFileSync("node", [path.join(repo, "scripts/internal-agent-skills/detect-changed-skills.mjs"), baseSha, headSha], {
    cwd: detectRepo,
    encoding: "utf8",
  }),
);
assert.deepEqual(detected, ["web/content/agent-skills/detect-skill/SKILL.md"]);

const skillDir = path.join(tmp, "web/content/agent-skills/example-skill");
mkdirSync(skillDir, { recursive: true });
const skillPath = path.join(skillDir, "SKILL.md");
writeFileSync(
  skillPath,
  `---
name: agentclash-example-skill
description: Use when testing internal skill harnesses.
metadata:
  agentclash.role: testing
  agentclash.version: "1"
  agentclash.requires_cli: "true"
---

# Example Skill

## Purpose
Test skill.
`,
);

const outDir = path.join(tmp, "out");
const manifestPath = execFileSync(
  "node",
  ["scripts/internal-agent-skills/prepare-skill-harnesses.mjs", JSON.stringify([skillPath]), outDir],
  {
    cwd: repo,
    env: {
      ...process.env,
      REPOSITORY_URL: "https://github.com/agentclash/agentclash.git",
      BASE_BRANCH: "codex/test",
      CODEX_MODEL: "codex-test-model",
      RUN_LABEL: "unit",
    },
    encoding: "utf8",
  },
).trim();

const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
assert.equal(manifest.harnesses.length, 1);
const spec = JSON.parse(readFileSync(manifest.harnesses[0].spec, "utf8"));
assert.match(spec.name, /^skill-self-test-unit-e3a05841-agentclash-example-skill$/);
assert.equal(spec.auth_mode, "api_key_secret");
assert.equal(spec.openai_api_key_secret_name, "OPENAI_API_KEY");
assert.equal(spec.repository_url, "https://github.com/agentclash/agentclash.git");
assert.equal(spec.base_branch, "codex/test");
assert.equal(spec.codex_model, "codex-test-model");
assert.match(spec.task_prompt, /Read only this file:/);
assert.match(spec.task_prompt, /agentclash-example-skill/);
assert.equal(spec.evaluation_config.validators[0].type, "command");
assert.match(spec.evaluation_config.validators[0].command, /validate-skill-harness-output\.mjs/);

const longSkillDir = path.join(tmp, "web/content/agent-skills/very-long-skill");
mkdirSync(longSkillDir, { recursive: true });
const longSkillPath = path.join(longSkillDir, "SKILL.md");
writeFileSync(
  longSkillPath,
  `---
name: agentclash-eval-pack-yaml-author-with-extra-long-name
description: Use when testing long harness names.
metadata:
  agentclash.role: testing
  agentclash.version: "1"
  agentclash.requires_cli: "false"
---

# Long Skill
`,
);
const longOutDir = path.join(tmp, "long-out");
const longManifestPath = execFileSync(
  "node",
  ["scripts/internal-agent-skills/prepare-skill-harnesses.mjs", JSON.stringify([longSkillPath]), longOutDir],
  {
    cwd: repo,
    env: {
      ...process.env,
      RUN_LABEL: "run-123456789-1",
    },
    encoding: "utf8",
  },
).trim();
const longManifest = JSON.parse(readFileSync(longManifestPath, "utf8"));
const longSpec = JSON.parse(readFileSync(longManifest.harnesses[0].spec, "utf8"));
assert.match(longSpec.name.slice(0, 60), /^skill-self-test-run-123456789-1-34cd64be-/);

const resultPath = path.join(tmp, "result.json");
writeFileSync(
  resultPath,
  JSON.stringify({
    skill_name: "agentclash-example-skill",
    skill_title: "Example Skill",
    used_only_skill_file: true,
    would_use_cli: true,
    hosted_backend: "https://api.agentclash.dev",
    commands: ["export AGENTCLASH_API_URL=https://api.agentclash.dev", "agentclash doctor"],
    files_or_payloads: ["N/A"],
    checks_before_mutation: ["agentclash doctor"],
    expected_outputs: ["doctor passes"],
    failure_modes: ["auth missing"],
    report_back: "ready",
    blockers_or_confirmations: ["workspace id"],
    confidence: "pass",
    notes: "ok",
  }),
);
execFileSync("node", ["scripts/internal-agent-skills/validate-skill-harness-output.mjs", skillPath, resultPath], {
  cwd: repo,
  encoding: "utf8",
});

writeFileSync(resultPath, JSON.stringify({ skill_name: "wrong" }));
const failed = spawnSync("node", ["scripts/internal-agent-skills/validate-skill-harness-output.mjs", skillPath, resultPath], {
  cwd: repo,
  encoding: "utf8",
});
assert.notEqual(failed.status, 0);
assert.match(failed.stderr, /skill_name/);

console.log("internal agent skill harness script tests passed");
