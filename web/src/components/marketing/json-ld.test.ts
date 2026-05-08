import { describe, expect, it } from "vitest";
import { blogIndexSchema, productSchema, SITE_URL } from "./json-ld";

describe("productSchema", () => {
  it("builds SoftwareApplication structured data with absolute URLs", () => {
    expect(
      productSchema({
        name: "AI Agent Evaluation Platform for Real Tasks - AgentClash",
        description: "Evaluate AI agents on real tasks.",
        url: "/platform/agent-evaluation",
        applicationSubCategory: "AI agent evaluation platform",
      }),
    ).toMatchObject({
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      name: "AI Agent Evaluation Platform for Real Tasks - AgentClash",
      alternateName: "Agent Clash",
      applicationCategory: "DeveloperApplication",
      applicationSubCategory: "AI agent evaluation platform",
      operatingSystem: "Web, macOS, Linux, Windows",
      description: "Evaluate AI agents on real tasks.",
      url: `${SITE_URL}/platform/agent-evaluation`,
      offers: {
        "@type": "Offer",
        price: "0",
        priceCurrency: "USD",
      },
    });
  });
});

describe("blogIndexSchema", () => {
  it("builds Blog and ItemList structured data with absolute post URLs", () => {
    const schema = blogIndexSchema([
      {
        slug: "agent-evaluation",
        title: "Agent Evaluation",
        description: "Evaluate agents on real tasks.",
        date: "2026-05-08",
        author: "AgentClash",
      },
    ]);

    expect(schema).toHaveLength(2);
    expect(schema[0]).toMatchObject({
      "@context": "https://schema.org",
      "@type": "Blog",
      name: "AgentClash Blog",
      url: `${SITE_URL}/blog`,
      blogPost: [
        {
          "@id": `${SITE_URL}/blog/agent-evaluation`,
        },
      ],
    });
    expect(schema[1]).toMatchObject({
      "@context": "https://schema.org",
      "@type": "ItemList",
      name: "AgentClash Blog Posts",
      numberOfItems: 1,
      itemListElement: [
        {
          "@type": "ListItem",
          position: 1,
          name: "Agent Evaluation",
          description: "Evaluate agents on real tasks.",
          url: `${SITE_URL}/blog/agent-evaluation`,
          item: {
            "@id": `${SITE_URL}/blog/agent-evaluation`,
          },
        },
      ],
    });
  });
});
