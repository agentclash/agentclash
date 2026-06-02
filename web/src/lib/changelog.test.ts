import { describe, expect, it } from "vitest";
import {
  CHANGELOG_FAQ,
  getChangelogLatestModified,
  getChangelogPeriods,
  renderChangelogMarkdown,
} from "./changelog";

describe("getChangelogPeriods", () => {
  it("returns periods sorted newest first", () => {
    const periods = getChangelogPeriods();

    expect(periods.length).toBeGreaterThan(0);
    expect(periods[0]?.id).toBe("2026-06-02");
    expect(periods.at(-1)?.id).toBe("2026-04-15");
    expect(
      periods.every(
        (period, index) =>
          index === 0 ||
          period.startDate <= periods[index - 1]!.startDate,
      ),
    ).toBe(true);
  });
});

describe("getChangelogLatestModified", () => {
  it("returns the newest period end date", () => {
    expect(getChangelogLatestModified()).toBe("2026-06-11");
  });
});

describe("renderChangelogMarkdown", () => {
  it("exports all periods with source link and category labels", () => {
    const markdown = renderChangelogMarkdown("https://example.test");

    expect(markdown).toContain("# AgentClash Changelog");
    expect(markdown).toContain("Source: https://example.test/changelog");
    expect(markdown).toContain("## Jun 02 – Jun 11, 2026");
    expect(markdown).toContain("**Added**: 25 portable Agent Skills");
    expect(markdown).toContain("**Added**: Datasets foundation");
    expect(markdown).toContain("**Security**: SecurityPolicy schema");
  });
});

describe("CHANGELOG_FAQ", () => {
  it("includes discoverability questions for answer engines", () => {
    expect(CHANGELOG_FAQ.length).toBeGreaterThanOrEqual(3);
    expect(CHANGELOG_FAQ[0]?.question).toContain("changelog");
  });
});
