#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";

const base = process.env.BASE_SHA || process.argv[2] || "";
const head = process.env.HEAD_SHA || process.argv[3] || "HEAD";
const outFile = process.env.GITHUB_OUTPUT || "";

function git(args) {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

function changedFiles() {
  if (base) {
    return git(["diff", "--name-only", `${base}...${head}`]);
  }
  return git(["diff", "--name-only", "HEAD~1", head]);
}

const files = changedFiles()
  .split("\n")
  .map((file) => file.trim())
  .filter(Boolean)
  .filter((file) => /^web\/content\/agent-skills\/.*\/SKILL\.md$/.test(file))
  .filter((file) => existsSync(path.resolve(file)));

const payload = JSON.stringify(files);
console.log(payload);

if (outFile) {
  const fs = await import("node:fs");
  fs.appendFileSync(outFile, `skills=${payload}\n`);
  fs.appendFileSync(outFile, `count=${files.length}\n`);
}
