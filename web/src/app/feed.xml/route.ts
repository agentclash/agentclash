import { buildBlogRssFeed } from "@/lib/rss";

export const revalidate = 3600;

export function GET() {
  return new Response(buildBlogRssFeed(), {
    headers: {
      "Cache-Control": "public, max-age=3600, stale-while-revalidate=86400",
      "Content-Type": "application/rss+xml; charset=utf-8",
    },
  });
}
