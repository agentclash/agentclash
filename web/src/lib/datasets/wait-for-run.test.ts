import { afterEach, describe, expect, it, vi } from "vitest";

import type { ApiClient } from "@/lib/api/client";
import type { Run } from "@/lib/api/types";
import { waitForRunCompletion } from "./wait-for-run";

function fakeApi(statuses: Run["status"][]) {
  let call = 0;
  const get = vi.fn(async () => {
    const status = statuses[Math.min(call, statuses.length - 1)];
    call += 1;
    return { id: "run-1", status } as Run;
  });
  return { api: { get } as unknown as ApiClient, get };
}

afterEach(() => {
  vi.useRealTimers();
});

describe("waitForRunCompletion", () => {
  it("polls until the run reaches a terminal status", async () => {
    const { api, get } = fakeApi(["running", "cancelled"]);
    const onStatus = vi.fn();
    const run = await waitForRunCompletion(api, "run-1", {
      pollIntervalMs: 1,
      onStatus,
    });
    expect(run.status).toBe("cancelled");
    expect(get).toHaveBeenCalledTimes(2);
    expect(onStatus).toHaveBeenCalledTimes(2);
  });

  it("stops polling and rejects when the signal aborts", async () => {
    vi.useFakeTimers();
    const { api, get } = fakeApi(["running"]);
    const onStatus = vi.fn();
    const controller = new AbortController();
    const done = waitForRunCompletion(api, "run-1", {
      pollIntervalMs: 5000,
      onStatus,
      signal: controller.signal,
    });
    const settled = done.catch((err: unknown) => err);

    await vi.advanceTimersByTimeAsync(0);
    expect(get).toHaveBeenCalledTimes(1);

    controller.abort();
    const err = await settled;
    expect((err as DOMException).name).toBe("AbortError");

    await vi.advanceTimersByTimeAsync(20_000);
    expect(get).toHaveBeenCalledTimes(1);
    expect(onStatus).toHaveBeenCalledTimes(1);
  });

  it("does not report a status that arrives after abort", async () => {
    let resolveGet!: (run: Run) => void;
    const get = vi.fn(
      () => new Promise<Run>((resolve) => (resolveGet = resolve)),
    );
    const api = { get } as unknown as ApiClient;
    const onStatus = vi.fn();
    const controller = new AbortController();
    const settled = waitForRunCompletion(api, "run-1", {
      onStatus,
      signal: controller.signal,
    }).catch((err: unknown) => err);

    await Promise.resolve();
    controller.abort();
    resolveGet({ id: "run-1", status: "completed" } as Run);

    const err = await settled;
    expect((err as DOMException).name).toBe("AbortError");
    expect(onStatus).not.toHaveBeenCalled();
  });
});
