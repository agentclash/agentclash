#!/usr/bin/env node
// Assemble npm packages from GoReleaser dist/ for an AgentClash release tag.
//
// Inputs:
//   argv[2]  release tag (e.g. "v0.3.0")                    [required]
//   argv[3]  path to the directory GoReleaser wrote         [default: ./dist]
//
// Output:
//   ./npm-out/cli/                          → publishes as `agentclash`
//   ./npm-out/platforms/<triple>/           → @agentclash/cli-<triple>
//
// Fails hard if any of the six platform archives is missing, fails the
// SHA-256 check, or is missing its expected binary entry. A partial
// assembly must never result in an npm publish of only some triples.
"use strict";

import { createHash } from "node:crypto";
import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync, copyFileSync, readdirSync, chmodSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const repoRoot = resolve(__dirname, "..", "..");

// GoReleaser archive → npm triple mapping. `os` / `cpu` match Node's
// `process.platform` / `process.arch` so the shim's require.resolve works.
const TRIPLES = [
  { go: "linux_amd64",   triple: "linux-x64",    os: "linux",  cpu: "x64",   ext: "",     archive: "tar.gz" },
  { go: "linux_arm64",   triple: "linux-arm64",  os: "linux",  cpu: "arm64", ext: "",     archive: "tar.gz" },
  { go: "darwin_amd64",  triple: "darwin-x64",   os: "darwin", cpu: "x64",   ext: "",     archive: "tar.gz" },
  { go: "darwin_arm64",  triple: "darwin-arm64", os: "darwin", cpu: "arm64", ext: "",     archive: "tar.gz" },
  { go: "windows_amd64", triple: "win32-x64",    os: "win32",  cpu: "x64",   ext: ".exe", archive: "zip"    },
  { go: "windows_arm64", triple: "win32-arm64",  os: "win32",  cpu: "arm64", ext: ".exe", archive: "zip"    },
];

function die(msg) {
  console.error(`assemble: ${msg}`);
  process.exit(1);
}

function sha256(path) {
  const h = createHash("sha256");
  h.update(readFileSync(path));
  return h.digest("hex");
}

function loadExpectedChecksums(checksumsPath) {
  const map = new Map();
  for (const line of readFileSync(checksumsPath, "utf8").split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const parts = trimmed.split(/\s+/);
    if (parts.length < 2) continue;
    map.set(parts[1], parts[0].toLowerCase());
  }
  return map;
}

function extract(archivePath, destDir) {
  mkdirSync(destDir, { recursive: true });
  if (archivePath.endsWith(".tar.gz")) {
    execFileSync("tar", ["-xzf", archivePath, "-C", destDir], { stdio: "inherit" });
  } else if (archivePath.endsWith(".zip")) {
    execFileSync("unzip", ["-q", "-o", archivePath, "-d", destDir], { stdio: "inherit" });
  } else {
    die(`unsupported archive format: ${archivePath}`);
  }
}

function render(template, replacements) {
  let out = template;
  for (const [key, value] of Object.entries(replacements)) {
    out = out.replaceAll(key, value);
  }
  return out;
}

function writeJSON(path, obj) {
  writeFileSync(path, JSON.stringify(obj, null, 2) + "\n");
}

