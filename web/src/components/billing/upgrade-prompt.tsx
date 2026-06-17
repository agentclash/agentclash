"use client";

import { useCallback, useMemo, useSyncExternalStore } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useApiQuery } from "@/lib/api/swr";
import type {
  BillingOverviewResponse,
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

interface UpgradePromptProps {
  orgId?: string;
  orgSlug?: string;
  isOrgAdmin?: boolean;
}

function storageKey(orgId: string) {
  return `agentclash:billing-upgrade-prompt-dismissed:${orgId}`;
}

const DISMISSED_EVENT = "agentclash:billing-upgrade-prompt-dismissed";

function subscribeDismissed(callback: () => void) {
  window.addEventListener("storage", callback);
  window.addEventListener(DISMISSED_EVENT, callback);
  return () => {
    window.removeEventListener("storage", callback);
    window.removeEventListener(DISMISSED_EVENT, callback);
  };
}

function dismissedSnapshot(orgId?: string, isOrgAdmin = false) {
  if (!orgId || !isOrgAdmin) return true;
  return window.localStorage.getItem(storageKey(orgId)) === "true";
}

export function UpgradePrompt({
  orgId,
  orgSlug,
  isOrgAdmin = false,
}: UpgradePromptProps) {
  const pathname = usePathname();
  const router = useRouter();
  const shouldFetch = Boolean(orgId && isOrgAdmin);
  const { data: overview } = useApiQuery<BillingOverviewResponse>(
    shouldFetch ? `/v1/organizations/${orgId}/billing` : null,
  );
  const { data: plansData } = useApiQuery<BillingPlansResponse>(
    shouldFetch ? "/v1/billing/plans" : null,
  );
  const dismissed = useSyncExternalStore(
    subscribeDismissed,
    useCallback(() => dismissedSnapshot(orgId, isOrgAdmin), [isOrgAdmin, orgId]),
    () => true,
  );

  const upgradePlans = useMemo(
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
    window.dispatchEvent(new Event(DISMISSED_EVENT));
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
          <DialogTitle>Need more AgentClash runs?</DialogTitle>
          <DialogDescription>
            You are on Free. Keep using it, or upgrade when you need more run
            volume, replay retention, and governance controls.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-2">
          {upgradePlans.map((plan) => (
            <button
              key={plan.key}
              type="button"
              onClick={() => {
                dismiss();
                if (orgSlug) {
                  router.push(`/orgs/${orgSlug}/billing?plan=${plan.key}`);
                }
              }}
              className="rounded-lg border border-white/[0.08] bg-white/[0.03] p-3 text-left transition-colors hover:bg-white/[0.06] disabled:opacity-60"
            >
              <div className="flex items-center justify-between gap-3">
                <span className="text-sm font-medium">
                  View {plan.display_name}
                </span>
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
          <Button variant="outline" onClick={dismiss}>
            Keep Free
          </Button>
          {orgSlug && (
            <Button
              variant="secondary"
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
