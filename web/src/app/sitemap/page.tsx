import type { Metadata } from "next";
import Link from "next/link";
import { DOCS_NAV } from "@/lib/docs";
import { getAllPosts } from "@/lib/blog";
import { SEO_PAGE_REGISTRY } from "@/lib/seo-pages";

export const metadata: Metadata = {
  title: "Sitemap - AgentClash",
  description:
    "Browse AgentClash public pages, AI agent evaluation resources, docs, and blog posts.",
  alternates: {
    canonical: "/sitemap",
  },
  openGraph: {
    title: "Sitemap - AgentClash",
    description:
      "Browse AgentClash public pages, AI agent evaluation resources, docs, and blog posts.",
    url: "/sitemap",
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        alt: "AgentClash sitemap social preview.",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "Sitemap - AgentClash",
    description:
      "Browse AgentClash public pages, AI agent evaluation resources, docs, and blog posts.",
    images: [
      {
        url: "/twitter-image.png",
        alt: "AgentClash sitemap social preview.",
      },
    ],
  },
};

const primaryPages = [
  {
    title: "Home",
    href: "/",
    description: "AgentClash overview and product entry point.",
  },
  {
    title: "Agent evaluation",
    href: "/platform/agent-evaluation",
    description: "Evaluate AI agents on real tasks with replay and scorecards.",
  },
  {
    title: "Agent regression testing",
    href: "/platform/agent-regression-testing",
    description: "Catch AI agent regressions with baseline comparisons and CI gates.",
  },
  {
    title: "Use cases",
    href: "/use-cases",
    description: "Coding, research, and support agent evaluation use cases.",
  },
  {
    title: "Features",
    href: "/features",
    description: "Scorecards, replay, and challenge packs for agent evaluation.",
  },
  {
    title: "Industries",
    href: "/industries",
    description: "Banking, insurance, and government agent evaluation playbooks.",
  },
  {
    title: "Glossary",
    href: "/glossary",
    description: "Definitions for agent evaluation, challenge packs, and release gates.",
  },
  {
    title: "Docs",
    href: "/docs",
    description: "Guides and references for running AgentClash.",
  },
  {
    title: "Blog",
    href: "/blog",
    description: "Engineering notes on AI agent evaluation and release gates.",
  },
  {
    title: "Changelog",
    href: "/changelog",
    description: "Product updates shipped every ten days since launch.",
  },
  {
    title: "Why AgentClash",
    href: "/why",
    description: "Why real-task AI agent evaluation matters.",
  },
  {
    title: "Team",
    href: "/team",
    description: "The engineers building AgentClash.",
  },
  {
    title: "LLMs index",
    href: "/llms.txt",
    description: "Machine-readable index for AI assistants and coding agents.",
  },
  {
    title: "Full LLMs bundle",
    href: "/llms-full.txt",
    description: "Complete machine-readable AgentClash docs bundle.",
  },
];

const seoLandingPages = SEO_PAGE_REGISTRY.map((page) => ({
  title: page.sitemapTitle,
  href: page.path,
  description: page.sitemapDescription,
}));

function LinkList({
  title,
  items,
}: {
  title: string;
  items: Array<{ title: string; href: string; description?: string }>;
}) {
  return (
    <section>
      <h2 className="text-sm font-semibold uppercase tracking-wider text-white/45">
        {title}
      </h2>
      <div className="mt-4 grid gap-3">
        {items.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className="rounded-lg border border-white/[0.08] bg-white/[0.03] px-5 py-4 transition-colors hover:border-white/15"
          >
            <span className="block text-sm font-medium text-white/90">
              {item.title}
            </span>
            {item.description && (
              <span className="mt-1 block text-xs leading-5 text-white/40">
                {item.description}
              </span>
            )}
          </Link>
        ))}
      </div>
    </section>
  );
}

export default function HtmlSitemapPage() {
  const posts = getAllPosts().map((post) => ({
    title: post.title,
    href: `/blog/${post.slug}`,
    description: post.description,
  }));

  return (
    <main className="min-h-screen px-6 py-16 text-white">
      <div className="mx-auto max-w-5xl">
        <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-wider text-white/35">
          Public sitemap
        </p>
        <h1 className="mt-4 font-[family-name:var(--font-display)] text-3xl leading-tight tracking-normal sm:text-5xl">
          AgentClash sitemap
        </h1>
        <p className="mt-4 max-w-2xl text-sm leading-7 text-white/50">
          Browse public AgentClash pages, AI agent evaluation resources, docs,
          and engineering posts.
        </p>

        <div className="mt-12 grid gap-10 lg:grid-cols-2">
          <LinkList title="Core pages" items={primaryPages} />
          <LinkList title="SEO landing pages" items={seoLandingPages} />
        </div>

        <div className="mt-12">
          <LinkList title="Blog posts" items={posts} />
        </div>

        <div className="mt-12 grid gap-10 lg:grid-cols-2">
          {DOCS_NAV.map((section) => (
            <LinkList
              key={section.title}
              title={section.title}
              items={section.items}
            />
          ))}
        </div>
      </div>
    </main>
  );
}
