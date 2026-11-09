import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";
import { defaultModels, type Operation } from "@/lib/vibe";
import { ProgressStrip } from "./activity-status";
import { PrototypeTrial } from "./prototype-trial";
let root: Root, node: HTMLDivElement;
const action = vi.fn();
const turn: Operation = { id: "turn", kind: "message", state: "RUNNING", created_at: "2026-10-30T10:00:00Z", models: defaultModels, billing: "RESERVED", max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [], conversation_decision: { intent: "prepare_tests", source_message_id: "task" } };
beforeEach(() => { vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true); vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn(), addListener: vi.fn(), removeListener: vi.fn() }))); vi.clearAllMocks(); vi.useFakeTimers(); vi.setSystemTime(new Date("2026-10-30T10:01:05Z")); node = document.createElement("div"); document.body.append(node); root = createRoot(node); });
afterEach(async () => { await act(async () => root.unmount()); node.remove(); vi.useRealTimers(); vi.unstubAllGlobals(); });
async function render(operation?: Operation, phase = "preparing", pendingLabel?: string, cycle = "cycle") { await act(async () => root.render(<ProgressStrip operation={operation} total={operation && ["check", "retest"].includes(operation.kind) ? 3 : 0} pendingLabel={pendingLabel} build={{ cycle_id: cycle, phase, clarifications_used: 0 }} startedAt={turn.created_at} onAction={action} />)); }
it("shows four server-derived stages, elapsed time and Stop consequences from preparation through the run", async () => {
  for (const [phase, label] of [["understanding", "Understand the task"], ["preparing", "Write the instructions"], ["reviewing", "Write 3 tests"], ["repairing", "Write 3 tests"]]) {
    await render({ ...turn, progress: { phase, total_cases: 0, completed_cases: 0 } }); expect(node.querySelector('[role="status"]')?.textContent).toBe(label); expect(node.querySelectorAll("li")).toHaveLength(4);
  }
  expect(node.textContent).toContain("Elapsed 1:05"); await act(async () => vi.advanceTimersByTime(2000)); expect(node.textContent).toContain("Elapsed 1:07");
  expect(node.textContent).toContain("Stop keeps finished results."); expect(node.querySelector('[role="progressbar"]')?.hasAttribute("data-paused")).toBe(true);
  await act(async () => node.querySelector<HTMLButtonElement>("button")!.click()); expect(action).toHaveBeenCalledWith("turn", "stop");
  await render({ ...turn, id: "run", kind: "check", progress: { phase: "grading", completed_cases: 2, total_cases: 3 } }, "checking");
  expect(node.querySelector('[role="status"]')?.textContent).toBe("Run tests (2 of 3)"); expect(node.querySelector('[role="progressbar"]')?.getAttribute("aria-valuenow")).toBe("2");
  expect(node.querySelectorAll("button")).toHaveLength(1);
});
it("keeps the stage from moving backward within a cycle and resets for a new cycle", async () => {
  await render({ ...turn, progress: { phase: "repairing", completed_cases: 0, total_cases: 0 } });
  await render({ ...turn, progress: { phase: "preparing", completed_cases: 0, total_cases: 0 } }); expect(node.querySelector('[role="status"]')?.textContent).toBe("Write 3 tests");
  await render(turn, "preparing", undefined, "new-cycle"); expect(node.querySelector('[role="status"]')?.textContent).toBe("Understand the task");
});
it("distinguishes done, current and upcoming stages without link styling", async () => {
  const style = document.createElement("style"); style.textContent = readFileSync("src/components/vibe/build-output.css", "utf8"); document.head.append(style);
  try {
    await render({ ...turn, progress: { phase: "preparing", completed_cases: 0, total_cases: 0 } });
    const stages = node.querySelectorAll("li");
    expect(stages[0].textContent).toContain("✓"); expect(stages[0].textContent).toContain("Done");
    expect(stages[1].getAttribute("aria-current")).toBe("step");
    expect(getComputedStyle(stages[1]).fontWeight).toBe("600");
    expect(getComputedStyle(stages[1]).textDecoration).toBe("none");
    expect(stages[2].getAttribute("data-state")).toBe("upcoming");
    expect(getComputedStyle(stages[2]).color).toBe("var(--vibe-muted)");
    expect(Array.from(stages, stage => stage.textContent)).toEqual(["✓Done: Understand the task", "Write the instructions", "Write 3 tests", "Run tests"]);
  } finally { style.remove(); }
});
it.each(["check", "retest"])("keeps ordinary %s counts after the first Build finishes", async kind => {
  await render({ ...turn, id: "later-run", kind, progress: { phase: "grading", completed_cases: 1, total_cases: 3 } }, "results");
  expect(node.querySelector('[role="status"]')?.textContent).toBe("1 of 3 tests finished");
  expect(node.querySelector('[aria-label="Build stages"]')).toBeNull();
});
it("shows a paused answer state and an immediate pending-send strip", async () => {
  await render(undefined, "preparing", "Starting your project…"); expect(node.querySelector('[role="status"]')?.textContent).toBe("Starting your project…");
  await render(undefined, "clarifying"); expect(node.querySelector('[role="status"]')?.textContent).toBe("Needs your answer"); expect(node.querySelectorAll("button")).toHaveLength(0);
});
it("includes the visible Try it label in the trial button name and keeps inline trial secondary", async () => {
  const props = { inline: true, busy: false, text: "Try a return question", onText: vi.fn(), onSend: vi.fn(), onBack: vi.fn(), onNew: vi.fn(), thread: "new", onThread: vi.fn(), history: [], messages: [] };
  await act(async () => root.render(<PrototypeTrial {...props} dock />));
  const button = node.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  expect(button.textContent).toContain("Try it"); expect(button.getAttribute("aria-label")).toContain("Try it");
  await act(async () => root.render(<PrototypeTrial {...props} workspace />)); expect(node.querySelector('button[type="submit"]')?.classList.contains("vibe-button-primary")).toBe(false);
});
