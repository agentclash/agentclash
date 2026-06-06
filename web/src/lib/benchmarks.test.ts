import { describe, expect, it } from "vitest";
import { parseBenchmarkReport } from "./benchmarks";

const VALID = `---
title: "Race report"
date: "2026-06-06"
description: "A head-to-head race."
author: "AgentClash"
featuredModel: "Claude Opus 4.8"
verdict: "Opus 4.8 won 4 of 5."
challengePack: "Real-World Agentic Tasks v1"
sample: true
runShareUrl: "https://www.agentclash.dev/share/abc"
tasks:
  - id: auth-bug
    name: "Fix the auth bug"
    summary: "Patch the JWT refresh flow."
results:
  - model: "GPT-5.1"
    provider: "OpenAI"
    rank: 2
    composite: 0.86
    correctness: "0.90"
  - model: "Claude Opus 4.8"
    provider: "Anthropic"
    rank: 1
    winner: true
    composite: 0.91
    costPerCorrectUsd: 0.14
---

Body content.
`;

describe("parseBenchmarkReport", () => {
  it("parses valid frontmatter and body", () => {
    const report = parseBenchmarkReport("race-report", VALID);
    expect(report).not.toBeNull();
    expect(report?.title).toBe("Race report");
    expect(report?.featuredModel).toBe("Claude Opus 4.8");
    expect(report?.sample).toBe(true);
    expect(report?.runShareUrl).toBe("https://www.agentclash.dev/share/abc");
    expect(report?.tasks).toEqual([
      {
        id: "auth-bug",
        name: "Fix the auth bug",
        summary: "Patch the JWT refresh flow.",
      },
    ]);
    expect(report?.content.trim()).toBe("Body content.");
  });

  it("sorts results by rank ascending", () => {
    const report = parseBenchmarkReport("race-report", VALID);
    expect(report?.results.map((r) => r.model)).toEqual([
      "Claude Opus 4.8",
      "GPT-5.1",
    ]);
    expect(report?.results[0].winner).toBe(true);
    expect(report?.results[1].winner).toBe(false);
  });

  it("coerces numeric scores and defaults missing ones to null", () => {
    const report = parseBenchmarkReport("race-report", VALID);
    const gpt = report?.results.find((r) => r.model === "GPT-5.1");
    expect(gpt?.correctness).toBe(0.9); // coerced from string
    expect(gpt?.reliability).toBeNull(); // absent
    const opus = report?.results.find((r) => r.model === "Claude Opus 4.8");
    expect(opus?.costPerCorrectUsd).toBe(0.14);
  });

  it("returns null when a required field is missing", () => {
    const missingModel = VALID.replace(
      'featuredModel: "Claude Opus 4.8"\n',
      "",
    );
    expect(parseBenchmarkReport("x", missingModel)).toBeNull();
  });

  it("returns null when the results array is empty", () => {
    const noResults = `---
title: "Race report"
date: "2026-06-06"
description: "A head-to-head race."
author: "AgentClash"
featuredModel: "Claude Opus 4.8"
verdict: "Nobody raced."
results: []
---
Body.
`;
    expect(parseBenchmarkReport("x", noResults)).toBeNull();
  });

  it("returns null for unparseable input", () => {
    expect(parseBenchmarkReport("x", "---\n: : :\n---")).toBeNull();
  });
});
