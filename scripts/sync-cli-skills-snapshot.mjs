#!/usr/bin/env node
// Sync the canonical Agent Skills (web/content/agent-skills) into a flattened,
// embeddable snapshot under cli/internal/skills/snapshot so the Go CLI can
// //go:embed them for `agentclash integration <agent> install`.
//
// web/content is the single source of truth; this snapshot is generated, never
// hand-edited. Run `make cli-skills-snapshot` after changing any skill; CI
// should fail if `git diff --exit-code cli/internal/skills/snapshot` is dirty.
import { createHash } from "node:crypto";
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
const DEST = path.join(repoRoot, "cli", "internal", "skills", "snapshot");
// The catalog/hub at the root of agent-skills is a docs-navigation index that
// cross-references the others by docs URL — not a self-contained installable
// skill — so it is excluded from the embedded snapshot.
const ROOT_CATALOG = path.join(SRC, "SKILL.md");

// parseFrontmatter mirrors scripts/internal-agent-skills/prepare-skill-harnesses.mjs:
// returns a flat map with `name`, `description`, and dotted `metadata.<key>` entries.
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

rmSync(DEST, { recursive: true, force: true });
mkdirSync(DEST, { recursive: true });

const skills = [];
const seen = new Set();

for (const file of files) {
  if (path.resolve(file) === path.resolve(ROOT_CATALOG)) continue;
  const content = readFileSync(file, "utf8");
  const meta = parseFrontmatter(content, file);
  if (!meta.name || !meta.description) {
    throw new Error(`${file}: frontmatter must include name and description`);
  }
  if (seen.has(meta.name)) {
    throw new Error(`duplicate skill name "${meta.name}" (flattening requires unique names)`);
  }
  seen.add(meta.name);

  const skillDir = path.join(DEST, meta.name);
  mkdirSync(skillDir, { recursive: true });
  writeFileSync(path.join(skillDir, "SKILL.md"), content);

  skills.push({
    name: meta.name,
    role: meta["metadata.agentclash.role"] || "",
    requires_cli: meta["metadata.agentclash.requires_cli"] === "true",
    sha256: createHash("sha256").update(content).digest("hex"),
  });
}

skills.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));

// Content-addressed version (worktree-stable; not a git sha) so `doctor` can
// report version drift without false positives across checkouts.
const versionHash = createHash("sha256");
for (const s of skills) versionHash.update(`${s.name}:${s.sha256}\n`);
const snapshotVersion = versionHash.digest("hex").slice(0, 12);

writeFileSync(
  path.join(DEST, "manifest.json"),
  `${JSON.stringify({ snapshot_version: snapshotVersion, skills }, null, 2)}\n`,
);

console.log(
  `Wrote ${skills.length} skills + manifest (v${snapshotVersion}) to ${path.relative(repoRoot, DEST)}`,
);
