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
      "/features/agent-scorecards",
      "/features/agent-replay",
      "/features/challenge-packs",
    ]);
    expect(SEO_PAGE_REGISTRY).toHaveLength(15);
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
});
