import { Badge } from "@/components/ui/badge";
import type {
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
  active: "default",
  muted: "outline",
  archived: "secondary",
};

const severityVariant: Record<
  RegressionSeverity,
  "default" | "outline" | "destructive"
> = {
  info: "outline",
  warning: "default",
  blocking: "destructive",
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
