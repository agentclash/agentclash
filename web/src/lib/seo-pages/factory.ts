import type { Metadata } from "next";
import type { SeoPageConfig } from "./types";

export function createSeoPageMetadata(config: SeoPageConfig): Metadata {
  return {
    title: config.pageTitle,
    description: config.metaDescription,
    alternates: {
      canonical: config.path,
    },
    openGraph: {
      title: config.pageTitle,
      description: config.metaDescription,
      url: config.path,
      type: "website",
      locale: "en_US",
      siteName: "AgentClash",
      images: [
        {
          url: "/og-image.png",
          width: 1200,
          height: 630,
          alt: config.socialImageAlt,
        },
      ],
    },
    twitter: {
      card: "summary_large_image",
      title: config.pageTitle,
      description: config.metaDescription,
      images: [
        {
          url: "/twitter-image.png",
          alt: config.socialImageAlt,
        },
      ],
    },
  };
}
