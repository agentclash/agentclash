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
