"use client";

import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useApiQuery } from "@/lib/api/swr";
import type {
  BillingOverviewResponse,
  BillingPlan,
  BillingPlansResponse,
  CreateBillingCheckoutResponse,
  CreateBillingPortalResponse,
} from "@/lib/api/types";
import {
  billingStatusLabel,
  formatBillingDate,
  formatBillingLimit,
  planLabel,
} from "@/lib/billing";
import { useOrgContext } from "../org-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CreditCard, ExternalLink, Loader2, RefreshCw } from "lucide-react";
import { toast } from "sonner";

type SelfServePeriod = "monthly" | "yearly";

const PRICE_COPY: Record<string, Record<SelfServePeriod, { value: string; suffix: string; note?: string }>> = {
  free: {
    monthly: { value: "$0", suffix: "/ month" },
    yearly: { value: "$0", suffix: "/ month" },
  },
  pro: {
    monthly: { value: "$49", suffix: "/ month", note: "Billed monthly" },
    yearly: { value: "$39", suffix: "/ month", note: "$468 / year" },
  },
  team: {
    monthly: { value: "$100", suffix: "/ month", note: "Billed monthly" },
    yearly: { value: "$80", suffix: "/ month", note: "$960 / year" },
  },
};

function LimitRow({ label, value }: { label: string; value?: number | null }) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-md border border-white/[0.06] px-3 py-2">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm font-medium">{formatBillingLimit(value)}</span>
    </div>
  );
}

function PlanCard({
  plan,
  currentPlan,
  billingPeriod,
  busyAction,
  onCheckout,
}: {
  plan: BillingPlan;
  currentPlan: string;
  billingPeriod: SelfServePeriod;
  busyAction: string | null;
  onCheckout: (plan: BillingPlan) => void;
}) {
  const isCurrent = plan.key === currentPlan;
  const busy = busyAction === plan.key;
  const canCheckout = plan.key !== "free" && plan.key !== "enterprise";
  const price = PRICE_COPY[plan.key]?.[billingPeriod];

  return (
    <div className="flex min-h-56 flex-col rounded-lg border border-white/[0.08] bg-white/[0.02] p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold">{plan.display_name}</h3>
          <p className="mt-1 text-xs text-muted-foreground">
            {plan.key === "free"
              ? "Run real evals before you pay."
              : plan.key === "enterprise"
                ? "Custom limits and billing terms."
                : "Upgrade when you need more runs, retention, or governance."}
          </p>
        </div>
        {isCurrent && <Badge variant="secondary">Current</Badge>}
      </div>

      {price && (
        <div className="mt-4">
          <div className="flex items-end gap-1">
            <span className="text-2xl font-semibold tracking-tight">
              {price.value}
            </span>
            <span className="pb-1 text-xs text-muted-foreground">
              {price.suffix}
            </span>
          </div>
          {price.note && (
            <p className="mt-1 text-xs text-muted-foreground">{price.note}</p>
          )}
        </div>
      )}

      <div className="mt-4 space-y-2">
        <LimitRow
          label="Runs / workspace"
          value={plan.limits.races_per_workspace_month.value}
        />
        <LimitRow
          label="Models / run"
          value={plan.limits.max_models_per_race.value}
        />
      </div>

      <div className="mt-auto flex flex-wrap gap-2 pt-4">
        {canCheckout && (
          <Button
            size="sm"
            disabled={busy}
            onClick={() => onCheckout(plan)}
          >
            {busy ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <CreditCard className="size-4" />
            )}
            Upgrade
          </Button>
        )}
      </div>
    </div>
  );
}

function PeriodToggle({
  value,
  onChange,
}: {
  value: SelfServePeriod;
  onChange: (value: SelfServePeriod) => void;
}) {
  return (
    <div
      className="inline-flex rounded-md border border-white/[0.08] bg-white/[0.03] p-1"
      role="group"
      aria-label="Billing period"
    >
      {(["monthly", "yearly"] as const).map((period) => (
        <button
          key={period}
          type="button"
          aria-pressed={value === period}
          onClick={() => onChange(period)}
          className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
            value === period
              ? "bg-white text-background"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          {period === "monthly" ? "Monthly" : "Yearly"}
        </button>
      ))}
    </div>
  );
}

