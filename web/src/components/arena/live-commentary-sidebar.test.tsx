import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it } from "vitest";

import {
  MAX_COMMENTARY_ENTRIES,
  type CommentaryEntry,
} from "@/hooks/use-agent-commentary";

import { LiveCommentarySidebar } from "./live-commentary-sidebar";

function makeEntry(index: number): CommentaryEntry {
  return {
    id: `entry-${index}`,
    occurredAt: `2026-04-22T23:15:${String(index).padStart(2, "0")}Z`,
    agentId: "agent-1",
    agentLabel: "Alpha",
    line: `Commentary line ${index}`,
    tone: "neutral",
  };
}

function renderSidebar(entries: CommentaryEntry[]) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(
      React.createElement(LiveCommentarySidebar, {
        entries,
        isActive: true,
      }),
    );
  });

  return {
    container,
    cleanup() {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
  };
}

describe("LiveCommentarySidebar", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  it("renders every available entry up to the bounded commentary limit", () => {
    const entries = Array.from(
      { length: MAX_COMMENTARY_ENTRIES },
      (_, index) => makeEntry(index),
    );

    const { container, cleanup } = renderSidebar(entries);

    expect(container.querySelectorAll("article")).toHaveLength(
      MAX_COMMENTARY_ENTRIES,
    );
    expect(container.textContent).toContain(
      `Commentary line ${MAX_COMMENTARY_ENTRIES - 1}`,
    );

    cleanup();
  });

  it("formats backend timestamps in UTC and labels them clearly", () => {
    const { container, cleanup } = renderSidebar([makeEntry(3)]);

    expect(container.textContent).toContain("23:15:03 UTC");

    cleanup();
  });
});
