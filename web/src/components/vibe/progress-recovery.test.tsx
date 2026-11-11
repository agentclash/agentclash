import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { RetryControl, retryWait } from "./retry-control";
import { conversationActivity, ProgressStrip } from "./activity-status";
import { recoveryMessage } from "./recovery-message";
import type { Operation } from "@/lib/vibe";
import { buildStageLabel } from "@/lib/vibe-build-timeline";

const operation = { kind: "check", state: "RUNNING", progress: { phase: "grading", completed_cases: 1, total_cases: 3 } } as Operation;
afterEach(() => vi.useRealTimers());

it("uses the provider stage and saved count without treating UNKNOWN as a pass", () => {
  expect(conversationActivity(operation)).toBe("Source unverified · 1 of 3 results saved · Grading the reply…");
  expect(conversationActivity({ ...operation, state: "CANCELLING" })).toBe("Source unverified · Stopping…");
  expect(conversationActivity({ ...operation, state: "FINALIZING" })).toBe("Source unverified · Saving your results…");
  expect(conversationActivity({ ...operation, kind: "message", progress: { phase: "reviewing", completed_cases: 0, total_cases: 0 } })).toBe("Reviewing the tests against your rules…");
  expect(conversationActivity({ ...operation, kind: "message", progress: { phase: "advisory_understanding", completed_cases: 0, total_cases: 0 } })).toBe("Understanding your request…");
  expect(conversationActivity({ ...operation, kind: "message", progress: { phase: "switching_assistant", completed_cases: 0, total_cases: 0 } })).toBe("Trying another model…");
  expect(conversationActivity({ ...operation, state: "CANCELLING", progress: { phase: "switching_assistant", completed_cases: 0, total_cases: 0 } })).toBe("Source unverified · Stopping…");
});

// A question to the user is the build phase "clarifying". No operation state
// asks for input, so the strip shows the answer state from the phase.
it("pauses the progress strip for an answer when the build phase is clarifying", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn(), addListener: vi.fn(), removeListener: vi.fn() })));
  const node = document.createElement("div"); document.body.append(node);
  const root = createRoot(node);
  const turn = { ...operation, id: "turn", kind: "message", progress: { phase: "understanding", completed_cases: 0, total_cases: 0 } } as Operation;
  const render = (phase: string) => act(async () => root.render(<ProgressStrip operation={turn} total={0} build={{ cycle_id: "cycle", phase, clarifications_used: 1 }} onAction={vi.fn()} />));
  try {
    await render("understanding");
    expect(node.querySelector('[role="status"]')?.textContent).not.toBe("Needs your answer");
    await render("clarifying");
    expect(node.querySelector('[role="status"]')?.textContent).toBe("Needs your answer");
    expect(buildStageLabel(turn, "clarifying")).toBe("Needs your answer");
  } finally { await act(async () => root.unmount()); node.remove(); vi.unstubAllGlobals(); }
});

it("counts down on the server clock without auto-submitting and keeps focus", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "performance"] });
  const node = document.createElement("div"); document.body.append(node);
  const root = createRoot(node);
  const retry = vi.fn();
  const serverTime = "2020-01-01T00:00:00Z", availableAt = "2020-01-01T00:00:02Z";
  expect(retryWait(availableAt, serverTime)).toBe(2000);
  expect(retryWait("garbage", serverTime)).toBe(0);
  try {
    await act(async () => root.render(<><input aria-label="Next message" /><RetryControl label="Try again" disabled={false} serverTime={serverTime} availableAt={availableAt} onRetry={retry} /></>));
    const input = node.querySelector("input")!; input.focus();
    expect(node.querySelector("button")!.disabled).toBe(true);
    expect(node.textContent).toContain("Try again in 2s");
    await act(async () => vi.advanceTimersByTime(2250));
    expect(node.querySelector("button")!.disabled).toBe(false);
    expect(document.activeElement).toBe(input);
    expect(retry).not.toHaveBeenCalled();
    await act(async () => node.querySelector("button")!.click());
    expect(retry).toHaveBeenCalledOnce();
  } finally { await act(async () => root.unmount()); node.remove(); }
});

it("distinguishes a connection configuration problem from an uncertain paid attempt", () => {
  expect(recoveryMessage({ ...operation, error: { code: "provider_auth", message: "secret provider diagnostic" } })).toContain("person running Vibe Evals");
  expect(recoveryMessage({ ...operation, billing: "RECONCILING", error: { code: "provider_timeout", message: "timeout" } })).toContain("checking whether the provider finished");
});
