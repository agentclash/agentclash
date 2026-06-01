import { describe, expect, it } from "vitest";
import { COMPETITORS } from "@/lib/comparison-data";
import { metadata as compareHubMetadata } from "./page";
import {
  generateMetadata as generateCompetitorMetadata,
  generateStaticParams,
} from "./[competitor]/page";

describe("compare hub metadata", () => {
  it("locks the canonical and emits a dynamic OG image", () => {
    expect(compareHubMetadata.alternates).toMatchObject({
      canonical: "/compare",
    });

    const openGraph = compareHubMetadata.openGraph as {
      images?: Array<{ url?: string }>;
    };
    expect(openGraph.images?.[0]?.url?.startsWith("/og?")).toBe(true);
  });
});

describe("compare competitor pages", () => {
  it("generates one static param per competitor", () => {
    const params = generateStaticParams();

    expect(params).toHaveLength(COMPETITORS.length);
    expect(params).toContainEqual({ competitor: "agentclash-vs-langsmith" });
  });

  it("locks per-competitor canonical metadata and titles", async () => {
    const metadata = await generateCompetitorMetadata({
      params: Promise.resolve({ competitor: "agentclash-vs-langsmith" }),
    });

    expect(metadata.alternates).toMatchObject({
      canonical: "/compare/agentclash-vs-langsmith",
    });
    expect(metadata.title).toContain("LangSmith");

    const openGraph = metadata.openGraph as {
      images?: Array<{ url?: string }>;
    };
    expect(openGraph.images?.[0]?.url?.startsWith("/og?")).toBe(true);
  });

  it("returns empty metadata for an unknown competitor", async () => {
    const metadata = await generateCompetitorMetadata({
      params: Promise.resolve({ competitor: "agentclash-vs-unknown" }),
    });

    expect(metadata).toEqual({});
  });
});
