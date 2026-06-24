import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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
  let warnSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    // Parse/validation failures now warn instead of failing silently — keep the
    // output clean and assert on the warning where it matters.
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    warnSpy.mockRestore();
  });

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

  it("warns (instead of silently dropping) when a required field is missing", () => {
    const missingVerdict = VALID.replace(
      'verdict: "Opus 4.8 won 4 of 5."\n',
      "",
    );
    expect(parseBenchmarkReport("missing-verdict", missingVerdict)).toBeNull();
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining("verdict"));
  });

  it("returns null and warns when the date is non-empty but unparseable", () => {
    const badDate = VALID.replace('date: "2026-06-06"', 'date: "Q2 2026"');
    expect(parseBenchmarkReport("bad-date", badDate)).toBeNull();
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("date (unparseable"),
    );
  });

  it("drops a score outside the 0-1 range and warns, leaving the cell empty", () => {
    // A common authoring typo: `91` meant `0.91`. Must not render as "9100".
    const outOfRange = VALID.replace("composite: 0.91", "composite: 91");
    const report = parseBenchmarkReport("race-report", outOfRange);
    const opus = report?.results.find((r) => r.model === "Claude Opus 4.8");
    expect(opus?.composite).toBeNull();
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining("outside the"));
  });

  it("keeps costPerCorrectUsd above 1 (a dollar amount, not a 0-1 score)", () => {
    const pricey = VALID.replace(
      "costPerCorrectUsd: 0.14",
      "costPerCorrectUsd: 4.2",
    );
    const opus = parseBenchmarkReport("race-report", pricey)?.results.find(
      (r) => r.model === "Claude Opus 4.8",
    );
    expect(opus?.costPerCorrectUsd).toBe(4.2);
  });

  it("assigns unique sequential ranks when only some rows specify rank", () => {
    const partial = `---
title: "R"
date: "2026-06-06"
description: "d"
author: "a"
featuredModel: "m"
verdict: "v"
results:
  - model: "A"
    composite: 0.7
  - model: "B"
    rank: 1
    composite: 0.6
  - model: "C"
    composite: 0.8
---
body
`;
    const report = parseBenchmarkReport("partial-ranks", partial);
    const ranks = report?.results.map((r) => r.rank);
    expect(ranks).toEqual([1, 2, 3]);
    expect(new Set(ranks).size).toBe(3);
    // B (explicit rank 1) leads; A and C (unranked) follow by composite desc.
    expect(report?.results.map((r) => r.model)).toEqual(["B", "C", "A"]);
  });

  it("never emits duplicate ranks even when explicit ranks collide", () => {
    const dup = `---
title: "R"
date: "2026-06-06"
description: "d"
author: "a"
featuredModel: "m"
verdict: "v"
results:
  - model: "A"
    rank: 1
    composite: 0.5
  - model: "B"
    rank: 1
    composite: 0.9
---
body
`;
    const ranks = parseBenchmarkReport("dup-ranks", dup)?.results.map(
      (r) => r.rank,
    );
    expect(ranks).toEqual([1, 2]);
  });
});
