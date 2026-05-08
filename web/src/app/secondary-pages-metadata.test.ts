import { describe, expect, it } from "vitest";
import { teamMetadata } from "./team/metadata";
import { whyMetadata } from "./why/metadata";

describe("secondary public page social metadata", () => {
  it("adds explicit social metadata for the why page", () => {
    expect(whyMetadata).toMatchObject({
      title: "Why AgentClash Exists - AI Agent Evaluation for Real Tasks",
      description:
        "Why AgentClash exists: evaluate AI agents on your real tasks with the same tools, same constraints, replayable failures, and regression gates.",
      alternates: {
        canonical: "/why",
      },
      openGraph: {
        title: "Why AgentClash Exists - AI Agent Evaluation for Real Tasks",
        description:
          "Why AgentClash exists: evaluate AI agents on your real tasks with the same tools, same constraints, replayable failures, and regression gates.",
        url: "/why",
        type: "website",
        siteName: "AgentClash",
        images: [
          {
            url: "/og-image.png",
            width: 1200,
            height: 630,
          },
        ],
      },
      twitter: {
        card: "summary_large_image",
        title: "Why AgentClash Exists - AI Agent Evaluation for Real Tasks",
        description:
          "Why AgentClash exists: evaluate AI agents on your real tasks with the same tools, same constraints, replayable failures, and regression gates.",
        images: [
          {
            url: "/twitter-image.png",
          },
        ],
      },
    });

    const openGraph = whyMetadata.openGraph as {
      images?: Array<{ alt?: string }>;
    };
    const twitter = whyMetadata.twitter as {
      images?: Array<{ alt?: string }>;
    };
    expect(openGraph.images?.[0]?.alt).toContain("Why AgentClash");
    expect(twitter.images?.[0]?.alt).toBe(openGraph.images?.[0]?.alt);
  });

  it("adds explicit social metadata for the team page", () => {
    expect(teamMetadata).toMatchObject({
      title: "Team - AgentClash",
      description:
        "Meet the engineers building AgentClash, an open-source AI agent evaluation platform for real-task races, replay, scorecards, and CI gates.",
      alternates: {
        canonical: "/team",
      },
      openGraph: {
        title: "Team - AgentClash",
        description:
          "Meet the engineers building AgentClash, an open-source AI agent evaluation platform for real-task races, replay, scorecards, and CI gates.",
        url: "/team",
        type: "website",
        siteName: "AgentClash",
        images: [
          {
            url: "/og-image.png",
            width: 1200,
            height: 630,
          },
        ],
      },
      twitter: {
        card: "summary_large_image",
        title: "Team - AgentClash",
        description:
          "Meet the engineers building AgentClash, an open-source AI agent evaluation platform for real-task races, replay, scorecards, and CI gates.",
        images: [
          {
            url: "/twitter-image.png",
          },
        ],
      },
    });

    const openGraph = teamMetadata.openGraph as {
      images?: Array<{ alt?: string }>;
    };
    const twitter = teamMetadata.twitter as {
      images?: Array<{ alt?: string }>;
    };
    expect(openGraph.images?.[0]?.alt).toContain("AgentClash team");
    expect(twitter.images?.[0]?.alt).toBe(openGraph.images?.[0]?.alt);
  });
});
