import { describe, expect, it } from "vitest";
import {
  getBlogRelatedResources,
  getMappedBlogRelatedResourceSlugs,
} from "./blog-related-resources";

describe("getBlogRelatedResources", () => {
  it("returns an empty list for unmapped blog posts", () => {
    expect(getBlogRelatedResources("nonexistent-slug")).toEqual([]);
  });

  it("returns two or three topic-selected links for mapped posts", () => {
    for (const slug of getMappedBlogRelatedResourceSlugs()) {
      const links = getBlogRelatedResources(slug);
      expect(links.length).toBeGreaterThanOrEqual(2);
      expect(links.length).toBeLessThanOrEqual(4);
      expect(new Set(links.map((link) => link.href)).size).toBe(links.length);
    }
  });

  it("selects comparison links for compare-focused posts", () => {
    const links = getBlogRelatedResources(
      "agentclash-vs-langsmith-braintrust-production",
    );
    expect(links.map((link) => link.href)).toContain("/compare");
  });

  it("links the changelog rollup post to /changelog", () => {
    const links = getBlogRelatedResources("product-updates-june-2026");
    expect(links[0]).toMatchObject({
      href: "/changelog",
      label: "Changelog",
    });
  });
});
