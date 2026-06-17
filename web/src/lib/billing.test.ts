import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api/errors";
import {
  billingGateToastMessage,
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
