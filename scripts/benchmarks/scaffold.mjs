#!/usr/bin/env node
// Scaffold a benchmark report MDX from an AgentClash run-ranking JSON.
//
// The ranking JSON is what `GET /workspaces/{ws}/runs/{run}/ranking` returns
// (see backend/internal/api/run_ranking.go) — either the full response
// (`{ state, ranking: {...} }`) or just the `ranking` payload. Each item maps to
// one scoreboard row; the winner is the item whose run_agent_id matches
// ranking.winner.run_agent_id.
//
// Usage:
//   node scripts/benchmarks/scaffold.mjs --ranking ranking.json \
//     --title "We raced X against the field" --model "X" --slug x-vs-the-field \
//     [--share-url https://www.agentclash.dev/share/TOKEN] [--out DIR] [--force]
//
//   # or pipe the JSON in:
//   agentclash run ranking <run-id> --json | node scripts/benchmarks/scaffold.mjs \
//     --title "..." --model "..." --slug "..."
//
// Output: an MDX file with `sample: false` and a prose skeleton to edit. Refuses
// to overwrite an existing file unless --force is passed. No external deps.

import fs from "node:fs";
import path from "node:path";
import process from "node:process";

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith("--")) continue;
    const key = token.slice(2);
    if (key === "force") {
      args.force = true;
      continue;
    }
    const value = argv[i + 1];
    if (value === undefined || value.startsWith("--")) {
      args[key] = true;
    } else {
      args[key] = value;
      i += 1;
    }
  }
  return args;
}

function fail(message) {
  process.stderr.write(`scaffold: ${message}\n`);
  process.exit(1);
}

function readInput(rankingPath) {
  if (rankingPath && rankingPath !== true) {
    return fs.readFileSync(rankingPath, "utf-8");
  }
  if (process.stdin.isTTY) {
    fail("provide --ranking <file> or pipe ranking JSON on stdin");
  }
  return fs.readFileSync(0, "utf-8");
}

// Accept either the full ranking response or the bare ranking payload.
function extractRanking(parsed) {
  if (parsed && parsed.ranking && Array.isArray(parsed.ranking.items)) {
    return parsed.ranking;
  }
  if (parsed && Array.isArray(parsed.items)) {
    return parsed;
  }
  fail("could not find ranking.items[] in the JSON");
  return null;
}

function num(value) {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function round(value, places) {
  if (value === null) return null;
  return Number(value.toFixed(places));
}

// Provider is not in the ranking document; infer a hint from the model label so
// the editor only has to confirm rather than fill from scratch.
function guessProvider(label) {
  const lower = label.toLowerCase();
  if (lower.includes("claude") || lower.includes("opus") || lower.includes("sonnet") || lower.includes("haiku")) return "Anthropic";
  if (lower.includes("gpt") || lower.includes("o1") || lower.includes("o3") || lower.includes("openai")) return "OpenAI";
  if (lower.includes("gemini")) return "Google";
  if (lower.includes("grok")) return "xAI";
  if (lower.includes("mistral") || lower.includes("mixtral")) return "Mistral";
  if (lower.includes("llama")) return "Meta";
  return "";
}

// Minimal YAML scalar quoting — wrap in double quotes and escape backslashes and
// quotes. The frontmatter schema is flat, so this is sufficient.
function yamlString(value) {
  return `"${String(value).replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

function buildResultsYaml(items, winnerId) {
  const lines = ["results:"];
  for (const [index, item] of items.entries()) {
    const label = item.label || `Lane ${item.lane_index ?? index}`;
    const rank = num(item.rank) ?? index + 1;
    const isWinner = winnerId && item.run_agent_id === winnerId;
    lines.push(`  - model: ${yamlString(label)}`);
    lines.push(`    provider: ${yamlString(guessProvider(label))}`);
    lines.push(`    rank: ${rank}`);
    if (isWinner) lines.push("    winner: true");
    const fields = [
      ["composite", round(num(item.composite_score), 2)],
      ["correctness", round(num(item.correctness_score), 2)],
      ["reliability", round(num(item.reliability_score), 2)],
      ["latency", round(num(item.latency_score), 2)],
      ["cost", round(num(item.cost_score), 2)],
      ["costPerCorrectUsd", round(num(item.cost_per_correct_usd), 4)],
    ];
    for (const [key, value] of fields) {
      if (value !== null) lines.push(`    ${key}: ${value}`);
    }
  }
  return lines.join("\n");
}

function today() {
  return new Date().toISOString().slice(0, 10);
}

function buildMdx({ title, model, shareUrl, results }) {
  const frontmatter = [
    "---",
    `title: ${yamlString(title)}`,
    `date: ${yamlString(today())}`,
    `description: ${yamlString(`A head-to-head AgentClash race of ${model} against the field on real agentic tasks, scored on correctness, reliability, latency, and cost.`)}`,
    `author: ${yamlString("AgentClash")}`,
    `featuredModel: ${yamlString(model)}`,
    `verdict: ${yamlString("TODO: one-line verdict — who won and the key trade-off.")}`,
    `challengePack: ${yamlString("TODO: challenge pack name")}`,
    "sample: false",
    ...(shareUrl ? [`runShareUrl: ${yamlString(shareUrl)}`] : []),
    "tasks:",
    "  - id: task-1",
    '    name: "TODO: task name"',
    '    summary: "TODO: what the agent had to do and how it was verified."',
    results,
    "---",
  ].join("\n");

  const body = [
    "",
    "## How the race worked",
    "",
    "TODO: same tasks, same sandbox, same tools. Describe the lineup and the pack.",
    "",
    "## The tasks",
    "",
    "TODO: why these five tasks, and what counted as success on each.",
    "",
    "## What we saw",
    "",
    "TODO: the story behind the scoreboard — where the featured model won, where",
    "the field stayed close, and any surprises.",
    "",
    "## Takeaway",
    "",
    "TODO: the trade-off a single number hides. Close with a CTA.",
    "",
    "[Run your own race →](/try)",
    "",
  ].join("\n");

  return `${frontmatter}\n${body}`;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const title = args.title;
  const model = args.model;
  const slug = args.slug;
  if (!title || title === true) fail("--title is required");
  if (!model || model === true) fail("--model is required");
  if (!slug || slug === true) fail("--slug is required");
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) {
    fail("--slug must be kebab-case (lowercase letters, digits, hyphens)");
  }

  const raw = readInput(args.ranking);
  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch (err) {
    fail(`could not parse ranking JSON: ${err.message}`);
  }
  const ranking = extractRanking(parsed);
  const winnerId = ranking.winner && ranking.winner.run_agent_id;
  const resultsYaml = buildResultsYaml(ranking.items, winnerId);

  const outDir =
    args.out && args.out !== true
      ? args.out
      : path.join("web", "content", "benchmarks");
  const outPath = path.join(outDir, `${slug}.mdx`);
  if (fs.existsSync(outPath) && !args.force) {
    fail(`${outPath} already exists (pass --force to overwrite)`);
  }

  const mdx = buildMdx({
    title,
    model,
    shareUrl: args["share-url"] && args["share-url"] !== true ? args["share-url"] : "",
    results: resultsYaml,
  });

  fs.mkdirSync(outDir, { recursive: true });
  fs.writeFileSync(outPath, mdx, "utf-8");
  process.stdout.write(`Wrote ${outPath}\n`);
  process.stdout.write(
    "Next: fill the TODO fields, verify the scoreboard, then publish.\n",
  );
}

main();