export function BillingSettingsClient() {
  const { orgId } = useOrgContext();
  const searchParams = useSearchParams();
  const { getAccessToken } = useAccessToken();
  const { data: overview, error, isLoading, mutate } =
    useApiQuery<BillingOverviewResponse>(`/v1/organizations/${orgId}/billing`);
  const { data: plansData } = useApiQuery<BillingPlansResponse>("/v1/billing/plans");
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [billingPeriod, setBillingPeriod] = useState<SelfServePeriod>("monthly");

  const plans = useMemo(
    () => (plansData?.items ?? []).filter((plan) => plan.key !== "enterprise"),
    [plansData],
  );
  const entitlements = overview?.entitlements;
  const hasDodoCustomer = Boolean(overview?.account?.dodo_customer_id);
  const checkoutReturned = searchParams.get("checkout") === "pending";
  const dodoReturnStatus = searchParams.get("status");
  const checkoutPending =
    checkoutReturned &&
    overview?.latest_checkout_intent?.status !== "completed";

  useEffect(() => {
    if (!checkoutReturned || !checkoutPending) return;
    const interval = window.setInterval(() => {
      void mutate();
    }, 4000);
    return () => window.clearInterval(interval);
  }, [checkoutPending, checkoutReturned, mutate]);

  async function createCheckout(plan: BillingPlan) {
    setBusyAction(plan.key);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      const returnURL = new URL(window.location.href);
      for (const key of [
        "checkout",
        "checkout_intent_id",
        "status",
        "payment_id",
        "subscription_id",
        "email",
        "license_key",
      ]) {
        returnURL.searchParams.delete(key);
      }
      const result = await api.post<CreateBillingCheckoutResponse>(
        `/v1/organizations/${orgId}/billing/checkout`,
        {
          plan_key: plan.key,
          billing_period: billingPeriod,
          seat_quantity: 1,
          return_url: returnURL.toString(),
        },
      );
      window.location.assign(result.checkout_url);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create checkout");
      setBusyAction(null);
    }
  }

  async function openPortal() {
    setBusyAction("portal");
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      const result = await api.post<CreateBillingPortalResponse>(
        `/v1/organizations/${orgId}/billing/portal`,
      );
      window.location.assign(result.portal_url);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to open billing portal");
      setBusyAction(null);
    }
  }

  if (isLoading && !overview) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !overview || !entitlements) {
    return (
      <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
        Failed to load billing.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Billing</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Review your plan, usage, and upgrade options.
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={busyAction === "portal" || !hasDodoCustomer}
          onClick={openPortal}
          title={
            hasDodoCustomer
              ? "Open Dodo customer portal"
              : "Available after checkout creates a Dodo customer"
          }
        >
          {busyAction === "portal" ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <ExternalLink className="size-4" />
          )}
          Manage Billing
        </Button>
      </div>

      {!hasDodoCustomer && (
        <p className="-mt-4 text-sm text-muted-foreground">
          Manage billing becomes available after the first Dodo checkout creates
          a customer record.
        </p>
      )}

      {checkoutReturned && (
        <section className="rounded-lg border border-amber-400/20 bg-amber-400/5 p-4">
          <div className="flex items-start gap-3">
            <RefreshCw className="mt-0.5 size-4 text-amber-300" />
            <div>
              <h2 className="text-sm font-semibold">
                {checkoutPending ? "Checkout pending" : "Checkout synced"}
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">
                {checkoutPending
                  ? "Dodo sent you back here. Billing access updates as soon as the webhook is processed."
                  : "Dodo billing is reflected on this organization."}
                {dodoReturnStatus ? ` Status: ${dodoReturnStatus}.` : ""}
              </p>
            </div>
          </div>
        </section>
      )}

      <section className="rounded-lg border border-white/[0.08] bg-white/[0.02] p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs font-medium uppercase text-muted-foreground">
              Current plan
            </p>
            <div className="mt-1 flex items-center gap-2">
              <h2 className="text-xl font-semibold">
                {planLabel(entitlements.plan_key)}
              </h2>
              <Badge
                variant={
                  entitlements.status === "expired" ||
                  entitlements.status === "inactive"
                    ? "destructive"
                    : "secondary"
                }
              >
                {billingStatusLabel(entitlements.status)}
              </Badge>
            </div>
          </div>
          {entitlements.expires_at && (
            <div className="text-right text-sm">
              <p className="text-muted-foreground">Access ends</p>
              <p className="font-medium">
                {formatBillingDate(entitlements.expires_at)}
              </p>
            </div>
          )}
        </div>
        <div className="mt-4 grid gap-2 sm:grid-cols-2">
          <LimitRow label="Workspaces" value={entitlements.workspaces_limit} />
          <LimitRow
            label="Runs / workspace / month"
            value={entitlements.races_per_workspace_month}
          />
          <LimitRow
            label="Concurrent runs"
            value={entitlements.concurrent_races}
          />
          <LimitRow
            label="Models / run"
            value={entitlements.max_models_per_race}
          />
          <LimitRow
            label="Replay retention days"
            value={entitlements.replay_retention_days}
          />
        </div>
      </section>

      <section>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-sm font-semibold">Plans</h2>
          <PeriodToggle value={billingPeriod} onChange={setBillingPeriod} />
        </div>
        <div className="grid gap-3 lg:grid-cols-3">
          {plans.map((plan) => (
            <PlanCard
              key={plan.key}
              plan={plan}
              currentPlan={entitlements.plan_key}
              billingPeriod={billingPeriod}
              busyAction={busyAction}
              onCheckout={createCheckout}
            />
          ))}
        </div>
      </section>
    </div>
  );
}
