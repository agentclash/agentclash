import { PRICING_TIERS } from "@/lib/pricing-data";

type Props = {
  id: string;
  data: Record<string, unknown> | Record<string, unknown>[];
};

export function JsonLd({ id, data }: Props) {
  return (
    <script
      id={id}
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: JSON.stringify(data) }}
    />
  );
}

export const SITE_URL = "https://www.agentclash.dev";

export function breadcrumbSchema(
  items: Array<{ name: string; url: string }>,
): Record<string, unknown> {
  return {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: items.map((item, index) => ({
      "@type": "ListItem",
      position: index + 1,
      name: item.name,
      item: item.url.startsWith("http") ? item.url : `${SITE_URL}${item.url}`,
    })),
  };
}

export function faqSchema(
  qa: Array<{ question: string; answer: string }>,
): Record<string, unknown> {
  return {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: qa.map(({ question, answer }) => ({
      "@type": "Question",
      name: question,
      acceptedAnswer: {
        "@type": "Answer",
        text: answer,
      },
    })),
  };
}

export function productSchema({
  name,
  description,
  url,
  applicationSubCategory,
  softwareVersion,
  featureList,
}: {
  name: string;
  description: string;
  url: string;
  applicationSubCategory?: string;
  softwareVersion?: string;
  featureList?: string[];
}): Record<string, unknown> {
  return {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name,
    alternateName: "Agent Clash",
    applicationCategory: "DeveloperApplication",
    ...(applicationSubCategory ? { applicationSubCategory } : {}),
    operatingSystem: "Web, macOS, Linux, Windows",
    description,
    url: url.startsWith("http") ? url : `${SITE_URL}${url}`,
    ...(softwareVersion ? { softwareVersion } : {}),
    ...(featureList?.length ? { featureList } : {}),
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
  };
}

// Extracts the numeric amount from a display price like "$49" or "$0".
// Returns null for non-numeric values such as "Custom" (Enterprise).
function parsePriceAmount(value: string): string | null {
  const match = value.replace(/[,$]/g, "").match(/\d+(?:\.\d+)?/);
  return match ? match[0] : null;
}

// Machine-readable pricing for buyer/agent consumption: a SoftwareApplication
// whose offers array is built from PRICING_TIERS (the same data the human
// /pricing page renders), so the two can never drift. Priced tiers expose a
// UnitPriceSpecification; the custom Enterprise tier is a contact offer with no
// fixed price. "Opaque pricing is invisible pricing" to an evaluating agent.
export function pricingSchema(): Record<string, unknown> {
  const offers = PRICING_TIERS.map((tier) => {
    const monthly = tier.prices.monthly;
    const amount = parsePriceAmount(monthly.value);
    const perSeat = monthly.suffix.includes("seat");

    const base: Record<string, unknown> = {
      "@type": "Offer",
      name: `AgentClash ${tier.name}`,
      description: tier.blurb,
      category:
        tier.name === "Free" ? "Free / open-source" : "Subscription",
      url: `${SITE_URL}/pricing`,
      availability: "https://schema.org/InStock",
    };

    if (amount === null) {
      // Enterprise: custom pricing, evaluated via a sales conversation.
      return {
        ...base,
        priceCurrency: "USD",
        priceSpecification: {
          "@type": "PriceSpecification",
          priceCurrency: "USD",
        },
      };
    }

    return {
      ...base,
      price: amount,
      priceCurrency: "USD",
      priceSpecification: {
        "@type": "UnitPriceSpecification",
        price: amount,
        priceCurrency: "USD",
        unitText: perSeat ? "seat per month" : "month",
        billingIncrement: 1,
      },
    };
  });

  return {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "AgentClash",
    alternateName: "Agent Clash",
    applicationCategory: "DeveloperApplication",
    applicationSubCategory: "AI agent evaluation platform",
    operatingSystem: "Web, macOS, Linux, Windows",
    description:
      "AgentClash pricing: a free hosted tier and open-source self-hosting, paid Pro and Team tiers, and custom Enterprise. Bring your own LLM keys on every tier.",
    url: `${SITE_URL}/pricing`,
    sameAs: [
      "https://github.com/agentclash/agentclash",
      "https://www.npmjs.com/package/agentclash",
    ],
    offers,
  };
}

