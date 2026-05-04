import { Badge } from "@/components/ui/badge";
import type {
  RegressionCaseValidationStatus,
  RegressionCaseStatus,
  RegressionSeverity,
  RegressionSuiteStatus,
} from "@/lib/api/types";

const suiteStatusVariant: Record<
  RegressionSuiteStatus,
  "default" | "secondary"
> = {
  active: "default",
  archived: "secondary",
};

const caseStatusVariant: Record<
  RegressionCaseStatus,
  "default" | "outline" | "secondary"
> = {
  proposed: "outline",
  active: "default",
  muted: "outline",
  archived: "secondary",
  rejected: "secondary",
};

const severityVariant: Record<
  RegressionSeverity,
  "default" | "outline" | "destructive"
> = {
  info: "outline",
  warning: "default",
  blocking: "destructive",
};

const validationVariant: Record<
  RegressionCaseValidationStatus,
  "default" | "outline" | "secondary" | "destructive"
> = {
  not_validated: "outline",
  collecting_signal: "outline",
  reproducing: "destructive",
  passing: "secondary",
  flaky: "default",
};

const validationLabel: Record<RegressionCaseValidationStatus, string> = {
  not_validated: "not validated",
  collecting_signal: "collecting",
  reproducing: "reproducing",
  passing: "passing",
  flaky: "flaky",
};

export function SuiteStatusBadge({ status }: { status: RegressionSuiteStatus }) {
  return <Badge variant={suiteStatusVariant[status]}>{status}</Badge>;
}

export function CaseStatusBadge({ status }: { status: RegressionCaseStatus }) {
  return <Badge variant={caseStatusVariant[status]}>{status}</Badge>;
}

export function SeverityBadge({ severity }: { severity: RegressionSeverity }) {
  return <Badge variant={severityVariant[severity]}>{severity}</Badge>;
}

export function ValidationBadge({
  status,
}: {
  status: RegressionCaseValidationStatus;
}) {
  return <Badge variant={validationVariant[status]}>{validationLabel[status]}</Badge>;
}
