import type { Metadata } from "next";
import { ogImageUrl } from "@/lib/seo";

const PAGE_TITLE = "Changelog - AgentClash";
const PAGE_DESCRIPTION =
  "Everything shipped in AgentClash from day one — scoring, regression, security packs, datasets, and CLI updates, grouped every ten days.";
const SOCIAL_IMAGE_ALT = "AgentClash product changelog social preview.";
const SOCIAL_IMAGE = ogImageUrl({
  title: "AgentClash Changelog",
  subtitle: "Product updates every ten days since launch",
  kind: "Changelog",
});

export const changelogMetadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  keywords: [
    "AgentClash changelog",
    "AgentClash release notes",
    "AI agent evaluation updates",
    "agent eval platform changelog",
    "AgentClash product updates",
    "what's new AgentClash",
  ],
  alternates: {
    canonical: "/changelog",
  },
  robots: {
    index: true,
    follow: true,
  },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: "/changelog",
    type: "website",
    locale: "en_US",
    siteName: "AgentClash",
    images: [
      {
        url: SOCIAL_IMAGE,
        width: 1200,
        height: 630,
        alt: SOCIAL_IMAGE_ALT,
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    images: [
      {
        url: SOCIAL_IMAGE,
        alt: SOCIAL_IMAGE_ALT,
      },
    ],
  },
};
