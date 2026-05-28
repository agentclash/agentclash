#!/usr/bin/env bun
import { writeFileSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import {
  DEFAULT_CONFIG_TEMPLATE,
  findConfigFile,
  loadConfigFile,
} from "@try-cli/core";

const cwd = process.cwd();
const args = process.argv.slice(2);
const command = args[0];

function usage() {
  console.log(`
try-cli — Interactive README demos (AgentClash primitive)

Usage:
  npx @agentclash/try-cli init          Create .trycli.yml
  npx @agentclash/try-cli validate      Validate config
  npx @agentclash/try-cli publish       Print README badge

Docs: https://www.agentclash.dev/docs/concepts/try-cli
Try live: https://www.agentclash.dev/try
`);
}

function init() {
  const target = join(cwd, ".trycli.yml");
  try {
    readFileSync(target);
    console.error("Error: .trycli.yml already exists");
    process.exit(1);
  } catch {
    writeFileSync(target, DEFAULT_CONFIG_TEMPLATE, "utf-8");
    console.log("Created .trycli.yml");
    console.log("\nNext steps:");
    console.log("  1. Edit .trycli.yml with your install steps and suggested commands");
    console.log("  2. Run: npx try-cli publish");
  }
}

function validate() {
  const file = findConfigFile(cwd);
  if (!file) {
    console.error("Error: no .trycli.yml found. Run: npx try-cli init");
    process.exit(1);
  }
  const demo = loadConfigFile(file);
  console.log(`✓ Valid config for "${demo.name}" (slug: ${demo.slug})`);
  console.log(`  ${demo.commands.length} suggested command(s)`);
  console.log(`  Session: ${demo.sessionMinutes} min`);
}

function publish() {
  const file = findConfigFile(cwd);
  if (!file) {
    console.error("Error: no .trycli.yml found. Run: npx try-cli init");
    process.exit(1);
  }
  const demo = loadConfigFile(file);
  const baseUrl = process.env.TRY_CLI_BASE_URL ?? "https://www.agentclash.dev/try";
  const badge = `[![Try on AgentClash](${baseUrl.replace(/\/try$/, "")}/api/try/badge/${demo.slug}.svg)](${baseUrl}/${demo.slug})`;

  console.log(`\nAdd this to your README:\n`);
  console.log(badge);
  console.log(`\nDemo URL: ${baseUrl}/${demo.slug}`);
  console.log(`\nTip: commit .trycli.yml to your repo so others can fork the config.`);
}

async function main() {
  switch (command) {
    case "init":
      init();
      break;
    case "validate":
      validate();
      break;
    case "publish":
    case "badge":
      publish();
      break;
    case undefined:
    case "--help":
    case "-h":
    case "help":
      usage();
      break;
    default:
      console.error(`Unknown command: ${command}`);
      usage();
      process.exit(1);
  }
}

main();
