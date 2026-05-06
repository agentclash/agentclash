#!/usr/bin/env node
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

const skillsArg = process.env.CHANGED_SKILLS_JSON || process.argv[2] || "[]";
const outDir = process.env.OUT_DIR || process.argv[3] || ".agentclash/skill-harnesses";
const repoUrl = process.env.REPOSITORY_URL || "https://github.com/agentclash/agentclash.git";
const baseBranch = process.env.BASE_BRANCH || "";
const openAISecretName = process.env.OPENAI_SECRET_NAME || "OPENAI_API_KEY";
const codexModel = process.env.CODEX_MODEL || "";
const runLabel = process.env.RUN_LABEL || "local";
const safeRunLabel = runLabel.replace(/[^a-z0-9-]+/gi, "-").replace(/^-+|-+$/g, "") || "local";

function parseFrontmatter(content, file) {
  if (!content.startsWith("---\n")) {
    throw new Error(`${file}: missing YAML frontmatter`);
  }
  const end = content.indexOf("\n---", 4);
  if (end === -1) {
    throw new Error(`${file}: unterminated YAML frontmatter`);
  }
  const raw = content.slice(4, end).trim();
  const fields = {};
  let inMetadata = false;
  for (const line of raw.split("\n")) {
    if (/^\s*$/.test(line)) continue;
    if (line === "metadata:") {
      inMetadata = true;
      continue;
    }
    const match = line.match(/^(\s*)([A-Za-z0-9_.-]+):\s*(.*)$/);
    if (!match) continue;
    const [, indent, key, rawValue] = match;
    const value = rawValue.replace(/^["']|["']$/g, "");
    if (inMetadata && indent.length > 0) {
      fields[`metadata.${key}`] = value;
    } else {
      inMetadata = false;
      fields[key] = value;
    }
  }
  return fields;
}

function titleFromSkill(content) {
  const match = content.match(/^#\s+(.+)$/m);
  return match ? match[1].trim() : "";
}

function taskPrompt(skillPath, meta, title) {
  const cliRequired = meta["metadata.agentclash.requires_cli"] === "true";
  return `You are an isolated coding agent testing whether an AgentClash skill is self-contained.

You know nothing about AgentClash or this repository. Do not inspect source code, tests, git history, GitHub issues, or docs outside the skill file. Read only this file:

${skillPath}

Use the skill exactly as a downstream coding agent would. Prepare an operational plan for a realistic user request that matches the skill description.

Write your result as JSON to .agentclash/skill-eval-result.json with this shape:

{
  "skill_name": "${meta.name}",
  "skill_title": "${title}",
  "used_only_skill_file": true,
  "would_use_cli": ${cliRequired},
  "hosted_backend": "https://api.agentclash.dev or N/A",
  "commands": ["ordered commands or N/A"],
  "files_or_payloads": ["JSON/YAML/config payloads or N/A"],
  "checks_before_mutation": ["read-only checks before create/update/delete/publish"],
  "expected_outputs": ["success signals from the skill"],
  "failure_modes": ["likely blockers and recovery steps"],
  "report_back": "concise report-back text",
  "blockers_or_confirmations": ["needed user confirmations or missing IDs/secrets"],
  "confidence": "pass or fail",
  "notes": "short explanation"
}

Rules:
- Do not claim you inspected repo source.
- Do not invent raw secret values.
- If the skill requires CLI use, include AGENTCLASH_API_URL=https://api.agentclash.dev or an equivalent hosted production setup command.
- If the skill is not self-contained, set confidence to "fail" and explain exactly what was missing.
- Keep the JSON valid and do not wrap it in Markdown.`;
}

function validatorCommand(skillPath) {
  return `node scripts/internal-agent-skills/validate-skill-harness-output.mjs ${JSON.stringify(skillPath)} .agentclash/skill-eval-result.json`;
}

function shortSkillKey(skillPath, skillName) {
  return createHash("sha256").update(`${skillPath}\0${skillName}`).digest("hex").slice(0, 8);
}

const skills = JSON.parse(skillsArg);
if (!Array.isArray(skills)) {
  throw new Error("changed skills input must be a JSON array");
}

mkdirSync(outDir, { recursive: true });
const manifest = [];

for (const skillPath of skills) {
  const content = readFileSync(skillPath, "utf8");
  const meta = parseFrontmatter(content, skillPath);
  if (!meta.name || !meta.description) {
    throw new Error(`${skillPath}: frontmatter must include name and description`);
  }
  const title = titleFromSkill(content);
  const slug = meta.name.replace(/[^a-z0-9-]+/gi, "-").toLowerCase();
  const skillKey = shortSkillKey(skillPath, meta.name);
  const specPath = path.join(outDir, `${slug}.harness.json`);
  const spec = {
    name: `skill-self-test-${skillKey}-${safeRunLabel}-${slug}`.slice(0, 120),
    description: `Internal blind self-containment test for ${skillPath}`,
    task_prompt: taskPrompt(skillPath, meta, title),
    codex_template: "codex",
    auth_mode: "api_key_secret",
    openai_api_key_secret_name: openAISecretName,
    repository_url: repoUrl,
    evaluation_config: {
      validators: [
        {
          type: "command",
          command: validatorCommand(skillPath),
          required: true,
          timeout_seconds: 60,
        },
      ],
    },
  };
  if (baseBranch) {
    spec.base_branch = baseBranch;
  }
  if (codexModel) {
    spec.codex_model = codexModel;
  }
  writeFileSync(specPath, `${JSON.stringify(spec, null, 2)}\n`);
  manifest.push({ skill: skillPath, name: meta.name, spec: specPath });
}

const manifestPath = path.join(outDir, "manifest.json");
writeFileSync(manifestPath, `${JSON.stringify({ harnesses: manifest }, null, 2)}\n`);
console.log(manifestPath);
