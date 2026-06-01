import type { MetadataRoute } from "next";
import { getAllPosts } from "@/lib/blog";
import {
  getChangelogLatestModified,
  getChangelogPeriodHref,
  getChangelogPeriods,
} from "@/lib/changelog";
import { COMPETITORS } from "@/lib/comparison-data";
import { DOCS_ORIGIN, getAllDocPaths } from "@/lib/docs";

export default function sitemap(): MetadataRoute.Sitemap {
  const posts = getAllPosts().map((post) => ({
    url: `${DOCS_ORIGIN}/blog/${post.slug}`,
    lastModified: new Date(post.date),
    changeFrequency: "monthly" as const,
    priority: 0.7,
  }));
  const docs = getAllDocPaths().map((docPath) => ({
    url: `${DOCS_ORIGIN}${docPath}`,
    lastModified: new Date(),
    changeFrequency: "weekly" as const,
    priority: docPath === "/docs" ? 0.85 : 0.75,
  }));
  const compare = [
    {
      url: `${DOCS_ORIGIN}/compare`,
      lastModified: new Date(),
      changeFrequency: "monthly" as const,
      priority: 0.8,
    },
    ...COMPETITORS.map((competitor) => ({
      url: `${DOCS_ORIGIN}/compare/${competitor.slug}`,
      lastModified: new Date(),
      changeFrequency: "monthly" as const,
      priority: 0.75,
    })),
  ];
  return [
    {
      url: DOCS_ORIGIN,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 1,
    },
    {
      url: `${DOCS_ORIGIN}/blog`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.8,
    },
    {
      url: `${DOCS_ORIGIN}/changelog`,
      lastModified: new Date(getChangelogLatestModified()),
      changeFrequency: "weekly",
      priority: 0.75,
    },
    ...getChangelogPeriods().map((period) => ({
      url: `${DOCS_ORIGIN}${getChangelogPeriodHref(period.id)}`,
      lastModified: new Date(period.endDate),
      changeFrequency: "monthly" as const,
      priority: 0.65,
    })),
    {
      url: `${DOCS_ORIGIN}/why`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.7,
    },
    {
      url: `${DOCS_ORIGIN}/team`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.5,
    },
    {
      url: `${DOCS_ORIGIN}/sitemap`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.5,
    },
    {
      url: `${DOCS_ORIGIN}/platform/agent-evaluation`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.85,
    },
    {
      url: `${DOCS_ORIGIN}/platform/agent-regression-testing`,
      lastModified: new Date(),
      changeFrequency: "monthly",
      priority: 0.82,
    },
    {
      url: `${DOCS_ORIGIN}/try`,
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.88,
    },
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
