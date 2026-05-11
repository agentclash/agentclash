import { describe, expect, it } from "vitest";
import type { BlogPost } from "./blog";
import { buildBlogRssFeed, escapeXml, formatRssDate } from "./rss";

const posts = [
  {
    slug: "older-post",
    title: "Older Post",
    date: "2026-04-01",
    description: "Earlier release note.",
    author: "Team",
  },
  {
    slug: "agent-evaluation",
    title: `Agent Eval & "Replay"`,
    date: "2026-05-07",
    description: "Compare <agents> & ship the reliable one.",
    author: "Atharva",
  },
] satisfies BlogPost[];

describe("rss feed", () => {
  it("escapes XML entities", () => {
    expect(escapeXml(`A & B <C> "D" 'E'`)).toBe(
      "A &amp; B &lt;C&gt; &quot;D&quot; &apos;E&apos;",
    );
  });

  it("formats calendar dates and ISO timestamps as RSS dates", () => {
    expect(formatRssDate("2026-05-07")).toBe(
      "Thu, 07 May 2026 00:00:00 GMT",
    );
    expect(formatRssDate("2026-05-07T18:30:00.000Z")).toBe(
      "Thu, 07 May 2026 18:30:00 GMT",
    );
  });

  it("builds a blog RSS feed with provided posts", () => {
    const feed = buildBlogRssFeed("https://example.test", posts);

    expect(feed).toContain('<?xml version="1.0" encoding="UTF-8"?>');
    expect(feed).toContain('<rss version="2.0"');
    expect(feed).toContain("<title>AgentClash Blog</title>");
    expect(feed).toContain("<link>https://example.test/blog</link>");
    expect(feed).toContain(
      '<atom:link href="https://example.test/feed.xml" rel="self" type="application/rss+xml" />',
    );
    expect(feed).toContain(
      "<title>Agent Eval &amp; &quot;Replay&quot;</title>",
    );
    expect(feed).toContain(
      "<link>https://example.test/blog/agent-evaluation</link>",
    );
    expect(feed).toContain(
      '<guid isPermaLink="true">https://example.test/blog/agent-evaluation</guid>',
    );
    expect(feed).toContain("<pubDate>Thu, 07 May 2026 00:00:00 GMT</pubDate>");
    expect(feed).toContain("<dc:creator>Atharva</dc:creator>");
    expect(feed).toContain(
      "<description>Compare &lt;agents&gt; &amp; ship the reliable one.</description>",
    );
  });

  it("omits posts with invalid or incomplete metadata", () => {
    const feed = buildBlogRssFeed("https://example.test", [
      posts[0],
      { ...posts[0], slug: "invalid-date", date: "not-a-date" },
      {
        ...posts[0],
        slug: "missing-title",
        title: undefined as unknown as string,
      },
    ]);

    expect(feed).toContain("<title>Older Post</title>");
    expect(feed).not.toContain("invalid-date");
    expect(feed).not.toContain("missing-title");
  });
});
