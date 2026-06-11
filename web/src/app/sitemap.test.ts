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

vi.mock("@/lib/benchmarks", () => ({
  getAllReports: vi.fn(() => [
    {
      slug: "claude-opus-4-8-vs-the-field",
      title: "Claude Opus 4.8 vs the field",
      date: "2026-06-06",
      verdict: "Opus 4.8 took 4 of 5.",
      sample: false,
    },
    {
      slug: "sample-illustrative",
      title: "Sample illustrative report",
      date: "2026-06-06",
      verdict: "Representative numbers only.",
      sample: true,
    },
    {
      slug: "bad-date-report",
      title: "Report with an unparseable date",
      date: "Q2 2026",
      verdict: "Date will not parse.",
      sample: false,
    },
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
    expect(byUrl.get("https://www.agentclash.dev/enterprise")).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.82,
    });
    expect(byUrl.get("https://www.agentclash.dev/services")).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.78,
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
    expect(
      byUrl.get("https://www.agentclash.dev/agent-evals"),
    ).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.84,
    });
    expect(
      byUrl.get("https://www.agentclash.dev/features/agent-replay"),
    ).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.76,
    });
    expect(byUrl.get("https://www.agentclash.dev/use-cases")).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.78,
    });
    expect(byUrl.get("https://www.agentclash.dev/industries")).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.78,
    });
    expect(byUrl.get("https://www.agentclash.dev/glossary")).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.78,
    });
    expect(byUrl.get("https://www.agentclash.dev/features")).toMatchObject({
      changeFrequency: "monthly",
      priority: 0.78,
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

  it("always includes the benchmarks hub index", () => {
    const entries = sitemap();
    const byUrl = new Map(entries.map((entry) => [entry.url, entry]));

    expect(
      byUrl.get("https://www.agentclash.dev/benchmarks"),
    ).toMatchObject({
      changeFrequency: "weekly",
      priority: 0.8,
    });
  });

  it("includes per-report pages when measured reports exist", () => {
    const entries = sitemap();
    const byUrl = new Map(entries.map((entry) => [entry.url, entry]));
    const report = byUrl.get(
      "https://www.agentclash.dev/benchmarks/claude-opus-4-8-vs-the-field",
    );
    expect(report).toMatchObject({
      lastModified: new Date("2026-06-06"),
      changeFrequency: "monthly",
      priority: 0.75,
    });
    const image = report?.images?.[0] ?? "";
    expect(image.startsWith("https://www.agentclash.dev/og?")).toBe(true);
    expect(image).toContain("kind=Benchmark");
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

  it("serializes to well-formed sitemap XML with escaped image URLs", () => {
    const entries = sitemap();

    // Mirror Next's sitemap serializer (resolve-route-data.js): it interpolates
    // url + image URLs into <loc>/<image:loc> WITHOUT XML-escaping, so the
    // strings we hand it must already be XML-safe. A raw `&` (from
    // URLSearchParams) would make the document non-well-formed and break crawler
    // parsing of the entire sitemap — this is invisible to assertions on the
    // in-memory array, so validate the serialized artifact.
    const body = entries
      .map((e) => {
        const imgs = (e.images ?? [])
          .map(
            (img) =>
              `<image:image><image:loc>${img}</image:loc></image:image>`,
          )
          .join("");
        return `<url><loc>${e.url}</loc>${imgs}</url>`;
      })
      .join("");
    const xml =
      `<?xml version="1.0" encoding="UTF-8"?>` +
      `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" ` +
      `xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">${body}</urlset>`;

    const doc = new DOMParser().parseFromString(xml, "text/xml");
    expect(doc.querySelector("parsererror")).toBeNull();

    // Directly guard the regression: a multi-param image URL exists and its
    // ampersands are entity-escaped (no bare `&`).
    const multi = entries
      .flatMap((e) => e.images ?? [])
      .find((img) => img.includes("kind=") && img.includes("title="));
    expect(multi).toBeDefined();
    expect(multi).toContain("&amp;");
    expect(/&(?!amp;|lt;|gt;|quot;|#39;)/.test(multi ?? "")).toBe(false);
  });

  it("excludes sample reports so fabricated numbers are never indexed", () => {
    const urls = new Set(sitemap().map((entry) => entry.url));
    expect(
      urls.has("https://www.agentclash.dev/benchmarks/sample-illustrative"),
    ).toBe(false);
  });

  it("survives an unparseable report date without crashing the whole sitemap", () => {
    const byUrl = new Map(sitemap().map((entry) => [entry.url, entry]));
    const bad = byUrl.get(
      "https://www.agentclash.dev/benchmarks/bad-date-report",
    );
    // Still listed, but with no Invalid Date lastModified that would make Next's
    // serializer throw on .toISOString() and take down the entire sitemap.xml.
    expect(bad).toBeDefined();
    expect(bad?.lastModified).toBeUndefined();
    expect(() => JSON.stringify(sitemap())).not.toThrow();
  });
});
