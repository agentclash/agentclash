import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import AgentEvaluationPage, {
  metadata as agentEvaluationMetadata,
} from "./agent-evaluation/page";
import AgentRegressionTestingPage, {
  metadata as agentRegressionTestingMetadata,
} from "./agent-regression-testing/page";

let root: Root | null = null;
let container: HTMLDivElement | null = null;

function render(element: React.ReactElement) {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root?.render(element);
  });
}

function getJsonLd(id: string) {
  const script = container?.querySelector<HTMLScriptElement>(`#${id}`);
  expect(script?.type).toBe("application/ld+json");
  return JSON.parse(script?.textContent ?? "[]") as Array<Record<string, unknown>>;
}

function getSocialImageAlt(metadata: {
  openGraph?: unknown;
  twitter?: unknown;
}) {
  const openGraph = metadata.openGraph as {
    images?: Array<{ alt?: string }>;
  };
  const twitter = metadata.twitter as {
    images?: Array<{ alt?: string }>;
  };

  return {
    ogAlt: openGraph.images?.[0]?.alt,
    twitterAlt: twitter.images?.[0]?.alt,
  };
}

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
});

describe("platform pages structured data", () => {
  it("emits breadcrumb, FAQ, and SoftwareApplication data for agent evaluation", () => {
    render(<AgentEvaluationPage />);

    const jsonLd = getJsonLd("agentclash-platform-agent-evaluation-schema");
    const software = jsonLd.find(
      (item) => item["@type"] === "SoftwareApplication",
    );

    expect(jsonLd.map((item) => item["@type"])).toEqual([
      "BreadcrumbList",
      "FAQPage",
      "SoftwareApplication",
    ]);
    expect(software).toMatchObject({
      name: "AI Agent Evaluation Platform for Real Tasks - AgentClash",
      description:
        "Evaluate AI agents on real tasks with same-tools races, sandboxed execution, replay, scorecards, challenge packs, and CI regression gates.",
      url: "https://www.agentclash.dev/platform/agent-evaluation",
      applicationCategory: "DeveloperApplication",
      applicationSubCategory: "AI agent evaluation platform",
    });
  });

  it("emits breadcrumb, FAQ, and SoftwareApplication data for regression testing", () => {
    render(<AgentRegressionTestingPage />);

    const jsonLd = getJsonLd(
      "agentclash-platform-agent-regression-testing-schema",
    );
    const software = jsonLd.find(
      (item) => item["@type"] === "SoftwareApplication",
    );

    expect(jsonLd.map((item) => item["@type"])).toEqual([
      "BreadcrumbList",
      "FAQPage",
      "SoftwareApplication",
    ]);
    expect(software).toMatchObject({
      name: "AI Agent Regression Testing and CI Gates - AgentClash",
      description:
        "Catch AI agent regressions before release with baseline comparisons, repeatable challenge packs, replay evidence, scorecards, and pull request gates.",
      url: "https://www.agentclash.dev/platform/agent-regression-testing",
      applicationCategory: "DeveloperApplication",
      applicationSubCategory: "AI agent regression testing software",
    });
  });
});

describe("platform page social metadata", () => {
  it("adds explicit social image metadata for agent evaluation", () => {
    const title = "AI Agent Evaluation Platform for Real Tasks - AgentClash";
    const description =
      "Evaluate AI agents on real tasks with same-tools races, sandboxed execution, replay, scorecards, challenge packs, and CI regression gates.";

    expect(agentEvaluationMetadata).toMatchObject({
      title,
      description,
      alternates: {
        canonical: "/platform/agent-evaluation",
      },
      openGraph: {
        title,
        description,
        url: "/platform/agent-evaluation",
        type: "website",
        locale: "en_US",
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
        title,
        description,
        images: [
          {
            url: "/twitter-image.png",
          },
        ],
      },
    });

    const { ogAlt, twitterAlt } = getSocialImageAlt(agentEvaluationMetadata);
    expect(ogAlt).toContain("AgentClash");
    expect(ogAlt).toContain("evaluation platform");
    expect(twitterAlt).toBe(ogAlt);
  });

  it("adds explicit social image metadata for regression testing", () => {
    const title = "AI Agent Regression Testing and CI Gates - AgentClash";
    const description =
      "Catch AI agent regressions before release with baseline comparisons, repeatable challenge packs, replay evidence, scorecards, and pull request gates.";

    expect(agentRegressionTestingMetadata).toMatchObject({
      title,
      description,
      alternates: {
        canonical: "/platform/agent-regression-testing",
      },
      openGraph: {
        title,
        description,
        url: "/platform/agent-regression-testing",
        type: "website",
        locale: "en_US",
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
        title,
        description,
        images: [
          {
            url: "/twitter-image.png",
          },
        ],
      },
    });

    const { ogAlt, twitterAlt } = getSocialImageAlt(
      agentRegressionTestingMetadata,
    );
    expect(ogAlt).toContain("AgentClash");
    expect(ogAlt).toContain("regression testing");
    expect(twitterAlt).toBe(ogAlt);
    expect(ogAlt).not.toBe(getSocialImageAlt(agentEvaluationMetadata).ogAlt);
  });
});
