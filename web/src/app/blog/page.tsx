import type { Metadata } from "next";
import Link from "next/link";
import { JsonLd, blogIndexSchema } from "@/components/marketing/json-ld";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { AgentPromoBanner } from "@/components/marketing/agent-promo-banner";
import { ResearchAudienceCTA } from "@/components/marketing/research-audience-cta";
import { ResourcePackCTA } from "@/components/marketing/resource-pack-cta";
import { getAllPosts } from "@/lib/blog";
import { blogRssAlternate } from "@/lib/seo";

const PAGE_TITLE = "AI Agent Evaluation Blog - AgentClash";
const PAGE_DESCRIPTION =
  "Engineering notes on AI agent evaluation, head-to-head agent evals, replayable failures, scorecards, and CI regression gates.";
const SOCIAL_IMAGE_ALT =
  "AgentClash AI agent evaluation blog social preview.";

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  alternates: {
    canonical: "/blog",
    types: blogRssAlternate,
  },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: "/blog",
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        alt: SOCIAL_IMAGE_ALT,
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    images: [
      {
        url: "/twitter-image.png",
        alt: SOCIAL_IMAGE_ALT,
      },
    ],
  },
};

export default function BlogPage() {
  const posts = getAllPosts();

  return (
    <MarketingShell banner={<AgentPromoBanner page="blog" />}>
      <JsonLd id="agentclash-blog-index-schema" data={blogIndexSchema(posts)} />
      <section className="mx-auto w-full max-w-3xl px-6 py-16">
        <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.14em] text-white/35">
          Blog
        </p>
        <h1 className="mt-3 font-[family-name:var(--font-display)] text-3xl tracking-[-0.02em] leading-[1.15] sm:text-4xl">
          AI agent evaluation, in practice
        </h1>
        <p className="mt-3 max-w-xl text-sm leading-relaxed text-white/45">
          Engineering notes on head-to-head agent evals, replayable failures,
          scorecards, and release gates — for teams deciding which agents to
          ship.
        </p>

        <ResearchAudienceCTA className="mt-8" />
        <ResourcePackCTA className="mt-4" compact />

        <div className="mt-10 flex flex-col gap-3">
          {posts.map((post) => (
            <Link
              key={post.slug}
              href={`/blog/${post.slug}`}
              className="group flex flex-col gap-1 rounded-lg border border-white/[0.08] bg-white/[0.03] px-5 py-4 hover:border-white/15 transition-colors"
            >
              <span className="font-[family-name:var(--font-mono)] text-2xs text-white/40">
                {post.date} &middot; {post.author}
              </span>
              <span className="text-sm font-medium text-white group-hover:text-white/90">
                {post.title}
              </span>
              <span className="text-xs text-white/35 leading-relaxed">
                {post.description}
              </span>
            </Link>
          ))}
        </div>

        {posts.length === 0 && (
          <p className="mt-10 text-xs text-white/20">No posts yet.</p>
        )}
      </section>
    </MarketingShell>
  );
}
