import type { MetadataRoute } from "next";
import { getAllPosts } from "@/lib/blog";
import {
  getChangelogLatestModified,
  getChangelogPeriodHref,
  getChangelogPeriods,
} from "@/lib/changelog";
import { COMPETITORS } from "@/lib/comparison-data";
import { DOCS_ORIGIN, getAllDocPaths } from "@/lib/docs";
import { SEO_PAGE_REGISTRY, seoPageSitemapPriority } from "@/lib/seo-pages";
import { ogImageUrl } from "@/lib/seo";

// `MetadataRoute.Sitemap` `images` entries must be ABSOLUTE URLs, and the
// metadataBase that upgrades ogImageUrl()'s root-relative `/og` path inside page
// metadata does NOT apply to sitemap entries — so prefix DOCS_ORIGIN here. Each
// image points at the existing dynamic OG card route, reusing the same cached
// card a page already renders for its social preview.
//
// Critical: Next's sitemap serializer interpolates the image URL into
// `<image:loc>…</image:loc>` WITHOUT XML-escaping, and ogImageUrl() joins query
// params with a raw `&` (URLSearchParams). A raw `&` is not well-formed XML and
// would corrupt the entire sitemap.xml, so XML-escape the URL here. This matters
// only for the sitemap; page metadata uses ogImageUrl() unescaped. (The `url`
// field needs no escaping — route paths carry no query string / XML-special
// chars.)
function xmlEscape(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function ogImage(args: {
  title: string;
  subtitle?: string;
  kind?: string;
}): string {
  return xmlEscape(`${DOCS_ORIGIN}${ogImageUrl(args)}`);
}

// Shared generic card for pages without a tailored OG image — one cached render.
const DEFAULT_OG = ogImage({
  title: "AgentClash",
  subtitle: "Open-source AI agent evaluation platform",
});

export default function sitemap(): MetadataRoute.Sitemap {
  const posts = getAllPosts().map((post) => ({
    url: `${DOCS_ORIGIN}/blog/${post.slug}`,
    lastModified: new Date(post.date),
    changeFrequency: "monthly" as const,
    priority: 0.7,
    images: [
      ogImage({ title: post.title, subtitle: post.description, kind: "Blog" }),
    ],
  }));
  const docs = getAllDocPaths().map((docPath) => ({
    url: `${DOCS_ORIGIN}${docPath}`,
    lastModified: new Date(),
    changeFrequency: "weekly" as const,
    priority: docPath === "/docs" ? 0.85 : 0.75,
    images: [ogImage({ title: "AgentClash Docs", kind: "Docs" })],
  }));
  const compare = [
    {
      url: `${DOCS_ORIGIN}/compare`,
      lastModified: new Date(),
      changeFrequency: "monthly" as const,
      priority: 0.8,
      images: [
        ogImage({
          title: "AgentClash vs prompt-eval tools",
          subtitle: "Agent evaluation, not prompt evaluation",
          kind: "Compare",
        }),
      ],
    },
    ...COMPETITORS.map((competitor) => ({
      url: `${DOCS_ORIGIN}/compare/${competitor.slug}`,
      lastModified: new Date(),
      changeFrequency: "monthly" as const,
      priority: 0.75,
      images: [
        ogImage({
          title: `AgentClash vs ${competitor.name}`,
          subtitle: "Agent eval vs prompt eval",
          kind: "Compare",
        }),
      ],
    })),
  ];
  return [
    {
      url: DOCS_ORIGIN,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 1,
      images: [DEFAULT_OG],
    },
    {
      url: `${DOCS_ORIGIN}/blog`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.8,
      images: [DEFAULT_OG],
    },
    {
      url: `${DOCS_ORIGIN}/changelog`,
      lastModified: new Date(getChangelogLatestModified()),
      changeFrequency: "weekly",
      priority: 0.75,
      images: [DEFAULT_OG],
    },
    ...getChangelogPeriods().map((period) => ({
      url: `${DOCS_ORIGIN}${getChangelogPeriodHref(period.id)}`,
      lastModified: new Date(period.endDate),
      changeFrequency: "monthly" as const,
      priority: 0.65,
      images: [DEFAULT_OG],
    })),
    {
      url: `${DOCS_ORIGIN}/why`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.7,
      images: [DEFAULT_OG],
    },
    {
      url: `${DOCS_ORIGIN}/team`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.5,
      images: [DEFAULT_OG],
    },
    {
      url: `${DOCS_ORIGIN}/sitemap`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.5,
      images: [DEFAULT_OG],
    },
    {
      url: `${DOCS_ORIGIN}/pricing`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.8,
      images: [
        ogImage({
          title: "AgentClash Pricing",
          subtitle: "Free and open-source, with hosted Pro, Team & Enterprise",
          kind: "Pricing",
        }),
      ],
    },
    {
      url: `${DOCS_ORIGIN}/platform/agent-evaluation`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.85,
      images: [
        ogImage({ title: "AI Agent Evaluation Platform", kind: "Platform" }),
      ],
    },
    {
      url: `${DOCS_ORIGIN}/platform/agent-regression-testing`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.82,
      images: [
        ogImage({ title: "AI Agent Regression Testing", kind: "Platform" }),
      ],
    },
    ...SEO_PAGE_REGISTRY.map((page) => ({
      url: `${DOCS_ORIGIN}${page.path}`,
      lastModified: new Date(),
      changeFrequency: "monthly" as const,
      priority: seoPageSitemapPriority(page.tier),
      images: [ogImage({ title: page.sitemapTitle, kind: "SEO" })],
    })),
    {
      url: `${DOCS_ORIGIN}/try`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.88,
      images: [DEFAULT_OG],
    },
    // .txt endpoints are intentionally imageless — they are not HTML pages.
    {
      url: `${DOCS_ORIGIN}/llms.txt`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.6,
    },
    {
      url: `${DOCS_ORIGIN}/llms-full.txt`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.55,
    },
    ...compare,
    ...docs,
    ...posts,
  ];
}
