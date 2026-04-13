import type { Metadata } from "next";
import Link from "next/link";
import { getAllPosts } from "@/lib/blog";

export const metadata: Metadata = {
  title: "Blog — AgentClash",
  description: "Engineering notes from the AgentClash team.",
};

export default function BlogPage() {
  const posts = getAllPosts();

  return (
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
  );
}
