import type { DriveStep } from "driver.js";

// Walkthrough for the tool canvas builder, in plain language for non-engineers.
// Anchored to [data-tour="…"] hooks on the toolbar palette, the sidebar, and
// the Save button, plus React Flow's own data-id on the Inputs node — keep
// those selectors in sync with this file.
export const CANVAS_TOUR_ID = "tools-canvas";

export const canvasTourSteps: DriveStep[] = [
  {
    popover: {
      title: "Build a tool, visually",
      description:
        "Drop nodes on the canvas and wire them together. One node makes a simple tool; connect a few to build a multi-step chain — no code, no JSON.",
    },
  },
  {
    element: '[data-tour="palette"]',
    popover: {
      title: "Add a step",
      description:
        "Add an Operation (a built-in action like calling a web API), another saved Tool, or a Canned response for rehearsing evals.",
      side: "bottom",
      align: "start",
    },
  },
  {
    element: '.react-flow__node[data-id="inputs"]',
    popover: {
      title: "Start from the agent's inputs",
      description:
        "This node holds the parameters the agent passes in. Click any node to configure it in the panel on the right.",
      side: "right",
    },
  },
  {
    element: '.react-flow__node[data-id="inputs"]',
    popover: {
      title: "Connect the steps",
      description:
        "Drag from a node's right edge to the next node to chain them. A wire means “this step can use that one's result.”",
      side: "right",
    },
  },
  {
    element: '[data-tour="sidebar"]',
    popover: {
      title: "Configure on the right",
      description:
        "Select a node to set it up here — pick the operation and fill in values. With nothing selected you get a plain-language summary and a checklist of what's left.",
      side: "left",
    },
  },
  {
    element: '[data-tour="save"]',
    popover: {
      title: "Save when it's ready",
      description:
        "We validate as you go. Once everything checks out, save — and you can replay this tour anytime from the “?” button.",
      side: "bottom",
      align: "end",
    },
  },
];
