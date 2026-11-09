import { describe, expect, it, vi } from "vitest";
import {
  buildPublicLlmsFull,
  buildPublicLlmsIndex,
  canonicalPathFromMarkdownSegments,
  getAllPublicContent,
  getPublicSearchIndex,
  normalizePublicPath,
  resolvePublicContent,
} from "./public-content";
import { isMarkdownNegotiablePath } from "./public-http";

describe("public content registry", () => {
  it("keeps known and unknown modification dates stable across build clocks", () => {
    try {
      vi.useFakeTimers();
      vi.setSystemTime(new Date("2026-10-01T00:00:00Z"));
      const before = getAllPublicContent().filter((item) => ["/", "/pricing", "/docs", "/docs/agent-skills"].includes(item.canonicalPath)).map(({ canonicalPath, lastModified }) => ({ canonicalPath, lastModified }));
      vi.setSystemTime(new Date("2026-10-28T00:00:00Z"));
      const after = getAllPublicContent().filter((item) => ["/", "/pricing", "/docs", "/docs/agent-skills"].includes(item.canonicalPath)).map(({ canonicalPath, lastModified }) => ({ canonicalPath, lastModified }));
      expect(after).toEqual(before);
      expect(after).toContainEqual({ canonicalPath: "/", lastModified: undefined });
      expect(after).toContainEqual({ canonicalPath: "/docs", lastModified: "2026-08-09" });
      expect(after).toContainEqual({ canonicalPath: "/docs/agent-skills", lastModified: undefined });
    } finally {
      vi.useRealTimers();
    }
  });

  it("has unique canonical and Markdown paths", () => {
    const items = getAllPublicContent();
    expect(new Set(items.map((item) => item.canonicalPath)).size).toBe(items.length);
    expect(new Set(items.map((item) => item.markdownPath)).size).toBe(items.length);
  });

  it("renders every registered page as semantic Markdown", () => {
    for (const item of getAllPublicContent()) {
      const markdown = item.renderMarkdown("https://example.test");
      expect(markdown, item.canonicalPath).toMatch(/^#\s+\S/m);
      expect(markdown, item.canonicalPath).toContain("Source: https://example.test");
      expect(markdown, item.canonicalPath).not.toContain("<nav");
      expect(markdown, item.canonicalPath).not.toContain("<footer");
      expect(isMarkdownNegotiablePath(item.canonicalPath), item.canonicalPath).toBe(true);
    }
  });

  it("covers core, structured, and generated route families", () => {
    for (const pathname of [
      "/",
      "/pricing",
      "/docs",
      "/docs/getting-started/quickstart",
      "/blog",
      "/changelog",
      "/compare/agentclash-vs-langsmith",
      "/features/agent-replay",
      "/try",
      "/try/codex",
      "/tryouts",
      "/agent-opportunity",
    ]) {
      expect(resolvePublicContent(pathname), pathname).not.toBeNull();
    }
  });

  it.each([
    "/../pricing",
    "/%2e%2e/pricing",
    "/docs//quickstart",
    "/share/token",
    "/workspaces/ws_123",
    "/api/private",
    "/pricing?token=secret",
  ])("rejects unsafe or private path %s", (pathname) => {
    expect(normalizePublicPath(pathname)).toBeNull();
    expect(resolvePublicContent(pathname)).toBeNull();
  });

  it("maps Markdown segments without accepting traversal", () => {
    expect(canonicalPathFromMarkdownSegments([])).toBe("/");
    expect(canonicalPathFromMarkdownSegments(["docs", "getting-started", "quickstart"])).toBe(
      "/docs/getting-started/quickstart",
    );
    expect(canonicalPathFromMarkdownSegments(["docs", "..", "pricing"])).toBeNull();
  });

  // The first full llms build runs the whole public registry, which can pass the
  // default 5s timeout on a busy runner; the assertions are unchanged.
  it("drives llms and search from exact registered URLs", { timeout: 30000 }, () => {
    const index = buildPublicLlmsIndex("https://example.test");
    const full = buildPublicLlmsFull("https://example.test");
    const search = getPublicSearchIndex();

    expect(index).toContain("https://example.test/md/pricing");
    expect(index).toContain("https://example.test/openapi.yaml");
    expect(index).toContain("https://example.test/cli-schema.json");
    expect(index).not.toContain("<agent>");
    expect(full).toContain("# AgentClash Pricing");
    expect(full).toContain("Monthly: $49/ month");
    expect(full).toContain("Annual billing: $39/ month");
    expect(full).not.toContain("/share/");
    expect(full).not.toContain("# Publication ");
    expect(search.some((item) => item.href === "/pricing")).toBe(true);
    expect(search.some((item) => item.href.startsWith("/workspaces"))).toBe(false);
  });
});

describe("editorial representations", () => {
  it("includes each page's concrete example and limitation in Markdown", async () => {
    const { SEO_PAGE_REGISTRY } = await import("@/lib/seo-pages");
    for (const page of SEO_PAGE_REGISTRY) {
      expect(page.editorial?.example, page.path).toBeTruthy();
      expect(page.editorial?.limitation, page.path).toBeTruthy();
      const markdown = resolvePublicContent(page.path)?.renderMarkdown();
      expect(markdown, page.path).toContain(page.editorial.example);
      expect(markdown, page.path).toContain(page.editorial.limitation);
    }
  });
  it("registers factual security and methodology pages", () => {
    expect(resolvePublicContent("/security")?.renderMarkdown()).toContain("private vulnerability");
    expect(resolvePublicContent("/security")?.includeIn.sitemap).toBe(true);
    expect(resolvePublicContent("/benchmarks/methodology")?.includeIn.sitemap).toBe(true);
    expect(resolvePublicContent("/benchmarks/methodology")?.renderMarkdown()).toContain("one run per model");
  });

  it("shares benchmark hub methodology and FAQ with Markdown", () => {
    const markdown = resolvePublicContent("/benchmarks")?.renderMarkdown() ?? "";
    expect(markdown).toContain("Freeze inputs");
    expect(markdown).toContain("Token efficiency is not dollar spend");
    expect(markdown).toContain("one run per model");
    expect(markdown).toContain("What does the public report compare?");
  });

  it("shares collection intros with Markdown", async () => {
    const { SEO_COLLECTION_INTROS } = await import("@/lib/seo-pages/collections");
    for (const [path, intro] of Object.entries(SEO_COLLECTION_INTROS)) {
      expect(resolvePublicContent(path)?.renderMarkdown(), path).toContain(intro);
    }
    expect(resolvePublicContent("/industries")?.renderMarkdown()).toContain(
      "no deployment or compliance result is implied",
    );
  });

  it("links named fixtures from their pages", () => {
    expect(resolvePublicContent("/use-cases/research-agent-evaluation")?.renderMarkdown()).toContain(
      "examples/challenge-packs/deep-research-analyst.yaml",
    );
    expect(resolvePublicContent("/use-cases/support-agent-evaluation")?.renderMarkdown()).toContain(
      "examples/challenge-packs/multi-turn-refund-recovery.yaml",
    );
    const banking = resolvePublicContent("/industries/banking")?.renderMarkdown() ?? "";
    expect(banking).toContain("examples/challenge-packs/multi-turn-refund-recovery.yaml");
    expect(banking).toContain("examples/challenge-packs/secret-hygiene-env.yaml");
  });
});
