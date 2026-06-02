#!/usr/bin/env node
// Build a `gh skill`-publishable bundle from the canonical Agent Skills
// (web/content/agent-skills) into dist/skills-bundle/skills/<name>/SKILL.md.
//
// web/content stays the single source of truth; this is a generated publish
// artifact (gitignored). The layout matches the agentskills.io discovery glob
// `skills/*/SKILL.md`, so a maintainer can:
//
//   node scripts/build-skills-bundle.mjs
//   gh skill publish dist/skills-bundle            # needs gh >= 2.90.0
//
// The script SELF-VALIDATES every skill against the spec rules `gh skill
// publish` enforces (kebab-case name <= 64 chars, description present <= 1024,
// folder name == frontmatter name, no duplicates) and exits non-zero on any
// violation — so a green run means the bundle is publish-ready even without gh.
import {
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "..");
const SRC = path.join(repoRoot, "web", "content", "agent-skills");
const OUT = path.join(repoRoot, "dist", "skills-bundle");
const SKILLS_DIR = path.join(OUT, "skills");
// The catalog/hub at the root of agent-skills is a docs-navigation index, not a
// standalone installable skill, and a root-level SKILL.md trips gh skill's
// name="." rejection (cli/cli#13552). It is excluded from the published bundle.
const ROOT_CATALOG = path.join(SRC, "SKILL.md");

const NAME_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/;

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

function walk(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walk(full));
    else if (entry.name === "SKILL.md") out.push(full);
  }
  return out;
}

const files = walk(SRC).sort();

rmSync(OUT, { recursive: true, force: true });
mkdirSync(SKILLS_DIR, { recursive: true });

const seen = new Set();
let count = 0;

for (const file of files) {
  if (path.resolve(file) === path.resolve(ROOT_CATALOG)) continue;
  const content = readFileSync(file, "utf8");
  const meta = parseFrontmatter(content, file);

  // Spec validation (mirrors what `gh skill publish` enforces).
  if (!meta.name) throw new Error(`${file}: frontmatter missing 'name'`);
  if (!NAME_RE.test(meta.name)) {
    throw new Error(`${file}: name "${meta.name}" must be kebab-case [a-z0-9-]`);
  }
  if (meta.name.length > 64) {
    throw new Error(`${file}: name "${meta.name}" exceeds 64 chars`);
  }
  if (!meta.description) throw new Error(`${file}: frontmatter missing 'description'`);
  if (meta.description.length > 1024) {
    throw new Error(`${file}: description exceeds 1024 chars`);
  }
  if (seen.has(meta.name)) {
    throw new Error(`duplicate skill name "${meta.name}"`);
  }
  seen.add(meta.name);

  // Folder name MUST equal the frontmatter name (a publish requirement).
  const skillDir = path.join(SKILLS_DIR, meta.name);
  mkdirSync(skillDir, { recursive: true });
  writeFileSync(path.join(skillDir, "SKILL.md"), content);
  count++;
}

console.log(
  `Built ${count} publish-ready skills in ${path.relative(repoRoot, SKILLS_DIR)}\n` +
    `Validation passed (kebab name <=64, description present <=1024, name==dirname, unique).\n` +
    `Publish with: gh skill publish ${path.relative(repoRoot, OUT)}   (requires gh >= 2.90.0)`,
);
