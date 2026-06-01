import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { SITE_URL } from "@/components/marketing/json-ld";
import {
  getChangelogLatestModified,
  getChangelogPeriods,
} from "@/lib/changelog";
import ChangelogPage from "./page";

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

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
});

describe("changelog index structured data", () => {
  it("renders breadcrumb, WebPage, ItemList, and FAQ JSON-LD", () => {
    const periods = getChangelogPeriods();
    const latestPeriod = periods[0];

    render(<ChangelogPage />);

    const script = container?.querySelector<HTMLScriptElement>(
      "#agentclash-changelog-index-schema",
    );
    expect(script?.type).toBe("application/ld+json");

    const jsonLd = JSON.parse(script?.textContent ?? "[]") as Array<
      Record<string, unknown>
    >;

    expect(jsonLd.map((item) => item["@type"])).toEqual([
      "BreadcrumbList",
      "WebPage",
      "ItemList",
      "FAQPage",
    ]);
    expect(jsonLd[1]).toMatchObject({
      "@type": "WebPage",
      name: "AgentClash Changelog",
      url: `${SITE_URL}/changelog`,
      dateModified: getChangelogLatestModified(),
    });
    expect(jsonLd[2]).toMatchObject({
      "@type": "ItemList",
      numberOfItems: periods.length,
    });
    expect(
      (jsonLd[2]?.itemListElement as Array<Record<string, unknown>>)[0],
    ).toMatchObject({
      "@type": "ListItem",
      position: 1,
      url: `${SITE_URL}/changelog/${latestPeriod?.id}`,
    });
    expect(jsonLd[3]).toMatchObject({
      "@type": "FAQPage",
    });
    expect(
      (jsonLd[3]?.mainEntity as Array<Record<string, unknown>>)[0],
    ).toMatchObject({
      "@type": "Question",
      name: "What is the AgentClash changelog?",
    });
  });
});