export function publisherSchema(): Record<string, unknown> {
  return {
    "@type": "Organization",
    name: "AgentClash",
    alternateName: "Agent Clash",
    url: SITE_URL,
    description:
      "Open-source AI agent evaluation platform for racing agents head-to-head on real tasks with sandboxed tools, replay, scorecards, and CI regression gates.",
    logo: {
      "@type": "ImageObject",
      url: `${SITE_URL}/icon.svg`,
    },
    sameAs: [
      "https://github.com/agentclash/agentclash",
      "https://www.npmjs.com/package/agentclash",
    ],
  };
}

// Standalone Organization node (with @context) for top-level use on the
// homepage. Nested usages (publisher/author) keep using publisherSchema().
export function organizationSchema(): Record<string, unknown> {
  return {
    "@context": "https://schema.org",
    ...publisherSchema(),
  };
}

// WebSite entity establishes AgentClash as a named site/entity for search and
// answer engines. No SearchAction: docs search is client-side with no
// query-param results URL, and a non-functional sitelinks searchbox can be
// flagged — entity establishment is the valuable part here.
export function websiteSchema(): Record<string, unknown> {
  return {
    "@context": "https://schema.org",
    "@type": "WebSite",
    name: "AgentClash",
    alternateName: "Agent Clash",
    url: SITE_URL,
    description:
      "Open-source AI agent evaluation platform. Race agents head-to-head on real tasks with sandboxed tools, replay, scorecards, and CI regression gates.",
    publisher: publisherSchema(),
  };
}

export function articleSchema({
  headline,
  description,
  url,
  datePublished,
  dateModified,
  authorName,
}: {
  headline: string;
  description: string;
  url: string;
  datePublished: string;
  dateModified?: string;
  authorName: string;
}): Record<string, unknown> {
  const absoluteUrl = url.startsWith("http") ? url : `${SITE_URL}${url}`;

  return {
    "@context": "https://schema.org",
    "@type": "BlogPosting",
    headline,
    description,
    url: absoluteUrl,
    mainEntityOfPage: {
      "@type": "WebPage",
      "@id": absoluteUrl,
      url: absoluteUrl,
    },
    image: {
      "@type": "ImageObject",
      url: `${SITE_URL}/og-image.png`,
      width: 1200,
      height: 630,
    },
    datePublished,
    dateModified: dateModified ?? datePublished,
    author: {
      "@type": "Person",
      name: authorName,
    },
    publisher: publisherSchema(),
  };
}

type BlogIndexPost = {
  slug: string;
  title: string;
  description: string;
  date: string;
  author: string;
};

export function blogIndexSchema(
  posts: BlogIndexPost[],
): Record<string, unknown>[] {
  const blogPosts = posts.map((post) => {
    const url = `${SITE_URL}/blog/${post.slug}`;

    return {
      atId: url,
      title: post.title,
      description: post.description,
      url,
    };
  });

  return [
    {
      "@context": "https://schema.org",
      "@type": "Blog",
      name: "AgentClash Blog",
      description:
        "Engineering notes on AI agent evaluation, replayable failures, scorecards, and CI regression gates.",
      url: `${SITE_URL}/blog`,
      publisher: publisherSchema(),
      blogPost: blogPosts.map((post) => ({
        "@id": post.atId,
      })),
    },
    {
      "@context": "https://schema.org",
      "@type": "ItemList",
      name: "AgentClash Blog Posts",
      url: `${SITE_URL}/blog`,
      numberOfItems: blogPosts.length,
      itemListElement: blogPosts.map((post, index) => ({
        "@type": "ListItem",
        position: index + 1,
        name: post.title,
        description: post.description,
        url: post.url,
        item: {
          "@id": post.atId,
        },
      })),
    },
  ];
}

