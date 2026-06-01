import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MDXRemote } from "next-mdx-remote/rsc";
import {
  JsonLd,
  articleSchema,
  breadcrumbSchema,
} from "@/components/marketing/json-ld";
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

  return (
    <main className="min-h-screen flex flex-col items-center px-6 py-16">
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
      <article className="w-full max-w-lg">
        <header className="mb-8">
          <p className="font-[family-name:var(--font-mono)] text-[11px] text-white/20">
            {post.date} &middot; {post.author}
          </p>
          <h1 className="mt-2 font-[family-name:var(--font-display)] text-2xl sm:text-3xl tracking-[-0.02em] leading-[1.15]">
            {post.title}
          </h1>
        </header>

        <div className="prose-agentclash">
          <MDXRemote source={post.content} options={mdxRemoteOptions} />
        </div>
      </article>

      <Link
        href="/blog"
        className="mt-12 text-xs text-white/30 hover:text-white/50 transition-colors"
      >
        &larr; All posts
      </Link>
    </main>
  );
}
