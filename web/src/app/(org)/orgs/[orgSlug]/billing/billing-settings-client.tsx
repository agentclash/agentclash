"use client";

import { useMemo, useState } from "react";
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
  isFreeActive,
  planLabel,
} from "@/lib/billing";
import { useOrgContext } from "../org-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Loader2, RefreshCw } from "lucide-react";
import { toast } from "sonner";

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
  trialAvailable,
  busyAction,
  onStartTrial,
  onCheckout,
}: {
  plan: BillingPlan;
  currentPlan: string;
  trialAvailable: boolean;
  busyAction: string | null;
  onStartTrial: (plan: BillingPlan) => void;
  onCheckout: (plan: BillingPlan) => void;
}) {
  const isCurrent = plan.key === currentPlan;
  const busy = busyAction === plan.key;
  const canTrial = trialAvailable && (plan.key === "pro" || plan.key === "team");
  const canCheckout = plan.key !== "free" && plan.key !== "enterprise";

  return (
    <div className="flex min-h-56 flex-col rounded-lg border border-white/[0.08] bg-white/[0.02] p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold">{plan.display_name}</h3>
          <p className="mt-1 text-xs text-muted-foreground">
            {plan.key === "free"
              ? "Stay on Free while you evaluate."
              : plan.key === "enterprise"
                ? "Custom limits and billing terms."
                : "Start a 45-day trial or create checkout."}
          </p>
        </div>
        {isCurrent && <Badge variant="secondary">Current</Badge>}
      </div>

      <div className="mt-4 space-y-2">
        <LimitRow label="Seats" value={plan.limits.seats.value} />
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
        {canTrial && (
          <Button
            size="sm"
            disabled={busy}
            onClick={() => onStartTrial(plan)}
          >
            {busy ? <Loader2 className="size-4 animate-spin" /> : "Start Trial"}
          </Button>
        )}
        {canCheckout && (
          <Button
            size="sm"
            variant={canTrial ? "outline" : "default"}
            disabled={busy}
            onClick={() => onCheckout(plan)}
          >
            Checkout
          </Button>
        )}
      </div>
    </div>
  );
}

export function BillingSettingsClient() {
  const { orgId } = useOrgContext();
  const { getAccessToken } = useAccessToken();
  const { data: overview, error, isLoading, mutate } =
    useApiQuery<BillingOverviewResponse>(`/v1/organizations/${orgId}/billing`);
  const { data: plansData } = useApiQuery<BillingPlansResponse>("/v1/billing/plans");
  const [busyAction, setBusyAction] = useState<string | null>(null);

  const plans = useMemo(
    () => (plansData?.items ?? []).filter((plan) => plan.key !== "enterprise"),
    [plansData],
  );
  const entitlements = overview?.entitlements;
  const trialAvailable = isFreeActive(entitlements);

  async function startTrial(plan: BillingPlan) {
    setBusyAction(plan.key);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      await api.post<BillingOverviewResponse>(
        `/v1/organizations/${orgId}/billing/trial`,
        {
          plan_key: plan.key,
          billing_period: "monthly",
        },
      );
      toast.success(`${plan.display_name} trial started`);
      await mutate();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to start trial");
    } finally {
      setBusyAction(null);
    }
  }

  async function createCheckout(plan: BillingPlan) {
    setBusyAction(plan.key);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      const result = await api.post<CreateBillingCheckoutResponse>(
        `/v1/organizations/${orgId}/billing/checkout`,
        {
          plan_key: plan.key,
          billing_period: "monthly",
          seat_quantity: Math.max(plan.minimum_seats, plan.default_seats),
          return_url: window.location.href,
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
            Review your plan, trial status, and usage limits.
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={busyAction === "portal"}
          onClick={openPortal}
        >
          {busyAction === "portal" ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            "Manage Billing"
          )}
        </Button>
      </div>

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
              <p className="text-muted-foreground">Trial ends</p>
              <p className="font-medium">
                {formatBillingDate(entitlements.expires_at)}
              </p>
            </div>
          )}
        </div>
        <div className="mt-4 grid gap-2 sm:grid-cols-2">
          <LimitRow label="Seats" value={entitlements.seats_limit} />
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

      {trialAvailable && (
        <section className="rounded-lg border border-primary/20 bg-primary/5 p-4">
          <div className="flex items-start gap-3">
            <RefreshCw className="mt-0.5 size-4 text-primary" />
            <div>
              <h2 className="text-sm font-semibold">Try a paid plan for 45 days</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Pick the plan you want to evaluate. The trial grants only that
                plan&apos;s limits and expires automatically after 45 days.
              </p>
            </div>
          </div>
        </section>
      )}

      <section>
        <h2 className="mb-3 text-sm font-semibold">Plans</h2>
        <div className="grid gap-3 lg:grid-cols-3">
          {plans.map((plan) => (
            <PlanCard
              key={plan.key}
              plan={plan}
              currentPlan={entitlements.plan_key}
              trialAvailable={trialAvailable}
              busyAction={busyAction}
              onStartTrial={startTrial}
              onCheckout={createCheckout}
            />
          ))}
        </div>
      </section>
    </div>
  );
}
