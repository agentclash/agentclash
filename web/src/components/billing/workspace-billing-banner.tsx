"use client";

import Link from "next/link";
import { useApiQuery } from "@/lib/api/swr";
import type { WorkspaceEntitlementsResponse } from "@/lib/api/types";
import {
  billingStatusLabel,
  formatBillingDate,
  formatBillingLimit,
  planLabel,
} from "@/lib/billing";
import { Badge } from "@/components/ui/badge";

export function WorkspaceBillingBanner({
  workspaceId,
  orgSlug,
}: {
  workspaceId: string;
  orgSlug?: string;
}) {
  const { data } = useApiQuery<WorkspaceEntitlementsResponse>(
    `/v1/workspaces/${workspaceId}/entitlements`,
  );
  const entitlements = data?.entitlements;
  if (!entitlements) return null;

  const shouldShow =
    entitlements.plan_key === "free" ||
    entitlements.status === "trialing" ||
    entitlements.status === "expired" ||
    entitlements.status === "inactive";

  if (!shouldShow) return null;

  const usage = data?.usage;
  const limit = entitlements.races_per_workspace_month;
  const billingHref = orgSlug ? `/orgs/${orgSlug}/billing` : undefined;

  return (
    <div className="border-b border-white/[0.06] bg-white/[0.025] px-4 py-2">
      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <Badge variant={entitlements.status === "expired" ? "destructive" : "secondary"}>
          {planLabel(entitlements.plan_key)} {billingStatusLabel(entitlements.status)}
        </Badge>
        {usage && (
          <span>
            {usage.race_count} / {formatBillingLimit(limit)} runs used this month
          </span>
        )}
        {entitlements.expires_at && (
          <span>Trial ends {formatBillingDate(entitlements.expires_at)}</span>
        )}
        {billingHref && (
          <Link
            href={billingHref}
            className="ml-auto font-medium text-foreground underline underline-offset-4"
          >
            Billing
          </Link>
        )}
      </div>
    </div>
  );
}
