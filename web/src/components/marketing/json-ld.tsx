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
}: {
  name: string;
  description: string;
  url: string;
  applicationSubCategory?: string;
  softwareVersion?: string;
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
    offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
  };
}

export function publisherSchema(): Record<string, unknown> {
  return {
    "@type": "Organization",
    name: "AgentClash",
    logo: {
      "@type": "ImageObject",
      url: `${SITE_URL}/icon.svg`,
    },
  };
}

export function articleSchema({
  headline,
  description,
  url,
  datePublished,
  authorName,
}: {
  headline: string;
  description: string;
  url: string;
  datePublished: string;
  authorName: string;
}): Record<string, unknown> {
  const absoluteUrl = url.startsWith("http") ? url : `${SITE_URL}${url}`;

  return {
    "@context": "https://schema.org",
    "@type": "BlogPosting",
    headline,
    description,
    url: absoluteUrl,
    mainEntityOfPage: absoluteUrl,
    datePublished,
    dateModified: datePublished,
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
