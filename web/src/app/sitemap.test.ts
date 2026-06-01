import { beforeEach, describe, expect, it, vi } from "vitest";
import { getChangelogLatestModified, getChangelogPeriods } from "@/lib/changelog";
import sitemap from "./sitemap";

vi.mock("@/lib/blog", () => ({
  getAllPosts: vi.fn(() => [
    {
      slug: "ai-agent-evaluation-regression-testing",
      date: "2026-05-07",
    },
  ]),
}));

vi.mock("@/lib/docs", () => ({
  DOCS_ORIGIN: "https://www.agentclash.dev",
  getAllDocPaths: vi.fn(() => [
    "/docs",
    "/docs/getting-started/quickstart",
  ]),
}));

describe("sitemap", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps core public discovery surfaces indexed", () => {
    const entries = sitemap();
    const byUrl = new Map(entries.map((entry) => [entry.url, entry]));

    expect(byUrl.get("https://www.agentclash.dev")).toMatchObject({
      changeFrequency: "weekly",
      priority: 1,
    });
    expect(byUrl.get("https://www.agentclash.dev/blog")).toMatchObject({
      changeFrequency: "weekly",
      priority: 0.8,
    });
    expect(byUrl.get("https://www.agentclash.dev/changelog")).toMatchObject({
      changeFrequency: "weekly",
      priority: 0.75,
      lastModified: new Date(getChangelogLatestModified()),
    });
    for (const period of getChangelogPeriods()) {
      expect(
        byUrl.get(`https://www.agentclash.dev/changelog/${period.id}`),
      ).toMatchObject({
        changeFrequency: "monthly",
        priority: 0.65,
        lastModified: new Date(period.endDate),
      });
    }
    expect(byUrl.get("https://www.agentclash.dev/why")).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.7,
    });
    expect(byUrl.get("https://www.agentclash.dev/team")).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.5,
    });
    expect(byUrl.get("https://www.agentclash.dev/sitemap")).toMatchObject({
      changeFrequency: "weekly",
      priority: 0.5,
    });
    expect(
      byUrl.get("https://www.agentclash.dev/platform/agent-evaluation"),
    ).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.85,
    });
    expect(
      byUrl.get(
        "https://www.agentclash.dev/platform/agent-regression-testing",
      ),
    ).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.82,
    });
    expect(byUrl.get("https://www.agentclash.dev/llms.txt")).toMatchObject({
      changeFrequency: "weekly",
      priority: 0.6,
    });
    expect(
      byUrl.get("https://www.agentclash.dev/llms-full.txt"),
    ).toMatchObject({
      changeFrequency: "weekly",
      priority: 0.55,
    });
  });

  it("includes docs pages and blog posts from source readers", () => {
    const entries = sitemap();
    const byUrl = new Map(entries.map((entry) => [entry.url, entry]));

    expect(byUrl.get("https://www.agentclash.dev/docs")).toMatchObject({
      changeFrequency: "weekly",
      priority: 0.85,
    });
    expect(
      byUrl.get("https://www.agentclash.dev/docs/getting-started/quickstart"),
    ).toMatchObject({
      changeFrequency: "weekly",
      priority: 0.75,
    });
    expect(
      byUrl.get(
        "https://www.agentclash.dev/blog/ai-agent-evaluation-regression-testing",
      ),
    ).toMatchObject({
      lastModified: new Date("2026-05-07"),
      changeFrequency: "monthly",
      priority: 0.7,
    });
  });

  it("includes the comparison hub and per-competitor pages", () => {
    const entries = sitemap();
    const urls = new Set(entries.map((entry) => entry.url));

    expect(urls.has("https://www.agentclash.dev/compare")).toBe(true);
    expect(
      urls.has("https://www.agentclash.dev/compare/agentclash-vs-langsmith"),
    ).toBe(true);
  });
});
