import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api/errors";
import {
  billingGateToastMessage,
  checkoutReturnState,
  formatBillingLimit,
  isFreeActive,
} from "@/lib/billing";

describe("billing helpers", () => {
  it("detects active Free entitlements", () => {
    expect(
      isFreeActive({
        plan_key: "free",
        billing_period: "monthly",
        status: "active",
        seat_quantity: 1,
        feature_flags: {},
      }),
    ).toBe(true);

    expect(
      isFreeActive({
        plan_key: "pro",
        billing_period: "monthly",
        status: "active",
        seat_quantity: 1,
        feature_flags: {},
      }),
    ).toBe(false);
  });

  it("formats unlimited limits", () => {
    expect(formatBillingLimit(null)).toBe("Unlimited");
    expect(formatBillingLimit(2500)).toBe("2,500");
  });

  it("maps billing gate errors to upgrade copy", () => {
    const message = billingGateToastMessage(
      new ApiError(402, "quota_exceeded", "quota exhausted", {
        planKey: "free",
        upgradeTarget: "pro",
        limit: 25,
        used: 25,
      }),
    );

    expect(message).toContain("Free");
    expect(message).toContain("25");
    expect(message).toContain("Upgrade");
  });
});

describe("checkoutReturnState", () => {
  it("is none when the user did not return from checkout", () => {
    expect(checkoutReturnState(false, undefined, false)).toBe("none");
  });

  it("is pending while the intent is not completed and polling is on", () => {
    expect(checkoutReturnState(true, undefined, false)).toBe("pending");
    expect(checkoutReturnState(true, "created", false)).toBe("pending");
  });

  it("is stalled once polling times out without completion", () => {
    expect(checkoutReturnState(true, "created", true)).toBe("stalled");
  });

  it("is stalled at once for an expired intent, so the page does not poll", () => {
    expect(checkoutReturnState(true, "expired", false)).toBe("stalled");
  });

  it("is synced once the intent is completed, even after a timeout", () => {
    expect(checkoutReturnState(true, "completed", false)).toBe("synced");
    expect(checkoutReturnState(true, "completed", true)).toBe("synced");
  });
});
