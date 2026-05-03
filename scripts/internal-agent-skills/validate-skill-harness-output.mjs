#!/usr/bin/env node
import { readFileSync } from "node:fs";

const [skillPath, outputPath] = process.argv.slice(2);
if (!skillPath || !outputPath) {
  console.error("usage: validate-skill-harness-output.mjs <skill-path> <result-json>");
  process.exit(2);
}

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

function asArray(value) {
  return Array.isArray(value) ? value : [];
}

function containsAny(values, patterns) {
  const haystack = values.join("\n").toLowerCase();
  return patterns.some((pattern) => haystack.includes(pattern));
}

const skillContent = readFileSync(skillPath, "utf8");
const meta = parseFrontmatter(skillContent, skillPath);
let result;
try {
  result = JSON.parse(readFileSync(outputPath, "utf8"));
} catch (error) {
  console.error(`result JSON is invalid: ${error.message}`);
  process.exit(1);
}

const errors = [];
if (result.skill_name !== meta.name) {
  errors.push(`skill_name = ${JSON.stringify(result.skill_name)}, want ${JSON.stringify(meta.name)}`);
}
if (result.used_only_skill_file !== true) {
  errors.push("used_only_skill_file must be true");
}
if (result.confidence !== "pass") {
  errors.push('confidence must be "pass"');
}

const requiredArrays = [
  "commands",
  "checks_before_mutation",
  "expected_outputs",
  "failure_modes",
  "blockers_or_confirmations",
];
for (const key of requiredArrays) {
  if (!Array.isArray(result[key]) || result[key].length === 0) {
    errors.push(`${key} must be a non-empty array`);
  }
}

if (typeof result.report_back !== "string" || result.report_back.trim() === "") {
  errors.push("report_back must be a non-empty string");
}

const requiresCLI = meta["metadata.agentclash.requires_cli"] === "true";
if (requiresCLI) {
  if (result.would_use_cli !== true) {
    errors.push("would_use_cli must be true for CLI-backed skills");
  }
  if (!containsAny(asArray(result.commands), ["agentclash"])) {
    errors.push("commands must include at least one agentclash command");
  }
  const hostedText = `${result.hosted_backend || ""}\n${asArray(result.commands).join("\n")}`;
  if (!hostedText.includes("https://api.agentclash.dev")) {
    errors.push("CLI-backed skills must preserve hosted production API setup");
  }
}

const forbiddenSourceClaims = [
  "i inspected the source",
  "i read the codebase",
  "from the repository source",
  "cli/cmd/",
  "backend/internal/",
  "web/src/",
];
const combined = JSON.stringify(result).toLowerCase();
for (const claim of forbiddenSourceClaims) {
  if (combined.includes(claim)) {
    errors.push(`result appears to rely on forbidden repo context: ${claim}`);
  }
}

if (errors.length > 0) {
  console.error("skill harness output failed validation:");
  for (const error of errors) {
    console.error(`- ${error}`);
  }
  process.exit(1);
}

console.log(`skill harness output passed for ${meta.name}`);
