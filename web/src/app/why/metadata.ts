import type { Metadata } from "next";

const PAGE_TITLE = "Why AgentClash Exists - AI Agent Evaluation for Real Tasks";
const PAGE_DESCRIPTION =
  "Why AgentClash exists: evaluate AI agents on your real tasks with the same tools, same constraints, replayable failures, and regression gates.";
const SOCIAL_IMAGE_ALT = "Why AgentClash exists social preview.";

export const whyMetadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  alternates: {
    canonical: "/why",
  },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: "/why",
    type: "website",
    siteName: "AgentClash",
    images: [
      {
        url: "/og-image.png",
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
        url: "/twitter-image.png",
        alt: SOCIAL_IMAGE_ALT,
      },
    ],
  },
};
