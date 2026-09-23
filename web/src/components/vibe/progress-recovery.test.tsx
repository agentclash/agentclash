import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { RetryControl, retryWait } from "./retry-control";
import { conversationActivity } from "./activity-status";
import { recoveryMessage } from "./recovery-message";
import type { Operation } from "@/lib/vibe";

const operation = { kind: "check", state: "RUNNING", progress: { phase: "grading", completed_cases: 1, total_cases: 3 } } as Operation;
afterEach(() => vi.useRealTimers());

it("uses the provider stage and saved count without treating UNKNOWN as a pass", () => {
  expect(conversationActivity(operation)).toBe("1 of 3 results saved · Checking the reply…");
  expect(conversationActivity({ ...operation, state: "CANCELLING" })).toBe("Stopping…");
  expect(conversationActivity({ ...operation, state: "FINALIZING" })).toBe("Saving your results…");
  expect(conversationActivity({ ...operation, state: "AWAITING_INPUT" })).toBe("One detail is needed to continue.");
  expect(conversationActivity({ ...operation, kind: "message", progress: { phase: "reviewing", completed_cases: 0, total_cases: 0 } })).toBe("Checking the tests against your rules…");
  expect(conversationActivity({ ...operation, kind: "message", progress: { phase: "advisory_understanding", completed_cases: 0, total_cases: 0 } })).toBe("Understanding your request…");
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
