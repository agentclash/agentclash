import { describe, expect, it } from "vitest";
import { productSchema, SITE_URL } from "./json-ld";

describe("productSchema", () => {
  it("builds SoftwareApplication structured data with absolute URLs", () => {
    expect(
      productSchema({
        name: "AI Agent Evaluation Platform for Real Tasks - AgentClash",
        description: "Evaluate AI agents on real tasks.",
        url: "/platform/agent-evaluation",
        applicationSubCategory: "AI agent evaluation platform",
      }),
    ).toMatchObject({
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      name: "AI Agent Evaluation Platform for Real Tasks - AgentClash",
      alternateName: "Agent Clash",
      applicationCategory: "DeveloperApplication",
      applicationSubCategory: "AI agent evaluation platform",
      operatingSystem: "Web, macOS, Linux, Windows",
      description: "Evaluate AI agents on real tasks.",
      url: `${SITE_URL}/platform/agent-evaluation`,
      offers: {
        "@type": "Offer",
        price: "0",
        priceCurrency: "USD",
      },
    });
  });
});
