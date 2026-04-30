"use client";

import { useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useApiQuery } from "@/lib/api/swr";
import type {
  BillingOverviewResponse,
  BillingPlan,
  BillingPlansResponse,
} from "@/lib/api/types";
import { isFreeActive } from "@/lib/billing";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

interface TrialUpgradePromptProps {
  orgId?: string;
  orgSlug?: string;
  isOrgAdmin?: boolean;
}

function storageKey(orgId: string) {
  return `agentclash:billing-trial-prompt-dismissed:${orgId}`;
}

export function TrialUpgradePrompt({
  orgId,
  orgSlug,
  isOrgAdmin = false,
}: TrialUpgradePromptProps) {
  const pathname = usePathname();
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [dismissed, setDismissed] = useState(true);
  const [busyPlan, setBusyPlan] = useState<string | null>(null);
  const shouldFetch = Boolean(orgId && isOrgAdmin);
  const { data: overview, mutate } = useApiQuery<BillingOverviewResponse>(
    shouldFetch ? `/v1/organizations/${orgId}/billing` : null,
  );
  const { data: plansData } = useApiQuery<BillingPlansResponse>(
    shouldFetch ? "/v1/billing/plans" : null,
  );

  useEffect(() => {
    if (!orgId || !isOrgAdmin) return;
    setDismissed(window.localStorage.getItem(storageKey(orgId)) === "true");
  }, [isOrgAdmin, orgId]);

  const trialPlans = useMemo(
    () => (plansData?.items ?? []).filter((plan) => plan.key === "pro" || plan.key === "team"),
    [plansData],
  );
  const onBillingPage = pathname.includes("/billing");
  const open =
    Boolean(orgId && isOrgAdmin) &&
    !dismissed &&
    !onBillingPage &&
    isFreeActive(overview?.entitlements);

  function dismiss() {
    if (orgId) window.localStorage.setItem(storageKey(orgId), "true");
    setDismissed(true);
  }

  async function startTrial(plan: BillingPlan) {
    if (!orgId) return;
    setBusyPlan(plan.key);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      await api.post<BillingOverviewResponse>(
        `/v1/organizations/${orgId}/billing/trial`,
        { plan_key: plan.key, billing_period: "monthly" },
      );
      toast.success(`${plan.display_name} trial started`);
      dismiss();
      await mutate();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to start trial");
    } finally {
      setBusyPlan(null);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) dismiss();
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Try AgentClash with higher limits</DialogTitle>
          <DialogDescription>
            You are on Free. You can keep using Free, or start a 45-day trial
            for the plan you want to evaluate.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-2">
          {trialPlans.map((plan) => (
            <button
              key={plan.key}
              type="button"
              disabled={Boolean(busyPlan)}
              onClick={() => startTrial(plan)}
              className="rounded-lg border border-white/[0.08] bg-white/[0.03] p-3 text-left transition-colors hover:bg-white/[0.06] disabled:opacity-60"
            >
              <div className="flex items-center justify-between gap-3">
                <span className="text-sm font-medium">
                  Start {plan.display_name} trial
                </span>
                {busyPlan === plan.key && (
                  <Loader2 className="size-4 animate-spin" />
                )}
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {plan.limits.max_models_per_race.value ?? "Unlimited"} models
                per run, {plan.limits.concurrent_races.value ?? "unlimited"}{" "}
                concurrent runs.
              </p>
            </button>
          ))}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={dismiss} disabled={Boolean(busyPlan)}>
            Keep Free
          </Button>
          {orgSlug && (
            <Button
              variant="secondary"
              disabled={Boolean(busyPlan)}
              onClick={() => {
                dismiss();
                router.push(`/orgs/${orgSlug}/billing`);
              }}
            >
              View Plans
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
