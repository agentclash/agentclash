import type { Metadata } from "next";
import Link from "next/link";
import { JsonLd, blogIndexSchema } from "@/components/marketing/json-ld";
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
    <>
      <JsonLd id="agentclash-blog-index-schema" data={blogIndexSchema(posts)} />
      <main className="min-h-screen flex flex-col items-center px-6 py-16">
        <h1 className="font-[family-name:var(--font-display)] text-3xl sm:text-4xl text-center tracking-[-0.02em] leading-[1.15]">
          Blog
        </h1>
        <p className="mt-3 text-sm text-white/25">
          Engineering notes from the team.
        </p>

        <div className="mt-10 w-full max-w-lg flex flex-col gap-3">
          {posts.map((post) => (
            <Link
              key={post.slug}
              href={`/blog/${post.slug}`}
              className="group flex flex-col gap-1 rounded-lg border border-white/[0.08] bg-white/[0.03] px-5 py-4 hover:border-white/15 transition-colors"
            >
              <span className="font-[family-name:var(--font-mono)] text-[11px] text-white/40">
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

        <Link
          href="/"
          className="mt-10 text-xs text-white/30 hover:text-white/50 transition-colors"
        >
          &larr; Back to AgentClash
        </Link>
      </main>
    </>
  );
}
