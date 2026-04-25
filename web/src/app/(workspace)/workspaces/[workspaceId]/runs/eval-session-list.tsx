"use client";

import Link from "next/link";
import { FlaskConical } from "lucide-react";

import type {
  EvalSessionListItem,
  EvalSessionStatus,
  ListEvalSessionsResponse,
} from "@/lib/api/types";
import { useApiQuery } from "@/lib/api/swr";
import { workspacePageSizes } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import {
  formatEvalSessionMetricName,
  normalizeEvalSessionWarnings,
  shortEvalSessionId,
} from "@/lib/eval-sessions";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { evalSessionStatusVariant } from "./status-variant";

const ACTIVE_STATUSES: EvalSessionStatus[] = ["queued", "running", "aggregating"];
const PAGE_SIZE = workspacePageSizes.runs;
const POLL_INTERVAL_MS = 5000;

function formatPrimaryMetric(item: EvalSessionListItem): string {
  const primaryMetric = item.aggregate_result?.metric_routing?.primary_metric;
  if (primaryMetric === "pass_at_k") return "pass@k";
  if (primaryMetric === "pass_pow_k") return "pass^k";
  return "—";
}

function formatRunSummary(item: EvalSessionListItem): string {
  const counts = item.summary.run_counts;
  if (counts.total === 0) return "No child runs";
  if (counts.completed === counts.total) return `${counts.completed}/${counts.total} completed`;
  if (counts.failed > 0) return `${counts.completed}/${counts.total} completed · ${counts.failed} failed`;
  if (counts.running > 0 || counts.queued > 0 || counts.provisioning > 0 || counts.scoring > 0) {
    return `${counts.completed}/${counts.total} completed · ${counts.running + counts.queued + counts.provisioning + counts.scoring} active`;
  }
  if (counts.cancelled > 0) return `${counts.completed}/${counts.total} completed · ${counts.cancelled} cancelled`;
  return `${counts.completed}/${counts.total} completed`;
}

export function EvalSessionList({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiQuery<ListEvalSessionsResponse>(
    "/v1/eval-sessions",
    {
      workspace_id: workspaceId,
      limit: PAGE_SIZE,
      offset: 0,
    },
    {
      refreshInterval: (response) =>
        response?.items.some((item) =>
          ACTIVE_STATUSES.includes(item.eval_session.status),
        )
          ? POLL_INTERVAL_MS
          : 0,
    },
  );
  const sessions = data?.items ?? [];

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
        Failed to load eval sessions.
      </div>
    );
  }

  if (sessions.length === 0) {
    return (
      <EmptyState
        icon={<FlaskConical className="size-10" />}
        title="No eval sessions yet"
        description="Create a repeated eval session to aggregate multiple benchmark runs into one reliable result."
      />
    );
  }

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">
        Recent eval sessions with repeated-run aggregation, pass metrics, and session-level evidence warnings.
      </p>

      <div className="rounded-lg border border-border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Session</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Repetitions</TableHead>
              <TableHead>Runs</TableHead>
              <TableHead>Primary Metric</TableHead>
              <TableHead>Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions.map((item) => {
              const warnings = normalizeEvalSessionWarnings(item.evidence_warnings);
              const warningCount = warnings.length;
              return (
                <TableRow key={item.eval_session.id}>
                  <TableCell>
                    <Link
                      href={`/workspaces/${workspaceId}/eval-sessions/${item.eval_session.id}`}
                      className="font-medium text-foreground hover:underline underline-offset-4"
                    >
                      Eval Session {shortEvalSessionId(item.eval_session.id)}
                    </Link>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {warningCount > 0
                        ? `${warningCount} warning${warningCount === 1 ? "" : "s"}`
                        : "No evidence warnings"}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={evalSessionStatusVariant[item.eval_session.status] ?? "outline"}
                    >
                      {item.eval_session.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {item.eval_session.repetitions}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatRunSummary(item)}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatPrimaryMetric(item) === "—"
                      ? "—"
                      : formatEvalSessionMetricName(formatPrimaryMetric(item))}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(item.eval_session.created_at).toLocaleDateString()}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
