import { getAllPosts, type BlogPost } from "./blog";
import { getAllReports, type BenchmarkReport } from "./benchmarks";

const SITE_URL = "https://www.agentclash.dev";

const XML_ENTITIES: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&apos;",
};

export function escapeXml(value: string) {
  return value.replace(/[&<>"']/g, (char) => XML_ENTITIES[char] ?? char);
}

function parseRssDate(date: string) {
  const rssDate = /^\d{4}-\d{2}-\d{2}$/.test(date)
    ? new Date(`${date}T00:00:00.000Z`)
    : new Date(date);

  return Number.isNaN(rssDate.getTime()) ? null : rssDate;
}

export function formatRssDate(date: string) {
  const rssDate = parseRssDate(date);

  if (!rssDate) {
    throw new Error(`Invalid blog post date: ${date}`);
  }

  return rssDate.toUTCString();
}

function requiredText(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function toRssPost(post: BlogPost) {
  const slug = requiredText(post.slug);
  const title = requiredText(post.title);
  const date = requiredText(post.date);
  const description = requiredText(post.description);
  const author = requiredText(post.author);
  const rssDate = parseRssDate(date);

  if (!slug || !title || !date || !description || !author || !rssDate) {
    return null;
  }

  return {
    slug,
    title,
    description,
    author,
    pubDate: rssDate.toUTCString(),
    publishedAtMs: rssDate.getTime(),
  };
}

export function buildBlogRssFeed(
  origin = SITE_URL,
  posts: BlogPost[] = getAllPosts(),
) {
  const rssPosts = posts
    .map(toRssPost)
    .filter((post) => post !== null)
    .sort((a, b) => b.publishedAtMs - a.publishedAtMs);
  const blogUrl = `${origin}/blog`;
  const feedUrl = `${origin}/feed.xml`;
  const lastBuildDate = rssPosts[0]?.pubDate ?? new Date().toUTCString();

  const items = rssPosts.map((post) => {
    const url = `${origin}/blog/${post.slug}`;

    return [
      "    <item>",
      `      <title>${escapeXml(post.title)}</title>`,
      `      <link>${escapeXml(url)}</link>`,
      `      <guid isPermaLink="true">${escapeXml(url)}</guid>`,
      `      <pubDate>${post.pubDate}</pubDate>`,
      `      <dc:creator>${escapeXml(post.author)}</dc:creator>`,
      `      <description>${escapeXml(post.description)}</description>`,
      "    </item>",
    ].join("\n");
  });

  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/elements/1.1/">',
    "  <channel>",
    "    <title>AgentClash Blog</title>",
    `    <link>${escapeXml(blogUrl)}</link>`,
    `    <atom:link href="${escapeXml(feedUrl)}" rel="self" type="application/rss+xml" />`,
    "    <description>Engineering notes on AI agent evaluation, replayable failures, scorecards, and CI regression gates.</description>",
    "    <language>en</language>",
    `    <lastBuildDate>${lastBuildDate}</lastBuildDate>`,
    ...items,
    "  </channel>",
    "</rss>",
  ].join("\n");
}

function toRssBenchmark(report: BenchmarkReport) {
  const slug = requiredText(report.slug);
  const title = requiredText(report.title);
  const date = requiredText(report.date);
  const description = requiredText(report.description);
  const author = requiredText(report.author);
  const rssDate = parseRssDate(date);

  if (!slug || !title || !date || !description || !author || !rssDate) {
    return null;
  }

  return {
    slug,
    title,
    description,
    author,
    pubDate: rssDate.toUTCString(),
    publishedAtMs: rssDate.getTime(),
  };
}

export function buildBenchmarkRssFeed(
  origin = SITE_URL,
  reports: BenchmarkReport[] = getAllReports(),
) {
  const rssReports = reports
    .map(toRssBenchmark)
    .filter((report) => report !== null)
    .sort((a, b) => b.publishedAtMs - a.publishedAtMs);
  const indexUrl = `${origin}/benchmarks`;
  const feedUrl = `${origin}/benchmarks/feed.xml`;
  const lastBuildDate = rssReports[0]?.pubDate ?? new Date().toUTCString();

  const items = rssReports.map((report) => {
    const url = `${origin}/benchmarks/${report.slug}`;

    return [
      "    <item>",
      `      <title>${escapeXml(report.title)}</title>`,
      `      <link>${escapeXml(url)}</link>`,
      `      <guid isPermaLink="true">${escapeXml(url)}</guid>`,
      `      <pubDate>${report.pubDate}</pubDate>`,
      `      <dc:creator>${escapeXml(report.author)}</dc:creator>`,
      `      <description>${escapeXml(report.description)}</description>`,
      "    </item>",
    ].join("\n");
  });

  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/elements/1.1/">',
    "  <channel>",
    "    <title>AgentClash Model Benchmarks</title>",
    `    <link>${escapeXml(indexUrl)}</link>`,
    `    <atom:link href="${escapeXml(feedUrl)}" rel="self" type="application/rss+xml" />`,
    "    <description>Head-to-head AI agent benchmarks — new models raced against the field on real agentic tasks.</description>",
    "    <language>en</language>",
    `    <lastBuildDate>${lastBuildDate}</lastBuildDate>`,
    ...items,
    "  </channel>",
    "</rss>",
  ].join("\n");
}
