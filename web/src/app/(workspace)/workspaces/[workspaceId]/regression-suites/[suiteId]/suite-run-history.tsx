"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { History, Loader2 } from "lucide-react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";

import { createApiClient } from "@/lib/api/client";
import type { Run } from "@/lib/api/types";
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

const WORKSPACE_RUN_SCAN_LIMIT = 20;

interface SuiteRunHistoryEntry {
  run: Run;
  caseCount: number;
  passCount: number;
  failCount: number;
  warnCount: number;
}

interface SuiteRunHistoryProps {
  workspaceId: string;
  suiteId: string;
  emptyTitle?: string;
  emptyDescription?: string;
}

export function SuiteRunHistory({
  workspaceId,
  suiteId,
  emptyTitle,
  emptyDescription,
}: SuiteRunHistoryProps) {
  const { getAccessToken } = useAccessToken();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [entries, setEntries] = useState<SuiteRunHistoryEntry[]>([]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError(undefined);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        // The list endpoint does not include regression_coverage; fetch
        // the most recent runs and then hydrate their detail in parallel.
        // We cap at WORKSPACE_RUN_SCAN_LIMIT to keep the blast radius
        // bounded.
        const list = await api.get<{ items: Run[] }>(
          `/v1/workspaces/${workspaceId}/runs`,
          { params: { limit: WORKSPACE_RUN_SCAN_LIMIT, offset: 0 } },
        );
        const runs = list.items ?? [];
        const details = await Promise.all(
          runs.map((run) =>
            api.get<Run>(`/v1/runs/${run.id}`).catch(() => null),
          ),
        );
        if (cancelled) return;
        const hydrated: SuiteRunHistoryEntry[] = [];
        for (const run of details) {
          if (!run) continue;
          const coverage = run.regression_coverage;
          if (!coverage) continue;
          const suite = coverage.suites.find((s) => s.id === suiteId);
          if (!suite) continue;
          hydrated.push({
            run,
            caseCount: suite.case_count,
            passCount: suite.pass_count,
            failCount: suite.fail_count,
            warnCount: Math.max(
              0,
              suite.case_count - suite.pass_count - suite.fail_count,
            ),
          });
        }
        hydrated.sort((a, b) => {
          const aKey = a.run.finished_at ?? a.run.created_at;
          const bKey = b.run.finished_at ?? b.run.created_at;
          return bKey.localeCompare(aKey);
        });
        setEntries(hydrated);
      } catch {
        if (!cancelled) setError("Could not load run history.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getAccessToken, workspaceId, suiteId]);

  if (loading) {
    return (
      <div className="rounded-lg border border-border p-6 text-center">
        <Loader2 className="size-5 animate-spin mx-auto mb-2 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">Loading run history...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-md border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
        {error}
      </div>
    );
  }

  if (entries.length === 0) {
    return (
      <EmptyState
        icon={<History className="size-10" />}
        title={emptyTitle ?? "No runs have executed this suite yet."}
        description={
          emptyDescription ??
          `Scanned the last ${WORKSPACE_RUN_SCAN_LIMIT} workspace runs. Queue a run that includes this suite to see entries here.`
        }
      />
    );
  }

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">
        Showing runs among the last {WORKSPACE_RUN_SCAN_LIMIT} workspace runs
        that included this suite.
      </p>
      <div className="rounded-lg border border-border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Run</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Cases</TableHead>
              <TableHead className="text-right">Pass</TableHead>
              <TableHead className="text-right">Fail</TableHead>
              <TableHead className="text-right">Warn</TableHead>
              <TableHead>Finished</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((entry) => (
              <TableRow key={entry.run.id}>
                <TableCell>
                  <Link
                    href={`/workspaces/${workspaceId}/runs/${entry.run.id}`}
                    className="font-medium text-foreground hover:underline underline-offset-4"
                  >
                    {entry.run.name}
                  </Link>
                </TableCell>
                <TableCell>
                  <Badge variant="outline">{entry.run.status}</Badge>
                </TableCell>
                <TableCell className="text-right font-[family-name:var(--font-mono)] text-sm">
                  {entry.caseCount}
                </TableCell>
                <TableCell className="text-right font-[family-name:var(--font-mono)] text-sm text-emerald-400">
                  {entry.passCount}
                </TableCell>
                <TableCell className="text-right font-[family-name:var(--font-mono)] text-sm text-red-400">
                  {entry.failCount}
                </TableCell>
                <TableCell className="text-right font-[family-name:var(--font-mono)] text-sm text-amber-400">
                  {entry.warnCount}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {entry.run.finished_at
                    ? new Date(entry.run.finished_at).toLocaleString()
                    : "—"}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
