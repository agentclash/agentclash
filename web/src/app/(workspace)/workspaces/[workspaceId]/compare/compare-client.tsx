"use client";

import type {
  ComparisonResponse,
  DeltaHighlight,
  Run,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AlertTriangle,
  CheckCircle2,
  XCircle,
  ArrowUpRight,
  ArrowDownRight,
  Minus,
} from "lucide-react";
import Link from "next/link";
import { scorePercent } from "@/lib/scores";
import { ReleaseGatesSection } from "./release-gates-section";

// --- State badge ---

function stateBadge(state: string) {
  switch (state) {
    case "comparable":
      return (
        <Badge variant="default">
          <CheckCircle2 data-icon="inline-start" className="size-3" />
          Comparable
        </Badge>
      );
    case "partial_evidence":
      return (
        <Badge variant="secondary">
          <AlertTriangle data-icon="inline-start" className="size-3" />
          Partial Evidence
        </Badge>
      );
    case "not_comparable":
      return (
        <Badge variant="destructive">
          <XCircle data-icon="inline-start" className="size-3" />
          Not Comparable
        </Badge>
      );
    default:
      return <Badge variant="outline">{state}</Badge>;
  }
}

// --- Delta display ---

function deltaDisplay(delta: DeltaHighlight) {
  const value = delta.delta;
  if (value == null) return <span className="text-muted-foreground">&mdash;</span>;

  const pct = (value * 100).toFixed(1);
  const formatted = value > 0 ? `+${pct}%` : `${pct}%`;

  switch (delta.outcome) {
    case "better": {
      const BetterIcon = value > 0 ? ArrowUpRight : ArrowDownRight;
      return (
        <span className="text-emerald-400 flex items-center gap-1 justify-end">
          <BetterIcon className="size-3" />
          {formatted}
        </span>
      );
    }
    case "worse": {
      const WorseIcon = value < 0 ? ArrowDownRight : ArrowUpRight;
      return (
        <span className="text-red-400 flex items-center gap-1 justify-end">
          <WorseIcon className="size-3" />
          {formatted}
        </span>
      );
    }
    case "same":
      return (
        <span className="text-muted-foreground flex items-center gap-1 justify-end">
          <Minus className="size-3" />
          0.0%
        </span>
      );
    default:
      return <span className="text-muted-foreground">{formatted}</span>;
  }
}

function metricLabel(metric: string): string {
  return metric
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

// --- Component ---

interface CompareClientProps {
  comparison: ComparisonResponse;
  baselineRun: Run;
  candidateRun: Run;
  workspaceId: string;
}

export function CompareClient({
  comparison,
  baselineRun,
  candidateRun,
  workspaceId,
}: CompareClientProps) {
  const isNotComparable = comparison.state === "not_comparable";
  const isPartial = comparison.state === "partial_evidence";

  return (
    <div className="space-y-6">
      {/* Header: Baseline vs Candidate */}
      <div>
        <div className="flex items-center gap-3 mb-2">
          <h1 className="text-lg font-semibold tracking-tight">
            <Link
              href={`/workspaces/${workspaceId}/runs/${baselineRun.id}`}
              className="hover:underline underline-offset-4"
            >
              {baselineRun.name}
            </Link>
            <span className="text-muted-foreground mx-2">vs</span>
            <Link
              href={`/workspaces/${workspaceId}/runs/${candidateRun.id}`}
              className="hover:underline underline-offset-4"
            >
              {candidateRun.name}
            </Link>
          </h1>
          {stateBadge(comparison.state)}
        </div>
        <p className="text-sm text-muted-foreground">
          Baseline: {baselineRun.name} &middot; Candidate: {candidateRun.name}
        </p>
      </div>

      {/* Partial evidence warning */}
      {isPartial && (
        <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3">
          <div className="flex items-center gap-2 text-sm font-medium text-amber-400 mb-1">
            <AlertTriangle className="size-4" />
            Partial Evidence
          </div>
          <p className="text-sm text-muted-foreground">
            This comparison is based on incomplete data. Some metrics may be
            unavailable or unreliable.
          </p>
          {comparison.evidence_quality.warnings &&
            comparison.evidence_quality.warnings.length > 0 && (
              <ul className="mt-2 text-xs text-muted-foreground list-disc list-inside space-y-0.5">
                {comparison.evidence_quality.warnings.map((w, i) => (
                  <li key={i}>{w}</li>
                ))}
              </ul>
            )}
        </div>
      )}

      {/* Not comparable — show reason */}
      {isNotComparable && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 px-4 py-3">
          <div className="flex items-center gap-2 text-sm font-medium text-destructive mb-1">
            <XCircle className="size-4" />
            Not Comparable
          </div>
          {comparison.regression_reasons.length > 0 ? (
            <ul className="text-sm text-destructive/80 list-disc list-inside space-y-0.5">
              {comparison.regression_reasons.map((reason, i) => (
                <li key={i}>{reason}</li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-destructive/80">
              These runs cannot be compared.
              {comparison.reason_code && ` Reason: ${comparison.reason_code}`}
            </p>
          )}
        </div>
      )}

      {/* Key deltas table */}
      {!isNotComparable && comparison.key_deltas.length > 0 && (
        <div>
          <h2 className="text-sm font-semibold mb-3">Key Deltas</h2>
          <div className="rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Metric</TableHead>
                  <TableHead className="text-right">Baseline</TableHead>
                  <TableHead className="text-right">Candidate</TableHead>
                  <TableHead className="text-right">Delta</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {comparison.key_deltas.map((delta) => (
                  <TableRow key={delta.metric}>
                    <TableCell className="font-medium">
                      {metricLabel(delta.metric)}
                    </TableCell>
                    <TableCell className="text-right">
                      {scorePercent(delta.baseline_value)}
                    </TableCell>
                    <TableCell className="text-right">
                      {scorePercent(delta.candidate_value)}
                    </TableCell>
                    <TableCell className="text-right">
                      {deltaDisplay(delta)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </div>
      )}

      {/* Regression reasons */}
      {!isNotComparable &&
        comparison.regression_reasons.length > 0 && (
          <div>
            <h2 className="text-sm font-semibold mb-3">Regressions</h2>
            <div className="rounded-lg border border-red-500/20 bg-red-500/5 px-4 py-3">
              <ul className="text-sm text-red-400 list-disc list-inside space-y-1">
                {comparison.regression_reasons.map((reason, i) => (
                  <li key={i}>{reason}</li>
                ))}
              </ul>
            </div>
          </div>
        )}

      {/* Evidence quality notes */}
      {comparison.evidence_quality.missing_fields &&
        comparison.evidence_quality.missing_fields.length > 0 && (
          <div>
            <h2 className="text-sm font-semibold mb-3">Evidence Notes</h2>
            <div className="rounded-lg border border-border bg-card px-4 py-3">
              <p className="text-xs text-muted-foreground mb-1">
                Missing data fields:
              </p>
              <ul className="text-xs text-muted-foreground list-disc list-inside space-y-0.5">
                {comparison.evidence_quality.missing_fields.map((field, i) => (
                  <li key={i}>{field}</li>
                ))}
              </ul>
            </div>
          </div>
        )}

      {/* Release Gates */}
      <ReleaseGatesSection
        baselineRunId={comparison.baseline_run_id}
        candidateRunId={comparison.candidate_run_id}
      />
    </div>
  );
}
