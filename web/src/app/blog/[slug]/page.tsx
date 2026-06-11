import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MDXRemote } from "next-mdx-remote/rsc";
import {
  JsonLd,
  articleSchema,
  breadcrumbSchema,
} from "@/components/marketing/json-ld";
import { BlogRelatedResources } from "@/components/marketing/blog-related-resources";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { ResearchAudienceCTA } from "@/components/marketing/research-audience-cta";
import { ResourcePackCTA } from "@/components/marketing/resource-pack-cta";
import { getBlogRelatedResources } from "@/lib/blog-related-resources";
import { getAllSlugs, getPostBySlug } from "@/lib/blog";
import { mdxRemoteOptions } from "@/lib/mdx-options";
import { blogRssAlternate, ogImageUrl } from "@/lib/seo";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return getAllSlugs().map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post) return {};
  const title = `${post.title} — AgentClash`;
  const imageAlt = `${post.title} — ${post.description}`;
  const ogImage = ogImageUrl({
    title: post.title,
    subtitle: post.description,
    kind: "Blog",
  });

  return {
    title,
    description: post.description,
    alternates: {
      canonical: `/blog/${post.slug}`,
      types: blogRssAlternate,
    },
    openGraph: {
      type: "article",
      title,
      description: post.description,
      url: `/blog/${post.slug}`,
      locale: "en_US",
      siteName: "AgentClash",
      publishedTime: post.date,
      authors: [post.author],
      images: [
        {
          url: ogImage,
          width: 1200,
          height: 630,
          alt: imageAlt,
        },
      ],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description: post.description,
      images: [
        {
          url: ogImage,
          alt: imageAlt,
        },
      ],
    },
  };
}

export default async function BlogPostPage({ params }: Props) {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post) notFound();
  const relatedResources = getBlogRelatedResources(slug);

  return (
    <MarketingShell>
      <JsonLd
        id={`agentclash-blog-${post.slug}-schema`}
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: "Blog", url: "/blog" },
            { name: post.title, url: `/blog/${post.slug}` },
          ]),
          articleSchema({
            headline: post.title,
            description: post.description,
            url: `/blog/${post.slug}`,
            datePublished: post.date,
            authorName: post.author,
          }),
        ]}
      />
      <article className="mx-auto w-full max-w-3xl px-6 py-16">
        <Link
          href="/blog"
          className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35 transition-colors hover:text-white/55"
        >
          &larr; Blog
        </Link>

        <header className="mt-6 mb-8">
          <p className="font-[family-name:var(--font-mono)] text-[11px] text-white/40">
            {post.date} &middot; {post.author}
          </p>
          <h1 className="mt-3 font-[family-name:var(--font-display)] text-3xl tracking-[-0.02em] leading-[1.15] sm:text-4xl">
            {post.title}
          </h1>
        </header>

        <div className="prose-agentclash">
          <MDXRemote source={post.content} options={mdxRemoteOptions} />
        </div>

        <ResourcePackCTA className="mt-12" />

        <BlogRelatedResources links={relatedResources} />

        <ResearchAudienceCTA
          className="mt-12"
          headline="Ready to evaluate agents on your workloads?"
          body="Book an eval workshop to map your agents to challenge packs, baselines, and release gates. Or see fixed-scope eval services and the enterprise pilot."
          secondaryHref="/services"
          secondaryLabel="Eval services"
        />
      </article>
    </MarketingShell>
  );
}
