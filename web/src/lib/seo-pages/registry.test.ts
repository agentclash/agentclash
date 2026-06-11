import { describe, expect, it } from "vitest";
import {
  getAllSeoPagePaths,
  getSeoPageByPath,
  getSeoPagesByPrefix,
  SEO_PAGE_REGISTRY,
} from "./registry";

describe("SEO page registry", () => {
  it("registers every planned landing page path once", () => {
    expect(getAllSeoPagePaths()).toEqual([
      "/open-source-ai-agent-evaluation",
      "/agent-evals",
      "/llm-agent-evaluation",
      "/agent-evaluation-framework",
      "/ai-agent-testing",
      "/agent-trajectory-evaluation",
      "/ci-cd-agent-evaluation",
      "/ai-agent-benchmark",
      "/agent-reliability-benchmark",
      "/use-cases/coding-agent-evaluation",
      "/use-cases/research-agent-evaluation",
      "/use-cases/support-agent-evaluation",
      "/industries/banking",
      "/industries/insurance",
      "/industries/government",
      "/glossary/agent-evaluation",
      "/glossary/challenge-pack",
      "/glossary/release-gate",
      "/features/agent-scorecards",
      "/features/agent-replay",
      "/features/challenge-packs",
    ]);
    expect(SEO_PAGE_REGISTRY).toHaveLength(21);
  });

  it("groups use-case and feature routes by prefix", () => {
    expect(getSeoPagesByPrefix("/use-cases").map((page) => page.path)).toEqual([
      "/use-cases/coding-agent-evaluation",
      "/use-cases/research-agent-evaluation",
      "/use-cases/support-agent-evaluation",
    ]);
    expect(getSeoPagesByPrefix("/features").map((page) => page.path)).toEqual([
      "/features/agent-scorecards",
      "/features/agent-replay",
      "/features/challenge-packs",
    ]);
    expect(getSeoPagesByPrefix("/industries").map((page) => page.path)).toEqual([
      "/industries/banking",
      "/industries/insurance",
      "/industries/government",
    ]);
    expect(getSeoPagesByPrefix("/glossary").map((page) => page.path)).toEqual([
      "/glossary/agent-evaluation",
      "/glossary/challenge-pack",
      "/glossary/release-gate",
    ]);
  });

  it("includes canonical metadata fields for each page", () => {
    for (const page of SEO_PAGE_REGISTRY) {
      expect(getSeoPageByPath(page.path)).toBe(page);
      expect(page.pageTitle.length).toBeGreaterThan(10);
      expect(page.metaDescription.length).toBeGreaterThan(40);
      expect(page.faqItems.length).toBeGreaterThanOrEqual(3);
      expect(page.breadcrumbs.at(-1)?.url).toBe(page.path);
    }
  });

  it("uses collection index URLs for nested page breadcrumbs", () => {
    for (const page of getSeoPagesByPrefix("/use-cases")) {
      expect(page.breadcrumbs).toEqual([
        { name: "Home", url: "/" },
        { name: "Use cases", url: "/use-cases" },
        { name: expect.any(String), url: page.path },
      ]);
      expect(new Set(page.breadcrumbs.map((crumb) => crumb.url)).size).toBe(
        page.breadcrumbs.length,
      );
    }

    for (const page of getSeoPagesByPrefix("/features")) {
      expect(page.breadcrumbs).toEqual([
        { name: "Home", url: "/" },
        { name: "Features", url: "/features" },
        { name: expect.any(String), url: page.path },
      ]);
      expect(new Set(page.breadcrumbs.map((crumb) => crumb.url)).size).toBe(
        page.breadcrumbs.length,
      );
    }

    for (const page of getSeoPagesByPrefix("/industries")) {
      expect(page.breadcrumbs).toEqual([
        { name: "Home", url: "/" },
        { name: "Industries", url: "/industries" },
        { name: expect.any(String), url: page.path },
      ]);
    }

    for (const page of getSeoPagesByPrefix("/glossary")) {
      expect(page.breadcrumbs).toEqual([
        { name: "Home", url: "/" },
        { name: "Glossary", url: "/glossary" },
        { name: expect.any(String), url: page.path },
      ]);
    }
  });
});
