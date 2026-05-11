import type { Metadata } from "next";

type AlternateTypes = NonNullable<NonNullable<Metadata["alternates"]>["types"]>;

export const blogRssAlternate = {
  "application/rss+xml": [
    {
      title: "AgentClash Blog",
      url: "/feed.xml",
    },
  ],
} satisfies AlternateTypes;
