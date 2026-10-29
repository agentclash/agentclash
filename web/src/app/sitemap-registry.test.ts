import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  PUBLIC_ORIGIN,
  getAllPublicContent,
  normalizePublicPath,
  resolvePublicContent,
} from "@/lib/public-content";
import sitemap from "./sitemap";
import { getAllDocPaths } from "@/lib/docs";
import { getAllPosts } from "@/lib/blog";
import { getAllReports } from "@/lib/benchmarks";
import { COMPETITORS } from "@/lib/comparison-data";
import { getChangelogPeriods } from "@/lib/changelog";
import { SEO_PAGE_REGISTRY } from "@/lib/seo-pages";
import matter from "gray-matter";
import { isMarkdownNegotiablePath } from "@/lib/public-http";

describe("sitemap from the real registry", () => {
  it("emits unique canonical URLs on the public origin", () => {
    const entries = sitemap();
    expect(entries.length).toBeGreaterThan(0);
    for (const entry of entries) {
      expect(entry.url === PUBLIC_ORIGIN || entry.url.startsWith(`${PUBLIC_ORIGIN}/`)).toBe(true);
    }
    expect(new Set(entries.map((entry) => entry.url)).size).toBe(entries.length);
  });

  it("includes only indexable, sitemap-enabled, permitted paths", () => {
    const registry = getAllPublicContent();
    const expected = registry.filter((item) => item.indexable && item.includeIn.sitemap);
    const entries = sitemap();
    expect(entries).toHaveLength(expected.length);
    for (const item of expected) {
      expect(normalizePublicPath(item.canonicalPath), item.canonicalPath).toBe(item.canonicalPath);
    }
  });

  it("omits unknown dates and keeps emitted dates valid and not in the future", () => {
    const future = Date.now() + 24 * 60 * 60 * 1000;
    for (const entry of sitemap()) {
      if (!entry.lastModified) continue;
      const time = new Date(entry.lastModified).getTime();
      expect(Number.isNaN(time), entry.url).toBe(false);
      expect(time, entry.url).toBeLessThan(future);
    }
  });

  it("includes the evidence-backed additions without freezing the URL count", () => {
    const urls = new Set(sitemap().map((entry) => entry.url));
    expect(urls.has(`${PUBLIC_ORIGIN}/benchmarks/methodology`)).toBe(true);
    expect(urls.has(`${PUBLIC_ORIGIN}/security`)).toBe(true);
  });
});

// Routes that exist as files but are intentionally absent from the public
// registry: private app surfaces, auth flows, and noindexed product pages.
const EXCLUDED_STATIC_ROUTES: Record<string, string> = {
  "/auth/login": "auth flow",
  "/auth/device": "auth flow",
  "/dashboard": "private app",
  "/onboard": "private app",
  "/github/setup": "integration setup",
  "/vibe-evals": "product app, deliberately excluded from indexing",
  "/resources/eval-checklist/thank-you": "post-submit page, noindex",
};

function staticAppRoutes(): string[] {
  const appDir = path.join(process.cwd(), "src", "app");
  const routes: string[] = [];
  const walk = (dir: string, segments: string[]) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (entry.name.startsWith("[") || entry.name === "api") continue;
        walk(full, [...segments, entry.name]);
      } else if (entry.name === "page.tsx") {
        const clean = segments.filter((segment) => !/^\(.*\)$/.test(segment));
        routes.push(clean.length ? `/${clean.join("/")}` : "/");
      }
    }
  };
  walk(appDir, []);
  return routes;
}

describe("public route coverage", () => {
  it("registers every public static app route or excludes it with a reason", () => {
    for (const route of staticAppRoutes()) {
      if (route in EXCLUDED_STATIC_ROUTES) continue;
      expect(resolvePublicContent(route), `${route} (exclude in EXCLUDED_STATIC_ROUTES if intentional)`).not.toBeNull();
    }
  });
});

// Expected URLs come from each content owner, independently of registry construction.
// Docs includes authored pages, synthesized reference bodies, skill pages and hubs.
// Samples are intentionally noindex. API publications have their own paginated route tests.
describe("provider-backed dynamic route coverage", () => {
  it("registers, negotiates and sitemaps every independently discovered public content item", () => {
    const demosDir = path.join(process.cwd(), "..", "try-cli", "demos");
    const demos = fs.readdirSync(demosDir).filter((name) => name.endsWith(".trycli.yml")).map((name) => {
      const data = matter(`---\n${fs.readFileSync(path.join(demosDir, name), "utf8")}\n---`).data;
      return `/try/${data.slug}`;
    });
    const families = {
      docs: getAllDocPaths(),
      blog: getAllPosts().map((post) => `/blog/${post.slug}`),
      reports: getAllReports().filter((report) => !report.sample).map((report) => `/benchmarks/${report.slug}`),
      seo: SEO_PAGE_REGISTRY.map((page) => page.path),
      competitors: COMPETITORS.map((competitor) => `/compare/${competitor.slug}`),
      changelog: getChangelogPeriods().map((period) => `/changelog/${period.id}`),
      demos,
    };
    const registered = new Set(getAllPublicContent().filter((item) => item.indexable).map((item) => item.canonicalPath));
    const sitemapPaths = new Set(sitemap().map((item) => new URL(item.url).pathname));
    for (const [family, paths] of Object.entries(families)) {
      expect(paths.length, family).toBeGreaterThan(0);
      for (const pathname of paths) {
        expect(registered.has(pathname), `${family}: registry missing ${pathname}`).toBe(true);
        expect(sitemapPaths.has(pathname), `${family}: sitemap missing ${pathname}`).toBe(true);
        expect(isMarkdownNegotiablePath(pathname), `${family}: negotiation missing ${pathname}`).toBe(true);
      }
    }
  });

  it("uses authored edit dates for the corrected June articles without changing publication dates", () => {
    const blog = getAllPosts().find((post) => post.slug === "coding-agent-benchmark-june-2026")!;
    const report = getAllReports().find((item) => item.slug === "gpt-generations-expression-evaluator")!;
    expect([blog.date, report.date]).toEqual(["2026-06-11", "2026-06-07"]);
    expect([blog.dateModified, report.dateModified]).toEqual(["2026-10-28", "2026-10-28"]);
    const entries = sitemap();
    for (const pathname of [`/blog/${blog.slug}`, `/benchmarks/${report.slug}`]) {
      expect(entries.find((item) => item.url === `${PUBLIC_ORIGIN}${pathname}`)?.lastModified).toEqual(new Date("2026-10-28"));
    }
  });
});
