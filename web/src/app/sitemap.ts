import type { MetadataRoute } from "next";
import { getAllPosts } from "@/lib/blog";
import { DOCS_ORIGIN, getAllDocMarkdownPaths, getAllDocPaths } from "@/lib/docs";

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
  const markdownDocs = getAllDocMarkdownPaths().map((docPath) => ({
    url: `${DOCS_ORIGIN}${docPath}`,
    lastModified: new Date(),
    changeFrequency: "weekly" as const,
    priority: docPath === "/docs-md" ? 0.5 : 0.4,
  }));

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
    ...docs,
    ...markdownDocs,
    ...posts,
  ];
}
