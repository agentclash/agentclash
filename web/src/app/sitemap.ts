import type { MetadataRoute } from "next";
import { getAllPosts } from "@/lib/blog";
import { getAllDocPaths } from "@/lib/docs";

export default function sitemap(): MetadataRoute.Sitemap {
  const posts = getAllPosts().map((post) => ({
    url: `https://agentclash.dev/blog/${post.slug}`,
    lastModified: new Date(post.date),
    changeFrequency: "monthly" as const,
    priority: 0.7,
  }));
  const docs = getAllDocPaths().map((docPath) => ({
    url: `https://agentclash.dev${docPath}`,
    lastModified: new Date(),
    changeFrequency: "weekly" as const,
    priority: docPath === "/docs" ? 0.85 : 0.75,
  }));

  return [
    {
      url: "https://agentclash.dev",
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 1,
    },
    {
      url: "https://agentclash.dev/blog",
      lastModified: new Date(),
      changeFrequency: "weekly",
      priority: 0.8,
    },
    ...docs,
    ...posts,
  ];
}
