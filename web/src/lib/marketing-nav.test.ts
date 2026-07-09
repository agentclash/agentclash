import { describe, expect, it } from "vitest";
import { DEFAULT_MARKETING_NAV } from "./marketing-nav";

describe("DEFAULT_MARKETING_NAV", () => {
  it("includes /compare for high-intent tool comparison discovery", () => {
    const compare = DEFAULT_MARKETING_NAV.find((item) => item.href === "/compare");
    expect(compare).toMatchObject({ label: "Compare" });
  });

  it("includes /pricing for conversion discovery", () => {
    const pricing = DEFAULT_MARKETING_NAV.find((item) => item.href === "/pricing");
    expect(pricing).toMatchObject({ label: "Pricing" });
  });

  it("keeps /benchmarks as the canonical benchmark hub", () => {
    const benchmarkLinks = DEFAULT_MARKETING_NAV.filter((item) =>
      item.href.includes("benchmark"),
    );
    expect(benchmarkLinks).toEqual([{ href: "/benchmarks", label: "Benchmarks" }]);
  });
});
