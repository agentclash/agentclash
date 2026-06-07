import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import type { BenchmarkResult } from "@/lib/benchmarks";
import { BenchmarkScoreboard } from "./benchmark-scoreboard";

const EM_DASH = "—";

function row(overrides: Partial<BenchmarkResult>): BenchmarkResult {
  return {
    model: "Model",
    provider: "Provider",
    rank: 1,
    winner: false,
    composite: 0.5,
    correctness: 0.5,
    reliability: 0.5,
    latency: 0.5,
    cost: 0.5,
    costPerCorrectUsd: 0.5,
    ...overrides,
  };
}

describe("BenchmarkScoreboard", () => {
  it("preserves a sub-cent $/correct instead of flattening to $0.00", () => {
    const html = renderToStaticMarkup(
      <BenchmarkScoreboard
        results={[row({ model: "Frugal", costPerCorrectUsd: 0.0042 })]}
      />,
    );
    // Cell renders the full sub-cent value; the old toFixed(2) path would have
    // produced exactly "$0.00" in the cell ("$0.0042" trivially contains the
    // substring "$0.00", so assert on the whole cell, not a substring).
    expect(html).toContain(">$0.0042</td>");
    expect(html).not.toContain(">$0.00</td>");
  });

  it("renders a missing score as an em dash, never NaN", () => {
    const html = renderToStaticMarkup(
      <BenchmarkScoreboard results={[row({ composite: null })]} />,
    );
    expect(html).not.toContain("NaN");
    expect(html).toContain(EM_DASH);
  });

  it("renders the distinct rank each row carries", () => {
    const html = renderToStaticMarkup(
      <BenchmarkScoreboard
        results={[
          row({ model: "Alpha", rank: 1 }),
          row({ model: "Bravo", rank: 2 }),
        ]}
      />,
    );
    // Rank cells render the raw number; both must appear, distinct.
    expect(html).toContain(">1</td>");
    expect(html).toContain(">2</td>");
  });
});
