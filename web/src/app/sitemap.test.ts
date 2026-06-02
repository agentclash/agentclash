import { beforeEach, describe, expect, it, vi } from "vitest";
import { getChangelogLatestModified, getChangelogPeriods } from "@/lib/changelog";
import sitemap from "./sitemap";

vi.mock("@/lib/blog", () => ({
  getAllPosts: vi.fn(() => [
    {
      slug: "ai-agent-evaluation-regression-testing",
      title: "AI agent evaluation and regression testing",
      date: "2026-05-07",
      description: "How AgentClash turns failed agent runs into regression tests.",
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

  it("attaches an absolute /og image to the compare hub entry", () => {
    const entries = sitemap();
    const byUrl = new Map(entries.map((entry) => [entry.url, entry]));

    const compare = byUrl.get("https://www.agentclash.dev/compare");
    expect(compare?.images).toHaveLength(1);
    const image = compare?.images?.[0] ?? "";
    // Must be ABSOLUTE (DOCS_ORIGIN prefix present) — metadataBase does not
    // rewrite sitemap image strings, and Google rejects a relative <image:loc>.
    expect(image.startsWith("https://www.agentclash.dev/og?")).toBe(true);
    expect(image).toContain("kind=Compare");
  });

  it("keeps imageless entries valid and never emits an empty images array", () => {
    const entries = sitemap();
    const byUrl = new Map(entries.map((entry) => [entry.url, entry]));

    // .txt endpoints are intentionally imageless (not HTML pages).
    expect(
      byUrl.get("https://www.agentclash.dev/llms.txt")?.images,
    ).toBeUndefined();
    expect(
      byUrl.get("https://www.agentclash.dev/llms-full.txt")?.images,
    ).toBeUndefined();

    // Every entry is still a valid sitemap entry: a string url, and images —
    // when present — is a non-empty array of absolute URLs (no `images: []`).
    for (const entry of entries) {
      expect(typeof entry.url).toBe("string");
      if (entry.images !== undefined) {
        expect(entry.images.length).toBeGreaterThan(0);
        for (const img of entry.images) {
          expect(img.startsWith("https://")).toBe(true);
        }
      }
    }
    expect(() => JSON.stringify(entries)).not.toThrow();
  });
});
