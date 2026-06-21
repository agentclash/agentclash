import { describe, expect, it } from "vitest";

import { INPUTS_NODE_ID } from "../lib/graph";
import { CANVAS_TOUR_ID, canvasTourSteps } from "./tour-steps";

describe("canvas tour steps", () => {
  it("has a stable tour id", () => {
    expect(CANVAS_TOUR_ID).toBe("tools-canvas");
  });

  it("every step has copy a non-engineer can read", () => {
    expect(canvasTourSteps.length).toBeGreaterThan(0);
    for (const step of canvasTourSteps) {
      expect(step.popover?.title?.trim()).toBeTruthy();
      expect(step.popover?.description?.trim()).toBeTruthy();
    }
  });

  it("anchors to the toolbar palette, sidebar, and save button", () => {
    const selectors = canvasTourSteps.map((s) => s.element).filter((e): e is string => typeof e === "string");
    expect(selectors).toContain('[data-tour="palette"]');
    expect(selectors).toContain('[data-tour="sidebar"]');
    expect(selectors).toContain('[data-tour="save"]');
  });

  it("targets the Inputs node by the same id the canvas renders — guards against drift", () => {
    const selectors = canvasTourSteps.map((s) => s.element).filter((e): e is string => typeof e === "string");
    // React Flow stamps data-id={node.id}; if INPUTS_NODE_ID is renamed without
    // updating the selector, the highlight would silently miss. Keep them tied.
    expect(selectors.some((sel) => sel.includes(`[data-id="${INPUTS_NODE_ID}"]`))).toBe(true);
  });
});
