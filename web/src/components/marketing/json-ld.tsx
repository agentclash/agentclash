import { PRICING_TIERS } from "@/lib/pricing-data";

type Props = {
  id: string;
  data: Record<string, unknown> | Record<string, unknown>[];
};

// Escape characters that can break out of an inline <script>: a literal
// `</script>` (or `<`) closes the element early, and U+2028/U+2029 terminate
// an inline script. Replacing each with its \uXXXX escape keeps the
// JSON-LD valid. Protects every JsonLd caller, not just benchmarks.
export function serializeJsonLd(
  data: Record<string, unknown> | Record<string, unknown>[],
): string {
  return JSON.stringify(data).replace(
    /[<\u2028\u2029]/g,
    (char) =>
      "\\u" + char.charCodeAt(0).toString(16).padStart(4, "0"),
  );
}

export function JsonLd({ id, data }: Props) {
  return (
    <script
      id={id}
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: serializeJsonLd(data) }}
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
      "Open-source AI agent evaluation platform for finding agent failures on real tasks, replaying every step, promoting regressions, and gating releases on scorecards.",
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
      "Open-source AI agent evaluation platform. Find where agents break on real tasks, replay failures, promote regressions, and gate releases on scorecards.",
    publisher: publisherSchema(),
  };
}

// SoftwareSourceCode is the schema.org type purpose-built for an open-source
// repository — it links the named entity to its code, languages, and license,
// which the SoftwareApplication node does not express. Emitted on the homepage.
export function softwareSourceCodeSchema(): Record<string, unknown> {
  return {
    "@context": "https://schema.org",
    "@type": "SoftwareSourceCode",
    name: "AgentClash",
    alternateName: "Agent Clash",
    description:
      "Open-source AI agent evaluation platform for replayable failures, regression gates, sandboxed tool use, and CI-ready scorecards on real tasks.",
    url: SITE_URL,
    codeRepository: "https://github.com/agentclash/agentclash",
    programmingLanguage: ["Go", "TypeScript"],
    runtimePlatform: ["Go", "Node.js", "Next.js"],
    license: "https://opensource.org/licenses/MIT",
    author: publisherSchema(),
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

type BenchmarkReportSchemaInput = {
  slug: string;
  title: string;
  description: string;
  date: string;
  author: string;
  verdict: string;
  results: Array<{ model: string; provider: string }>;
};

// A benchmark report carries both an editorial article and a structured
// comparison. We emit a breadcrumb, a BlogPosting (via articleSchema), and a
// Dataset node — the Dataset is what makes the comparison legible to search
// and answer engines for "benchmark" intent.
export function benchmarkReportSchema(
  report: BenchmarkReportSchemaInput,
): Record<string, unknown>[] {
  const url = `${SITE_URL}/benchmarks/${report.slug}`;
  return [
    breadcrumbSchema([
      { name: "Home", url: "/" },
      { name: "Benchmarks", url: "/benchmarks" },
      { name: report.title, url: `/benchmarks/${report.slug}` },
    ]),
    articleSchema({
      headline: report.title,
      description: report.description,
      url: `/benchmarks/${report.slug}`,
      datePublished: report.date,
      authorName: report.author,
    }),
    {
      "@context": "https://schema.org",
      "@type": "Dataset",
      name: report.title,
      description: report.verdict || report.description,
      url,
      datePublished: report.date,
      keywords: [
        "AI agent benchmark",
        "LLM agent evaluation",
        "same-task model comparison",
        ...report.results.map((result) => result.model),
      ],
      variableMeasured: [
        "Composite score",
        "Correctness",
        "Reliability",
        "Latency",
        "Cost",
        "Cost per correct answer",
      ],
      creator: publisherSchema(),
      publisher: publisherSchema(),
    },
  ];
}

type BenchmarkIndexReport = {
  slug: string;
  title: string;
  description: string;
};

export function benchmarkIndexSchema(
  reports: BenchmarkIndexReport[],
): Record<string, unknown>[] {
  return benchmarkHubSchema(reports);
}

export function benchmarkHubSchema(
  reports: BenchmarkIndexReport[],
  faqItems: Array<{ question: string; answer: string }> = [],
): Record<string, unknown>[] {
  const pageUrl = `${SITE_URL}/benchmarks`;
  const items = reports.map((report) => ({
    url: `${SITE_URL}/benchmarks/${report.slug}`,
    title: report.title,
    description: report.description,
  }));

  const schemas: Record<string, unknown>[] = [
    breadcrumbSchema([
      { name: "Home", url: "/" },
      { name: "Benchmarks", url: "/benchmarks" },
    ]),
    {
      "@context": "https://schema.org",
      "@type": "CollectionPage",
      name: "AgentClash AI Agent Benchmarks",
      description:
        "Public AI agent benchmarks hub with frozen challenge packs, same-task eval runs, replay evidence, scorecards, and monthly reports.",
      url: pageUrl,
      isPartOf: {
        "@type": "WebSite",
        name: "AgentClash",
        url: SITE_URL,
      },
      publisher: publisherSchema(),
    },
  ];

  if (faqItems.length > 0) {
    schemas.push(faqSchema(faqItems));
  }

  if (items.length > 0) {
    schemas.push({
      "@context": "https://schema.org",
      "@type": "ItemList",
      name: "AgentClash Model Benchmarks",
      url: pageUrl,
      numberOfItems: items.length,
      itemListElement: items.map((item, index) => ({
        "@type": "ListItem",
        position: index + 1,
        name: item.title,
        description: item.description,
        url: item.url,
        item: { "@id": item.url },
      })),
    });
  }

  return schemas;
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
