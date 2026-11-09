import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CHECKOUT_POLL_INTERVAL_MS, CHECKOUT_POLL_MAX_MS } from "@/lib/billing";
import { BillingSettingsClient } from "./billing-settings-client";

const mocks = vi.hoisted(() => ({ mutate: vi.fn() }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams("checkout=pending"),
}));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: vi.fn(async () => "token") }),
}));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));
vi.mock("../org-context", () => ({
  useOrgContext: () => ({ orgId: "org-1" }),
}));
vi.mock("@/lib/api/swr", () => ({
  useApiQuery: (path: string) =>
    path === "/v1/billing/plans"
      ? { data: { items: [] } }
      : {
          data: {
            entitlements: { plan_key: "free", status: "active" },
            // An abandoned checkout: the backend never completes it.
            latest_checkout_intent: { status: "created" },
          },
          error: undefined,
          isLoading: false,
          mutate: mocks.mutate,
        },
}));

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

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

describe("BillingSettingsClient checkout return", () => {
  it("stops polling an abandoned checkout and offers a manual refresh", async () => {
    await act(async () => root.render(<BillingSettingsClient />));
    expect(container.textContent).toContain("Checkout pending");

    await advance(CHECKOUT_POLL_INTERVAL_MS);
    expect(mocks.mutate).toHaveBeenCalledTimes(1);

    await advance(CHECKOUT_POLL_MAX_MS);
    expect(container.textContent).toContain("Checkout not confirmed yet");
    const callsAtTimeout = mocks.mutate.mock.calls.length;

    await advance(CHECKOUT_POLL_MAX_MS);
    expect(mocks.mutate).toHaveBeenCalledTimes(callsAtTimeout);

    const refresh = [...container.querySelectorAll("button")].find(
      (b) => b.textContent === "Refresh",
    )!;
    await act(async () => refresh.click());
    expect(mocks.mutate).toHaveBeenCalledTimes(callsAtTimeout + 1);
    expect(container.textContent).toContain("Checkout pending");
  });
});
