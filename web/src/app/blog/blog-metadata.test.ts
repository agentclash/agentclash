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

describe("blog index social metadata", () => {
  it("adds explicit Open Graph image and Twitter card metadata", () => {
    expect(blogMetadata).toMatchObject({
      title: "AI Agent Evaluation Blog - AgentClash",
      description:
        "Engineering notes on AI agent evaluation, head-to-head agent evals, replayable failures, scorecards, and CI regression gates.",
      openGraph: {
        title: "AI Agent Evaluation Blog - AgentClash",
        description:
          "Engineering notes on AI agent evaluation, head-to-head agent evals, replayable failures, scorecards, and CI regression gates.",
        url: "/blog",
        type: "website",
        siteName: "AgentClash",
        images: [
          {
            url: "/og-image.png",
            width: 1200,
            height: 630,
          },
        ],
      },
      twitter: {
        card: "summary_large_image",
        title: "AI Agent Evaluation Blog - AgentClash",
        description:
          "Engineering notes on AI agent evaluation, head-to-head agent evals, replayable failures, scorecards, and CI regression gates.",
        images: [
          {
            url: "/twitter-image.png",
          },
        ],
      },
    });

    const openGraph = blogMetadata.openGraph as {
      images?: Array<{ alt?: string }>;
    };
    const twitter = blogMetadata.twitter as {
      images?: Array<{ alt?: string }>;
    };
    const ogAlt = openGraph.images?.[0]?.alt;

    expect(ogAlt).toContain("AgentClash");
    expect(ogAlt).toContain("evaluation blog");
    expect(twitter.images?.[0]?.alt).toBe(ogAlt);
  });
});

describe("blog post social metadata", () => {
  it("adds explicit Open Graph image and Twitter card metadata", async () => {
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

    expect(metadata).toMatchObject({
      title: "Fixture Post — AgentClash",
      description: "Fixture description.",
      openGraph: {
        type: "article",
        title: "Fixture Post — AgentClash",
        description: "Fixture description.",
        url: "/blog/fixture-post",
        siteName: "AgentClash",
        publishedTime: "2026-05-08",
        authors: ["AgentClash"],
        images: [
          {
            url: "/og-image.png",
            width: 1200,
            height: 630,
            alt: "Fixture Post — Fixture description.",
          },
        ],
      },
      twitter: {
        card: "summary_large_image",
        title: "Fixture Post — AgentClash",
        description: "Fixture description.",
        images: [
          {
            url: "/twitter-image.png",
            alt: "Fixture Post — Fixture description.",
          },
        ],
      },
    });
  });
});
