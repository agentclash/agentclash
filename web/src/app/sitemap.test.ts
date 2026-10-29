import { describe, expect, it, vi } from "vitest";
import type { PublicContentDescriptor } from "@/lib/public-content";

const getAllPublicContentMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/public-content", () => ({
  PUBLIC_ORIGIN: "https://www.agentclash.dev",
  getAllPublicContent: getAllPublicContentMock,
}));

import sitemap from "./sitemap";

function descriptor(
  overrides: Partial<PublicContentDescriptor> &
    Pick<PublicContentDescriptor, "canonicalPath" | "title">,
): PublicContentDescriptor {
  return {
    canonicalPath: overrides.canonicalPath,
    markdownPath:
      overrides.canonicalPath === "/"
        ? "/md"
        : `/md${overrides.canonicalPath}`,
    title: overrides.title,
    description: overrides.description ?? "Fixture description",
    kind: overrides.kind ?? "marketing",
    lastModified: overrides.lastModified,
    indexable: overrides.indexable ?? true,
    includeIn: overrides.includeIn ?? {
      sitemap: true,
      llms: true,
      llmsFull: true,
      search: true,
      indexNow: true,
    },
    sitemapPriority: overrides.sitemapPriority ?? 0.7,
    changeFrequency: overrides.changeFrequency ?? "monthly",
    renderMarkdown: () => `# ${overrides.title}`,
  };
}

describe("sitemap", () => {
  it("serializes canonical registry URLs with real dates and metadata", () => {
    getAllPublicContentMock.mockReturnValue([
      descriptor({
        canonicalPath: "/",
        title: "AgentClash",
        lastModified: "2026-08-12",
        sitemapPriority: 1,
        changeFrequency: "weekly",
      }),
      descriptor({
        canonicalPath: "/docs",
        title: "Docs",
        lastModified: "2026-07-10",
        sitemapPriority: 0.85,
        changeFrequency: "weekly",
      }),
    ]);

    const entries = sitemap();
    expect(entries).toHaveLength(2);
    expect(entries[0]).toMatchObject({
      url: "https://www.agentclash.dev",
      lastModified: new Date("2026-08-12"),
      priority: 1,
      changeFrequency: "weekly",
    });
    expect(entries[1]).toMatchObject({
      url: "https://www.agentclash.dev/docs",
      lastModified: new Date("2026-07-10"),
      priority: 0.85,
    });
  });

  it("excludes non-indexable and sitemap-disabled content", () => {
    getAllPublicContentMock.mockReturnValue([
      descriptor({ canonicalPath: "/public", title: "Public" }),
      descriptor({
        canonicalPath: "/sample",
        title: "Sample",
        indexable: false,
      }),
      descriptor({
        canonicalPath: "/machine",
        title: "Machine",
        includeIn: {
          sitemap: false,
          llms: true,
          llmsFull: true,
          search: false,
          indexNow: false,
        },
      }),
    ]);

    expect(sitemap().map((entry) => entry.url)).toEqual([
      "https://www.agentclash.dev/public",
    ]);
  });

  it("never emits Markdown or machine-artifact URLs", () => {
    getAllPublicContentMock.mockReturnValue([
      descriptor({ canonicalPath: "/pricing", title: "Pricing" }),
    ]);
    const urls = sitemap().map((entry) => entry.url);
    expect(urls).not.toContain("https://www.agentclash.dev/md/pricing");
    expect(urls).not.toContain("https://www.agentclash.dev/llms.txt");
    expect(urls).not.toContain("https://www.agentclash.dev/openapi.yaml");
  });

  it("escapes ampersands in absolute Open Graph image URLs", () => {
    getAllPublicContentMock.mockReturnValue([
      descriptor({ canonicalPath: "/compare", title: "A & B" }),
    ]);
    const image = sitemap()[0]?.images?.[0] ?? "";
    expect(image.startsWith("https://www.agentclash.dev/og?")).toBe(true);
    expect(image).toContain("&amp;");
    expect(image).not.toMatch(/&(?!amp;|lt;|gt;|quot;|#39;)/);
  });

  it("omits dates when content has no known modification date", () => {
    getAllPublicContentMock.mockReturnValue([descriptor({ canonicalPath: "/unknown", title: "Unknown" })]);
    expect(sitemap()[0]?.lastModified).toBeUndefined();
  });

  it("omits invalid dates without failing the whole sitemap", () => {
    getAllPublicContentMock.mockReturnValue([
      descriptor({
        canonicalPath: "/bad-date",
        title: "Bad date",
        lastModified: "not-a-date",
      }),
    ]);
    expect(sitemap()[0]?.lastModified).toBeUndefined();
  });
});