function main() {
  const tag = process.argv[2];
  if (!tag) die("usage: assemble.mjs <tag> [distDir]");
  if (!/^v\d/.test(tag)) die(`tag must start with 'v' and a digit, got: ${tag}`);
  const version = tag.replace(/^v/, "");

  const distDir = resolve(process.argv[3] ?? join(repoRoot, "dist"));
  if (!existsSync(distDir)) die(`dist dir does not exist: ${distDir}`);

  const checksumsPath = join(distDir, "checksums.txt");
  if (!existsSync(checksumsPath)) die(`missing ${checksumsPath}`);
  const expected = loadExpectedChecksums(checksumsPath);

  const outRoot = join(repoRoot, "npm-out");
  rmSync(outRoot, { recursive: true, force: true });
  mkdirSync(outRoot, { recursive: true });

  const rootLicense = join(repoRoot, "LICENSE");
  if (!existsSync(rootLicense)) die("repo root LICENSE is missing; Part D of the plan must land first");

  const cliTemplatePkg = JSON.parse(readFileSync(join(repoRoot, "npm", "cli", "package.json"), "utf8"));
  const platformTemplatePkg = readFileSync(join(repoRoot, "npm", "platforms", "template", "package.json"), "utf8");
  const platformTemplateReadme = readFileSync(join(repoRoot, "npm", "platforms", "template", "README.md"), "utf8");

  // ---- Platform packages ------------------------------------------------
  for (const t of TRIPLES) {
    const archiveName = `agentclash_${t.go}.${t.archive}`;
    let archivePath = join(distDir, archiveName);
    if (!existsSync(archivePath)) {
      // Some GoReleaser setups drop archives into subdirectories; look one
      // level deep before giving up, and then actually use that path for the
      // checksum + extract steps below.
      const nested = readdirSync(distDir, { withFileTypes: true })
        .filter((e) => e.isDirectory())
        .map((e) => join(distDir, e.name, archiveName))
        .find((p) => existsSync(p));
      if (!nested) die(`missing archive for ${t.triple}: ${archivePath}`);
      archivePath = nested;
    }
    const expectedHash = expected.get(archiveName);
    if (!expectedHash) die(`checksums.txt does not list ${archiveName}`);
    const actualHash = sha256(archivePath);
    if (actualHash !== expectedHash) {
      die(`checksum mismatch for ${archiveName}: got ${actualHash}, want ${expectedHash}`);
    }

    const stageDir = join(outRoot, "stage", t.triple);
    extract(archivePath, stageDir);

    const binaryName = `agentclash${t.ext}`;
    let binarySrc = join(stageDir, binaryName);
    if (!existsSync(binarySrc)) {
      // GoReleaser often nests the binary in a subdir named after the archive.
      const nested = readdirSync(stageDir, { withFileTypes: true })
        .filter((e) => e.isDirectory())
        .map((e) => join(stageDir, e.name, binaryName))
        .find((p) => existsSync(p));
      if (!nested) die(`archive ${archiveName} did not contain ${binaryName}`);
      binarySrc = nested;
    }

    const pkgDir = join(outRoot, "platforms", t.triple);
    const binDir = join(pkgDir, "bin");
    mkdirSync(binDir, { recursive: true });
    copyFileSync(binarySrc, join(binDir, binaryName));
    if (t.ext === "") {
      chmodSync(join(binDir, binaryName), 0o755);
    }

    const rendered = render(platformTemplatePkg, {
      "__TRIPLE__": t.triple,
      "__OS__":     t.os,
      "__CPU__":    t.cpu,
      "__EXT__":    t.ext,
    });
    const pkg = JSON.parse(rendered);
    pkg.version = version;
    writeJSON(join(pkgDir, "package.json"), pkg);

    const readme = render(platformTemplateReadme, {
      "__TRIPLE__": t.triple,
      "__OS__":     t.os,
      "__CPU__":    t.cpu,
    });
    writeFileSync(join(pkgDir, "README.md"), readme);
    copyFileSync(rootLicense, join(pkgDir, "LICENSE"));

    console.log(`assembled ${pkg.name}@${version}`);
  }

  // ---- Root wrapper package --------------------------------------------
  const cliOutDir = join(outRoot, "cli");
  mkdirSync(join(cliOutDir, "bin"), { recursive: true });
  copyFileSync(
    join(repoRoot, "npm", "cli", "bin", "agentclash.js"),
    join(cliOutDir, "bin", "agentclash.js"),
  );
  copyFileSync(join(repoRoot, "npm", "cli", "README.md"), join(cliOutDir, "README.md"));
  copyFileSync(rootLicense, join(cliOutDir, "LICENSE"));

  const rootPkg = structuredClone(cliTemplatePkg);
  rootPkg.version = version;
  rootPkg.optionalDependencies = Object.fromEntries(
    TRIPLES.map((t) => [`@agentclash/cli-${t.triple}`, version]),
  );
  writeJSON(join(cliOutDir, "package.json"), rootPkg);
  console.log(`assembled ${rootPkg.name}@${version}`);

  // ---- Clean staging ---------------------------------------------------
  rmSync(join(outRoot, "stage"), { recursive: true, force: true });

  console.log(`\nnpm packages written to ${outRoot}`);
  console.log(`Publish order: platforms/* first, cli last.`);
}

try {
  main();
} catch (err) {
  console.error(err);
  process.exit(1);
}

// Silence unused-import lint warnings for tooling that imports spawnSync
// but this script uses execFileSync; keep spawnSync available for callers
// that want to shell out during local rehearsal.
void spawnSync;
