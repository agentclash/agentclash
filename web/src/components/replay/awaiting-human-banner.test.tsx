import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { HumanTurnStatus } from "@/lib/api/multi-turn";
import {
  AwaitingHumanBanner,
  humanTurnKey,
  nextSubmittedTurn,
} from "./awaiting-human-banner";

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  submit: vi.fn(),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/api/client", () => ({ createApiClient: () => ({}) }));
vi.mock("@/lib/api/multi-turn", () => ({
  getHumanTurnStatus: (...args: unknown[]) => mocks.getStatus(...args),
  submitHumanTurn: (...args: unknown[]) => mocks.submit(...args),
}));

const turn3: HumanTurnStatus = { awaiting_human: true, turn_index: 3 };
const turn4: HumanTurnStatus = { awaiting_human: true, turn_index: 4 };

describe("nextSubmittedTurn", () => {
  it("keeps the marker while the server still shows the submitted turn", () => {
    expect(nextSubmittedTurn(humanTurnKey(turn3), turn3)).toBe(humanTurnKey(turn3));
  });

  it("clears the marker on a new turn or when input is no longer awaited", () => {
    expect(nextSubmittedTurn(humanTurnKey(turn3), turn4)).toBeNull();
    expect(
      nextSubmittedTurn(humanTurnKey(turn3), { awaiting_human: false }),
    ).toBeNull();
  });
});

describe("AwaitingHumanBanner", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  function submitButton() {
    return container.querySelector("button")!;
  }

  async function typeAndSubmit(value: string) {
    const textarea = container.querySelector("textarea")!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(
        HTMLTextAreaElement.prototype,
        "value",
      )!.set!.call(textarea, value);
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => submitButton().click());
  }

  async function advance(ms: number) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ms);
    });
  }

  it("blocks a second submit of the same turn until a new turn shows up", async () => {
    mocks.getStatus.mockResolvedValueOnce(turn3);
    mocks.submit.mockResolvedValue({ status: "accepted" });
    await act(async () =>
      root.render(
        <AwaitingHumanBanner
          getAccessToken={async () => "token"}
          workspaceId="ws-1"
          runId="run-1"
          runAgentId="agent-1"
          enabled
        />,
      ),
    );
    await advance(0);
    expect(container.textContent).toContain("turn 3");

    // The refresh after submit fails, so the banner still shows turn 3.
    mocks.getStatus.mockRejectedValueOnce(new Error("network"));
    await typeAndSubmit("hello");
    expect(mocks.submit).toHaveBeenCalledTimes(1);
    expect(container.textContent).toContain("turn 3");
    expect(submitButton().disabled).toBe(true);

    await typeAndSubmit("hello again");
    expect(mocks.submit).toHaveBeenCalledTimes(1);

    // The server still reports turn 3 for a while.
    mocks.getStatus.mockResolvedValueOnce(turn3);
    await advance(3000);
    expect(submitButton().disabled).toBe(true);

    // A new turn arrives, so submit opens again.
    mocks.getStatus.mockResolvedValue(turn4);
    await advance(3000);
    expect(container.textContent).toContain("turn 4");
    expect(submitButton().disabled).toBe(false);
  });
});
