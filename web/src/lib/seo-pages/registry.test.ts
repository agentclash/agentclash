import { describe, expect, it } from "vitest";
import {
  getAllSeoPagePaths,
  getSeoPageByPath,
  SEO_PAGE_REGISTRY,
} from "./registry";

describe("SEO page registry cross-links", () => {
  it("adds hub links to /compare and peer SEO pages without self-links", () => {
    for (const page of SEO_PAGE_REGISTRY) {
      const hrefs = page.relatedLinks.map((link) => link.href);
      expect(hrefs).toContain("/compare");
      expect(hrefs).not.toContain(page.path);
      expect(new Set(hrefs).size).toBe(hrefs.length);
    }
  });

  it("indexes every registry path through getAllSeoPagePaths", () => {
    expect(getAllSeoPagePaths()).toHaveLength(SEO_PAGE_REGISTRY.length);
    expect(getSeoPageByPath("/agent-evals")?.relatedLinks.length).toBeGreaterThan(
      3,
    );
  });
});
