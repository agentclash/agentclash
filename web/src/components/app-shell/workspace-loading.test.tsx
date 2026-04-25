import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { WorkspaceDetailLoading, WorkspaceListLoading } from "./workspace-loading";

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

describe("workspace loading shells", () => {
  it("renders list-page skeleton rows", () => {
    render(<WorkspaceListLoading rows={4} showTabs actionCount={2} />);

    const skeletons = container?.querySelectorAll('[data-slot="skeleton"]');
    expect(skeletons?.length).toBeGreaterThanOrEqual(10);
  });

  it("renders heavy-detail skeleton sections", () => {
    render(<WorkspaceDetailLoading />);

    const skeletons = container?.querySelectorAll('[data-slot="skeleton"]');
    expect(skeletons?.length).toBeGreaterThanOrEqual(8);
  });
});
