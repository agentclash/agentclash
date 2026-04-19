"use client";

import Link from "next/link";
import { AlertTriangle, ArrowRightCircle } from "lucide-react";

import {
  REGRESSION_BLOCKING_RULES,
  regressionRuleLabel,
} from "@/lib/api/release-gates";
import type { ReleaseGate } from "@/lib/api/types";

interface RegressionAlertBannerProps {
  workspaceId: string;
  gates: ReleaseGate[];
}

/**
 * Renders the "New blocking regression" banner at the top of the compare
 * page when any release gate evaluated for this comparison produced a
 * blocking regression violation. Silent when there are no gates, or when
 * no gate violates one of the two blocking regression rules.
 */
export function RegressionAlertBanner({
  workspaceId,
  gates,
}: RegressionAlertBannerProps) {
  const offending = gates.find((gate) => {
    if (gate.verdict !== "fail") return false;
    const violations = gate.evaluation_details.regression_violations ?? [];
    return violations.some((v) => REGRESSION_BLOCKING_RULES.has(v.rule));
  });
  if (!offending) return null;

  const violation = (offending.evaluation_details.regression_violations ?? [])
    .find((v) => REGRESSION_BLOCKING_RULES.has(v.rule));
  if (!violation) return null;

  const caseHref = `/workspaces/${workspaceId}/regression-suites/${violation.suite_id}/cases/${violation.regression_case_id}`;

  return (
    <div className="rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-3">
      <div className="flex items-center gap-2 text-sm font-semibold text-red-300">
        <AlertTriangle className="size-4" />
        New blocking regression
      </div>
      <p className="mt-1 text-sm text-red-200/90">
        {offending.summary ||
          `A release gate failed on ${regressionRuleLabel(violation.rule)}.`}
      </p>
      <div className="mt-2 flex flex-wrap items-center gap-3 text-xs">
        <span className="text-muted-foreground">
          Gate: {offending.policy_key} v{offending.policy_version}
        </span>
        <span className="text-muted-foreground">
          Rule: {regressionRuleLabel(violation.rule)}
        </span>
        <Link
          href={caseHref}
          className="inline-flex items-center gap-1 font-medium text-red-200 hover:text-red-100 transition-colors"
        >
          View regression case
          <ArrowRightCircle className="size-3" />
        </Link>
      </div>
    </div>
  );
}
