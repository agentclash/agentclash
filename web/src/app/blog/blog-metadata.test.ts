import { describe, expect, it } from "vitest";
import { blogRssAlternate } from "@/lib/seo";
import { metadata as blogMetadata } from "./page";
import { generateMetadata } from "./[slug]/page";

describe("blog RSS autodiscovery metadata", () => {
  it("links the RSS feed from the blog index metadata", () => {
    expect(blogMetadata.alternates).toMatchObject({
      canonical: "/blog",
      types: blogRssAlternate,
    });
  });

  it("links the RSS feed from blog post metadata", async () => {
    const metadata = await generateMetadata({
      params: Promise.resolve({
        slug: "ai-agent-evaluation-regression-testing",
      }),
    });

    expect(metadata.alternates).toMatchObject({
      canonical: "/blog/ai-agent-evaluation-regression-testing",
      types: blogRssAlternate,
    });
  });
});
