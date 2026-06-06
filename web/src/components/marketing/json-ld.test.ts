import { describe, expect, it } from "vitest";
import {
  articleSchema,
  blogIndexSchema,
  changelogIndexSchema,
  docsPageSchema,
  organizationSchema,
  productSchema,
  SITE_URL,
  websiteSchema,
} from "./json-ld";

describe("productSchema", () => {
  it("builds SoftwareApplication structured data with absolute URLs", () => {
    expect(
      productSchema({
        name: "AI Agent Evaluation Platform for Real Tasks - AgentClash",
        description: "Evaluate AI agents on real tasks.",
        url: "/platform/agent-evaluation",
        applicationSubCategory: "AI agent evaluation platform",
        featureList: ["Replay evidence", "CI regression gates"],
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
      featureList: ["Replay evidence", "CI regression gates"],
      sameAs: [
        "https://github.com/agentclash/agentclash",
        "https://www.npmjs.com/package/agentclash",
      ],
      offers: {
        "@type": "Offer",
        name: "Open-source self-hosted edition",
        price: "0",
        priceCurrency: "USD",
        availability: "https://schema.org/InStock",
        category: "Open-source software",
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

describe("changelogIndexSchema", () => {
  it("builds breadcrumb, WebPage, ItemList, and FAQ structured data", () => {
    const schema = changelogIndexSchema(
      [
        {
          id: "2026-05-25",
          label: "May 25 – Jun 01, 2026",
          headline: "Datasets and /try demos",
          endDate: "2026-06-01",
          entryCount: 12,
        },
      ],
      [
        {
          question: "What is the AgentClash changelog?",
          answer: "Release notes grouped every ten days.",
        },
      ],
    );

    expect(schema).toHaveLength(4);
    expect(schema[0]).toMatchObject({
      "@type": "BreadcrumbList",
    });
    expect(schema[0].itemListElement).toEqual([
      {
        "@type": "ListItem",
        position: 1,
        name: "Home",
        item: `${SITE_URL}/`,
      },
      {
        "@type": "ListItem",
        position: 2,
        name: "Changelog",
        item: `${SITE_URL}/changelog`,
      },
    ]);
    expect(schema[1]).toMatchObject({
      "@type": "WebPage",
      url: `${SITE_URL}/changelog`,
      dateModified: "2026-06-01",
    });
    expect(schema[2]).toMatchObject({
      "@type": "ItemList",
      numberOfItems: 1,
      itemListElement: [
        {
          "@type": "ListItem",
          position: 1,
          url: `${SITE_URL}/changelog/2026-05-25`,
        },
      ],
    });
    expect(schema[3]).toMatchObject({
      "@type": "FAQPage",
    });
  });
});

describe("articleSchema", () => {
  it("builds BlogPosting structured data with a WebPage main entity", () => {
    expect(
      articleSchema({
        headline: "Agent Evaluation",
        description: "Evaluate agents on real tasks.",
        url: "/blog/agent-evaluation",
        datePublished: "2026-05-08",
        authorName: "AgentClash",
      }),
    ).toMatchObject({
      "@context": "https://schema.org",
      "@type": "BlogPosting",
      headline: "Agent Evaluation",
      description: "Evaluate agents on real tasks.",
      url: `${SITE_URL}/blog/agent-evaluation`,
      mainEntityOfPage: {
        "@type": "WebPage",
        "@id": `${SITE_URL}/blog/agent-evaluation`,
        url: `${SITE_URL}/blog/agent-evaluation`,
      },
      image: {
        "@type": "ImageObject",
        url: `${SITE_URL}/og-image.png`,
        width: 1200,
        height: 630,
      },
      datePublished: "2026-05-08",
      dateModified: "2026-05-08",
      author: {
        "@type": "Person",
        name: "AgentClash",
      },
      publisher: {
        "@type": "Organization",
        name: "AgentClash",
      },
    });
  });
});

describe("docsPageSchema", () => {
  it("builds BreadcrumbList and TechArticle structured data for docs pages", () => {
    const schema = docsPageSchema({
      title: "Quickstart",
      description: "Run your first AgentClash eval.",
      href: "/docs/getting-started/quickstart",
    });

    expect(schema).toHaveLength(2);
    expect(schema[0]).toMatchObject({
      "@context": "https://schema.org",
      "@type": "BreadcrumbList",
      itemListElement: [
        {
          "@type": "ListItem",
          position: 1,
          name: "Home",
          item: `${SITE_URL}/`,
        },
        {
          "@type": "ListItem",
          position: 2,
          name: "Docs",
          item: `${SITE_URL}/docs`,
        },
        {
          "@type": "ListItem",
          position: 3,
          name: "Quickstart",
          item: `${SITE_URL}/docs/getting-started/quickstart`,
        },
      ],
    });
    expect(schema[1]).toMatchObject({
      "@context": "https://schema.org",
      "@type": "TechArticle",
      headline: "Quickstart",
      description: "Run your first AgentClash eval.",
      url: `${SITE_URL}/docs/getting-started/quickstart`,
      mainEntityOfPage: {
        "@type": "WebPage",
        "@id": `${SITE_URL}/docs/getting-started/quickstart`,
        url: `${SITE_URL}/docs/getting-started/quickstart`,
      },
      author: {
        "@type": "Organization",
        name: "AgentClash",
      },
      isPartOf: {
        "@type": "WebSite",
        name: "AgentClash Docs",
        url: `${SITE_URL}/docs`,
      },
    });
  });

  it("uses a two-item breadcrumb for the docs home page", () => {
    const schema = docsPageSchema({
      title: "AgentClash Docs",
      description: "Documentation for AgentClash.",
      href: "/docs/",
      faqItems: [
        {
          question: "Where should I start?",
          answer: "Start with the quickstart.",
        },
      ],
    });

    expect(schema).toHaveLength(2);
    expect(schema[0]).toMatchObject({
      "@type": "BreadcrumbList",
      itemListElement: [
        {
          "@type": "ListItem",
          position: 1,
          name: "Home",
          item: `${SITE_URL}/`,
        },
        {
          "@type": "ListItem",
          position: 2,
          name: "Docs",
          item: `${SITE_URL}/docs`,
        },
      ],
    });
    expect(schema[1]).toMatchObject({
      "@type": "FAQPage",
      mainEntity: [
        {
          "@type": "Question",
          name: "Where should I start?",
          acceptedAnswer: {
            "@type": "Answer",
            text: "Start with the quickstart.",
          },
        },
      ],
    });
    expect(schema.map((item) => item["@type"])).not.toContain("TechArticle");
  });

  it("rejects FAQ items for non-home docs pages", () => {
    expect(() =>
      docsPageSchema({
        title: "Quickstart",
        description: "Run your first AgentClash eval.",
        href: "/docs/getting-started/quickstart",
        faqItems: [
          {
            question: "Where should I start?",
            answer: "Start with the quickstart.",
          },
        ],
      }),
    ).toThrow("docsPageSchema faqItems are only supported for /docs");
  });
});

describe("organizationSchema", () => {
  it("builds a rich Organization node with site URL and social profiles", () => {
    expect(organizationSchema()).toMatchObject({
      "@context": "https://schema.org",
      "@type": "Organization",
      name: "AgentClash",
      url: SITE_URL,
      sameAs: [
        "https://github.com/agentclash/agentclash",
        "https://www.npmjs.com/package/agentclash",
      ],
    });
  });
});

describe("websiteSchema", () => {
  it("builds a WebSite entity without a non-functional SearchAction", () => {
    const schema = websiteSchema();

    expect(schema).toMatchObject({
      "@context": "https://schema.org",
      "@type": "WebSite",
      name: "AgentClash",
      url: SITE_URL,
      publisher: { "@type": "Organization", name: "AgentClash" },
    });
    expect(schema.potentialAction).toBeUndefined();
  });
});

describe("docsPageSchema freshness dates", () => {
  it("emits datePublished and dateModified on the TechArticle when provided", () => {
    const schema = docsPageSchema({
      title: "Quickstart",
      description: "Run your first AgentClash eval.",
      href: "/docs/getting-started/quickstart",
      datePublished: "2026-01-01",
      dateModified: "2026-05-01",
    });
    const techArticle = schema.find((node) => node["@type"] === "TechArticle");

    expect(techArticle).toMatchObject({
      datePublished: "2026-01-01",
      dateModified: "2026-05-01",
    });
  });

  it("omits dates when none are provided", () => {
    const schema = docsPageSchema({
      title: "Quickstart",
      description: "Run your first AgentClash eval.",
      href: "/docs/getting-started/quickstart",
    });
    const techArticle = schema.find((node) => node["@type"] === "TechArticle");

    expect(techArticle?.datePublished).toBeUndefined();
    expect(techArticle?.dateModified).toBeUndefined();
  });
});
