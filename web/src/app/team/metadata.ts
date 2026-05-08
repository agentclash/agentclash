import type { Metadata } from "next";

const PAGE_TITLE = "Team - AgentClash";
const PAGE_DESCRIPTION =
  "Meet the engineers building AgentClash, an open-source AI agent evaluation platform for real-task races, replay, scorecards, and CI gates.";
const SOCIAL_IMAGE_ALT = "AgentClash team social preview.";

export const teamMetadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  alternates: {
    canonical: "/team",
  },
  openGraph: {
    title: PAGE_TITLE,
    description: PAGE_DESCRIPTION,
    url: "/team",
    type: "website",
    locale: "en_US",
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