type ChangelogIndexPeriod = {
  id: string;
  label: string;
  headline: string;
  endDate: string;
  entryCount: number;
};

export function changelogIndexSchema(
  periods: ChangelogIndexPeriod[],
  faqItems: Array<{ question: string; answer: string }> = [],
): Record<string, unknown>[] {
  const pageUrl = `${SITE_URL}/changelog`;
  const latestPeriod = periods[0];

  return [
    breadcrumbSchema([
      { name: "Home", url: "/" },
      { name: "Changelog", url: "/changelog" },
    ]),
    {
      "@context": "https://schema.org",
      "@type": "WebPage",
      name: "AgentClash Changelog",
      description:
        "Everything shipped in AgentClash from day one — scoring, regression, security packs, datasets, and CLI updates, grouped every ten days.",
      url: pageUrl,
      ...(latestPeriod
        ? {
            dateModified: latestPeriod.endDate,
          }
        : {}),
      isPartOf: {
        "@type": "WebSite",
        name: "AgentClash",
        url: SITE_URL,
      },
      publisher: publisherSchema(),
      mainEntity: {
        "@type": "ItemList",
        name: "AgentClash product updates",
        numberOfItems: periods.length,
      },
    },
    {
      "@context": "https://schema.org",
      "@type": "ItemList",
      name: "AgentClash product updates",
      url: pageUrl,
      numberOfItems: periods.length,
      itemListElement: periods.map((period, index) => ({
        "@type": "ListItem",
        position: index + 1,
        name: period.headline,
        description: `${period.label} — ${period.entryCount} updates`,
        url: `${pageUrl}/${period.id}`,
        item: {
          "@id": `${pageUrl}/${period.id}`,
        },
      })),
    },
    ...(faqItems.length ? [faqSchema(faqItems)] : []),
  ];
}

export function docsPageSchema({
  title,
  description,
  href,
  faqItems = [],
  datePublished,
  dateModified,
}: {
  title: string;
  description: string;
  href: string;
  faqItems?: Array<{ question: string; answer: string }>;
  datePublished?: string;
  dateModified?: string;
}): Record<string, unknown>[] {
  const absoluteUrl = href.startsWith("http") ? href : `${SITE_URL}${href}`;
  const pathname = href.replace(/^https?:\/\/[^/]+/, "").replace(/\/+$/, "");
  const isDocsHome = pathname === "/docs";
  const breadcrumbs =
    isDocsHome
      ? [
          { name: "Home", url: "/" },
          { name: "Docs", url: "/docs" },
        ]
      : [
          { name: "Home", url: "/" },
          { name: "Docs", url: "/docs" },
          { name: title, url: href },
        ];

  const schema = [
    breadcrumbSchema(breadcrumbs),
    ...(isDocsHome && faqItems.length ? [faqSchema(faqItems)] : []),
  ];
  if (!isDocsHome && faqItems.length) {
    throw new Error("docsPageSchema faqItems are only supported for /docs");
  }
  if (isDocsHome) return schema;

  return [
    ...schema,
    {
      "@context": "https://schema.org",
      "@type": "TechArticle",
      headline: title,
      description,
      url: absoluteUrl,
      ...(datePublished ? { datePublished } : {}),
      ...(dateModified ?? datePublished
        ? { dateModified: dateModified ?? datePublished }
        : {}),
      mainEntityOfPage: {
        "@type": "WebPage",
        "@id": absoluteUrl,
        url: absoluteUrl,
      },
      author: publisherSchema(),
      publisher: publisherSchema(),
      isPartOf: {
        "@type": "WebSite",
        name: "AgentClash Docs",
        url: `${SITE_URL}/docs`,
      },
    },
  ];
}
