import { describe, expect, it, vi } from "vitest";
import { blogRssAlternate } from "@/lib/seo";
import { metadata as blogMetadata } from "./page";
import { generateMetadata } from "./[slug]/page";

const getPostBySlugMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/blog", () => ({
  getAllPosts: vi.fn(() => []),
  getAllSlugs: vi.fn(() => []),
  getPostBySlug: getPostBySlugMock,
}));

describe("blog RSS autodiscovery metadata", () => {
  it("links the RSS feed from the blog index metadata", () => {
    expect(blogMetadata.alternates).toMatchObject({
      canonical: "/blog",
      types: blogRssAlternate,
    });
  });

  it("links the RSS feed from blog post metadata", async () => {
    getPostBySlugMock.mockReturnValue({
      slug: "fixture-post",
      title: "Fixture Post",
      date: "2026-05-08",
      description: "Fixture description.",
      author: "AgentClash",
      content: "",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({
        slug: "fixture-post",
      }),
    });

    expect(metadata.alternates).toMatchObject({
      canonical: "/blog/fixture-post",
      types: blogRssAlternate,
    });
  });
});
