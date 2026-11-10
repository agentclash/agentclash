import { ApiError } from "@/lib/api/errors";
import type { EffectiveEntitlements } from "@/lib/api/types";

const planLabels: Record<string, string> = {
  free: "Free",
  pro: "Pro",
  team: "Team",
  enterprise: "Enterprise",
};

export function planLabel(planKey?: string): string {
  if (!planKey) return "your plan";
  return planLabels[planKey] ?? planKey;
}

export function billingStatusLabel(status?: string): string {
  switch (status) {
    case "trialing":
      return "Promotional";
    case "active":
      return "Active";
    case "expired":
      return "Expired";
    case "inactive":
      return "Inactive";
    default:
      return status ?? "Unknown";
  }
}

export function formatBillingDate(value?: string): string {
  if (!value) return "";
  return new Date(value).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function formatBillingLimit(value?: number | null): string {
  return value == null ? "Unlimited" : new Intl.NumberFormat().format(value);
}

/** How long the billing page polls after a Dodo checkout returns. */
export const CHECKOUT_POLL_INTERVAL_MS = 4000;
export const CHECKOUT_POLL_MAX_MS = 2 * 60 * 1000;

/**
 * State of the banner shown after a Dodo checkout returns. The backend only
 * ever marks an intent completed, so an abandoned checkout stays pending
 * forever. Polling stops after CHECKOUT_POLL_MAX_MS and the banner moves to
 * "stalled", where the user can refresh by hand.
 */
export type CheckoutReturnState = "none" | "pending" | "stalled" | "synced";

export function checkoutReturnState(
  returned: boolean,
  intentStatus: string | undefined,
  pollTimedOut: boolean,
): CheckoutReturnState {
  if (!returned) return "none";
  if (intentStatus === "completed") return "synced";
  return pollTimedOut ? "stalled" : "pending";
}

export function isFreeActive(entitlements?: EffectiveEntitlements): boolean {
  return entitlements?.plan_key === "free" && entitlements.status === "active";
}

export function isBillingGateError(err: unknown): err is ApiError {
  return (
    err instanceof ApiError &&
    [
      "plan_limit_exceeded",
      "quota_exceeded",
      "concurrency_limit_exceeded",
      "seat_limit_exceeded",
      "workspace_limit_exceeded",
      "feature_not_entitled",
      "entitlement_expired",
      "entitlement_inactive",
    ].includes(err.code)
  );
}

export function billingGateToastMessage(err: unknown): string | null {
  if (!isBillingGateError(err)) return null;

  const current = planLabel(err.planKey);
  const upgrade = planLabel(err.upgradeTarget);
  const reset = err.resetAt ? ` Resets ${formatBillingDate(err.resetAt)}.` : "";
  const expiry = err.expiresAt ? ` Access ended ${formatBillingDate(err.expiresAt)}.` : "";

  switch (err.code) {
    case "feature_not_entitled":
      return `${current} does not include this feature. Upgrade to ${upgrade}.`;
    case "quota_exceeded":
      return `${current} has used ${err.used ?? "all"} of ${formatBillingLimit(err.limit)} monthly runs.${reset} Upgrade to continue.`;
    case "concurrency_limit_exceeded":
      return `${current} allows ${formatBillingLimit(err.limit)} concurrent run${err.limit === 1 ? "" : "s"}. Try again after one finishes, or upgrade to ${upgrade}.`;
    case "plan_limit_exceeded":
      return `${current} allows up to ${formatBillingLimit(err.limit)} models per run. Upgrade to ${upgrade}.`;
    case "seat_limit_exceeded":
      return `${current} has reached its member limit. Upgrade to ${upgrade}.`;
    case "workspace_limit_exceeded":
      return `${current} has reached its workspace limit. Upgrade to ${upgrade}.`;
    case "entitlement_expired":
      return `${current} access has expired.${expiry} Add billing to continue.`;
    case "entitlement_inactive":
      return `${current} billing is inactive. Add billing to continue.`;
    default:
      return err.message;
  }
}
