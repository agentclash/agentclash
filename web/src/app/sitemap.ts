import type { MetadataRoute } from "next";
import { getAllPublicContent, PUBLIC_ORIGIN } from "@/lib/public-content";
import { ogImageUrl } from "@/lib/seo";

function xmlEscape(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function sitemapImage(args: {
  title: string;
  subtitle: string;
  kind: string;
}): string {
  return xmlEscape(
    `${PUBLIC_ORIGIN}${ogImageUrl({
      title: args.title,
      subtitle: args.subtitle,
      kind: args.kind,
    })}`,
  );
}

export default function sitemap(): MetadataRoute.Sitemap {
  return getAllPublicContent()
    .filter((item) => item.indexable && item.includeIn.sitemap)
    .map((item) => {
      const parsedDate = item.lastModified ? new Date(item.lastModified) : undefined;
      return {
        url:
          item.canonicalPath === "/"
            ? PUBLIC_ORIGIN
            : `${PUBLIC_ORIGIN}${item.canonicalPath}`,
        ...(!parsedDate || Number.isNaN(parsedDate.getTime())
          ? {}
          : { lastModified: parsedDate }),
        changeFrequency: item.changeFrequency ?? "monthly",
        priority: item.sitemapPriority ?? 0.7,
        images: [
          sitemapImage({
            title: item.title,
            subtitle: item.description,
            kind: item.kind,
          }),
        ],
      };
    });
}
